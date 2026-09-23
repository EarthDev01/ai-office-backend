package test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

// realOffice: demoOffice ที่ตั้ง BackofficeAPIURL ไว้ (แค่ flag ว่าใช้ JWT จริง
// ไม่ใช่ dev-token mode — backend อ่านตัวตนจาก JWT ไม่ได้เรียก URL นี้)
func realOffice() domain.Office {
	o := demoOffice()
	o.BackofficeAPIURL = "https://demo-staging-office.example"
	return o
}

func getBoot(t *testing.T, r http.Handler, key, serviceID, token string) (int, domain.Bootstrap, string) {
	t.Helper()
	w := do(t, r, req{method: http.MethodGet, path: bootPath(key, serviceID), origin: officeOrigin, token: token})
	return w.Code, payload[domain.Bootstrap](t, w), w.Body.String()
}

func TestBootstrap_Enabled(t *testing.T) {
	r := newRouter(t)
	code, b, raw := getBoot(t, r, demoKey, "K11S", devToken("adm_ploy", "K11S", "PG99"))

	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("ควรเปิด: %d %s", code, raw)
	}
	if b.OfficeID != "demo" || b.ServiceID != "K11S" {
		t.Fatalf("office/service ผิด: %+v", b)
	}
}

// หัวใจของโมเดล 2 ชั้น: key เดียวกัน คนละ service ต้องได้คนละผล
func TestBootstrap_OneKeyManyServices(t *testing.T) {
	r := newRouter(t)
	tok := devToken("adm_ploy", "K11S", "PG99")

	_, k, _ := getBoot(t, r, demoKey, "K11S", tok)
	_, p, _ := getBoot(t, r, demoKey, "PG99", tok)

	if !k.Enabled || k.ServiceID != "K11S" {
		t.Fatalf("K11S ควรเปิด: %+v", k)
	}
	if p.Enabled || p.Reason != "service_disabled" {
		t.Fatalf("PG99 ยังปิดอยู่ ต้องได้ service_disabled: %+v", p)
	}
}

// ██ กฎที่แทน GC-1 เดิม
// หน้าเว็บบอก service ได้ (เพราะไม่มี session ฝั่ง server) แต่ต้องมีสิทธิ์จริง
func TestBootstrap_ServiceMustBeInListService(t *testing.T) {
	o := demoOffice()
	o.Services[1].Enabled = true
	r := newRouter(t, o)

	// แอดมินคนนี้มีสิทธิ์เฉพาะ K11S
	tok := devToken("adm_ploy", "K11S")

	if code, b, _ := getBoot(t, r, demoKey, "K11S", tok); code != http.StatusOK || !b.Enabled {
		t.Fatalf("service ที่มีสิทธิ์ต้องผ่าน: %d %+v", code, b)
	}

	// แก้ localStorage ให้ชี้ PG99 ทั้งที่ไม่มีสิทธิ์ → ต้องโดนปฏิเสธที่ชั้นเรา
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "PG99"), origin: officeOrigin, token: tok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("service ที่ไม่มีสิทธิ์ต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SERVICE_NOT_ALLOWED") {
		t.Fatalf("อยากได้ SERVICE_NOT_ALLOWED ได้ %s", w.Body.String())
	}
}

