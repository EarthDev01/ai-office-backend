package test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

// หลังบ้านหลายชนิดบน backend เดียว — office เลือกชนิดด้วย kind (connectors/<kind>/)

const abatechOrigin = "http://127.0.0.1:8080"

// abatechOffice: office ชนิด office-abatech 1 เว็บ (DEMOSLOT เปิดอยู่)
func abatechOffice() domain.Office {
	o := domain.NewOffice("aba", "หลังบ้าน abatech")
	o.PublicKey = "pk_aba_test"
	o.Kind = "office-abatech"
	o.AllowedOrigins = []string{abatechOrigin}
	o.HostAPIBase = "http://localhost:7777/api"
	s := domain.DefaultService("DEMOSLOT", "เว็บ DEMOSLOT")
	s.Enabled = true
	o.Services = []domain.Service{s}
	return o
}

// abatechJWT = headertoken ของ GOTOPOPOFFICE (claim result = employee.User · role.permission ถูกล้าง)
func abatechJWT(level int, dept string, services ...string) string {
	svc := make([]map[string]any, 0, len(services))
	for _, s := range services {
		svc = append(svc, map[string]any{"service": s, "group_type": "TOPUP", "group_name": "TOPUP"})
	}
	b, _ := json.Marshal(map[string]any{
		"exp": time.Now().Add(time.Hour).Unix(),
		"result": map[string]any{
			"id": "64b0c0ffee0000000000abcd", "name": "admin01", "full_name": "แอดมิน หนึ่ง",
			"deptcode": dept, "level": level, "service": svc,
			"role": map[string]any{"name": "Admin", "permission": nil},
		},
	})
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	return head + "." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

func abaBoot(t *testing.T, r http.Handler, serviceID, token string) (int, domain.Bootstrap, string) {
	t.Helper()
	w := do(t, r, req{method: http.MethodGet, path: bootPath(serviceID), origin: abatechOrigin, token: token})
	return w.Code, payload[domain.Bootstrap](t, w), w.Body.String()
}

func TestKind_PageConfigFollowsOfficeKind(t *testing.T) {
	r := newRouter(t, demoOffice(), abatechOffice())

	type pc struct {
		Kind        string `json:"kind"`
		HostAPIBase string `json:"host_api_base"`
		Token       struct {
			Source, Key, Format string
		} `json:"token"`
		Service struct {
			Source, Key, Encoding string
		} `json:"service"`
		AuthScheme string         `json:"auth_scheme"`
		Identity   map[string]any `json:"identity"`
	}
	get := func(origin string) pc {
		w := do(t, r, req{method: http.MethodGet, path: "/api/ai/widget/page-config", origin: origin})
		if w.Code != http.StatusOK {
			t.Fatalf("page-config %s: %d %s", origin, w.Code, w.Body.String())
		}
		return payload[pc](t, w)
	}

	// office เก่าที่ไม่มี kind = office-v10x
	v := get(officeOrigin)
	if v.Kind != "office-v10x" || v.Token.Key != "auth_token" || v.Service.Key != "web-service" || v.HostAPIBase != officeOrigin+"/api" {
		t.Fatalf("v10x page-config ผิด: %+v", v)
	}

	a := get(abatechOrigin)
	if a.Kind != "office-abatech" || a.Token.Key != "headertoken" || a.Token.Format != "raw" ||
		a.Service.Source != "query" || a.Service.Encoding != "base64" || a.AuthScheme != "Bearer" {
		t.Fatalf("abatech page-config ผิด: %+v", a)
	}
	if a.HostAPIBase != "http://localhost:7777/api" {
		t.Fatalf("host_api_base ต้องมาจาก office ก่อน template: %q", a.HostAPIBase)
	}
	// กติกาเว็บที่เข้าได้ตัดสินที่ server — ไม่ส่งลงหน้าเว็บ
	for _, k := range []string{"services", "services_empty", "all_services_when"} {
		if _, ok := a.Identity[k]; ok {
			t.Fatalf("page-config ไม่ควรมี identity.%s: %v", k, a.Identity)
		}
	}
}

func TestKind_AbatechBootstrap(t *testing.T) {
	r := newRouter(t, abatechOffice())

	if code, b, raw := abaBoot(t, r, "DEMOSLOT", abatechJWT(5, "D003", "DEMOSLOT")); code != http.StatusOK || !b.Enabled {
		t.Fatalf("เว็บที่อยู่ในรายชื่อต้องเปิด: %d %s", code, raw)
	}
	// level >= 10 เห็นทุกเว็บ แม้รายชื่อว่าง
	if code, b, raw := abaBoot(t, r, "DEMOSLOT", abatechJWT(99, "D001")); code != http.StatusOK || !b.Enabled {
		t.Fatalf("level 99 ต้องเข้าได้: %d %s", code, raw)
	}
	// level ต่ำ + รายชื่อว่าง = เข้าไม่ได้ (ตรงกับเมนูของ office-abatech)
	if code, _, raw := abaBoot(t, r, "DEMOSLOT", abatechJWT(5, "D003")); code != http.StatusForbidden {
		t.Fatalf("รายชื่อว่างต้อง 403: %d %s", code, raw)
	}
	// มีเว็บอื่นในรายชื่อ แต่ไม่มีเว็บนี้
	if code, _, raw := abaBoot(t, r, "DEMOSLOT", abatechJWT(5, "D003", "FULLSLOT999")); code != http.StatusForbidden {
		t.Fatalf("ไม่มีเว็บนี้ในรายชื่อต้อง 403: %d %s", code, raw)
	}
}

// token รูปแบบ v10x ใช้เปิดเว็บของ office abatech ไม่ได้ — อ่านตาม connector ของ office นั้นเท่านั้น
// (JWT v10x ไม่มี result.service → รายชื่อเว็บว่าง + level 0 → เข้าไม่ได้ตามกติกา abatech)
func TestKind_TokenReadByOfficeConnector(t *testing.T) {
	r := newRouter(t, abatechOffice())
	if code, _, raw := abaBoot(t, r, "DEMOSLOT", adminJWT("adm_ploy", "DEMOSLOT")); code != http.StatusForbidden {
		t.Fatalf("JWT ของ v10x ต้องเปิดเว็บ abatech ไม่ได้ (403): %d %s", code, raw)
	}
}

func TestKind_AdminSetAndValidate(t *testing.T) {
	r := newRouter(t)

	code, o := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"group_id":"g-test","id":"aba2","label":"ABA","kind":"office-abatech"}`)
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("สร้าง office พร้อม kind: %d %v", code, o)
	}
	if o.Kind != "office-abatech" {
		t.Fatalf("kind ไม่ถูกบันทึก: %v", o)
	}
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"kind":"office-ไม่มีจริง"}`); code != http.StatusBadRequest {
		t.Fatalf("kind ที่ไม่มี connector ต้อง 400 ได้ %d", code)
	}
	if code, _ := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"group_id":"g-test","id":"bad","kind":"nope"}`); code != http.StatusBadRequest {
		t.Fatalf("สร้างด้วย kind ที่ไม่มีต้อง 400 ได้ %d", code)
	}
	code, o = admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"kind":"office-abatech"}`)
	if code != http.StatusOK || o.Kind != "office-abatech" {
		t.Fatalf("แก้ kind: %d %v", code, o)
	}

	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/kinds", console: consoleToken})
	kinds := payload[[]struct {
		Kind    string `json:"kind"`
		Label   string `json:"label"`
		Default bool   `json:"default"`
	}](t, w)
	found := map[string]bool{}
	for _, k := range kinds {
		found[k.Kind] = true
		if k.Kind == "office-v10x" && !k.Default {
			t.Errorf("office-v10x ต้องเป็นค่าตั้งต้น: %+v", k)
		}
		if k.Label == "" {
			t.Errorf("kind %s ไม่มี label", k.Kind)
		}
	}
	if !found["office-v10x"] || !found["office-abatech"] {
		t.Fatalf("kinds ต้องมีทั้ง 2 ชนิด: %+v", kinds)
	}
}
