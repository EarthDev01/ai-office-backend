package port

import (
	"time"

	"ai-office-backend/internal/core/domain"
)

// AccessTicketIssuer ออก/ตรวจตั๋วของ widget (คนละ secret กับ console JWT)
type AccessTicketIssuer interface {
	Issue(t domain.AccessTicket) (string, error)
	Verify(token string) (domain.AccessTicket, error)
}

// GrantSealer ปิดผนึก grant ของ host ไว้ในตั๋ว — ผูกกับ office/service/user (AAD) กันสลับตั๋ว
type GrantSealer interface {
	Seal(grant string, bind string) (string, error)
	Open(sealed string, bind string) (string, error)
}

// Clock ให้ test คุมเวลาได้ (ตัดวัน/หมดอายุ)
type Clock func() time.Time
