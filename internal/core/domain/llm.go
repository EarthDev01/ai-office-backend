package domain

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// LLMSettings คือโมเดลที่ใช้ตอบแชท — แก้ได้จากคอนโซล มีผลกับคำถามถัดไปทันที
//
// API key ไม่อยู่ที่นี่ — อยู่ใน .env แยกตาม provider (ดู LLMProviderInfo.EnvKey) ไม่เข้า DB ไม่ออกหน้าเว็บ
type LLMSettings struct {
	Provider string `json:"provider" bson:"provider"`
	Model    string `json:"model"    bson:"model"`
	Effort   string `json:"effort"   bson:"effort"`   // anthropic เท่านั้น
	BaseURL  string `json:"base_url" bson:"base_url"` // openai เท่านั้น
}

const (
	LLMAnthropic = "anthropic"
	LLMGemini    = "gemini"
	LLMOpenAI    = "openai"
)

// LLMProviderInfo คือ provider ที่โค้ดรองรับ — หน้าคอนโซลใช้สร้างตัวเลือก
type LLMProviderInfo struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	EnvKey   string   `json:"env_key"` // ชื่อตัวแปรใน .env ที่เก็บ key ของ provider นี้
	Models   []string `json:"models"`  // ชื่อที่แนะนำ — พิมพ์ชื่ออื่นเองได้
	Efforts  []string `json:"efforts,omitempty"`
	NeedsURL bool     `json:"needs_base_url"`
}

var LLMProviderCatalog = []LLMProviderInfo{
	{
		ID: LLMAnthropic, Label: "Claude (Anthropic)", EnvKey: "ANTHROPIC_API_KEY",
		Models:  []string{"claude-sonnet-5", "claude-opus-5-5", "claude-haiku-4-5-20251001"},
		Efforts: []string{"low", "medium", "high", "xhigh", "max"},
	},
	{
		ID: LLMGemini, Label: "Gemini (Google)", EnvKey: "GEMINI_API_KEY",
		Models: []string{"gemini-2.5-flash", "gemini-2.5-pro"},
	},
	{
		ID: LLMOpenAI, Label: "OpenAI-compatible (เช่น GLM)", EnvKey: "OPENAI_API_KEY", NeedsURL: true,
		Models: []string{"glm-4.6"},
	},
}

func LLMProviderByID(id string) (LLMProviderInfo, bool) {
	for _, p := range LLMProviderCatalog {
		if p.ID == id {
			return p, true
		}
	}
	return LLMProviderInfo{}, false
}

// DefaultLLMSettings ใช้เมื่อยังไม่เคยตั้ง และ .env ไม่ได้บอกค่าตั้งต้นมา
func DefaultLLMSettings() LLMSettings {
	return LLMSettings{Provider: LLMAnthropic, Model: "claude-sonnet-5", Effort: "low"}
}

// Normalize ตัดช่องว่าง และล้าง field ที่ provider นั้นไม่ใช้ (กันค่าค้างจาก provider เดิม)
func (l *LLMSettings) Normalize() {
	l.Provider = strings.ToLower(strings.TrimSpace(l.Provider))
	l.Model = strings.TrimSpace(l.Model)
	l.Effort = strings.ToLower(strings.TrimSpace(l.Effort))
	l.BaseURL = strings.TrimRight(strings.TrimSpace(l.BaseURL), "/")
	if l.Provider == LLMAnthropic {
		if l.Effort == "" {
			l.Effort = "low"
		}
	} else {
		l.Effort = ""
	}
	if l.Provider != LLMOpenAI {
		l.BaseURL = ""
	}
}

// Validate เรียกหลัง Normalize — ไม่ได้ตรวจว่ามี key (ขึ้นกับ .env ของเครื่อง ไม่ใช่ค่าที่กรอก)
func (l LLMSettings) Validate() error {
	p, ok := LLMProviderByID(l.Provider)
	if !ok {
		return fmt.Errorf("llm.provider %q ไม่รู้จัก — ใช้ได้: anthropic | gemini | openai", l.Provider)
	}
	if l.Model == "" || len(l.Model) > 100 || strings.ContainsAny(l.Model, " \t\n") {
		return fmt.Errorf("llm.model ต้องไม่ว่าง ไม่มีช่องว่าง และยาวไม่เกิน 100 ตัวอักษร")
	}
	if len(p.Efforts) > 0 && !slices.Contains(p.Efforts, l.Effort) {
		return fmt.Errorf("llm.effort ต้องเป็น %s", strings.Join(p.Efforts, " | "))
	}
	if p.NeedsURL {
		u, err := url.Parse(l.BaseURL)
		if l.BaseURL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("llm.base_url ต้องเป็น URL แบบ http(s):// (provider %s ต้องมี)", l.Provider)
		}
	}
	return nil
}
