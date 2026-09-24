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

// AdminService = ข้อมูลฝั่งคอนโซล (V10) · เส้นเดียวที่อ่านข้ามเว็บได้ → ทุกการเปิดอ่านเนื้อความต้องมี access_log (P-10 · P-13)
type AdminService struct {
	Offices  port.OfficeService
	Convs    port.ConversationRepository
	Msgs     port.MessageRepository
	Verifs   port.VerificationRepository
	Access   port.AccessLogRepository
	Deletes  port.DeletionRepository
	Rollups  port.RollupRepository
	Quota    *QuotaService
	Settings *SettingsService
	Now      port.Clock
}

// OldThreshold = เก่ากว่านี้ต้องกดยืนยันเพิ่มก่อนเปิดอ่าน (spec §9 ส่วนที่ 1)
const OldThreshold = 90 * 24 * time.Hour

var ErrConfirmOld = errors.New("CONFIRM_OLD_REQUIRED")

func (s *AdminService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *AdminService) log(ctx context.Context, operator, action, officeID, serviceID, convID, msgID, detail string) error {
	if operator == "" {
		return fmt.Errorf("ไม่รู้ว่าใครเปิดอ่าน — ปฏิเสธ")
	}
	now := s.now()
	return s.Access.Insert(ctx, domain.AccessLog{
		ID: domain.NewID(), Operator: operator, Action: action, OfficeID: officeID, ServiceID: serviceID,
		ConversationID: convID, MessageID: msgID, Detail: detail, At: now, CreatedAt: now,
	})
}

// SearchConversations = รายการห้อง (หัวข้อคือคำถามแรกของผู้ใช้ จึงถือเป็นเนื้อความ → log "search")
func (s *AdminService) SearchConversations(ctx context.Context, operator string, f port.ConversationFilter) ([]domain.Conversation, int64, error) {
	if err := s.log(ctx, operator, "search", f.OfficeID, f.ServiceID, "", "",
		fmt.Sprintf("user=%s text=%q verification=%s", f.UserID, f.Text, f.Verification)); err != nil {
		return nil, 0, err
	}
	if f.Verification != "" {
		// กรองสถานะตรวจ: หาห้องจากคำตอบที่มีสถานะนั้น (สถานะเก็บต่อข้อความ ไม่ใช่ต่อห้อง)
		msgs, _, err := s.Msgs.Search(ctx, port.MessageFilter{OfficeID: f.OfficeID, ServiceID: f.ServiceID,
			Role: domain.RoleAssistant, Verification: f.Verification, From: f.From, To: f.To, Limit: 5000})
		if err != nil {
			return nil, 0, err
		}
		ids := []string{}
		seen := map[string]bool{}
		for _, m := range msgs {
			if !seen[m.ConversationID] {
				seen[m.ConversationID] = true
				ids = append(ids, m.ConversationID)
			}
		}
		f.IDs = ids
	}
	return s.Convs.Search(ctx, f)
}

type ConversationView struct {
	Conversation domain.Conversation `json:"conversation"`
	Messages     []domain.Message    `json:"messages"`
}

// ReadConversation เปิดอ่านทั้งห้อง = 1 แถว access_log (V10 verify: เปิด 1 ห้อง = 1 แถว)
func (s *AdminService) ReadConversation(ctx context.Context, operator, id string, confirmOld bool) (ConversationView, error) {
	c, err := s.Convs.GetAny(ctx, id)
	if err != nil {
		return ConversationView{}, err
	}
	action := "read_conversation"
	if s.now().Sub(c.OpenedAt) > OldThreshold {
		if !confirmOld {
			return ConversationView{}, ErrConfirmOld
		}
		action = "read_old"
	}
	msgs, err := s.Msgs.ListByConversation(ctx, c.OfficeID, c.ServiceID, c.ID, 500)
	if err != nil {
		return ConversationView{}, err
	}
	if err := s.log(ctx, operator, action, c.OfficeID, c.ServiceID, c.ID, "", ""); err != nil {
		return ConversationView{}, err
	}
	return ConversationView{Conversation: c, Messages: msgs}, nil
}

func (s *AdminService) ReadMessage(ctx context.Context, operator, id string, confirmOld bool) (domain.Message, *domain.Verification, error) {
	m, err := s.Msgs.Get(ctx, id)
	if err != nil {
		return domain.Message{}, nil, err
	}
	action := "read_message"
	if s.now().Sub(m.CreatedAt) > OldThreshold {
		if !confirmOld {
			return domain.Message{}, nil, ErrConfirmOld
		}
		action = "read_old"
	}
	if err := s.log(ctx, operator, action, m.OfficeID, m.ServiceID, m.ConversationID, m.ID, ""); err != nil {
		return domain.Message{}, nil, err
	}
	var vp *domain.Verification
	if v, err := s.Verifs.GetByMessage(ctx, id); err == nil {
		vp = &v
	}
	return m, vp, nil
}

