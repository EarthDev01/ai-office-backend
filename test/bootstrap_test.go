package test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

func getBoot(t *testing.T, r http.Handler, serviceID, token string) (int, domain.Bootstrap, string) {
	t.Helper()
	w := do(t, r, req{method: http.MethodGet, path: bootPath(serviceID), origin: officeOrigin, token: token})
	return w.Code, payload[domain.Bootstrap](t, w), w.Body.String()
}

func TestBootstrap_Enabled(t *testing.T) {
	r := newRouter(t)
	code, b, raw := getBoot(t, r, "K11S", adminJWT("adm_ploy", "K11S", "PG99"))

	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("ควรเปิด: %d %s", code, raw)
	}
	if b.OfficeID != "demo" || b.ServiceID != "K11S" {
		t.Fatalf("office/service ผิด: %+v", b)
	}
}

// หัวใจของโมเดล 2 ชั้น: โดเมนเดียวกัน คนละ service ต้องได้คนละผล
func TestBootstrap_OneKeyManyServices(t *testing.T) {
	r := newRouter(t)
	tok := adminJWT("adm_ploy", "K11S", "PG99")

	_, k, _ := getBoot(t, r, "K11S", tok)
	_, p, _ := getBoot(t, r, "PG99", tok)

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
	tok := adminJWT("adm_ploy", "K11S")

	if code, b, _ := getBoot(t, r, "K11S", tok); code != http.StatusOK || !b.Enabled {
		t.Fatalf("service ที่มีสิทธิ์ต้องผ่าน: %d %+v", code, b)
	}

	// แก้ localStorage ให้ชี้ PG99 ทั้งที่ไม่มีสิทธิ์ → ต้องโดนปฏิเสธที่ชั้นเรา
	w := do(t, r, req{method: http.MethodGet, path: bootPath("PG99"), origin: officeOrigin, token: tok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("service ที่ไม่มีสิทธิ์ต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SERVICE_NOT_ALLOWED") {
		t.Fatalf("อยากได้ SERVICE_NOT_ALLOWED ได้ %s", w.Body.String())
	}
}

// service ที่ ListService บอกว่า permission=false ต้องใช้ไม่ได้
func TestBootstrap_ListServicePermissionFalse(t *testing.T) {
	r := newRouter(t, demoOffice())
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": false, "PG99": true}, nil)

	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: officeOrigin, token: tok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("permission=false ต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
}

// เส้นทางจริง: อ่านว่า token นี้เป็นใครจาก JWT payload โดยตรง
func TestBootstrap_ResolvesFromJWT(t *testing.T) {
	r := newRouter(t, demoOffice())
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": true}, []string{"view_member"})

	code, b, raw := getBoot(t, r, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("ควรผ่าน: %d %s", code, raw)
	}
}

func TestBootstrap_BadToken(t *testing.T) {
	r := newRouter(t, demoOffice())

	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: officeOrigin, token: "token-ปลอม"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("token ที่ decode ไม่ได้ ต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
}

// allowlist ว่าง = เปิดให้ทุกคนที่ล็อกอิน office สำเร็จ (ไม่ต้องกรองรายคน)
func TestBootstrap_EmptyAllowlistMeansEveryone(t *testing.T) {
	o := demoOffice()
	o.Services[0].Allowlist = nil // K11S: allowlist ว่าง
	r := newRouter(t, o)

	// user ที่ไม่มีในลิสต์ไหนเลย แต่ล็อกอิน + มีสิทธิ์ service → ต้องผ่าน
	tok := officeJWT("someone_new", map[string]bool{"K11S": true}, nil)
	code, b, raw := getBoot(t, r, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("allowlist ว่างต้องเปิดให้ทุกคน: %d %s", code, raw)
	}
}

// ยกเลิก allowlist แล้ว — แม้ service จะมีรายชื่อเดิมติดอยู่ ใครที่ล็อกอิน + มีสิทธิ์
// service นั้น ก็ใช้ AI ได้ทุกคน (ไม่กรองรายคนอีกต่อไป)
func TestBootstrap_AllowlistNoLongerFilters(t *testing.T) {
	r := newRouter(t, demoOffice()) // K11S allowlist = [adm_ploy]

	tok := officeJWT("someone_new", map[string]bool{"K11S": true}, nil) // ไม่อยู่ในลิสต์
	code, b, raw := getBoot(t, r, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("ยกเลิก allowlist แล้ว ทุกคนที่ล็อกอิน+มีสิทธิ์ service ต้องผ่าน: %d %s", code, raw)
	}
}

// token หมดอายุ → ต้องบอกชัดว่า 401 ไม่ใช่แกล้งทำเป็นว่าไม่มีสิทธิ์
func TestBootstrap_ExpiredToken(t *testing.T) {
	r := newRouter(t, demoOffice())
	tok := officeJWTExp("adm_ploy", map[string]bool{"K11S": true}, nil, time.Now().Add(-time.Minute).Unix())

	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: officeOrigin, token: tok})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("token หมดอายุต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
}

// ไม่มี Origin = ระบุไม่ได้ว่าเป็นลูกค้าเจ้าไหน → ปฏิเสธ (เดิมถือว่า same-origin แล้วปล่อยผ่าน)
func TestBootstrap_NoOriginRejected(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), token: adminJWT("adm_ploy", "K11S")})
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "ORIGIN_REQUIRED") {
		t.Fatalf("ไม่มี Origin ต้อง 403 ORIGIN_REQUIRED ได้ %d %s", w.Code, w.Body.String())
	}
}

