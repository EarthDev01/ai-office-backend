package port

import (
	"context"
	"time"

	"ai-office-backend/internal/core/domain"
)

// ChatTicketIssuer ออก/ตรวจตั๋วแชทของ widget (คนละ secret กับ session ของคอนโซล)
type ChatTicketIssuer interface {
	Issue(t domain.ChatTicket) (string, error)
	Verify(token string) (domain.ChatTicket, error)
	TTL() time.Duration
}

// ChatRepository เก็บห้องแชทและข้อความ (collection conversations / messages)
type ChatRepository interface {
	CreateConversation(ctx context.Context, c domain.Conversation) error
	// GetConversation — ไม่พบ = domain.ErrNotFound
	GetConversation(ctx context.Context, id string) (domain.Conversation, error)
	// TouchConversation ตั้งเวลาข้อความล่าสุดและเพิ่มจำนวนข้อความ
	TouchConversation(ctx context.Context, id string, at time.Time, addMessages int) error
	AppendMessage(ctx context.Context, m domain.ChatMessage) error
	// RecentMessages คืน limit ข้อความล่าสุดของห้อง เรียงเก่า→ใหม่
	RecentMessages(ctx context.Context, conversationID string, limit int) ([]domain.ChatMessage, error)

	// ---- ฝั่งคอนโซล (อ่านข้ามทุกเว็บ) ----

	SearchConversations(ctx context.Context, f ConversationFilter) ([]domain.Conversation, int64, error)
	GetMessage(ctx context.Context, id string) (domain.ChatMessage, error)
	SearchMessages(ctx context.Context, f MessageFilter) ([]domain.ChatMessage, int64, error)
	SetVerification(ctx context.Context, messageID, status string) error
	// CountByVerification นับคำตอบตามสถานะตรวจ (office/service ว่าง = ทั้งหมด)
	CountByVerification(ctx context.Context, officeID, serviceID string) (map[string]int64, error)

	// ---- ลบตามคำขอ ----

	// MessageIDsInScope คืน id ข้อความทั้งหมดในขอบเขต (ใช้ลบผลตรวจของข้อความเหล่านั้น)
	MessageIDsInScope(ctx context.Context, s domain.DeletionScope) ([]string, error)
	DeleteMessagesInScope(ctx context.Context, s domain.DeletionScope) (int64, error)
	// DeleteEmptyConversations ลบห้องในขอบเขต office/service/ผู้ใช้ ที่ไม่เหลือข้อความแล้ว
	DeleteEmptyConversations(ctx context.Context, s domain.DeletionScope) (int64, error)
}

// ConversationFilter — ค่าว่าง = ไม่กรอง · Text ค้นในหัวข้อห้อง · IDs = จำกัดเฉพาะห้องเหล่านี้ (nil = ไม่จำกัด)
type ConversationFilter struct {
	OfficeID, ServiceID, User, Text string
	From, To                        *time.Time // กรองเวลาเปิดห้อง
	IDs                             []string
	Limit, Offset                   int
}

// MessageFilter — Ascending = เก่าสุดก่อน (คิวตรวจ) · ไม่ใส่ = ใหม่สุดก่อน
type MessageFilter struct {
	OfficeID, ServiceID, Role, Verification string
	From, To                                *time.Time
	Limit, Offset                           int
	Ascending                               bool
}

// NormalizePage ใช้กับทุก list: limit ≤0 หรือ >500 → 50 · offset ติดลบ → 0
func NormalizePage(limit, offset int) (int, int) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// VerificationRepository — collection verifications (unique ต่อ message_id)
type VerificationRepository interface {
	Upsert(ctx context.Context, v domain.Verification) error
	DeleteByMessages(ctx context.Context, messageIDs []string) (int64, error)
	// GetByMessage — ไม่พบ = domain.ErrNotFound
	GetByMessage(ctx context.Context, messageID string) (domain.Verification, error)
}

// AccessLogRepository — collection access_log
type AccessLogRepository interface {
	Insert(ctx context.Context, e domain.AccessLog) error
	// List เรียงใหม่สุดก่อน
	List(ctx context.Context, f AccessLogFilter) ([]domain.AccessLog, int64, error)
}
