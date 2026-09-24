package filestore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

// ประวัติต้องอยู่รอดหลัง restart และบรรทัดที่พัง (ไฟดับกลางคัน) ต้องไม่ทำให้เปิดไม่ขึ้น
func TestAuditRepo_PersistsAcrossReopenAndSkipsCorruptLine(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "audit_logs.jsonl")

	r, err := NewAuditRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	for i, action := range []string{domain.AuditAuthLogin, domain.AuditOfficeUpdate} {
		e := domain.AuditEntry{ID: domain.NewAuditID(), At: base.Add(time.Duration(i) * time.Minute),
			Actor: "earth", Action: action, Category: domain.AuditCategory(action), Status: domain.AuditSuccess,
			Changes: []domain.FieldChange{{Field: "label", Before: "a", After: "b"}}}
		if err := r.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString(`{"id":"broken","at":` + "\n")
	_ = f.Close()

	r2, err := NewAuditRepository(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	items, total, err := r2.Query(ctx, domain.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || items[0].Action != domain.AuditOfficeUpdate {
		t.Fatalf("want 2 items newest first, got total=%d %+v", total, items)
	}
	if len(items[0].Changes) != 1 || items[0].Changes[0].After != "b" {
		t.Fatalf("changes lost: %+v", items[0].Changes)
	}

	window, n, _ := r2.Query(ctx, domain.AuditFilter{From: base, To: base.Add(time.Minute)})
	if n != 1 || window[0].Action != domain.AuditAuthLogin {
		t.Fatalf("time window [from,to): %+v", window)
	}
}
