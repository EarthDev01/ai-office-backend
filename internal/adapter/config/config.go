package config

import (
	"encoding/json"
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

// LLM = โมเดลที่ใช้ตอบ — ไม่ผูกค่าย (port.LLM) · เลือกด้วย LLM_PROVIDER
//
//	anthropic (ค่าเริ่มต้น) : Anthropic Messages API หรือ endpoint ที่เข้ากับ Anthropic
//	openai                   : OpenAI Chat Completions API — local model (Ollama/vLLM/LM Studio) และบริการที่เข้ากันได้
type LLM struct {
	Provider  string         // LLM_PROVIDER
	APIKey    string         // LLM_API_KEY (หรือ ANTHROPIC_API_KEY) ██ secret · local model ว่างได้
	BaseURL   string         // LLM_BASE_URL (หรือ ANTHROPIC_BASE_URL) · ว่าง = ค่าเริ่มต้นของ provider
	Model     string         // LLM_MODEL — ว่าง = ตาม settings
	ExtraBody map[string]any // LLM_EXTRA_BODY (JSON) — field เพิ่มตามผู้ให้บริการ เช่น {"thinking":{"type":"disabled"}}
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
		LLM:       newLLM(),
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
	switch c.LLM.Provider {
	case "anthropic":
	case "openai":
		if c.LLM.BaseURL == "" || c.LLM.Model == "" {
			errs = append(errs, "LLM_PROVIDER=openai ต้องตั้ง LLM_BASE_URL และ LLM_MODEL")
		}
	default:
		errs = append(errs, "LLM_PROVIDER ต้องเป็น anthropic หรือ openai")
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

func newLLM() *LLM {
	l := &LLM{
		Provider: strings.ToLower(env("LLM_PROVIDER", "anthropic")),
		APIKey:   env("LLM_API_KEY", os.Getenv("ANTHROPIC_API_KEY")),
		BaseURL:  env("LLM_BASE_URL", os.Getenv("ANTHROPIC_BASE_URL")),
		Model:    os.Getenv("LLM_MODEL"),
	}
	if raw := strings.TrimSpace(os.Getenv("LLM_EXTRA_BODY")); raw != "" {
		_ = json.Unmarshal([]byte(raw), &l.ExtraBody)
	}
	return l
}
