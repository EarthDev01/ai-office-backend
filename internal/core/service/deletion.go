package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// DeletionService ลบข้อมูลแชทตามคำขอ (PDPA) — ลบจริง ไม่ใช่ซ่อน
//
// ลบ: messages · verifications ของข้อความที่ลบ · ห้องที่ไม่เหลือข้อความ
// ไม่ลบ: ตัวนับการใช้ token/สรุปรายวัน (มีแค่ตัวเลข) · access_log · audit_logs · deletion_requests
type DeletionService struct {
	chats   port.ChatRepository
	verifs  port.VerificationRepository
	access  port.AccessLogRepository
	repo    port.DeletionRepository
	offices port.OfficeService
	audit   port.AuditRecorder
	now     func() time.Time
}

func NewDeletionService(chats port.ChatRepository, verifs port.VerificationRepository, access port.AccessLogRepository,
	repo port.DeletionRepository, offices port.OfficeService, audit port.AuditRecorder) *DeletionService {
	if audit == nil {
		audit = port.NopAudit{}
	}
	return &DeletionService{chats: chats, verifs: verifs, access: access, repo: repo, offices: offices, audit: audit, now: time.Now}
}

// RecoverStale — ตอนเปิด server: คำขอที่ค้าง running แปลว่า process ตายกลางทาง
func (s *DeletionService) RecoverStale(ctx context.Context) {
	n, err := s.repo.FailRunning(ctx, "server หยุดทำงานระหว่างลบ — ข้อมูลอาจถูกลบไปบางส่วน ให้ส่งคำขอเดิมซ้ำ", s.now())
	if err != nil {
		log.Printf("[WARN] ตรวจคำขอลบที่ค้างไม่สำเร็จ: %v", err)
	} else if n > 0 {
		log.Printf("[WARN] พบคำขอลบค้าง %d รายการ — เปลี่ยนเป็น failed", n)
	}
}

type DeletionInput struct {
	OfficeID  string     `json:"office_id"`
	ServiceID string     `json:"service_id"`
	User      string     `json:"user"`
	From      *time.Time `json:"from"`
	To        *time.Time `json:"to"`
	Reason    string     `json:"reason"`
}

// Create ตรวจขอบเขต → สร้างคำขอ running → ลบ → done/failed · ทำจบใน request เดียว
func (s *DeletionService) Create(ctx context.Context, operator string, in DeletionInput) (domain.DeletionRequest, error) {
	in.OfficeID, in.ServiceID, in.User, in.Reason = strings.TrimSpace(in.OfficeID), strings.TrimSpace(in.ServiceID),
		strings.TrimSpace(in.User), strings.TrimSpace(in.Reason)
	switch {
	case operator == "":
		return domain.DeletionRequest{}, fmt.Errorf("ต้องรู้ว่าใครเป็นผู้ขอลบ")
	case in.OfficeID == "":
		return domain.DeletionRequest{}, fmt.Errorf("ต้องระบุ office")
	case in.Reason == "":
		return domain.DeletionRequest{}, fmt.Errorf("ต้องระบุเหตุผลหรือเลขที่คำขอ")
	case in.ServiceID == "" && in.User == "" && in.From == nil && in.To == nil:
		return domain.DeletionRequest{}, fmt.Errorf("ขอบเขตกว้างเกินไป — ต้องระบุ service, ผู้ใช้ หรือช่วงเวลาอย่างน้อย 1 อย่าง")
	case in.From != nil && in.To != nil && in.To.Before(*in.From):
		return domain.DeletionRequest{}, fmt.Errorf("เวลาสิ้นสุดต้องไม่ก่อนเวลาเริ่ม")
	}
	if _, err := s.offices.Get(ctx, in.OfficeID); err != nil {
		return domain.DeletionRequest{}, fmt.Errorf("ไม่พบ office %s", in.OfficeID)
	}

	scope := domain.DeletionScope{OfficeID: in.OfficeID, ServiceID: in.ServiceID, User: in.User, From: in.From, To: in.To}
	req := domain.DeletionRequest{
		ID: domain.NewID(), Scope: scope, Reason: in.Reason, RequestedBy: operator, Status: "running", CreatedAt: s.now(),
	}
	if err := s.repo.Create(ctx, req); err != nil {
		return req, fmt.Errorf("บันทึกคำขอไม่สำเร็จ: %w", err)
	}

	err := s.run(ctx, scope, &req.Result)
	done := s.now()
	req.CompletedAt = &done
	req.Status = "done"
	if err != nil {
		req.Status, req.Error = "failed", err.Error()
	}
	if uerr := s.repo.Update(ctx, req); uerr != nil {
		log.Printf("[ERROR] บันทึกผลคำขอลบ %s ไม่สำเร็จ: %v", req.ID, uerr)
	}

	detail := fmt.Sprintf("request=%s messages=%d conversations=%d verifications=%d status=%s",
		req.ID, req.Result.Messages, req.Result.Conversations, req.Result.Verifications, req.Status)
	if lerr := s.access.Insert(ctx, domain.AccessLog{ID: domain.NewID(), Operator: operator, Action: "delete",
		OfficeID: scope.OfficeID, ServiceID: scope.ServiceID, Detail: detail, At: done}); lerr != nil {
		log.Printf("[ERROR] บันทึก access_log ของคำขอลบ %s ไม่สำเร็จ: %v", req.ID, lerr)
	}
	entry := domain.AuditEntry{
		Action: domain.AuditDeletionRequest, TargetType: "office", TargetID: scope.OfficeID, TargetLabel: scope.OfficeID,
		Summary: fmt.Sprintf("ลบข้อมูลแชทตามคำขอ (%s): ข้อความ %d · ห้อง %d · ผลตรวจ %d",
			in.Reason, req.Result.Messages, req.Result.Conversations, req.Result.Verifications),
		Meta: map[string]string{"request_id": req.ID, "service_id": scope.ServiceID, "user": scope.User},
	}
	if err != nil {
		entry.Status, entry.Reason = domain.AuditFailure, err.Error()
	}
	s.audit.Record(ctx, entry)
	return req, err
}

func (s *DeletionService) run(ctx context.Context, scope domain.DeletionScope, res *domain.DeletionResult) error {
	ids, err := s.chats.MessageIDsInScope(ctx, scope)
	if err != nil {
		return fmt.Errorf("หาข้อความในขอบเขตไม่สำเร็จ: %w", err)
	}
	if res.Verifications, err = s.verifs.DeleteByMessages(ctx, ids); err != nil {
		return fmt.Errorf("ลบผลตรวจไม่สำเร็จ: %w", err)
	}
	if res.Messages, err = s.chats.DeleteMessagesInScope(ctx, scope); err != nil {
		return fmt.Errorf("ลบข้อความไม่สำเร็จ: %w", err)
	}
	if res.Conversations, err = s.chats.DeleteEmptyConversations(ctx, scope); err != nil {
		return fmt.Errorf("ลบห้องไม่สำเร็จ: %w", err)
	}
	return nil
}

func (s *DeletionService) List(ctx context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *DeletionService) AccessLog(ctx context.Context, f port.AccessLogFilter) ([]domain.AccessLog, int64, error) {
	return s.access.List(ctx, f)
}
