package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Container เก็บเฉพาะค่าที่มาจาก env จริง 6 ตัว: APP_MODE, HTTP_PORT, STORE_DRIVER,
// DB_URI, DB_NAME, CONSOLE_JWT_SECRET · ค่าคงที่อื่น ๆ (CORS origin, TTL, timeout,
// path ของ file store) ไม่ได้อยู่ตรงนี้ — ฝังไว้ที่จุดใช้ใน _cmd/main.go แทน
type Container struct {
	App     *App
	HTTP    *HTTP
	Store   *Store
	Console *Console
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
	}, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
