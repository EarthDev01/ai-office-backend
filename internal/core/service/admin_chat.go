package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ChatAdminService คือข้อมูลแชทฝั่งคอนโซล: ประวัติแชทและตรวจคำตอบ
//
// เป็นเส้นเดียวที่อ่านแชทข้ามทุกเว็บได้ — ทุกการค้น/เปิดอ่านเนื้อความต้องบันทึก access_log ก่อน
// ถ้าบันทึกไม่ได้หรือไม่รู้ว่าใครเป็นคนอ่าน = ไม่ให้อ่าน (fail-closed)
type ChatAdminService struct {
	chats  port.ChatRepository
	verifs port.VerificationRepository
	access port.AccessLogRepository
	usage  *UsageService // ตัวนับถูก/ผิดในสรุปรายวัน · nil = ไม่นับ
	now    func() time.Time
}

func NewChatAdminService(chats port.ChatRepository, verifs port.VerificationRepository, access port.AccessLogRepository,
	usage *UsageService) *ChatAdminService {
	return &ChatAdminService{chats: chats, verifs: verifs, access: access, usage: usage, now: time.Now}
}

// OldThreshold — ห้องที่เปิดมานานกว่านี้ต้องกดยืนยันก่อนเปิดอ่าน
const OldThreshold = 90 * 24 * time.Hour

var ErrConfirmOld = errors.New("CONFIRM_OLD_REQUIRED")

// จำนวนข้อความสูงสุดที่โหลดต่อห้อง
const (
	adminConversationMessages = 500
	queuePairMessages         = 200
)

func (s *ChatAdminService) log(ctx context.Context, operator, action, officeID, serviceID, convID, msgID, detail string) error {
	if operator == "" {
		return fmt.Errorf("ไม่รู้ว่าใครเปิดอ่าน — ปฏิเสธ")
	}
	return s.access.Insert(ctx, domain.AccessLog{
		ID: domain.NewID(), Operator: operator, Action: action, OfficeID: officeID, ServiceID: serviceID,
		ConversationID: convID, MessageID: msgID, Detail: detail, At: s.now(),
	})
}

// SearchConversations — หัวข้อห้องคือคำถามแรกของผู้ใช้ จึงนับเป็นเนื้อความ (บันทึก search ก่อนค้น)
//
// verification กรองห้องที่มีคำตอบสถานะนั้น (สถานะเก็บต่อข้อความ ไม่ใช่ต่อห้อง)
func (s *ChatAdminService) SearchConversations(ctx context.Context, operator string, f port.ConversationFilter, verification string) ([]domain.Conversation, int64, error) {
	if err := s.log(ctx, operator, "search", f.OfficeID, f.ServiceID, "", "",
		fmt.Sprintf("user=%s q=%q verification=%s", f.User, f.Text, verification)); err != nil {
		return nil, 0, err
	}
	if verification != "" {
		msgs, _, err := s.chats.SearchMessages(ctx, port.MessageFilter{OfficeID: f.OfficeID, ServiceID: f.ServiceID,
			Role: "assistant", Verification: verification, From: f.From, To: f.To, Limit: 500})
		if err != nil {
			return nil, 0, err
		}
		seen := map[string]bool{}
		f.IDs = []string{}
		for _, m := range msgs {
			if !seen[m.ConversationID] {
				seen[m.ConversationID] = true
				f.IDs = append(f.IDs, m.ConversationID)
			}
		}
	}
	return s.chats.SearchConversations(ctx, f)
}

// ConversationView คือห้อง 1 ห้องพร้อมข้อความ และผลตรวจของคำตอบที่ตรวจแล้ว (key = message id)
type ConversationView struct {
	Conversation  domain.Conversation            `json:"conversation"`
	Messages      []domain.ChatMessage           `json:"messages"`
	Verifications map[string]domain.Verification `json:"verifications"`
}

