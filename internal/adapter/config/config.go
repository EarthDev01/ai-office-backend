package config

import (
	"fmt"
	"os"

	"ai-office-backend/internal/core/domain"

	"github.com/joho/godotenv"
)

// Container เก็บเฉพาะค่าที่มาจาก env จริง: APP_MODE, HTTP_PORT, STORE_DRIVER,
// DB_URI, DB_NAME, CONSOLE_JWT_SECRET, CHAT_TICKET_SECRET, key ของ LLM (+ LLM_* ค่าตั้งต้นครั้งแรก) · ค่าคงที่อื่น ๆ (CORS origin, TTL, timeout,
// path ของ file store) ไม่ได้อยู่ตรงนี้ — ฝังไว้ที่จุดใช้ใน _cmd/main.go แทน
type Container struct {
	App     *App
	HTTP    *HTTP
	Store   *Store
	Console *Console
	LLM     *LLM
}

// LLM — โมเดลและ API key ตั้งที่คอนโซล (ตั้งค่าระบบ) · .env เหลือ LLM_KEY_SECRET
//
// Keys ██ secret ห้าม log · ชื่อตัวแปรมาจาก domain.LLMProviderCatalog (ANTHROPIC_API_KEY / GEMINI_API_KEY / OPENAI_API_KEY)
// ถูกย้ายเข้า DB (เข้ารหัส) ครั้งแรกที่เปิด server แล้วไม่อ่านอีก · LLM_API_KEY เดิม = key ของ LLM_PROVIDER
// Provider/Model/Effort/BaseURL ใช้เป็นค่าตั้งต้นครั้งแรกเท่านั้น (ก่อนบันทึกจากคอนโซล)
type LLM struct {
	Keys map[string]string

	// KeySecret ██ secret — กุญแจหลักเข้ารหัส API key ที่ตั้งจากคอนโซล (base64 32 byte)
	// ว่าง = ตั้ง key จากหน้าเว็บไม่ได้ ใช้ Keys จาก .env แทน (production ไม่ยอมเปิด)
	KeySecret string

	Provider string
	Model    string
	Effort   string
	BaseURL  string
}

// Validate — provider ที่พิมพ์ผิดเคยทำให้ key ไปผิดตัวแล้วแชทตอบ timeout ทุกข้อความโดยไม่มีใครรู้สาเหตุ
func (l *LLM) Validate() error {
	if _, ok := domain.LLMProviderByID(l.Provider); !ok {
		return fmt.Errorf("LLM_PROVIDER=%q ไม่รู้จัก — ใช้ได้: anthropic | gemini | openai", l.Provider)
	}
	return nil
}

// Seed คือโมเดลตั้งต้นจาก .env · ไม่ได้ตั้ง LLM_MODEL = ใช้ตัวแรกที่แนะนำของ provider นั้น
func (l *LLM) Seed() domain.LLMSettings {
	s := domain.LLMSettings{Provider: l.Provider, Model: l.Model, Effort: l.Effort, BaseURL: l.BaseURL}
	if s.Model == "" {
		if p, ok := domain.LLMProviderByID(l.Provider); ok && len(p.Models) > 0 {
			s.Model = p.Models[0]
		}
	}
	return s
}

func loadLLM() *LLM {
	l := &LLM{
		Keys:      map[string]string{},
		KeySecret: os.Getenv("LLM_KEY_SECRET"),
		Provider:  env("LLM_PROVIDER", domain.LLMAnthropic),
		Model:     os.Getenv("LLM_MODEL"),
		Effort:    env("LLM_EFFORT", "low"),
		BaseURL:   os.Getenv("LLM_BASE_URL"),
	}
	for _, p := range domain.LLMProviderCatalog {
		if v := os.Getenv(p.EnvKey); v != "" {
			l.Keys[p.ID] = v
		}
	}
	if legacy := os.Getenv("LLM_API_KEY"); legacy != "" && l.Keys[l.Provider] == "" {
		l.Keys[l.Provider] = legacy
	}
	return l
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
		LLM: loadLLM(),
	}, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
