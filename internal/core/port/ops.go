package port

import (
	"context"
	"time"

	"ai-office-backend/internal/core/domain"
)

// SettingsRepository — doc เดียว · Get ไม่พบ = domain.ErrNotFound (ใช้ค่าเริ่มต้นแทน)
type SettingsRepository interface {
	Get(ctx context.Context) (domain.Settings, error)
	Save(ctx context.Context, s domain.Settings) error
}

// UsageRepository — ตัวนับ token รายเดือน + สรุปรายวัน (บวกด้วย $inc)
type UsageRepository interface {
	AddPeriod(ctx context.Context, officeID, serviceID, period string, d domain.RollupDelta, at time.Time) error
	AddRollup(ctx context.Context, officeID, serviceID, date string, d domain.RollupDelta) error
	ListPeriods(ctx context.Context, period string) ([]domain.UsagePeriod, error)
	// ListRollups — office/service ว่าง = ทุกตัว · from/to = YYYY-MM-DD (รวมทั้งสองวัน) · เรียงวันเก่า→ใหม่
	ListRollups(ctx context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error)
}

// DeletionRepository — deletion_requests (เก็บถาวร)
type DeletionRepository interface {
	Create(ctx context.Context, r domain.DeletionRequest) error
	Update(ctx context.Context, r domain.DeletionRequest) error
	List(ctx context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error)
	// FailRunning เปลี่ยนคำขอที่ค้าง running (process ตายกลางทาง) เป็น failed · คืนจำนวนที่เปลี่ยน
	FailRunning(ctx context.Context, reason string, at time.Time) (int64, error)
}

// AccessLogFilter — ค่าว่าง = ไม่กรอง
type AccessLogFilter struct {
	OfficeID, ServiceID, Operator string
	Limit, Offset                 int
}
