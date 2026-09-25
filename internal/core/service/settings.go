package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// SettingsService อ่าน/แก้ตั้งค่าระบบ · เก็บค่าล่าสุดไว้ใน memory (backend รันเครื่องเดียว — แก้แล้วมีผลทันที)
//
// DB อ่านไม่ได้ = ใช้ค่าล่าสุดที่มี (หรือค่าเริ่มต้น) แชทต้องไม่ล้มเพราะอ่าน settings ไม่ได้
type SettingsService struct {
	repo  port.SettingsRepository
	audit port.AuditRecorder
	now   func() time.Time

	mu  sync.Mutex
	cur *domain.Settings
}

func NewSettingsService(repo port.SettingsRepository, audit port.AuditRecorder) *SettingsService {
	if audit == nil {
		audit = port.NopAudit{}
	}
	return &SettingsService{repo: repo, audit: audit, now: time.Now}
}

// Get คืนค่าปัจจุบัน (อ่าน DB ครั้งแรกครั้งเดียว)
func (s *SettingsService) Get(ctx context.Context) domain.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur != nil {
		return *s.cur
	}
	v, err := s.repo.Get(ctx)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		v = domain.DefaultSettings()
	case err != nil:
		log.Printf("[WARN] อ่าน settings ไม่ได้ — ใช้ค่าเริ่มต้นชั่วคราว: %v", err)
		return domain.DefaultSettings() // ไม่ cache — ครั้งหน้าลองอ่านใหม่
	default:
		v.FillDefaults()
	}
	s.cur = &v
	return v
}

// Current ใช้ในจุดที่ไม่มี ctx ของ request (เช่นตั้งอายุตั๋ว)
func (s *SettingsService) Current() domain.Settings { return s.Get(context.Background()) }

// SettingsPatch — field ที่ไม่ส่ง (nil) คงค่าเดิม · UpdatedAt = ค่าที่ผู้แก้เห็นล่าสุด (กันทับกัน)
type SettingsPatch struct {
	MaxConcurrent   *int      `json:"max_concurrent"`
	TicketTTLMin    *int      `json:"ticket_ttl_min"`
	SupportMessage  *string   `json:"support_message"`
	LLMTimeoutSec   *int      `json:"llm_timeout_sec"`
	StreamTimeout   *int      `json:"stream_timeout_sec"`
	MaxOutputTokens *int      `json:"max_output_tokens"`
	HistoryTurns    *int      `json:"history_turns"`
	ToolTimeoutMs   *int      `json:"tool_timeout_ms"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ErrSettingsConflict — มีคนแก้ไปก่อนตั้งแต่ผู้แก้โหลดหน้า
var ErrSettingsConflict = errors.New("SETTINGS_CONFLICT")

func sameMillis(a, b time.Time) bool {
	return a.Truncate(time.Millisecond).Equal(b.Truncate(time.Millisecond))
}

func (s *SettingsService) Update(ctx context.Context, p SettingsPatch, actor string) (domain.Settings, error) {
	before := s.Get(ctx)
	if !sameMillis(p.UpdatedAt, before.UpdatedAt) {
		return before, ErrSettingsConflict
	}
	next := before
	for _, f := range []struct {
		src *int
		dst *int
	}{
		{p.MaxConcurrent, &next.MaxConcurrent}, {p.TicketTTLMin, &next.TicketTTLMin},
		{p.LLMTimeoutSec, &next.LLMTimeoutSec}, {p.StreamTimeout, &next.StreamTimeout},
		{p.MaxOutputTokens, &next.MaxOutputTokens}, {p.HistoryTurns, &next.HistoryTurns},
		{p.ToolTimeoutMs, &next.ToolTimeoutMs},
	} {
		if f.src != nil {
			*f.dst = *f.src
		}
	}
	if p.SupportMessage != nil {
		next.SupportMessage = *p.SupportMessage
	}
	if err := next.Validate(); err != nil {
		return before, err
	}
	next.UpdatedAt = s.now().Truncate(time.Millisecond) // Mongo เก็บละเอียดแค่ ms
	next.UpdatedBy = actor
	if err := s.repo.Save(ctx, next); err != nil {
		return before, fmt.Errorf("บันทึก settings ไม่สำเร็จ: %w", err)
	}
	s.mu.Lock()
	s.cur = &next
	s.mu.Unlock()

	if changes := domain.DiffFields(before, next, "updated_at", "updated_by"); len(changes) > 0 {
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditSettingsUpdate, TargetType: "settings", TargetID: "global", TargetLabel: "ตั้งค่าระบบ",
			Summary: fmt.Sprintf("แก้ตั้งค่าระบบ %d รายการ: %s", len(changes), changedFieldNames(changes)),
			Changes: changes,
		})
	}
	return next, nil
}
