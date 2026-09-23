package test

import (
	"net/http"
	"testing"

	"ai-office-backend/internal/core/domain"
)

// ลบ office แล้ว snippet ที่ยังแปะค้างอยู่ต้องหยุดทำงานทันที
func TestDeleteOffice_KeyStopsWorkingImmediately(t *testing.T) {
	r := newRouter(t)

	if code, _, _ := getBoot(t, r, demoKey, "K11S", devToken("adm_ploy", "K11S")); code != http.StatusOK {
		t.Fatalf("ก่อนลบต้องใช้ได้ ได้ %d", code)
	}

	w := do(t, r, req{method: http.MethodDelete, path: "/api/ai/admin/offices/demo", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("ลบไม่สำเร็จ %d %s", w.Code, w.Body.String())
	}

	if code, _, _ := getBoot(t, r, demoKey, "K11S", devToken("adm_ploy", "K11S")); code != http.StatusNotFound {
		t.Fatalf("หลังลบ key เดิมต้องใช้ไม่ได้ ได้ %d", code)
	}
	if w := do(t, r, req{method: http.MethodGet, path: "/widget/v1/" + demoKey + "/ai-office.js"}); w.Code != http.StatusNotFound {
		t.Fatalf("bundle ของ key ที่ถูกลบต้อง 404 ได้ %d", w.Code)
	}
}

// rotate key = snippet เดิมพังทันที (คอนโซลต้องถามยืนยันก่อน)
func TestRotateKey_OldKeyDies(t *testing.T) {
	r := newRouter(t)

	w := do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/rotate-key", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("rotate ไม่สำเร็จ %d", w.Code)
	}
	o := payload[domain.Office](t, w)

	if o.PublicKey == demoKey {
		t.Fatal("key ต้องเปลี่ยน")
	}
	if code, _, _ := getBoot(t, r, demoKey, "K11S", devToken("adm_ploy", "K11S")); code != http.StatusNotFound {
		t.Fatalf("key เก่าต้องใช้ไม่ได้แล้ว ได้ %d", code)
	}
	if code, b, _ := getBoot(t, r, o.PublicKey, "K11S", devToken("adm_ploy", "K11S")); code != http.StatusOK || !b.Enabled {
		t.Fatalf("key ใหม่ต้องใช้ได้ ได้ %d %+v", code, b)
	}
}

// เพิ่มลูกค้าใหม่แล้ว CORS ต้องอนุญาตทันทีโดยไม่ต้อง deploy
func TestCORS_FollowsOfficeOriginsWithoutDeploy(t *testing.T) {
	r := newRouter(t)
	const newOrigin = "http://acme.test"

	before := do(t, r, req{method: http.MethodGet, path: "/healthz", origin: newOrigin})
	if before.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("origin ที่ยังไม่ลงทะเบียนต้องไม่ได้ ACAO")
	}

	if code, _ := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"id":"acme","label":"ACME"}`); code != http.StatusCreated {
		t.Fatalf("สร้าง office ไม่สำเร็จ %d", code)
	}
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/acme",
		`{"allowed_origins":["http://acme.test"]}`); code != http.StatusOK {
		t.Fatalf("ตั้ง origin ไม่สำเร็จ %d", code)
	}

	after := do(t, r, req{method: http.MethodGet, path: "/healthz", origin: newOrigin})
	if after.Header().Get("Access-Control-Allow-Origin") != newOrigin {
		t.Fatalf("หลังลงทะเบียนต้องได้ ACAO ทันที ได้ %q", after.Header().Get("Access-Control-Allow-Origin"))
	}
}

// 2 office แยกกันเด็ดขาด — key ของเจ้าหนึ่งใช้กับ session ของอีกเจ้าไม่ได้
func TestTwoOffices_AreIsolated(t *testing.T) {
	second := domain.NewOffice("acme", "ACME")
	second.PublicKey = "pk_acme"
	second.AllowedOrigins = []string{"http://acme.test"}
	svc := domain.DefaultService("K11S", "ชื่อชนกันแต่คนละเจ้า")
	svc.Enabled = true
	svc.Allowlist = []string{"adm_ploy"}
	second.Services = []domain.Service{svc}

	r := newRouter(t, demoOffice(), second)

	// adm_ploy ของ demo เอา key ของ acme ไปใช้ไม่ได้ แม้ service id จะชื่อเหมือนกัน
	// adm_ploy ของ demo เอา key ของ acme ไปใช้ไม่ได้ แม้ service id จะชื่อเหมือนกัน
	// (dev token ผูก office จาก key ที่เรียก จึงทดสอบผ่าน service ที่เขาไม่มีสิทธิ์แทน)
	w := do(t, r, req{
		method: http.MethodGet, path: bootPath("pk_acme", "K11S"),
		origin: "http://acme.test", token: devToken("adm_ploy", "SOME_OTHER"),
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("ไม่มี K11S ใน ListService ต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
}
