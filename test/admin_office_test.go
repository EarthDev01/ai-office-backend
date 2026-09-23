package test

import (
	"net/http"
	"testing"

	"ai-office-backend/internal/core/domain"
)

func admin(t *testing.T, r http.Handler, method, path, body string) (int, *domain.Office) {
	t.Helper()
	w := do(t, r, req{method: method, path: path, body: body, console: consoleToken})
	o := payload[domain.Office](t, w)
	return w.Code, &o
}

func TestConsole_RequiresConsoleToken(t *testing.T) {
	r := newRouter(t)
	for _, tok := range []string{"", "session-ของแอดมินเว็บ"} {
		w := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/offices", console: tok})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("token %q ต้อง 401 ได้ %d", tok, w.Code)
		}
	}
	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/offices", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("token ถูกต้องได้ 200 ได้ %d", w.Code)
	}
}

func TestOffice_CreateGeneratesUniqueKey(t *testing.T) {
	r := newRouter(t)

	code, a := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"id":"acme","label":"ACME"}`)
	if code != http.StatusCreated {
		t.Fatalf("อยากได้ 201 ได้ %d", code)
	}
	_, b := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"id":"beta","label":"BETA"}`)

	if a.PublicKey == "" || a.PublicKey == b.PublicKey {
		t.Fatalf("key ต้องมีและต้องไม่ซ้ำกัน: %q / %q", a.PublicKey, b.PublicKey)
	}
	if a.Enabled != true || len(a.Services) != 0 {
		t.Fatalf("office ใหม่ควรเปิดอยู่และยังไม่มี service: %+v", a)
	}
}

func TestOffice_DuplicateIDRejected(t *testing.T) {
	r := newRouter(t)
	if code, _ := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"id":"demo"}`); code != http.StatusConflict {
		t.Fatalf("id ซ้ำต้อง 409 ได้ %d", code)
	}
}

func TestOffice_AddAndRemoveService(t *testing.T) {
	r := newRouter(t)

	code, o := admin(t, r, http.MethodPost, "/api/ai/admin/offices/demo/services", `{"id":"NEW1","label":"เว็บใหม่"}`)
	if code != http.StatusCreated || len(o.Services) != 3 {
		t.Fatalf("เพิ่ม service ไม่สำเร็จ: %d %+v", code, o.Services)
	}
	if _, ok := o.FindService("NEW1"); !ok {
		t.Fatal("ไม่เจอ service ที่เพิ่ง เพิ่ม")
	}

	code, o = admin(t, r, http.MethodDelete, "/api/ai/admin/offices/demo/services/NEW1", "")
	if code != http.StatusOK || len(o.Services) != 2 {
		t.Fatalf("ลบ service ไม่สำเร็จ: %d %+v", code, o.Services)
	}
	if _, ok := o.FindService("NEW1"); ok {
		t.Fatal("service ควรถูกลบแล้ว")
	}
	// ลบ service หนึ่งต้องไม่กระทบอีกอัน
	if _, ok := o.FindService("K11S"); !ok {
		t.Fatal("K11S หายไปด้วย — การลบไม่ควรกระทบ service อื่น")
	}
}

func TestOffice_DuplicateServiceRejected(t *testing.T) {
	r := newRouter(t)
	if code, _ := admin(t, r, http.MethodPost, "/api/ai/admin/offices/demo/services", `{"id":"K11S"}`); code != http.StatusConflict {
		t.Fatalf("service id ซ้ำต้อง 409 ได้ %d", code)
	}
}

func TestService_PatchAppliesOnlySentFields(t *testing.T) {
	r := newRouter(t)
	code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S",
		`{"display_name":"ผู้ช่วยเว็บ K11S"}`)
	if code != http.StatusOK {
		t.Fatalf("อยากได้ 200 ได้ %d", code)
	}
	svc, _ := o.FindService("K11S")
	if svc.DisplayName != "ผู้ช่วยเว็บ K11S" {
		t.Fatalf("display_name ไม่ถูกแก้: %+v", svc)
	}
	if !svc.Enabled || len(svc.Allowlist) != 1 {
		t.Fatalf("field ที่ไม่ได้ส่งมาต้องไม่ถูกแตะ: %+v", svc)
	}
}

// false กับ "ไม่ได้ส่งมา" ต้องแยกออกจากกัน ไม่งั้นปิดสวิตช์ไม่ได้
func TestService_CanSetFalse(t *testing.T) {
	r := newRouter(t)
	_, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S", `{"enabled":false}`)
	svc, _ := o.FindService("K11S")
	if svc.Enabled {
		t.Fatal("สวิตช์ปิดไม่ได้")
	}
}

func TestOffice_RejectsBadValues(t *testing.T) {
	r := newRouter(t)
	cases := map[string]string{
		"theme มั่ว":          `{"theme":"neon"}`,
		"position มั่ว":       `{"placement":{"position":"top-right","offset_x":12,"offset_y":12}}`,
		"offset เกิน":         `{"placement":{"position":"bottom-right","offset_x":9999,"offset_y":12}}`,
		"origin ไม่มี scheme": `{"allowed_origins":["office.test"]}`,
		"origin มี path":      `{"allowed_origins":["http://office.test/app"]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", body); code != http.StatusBadRequest {
				t.Fatalf("ควรถูกปฏิเสธ ได้ %d", code)
			}
		})
	}
}

func TestOffice_RecordsWhoChanged(t *testing.T) {
	r := newRouter(t)
	_, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"label":"ชื่อใหม่"}`)
	if o.UpdatedBy == "" || o.UpdatedAt.IsZero() {
		t.Fatalf("ทุกการแก้ต้องมีร่องรอย: %+v", o)
	}
}
