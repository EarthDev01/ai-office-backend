package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

type OfficeRepository interface {
	List(ctx context.Context) ([]domain.Office, error)
	Get(ctx context.Context, id string) (domain.Office, error)
	GetByPublicKey(ctx context.Context, key string) (domain.Office, error)
	Save(ctx context.Context, o domain.Office) error
	Delete(ctx context.Context, id string) error
	// AllOrigins รวม origin ของทุก office ไว้ใช้ทำ CORS แบบไม่ต้อง deploy เวลาเพิ่มลูกค้า
	AllOrigins(ctx context.Context) ([]string, error)
}

type OfficeService interface {
	List(ctx context.Context) ([]domain.Office, error)
	Get(ctx context.Context, id string) (domain.Office, error)
	Create(ctx context.Context, id, label, actor string) (domain.Office, error)
	Update(ctx context.Context, id string, patch domain.UpdateOffice, actor string) (domain.Office, error)
	Delete(ctx context.Context, id string) error
	RotateKey(ctx context.Context, id, actor string) (domain.Office, error)

	AddService(ctx context.Context, officeID, serviceID, label, actor string) (domain.Office, error)
	UpdateService(ctx context.Context, officeID, serviceID string, patch domain.UpdateService, actor string) (domain.Office, error)
	RemoveService(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error)

	// Bootstrap หา office จาก public key แล้วตรวจว่า caller เข้า serviceID นั้นได้จริงไหม
	//
	// serviceID มาจากหน้าเว็บ (localStorage["web-service"]) เพราะไม่มี session ฝั่ง server
	// ให้เอา — แต่ต้องผ่านการตรวจกับ Caller.Services ก่อนเสมอ
	Bootstrap(ctx context.Context, publicKey, serviceID, origin string, caller domain.Caller) (domain.Bootstrap, error)
	// ResolveByPublicKey ใช้ตอน serve bundle — ตรวจแค่ว่า key มีจริงและ office เปิดอยู่
	ResolveByPublicKey(ctx context.Context, publicKey string) (domain.Office, error)
	AllowedOrigins(ctx context.Context) []string
}
