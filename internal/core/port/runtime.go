package port

import (
	"context"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
)

// ---- ตัวนับ/ลิมิต (P-14: อยู่ใน Redis ไม่ใช่หน่วยความจำโปรเซส) ----

// Semaphore จำกัดคำขอพร้อมกันต่อ service · เกินลิมิต = ปฏิเสธทันที ไม่เข้าคิว
type Semaphore interface {
	Acquire(ctx context.Context, key string, limit int, ttl time.Duration) (release func(), ok bool, err error)
}

// QuotaCounter = ตัวนับสดต่อ service ต่อเดือน
type QuotaCounter interface {
	Get(ctx context.Context, key string) (tokens, questions int64, found bool, err error)
	Init(ctx context.Context, key string, tokens, questions int64, ttl time.Duration) error
	Add(ctx context.Context, key string, tokens, questions int64, ttl time.Duration) (newTokens, newQuestions int64, err error)
}

// Cache = แคชผล tool ≤ 60 วิ (B-7) · key มี office+service เสมอ
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}

// ---- host (server-to-server) ----

type HostRequest struct {
	Method  string
	URL     string
	Header  map[string]string
	Query   map[string]string
	Body    any
	Timeout time.Duration
}

type HostResponse struct {
	Status int
	Body   []byte
	Ms     int64
}

// HostClient ยิง host ด้วยกุญแจดอกเล็กเท่านั้น — ไม่มีทางส่ง token ของแอดมิน (P-9)
type HostClient interface {
	Do(ctx context.Context, req HostRequest) (HostResponse, error)
}

// ScopedTokenSource ขอ/จำกุญแจดอกเล็ก (5 นาที) ในหน่วยความจำเท่านั้น — ไม่ลง DB/Redis
type ScopedTokenSource interface {
	Token(ctx context.Context, office domain.Office, conn *connector.Connector, t domain.AccessTicket) (string, error)
	Invalidate(office domain.Office, t domain.AccessTicket)
}

// ---- connector ----

type ConnectorRegistry interface {
	Get(kind string) (*connector.Connector, bool)
	Kinds() []string
}

// ---- repositories ของ ai_office (ทุก query กรอง office+service — P-10) ----

type SettingsRepository interface {
	Get(ctx context.Context) (domain.Settings, error)
	Save(ctx context.Context, s domain.Settings) error
}

type ConversationFilter struct {
	OfficeID      string
	ServiceID     string
	UserID        string
	From, To      *time.Time
	Text          string
	Verification  string   // pending | correct | wrong — ห้องที่มีคำตอบสถานะนี้อย่างน้อย 1 ข้อ
	IDs           []string // จำกัดเฉพาะห้องเหล่านี้ (nil = ไม่จำกัด · ว่าง = ไม่มีห้องใดผ่าน)
	Limit, Offset int
}

type ConversationRepository interface {
	Create(ctx context.Context, c domain.Conversation) error
	Get(ctx context.Context, officeID, serviceID, id string) (domain.Conversation, error)
	Touch(ctx context.Context, officeID, serviceID, id string, at time.Time, title string) error
	Close(ctx context.Context, officeID, serviceID, id string, at time.Time, reason string) error
	ListForUser(ctx context.Context, officeID, serviceID, userID string, since time.Time, limit int) ([]domain.Conversation, error)
	// Search สำหรับคอนโซล (อ่านข้ามเว็บได้ — เส้นเดียวที่ทำได้ และต้องมี access_log)
	Search(ctx context.Context, f ConversationFilter) ([]domain.Conversation, int64, error)
	GetAny(ctx context.Context, id string) (domain.Conversation, error)
	DeleteScope(ctx context.Context, s domain.DeletionScope) (int64, error)
}

type MessageFilter struct {
	OfficeID       string
	ServiceID      string
	ConversationID string
	UserID         string
	Role           string
	Verification   string // pending | correct | wrong
	From, To       *time.Time
	Text           string
	Limit, Offset  int
	Ascending      bool
}

type MessageRepository interface {
	Insert(ctx context.Context, m domain.Message) error
	ListByConversation(ctx context.Context, officeID, serviceID, conversationID string, limit int) ([]domain.Message, error)
	Get(ctx context.Context, id string) (domain.Message, error)
	Search(ctx context.Context, f MessageFilter) ([]domain.Message, int64, error)
	SetVerification(ctx context.Context, id, status string) error
	CountByVerification(ctx context.Context, officeID, serviceID string) (map[string]int64, error)
	DeleteScope(ctx context.Context, s domain.DeletionScope) (int64, error)
	MessageIDsInScope(ctx context.Context, s domain.DeletionScope) ([]string, error)
	Estimate(ctx context.Context) (int64, error)
}

type VerificationRepository interface {
	Upsert(ctx context.Context, v domain.Verification) error
	GetByMessage(ctx context.Context, messageID string) (domain.Verification, error)
	List(ctx context.Context, officeID, serviceID, status string, limit, offset int) ([]domain.Verification, int64, error)
	DeleteScope(ctx context.Context, s domain.DeletionScope, messageIDs []string) (int64, error)
}

type AccessLogRepository interface {
	Insert(ctx context.Context, l domain.AccessLog) error
	List(ctx context.Context, officeID, serviceID, operator string, limit, offset int) ([]domain.AccessLog, int64, error)
}

type DeletionRepository interface {
	Insert(ctx context.Context, d domain.DeletionRequest) error
	Update(ctx context.Context, d domain.DeletionRequest) error
	List(ctx context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error)
}

type QuotaRepository interface {
	Get(ctx context.Context, officeID, serviceID, period string) (domain.QuotaPeriod, error)
	AddUsage(ctx context.Context, officeID, serviceID, period string, tokens, questions int64, cost float64) (domain.QuotaPeriod, error)
	// MarkAlert ตั้ง flag ครั้งเดียว (atomic) — คืน true ถ้าเป็นคนแรกที่ตั้ง (ส่งแจ้งเตือนครั้งเดียว)
	MarkAlert(ctx context.Context, officeID, serviceID, period, flag string) (bool, error)
	List(ctx context.Context, period string) ([]domain.QuotaPeriod, error)
}

type RollupRepository interface {
	Add(ctx context.Context, officeID, serviceID, date string, d domain.RollupDelta) error
	List(ctx context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error)
}

// Notifier ส่งแจ้งเตือน (Telegram) · ไม่ตั้งค่า = log อย่างเดียว
type Notifier interface {
	Notify(ctx context.Context, room, text string) error
}
