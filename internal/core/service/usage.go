package service

import (
	"context"
	"log"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// UsageService เก็บการใช้ token (รายเดือนต่อ service) และสรุปรายวัน — ยังไม่มีลิมิต/ราคา
type UsageService struct {
	repo port.UsageRepository
	now  func() time.Time
}

func NewUsageService(repo port.UsageRepository) *UsageService {
	return &UsageService{repo: repo, now: time.Now}
}

// Record บวกเข้าตัวนับของเดือนและวันของ at (เวลาไทย) · บันทึกไม่ได้แค่ log — ห้ามทำให้คำตอบล้ม
func (s *UsageService) Record(ctx context.Context, officeID, serviceID string, at time.Time, d domain.RollupDelta) {
	if s == nil {
		return
	}
	if d.Questions != 0 || d.InputTokens != 0 || d.OutputTokens != 0 || d.CacheRead != 0 || d.CacheWrite != 0 {
		if err := s.repo.AddPeriod(ctx, officeID, serviceID, domain.PeriodOf(at), d, s.now()); err != nil {
			log.Printf("[WARN] บันทึกการใช้ token รายเดือนไม่สำเร็จ (office=%s service=%s): %v", officeID, serviceID, err)
		}
	}
	if err := s.repo.AddRollup(ctx, officeID, serviceID, domain.DateOf(at), d); err != nil {
		log.Printf("[WARN] บันทึกสรุปรายวันไม่สำเร็จ (office=%s service=%s): %v", officeID, serviceID, err)
	}
}

// Periods คืนการใช้ของทุก service ในเดือน period (YYYY-MM · ว่าง = เดือนนี้)
func (s *UsageService) Periods(ctx context.Context, period string) (string, []domain.UsagePeriod, error) {
	if period == "" {
		period = domain.PeriodOf(s.now())
	}
	list, err := s.repo.ListPeriods(ctx, period)
	return period, list, err
}

// Rollups — from/to ว่าง = 30 วันล่าสุด (เวลาไทย)
func (s *UsageService) Rollups(ctx context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error) {
	if to == "" {
		to = domain.DateOf(s.now())
	}
	if from == "" {
		t, err := time.ParseInLocation("2006-01-02", to, domain.Bangkok())
		if err != nil {
			return nil, err
		}
		from = domain.DateOf(t.AddDate(0, 0, -29))
	}
	return s.repo.ListRollups(ctx, officeID, serviceID, from, to)
}
