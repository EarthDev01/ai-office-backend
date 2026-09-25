package test

import (
	"net/http"
	"strings"
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

func TestOffice_CreateDefaults(t *testing.T) {
	r := newRouter(t)

	code, a := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"group_id":"g-test","id":"acme","label":"ACME"}`)
	if code != http.StatusCreated {
		t.Fatalf("อยากได้ 201 ได้ %d", code)
	}
	_, b := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"group_id":"g-test","id":"beta","label":"BETA"}`)

	// public_key เลิกใช้ระบุ office แล้ว แต่ยังต้องสุ่มไม่ซ้ำ (Mongo มี unique index เดิมอยู่)
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

// 1 โดเมนอยู่ได้แค่ office เดียว — ไม่งั้นแยกไม่ได้ว่าเป็นลูกค้าเจ้าไหน
func TestOffice_OriginMustBeUniqueAcrossOffices(t *testing.T) {
	r := newRouter(t)
	if code, _ := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"group_id":"g-test","id":"acme"}`); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	// เขียนต่างรูปแบบ (ตัวพิมพ์ใหญ่ + / ท้าย) ก็ยังนับว่าซ้ำ
	w := do(t, r, req{method: http.MethodPatch, path: "/api/ai/admin/offices/acme", console: consoleToken,
		body: `{"allowed_origins":["http://OFFICE.test/"]}`})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ORIGIN_TAKEN") {
		t.Fatalf("โดเมนซ้ำต้อง 409 ORIGIN_TAKEN ได้ %d %s", w.Code, w.Body.String())
	}
	owner := payload[struct {
		OfficeID string `json:"office_id"`
	}](t, w)
	if owner.OfficeID != "demo" {
		t.Fatalf("ต้องบอกว่าโดเมนเป็นของ office ไหน: %+v", owner)
	}

	// office เดิมบันทึกโดเมนของตัวเองซ้ำได้ + ถูก normalize และตัดตัวซ้ำ
	code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo",
		`{"allowed_origins":["http://Office.test/","http://office.test"]}`)
	if code != http.StatusOK || len(o.AllowedOrigins) != 1 || o.AllowedOrigins[0] != officeOrigin {
		t.Fatalf("normalize/dedupe ผิด: %d %+v", code, o.AllowedOrigins)
	}
	// 1 domain = 1 URL — URL ที่ต่างกันต้องเป็น domain แยก
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo",
		`{"allowed_origins":["http://office.test","https://new.example"]}`); code != http.StatusBadRequest {
		t.Fatalf("ใส่ 2 URL ต้องได้ 400 ได้ %d", code)
	}
}

func TestAdmin_HostAPIBase_UsedByPageConfig(t *testing.T) {
	r := newRouter(t)
	pageBase := func() string {
		w := do(t, r, req{method: "GET", path: "/api/ai/widget/page-config", origin: officeOrigin})
		return payload[struct {
			HostAPIBase string `json:"host_api_base"`
		}](t, w).HostAPIBase
	}
	if got := pageBase(); got != officeOrigin+"/api" {
		t.Fatalf("ไม่ได้ตั้งค่า ต้องใช้ {origin}/api ได้ %q", got)
	}

	code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"host_api_base":" https://API.dev.example/api/ "}`)
	if code != http.StatusOK || o.HostAPIBase != "https://api.dev.example/api" {
		t.Fatalf("บันทึก = %d %q", code, o.HostAPIBase)
	}
	if got := pageBase(); got != "https://api.dev.example/api" {
		t.Fatalf("page-config ต้องใช้ค่าที่ตั้งไว้ ได้ %q", got)
	}

	// ส่งค่าว่าง = ล้าง กลับไปใช้ {origin}/api
	if code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"host_api_base":""}`); code != http.StatusOK || o.HostAPIBase != "" {
		t.Fatalf("ล้างค่า = %d %q", code, o.HostAPIBase)
	}
	if got := pageBase(); got != officeOrigin+"/api" {
		t.Fatalf("ล้างแล้วต้องกลับไปใช้ {origin}/api ได้ %q", got)
	}
}

func TestAdmin_HostAPIBase_Rejects(t *testing.T) {
	r := newRouter(t)
	for name, body := range map[string]string{
		"ไม่มี scheme": `{"host_api_base":"api.example/api"}`,
		"มี query":     `{"host_api_base":"https://api.example/api?x=1"}`,
		"javascript:":  `{"host_api_base":"javascript:alert(1)"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", body); code != http.StatusBadRequest {
				t.Fatalf("ต้องได้ 400 ได้ %d", code)
			}
		})
	}
}