// ReadConversation เปิดอ่านทั้งห้อง = 1 แถว access_log · เก่ากว่า 90 วันต้องส่ง confirmOld
func (s *ChatAdminService) ReadConversation(ctx context.Context, operator, id string, confirmOld bool) (ConversationView, error) {
	c, err := s.chats.GetConversation(ctx, id)
	if err != nil {
		return ConversationView{}, err
	}
	action := "read_conversation"
	if s.now().Sub(c.CreatedAt) > OldThreshold {
		if !confirmOld {
			return ConversationView{}, ErrConfirmOld
		}
		action = "read_old"
	}
	if err := s.log(ctx, operator, action, c.OfficeID, c.ServiceID, c.ID, "", ""); err != nil {
		return ConversationView{}, err
	}
	msgs, err := s.chats.RecentMessages(ctx, c.ID, adminConversationMessages)
	if err != nil {
		return ConversationView{}, err
	}
	v := ConversationView{Conversation: c, Messages: msgs, Verifications: map[string]domain.Verification{}}
	for _, m := range msgs {
		if m.VerificationStatus == domain.VerifyCorrect || m.VerificationStatus == domain.VerifyWrong {
			if ver, err := s.verifs.GetByMessage(ctx, m.ID); err == nil {
				v.Verifications[m.ID] = ver
			}
		}
	}
	return v, nil
}

// QueueItem คือคำตอบที่รอตรวจ + คำถามของผู้ใช้ที่อยู่ก่อนหน้า
type QueueItem struct {
	Question string             `json:"question"`
	Answer   domain.ChatMessage `json:"answer"`
}

