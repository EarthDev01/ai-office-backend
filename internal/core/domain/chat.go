package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// NewID สุ่ม id 12 bytes (hex 24 ตัว)
func NewID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var (
	bkkOnce sync.Once
	bkkLoc  *time.Location
)

// Bangkok = timezone หลักของระบบ · ไม่มี tzdata ก็ยังถูก (+07:00 ไม่มี DST)
func Bangkok() *time.Location {
	bkkOnce.Do(func() {
		l, err := time.LoadLocation("Asia/Bangkok")
		if err != nil {
			l = time.FixedZone("ICT", 7*3600)
		}
		bkkLoc = l
	})
	return bkkLoc
}

var (
	ErrBusy          = errors.New("BUSY")
	ErrQuotaExceeded = errors.New("QUOTA_EXCEEDED")
)

// ChatTicket คือตั๋วที่ widget ได้ตอนเปิด session — ผูก office + service + ผู้ใช้ + สิทธิ์
//
// ID ใช้ผูกผลของ relay: ตั๋วใบอื่นส่งผลแทรกเข้ามาไม่ได้
type ChatTicket struct {
	ID          string
	OfficeID    string
	ServiceID   string
	AdminID     string
	Username    string
	Level       int32
	Permissions []string
	ExpiresAt   time.Time
}

// Card คือข้อมูลที่ระบบสร้างเองจากผล API — ตัวเลข/ชื่อทั้งหมดอยู่ตรงนี้ ไม่ผ่าน LLM
type Card struct {
	ID        string      `json:"id"              bson:"id"`
	Kind      string      `json:"kind"            bson:"kind"` // ok | not_found | error | denied | reference
	Tool      string      `json:"tool"            bson:"tool"`
	Title     string      `json:"title"           bson:"title"`
	Fields    []CardField `json:"fields"          bson:"fields"`
	Table     *CardTable  `json:"table,omitempty" bson:"table,omitempty"`
	Note      string      `json:"note,omitempty"  bson:"note,omitempty"`
	FetchedAt time.Time   `json:"fetched_at"      bson:"fetched_at"`
	Link      *CardLink   `json:"link,omitempty"  bson:"link,omitempty"`
	Cached    bool        `json:"cached"          bson:"cached"`
}

// CardField — Display คือข้อความที่โชว์ · Value คือค่าดิบ (ไว้ให้ guard รู้จัก)
type CardField struct {
	Label   string `json:"label"   bson:"label"`
	Display string `json:"display" bson:"display"`
	Value   any    `json:"value"   bson:"value"`
	Format  string `json:"format"  bson:"format"`
}

type CardTable struct {
	Columns []CardColumn `json:"columns" bson:"columns"`
	Rows    [][]CardCell `json:"rows"    bson:"rows"`
}

type CardColumn struct {
	Label  string `json:"label"  bson:"label"`
	Format string `json:"format" bson:"format"`
}

type CardCell struct {
	Display string `json:"display" bson:"display"`
	Value   any    `json:"value"   bson:"value"`
}

// CardLink พาไปหน้าจริงของหลังบ้าน (path ของหน้า office ไม่ใช่ API)
type CardLink struct {
	Label string `json:"label" bson:"label"`
	Path  string `json:"path"  bson:"path"`
}

// ToolCall คือบันทึกการเรียก tool 1 ครั้ง — เก็บ template ของ path ไม่เก็บค่าจริง (ไม่มี PII)
type ToolCall struct {
	Tool     string `json:"tool"            bson:"tool"`
	Endpoint string `json:"endpoint"        bson:"endpoint"`
	Method   string `json:"method"          bson:"method"`
	Status   int    `json:"status"          bson:"status"`
	Ms       int64  `json:"ms"              bson:"ms"`
	OK       bool   `json:"ok"              bson:"ok"`
	Cached   bool   `json:"cached"          bson:"cached"`
	Error    string `json:"error,omitempty" bson:"error,omitempty"`
}

// Usage คือ token ที่ใช้ไปทั้งสองรอบของคำตอบ 1 ครั้ง
type Usage struct {
	InputTokens  int `json:"input_tokens"  bson:"input_tokens"`
	OutputTokens int `json:"output_tokens" bson:"output_tokens"`
}

type Conversation struct {
	ID        string    `json:"id"         bson:"_id"`
	OfficeID  string    `json:"office_id"  bson:"office_id"`
	ServiceID string    `json:"service_id" bson:"service_id"`
	AdminID   string    `json:"admin_id"   bson:"admin_id"`
	Username  string    `json:"username"   bson:"username"`
	Title     string    `json:"title"      bson:"title"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// ChatMessage คือข้อความ 1 ชิ้นในห้อง — Role: user | assistant
//
// Status ของ assistant: ok | aborted | error
type ChatMessage struct {
	ID             string     `json:"id"                         bson:"_id"`
	ConversationID string     `json:"conversation_id"            bson:"conversation_id"`
	OfficeID       string     `json:"office_id"                  bson:"office_id"`
	ServiceID      string     `json:"service_id"                 bson:"service_id"`
	Role           string     `json:"role"                       bson:"role"`
	Text           string     `json:"text"                       bson:"text"`
	Cards          []Card     `json:"cards,omitempty"            bson:"cards,omitempty"`
	ToolCalls      []ToolCall `json:"tool_calls,omitempty"       bson:"tool_calls,omitempty"`
	Usage          Usage      `json:"usage"                      bson:"usage"`
	FirstTokenMs   int64      `json:"first_token_ms,omitempty"   bson:"first_token_ms,omitempty"`
	GuardHits      int        `json:"guard_hits,omitempty"       bson:"guard_hits,omitempty"`
	Category       string     `json:"category,omitempty"         bson:"category,omitempty"`
	Status         string     `json:"status,omitempty"           bson:"status,omitempty"`
	CreatedAt      time.Time  `json:"created_at"                 bson:"created_at"`
}
