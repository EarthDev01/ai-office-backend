package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

// AuditRepository เก็บประวัติการทำงานแบบ append-only — ไม่มี Update/Delete โดยตั้งใจ
type AuditRepository interface {
	Append(ctx context.Context, e domain.AuditEntry) error
	// Query คืนรายการที่ตรงเงื่อนไข เรียงใหม่→เก่า ตามหน้าที่ขอ + จำนวนทั้งหมดที่ตรง
	// (store นับหยุดที่ AuditCountCap+1 ได้ — ผู้เรียกดูแค่ว่าเกิน cap หรือไม่)
	Query(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, int, error)
	// Actors คืนรายชื่อผู้กระทำที่เคยมีในประวัติ (ไว้ทำตัวกรอง) เรียงตามตัวอักษร
	Actors(ctx context.Context) ([]string, error)
}

// AuditRecorder คือสิ่งที่ service อื่นใช้บันทึกเหตุการณ์
//
// Record ไม่คืน error — บันทึกประวัติพลาดต้องไม่ทำให้งานหลักของผู้ใช้ล้ม (log ไว้แทน)
type AuditRecorder interface {
	Record(ctx context.Context, e domain.AuditEntry)
}

// NopAudit ใช้ตอนไม่ต้องการบันทึก (เช่น unit test)
type NopAudit struct{}

func (NopAudit) Record(context.Context, domain.AuditEntry) {}
