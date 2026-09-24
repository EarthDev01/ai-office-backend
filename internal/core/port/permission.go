package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

// RoleConfigRepository เก็บ role config (รายชื่อ role + matrix) แบบ record เดียวทั้งระบบ
//
// Get บนที่ยังไม่เคยมีของถูก save เลยต้องคืน domain.DefaultRoleConfig() ไม่ใช่ error
// (ดู service.PermissionService ที่ห่ออีกชั้น)
type RoleConfigRepository interface {
	Get(ctx context.Context) (domain.RoleConfig, error)
	Save(ctx context.Context, cfg domain.RoleConfig) error
}