// snippet ถูกเอาไปแปะโดเมนที่ไม่ได้ลงทะเบียน ต้องใช้ไม่ได้
func TestBootstrap_OriginNotRegistered(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: "http://evil.example", token: adminJWT("adm_ploy", "K11S")})
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "ORIGIN_NOT_REGISTERED") {
		t.Fatalf("โดเมนที่ไม่ได้ลงทะเบียนต้อง 403 ORIGIN_NOT_REGISTERED ได้ %d %s", w.Code, w.Body.String())
	}
}

// Origin ต่างกันแค่ตัวพิมพ์ / / ท้าย / พอร์ตมาตรฐาน ต้องถือเป็นโดเมนเดียวกัน
func TestBootstrap_OriginNormalized(t *testing.T) {
	o := demoOffice()
	o.AllowedOrigins = []string{"https://admin.k11s.com"}
	r := newRouter(t, o)
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": true}, nil)
	for _, origin := range []string{"https://ADMIN.k11s.com", "https://admin.k11s.com:443", "https://admin.k11s.com/"} {
		w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: origin, token: tok})
		if w.Code != http.StatusOK {
			t.Fatalf("origin %q ต้องผ่าน ได้ %d %s", origin, w.Code, w.Body.String())
		}
	}
	// คนละ scheme = คนละโดเมน
	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: "http://admin.k11s.com", token: tok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("http กับ https ต้องไม่ถือเป็นโดเมนเดียวกัน ได้ %d", w.Code)
	}
}

func TestBootstrap_OfficeKillSwitch(t *testing.T) {
	o := demoOffice()
	o.Enabled = false
	r := newRouter(t, o)

	_, b, _ := getBoot(t, r, "K11S", adminJWT("adm_ploy", "K11S"))
	if b.Enabled || b.Reason != "office_disabled" {
		t.Fatalf("ปิดทั้ง office แล้วทุก service ต้องปิดตาม ได้ %+v", b)
	}
}

func TestBootstrap_NoToken(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: officeOrigin})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ไม่มี token ต้อง 401 ได้ %d", w.Code)
	}
}

// ข้อมูลความปลอดภัยห้ามหลุดลงหน้าเว็บ
func TestBootstrap_NeverLeaksSecrets(t *testing.T) {
	r := newRouter(t, demoOffice())
	tok := officeJWT("adm_ploy", map[string]bool{"K11S": true, "SECRET_SVC": true}, []string{"view_member"})

	_, _, raw := getBoot(t, r, "K11S", tok)
	for _, bad := range []string{`"allowlist"`, "SECRET_SVC", "allowed_origins", "backoffice_api_url", "view_member"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("bootstrap ไม่ควรมี %q แต่เจอใน %s", bad, raw)
		}
	}
}

// ██ ข้อมูลจริง: list_service ว่างทุกคนใน demo-staging_office
// ว่าง = ไม่จำกัด (ตรงกับพฤติกรรมของ office-api เองที่ไม่ได้เช็ค field นี้)
func TestBootstrap_EmptyListServiceMeansNoRestriction(t *testing.T) {
	r := newRouter(t, demoOffice())
	tok := officeJWT("adm_ploy", map[string]bool{}, []string{"REPORT"}) // list_service ว่าง

	code, b, raw := getBoot(t, r, "K11S", tok)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("list_service ว่างต้องไม่ถูกจำกัด: %d %s", code, raw)
	}
}

// แต่ service ที่ไม่มีใน office นี้ ยังต้องถูกปฏิเสธแม้ list_service จะว่าง
func TestBootstrap_ServiceMustExistInOfficeEvenWhenUnrestricted(t *testing.T) {
	r := newRouter(t, demoOffice())
	tok := officeJWT("adm_ploy", map[string]bool{}, nil)

	_, b, _ := getBoot(t, r, "SERVICE_ที่ไม่มี", tok)
	if b.Enabled || b.Reason != "service_not_in_office" {
		t.Fatalf("service ที่ไม่มีใน office ต้องถูกปฏิเสธ ได้ %+v", b)
	}
}

// รับเฉพาะ JWT จริง — token ทดสอบรูปแบบเก่า "dev:..." ต้องถูกปฏิเสธ
func TestBootstrap_DevTokenRejected(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath("K11S"), origin: officeOrigin, token: "dev:adm_ploy:K11S"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("dev token ต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
}
