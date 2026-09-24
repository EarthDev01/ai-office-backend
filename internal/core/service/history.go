package service

import (
	"context"
	"fmt"
	"time"

	"ai-office-backend/internal/core/domain"
)

// ประวัติที่ผู้ใช้เห็นเองใน widget = 7 วัน · เฉพาะ office+service+user ในตั๋ว (spec §7.2 · B-15)
const UserHistoryWindow = 7 * 24 * time.Hour

func (s *ChatService) ListHistory(ctx context.Context, office domain.Office, svc domain.Service, t domain.AccessTicket, days int) ([]domain.Conversation, error) {
	if days <= 0 || days > 7 {
		days = 7
	}
	since := s.Now().Add(-time.Duration(days) * 24 * time.Hour)
	list, err := s.Convs.ListForUser(ctx, office.ID, svc.ID, t.User.ID, since, 50)
	if err != nil || t.TokenFP == "" {
		return list, err
	}
	out := list[:0]
	for _, c := range list {
		if ownsConversation(c, t) {
			out = append(out, c)
		}
	}
	return out, nil
}

// ownsConversation — ห้องเป็นของผู้ถือตั๋วนี้ · โหมด browser ต้องเป็นรอบล็อกอินเดียวกันด้วย
// (ตัวตนในโหมด browser มาจากหน้าเว็บ ไม่ได้ตรวจลายเซ็น — กันคนอ้างชื่อผู้อื่นมาเปิดประวัติ)
func ownsConversation(c domain.Conversation, t domain.AccessTicket) bool {
	if c.UserID != t.User.ID {
		return false
	}
	return t.TokenFP == "" || c.TokenFP == t.TokenFP
}

// UserMessage = ข้อความที่ส่งกลับ widget (ตัดร่องรอยภายในออก)
type UserMessage struct {
	ID        string        `json:"id"`
	Role      string        `json:"role"`
	Text      string        `json:"text"`
	Cards     []domain.Card `json:"cards"`
	CreatedAt time.Time     `json:"created_at"`
}

func (s *ChatService) GetHistory(ctx context.Context, office domain.Office, svc domain.Service, t domain.AccessTicket, id string) (domain.Conversation, []UserMessage, error) {
	c, err := s.Convs.Get(ctx, office.ID, svc.ID, id)
	if err != nil {
		return domain.Conversation{}, nil, err
	}
	if !ownsConversation(c, t) || s.Now().Sub(c.LastMessageAt) > UserHistoryWindow {
		return domain.Conversation{}, nil, domain.ErrNotFound
	}
	msgs, err := s.Msgs.ListByConversation(ctx, office.ID, svc.ID, c.ID, 200)
	if err != nil {
		return domain.Conversation{}, nil, err
	}
	out := make([]UserMessage, 0, len(msgs))
	for _, m := range msgs {
		cards := m.Cards
		if cards == nil {
			cards = []domain.Card{}
		}
		out = append(out, UserMessage{ID: m.ID, Role: m.Role, Text: m.Text, Cards: cards, CreatedAt: m.CreatedAt})
	}
	return c, out, nil
}

var closeReasons = map[string]bool{"switch_service": true, "user_closed": true, "logout": true}

// CloseConversation = เปลี่ยน service/ปิดห้อง (B-15) · ห้องที่ปิดแล้วถามต่อไม่ได้ (ถามใหม่ = ห้องใหม่)
func (s *ChatService) CloseConversation(ctx context.Context, office domain.Office, svc domain.Service, t domain.AccessTicket, id, reason string) error {
	if !closeReasons[reason] {
		return fmt.Errorf("reason ไม่รู้จัก")
	}
	c, err := s.Convs.Get(ctx, office.ID, svc.ID, id)
	if err != nil {
		return err
	}
	if !ownsConversation(c, t) {
		return domain.ErrNotFound
	}
	return s.Convs.Close(ctx, office.ID, svc.ID, id, s.Now(), reason)
}
