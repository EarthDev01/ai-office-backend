package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

// IdentityResolver แปลง Bearer token ของหน้า office เป็น Caller ที่ตรวจแล้ว
//
// ต้องไปถาม office-api-v10 จริง — ห้ามเชื่อค่าที่ client ส่งมาเอง
// office ถูกส่งเข้ามาด้วยเพราะแต่ละ office มี backoffice_api_url ของตัวเอง
type IdentityResolver interface {
	Resolve(ctx context.Context, office domain.Office, cred domain.Credential) (domain.Caller, error)
}

// BackofficeAPI คือช่องทางเดียวที่เราคุยกับ office-api-v10
//
// อ่านอย่างเดียวทั้งหมด — ไม่มี method ที่เขียนข้อมูลได้ตั้งแต่แรก (GC-2)
type BackofficeAPI interface {
	// EmployeeByID = GET /api/employees-byid — ระบบรู้ว่าเป็นใครจาก token เอง ไม่ต้องส่ง id
	EmployeeByID(ctx context.Context, baseURL string, cred domain.Credential) (domain.Caller, error)
}
