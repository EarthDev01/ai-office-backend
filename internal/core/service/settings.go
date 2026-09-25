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

	llmSeed domain.LLMSettings         // ค่าตั้งต้นจาก .env ใช้จนกว่าจะบันทึกจากคอนโซลครั้งแรก
	hasKey  func(provider string) bool // nil = ไม่ตรวจ key (เทส)
}

// SetLLMSeed ใช้ค่าจาก .env เป็นโมเดลตั้งต้น เมื่อ DB ยังไม่มีค่า llm
func (s *SettingsService) SetLLMSeed(l domain.LLMSettings) {
	l.Normalize()
	if l.Validate() != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.llmSeed = l
	s.cur = nil
}

// SetLLMKeyCheck ให้การเปลี่ยน provider ตรวจว่ามี key ใน .env แล้ว
func (s *SettingsService) SetLLMKeyCheck(f func(provider string) bool) { s.hasKey = f }

func (s *SettingsService) withSeed(v domain.Settings) domain.Settings {
	if v.LLM.Provider == "" && s.llmSeed.Provider != "" {
		v.LLM = s.llmSeed
	}
	return v
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
		v.LLM = domain.LLMSettings{}
		v = s.withSeed(v)
		v.FillDefaults()
	case err != nil:
		log.Printf("[WARN] อ่าน settings ไม่ได้ — ใช้ค่าเริ่มต้นชั่วคราว: %v", err)
		d := domain.DefaultSettings()
		d.LLM = domain.LLMSettings{}
		d = s.withSeed(d)
		d.FillDefaults()
		return d // ไม่ cache — ครั้งหน้าลองอ่านใหม่
	default:
		v = s.withSeed(v)
		v.FillDefaults()
	}
	s.cur = &v
	return v
}

// Current ใช้ในจุดที่ไม่มี ctx ของ request (เช่นตั้งอายุตั๋ว)
func (s *SettingsService) Current() domain.Settings { return s.Get(context.Background()) }

// SettingsPatch — field ที่ไม่ส่ง (nil) คงค่าเดิม · UpdatedAt = ค่าที่ผู้แก้เห็นล่าสุด (กันทับกัน)
type SettingsPatch struct {
	MaxConcurrent   *int    `json:"max_concurrent"`
	TicketTTLMin    *int    `json:"ticket_ttl_min"`
	SupportMessage  *string `json:"support_message"`
	LLMTimeoutSec   *int    `json:"llm_timeout_sec"`
	StreamTimeout   *int    `json:"stream_timeout_sec"`
	MaxOutputTokens *int    `json:"max_output_tokens"`
	HistoryTurns    *int    `json:"history_turns"`
	ToolTimeoutMs   *int    `json:"tool_timeout_ms"`
	// LLM ส่งมาทั้งชุด (provider/model/effort/base_url) แทนที่ค่าเดิม
	LLM       *domain.LLMSettings `json:"llm"`
	UpdatedAt time.Time           `json:"updated_at"`
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
	next.RememberLLM() // ค่าเดิม (เช่นที่มาจาก .env) ต้องไม่หายเมื่อเปลี่ยนไป provider อื่น
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
	if p.LLM != nil {
		next.LLM = *p.LLM
	}
	if err := next.Validate(); err != nil {
		return before, err
	}
	// ตรวจ key เฉพาะตอนเปลี่ยนโมเดล — เครื่องที่ยังไม่มี key ต้องแก้ค่าอื่นได้ตามปกติ
	if next.LLM != before.LLM && s.hasKey != nil && !s.hasKey(next.LLM.Provider) {
		info, _ := domain.LLMProviderByID(next.LLM.Provider)
		return before, fmt.Errorf("ยังไม่มี API key ของ %s — ตั้ง key ในการ์ดผู้ให้บริการก่อน (หรือ %s ใน .env ถ้ายังไม่ได้ตั้ง LLM_KEY_SECRET)", info.Label, info.EnvKey)
	}
	next.RememberLLM()
	next.UpdatedAt = s.now().Truncate(time.Millisecond) // Mongo เก็บละเอียดแค่ ms
	next.UpdatedBy = actor
	if err := s.repo.Save(ctx, next); err != nil {
		return before, fmt.Errorf("บันทึก settings ไม่สำเร็จ: %w", err)
	}
	s.mu.Lock()
	s.cur = &next
	s.mu.Unlock()

	changes := domain.DiffFields(before, next, "updated_at", "updated_by", "llm", "llm_recent")
	for _, c := range domain.DiffFields(before.LLM, next.LLM) {
		c.Field = "llm." + c.Field
		changes = append(changes, c)
	}
	if len(changes) > 0 {
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditSettingsUpdate, TargetType: "settings", TargetID: "global", TargetLabel: "ตั้งค่าระบบ",
			Summary: fmt.Sprintf("แก้ตั้งค่าระบบ %d รายการ: %s", len(changes), changedFieldNames(changes)),
			Changes: changes,
		})
	}
	return next, nil
}
