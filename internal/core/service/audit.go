package service

import (
	"context"
	"log"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// AuditService เติม id/เวลา/ผู้กระทำ/ip ให้ entry แล้วส่งลง repository
//
// ผู้กระทำกับ ip อ่านจาก context ที่ middleware ใส่ไว้ (domain.WithAuditActor / WithRequestMeta)
// ถ้า entry ตั้ง Actor มาเองแล้ว (เช่นตอน login ที่ยังไม่มี session) จะไม่ทับ
type AuditService struct {
	repo port.AuditRepository
	now  func() time.Time
}

func NewAuditService(repo port.AuditRepository, now func() time.Time) *AuditService {
	return &AuditService{repo: repo, now: now}
}

func (s *AuditService) Record(ctx context.Context, e domain.AuditEntry) {
	if e.ID == "" {
		e.ID = domain.NewAuditID()
	}
	if e.At.IsZero() {
		e.At = s.now()
	}
	if e.Category == "" {
		e.Category = domain.AuditCategory(e.Action)
	}
	if e.Status == "" {
		e.Status = domain.AuditSuccess
	}
	if e.Actor == "" && e.ActorID == "" {
		if a, ok := domain.AuditActorFrom(ctx); ok {
			e.ActorID = a.ID
			e.Actor = a.Username
			e.ActorRole = string(a.Role)
		}
	}
	if m, ok := domain.RequestMetaFrom(ctx); ok {
		e.IP, e.UserAgent, e.Method, e.Path = m.IP, m.UserAgent, m.Method, m.Path
	}
	if e.Changes == nil {
		e.Changes = []domain.FieldChange{}
	}
	if e.Meta == nil {
		e.Meta = map[string]string{}
	}

	// ใช้ context ใหม่ — request ที่ถูกยกเลิกกลางคัน (client ปิดแท็บ) ต้องไม่ทำให้ประวัติหาย
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.repo.Append(wctx, e); err != nil {
		log.Printf("[ERROR] บันทึกประวัติไม่สำเร็จ action=%s actor=%s: %v", e.Action, e.Actor, err)
	}
}

func (s *AuditService) Query(ctx context.Context, f domain.AuditFilter) (domain.AuditPage, error) {
	f.Normalize()
	if err := f.ApplyRange(s.now()); err != nil {
		return domain.AuditPage{}, err
	}
	items, total, err := s.repo.Query(ctx, f)
	if err != nil {
		return domain.AuditPage{}, err
	}
	page := domain.AuditPage{Items: items, Total: total}
	if total > domain.AuditCountCap {
		page.Total, page.TotalCapped = domain.AuditCountCap, true
	}
	return page, nil
}

func (s *AuditService) Actors(ctx context.Context) ([]string, error) {
	return s.repo.Actors(ctx)
}

// auditOr กัน nil — service ที่ไม่ได้ส่ง recorder มาจะไม่บันทึกอะไร
func auditOr(a port.AuditRecorder) port.AuditRecorder {
	if a == nil {
		return port.NopAudit{}
	}
	return a
}
