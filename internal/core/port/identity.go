package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

// IdentityResolver แปลง Bearer token ของหน้า office เป็น Caller ที่ตรวจแล้ว
//
// office ถูกส่งเข้ามาเพื่อผูกตัวตนกับ office นั้น (cache แยกราย office และ caller.OfficeID)
// รับเฉพาะ JWT จริงที่ officeลูกค้า ออกให้แอดมินตอน login
type IdentityResolver interface {
	Resolve(ctx context.Context, office domain.Office, cred domain.Credential) (domain.Caller, error)
}
