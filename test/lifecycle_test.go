package test

import (
	"net/http"
	"strings"
	"testing"

	"ai-office-backend/internal/core/domain"
)

// ลบ office แล้ว โดเมนของเขาต้องใช้ widget ไม่ได้ทันที (snippet ที่แปะค้างไว้หยุดทำงาน)
func TestDeleteOffice_OriginStopsWorkingImmediately(t *testing.T) {
	r := newRouter(t)

	if code, _, _ := getBoot(t, r, "K11S", adminJWT("adm_ploy", "K11S")); code != http.StatusOK {
		t.Fatalf("ก่อนลบต้องใช้ได้ ได้ %d", code)
	}

	w := do(t, r, req{method: http.MethodDelete, path: "/api/ai/admin/offices/demo", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("ลบไม่สำเร็จ %d %s", w.Code, w.Body.String())
	}

	if code, _, raw := getBoot(t, r, "K11S", adminJWT("adm_ploy", "K11S")); code != http.StatusForbidden || !strings.Contains(raw, "ORIGIN_NOT_REGISTERED") {
		t.Fatalf("หลังลบ โดเมนเดิมต้องใช้ไม่ได้ ได้ %d %s", code, raw)
	}
}

// widget bundle เป็นไฟล์เดียวกันทุกเจ้า — ทั้ง path ใหม่ (ไม่มี key) และแบบเก่าที่มี key
func TestWidgetBundle_SameFileForEveryone(t *testing.T) {
	r := newRouter(t)
	// test router ชี้ bundle ไปไฟล์ที่ไม่มีอยู่ → ได้ 404 ข้อความ "ยังไม่ได้ build" ทั้งสอง path
	// สิ่งที่เช็คคือ route มีอยู่จริงและไม่ตัดสินจาก key
	for _, path := range []string{"/widget/v1/ai-office.js", "/widget/v1/pk_whatever/ai-office.js"} {
		w := do(t, r, req{method: http.MethodGet, path: path})
		if !strings.Contains(w.Body.String(), "build widget") {
			t.Fatalf("%s: ต้องไปถึงตัวเสิร์ฟไฟล์ ได้ %d %s", path, w.Code, w.Body.String())
		}
	}
}

// snippet รุ่นก่อน (มี key ใน path) ยังใช้ได้ — key ไม่มีผลแล้ว หา office จากโดเมนเหมือนกัน
func TestLegacyBootstrapPath_IgnoresKey(t *testing.T) {
	r := newRouter(t)
	for _, key := range []string{demoKey, "pk_key_ที่ไม่มีจริง"} {
		w := do(t, r, req{method: http.MethodGet, path: legacyBootPath(key, "K11S"), origin: officeOrigin, token: adminJWT("adm_ploy", "K11S")})
		if w.Code != http.StatusOK || !payload[domain.Bootstrap](t, w).Enabled {
			t.Fatalf("key %q: path เก่าต้องยังใช้ได้ ได้ %d %s", key, w.Code, w.Body.String())
		}
	}
}

// CORS เปิดทุก origin (เหมือน office-api-v10) — origin ไหนก็ได้ ACAO = "*"
func TestCORS_AllowsAnyOrigin(t *testing.T) {
	r := newRouter(t)

	for _, origin := range []string{"http://acme.test", "http://evil.example", "http://localhost:5173"} {
		w := do(t, r, req{method: http.MethodGet, path: "/healthz", origin: origin})
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("origin %q ต้องได้ ACAO=* ได้ %q", origin, got)
		}
	}

	// preflight ต้องตอบ 204 พร้อม header ครบ
	pre := do(t, r, req{method: http.MethodOptions, path: "/api/ai/admin/offices", origin: "http://acme.test"})
	if pre.Code != http.StatusNoContent {
		t.Fatalf("preflight ต้องได้ 204 ได้ %d", pre.Code)
	}
	if pre.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight ต้องได้ ACAO=* ได้ %q", pre.Header().Get("Access-Control-Allow-Origin"))
	}
}

// หัวใจของการเปลี่ยนรอบนี้: office-v10x ใช้ snippet เดียวกันทุกโดเมน
// 2 โดเมน → 2 office คนละเจ้า ได้ config คนละชุด แม้รหัส service จะชนกัน
func TestOneSnippetManyOrigins_ResolveDifferentOffices(t *testing.T) {
	acme := domain.NewOffice("acme", "ACME")
	acme.AllowedOrigins = []string{"http://acme.test"}
	svc := domain.DefaultService("K11S", "ชื่อชนกันแต่คนละเจ้า")
	svc.Enabled = true
	svc.DisplayName = "ผู้ช่วย ACME"
	acme.Services = []domain.Service{svc}

	r := newRouter(t, demoOffice(), acme)
	tok := adminJWT("adm_ploy", "K11S")

	boot := func(origin string) domain.Bootstrap {
		w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: origin, token: tok})
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", origin, w.Code, w.Body.String())
		}
		return payload[domain.Bootstrap](t, w)
	}
	d, a := boot(officeOrigin), boot("http://acme.test")
	if d.OfficeID != "demo" || a.OfficeID != "acme" {
		t.Fatalf("ต้องได้คนละ office: demo=%+v acme=%+v", d, a)
	}
	if a.DisplayName != "ผู้ช่วย ACME" || d.DisplayName == a.DisplayName {
		t.Fatalf("config ต้องมาจาก office ของโดเมนนั้น: demo=%q acme=%q", d.DisplayName, a.DisplayName)
	}
}

// 2 office แยกกันเด็ดขาด — สิทธิ์ service ยังถูกตรวจในแต่ละเจ้า
func TestTwoOffices_AreIsolated(t *testing.T) {
	second := domain.NewOffice("acme", "ACME")
	second.AllowedOrigins = []string{"http://acme.test"}
	svc := domain.DefaultService("K11S", "ชื่อชนกันแต่คนละเจ้า")
	svc.Enabled = true
	second.Services = []domain.Service{svc}

	r := newRouter(t, demoOffice(), second)

	w := do(t, r, req{
		method: http.MethodGet, path: bootPath("K11S"),
		origin: "http://acme.test", token: adminJWT("adm_ploy", "SOME_OTHER"),
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("ไม่มี K11S ใน ListService ต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
}
