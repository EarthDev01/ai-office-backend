package test

import (
	"net/http"
	"testing"

	"ai-office-backend/internal/core/domain"
)

// ลบ office แล้ว snippet ที่ยังแปะค้างอยู่ต้องหยุดทำงานทันที (รวมตั๋วที่ออกไปแล้ว)
func TestDeleteOffice_KeyStopsWorkingImmediately(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	if code, _, _ := getBoot(t, r, demoKey, "K11S", tk); code != http.StatusOK {
		t.Fatalf("ก่อนลบต้องใช้ได้ ได้ %d", code)
	}

	w := do(t, r, req{method: http.MethodDelete, path: "/api/ai/admin/offices/demo", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("ลบไม่สำเร็จ %d %s", w.Code, w.Body.String())
	}

	if code, _, _ := getBoot(t, r, demoKey, "K11S", tk); code != http.StatusNotFound {
		t.Fatalf("หลังลบ key เดิมต้องใช้ไม่ได้ ได้ %d", code)
	}
	if w := sessionReq(t, r, demoKey, "K11S", secretK11S, adminUser(), testKind); w.Code != http.StatusNotFound {
		t.Fatalf("หลังลบ ขอตั๋วต้องไม่ได้ ได้ %d", w.Code)
	}
	if w := do(t, r, req{method: http.MethodGet, path: "/widget/v1/" + demoKey + "/ai-office.js"}); w.Code != http.StatusNotFound {
		t.Fatalf("bundle ของ key ที่ถูกลบต้อง 404 ได้ %d", w.Code)
	}
}

// rotate key = snippet เดิมพังทันที (คอนโซลต้องถามยืนยันก่อน)
func TestRotateKey_OldKeyDies(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())

	w := do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/rotate-key", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("rotate ไม่สำเร็จ %d", w.Code)
	}
	o := payload[domain.Office](t, w)
	if o.PublicKey == demoKey {
		t.Fatal("key ต้องเปลี่ยน")
	}
	if code, _, _ := getBoot(t, r, demoKey, "K11S", tk); code != http.StatusNotFound {
		t.Fatalf("key เก่าต้องใช้ไม่ได้แล้ว ได้ %d", code)
	}
	// ตั๋วเดิมผูก office ไม่ใช่ key — ใช้กับ key ใหม่ได้จนหมดอายุ
	if code, b, _ := getBoot(t, r, o.PublicKey, "K11S", tk); code != http.StatusOK || !b.Enabled {
		t.Fatalf("key ใหม่ต้องใช้ได้ ได้ %d %+v", code, b)
	}
}

// CORS เปิดทุก origin (ไม่ใช้ cookie) — ด่านจริงคือ origin ใน page-config/TicketAuth
func TestCORS_AllowsAnyOrigin(t *testing.T) {
	r := newRouter(t)
	for _, origin := range []string{"http://acme.test", "http://evil.example", "http://localhost:5173"} {
		w := do(t, r, req{method: http.MethodGet, path: "/healthz", origin: origin})
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("origin %q ต้องได้ ACAO=* ได้ %q", origin, got)
		}
	}
	pre := do(t, r, req{method: http.MethodOptions, path: "/api/ai/admin/offices", origin: "http://acme.test"})
	if pre.Code != http.StatusNoContent || pre.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight ต้องได้ 204 + ACAO=* ได้ %d %q", pre.Code, pre.Header().Get("Access-Control-Allow-Origin"))
	}
}

// 2 office แยกกันเด็ดขาด — ตั๋วของเจ้าหนึ่งใช้กับ key ของอีกเจ้าไม่ได้ แม้ service id ชื่อเหมือนกัน
func TestTwoOffices_AreIsolated(t *testing.T) {
	second := domain.NewOffice("acme", "ACME")
	second.PublicKey = "pk_acme"
	second.Kind = testKind
	second.AllowedOrigins = []string{officeOrigin}
	svc := domain.DefaultService("K11S", "ชื่อชนกันแต่คนละเจ้า")
	svc.Enabled = true
	svc.SecretKeyHash = domain.HashSecret("aisk_acme_secret_000000000000000000000")
	second.Services = []domain.Service{svc}

	r := newRouter(t, demoOffice(), second)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	w := do(t, r, req{method: http.MethodGet, path: bootPath("pk_acme", "K11S"), origin: officeOrigin, token: tk})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ตั๋วของ office อื่นต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
	// secret ของ demo ใช้ขอตั๋วของ acme ไม่ได้
	if w := sessionReq(t, r, "pk_acme", "K11S", secretK11S, adminUser(), testKind); w.Code != http.StatusForbidden {
		t.Fatalf("secret ของ office อื่นต้อง 403 ได้ %d", w.Code)
	}
}
