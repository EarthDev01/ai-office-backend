package service

import (
	"context"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

type countingAuditRepo struct{ total int }

func (countingAuditRepo) Append(context.Context, domain.AuditEntry) error { return nil }
func (r countingAuditRepo) Query(context.Context, domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	return []domain.AuditEntry{}, r.total, nil
}
func (countingAuditRepo) Actors(context.Context) ([]string, error) { return nil, nil }

func TestAuditQuery_CapsTotal(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		repoTotal, want int
		capped          bool
	}{
		{domain.AuditCountCap, domain.AuditCountCap, false},
		{domain.AuditCountCap + 1, domain.AuditCountCap, true},
	} {
		svc := NewAuditService(countingAuditRepo{total: tc.repoTotal}, time.Now)
		page, err := svc.Query(ctx, domain.AuditFilter{})
		if err != nil || page.Total != tc.want || page.TotalCapped != tc.capped {
			t.Fatalf("repo=%d: total=%d capped=%v err=%v", tc.repoTotal, page.Total, page.TotalCapped, err)
		}
	}
}
