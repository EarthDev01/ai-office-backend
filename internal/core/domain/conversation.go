package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// NewID สุ่ม id 12 bytes (hex 24 ตัว) — ใช้เป็น _id ของ collection ของ AI เอง
func NewID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ทุก collection ประวัติ: มี office_id + service_id + created_at เสมอ (D-41 · AC-29)

// Conversation = ห้องแชท 1 ห้อง ของ 1 ผู้ใช้ ใน 1 service · TTL 90 วัน (spec §7.5)
type Conversation struct {
	ID        string `json:"id"              bson:"_id"`
	OfficeID  string `json:"office_id"       bson:"office_id"`
	ServiceID string `json:"service_id"      bson:"service_id"`
	Kind      string `json:"kind"            bson:"kind"`
	UserID    string `json:"user_id"         bson:"user_id"`
	UserName  string `json:"user_name"       bson:"user_name"`
	// TokenFP (โหมด browser) = ลายนิ้วมือ token หลังบ้านรอบล็อกอินที่เปิดห้อง — เปิดประวัติได้เฉพาะรอบล็อกอินเดียวกัน
	TokenFP       string     `json:"-"               bson:"token_fp,omitempty"`
	Title         string     `json:"title"           bson:"title"`
	MessageCount  int        `json:"message_count"   bson:"message_count"`
	OpenedAt      time.Time  `json:"opened_at"       bson:"opened_at"`
	LastMessageAt time.Time  `json:"last_message_at" bson:"last_message_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty" bson:"closed_at,omitempty"`
	ClosedReason  string     `json:"closed_reason,omitempty" bson:"closed_reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"      bson:"created_at"`
}

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message = 1 แถวต่อ 1 ข้อความ · append-only · **ไม่มี TTL** จนกว่าจะมี ClickHouse (D-65)
// ██ ไม่เก็บ prompt ที่ส่งเข้าโมเดล (D-43) — เก็บแค่ข้อความที่ผู้ใช้เห็น + การ์ด + ร่องรอย tool
type Message struct {
	ID             string     `json:"id"              bson:"_id"`
	OfficeID       string     `json:"office_id"       bson:"office_id"`
	ServiceID      string     `json:"service_id"      bson:"service_id"`
	ConversationID string     `json:"conversation_id" bson:"conversation_id"`
	UserID         string     `json:"user_id"         bson:"user_id"`
	UserName       string     `json:"user_name"       bson:"user_name"`
	Role           string     `json:"role"            bson:"role"`
	Text           string     `json:"text"            bson:"text"`
	Cards          []Card     `json:"cards"           bson:"cards"`
	ToolCalls      []ToolCall `json:"tool_calls"      bson:"tool_calls"`
	Usage          Usage      `json:"usage"           bson:"usage"`
	Cost           Cost       `json:"cost"            bson:"cost"`
	LatencyMs      int64      `json:"latency_ms"      bson:"latency_ms"`
	FirstTokenMs   int64      `json:"first_token_ms"  bson:"first_token_ms"`
	Model          string     `json:"model"           bson:"model"`
	Category       string     `json:"category"        bson:"category"` // data | greeting | refuse_* | … (จาก tool ที่เลือก)
	GuardHits      int        `json:"guard_hits"      bson:"guard_hits"`
	Error          string     `json:"error,omitempty" bson:"error,omitempty"`
	Aborted        bool       `json:"aborted,omitempty" bson:"aborted,omitempty"`
	// สถานะตรวจคำตอบ (denormalize ไว้กรองเร็ว) · "" = ไม่ต้องตรวจ · pending | correct | wrong
	VerificationStatus string    `json:"verification_status" bson:"verification_status"`
	CreatedAt          time.Time `json:"created_at"          bson:"created_at"`
}

// Card = ค่าที่มาจาก tool โดยตรง ไม่ผ่านการพิมพ์ของโมเดล (B-4) · มีเวลาที่ดึง + ลิงก์หน้าจริงเสมอ
type Card struct {
	ID        string      `json:"id"         bson:"id"`
	Kind      string      `json:"kind"       bson:"kind"` // ok | not_found | error | denied | reference
	Tool      string      `json:"tool"       bson:"tool"`
	Title     string      `json:"title"      bson:"title"`
	Fields    []CardField `json:"fields"     bson:"fields"`
	Table     *CardTable  `json:"table,omitempty" bson:"table,omitempty"`
	Note      string      `json:"note,omitempty"  bson:"note,omitempty"`
	FetchedAt time.Time   `json:"fetched_at" bson:"fetched_at"`
	Link      *CardLink   `json:"link,omitempty"  bson:"link,omitempty"`
	Cached    bool        `json:"cached"     bson:"cached"`
}

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

// CardLink.Path = path บนหน้าหลังบ้านจริง (relative กับ origin ของหน้า) — widget ต่อ origin เอง
type CardLink struct {
	Label string `json:"label" bson:"label"`
	Path  string `json:"path"  bson:"path"`
}

// ToolCall = ร่องรอยการยิง host 1 ครั้ง (ไม่เก็บ response body)
type ToolCall struct {
	Tool     string `json:"tool"     bson:"tool"`
	Endpoint string `json:"endpoint" bson:"endpoint"`
	Method   string `json:"method"   bson:"method"`
	Status   int    `json:"status"   bson:"status"`
	Ms       int64  `json:"ms"       bson:"ms"`
	OK       bool   `json:"ok"       bson:"ok"`
	Cached   bool   `json:"cached"   bson:"cached"`
	Error    string `json:"error,omitempty" bson:"error,omitempty"`
}

type Usage struct {
	In         int64 `json:"in"          bson:"in"`
	Out        int64 `json:"out"         bson:"out"`
	CacheRead  int64 `json:"cache_read"  bson:"cache_read"`
	CacheWrite int64 `json:"cache_write" bson:"cache_write"`
}

func (u Usage) Add(o Usage) Usage {
	return Usage{In: u.In + o.In, Out: u.Out + o.Out, CacheRead: u.CacheRead + o.CacheRead, CacheWrite: u.CacheWrite + o.CacheWrite}
}

// Total = หน่วยที่ใช้นับโควตา (input ทุกชนิด + output)
func (u Usage) Total() int64 { return u.In + u.Out + u.CacheRead + u.CacheWrite }

// Cost ใช้เรต ณ ตอนบันทึกเสมอ (rate_snapshot) — ไม่คิดย้อนหลังด้วยเรตปัจจุบัน
type Cost struct {
	Amount        float64 `json:"amount"         bson:"amount"`
	Currency      string  `json:"currency"       bson:"currency"`
	AmountLocal   float64 `json:"amount_local"   bson:"amount_local"`
	LocalCurrency string  `json:"local_currency" bson:"local_currency"`
	RateSnapshot  Pricing `json:"rate_snapshot"  bson:"rate_snapshot"`
}

func ComputeCost(u Usage, p Pricing) Cost {
	amount := (float64(u.In)*p.InputPerMTok +
		float64(u.Out)*p.OutputPerMTok +
		float64(u.CacheWrite)*p.CacheWritePerMTok +
		float64(u.CacheRead)*p.CacheReadPerMTok) / 1_000_000
	c := Cost{Amount: amount, Currency: p.Currency, RateSnapshot: p}
	if p.FXToLocal > 0 {
		c.AmountLocal = amount * p.FXToLocal
		c.LocalCurrency = p.LocalCurrency
	}
	return c
}
