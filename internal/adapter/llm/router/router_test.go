package router

import (
	"errors"
	"testing"

	"ai-office-backend/internal/core/domain"
)

func TestRouter_RebuildsOnlyWhenSettingsChange(t *testing.T) {
	cur := domain.LLMSettings{Provider: domain.LLMAnthropic, Model: "claude-sonnet-5", Effort: "low"}
	keys := map[string]string{domain.LLMAnthropic: "k", domain.LLMGemini: "g"}
	r := New(StaticKeys(keys), func() domain.LLMSettings { return cur })

	a, err := r.get()
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := r.get(); b != a {
		t.Fatal("ค่าเดิมต้องใช้ client เดิม")
	}

	cur.Model = "claude-opus-5-5"
	c, _ := r.get()
	if c == a {
		t.Fatal("เปลี่ยนโมเดลต้องสร้าง client ใหม่")
	}

	keys[domain.LLMAnthropic] = "k2"
	if d, _ := r.get(); d == c {
		t.Fatal("เปลี่ยน key ต้องสร้าง client ใหม่")
	}

	cur = domain.LLMSettings{Provider: domain.LLMGemini, Model: "gemini-2.5-flash"}
	if _, err := r.get(); err != nil || !r.Ready() {
		t.Fatalf("สลับเป็น gemini = %v ready=%v", err, r.Ready())
	}
}

func TestRouter_NoKey(t *testing.T) {
	cur := domain.LLMSettings{Provider: domain.LLMOpenAI, Model: "glm-4.6", BaseURL: "https://api.example.com/v4"}
	r := New(StaticKeys(map[string]string{domain.LLMAnthropic: "k"}), func() domain.LLMSettings { return cur })
	if r.Ready() || r.HasKey(domain.LLMOpenAI) {
		t.Fatal("ไม่มี key ต้องไม่ ready")
	}
	if _, err := r.get(); !errors.Is(err, ErrNoKey) {
		t.Fatalf("ต้องได้ ErrNoKey ได้ %v", err)
	}
}
