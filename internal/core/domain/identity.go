package domain

import "errors"

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
)

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

// Credential คือสิ่งที่หน้า office ส่งมาเพื่อยืนยันตัวตน
//
// หน้า office เก็บ token ไว้ที่ localStorage["auth_token"] แล้วส่งเป็น Authorization: Bearer
// ไม่ใช่ cookie
type Credential struct {
	Token string
}

// Caller คือตัวตนที่อ่านได้จาก token ของหน้า office
//
// ServiceID ไม่ได้อยู่ในนี้โดยเจตนา — หน้าเว็บเป็นคนบอกว่ากำลังเปิด service ไหน
// (เก็บที่ localStorage["web-service"] ไม่มี session ฝั่ง server ให้เอา)
// แล้วเราตรวจกับ Services ข้างล่างนี้ว่าเขามีสิทธิ์จริงไหม
type Caller struct {
	AdminID     string
	Username    string
	OfficeID    string
	RoleName    string
	Level       int32
	Permissions []string // permission code จาก Role.Permission
	Services    []string // service id จาก Role.ListService ที่ Permission == true
}

// CanAccessService — แหล่งความจริงว่าแอดมินคนนี้เข้า service ไหนได้ (มาจาก Role.ListService)
func (c Caller) CanAccessService(serviceID string) bool {
	for _, s := range c.Services {
		if s == serviceID {
			return true
		}
	}
	return false
}
