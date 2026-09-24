package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

// IdentityResolver แปลง Bearer token ของหน้า office เป็น Caller ที่ตรวจแล้ว
//
// office ถูกส่งเข้ามาด้วยเพราะแต่ละ office ตั้ง backoffice_api_url ของตัวเอง
type IdentityResolver interface {
	Resolve(ctx context.Context, office domain.Office, cred domain.Credential) (domain.Caller, error)
}
