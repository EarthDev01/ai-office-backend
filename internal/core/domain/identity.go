package domain

import (
	"errors"
	"time"
)

var (
	ErrNotAuthenticated  = errors.New("NOT_AUTHENTICATED")
	ErrSessionExpired    = errors.New("SESSION_EXPIRED")
	ErrForbidden         = errors.New("FORBIDDEN")
	ErrNotFound          = errors.New("NOT_FOUND")
	ErrOriginNotAllowed  = errors.New("ORIGIN_NOT_ALLOWED")
	ErrOriginRequired    = errors.New("ORIGIN_REQUIRED")
	ErrServiceNotAllowed = errors.New("SERVICE_NOT_ALLOWED")
	ErrConflict          = errors.New("CONFLICT")
	ErrUpstream          = errors.New("BACKOFFICE_UNAVAILABLE")
	ErrSecretInvalid     = errors.New("SECRET_INVALID")
	ErrTicketInvalid     = errors.New("TICKET_INVALID")
	ErrTicketExpired     = errors.New("TICKET_EXPIRED")
	ErrQuotaExceeded     = errors.New("QUOTA_EXCEEDED")
	ErrBusy              = errors.New("TOO_MANY_CONCURRENT")
	ErrBadRequest        = errors.New("BAD_REQUEST")
)

// RefusalError = ปฏิเสธพร้อมเหตุผลที่ widget/host เข้าใจได้ (office_disabled, not_in_allowlist, …)
type RefusalError struct{ Reason string }

func (e *RefusalError) Error() string { return "REFUSED:" + e.Reason }

func Refuse(reason string) error { return &RefusalError{Reason: reason} }

// HostUser คือตัวตนที่ host ยืนยันแล้วส่งมาตอนขอตั๋ว (server-to-server ด้วย secret_key)
//
// backend ไม่เคยเห็น token ของผู้ใช้เอง — host เป็นที่เดียวที่ตรวจลายเซ็นได้ (D-87)
// Permissions = code ที่ผู้ใช้เปิดดูได้จริงในหลังบ้านนั้น · ห้ามมี PII
type HostUser struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
	Level       int      `json:"level"`
	Dept        string   `json:"dept"`
}

// Matches ใช้กับ allowlist ที่คนกรอกในคอนโซล — รับทั้ง username และ id
func (u HostUser) Matches(v string) bool {
	return v != "" && (v == u.Username || v == u.ID)
}

func (u HostUser) HasPermission(code string) bool {
	for _, p := range u.Permissions {
		if p == code {
			return true
		}
	}
	return false
}

// AccessTicket = ตั๋วที่ backend ออกให้ widget (JWT อายุ ~30 นาที)
//
// office/service/user ทุกอย่างของ request ฝั่งแชทมาจากตั๋วนี้เท่านั้น (P-10)
// SealedGrant = grant ของ host ที่ถูกเข้ารหัส — เบราว์เซอร์อ่าน/ใช้ขอกุญแจดอกเล็กเองไม่ได้ (D-86)
type AccessTicket struct {
	ID          string
	OfficeID    string
	ServiceID   string
	Kind        string
	User        HostUser
	SealedGrant string
	// TokenFP = ลายนิ้วมือ (sha256) ของ token หลังบ้านรอบล็อกอินนี้ — โหมด browser เท่านั้น
	// ประวัติแชทผูกกับค่านี้ คนที่อ้างชื่อผู้อื่นเฉย ๆ จึงเปิดประวัติของเขาไม่ได้
	TokenFP     string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

func (t AccessTicket) Permissions() []string { return t.User.Permissions }

// OriginTakenError — โดเมนนี้เป็นของ office อื่นอยู่แล้ว (1 โดเมนอยู่ได้แค่ office เดียว
// เพราะโดเมนคือตัวระบุว่าเป็นลูกค้าเจ้าไหน)
type OriginTakenError struct {
	Origin      string
	OfficeID    string
	OfficeLabel string
}

func (e *OriginTakenError) Error() string {
	return "โดเมน " + e.Origin + " ถูกใช้แล้วโดย office " + e.OfficeLabel + " (" + e.OfficeID + ")"
}