// QueueItem = ข้อความของ AI ที่รอตรวจ + คำถามของผู้ใช้ที่อยู่ก่อนหน้า
type QueueItem struct {
	Question string         `json:"question"`
	Answer   domain.Message `json:"answer"`
}

// VerificationQueue = คิวตรวจคำตอบ (K3) · เปิดคิวก็คือเปิดอ่านเนื้อความ → log
func (s *AdminService) VerificationQueue(ctx context.Context, operator string, f port.MessageFilter) ([]QueueItem, int64, error) {
	f.Role = domain.RoleAssistant
	if f.Verification == "" {
		f.Verification = domain.VerifyPending
	}
	f.Ascending = true
	list, total, err := s.Msgs.Search(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	if err := s.log(ctx, operator, "read_queue", f.OfficeID, f.ServiceID, "", "", fmt.Sprintf("n=%d", len(list))); err != nil {
		return nil, 0, err
	}
	out := make([]QueueItem, 0, len(list))
	for _, m := range list {
		q := ""
		if hist, err := s.Msgs.ListByConversation(ctx, m.OfficeID, m.ServiceID, m.ConversationID, 500); err == nil {
			for _, h := range hist {
				if h.Role == domain.RoleUser && !h.CreatedAt.After(m.CreatedAt) {
					q = h.Text
				}
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

// Verify บันทึกผลตรวจ · ผิดต้องมีประเภท + คำตอบที่ถูก (AC-27) · ไม่แก้ข้อความเดิมในห้อง
func (s *AdminService) Verify(ctx context.Context, operator string, in VerifyInput) (domain.Verification, error) {
	if operator == "" {
		return domain.Verification{}, fmt.Errorf("ต้องรู้ว่าใครเป็นผู้ตรวจ")
	}
	switch in.Status {
	case domain.VerifyCorrect:
		in.ErrorType = ""
	case domain.VerifyWrong:
		if _, ok := domain.VerificationErrorTypes[in.ErrorType]; !ok {
			return domain.Verification{}, fmt.Errorf("ผิดต้องเลือกประเภท 1 ใน 5")
		}
		if strings.TrimSpace(in.CorrectAnswer) == "" {
			return domain.Verification{}, fmt.Errorf("ผิดต้องพิมพ์คำตอบที่ถูกเก็บไว้")
		}
	default:
		return domain.Verification{}, fmt.Errorf("status ต้องเป็น correct หรือ wrong")
	}
	m, err := s.Msgs.Get(ctx, in.MessageID)
	if err != nil {
		return domain.Verification{}, err
	}
	if m.Role != domain.RoleAssistant {
		return domain.Verification{}, fmt.Errorf("ตรวจได้เฉพาะคำตอบของ AI")
	}
	prev, prevErr := s.Verifs.GetByMessage(ctx, m.ID)
	now := s.now()
	v := domain.Verification{
		ID: domain.NewID(), OfficeID: m.OfficeID, ServiceID: m.ServiceID, MessageID: m.ID, ConversationID: m.ConversationID,
		Status: in.Status, ErrorType: in.ErrorType, CorrectAnswer: strings.TrimSpace(in.CorrectAnswer), Note: strings.TrimSpace(in.Note),
		VerifiedBy: operator, VerifiedAt: now, CreatedAt: now,
	}
	if err := s.Verifs.Upsert(ctx, v); err != nil {
		return domain.Verification{}, err
	}
	if err := s.Msgs.SetVerification(ctx, m.ID, in.Status); err != nil {
		return domain.Verification{}, err
	}
	if s.Rollups != nil {
		d := domain.RollupDelta{}
		// ตรวจซ้ำ (เปลี่ยนผล) ต้องหักของเดิมออกก่อน
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
		_ = s.Rollups.Add(ctx, m.OfficeID, m.ServiceID, domain.DateOf(m.CreatedAt), d)
	}
	_ = s.log(ctx, operator, "verify", m.OfficeID, m.ServiceID, m.ConversationID, m.ID, in.Status)
	return v, nil
}

// VerificationStats = % ถูก **คู่กับจำนวนยังไม่ตรวจเสมอ** (AC-28) · บอกฐานที่ใช้คำนวณ
type VerificationStats struct {
	Correct   int64   `json:"correct"`
	Wrong     int64   `json:"wrong"`
	Pending   int64   `json:"pending"`
	Verified  int64   `json:"verified"`
	Total     int64   `json:"total"`
	PercentOK float64 `json:"percent_correct"`
	Basis     string  `json:"basis"`
	OfficeID  string  `json:"office_id"`
	ServiceID string  `json:"service_id"`
}

func (s *AdminService) VerificationStats(ctx context.Context, officeID, serviceID string) (VerificationStats, error) {
	counts, err := s.Msgs.CountByVerification(ctx, officeID, serviceID)
	if err != nil {
		return VerificationStats{}, err
	}
	st := VerificationStats{
		Correct: counts[domain.VerifyCorrect], Wrong: counts[domain.VerifyWrong], Pending: counts[domain.VerifyPending],
		OfficeID: officeID, ServiceID: serviceID,
	}
	st.Verified = st.Correct + st.Wrong
	st.Total = st.Verified + st.Pending
	if st.Verified > 0 {
		st.PercentOK = float64(st.Correct) * 100 / float64(st.Verified)
	}
	st.Basis = fmt.Sprintf("ถูก %d จากที่ตรวจแล้ว %d ข้อความ · ยังไม่ตรวจ %d จากคำตอบทั้งหมด %d", st.Correct, st.Verified, st.Pending, st.Total)
	return st, nil
}

// ---- ลบตามคำขอ PDPA (K5) ----

type DeletionInput struct {
	OfficeID   string     `json:"office_id"`
	ServiceID  string     `json:"service_id"`
	UserID     string     `json:"user_id"`
	From       *time.Time `json:"from"`
	To         *time.Time `json:"to"`
	Reason     string     `json:"reason"`
	ApprovedBy string     `json:"approved_by"`
}

// Delete ลบ messages + conversations + verifications ในขอบเขต · บันทึกผู้ขอ/ผู้อนุมัติ/ผล (P-13)
// ClickHouse ยังไม่มีในเฟส 1 — เมื่อมีต้องลบที่นั่นด้วย (spec §7.6)
func (s *AdminService) Delete(ctx context.Context, operator string, in DeletionInput) (domain.DeletionRequest, error) {
	if operator == "" {
		return domain.DeletionRequest{}, fmt.Errorf("ต้องรู้ว่าใครเป็นผู้ลบ")
	}
	if strings.TrimSpace(in.OfficeID) == "" {
		return domain.DeletionRequest{}, fmt.Errorf("ต้องระบุ office")
	}
	if strings.TrimSpace(in.Reason) == "" {
		return domain.DeletionRequest{}, fmt.Errorf("ต้องระบุเหตุผล/เลขที่คำขอ")
	}
	if in.ServiceID == "" && in.UserID == "" && in.From == nil && in.To == nil {
		return domain.DeletionRequest{}, fmt.Errorf("ขอบเขตกว้างเกินไป — ต้องระบุ service, ผู้ใช้ หรือช่วงเวลาอย่างน้อย 1 อย่าง")
	}
	if in.From != nil && in.To != nil && in.To.Before(*in.From) {
		return domain.DeletionRequest{}, fmt.Errorf("ช่วงเวลาไม่ถูกต้อง")
	}
	scope := domain.DeletionScope{OfficeID: in.OfficeID, ServiceID: in.ServiceID, UserID: in.UserID, From: in.From, To: in.To}
	now := s.now()
	req := domain.DeletionRequest{
		ID: domain.NewID(), OfficeID: in.OfficeID, ServiceID: in.ServiceID, Scope: scope,
		Reason: strings.TrimSpace(in.Reason), RequestedBy: operator, ApprovedBy: strings.TrimSpace(in.ApprovedBy),
		Status: "running", CreatedAt: now,
	}
	if err := s.Deletes.Insert(ctx, req); err != nil {
		return domain.DeletionRequest{}, err
	}
	ids, err := s.Msgs.MessageIDsInScope(ctx, scope)
	if err == nil {
		req.Result.Verifications, err = s.Verifs.DeleteScope(ctx, scope, ids)
	}
	if err == nil {
		req.Result.Messages, err = s.Msgs.DeleteScope(ctx, scope)
	}
	if err == nil {
		req.Result.Conversations, err = s.Convs.DeleteScope(ctx, scope)
	}
	done := s.now()
	req.CompletedAt = &done
	if err != nil {
		req.Status, req.Error = "failed", err.Error()
	} else {
		req.Status = "done"
	}
	_ = s.Deletes.Update(ctx, req)
	_ = s.log(ctx, operator, "delete", in.OfficeID, in.ServiceID, "", "",
		fmt.Sprintf("request=%s user=%s messages=%d conversations=%d", req.ID, in.UserID, req.Result.Messages, req.Result.Conversations))
	if err != nil {
		return req, err
	}
	return req, nil
}

// Quotas = สถานะโควตาทุก service ของทุก office (K4)
func (s *AdminService) Quotas(ctx context.Context) ([]domain.QuotaStatus, error) {
	offices, err := s.Offices.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []domain.QuotaStatus{}
	for _, o := range offices {
		for _, svc := range o.Services {
			st, err := s.Quota.Status(ctx, o, svc)
			if err != nil {
				return nil, err
			}
			out = append(out, st)
		}
	}
	return out, nil
}

// SearchMessages = ค้นเนื้อความ (คอนโซล) → log
func (s *AdminService) SearchMessages(ctx context.Context, operator string, f port.MessageFilter) ([]domain.Message, int64, error) {
	if err := s.log(ctx, operator, "search_messages", f.OfficeID, f.ServiceID, "", "",
		fmt.Sprintf("user=%s text=%q verification=%s role=%s", f.UserID, f.Text, f.Verification, f.Role)); err != nil {
		return nil, 0, err
	}
	return s.Msgs.Search(ctx, f)
}
