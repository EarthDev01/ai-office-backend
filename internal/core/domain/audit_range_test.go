package domain

import (
	"testing"
	"time"
)

func TestAuditFilterApplyRange(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	var f AuditFilter
	if err := f.ApplyRange(now); err != nil {
		t.Fatal(err)
	}
	if !f.From.Equal(now.Add(-AuditDefaultRange)) || !f.To.IsZero() {
		t.Fatalf("default: from=%v to=%v", f.From, f.To)
	}

	to := now.AddDate(0, -3, 0)
	f = AuditFilter{To: to}
	if err := f.ApplyRange(now); err != nil || !f.From.Equal(to.Add(-AuditDefaultRange)) {
		t.Fatalf("default from ต้องนับจาก to: from=%v err=%v", f.From, err)
	}

	f = AuditFilter{From: now.Add(-AuditMaxRange)}
	if err := f.ApplyRange(now); err != nil {
		t.Fatalf("กว้างพอดี max ต้องผ่าน: %v", err)
	}

	f = AuditFilter{From: now.Add(-AuditMaxRange - time.Second)}
	if err := f.ApplyRange(now); err != ErrAuditRangeTooWide {
		t.Fatalf("เกิน max: got %v", err)
	}

	f = AuditFilter{From: now, To: now.Add(-time.Hour)}
	if err := f.ApplyRange(now); err != ErrAuditRangeInvalid {
		t.Fatalf("from หลัง to: got %v", err)
	}
}

func TestAuditFilterNormalizeActor(t *testing.T) {
	f := AuditFilter{Actor: "  Earth "}
	f.Normalize()
	if f.Actor != "earth" {
		t.Fatalf("got %q", f.Actor)
	}
}
