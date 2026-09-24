package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

type OfficeRepository interface {
	List(ctx context.Context) ([]domain.Office, error)
	Get(ctx context.Context, id string) (domain.Office, error)
	GetByPublicKey(ctx context.Context, key string) (domain.Office, error)
	// GetByOrigin หา office ที่มีโดเมนนี้ (origin ต้องผ่าน domain.NormalizeOrigin มาแล้ว)
	GetByOrigin(ctx context.Context, origin string) (domain.Office, error)
	Save(ctx context.Context, o domain.Office) error
	Delete(ctx context.Context, id string) error
	// AllOrigins รวม origin ของทุก office
	AllOrigins(ctx context.Context) ([]string, error)
}

// SessionRequest = สิ่งที่ host ส่งมาขอตั๋ว (server-to-server) — ดู contract-host-ai.md §2
type SessionRequest struct {
	PublicKey string
	ServiceID string
	Secret    string
	Kind      string
	User      domain.HostUser
	Grant     string
}

// BrowserSessionRequest = widget ขอตั๋วเอง (connector โหมด browser · ไม่มี host ออกให้)
//
// ตัวตนมาจากหน้าเว็บ (ไม่ได้ตรวจลายเซ็น) — ข้อมูลจริงยังถูกคุมโดยหลังบ้าน เพราะ widget ยิงด้วย token ของแอดมินเอง
type BrowserSessionRequest struct {
	PublicKey string
	ServiceID string
	Origin    string
	User      domain.HostUser
	TokenFP   string
}

type SessionResult struct {
	Ticket    string `json:"ticket"`
	ExpiresAt int64  `json:"expires_at"`
	TTLSec    int64  `json:"ttl_sec"`
}

type OfficeService interface {
	List(ctx context.Context) ([]domain.Office, error)
	Get(ctx context.Context, id string) (domain.Office, error)
	Create(ctx context.Context, id, label, kind, actor string) (domain.Office, error)
	Update(ctx context.Context, id string, patch domain.UpdateOffice, actor string) (domain.Office, error)
	Delete(ctx context.Context, id string) error
	RotateKey(ctx context.Context, id, actor string) (domain.Office, error)

	AddService(ctx context.Context, officeID, serviceID, label, actor string) (domain.Office, error)
	UpdateService(ctx context.Context, officeID, serviceID string, patch domain.UpdateService, actor string) (domain.Office, error)
	RemoveService(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error)

	// secret_key ต่อ service (R3) — คืน plaintext ครั้งเดียว
	IssueSecret(ctx context.Context, officeID, serviceID, actor string) (string, domain.Office, error)
	RotateSecret(ctx context.Context, officeID, serviceID, actor string) (string, domain.Office, error)
	CommitSecret(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error)
	RevokeSecret(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error)

	// เพิ่มเพดานชั่วคราวของรอบเดือนนี้ (บันทึกผู้ทำ)
	IncreaseQuota(ctx context.Context, officeID, serviceID string, amount int64, reason, actor string) (domain.Office, error)

	// ResolveByPublicKey ใช้ตอน serve bundle / page-config — ตรวจแค่ว่า key มีจริง
	ResolveByPublicKey(ctx context.Context, publicKey string) (domain.Office, error)
	// ResolveByOrigin หา office จาก header Origin — ไม่มี Origin = ErrOriginRequired · ไม่มี office ไหนลงทะเบียนโดเมนนี้ = ErrOriginNotAllowed
	ResolveByOrigin(ctx context.Context, origin string) (domain.Office, error)
	// OriginIssues ตรวจข้อมูลเดิมตอนเริ่มระบบ: โดเมนที่ซ้ำข้าม office หรือรูปแบบผิด (ไว้เตือนใน log)
	OriginIssues(ctx context.Context) ([]string, error)

	// OpenSession = host ขอตั๋วให้ผู้ใช้ที่ host ยืนยันแล้ว (R4)
	OpenSession(ctx context.Context, req SessionRequest) (SessionResult, error)
	OpenBrowserSession(ctx context.Context, req BrowserSessionRequest) (SessionResult, error)
	// Bootstrap = widget ถือตั๋วมาถามหน้าตา/สถานะ (หลัง D-87 ไม่มีการอ่าน token ของ host แล้ว)
	Bootstrap(ctx context.Context, publicKey, serviceID, origin string, t domain.AccessTicket) (domain.Bootstrap, error)
	// CheckTicket ตรวจตั๋วกับ path + สถานะล่าสุดของ office/service (revoke/ปิด มีผลทันที)
	CheckTicket(ctx context.Context, publicKey, serviceID string, t domain.AccessTicket) (domain.Office, domain.Service, error)
}