// VerificationQueue — คำตอบสถานะ status (ค่าเริ่มต้น pending) เรียงเก่าสุดก่อน · เปิดคิว = อ่านเนื้อความ → บันทึก
func (s *ChatAdminService) VerificationQueue(ctx context.Context, operator string, f port.MessageFilter) ([]QueueItem, int64, error) {
	f.Role = "assistant"
	if f.Verification == "" {
		f.Verification = domain.VerifyPending
	}
	f.Ascending = true
	if err := s.log(ctx, operator, "read_queue", f.OfficeID, f.ServiceID, "", "", "status="+f.Verification); err != nil {
		return nil, 0, err
	}
	list, total, err := s.chats.SearchMessages(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	// คำถามคู่กัน = ข้อความผู้ใช้ล่าสุดในห้องเดียวกันที่มาก่อนคำตอบ · โหลดประวัติห้องละครั้ง
	history := map[string][]domain.ChatMessage{}
	out := make([]QueueItem, 0, len(list))
	for _, m := range list {
		h, ok := history[m.ConversationID]
		if !ok {
			h, _ = s.chats.RecentMessages(ctx, m.ConversationID, queuePairMessages)
			history[m.ConversationID] = h
		}
		q := ""
		for _, x := range h {
			if x.Role == "user" && !x.CreatedAt.After(m.CreatedAt) {
				q = x.Text
			}
		}
		out = append(out, QueueItem{Question: q, Answer: m})
	}
	return out, total, nil
}

type VerifyInput struct {
	MessageID     string `json:"message_id"`
	Status        string `json:"status"`
	ErrorType     string `json:"error_type"`
	CorrectAnswer string `json:"correct_answer"`
	Note          string `json:"note"`
}

// Verify บันทึกผลตรวจ · ผิดต้องมีประเภท + คำตอบที่ถูก · ตรวจซ้ำทับผลเดิม · ไม่แก้ข้อความในห้อง
func (s *ChatAdminService) Verify(ctx context.Context, operator string, in VerifyInput) (domain.Verification, error) {
	if operator == "" {
		return domain.Verification{}, fmt.Errorf("ต้องรู้ว่าใครเป็นผู้ตรวจ")
	}
	in.CorrectAnswer, in.Note = strings.TrimSpace(in.CorrectAnswer), strings.TrimSpace(in.Note)
	switch in.Status {
	case domain.VerifyCorrect:
		in.ErrorType, in.CorrectAnswer = "", ""
	case domain.VerifyWrong:
		if _, ok := domain.VerificationErrorTypes[in.ErrorType]; !ok {
			return domain.Verification{}, fmt.Errorf("ตอบผิดต้องเลือกประเภท 1 ใน 5")
		}
		if in.CorrectAnswer == "" {
			return domain.Verification{}, fmt.Errorf("ตอบผิดต้องพิมพ์คำตอบที่ถูกเก็บไว้")
		}
	default:
		return domain.Verification{}, fmt.Errorf("status ต้องเป็น correct หรือ wrong")
	}
	m, err := s.chats.GetMessage(ctx, in.MessageID)
	if err != nil {
		return domain.Verification{}, err
	}
	if m.Role != "assistant" || m.VerificationStatus == "" {
		return domain.Verification{}, fmt.Errorf("ตรวจได้เฉพาะคำตอบของ AI ที่ตอบสำเร็จ")
	}
	prev, prevErr := s.verifs.GetByMessage(ctx, m.ID)
	now := s.now()
	v := domain.Verification{
		ID: domain.NewID(), OfficeID: m.OfficeID, ServiceID: m.ServiceID, MessageID: m.ID, ConversationID: m.ConversationID,
		Status: in.Status, ErrorType: in.ErrorType, CorrectAnswer: in.CorrectAnswer, Note: in.Note,
		VerifiedBy: operator, VerifiedAt: now, CreatedAt: now,
	}
	if err := s.verifs.Upsert(ctx, v); err != nil {
		return domain.Verification{}, err
	}
	if err := s.chats.SetVerification(ctx, m.ID, in.Status); err != nil {
		return domain.Verification{}, err
	}
	// สรุปรายวันลงวันของ "คำตอบ" ไม่ใช่วันที่ตรวจ · ตรวจซ้ำต้องหักผลเดิมออกก่อน
	var d domain.RollupDelta
	if prevErr == nil {
		switch prev.Status {
		case domain.VerifyCorrect:
			d.Correct--
		case domain.VerifyWrong:
			d.Wrong--
		}
	}
	if in.Status == domain.VerifyCorrect {
		d.Correct++
	} else {
		d.Wrong++
	}
	s.usage.Record(ctx, m.OfficeID, m.ServiceID, m.CreatedAt, d)
	// บันทึกแบบ best effort — ผลตรวจบันทึกไปแล้ว
	_ = s.log(ctx, operator, "verify", m.OfficeID, m.ServiceID, m.ConversationID, m.ID, in.Status)
	return v, nil
}

// VerificationStats — % ถูกแสดงคู่กับจำนวนที่ยังไม่ตรวจเสมอ พร้อมประโยคบอกฐานที่ใช้คำนวณ
type VerificationStats struct {
	Correct   int64   `json:"correct"`
	Wrong     int64   `json:"wrong"`
	Pending   int64   `json:"pending"`
	Verified  int64   `json:"verified"`
	Total     int64   `json:"total"`
	PercentOK float64 `json:"percent_correct"`
	Basis     string  `json:"basis"`
}

func (s *ChatAdminService) VerificationStats(ctx context.Context, officeID, serviceID string) (VerificationStats, error) {
	counts, err := s.chats.CountByVerification(ctx, officeID, serviceID)
	if err != nil {
		return VerificationStats{}, err
	}
	st := VerificationStats{Correct: counts[domain.VerifyCorrect], Wrong: counts[domain.VerifyWrong], Pending: counts[domain.VerifyPending]}
	st.Verified = st.Correct + st.Wrong
	st.Total = st.Verified + st.Pending
	if st.Verified > 0 {
		st.PercentOK = float64(st.Correct) * 100 / float64(st.Verified)
	}
	st.Basis = fmt.Sprintf("ถูก %d จากที่ตรวจแล้ว %d ข้อความ · ยังไม่ตรวจ %d จากคำตอบทั้งหมด %d", st.Correct, st.Verified, st.Pending, st.Total)
	return st, nil
}
