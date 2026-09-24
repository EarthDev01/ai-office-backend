package domain

import "time"

// Settings = ค่าระดับระบบที่แก้จากคอนโซลได้โดยไม่ต้อง deploy (spec §7.5 `settings`)
//
// ██ สิ่งที่ตั้งจากที่นี่ไม่ได้ (NG-21): กฎความปลอดภัย · สิทธิ์ · รายการ tool — อยู่ใน connectors/ + deploy
type Settings struct {
	Model               string        `json:"model"                 bson:"model"`
	MaxConcurrent       int           `json:"max_concurrent"        bson:"max_concurrent"`
	TicketTTLMin        int           `json:"ticket_ttl_min"        bson:"ticket_ttl_min"`
	DefaultMonthlyLimit int64         `json:"default_monthly_limit" bson:"default_monthly_limit"`
	SupportMessage      string        `json:"support_message"       bson:"support_message"`
	TelegramRooms       TelegramRooms `json:"telegram_rooms"        bson:"telegram_rooms"`
	Pricing             Pricing       `json:"pricing"               bson:"pricing"`
	LLMTimeoutSec       int           `json:"llm_timeout_sec"       bson:"llm_timeout_sec"`
	MaxOutputTokens     int           `json:"max_output_tokens"     bson:"max_output_tokens"`
	HistoryTurns        int           `json:"history_turns"         bson:"history_turns"`
	ToolTimeoutMs       int           `json:"tool_timeout_ms"       bson:"tool_timeout_ms"`
	Effort              string        `json:"effort"                bson:"effort"` // "" = ค่าเริ่มต้นของโมเดล · low|medium|high
	UpdatedAt           time.Time     `json:"updated_at"            bson:"updated_at"`
	UpdatedBy           string        `json:"updated_by"            bson:"updated_by"`
}

type TelegramRooms struct {
	Alerts        string `json:"alerts"         bson:"alerts"`
	ContractTests string `json:"contract_tests" bson:"contract_tests"`
}

// Pricing = ราคาต่อ 1 ล้าน token ของโมเดลที่ใช้ · เก็บ snapshot ลงทุกข้อความ (AC-6: เรต ณ ตอนบันทึก)
type Pricing struct {
	Currency          string  `json:"currency"            bson:"currency"`
	InputPerMTok      float64 `json:"input_per_mtok"      bson:"input_per_mtok"`
	OutputPerMTok     float64 `json:"output_per_mtok"     bson:"output_per_mtok"`
	CacheWritePerMTok float64 `json:"cache_write_per_mtok" bson:"cache_write_per_mtok"`
	CacheReadPerMTok  float64 `json:"cache_read_per_mtok"  bson:"cache_read_per_mtok"`
	// FXToLocal > 0 = คูณเป็นสกุลเงินท้องถิ่น (เช่น THB) เพิ่มอีกค่า · 0 = ไม่แปลง
	FXToLocal     float64 `json:"fx_to_local"    bson:"fx_to_local"`
	LocalCurrency string  `json:"local_currency" bson:"local_currency"`
}

// DefaultSettings — ค่าเริ่มต้นตาม spec (P-C13) · ราคา claude-sonnet-5 ของ Anthropic API ($2 / $10 ต่อ MTok)
func DefaultSettings() Settings {
	return Settings{
		Model:               "claude-sonnet-5",
		MaxConcurrent:       3,
		TicketTTLMin:        30,
		DefaultMonthlyLimit: 1_000_000,
		SupportMessage:      "หากต้องการความช่วยเหลือเพิ่มเติม กรุณาติดต่อทีม support ผ่านช่องทางที่ใช้ติดต่ออยู่ประจำ",
		Pricing: Pricing{
			Currency:          "USD",
			InputPerMTok:      2.0,
			OutputPerMTok:     10.0,
			CacheWritePerMTok: 2.5,
			CacheReadPerMTok:  0.2,
		},
		LLMTimeoutSec:   25,
		MaxOutputTokens: 1024,
		HistoryTurns:    6,
		ToolTimeoutMs:   6000,
	}
}

// Normalize เติมค่าที่ว่าง/ผิดด้วยค่าเริ่มต้น — กันคอนโซลตั้งค่าที่ทำให้ระบบพัง (เช่น concurrency 0)
func (s Settings) Normalize() Settings {
	d := DefaultSettings()
	if s.Model == "" {
		s.Model = d.Model
	}
	if s.MaxConcurrent <= 0 || s.MaxConcurrent > 50 {
		s.MaxConcurrent = d.MaxConcurrent
	}
	if s.TicketTTLMin < 5 || s.TicketTTLMin > 120 {
		s.TicketTTLMin = d.TicketTTLMin
	}
	if s.DefaultMonthlyLimit <= 0 {
		s.DefaultMonthlyLimit = d.DefaultMonthlyLimit
	}
	if s.SupportMessage == "" {
		s.SupportMessage = d.SupportMessage
	}
	if s.Pricing.Currency == "" {
		s.Pricing = d.Pricing
	}
	if s.LLMTimeoutSec <= 0 || s.LLMTimeoutSec > 120 {
		s.LLMTimeoutSec = d.LLMTimeoutSec
	}
	if s.MaxOutputTokens < 256 || s.MaxOutputTokens > 8192 {
		s.MaxOutputTokens = d.MaxOutputTokens
	}
	if s.HistoryTurns < 0 || s.HistoryTurns > 20 {
		s.HistoryTurns = d.HistoryTurns
	}
	if s.ToolTimeoutMs < 500 || s.ToolTimeoutMs > 30000 {
		s.ToolTimeoutMs = d.ToolTimeoutMs
	}
	switch s.Effort {
	case "", "low", "medium", "high":
	default:
		s.Effort = ""
	}
	return s
}
