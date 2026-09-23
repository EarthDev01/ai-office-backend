package domain

import "errors"

var (
	ErrNotAuthenticated  = errors.New("NOT_AUTHENTICATED")
	ErrSessionExpired    = errors.New("SESSION_EXPIRED")
	ErrForbidden         = errors.New("FORBIDDEN")
	ErrNotFound          = errors.New("NOT_FOUND")
	ErrOriginNotAllowed  = errors.New("ORIGIN_NOT_ALLOWED")
	ErrServiceNotAllowed = errors.New("SERVICE_NOT_ALLOWED")
	ErrConflict          = errors.New("CONFLICT")
	ErrUpstream          = errors.New("BACKOFFICE_UNAVAILABLE")
)

// Credential คือสิ่งที่หน้า office ส่งมาเพื่อยืนยันตัวตน
//
// office-v10x เก็บ token ไว้ที่ localStorage["auth_token"] แล้วส่งเป็น Authorization: Bearer
// ไม่ใช่ cookie — ดู backoffice-api-survey.md §2.1
//
// UserAgent / ClientIP ต้องพกไปด้วยเพราะ office-api-v10 คำนวณ secret ของ JWT
// จาก ACCESS_SECRET + User-Agent + IP ของ client (survey §2.2) — ยังเป็นข้อ O9 ที่รอตัดสิน
type Credential struct {
	Token     string
	UserAgent string
	ClientIP  string
}

// Caller คือตัวตนที่ตรวจแล้วจาก office-api-v10
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
	Cred        Credential
}

// CanAccessService — แหล่งความจริงว่าแอดมินคนนี้เข้า service ไหนได้
// มาจาก Role.ListService ของ office-api-v10 (model/employee.go:157)
func (c Caller) CanAccessService(serviceID string) bool {
	for _, s := range c.Services {
		if s == serviceID {
			return true
		}
	}
	return false
}

// Matches ใช้กับ allowlist ที่คนกรอกในคอนโซล
//
// คนกรอก "username" เป็นหลัก แต่ยอมรับ id ด้วยเผื่อบางที่ใช้ id
func (c Caller) Matches(v string) bool {
	return v != "" && (v == c.Username || v == c.AdminID)
}

func (c Caller) HasPermission(code string) bool {
	for _, p := range c.Permissions {
		if p == code {
			return true
		}
	}
	return false
}
