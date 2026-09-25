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
	TouchConversation(ctx context.Context, id string, at time.Time) error
	AppendMessage(ctx context.Context, m domain.ChatMessage) error
	// RecentMessages คืน limit ข้อความล่าสุดของห้อง เรียงเก่า→ใหม่
	RecentMessages(ctx context.Context, conversationID string, limit int) ([]domain.ChatMessage, error)
	// CountAnswersSince นับคำตอบของ assistant ใน service นี้ตั้งแต่ since (ใช้คิดโควตา)
	CountAnswersSince(ctx context.Context, officeID, serviceID string, since time.Time) (int64, error)
}
