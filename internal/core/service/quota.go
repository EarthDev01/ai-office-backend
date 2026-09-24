package service

import (
	"context"
	"fmt"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// QuotaService = เพดานต่อ service ต่อเดือน (tokens) · เตือน 80/95% · 100% หยุดเฉพาะเว็บนั้น (AC-25)
//
// ตัวนับสดอยู่ Redis (P-14 · หลาย replica เห็นตรงกัน) · บันทึกถาวรอยู่ quota_periods
type QuotaService struct {
	counter  port.QuotaCounter
	repo     port.QuotaRepository
	settings *SettingsService
	notifier port.Notifier
	now      port.Clock
}

func NewQuotaService(counter port.QuotaCounter, repo port.QuotaRepository, settings *SettingsService, notifier port.Notifier, now port.Clock) *QuotaService {
	if now == nil {
		now = time.Now
	}
	return &QuotaService{counter: counter, repo: repo, settings: settings, notifier: notifier, now: now}
}

const quotaKeyTTL = 40 * 24 * time.Hour

func quotaKey(officeID, serviceID, period string) string {
	return "ai:quota:" + officeID + ":" + serviceID + ":" + period
}

// used อ่านตัวนับสด · Redis ยังไม่มี key (รอบใหม่/Redis เพิ่งเริ่ม) → เติมจากบันทึกถาวร
func (q *QuotaService) used(ctx context.Context, officeID, serviceID, period string) (int64, int64, error) {
	key := quotaKey(officeID, serviceID, period)
	tokens, questions, found, err := q.counter.Get(ctx, key)
	if err != nil {
		return 0, 0, err
	}
	if found {
		return tokens, questions, nil
	}
	var p domain.QuotaPeriod
	if q.repo != nil {
		p, err = q.repo.Get(ctx, officeID, serviceID, period)
		if err != nil && err != domain.ErrNotFound {
			return 0, 0, err
		}
	}
	if err := q.counter.Init(ctx, key, p.UsedTokens, p.UsedQuestions, quotaKeyTTL); err != nil {
		return 0, 0, err
	}
	return p.UsedTokens, p.UsedQuestions, nil
}

func (q *QuotaService) Status(ctx context.Context, o domain.Office, svc domain.Service) (domain.QuotaStatus, error) {
	st, _ := q.settings.Get(ctx)
	period := domain.PeriodOf(q.now())
	limit := svc.Quota.EffectiveLimit(period, st.DefaultMonthlyLimit)
	tokens, questions, err := q.used(ctx, o.ID, svc.ID, period)
	if err != nil {
		return domain.QuotaStatus{}, err
	}
	out := domain.QuotaStatus{
		OfficeID: o.ID, ServiceID: svc.ID, ServiceLabel: svc.Label, Period: period,
		Limit: limit, UsedTokens: tokens, UsedQuestions: questions,
		Currency: st.Pricing.Currency, Cut: tokens >= limit,
		TempIncreases: []domain.TempIncrease{},
	}
	if limit > 0 {
		out.Percent = float64(tokens) * 100 / float64(limit)
	}
	for _, t := range svc.Quota.TempIncreases {
		if t.Period == period {
			out.TempIncreases = append(out.TempIncreases, t)
		}
	}
	if q.repo != nil {
		if p, err := q.repo.Get(ctx, o.ID, svc.ID, period); err == nil {
			out.CostAmount = p.CostAmount
		}
	}
	return out, nil
}

// Allow = ก่อนเริ่มคำถาม · เต็มแล้ว → ErrQuotaExceeded (เฉพาะ service นี้)
func (q *QuotaService) Allow(ctx context.Context, o domain.Office, svc domain.Service) error {
	st, err := q.Status(ctx, o, svc)
	if err != nil {
		// ตัวนับล่ม: ปฏิเสธดีกว่าปล่อยค่าใช้จ่ายบานโดยไม่มีตัวนับ
		return fmt.Errorf("quota counter: %w", err)
	}
	if st.Cut {
		return domain.ErrQuotaExceeded
	}
	return nil
}

// Record หักโควตาหลังตอบเสร็จ + แจ้งเตือนครั้งเดียวต่อ threshold ต่อรอบ
func (q *QuotaService) Record(ctx context.Context, o domain.Office, svc domain.Service, u domain.Usage, cost float64) {
	period := domain.PeriodOf(q.now())
	if _, _, err := q.used(ctx, o.ID, svc.ID, period); err != nil {
		return
	}
	tokens, _, err := q.counter.Add(ctx, quotaKey(o.ID, svc.ID, period), u.Total(), 1, quotaKeyTTL)
	if err != nil {
		return
	}
	if q.repo != nil {
		_, _ = q.repo.AddUsage(ctx, o.ID, svc.ID, period, u.Total(), 1, cost)
	}
	st, _ := q.settings.Get(ctx)
	limit := svc.Quota.EffectiveLimit(period, st.DefaultMonthlyLimit)
	if limit <= 0 || q.repo == nil {
		return
	}
	pct := float64(tokens) * 100 / float64(limit)
	for _, th := range []struct {
		flag string
		at   float64
		text string
	}{
		{"alerted_80", 80, "ใช้ไปแล้ว 80%"},
		{"alerted_95", 95, "ใช้ไปแล้ว 95%"},
		{"cut", 100, "ครบ 100% — หยุดตอบเฉพาะเว็บนี้แล้ว"},
	} {
		if pct < th.at {
			continue
		}
		first, err := q.repo.MarkAlert(ctx, o.ID, svc.ID, period, th.flag)
		if err != nil || !first {
			continue
		}
		if q.notifier != nil {
			msg := fmt.Sprintf("[AI Office] โควตา %s/%s รอบ %s %s (%d/%d tokens)", o.ID, svc.ID, period, th.text, tokens, limit)
			_ = q.notifier.Notify(ctx, st.TelegramRooms.Alerts, msg)
		}
	}
}
