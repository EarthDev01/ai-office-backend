package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
)

// makeJWT สร้าง token 3 ส่วน (header.payload.signature) โดย payload = JSON ที่ให้มา
// signature เป็นค่าอะไรก็ได้ เพราะ parseOfficeJWT ไม่ verify signature
func makeJWT(payload map[string]any) string {
	b, _ := json.Marshal(payload)
	p := base64.RawURLEncoding.EncodeToString(b)
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	return h + "." + p + ".sig"
}

// identitySpec อ่าน page_auth.identity จาก connector จริงใน repo — test จึงพังเมื่อ yaml เปลี่ยนรูป
func identitySpec(t *testing.T, kind string) *connector.IdentitySpec {
	t.Helper()
	c, err := connector.Load("../../../connectors/" + kind)
	if err != nil {
		t.Fatalf("load %s: %v", kind, err)
	}
	return c.Host.PageAuth.Identity
}

// payload ตาม model.EmployeeModel ของ office-api-v10
func v10xResult(exp int64) map[string]any {
	return map[string]any{
		"exp": exp,
		"result": map[string]any{
			"id":       "665f0a1b2c3d4e5f60718293",
			"username": "adm_ploy",
			"level":    6,
			"role": map[string]any{
				"name":       "manager",
				"level":      3,
				"permission": nil, // office-api-v10 ล้างก่อนเซ็นเสมอ — สิทธิ์มาจาก permissions_request
				"list_service": []map[string]any{
					{"service": "DEMO-STAGING", "permission": true},
					{"service": "SECRET-SVC", "permission": false}, // ต้องถูกข้าม
					{"service": "", "permission": true},            // ต้องถูกข้าม
				},
			},
		},
	}
}

func TestParseOfficeJWT_V10x(t *testing.T) {
	spec := identitySpec(t, "office-v10x")
	caller, err := parseOfficeJWT(makeJWT(v10xResult(time.Now().Add(time.Hour).Unix())), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caller.AdminID != "665f0a1b2c3d4e5f60718293" || caller.Username != "adm_ploy" {
		t.Errorf("ตัวตนผิด: %+v", caller)
	}
	if caller.Level != 6 {
		t.Errorf("Level = %d (ต้องมาจาก result.level ตาม host.yaml)", caller.Level)
	}
	if len(caller.Permissions) != 0 {
		t.Errorf("v10x อ่านสิทธิ์จาก permissions_request ไม่ใช่ JWT: %v", caller.Permissions)
	}
	if got := caller.Services; len(got) != 1 || got[0] != "DEMO-STAGING" {
		t.Errorf("Services = %v (want [DEMO-STAGING])", got)
	}
	if caller.AllServices || !caller.ServiceAllowed("DEMO-STAGING") || caller.ServiceAllowed("SECRET-SVC") {
		t.Errorf("สิทธิ์เว็บผิด: %+v", caller)
	}
}

// list_service ว่าง = ไม่จำกัด (พฤติกรรมเดิมของ v10x)
func TestParseOfficeJWT_V10xEmptyListServiceUnrestricted(t *testing.T) {
	p := v10xResult(time.Now().Add(time.Hour).Unix())
	p["result"].(map[string]any)["role"].(map[string]any)["list_service"] = []any{}
	caller, err := parseOfficeJWT(makeJWT(p), identitySpec(t, "office-v10x"))
	if err != nil {
		t.Fatal(err)
	}
	if !caller.ServiceAllowed("ANY") {
		t.Errorf("list_service ว่างต้องไม่จำกัด: %+v", caller)
	}
}

// payload ตาม employee.User ของ GOTOPOPOFFICE (headertoken — role.permission ถูกล้างก่อนเซ็น)
func abatechResult(level int, dept string, services ...string) map[string]any {
	svc := make([]map[string]any, 0, len(services))
	for _, s := range services {
		svc = append(svc, map[string]any{"service": s, "group_type": "TOPUP", "group_name": "TOPUP"})
	}
	return map[string]any{
		"exp": time.Now().Add(time.Hour).Unix(),
		"result": map[string]any{
			"id": "64b0c0ffee0000000000abcd", "name": "admin01", "full_name": "แอดมิน หนึ่ง",
			"deptcode": dept, "level": level, "service": svc,
			"role": map[string]any{"name": "Admin", "permission": nil},
		},
	}
}

func TestParseOfficeJWT_Abatech(t *testing.T) {
	spec := identitySpec(t, "office-abatech")

	caller, err := parseOfficeJWT(makeJWT(abatechResult(5, "D003", "DEMOSLOT")), spec)
	if err != nil {
		t.Fatal(err)
	}
	if caller.AdminID != "64b0c0ffee0000000000abcd" || caller.Username != "admin01" || caller.Level != 5 {
		t.Errorf("ตัวตนผิด: %+v", caller)
	}
	if len(caller.Permissions) != 0 {
		t.Errorf("abatech อ่านสิทธิ์จาก permissions_token ในหน้า ไม่ใช่ headertoken: %v", caller.Permissions)
	}
	if !caller.ServiceAllowed("DEMOSLOT") || caller.ServiceAllowed("FULLSLOT999") {
		t.Errorf("level < 10 เข้าได้เฉพาะ service ในรายชื่อ: %+v", caller)
	}

	// รายชื่อว่าง = เข้าไม่ได้สักเว็บ (ตรงกับเมนูของ office-abatech)
	caller, _ = parseOfficeJWT(makeJWT(abatechResult(5, "D003")), spec)
	if caller.ServiceAllowed("DEMOSLOT") {
		t.Errorf("service ว่างต้องเข้าไม่ได้: %+v", caller)
	}

	// level >= 10 หรือ D002 เห็นทุกเว็บ (layouts/menu.vue:449-465)
	for _, c := range []struct {
		level int
		dept  string
	}{{10, "D003"}, {99, "D001"}, {1, "D002"}} {
		caller, _ = parseOfficeJWT(makeJWT(abatechResult(c.level, c.dept)), spec)
		if !caller.ServiceAllowed("ANY") {
			t.Errorf("level %d dept %s ต้องเข้าได้ทุกเว็บ: %+v", c.level, c.dept, caller)
		}
	}
}

func TestParseOfficeJWT_Expired(t *testing.T) {
	token := makeJWT(v10xResult(time.Now().Add(-time.Minute).Unix()))
	if _, err := parseOfficeJWT(token, identitySpec(t, "office-v10x")); err != domain.ErrSessionExpired {
		t.Errorf("err = %v (want ErrSessionExpired)", err)
	}
}

func TestParseOfficeJWT_Malformed(t *testing.T) {
	spec := identitySpec(t, "office-v10x")
	cases := map[string]string{
		"not a jwt":    "abc",
		"two parts":    "aaa.bbb",
		"bad base64":   "aaa.$$$.ccc",
		"empty result": makeJWT(map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "result": map[string]any{}}),
		"no result":    makeJWT(map[string]any{"exp": time.Now().Add(time.Hour).Unix()}),
	}
	for name, tok := range cases {
		if _, err := parseOfficeJWT(tok, spec); err != domain.ErrNotAuthenticated {
			t.Errorf("%s: err = %v (want ErrNotAuthenticated)", name, err)
		}
	}
}
