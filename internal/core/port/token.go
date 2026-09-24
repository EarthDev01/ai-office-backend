package port

import (
	"ai-office-backend/internal/core/domain"
)

// Claims คือ session claims ของ console user ที่ใช้ออก/ตรวจ token
// (ticket สำหรับ 2FA แยกไป implement ในงานอื่น ไม่ใช่ที่นี่)
type Claims struct {
	UserID   string
	Username string
	Role     domain.Role
}

type TokenIssuer interface {
	Issue(c Claims) (string, error)
	Verify(token string) (Claims, error) // err ถ้า signature/exp ผิด
}
