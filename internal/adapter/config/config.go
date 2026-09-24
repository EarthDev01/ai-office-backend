package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Container = ค่าที่มาจาก env · ค่าคงที่อื่น ๆ (TTL, timeout) อยู่ที่จุดใช้ หรือใน settings (แก้จากคอนโซล)
//
//	APP_MODE, HTTP_PORT, STORE_DRIVER, DB_URI, DB_NAME, CONSOLE_JWT_SECRET  (เดิม)
//	TICKET_SECRET        เซ็นตั๋ว widget + ปิดผนึก grant ของ host  ██ secret
//	ANTHROPIC_API_KEY    คีย์โมเดล (K8s Secret)                      ██ secret
//	REDIS_URL            ตัวนับ/ลิมิต/แคช (บังคับบน production — P-14)
//	CONNECTORS_DIR       โฟลเดอร์ connectors/<kind>/ (ค่าเริ่มต้น ./connectors)
//	TELEGRAM_BOT_TOKEN   แจ้งเตือนโควตา (ไม่ตั้ง = log อย่างเดียว)      ██ secret
//	METRICS_TOKEN        ถ้าตั้ง /metrics ต้องส่ง Bearer นี้
type Container struct {
	App       *App
	HTTP      *HTTP
	Store     *Store
	Console   *Console
	Ticket    *Ticket
	LLM       *LLM
	Redis     *Redis
	Connector *Connector
	Notify    *Notify
	Metrics   *Metrics
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

type Ticket struct {
	Secret string // ██ secret
}

type LLM struct {
	APIKey string // ██ secret
}

type Redis struct {
	URL string // ██ อาจมีรหัสผ่าน ห้าม log
}

type Connector struct {
	Dir string
}

type Notify struct {
	TelegramToken string // ██ secret
}

type Metrics struct {
	Token string
}

const devTicketSecret = "dev-ticket-secret-change-me-dev-only-000000"

func New() (*Container, error) {
	// โหลด .env ถ้ามี — ไม่มีก็ไม่เป็นไร ใช้ env ของระบบแทน
	// ██ ค่าใน .env เป็น secret ห้าม log ออกมา
	_ = godotenv.Load()

	c := &Container{
		App:  &App{Mode: env("APP_MODE", "dev")},
		HTTP: &HTTP{Port: env("HTTP_PORT", "6767")},
		Store: &Store{
			Driver: env("STORE_DRIVER", "file"),
			URI:    os.Getenv("DB_URI"),
			DBName: env("DB_NAME", "ai_office"),
		},
		Console:   &Console{JWTSecret: env("CONSOLE_JWT_SECRET", "dev-console-jwt-secret")},
		Ticket:    &Ticket{Secret: env("TICKET_SECRET", devTicketSecret)},
		LLM:       &LLM{APIKey: os.Getenv("ANTHROPIC_API_KEY")},
		Redis:     &Redis{URL: os.Getenv("REDIS_URL")},
		Connector: &Connector{Dir: env("CONNECTORS_DIR", "./connectors")},
		Notify:    &Notify{TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN")},
		Metrics:   &Metrics{Token: os.Getenv("METRICS_TOKEN")},
	}
	return c, c.validate()
}

// validate = prod hardening (P-16 · plan V4): production ต้องมี secret จริง · Mongo · Redis
func (c *Container) validate() error {
	if c.App.IsDev() {
		return nil
	}
	var errs []string
	if os.Getenv("TICKET_SECRET") == "" || len(c.Ticket.Secret) < 32 {
		errs = append(errs, "TICKET_SECRET ต้องตั้งและยาว ≥ 32 ตัวอักษร")
	}
	if os.Getenv("CONSOLE_JWT_SECRET") == "" || len(c.Console.JWTSecret) < 32 {
		errs = append(errs, "CONSOLE_JWT_SECRET ต้องตั้งและยาว ≥ 32 ตัวอักษร")
	}
	if !c.Store.IsMongo() {
		errs = append(errs, "production ต้องใช้ STORE_DRIVER=mongo")
	}
	if c.Redis.URL == "" {
		errs = append(errs, "production ต้องตั้ง REDIS_URL (ตัวนับ/ลิมิตต้องอยู่ Redis — P-14)")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, " · "))
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