// service ที่ ListService บอกว่า permission=false ต้องใช้ไม่ได้
func TestBootstrap_ListServicePermissionFalse(t *testing.T) {
	r := newRouter(t, realOffice())
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": false, "PG99": true}, nil)

	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: tok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("permission=false ต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
}

// เส้นทางจริง: อ่านว่า token นี้เป็นใครจาก JWT payload โดยตรง
func TestBootstrap_ResolvesFromJWT(t *testing.T) {
	r := newRouter(t, realOffice())
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": true}, []string{"view_member"})

	code, b, raw := getBoot(t, r, demoKey, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("ควรผ่าน: %d %s", code, raw)
	}
}

func TestBootstrap_BadToken(t *testing.T) {
	r := newRouter(t, realOffice())

	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: "token-ปลอม"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("token ที่ decode ไม่ได้ ต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
}

// allowlist ว่าง = เปิดให้ทุกคนที่ล็อกอิน office สำเร็จ (ไม่ต้องกรองรายคน)
func TestBootstrap_EmptyAllowlistMeansEveryone(t *testing.T) {
	o := realOffice()
	o.Services[0].Allowlist = nil // K11S: allowlist ว่าง
	r := newRouter(t, o)

	// user ที่ไม่มีในลิสต์ไหนเลย แต่ล็อกอิน + มีสิทธิ์ service → ต้องผ่าน
	tok := officeJWT("someone_new", map[string]bool{"K11S": true}, nil)
	code, b, raw := getBoot(t, r, demoKey, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("allowlist ว่างต้องเปิดให้ทุกคน: %d %s", code, raw)
	}
}

// แต่ถ้า allowlist มีรายชื่อ ต้องกรองเฉพาะคนในลิสต์
func TestBootstrap_NonEmptyAllowlistStillFilters(t *testing.T) {
	r := newRouter(t, realOffice()) // K11S allowlist = [adm_ploy]

	tok := officeJWT("someone_new", map[string]bool{"K11S": true}, nil)
	_, b, _ := getBoot(t, r, demoKey, "K11S", tok)
	if b.Enabled || b.Reason != "not_in_allowlist" {
		t.Fatalf("คนนอกลิสต์ต้องโดน not_in_allowlist: %+v", b)
	}
}

// token หมดอายุ → ต้องบอกชัดว่า 401 ไม่ใช่แกล้งทำเป็นว่าไม่มีสิทธิ์
func TestBootstrap_ExpiredToken(t *testing.T) {
	r := newRouter(t, realOffice())
	tok := officeJWTExp("adm_ploy", map[string]bool{"K11S": true}, nil, time.Now().Add(-time.Minute).Unix())

	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: tok})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("token หมดอายุต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
}

func TestBootstrap_UnknownKey(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath("pk_no_such_key", "K11S"), origin: officeOrigin, token: devToken("adm_ploy", "K11S")})
	if w.Code != http.StatusNotFound {
		t.Fatalf("key มั่วต้อง 404 ได้ %d", w.Code)
	}
}

// key หลุดไปแล้วเอาไปแปะโดเมนอื่นต้องใช้ไม่ได้
func TestBootstrap_OriginNotRegistered(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: "http://evil.example", token: devToken("adm_ploy", "K11S")})
	if w.Code != http.StatusForbidden {
		t.Fatalf("โดเมนที่ไม่ได้ลงทะเบียนต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
}

func TestBootstrap_OfficeKillSwitch(t *testing.T) {
	o := demoOffice()
	o.Enabled = false
	r := newRouter(t, o)

	_, b, _ := getBoot(t, r, demoKey, "K11S", devToken("adm_ploy", "K11S"))
	if b.Enabled || b.Reason != "office_disabled" {
		t.Fatalf("ปิดทั้ง office แล้วทุก service ต้องปิดตาม ได้ %+v", b)
	}
}

func TestBootstrap_NoToken(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ไม่มี token ต้อง 401 ได้ %d", w.Code)
	}
}

// ข้อมูลความปลอดภัยห้ามหลุดลงหน้าเว็บ
func TestBootstrap_NeverLeaksSecrets(t *testing.T) {
	r := newRouter(t, realOffice())
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": true, "SECRET_SVC": true}, []string{"view_member"})

	_, _, raw := getBoot(t, r, demoKey, "K11S", tok)
	for _, bad := range []string{`"allowlist"`, "SECRET_SVC", "allowed_origins", "backoffice_api_url", "view_member"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("bootstrap ไม่ควรมี %q แต่เจอใน %s", bad, raw)
		}
	}
}

// ██ ข้อมูลจริง: list_service ว่างทุกคนใน demo-staging_office
// ว่าง = ไม่จำกัด (ตรงกับพฤติกรรมของ office-api เองที่ไม่ได้เช็ค field นี้)
func TestBootstrap_EmptyListServiceMeansNoRestriction(t *testing.T) {
	r := newRouter(t, realOffice())
	tok := officeJWT("adm_ploy", map[string]bool{}, []string{"REPORT"}) // list_service ว่าง

	code, b, raw := getBoot(t, r, demoKey, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("list_service ว่างต้องไม่ถูกจำกัด: %d %s", code, raw)
	}
}

// แต่ service ที่ไม่มีใน office นี้ ยังต้องถูกปฏิเสธแม้ list_service จะว่าง
func TestBootstrap_ServiceMustExistInOfficeEvenWhenUnrestricted(t *testing.T) {
	r := newRouter(t, realOffice())
	tok := officeJWT("adm_ploy", map[string]bool{}, nil)

	_, b, _ := getBoot(t, r, demoKey, "SERVICE_ที่ไม่มี", tok)
	if b.Enabled || b.Reason != "service_not_in_office" {
		t.Fatalf("service ที่ไม่มีใน office ต้องถูกปฏิเสธ ได้ %+v", b)
	}
}
