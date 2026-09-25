package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

type OfficeRepository interface {
	List(ctx context.Context) ([]domain.Office, error)
	Get(ctx context.Context, id string) (domain.Office, error)
	// GetByOrigin หา office ที่มีโดเมนนี้ (origin ต้องผ่าน domain.NormalizeOrigin มาแล้ว)
	GetByOrigin(ctx context.Context, origin string) (domain.Office, error)
	Save(ctx context.Context, o domain.Office) error
	Delete(ctx context.Context, id string) error
}

// OfficeGroupRepository — collection office_groups · Get ไม่พบ = domain.ErrNotFound
type OfficeGroupRepository interface {
	List(ctx context.Context) ([]domain.OfficeGroup, error)
	Get(ctx context.Context, id string) (domain.OfficeGroup, error)
	Save(ctx context.Context, g domain.OfficeGroup) error
	Delete(ctx context.Context, id string) error
}

type OfficeService interface {
	List(ctx context.Context) ([]domain.Office, error)
	Get(ctx context.Context, id string) (domain.Office, error)
	// Create เพิ่ม domain (office) ในกลุ่ม · กลุ่มต้องมีอยู่จริง · URL ไม่เกิน 1
	Create(ctx context.Context, in domain.CreateOffice, actor string) (domain.Office, error)
	Update(ctx context.Context, id string, patch domain.UpdateOffice, actor string) (domain.Office, error)
	Delete(ctx context.Context, id string) error

	AddService(ctx context.Context, officeID, serviceID, label, actor string) (domain.Office, error)
	UpdateService(ctx context.Context, officeID, serviceID string, patch domain.UpdateService, actor string) (domain.Office, error)
	RemoveService(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error)

	// Bootstrap หา office จากโดเมนที่เรียกเข้ามา แล้วตรวจว่า caller เข้า serviceID นั้นได้จริงไหม
	//
	// serviceID มาจากหน้าเว็บ (localStorage["web-service"]) เพราะไม่มี session ฝั่ง server
	// ให้เอา — แต่ต้องผ่านการตรวจกับ Caller.Services ก่อนเสมอ
	Bootstrap(ctx context.Context, office domain.Office, serviceID string, caller domain.Caller) (domain.Bootstrap, error)
	// ResolveByOrigin หา office จาก header Origin — ไม่มี Origin = ErrOriginRequired,
	// ไม่มี office ไหนลงทะเบียนโดเมนนี้ = ErrOriginNotAllowed
	ResolveByOrigin(ctx context.Context, origin string) (domain.Office, error)
	ListGroups(ctx context.Context) ([]domain.OfficeGroup, error)
	CreateGroup(ctx context.Context, name, actor string) (domain.OfficeGroup, error)
	RenameGroup(ctx context.Context, id, name, actor string) (domain.OfficeGroup, error)
	// DeleteGroup ลบได้เฉพาะกลุ่มที่ไม่มี domain แล้ว
	DeleteGroup(ctx context.Context, id, actor string) error

	// OriginIssues ตรวจข้อมูลเดิมตอนเริ่มระบบ: โดเมนที่ซ้ำข้าม office หรือรูปแบบผิด (ไว้เตือนใน log)
	OriginIssues(ctx context.Context) ([]string, error)
}
