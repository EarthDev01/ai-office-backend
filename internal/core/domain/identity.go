package domain

import (
	"errors"
	"regexp"
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
	// GuestID = ผู้ที่ยังไม่ล็อกอินบนหน้าเว็บผู้เล่น (header Authorization: Guest <id>) · id สุ่มโดย widget เก็บใน localStorage
	GuestID string
}

// GuestIDRe — guest_id ที่ widget สร้าง (สุ่ม base64url)
var GuestIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// GuestAdminID — ใส่ prefix ให้ไม่ชนกับยูสจริงของหลังบ้าน
func GuestAdminID(guestID string) string { return "guest:" + guestID }

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
	Services    []string // service ที่เข้าได้ตามรายชื่อใน token (ใช้เมื่อ AllServices = false)
	// AllServices = เข้าได้ทุก service ของ office (connector ไม่จำกัด หรือบัญชีเข้าเงื่อนไข all_services_when)
	AllServices bool
	// ServicesStrict = รายชื่อว่างแปลว่าเข้าไม่ได้สักเว็บ (services_empty: none) · false = ว่างแปลว่าไม่จำกัด
	ServicesStrict bool
	// Guest = ผู้ที่ยังไม่ล็อกอิน (หน้าเว็บผู้เล่น) · AdminID = guest:<guest_id> · ไม่มี Username
	Guest bool
}

// ServiceAllowed — ตัดสินว่าแอดมินคนนี้เปิด service นี้ได้ไหม (ตามกติกาของหลังบ้านชนิดนั้น)
func (c Caller) ServiceAllowed(serviceID string) bool {
	if c.AllServices {
		return true
	}
	if len(c.Services) == 0 {
		return !c.ServicesStrict
	}
	return c.CanAccessService(serviceID)
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
