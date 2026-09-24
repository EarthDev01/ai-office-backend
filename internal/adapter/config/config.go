package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Container เก็บเฉพาะค่าที่มาจาก env จริง: APP_MODE, HTTP_PORT, STORE_DRIVER,
// DB_URI, DB_NAME, CONSOLE_JWT_SECRET, LLM_* / GLM_* · ค่าคงที่อื่น ๆ (CORS origin, TTL, timeout,
// path ของ file store) ไม่ได้อยู่ตรงนี้ — ฝังไว้ที่จุดใช้ใน _cmd/main.go แทน
type Container struct {
	App     *App
	HTTP    *HTTP
	Store   *Store
	Console *Console
	LLM     *LLM
}

// LLM ██ SPIKE — Provider เลือกตัวที่ใช้ตอบ (gemini | glm) · key ว่าง = ปิดแชท (endpoint ตอบ 503)
// ตัวจริงจะเป็น Claude — gemini/glm มีไว้ทดสอบเท่านั้น · key ทุกตัว ██ secret ห้าม log
type LLM struct {
	Provider string

	APIKey string // gemini
	Model  string // gemini

	GLMKey     string
	GLMModel   string
	GLMBaseURL string // key ของ Coding Plan ใช้ได้เฉพาะ endpoint /api/coding/paas/v4
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
type Console struct {
	JWTSecret string
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
		Console: &Console{JWTSecret: env("CONSOLE_JWT_SECRET", "dev-console-jwt-secret")},
		LLM: &LLM{
			Provider:   env("LLM_PROVIDER", "gemini"),
			APIKey:     os.Getenv("LLM_API_KEY"),
			Model:      env("LLM_MODEL", "gemini-3.6-flash"),
			GLMKey:     os.Getenv("GLM_API_KEY"),
			GLMModel:   env("GLM_MODEL", "glm-5.3"),
			GLMBaseURL: env("GLM_BASE_URL", "https://api.z.ai/api/coding/paas/v4"),
		},
	}, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
