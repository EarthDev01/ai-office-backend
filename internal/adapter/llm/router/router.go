// Package router เลือก LLM client ตามตั้งค่าระบบ ณ ตอนเรียก — แก้โมเดลในคอนโซลแล้วมีผลกับคำถามถัดไป
package router

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	anthropicllm "ai-office-backend/internal/adapter/llm/anthropic"
	"ai-office-backend/internal/adapter/llm/gemini"
	"ai-office-backend/internal/adapter/llm/openaicompat"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ErrNoKey — provider ที่เลือกไม่มี key ใน .env
var ErrNoKey = errors.New("LLM_KEY_MISSING")

type Router struct {
	keys    func(provider string) string // ██ ค่าที่คืนเป็น secret ห้าม log
	current func() domain.LLMSettings

	mu        sync.Mutex
	active    domain.LLMSettings
	activeKey string // key ที่ client ปัจจุบันใช้ — เปลี่ยน key ในคอนโซลแล้วต้องสร้าง client ใหม่
	client    port.LLMClient
}

var _ port.LLMRouter = (*Router)(nil)

// New — keys คืน key ของ provider (มาจาก LLMKeyService) · current คืนตั้งค่าล่าสุด (มี cache อยู่แล้วใน SettingsService)
func New(keys func(provider string) string, current func() domain.LLMSettings) *Router {
	return &Router{keys: keys, current: current}
}

// StaticKeys ใช้ในเทส / ตอนมีแค่ key ชุดตายตัว
func StaticKeys(m map[string]string) func(string) string {
	return func(p string) string { return m[p] }
}

func (r *Router) HasKey(provider string) bool { return r.keys(provider) != "" }

func (r *Router) Ready() bool { return r.HasKey(r.current().Provider) }

// client คืนตัวที่ตรงกับตั้งค่าล่าสุด · ค่าเปลี่ยน = สร้างใหม่ (คำถามที่กำลังตอบอยู่ใช้ตัวเดิมจนจบ)
func (r *Router) get() (port.LLMClient, error) {
	cfg := r.current()
	key := r.keys(cfg.Provider)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil && r.active == cfg && r.activeKey == key {
		return r.client, nil
	}
	c, err := build(cfg, key)
	if err != nil {
		return nil, err
	}
	r.client, r.active, r.activeKey = c, cfg, key
	log.Printf("[INFO] chat LLM: %s · model=%s", cfg.Provider, cfg.Model)
	return c, nil
}

func build(cfg domain.LLMSettings, key string) (port.LLMClient, error) {
	if key == "" {
		p, _ := domain.LLMProviderByID(cfg.Provider)
		return nil, fmt.Errorf("%w: ยังไม่ได้ตั้ง API key ของ %s", ErrNoKey, p.Label)
	}
	switch cfg.Provider {
	case domain.LLMAnthropic:
		return anthropicllm.New(key, cfg.Model, cfg.Effort), nil
	case domain.LLMGemini:
		return gemini.New(key, cfg.Model), nil
	case domain.LLMOpenAI:
		// GLM ปิดการคิดได้ผ่าน field thinking — ปิดแล้วตอบไวกว่าราว 2 เท่า · provider อื่นไม่รู้จัก field นี้
		var thinking map[string]string
		if strings.Contains(cfg.BaseURL, "z.ai") || strings.Contains(cfg.BaseURL, "bigmodel") {
			thinking = map[string]string{"type": "disabled"}
		}
		return openaicompat.New(cfg.BaseURL, key, cfg.Model, thinking), nil
	}
	return nil, fmt.Errorf("llm provider %q ไม่รู้จัก", cfg.Provider)
}

func (r *Router) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	c, err := r.get()
	if err != nil {
		return port.LLMResponse{}, err
	}
	return c.Complete(ctx, req)
}

func (r *Router) Stream(ctx context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	c, err := r.get()
	if err != nil {
		return port.LLMUsage{}, err
	}
	return c.Stream(ctx, req, onDelta)
}

// testTimeout — ทดสอบจากหน้าตั้งค่า ไม่ควรค้างนานกว่ารอบเลือก tool ปกติ
const testTimeout = 30 * time.Second

// Test ยิงข้อความสั้น 1 ครั้งด้วยค่าที่กรอก (ยังไม่บันทึก) กับ key ที่เก็บไว้ — ไม่แตะ client ที่แชทใช้อยู่
func (r *Router) Test(ctx context.Context, cfg domain.LLMSettings) (port.LLMTestResult, error) {
	return r.TestWithKey(ctx, cfg, r.keys(cfg.Provider))
}

// TestWithKey เหมือน Test แต่ใช้ key ที่ส่งมา — ใช้ลอง key ใหม่ก่อนบันทึก
func (r *Router) TestWithKey(ctx context.Context, cfg domain.LLMSettings, key string) (port.LLMTestResult, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return port.LLMTestResult{}, err
	}
	c, err := build(cfg, key)
	if err != nil {
		return port.LLMTestResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	start := time.Now()
	resp, err := c.Complete(ctx, port.LLMRequest{
		System:    "ตอบสั้นที่สุด",
		Messages:  []port.LLMMessage{{Role: "user", Text: "ตอบกลับคำว่า OK คำเดียว"}},
		MaxTokens: 256, // บางโมเดลใช้ token ส่วนหนึ่งคิดก่อนตอบ
	})
	if err != nil {
		return port.LLMTestResult{}, err
	}
	reply := strings.TrimSpace(resp.Text)
	if rs := []rune(reply); len(rs) > 200 {
		reply = string(rs[:200]) + "…"
	}
	return port.LLMTestResult{LatencyMs: time.Since(start).Milliseconds(), Reply: reply, Usage: resp.Usage}, nil
}
