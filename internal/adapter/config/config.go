package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Container struct {
	App        *App
	HTTP       *HTTP
	Store      *Store
	Admin      *Admin
	Backoffice *Backoffice
}

// Backoffice = office-api-v10 · base url อยู่ราย office ในฐานข้อมูล ไม่ได้อยู่ตรงนี้
type Backoffice struct {
	Timeout     time.Duration
	IdentityTTL time.Duration
}

type App struct {
	Mode string // dev | production
}

func (a *App) IsDev() bool { return a.Mode != "production" }

type HTTP struct {
	Port           string
	AllowedOrigins []string
}

type Store struct {
	Driver     string // mongo | file
	ConfigPath string // ใช้เมื่อ Driver=file
	URI        string // ██ secret — ห้าม log
	DBName     string
}

func (s *Store) IsMongo() bool { return s.Driver == "mongo" }

type Admin struct {
	ConsoleToken string
}

func New() (*Container, error) {
	// โหลด .env ถ้ามี — ไม่มีก็ไม่เป็นไร ใช้ env ของระบบแทน
	// ██ ค่าใน .env เป็น secret ห้าม log ออกมา
	_ = godotenv.Load()

	return &Container{
		App: &App{Mode: env("APP_MODE", "dev")},
		HTTP: &HTTP{
			Port:           env("HTTP_PORT", "6767"),
			AllowedOrigins: split(env("HTTP_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174")),
		},
		Store: &Store{
			Driver:     env("STORE_DRIVER", "file"),
			ConfigPath: env("CONFIG_STORE_PATH", "./data/offices.json"),
			URI:        os.Getenv("DB_URI"),
			DBName:     env("DB_NAME", "ai_office"),
		},
		Admin: &Admin{ConsoleToken: env("ADMIN_CONSOLE_TOKEN", "dev-console-token")},
		Backoffice: &Backoffice{
			Timeout:     envDuration("BACKOFFICE_TIMEOUT_MS", 8000),
			IdentityTTL: envDuration("IDENTITY_CACHE_TTL_MS", 60000),
		},
	}, nil
}

func envDuration(k string, defMS int) time.Duration {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return time.Duration(defMS) * time.Millisecond
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func split(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
