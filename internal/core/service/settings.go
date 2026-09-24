package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// SettingsService อ่าน settings ระดับระบบ (cache สั้น ๆ) — แก้จากคอนโซลมีผลภายในไม่กี่วินาที
type SettingsService struct {
	repo port.SettingsRepository
	ttl  time.Duration

	mu     sync.Mutex
	cached domain.Settings
	until  time.Time
}

func NewSettingsService(repo port.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo, ttl: 15 * time.Second}
}

func (s *SettingsService) Get(ctx context.Context) (domain.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.until) {
		return s.cached, nil
	}
	if s.repo == nil {
		s.cached, s.until = domain.DefaultSettings(), time.Now().Add(s.ttl)
		return s.cached, nil
	}
	st, err := s.repo.Get(ctx)
	if errors.Is(err, domain.ErrNotFound) {
		st, err = domain.DefaultSettings(), nil
	}
	if err != nil {
		// DB ล่มชั่วคราว: ใช้ค่าเดิมที่เคยอ่านได้ ถ้าไม่เคยมีใช้ค่าเริ่มต้น — ไม่ลากแชทล่มตาม
		if !s.until.IsZero() {
			return s.cached, nil
		}
		return domain.DefaultSettings(), nil
	}
	s.cached, s.until = st.Normalize(), time.Now().Add(s.ttl)
	return s.cached, nil
}

// SettingsPatch = field ที่คอนโซลแก้ได้ (pointer = ส่งมาถึงแก้)
type SettingsPatch struct {
	Model               *string               `json:"model"`
	MaxConcurrent       *int                  `json:"max_concurrent"`
	TicketTTLMin        *int                  `json:"ticket_ttl_min"`
	DefaultMonthlyLimit *int64                `json:"default_monthly_limit"`
	SupportMessage      *string               `json:"support_message"`
	TelegramRooms       *domain.TelegramRooms `json:"telegram_rooms"`
	Pricing             *domain.Pricing       `json:"pricing"`
	LLMTimeoutSec       *int                  `json:"llm_timeout_sec"`
	MaxOutputTokens     *int                  `json:"max_output_tokens"`
	HistoryTurns        *int                  `json:"history_turns"`
	ToolTimeoutMs       *int                  `json:"tool_timeout_ms"`
	Effort              *string               `json:"effort"`
}

func (s *SettingsService) Update(ctx context.Context, p SettingsPatch, actor string) (domain.Settings, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	if p.Model != nil {
		cur.Model = *p.Model
	}
	if p.MaxConcurrent != nil {
		cur.MaxConcurrent = *p.MaxConcurrent
	}
	if p.TicketTTLMin != nil {
		cur.TicketTTLMin = *p.TicketTTLMin
	}
	if p.DefaultMonthlyLimit != nil {
		cur.DefaultMonthlyLimit = *p.DefaultMonthlyLimit
	}
	if p.SupportMessage != nil {
		cur.SupportMessage = *p.SupportMessage
	}
	if p.TelegramRooms != nil {
		cur.TelegramRooms = *p.TelegramRooms
	}
	if p.Pricing != nil {
		cur.Pricing = *p.Pricing
	}
	if p.LLMTimeoutSec != nil {
		cur.LLMTimeoutSec = *p.LLMTimeoutSec
	}
	if p.MaxOutputTokens != nil {
		cur.MaxOutputTokens = *p.MaxOutputTokens
	}
	if p.HistoryTurns != nil {
		cur.HistoryTurns = *p.HistoryTurns
	}
	if p.ToolTimeoutMs != nil {
		cur.ToolTimeoutMs = *p.ToolTimeoutMs
	}
	if p.Effort != nil {
		cur.Effort = *p.Effort
	}
	cur = cur.Normalize()
	cur.UpdatedAt = time.Now()
	cur.UpdatedBy = actor
	if s.repo != nil {
		if err := s.repo.Save(ctx, cur); err != nil {
			return domain.Settings{}, err
		}
	}
	s.mu.Lock()
	s.cached, s.until = cur, time.Now().Add(s.ttl)
	s.mu.Unlock()
	return cur, nil
}
