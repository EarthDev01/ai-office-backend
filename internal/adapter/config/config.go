package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Container เก็บเฉพาะค่าที่มาจาก env จริง: APP_MODE, HTTP_PORT, STORE_DRIVER,
// DB_URI, DB_NAME, CONSOLE_JWT_SECRET, CHAT_TICKET_SECRET, LLM_* · ค่าคงที่อื่น ๆ (CORS origin, TTL, timeout,
// path ของ file store) ไม่ได้อยู่ตรงนี้ — ฝังไว้ที่จุดใช้ใน _cmd/main.go แทน
type Container struct {
	App     *App
	HTTP    *HTTP
	Store   *Store
	Console *Console
	LLM     *LLM
}

// LLM เลือกตัวที่ใช้ตอบแชท · key ว่าง = ปิดแชท (endpoint ตอบ 503) · key ██ secret ห้าม log
//
//	anthropic  Claude — ตัวที่ใช้บน production
//	gemini     ทดสอบเท่านั้น
//	openai     API แบบ Chat Completions (เช่น GLM ของ z.ai) ทดสอบเท่านั้น · ต้องตั้ง LLM_BASE_URL
type LLM struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string // openai เท่านั้น — key ของ GLM Coding Plan ใช้ได้เฉพาะ /api/coding/paas/v4
	Effort   string // anthropic เท่านั้น
}

// LLMProviders คือค่า LLM_PROVIDER ที่โค้ดรู้จัก — ค่าอื่นต้องไม่ถูกปล่อยผ่านเงียบ ๆ
var LLMProviders = map[string]bool{"anthropic": true, "gemini": true, "openai": true}

// Validate ตรวจตอนเปิด server ทุก mode — provider ที่พิมพ์ผิดเคยทำให้ตกไปใช้ตัวอื่นด้วย key ผิด
// แล้วแชทตอบ "ไม่ตอบกลับในเวลาที่กำหนด" ทุกข้อความโดยไม่มีใครรู้สาเหตุ
func (l *LLM) Validate() error {
	if !LLMProviders[l.Provider] {
		return fmt.Errorf("LLM_PROVIDER=%q ไม่รู้จัก — ใช้ได้: anthropic | gemini | openai", l.Provider)
	}
	if l.APIKey == "" {
		return nil // ปิดแชท แต่ระบบส่วนอื่นยังทำงาน
	}
	if l.Model == "" {
		return fmt.Errorf("LLM_MODEL ว่าง (provider=%s)", l.Provider)
	}
	if l.Provider == "openai" && l.BaseURL == "" {
		return fmt.Errorf("LLM_PROVIDER=openai ต้องตั้ง LLM_BASE_URL")
	}
	return nil
}

type App struct {
	Mode string // dev | production
}

func (a *App) IsDev() bool { return a.Mode != "production" }

type HTTP struct {
	Port string
}

type Store struct {
	Driver string // mongo | file
	URI    string // ██ secret — ห้าม log
	DBName string
}

func (s *Store) IsMongo() bool { return s.Driver == "mongo" }

// Console = ai-office console auth (JWT session)
// JWTSecret ██ secret ห้าม log — ใช้เซ็น/ตรวจ session token
// ChatTicketSecret ██ secret ห้าม log — เซ็นตั๋วแชทของ widget (คนละ secret กับคอนโซล)
type Console struct {
	JWTSecret        string
	ChatTicketSecret string
}

func New() (*Container, error) {
	// โหลด .env ถ้ามี — ไม่มีก็ไม่เป็นไร ใช้ env ของระบบแทน
	// ██ ค่าใน .env เป็น secret ห้าม log ออกมา
	_ = godotenv.Load()

	return &Container{
		App:  &App{Mode: env("APP_MODE", "dev")},
		HTTP: &HTTP{Port: env("HTTP_PORT", "6767")},
		Store: &Store{
			Driver: env("STORE_DRIVER", "file"),
			URI:    os.Getenv("DB_URI"),
			DBName: env("DB_NAME", "ai_office"),
		},
		Console: &Console{
			JWTSecret:        env("CONSOLE_JWT_SECRET", "dev-console-jwt-secret"),
			ChatTicketSecret: env("CHAT_TICKET_SECRET", "dev-chat-ticket-secret"),
		},
		LLM: &LLM{
			Provider: env("LLM_PROVIDER", "anthropic"),
			APIKey:   os.Getenv("LLM_API_KEY"),
			Model:    os.Getenv("LLM_MODEL"),
			BaseURL:  os.Getenv("LLM_BASE_URL"),
			Effort:   env("LLM_EFFORT", "low"),
		},
	}, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
