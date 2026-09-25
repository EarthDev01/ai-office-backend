package domain

import (
	"fmt"
	"maps"
	"strings"
	"time"
)

// Settings คือค่าระดับระบบที่แก้ได้จากคอนโซล (Mongo settings doc เดียว _id "global")
//
// ไม่ใช่ settings: API key ของ LLM (อยู่ใน .env) · กฎความปลอดภัย รายการ tool สิทธิ์ (อยู่ใน connector/โค้ด)
type Settings struct {
	MaxConcurrent   int    `json:"max_concurrent"    bson:"max_concurrent"`      // คนถามพร้อมกันต่อ office+service · เกิน = busy
	TicketTTLMin    int    `json:"ticket_ttl_min"    bson:"ticket_ttl_min"`      // อายุตั๋วแชทของ widget (นาที)
	SupportMessage  string `json:"support_message"   bson:"support_message"`     // ใช้เมื่อ connector ไม่ได้ตั้งไว้
	LLMTimeoutSec   int    `json:"llm_timeout_sec"   bson:"llm_timeout_sec"`     // รอบเลือก tool
	StreamTimeout   int    `json:"stream_timeout_sec" bson:"stream_timeout_sec"` // รอบเขียนคำตอบ
	MaxOutputTokens int    `json:"max_output_tokens" bson:"max_output_tokens"`   // รอบเขียนคำตอบ
	HistoryTurns    int    `json:"history_turns"     bson:"history_turns"`       // จำนวนรอบถาม-ตอบที่ส่งให้ LLM · 0 = ไม่ส่งประวัติ
	ToolTimeoutMs   int    `json:"tool_timeout_ms"   bson:"tool_timeout_ms"`     // ค่าเริ่มต้นเมื่อ connector ไม่ได้ตั้ง

	LLM LLMSettings `json:"llm" bson:"llm"` // โมเดลที่ใช้ตอบแชท

	// AssistantEnabled เปิดปุ่มผู้ช่วย AI ในคอนโซล · ค่าเริ่มต้นปิด (ใช้ token จริงทุกคำถาม)
	AssistantEnabled bool `json:"assistant_enabled" bson:"assistant_enabled"`
	// LLMRecent คือค่าล่าสุดที่เคยบันทึกของแต่ละ provider (key = provider id) — สลับกลับมาแล้วไม่ต้องกรอกใหม่
	// server ดูแลเอง ไม่รับจาก patch
	LLMRecent map[string]LLMSettings `json:"llm_recent" bson:"llm_recent,omitempty"`

	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
	UpdatedBy string    `json:"updated_by" bson:"updated_by"`
}

func DefaultSettings() Settings {
	return Settings{
		MaxConcurrent:   5,
		TicketTTLMin:    30,
		SupportMessage:  "หากต้องการความช่วยเหลือเพิ่มเติม กรุณาติดต่อทีม support ผ่านช่องทางที่ใช้ติดต่ออยู่ประจำ",
		LLMTimeoutSec:   25,
		StreamTimeout:   60,
		MaxOutputTokens: 1024,
		HistoryTurns:    10,
		ToolTimeoutMs:   20000,
		LLM:             DefaultLLMSettings(),
	}
}

// settingRange คือช่วงที่รับของแต่ละ field ตัวเลข
var settingRange = []struct {
	field    string
	get      func(*Settings) *int
	min, max int
}{
	{"max_concurrent", func(s *Settings) *int { return &s.MaxConcurrent }, 1, 50},
	{"ticket_ttl_min", func(s *Settings) *int { return &s.TicketTTLMin }, 5, 120},
	{"llm_timeout_sec", func(s *Settings) *int { return &s.LLMTimeoutSec }, 5, 120},
	{"stream_timeout_sec", func(s *Settings) *int { return &s.StreamTimeout }, 10, 300},
	{"max_output_tokens", func(s *Settings) *int { return &s.MaxOutputTokens }, 256, 8192},
	{"history_turns", func(s *Settings) *int { return &s.HistoryTurns }, 0, 20},
	{"tool_timeout_ms", func(s *Settings) *int { return &s.ToolTimeoutMs }, 1000, 60000},
}

// SettingRanges คืน field → [min, max] ให้หน้าคอนโซลตั้งกรอบช่องกรอก
func SettingRanges() map[string][2]int {
	out := map[string][2]int{}
	for _, r := range settingRange {
		out[r.field] = [2]int{r.min, r.max}
	}
	return out
}

// Validate ค่านอกช่วง = error บอกชื่อ field (ไม่ปัดเป็นค่าเริ่มต้นเงียบ ๆ)
func (s *Settings) Validate() error {
	var bad []string
	for _, r := range settingRange {
		if v := *r.get(s); v < r.min || v > r.max {
			bad = append(bad, fmt.Sprintf("%s ต้องอยู่ระหว่าง %d–%d (ได้ %d)", r.field, r.min, r.max, v))
		}
	}
	s.SupportMessage = strings.TrimSpace(s.SupportMessage)
	if s.SupportMessage == "" {
		bad = append(bad, "support_message ห้ามว่าง")
	}
	s.LLM.Normalize()
	if err := s.LLM.Validate(); err != nil {
		bad = append(bad, err.Error())
	}
	if len(bad) > 0 {
		return fmt.Errorf("ค่าไม่ถูกต้อง: %s", strings.Join(bad, " · "))
	}
	return nil
}

// FillDefaults เติมค่าเริ่มต้นให้ field ที่ยังไม่มีใน doc (เช่น doc ที่บันทึกก่อนมี field นั้น)
func (s *Settings) FillDefaults() {
	d := DefaultSettings()
	for _, r := range settingRange {
		if *r.get(s) == 0 && r.min > 0 {
			*r.get(s) = *r.get(&d)
		}
	}
	if strings.TrimSpace(s.SupportMessage) == "" {
		s.SupportMessage = d.SupportMessage
	}
	if s.LLM.Provider == "" {
		s.LLM = d.LLM
	}
}

// RememberLLM จำค่า llm ปัจจุบันไว้ใน LLMRecent ของ provider นั้น — clone map ก่อนแก้ (ค่าเดิมอาจถูกแชร์อยู่)
func (s *Settings) RememberLLM() {
	if s.LLM.Provider == "" {
		return
	}
	m := maps.Clone(s.LLMRecent)
	if m == nil {
		m = map[string]LLMSettings{}
	}
	m[s.LLM.Provider] = s.LLM
	s.LLMRecent = m
}
