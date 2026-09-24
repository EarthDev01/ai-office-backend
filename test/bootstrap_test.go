package test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// หลัง D-87: widget ขอตั๋วจาก host ก่อน (host → /session ด้วย secret) แล้วถือตั๋วมา bootstrap
// backend ไม่เคยเห็น token ของแอดมินเลย — test ชุดนี้แทน test เดิมที่อ่าน JWT ของแอดมิน

func TestBootstrap_Enabled(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	code, b, raw := getBoot(t, r, demoKey, "K11S", tk)
	if code != http.StatusOK || !b.Enabled {
		t.Fatalf("ควรเปิด: %d %s", code, raw)
	}
	if b.OfficeID != "demo" || b.ServiceID != "K11S" || b.Kind != testKind || b.TicketExpiresAt == 0 {
		t.Fatalf("office/service/kind ผิด: %+v", b)
	}
}

// หัวใจของโมเดล 2 ชั้น: key เดียวกัน คนละ service ต้องได้คนละผล
func TestSession_OneKeyManyServices(t *testing.T) {
	r := newRouter(t)
	if w := sessionReq(t, r, demoKey, "K11S", secretK11S, adminUser(), testKind); w.Code != http.StatusOK {
		t.Fatalf("K11S ต้องได้ตั๋ว: %d %s", w.Code, w.Body.String())
	}
	w := sessionReq(t, r, demoKey, "PG99", secretPG99, adminUser(), testKind)
	if w.Code != http.StatusForbidden || payload[map[string]string](t, w)["reason"] != domain.ReasonServiceDisabled {
		t.Fatalf("PG99 ยังปิดอยู่ ต้องได้ service_disabled: %d %s", w.Code, w.Body.String())
	}
}

// ตั๋วของ K11S ใช้กับ path ของ PG99 ไม่ได้ (แก้ service ฝั่ง client → ปฏิเสธ · AC-3)
func TestTicket_BoundToService(t *testing.T) {
	o := demoOffice()
	o.Services[1].Enabled = true
	r := newRouter(t, o)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "PG99"), origin: officeOrigin, token: tk})
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "TICKET_INVALID") {
		t.Fatalf("ตั๋วคนละ service ต้อง 401 ได้ %d %s", w.Code, w.Body.String())
	}
}

func TestBootstrap_BadTicket(t *testing.T) {
	r := newRouter(t)
	for _, tok := range []string{"token-ปลอม", "a.b.c"} {
		w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: tok})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("ตั๋วปลอม %q ต้อง 401 ได้ %d %s", tok, w.Code, w.Body.String())
		}
	}
}

// ตั๋วที่เซ็นด้วย secret อื่น (แต่ง claims เอง) ต้องใช้ไม่ได้ (AC-32 ฝั่ง backend)
func TestTicket_ForgedWithOtherSecret(t *testing.T) {
	r := newRouter(t)
	other := auth.NewAccessTicketIssuer("attacker-secret-xxxxxxxxxxxxxxxxxxxxxxxx")
	forged, _ := other.Issue(domain.AccessTicket{ID: "x", OfficeID: "demo", ServiceID: "K11S", Kind: testKind,
		User: domain.HostUser{ID: "emp_1", Username: "adm_ploy", Permissions: []string{"W_VIEW"}},
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: forged})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ตั๋วปลอมต้อง 401 ได้ %d", w.Code)
	}
}

// allow_all=true = ทุกคนที่ host ยืนยันแล้วใช้ได้ (R5)
func TestSession_AllowAllTrueMeansEveryone(t *testing.T) {
	o := demoOffice()
	o.Services[0].AllowAll = true
	r := newRouter(t, o)
	u := hostUser{ID: "emp_9", Username: "someone_new"}
	if w := sessionReq(t, r, demoKey, "K11S", secretK11S, u, testKind); w.Code != http.StatusOK {
		t.Fatalf("allow_all=true ต้องเปิดให้ทุกคน: %d %s", w.Code, w.Body.String())
	}
}

// allow_all=false: เฉพาะคนในลิสต์ · ลิสต์ว่าง = ไม่มีใคร (AC-17)
func TestSession_AllowAllFalseFilters(t *testing.T) {
	r := newRouter(t) // K11S allow_all=false · allowlist [adm_ploy]
	w := sessionReq(t, r, demoKey, "K11S", secretK11S, hostUser{ID: "emp_9", Username: "someone_new"}, testKind)
	if w.Code != http.StatusForbidden || payload[map[string]string](t, w)["reason"] != domain.ReasonNotInAllowlist {
		t.Fatalf("คนนอกลิสต์ต้องโดน not_in_allowlist: %d %s", w.Code, w.Body.String())
	}
}

func TestSession_AllowAllFalseEmptyMeansNobody(t *testing.T) {
	o := demoOffice()
	o.Services[0].Allowlist = nil
	r := newRouter(t, o)
	w := sessionReq(t, r, demoKey, "K11S", secretK11S, adminUser(), testKind)
	if w.Code != http.StatusForbidden || payload[map[string]string](t, w)["reason"] != domain.ReasonNotInAllowlist {
		t.Fatalf("allowlist ว่าง + allow_all=false ต้องไม่มีใครใช้ได้: %d %s", w.Code, w.Body.String())
	}
}

// AC-24: 5 กรณีที่ /session ต้องปฏิเสธ
func TestSession_RefusesBadSecrets(t *testing.T) {
	cases := map[string]struct {
		office func(*domain.Office)
		sid    string
		secret string
		want   int
	}{
		"ไม่มี secret":           {nil, "K11S", "", http.StatusForbidden},
		"secret ผิด":            {nil, "K11S", "aisk_wrong", http.StatusForbidden},
		"secret ของ service อื่น": {func(o *domain.Office) { o.Services[1].Enabled = true }, "K11S", secretPG99, http.StatusForbidden},
		"ใช้ public_key แทน":     {nil, "K11S", demoKey, http.StatusForbidden},
		"ถูก revoke":             {func(o *domain.Office) { o.Services[0].SecretKeyHash = "" }, "K11S", secretK11S, http.StatusForbidden},
		"office ปิด":            {func(o *domain.Office) { o.Enabled = false }, "K11S", secretK11S, http.StatusForbidden},
		"service ปิด":           {func(o *domain.Office) { o.Services[0].Enabled = false }, "K11S", secretK11S, http.StatusForbidden},
		"service ไม่มีใน office":  {nil, "NOPE", secretK11S, http.StatusForbidden},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := demoOffice()
			if tc.office != nil {
				tc.office(&o)
			}
			r := newRouter(t, o)
			w := sessionReq(t, r, demoKey, tc.sid, tc.secret, adminUser(), testKind)
			if w.Code != tc.want {
				t.Fatalf("อยากได้ %d ได้ %d %s", tc.want, w.Code, w.Body.String())
			}
			// host ส่ง body นี้ต่อให้ browser — body ห้ามมี code 401 (UI หลังบ้าน logout ทันที · contract §2)
			if strings.Contains(w.Body.String(), `"code":401`) {
				t.Fatalf("body ห้ามมี code 401: %s", w.Body.String())
			}
		})
	}
}

func TestSession_KindMustMatch(t *testing.T) {
	r := newRouter(t)
	w := sessionReq(t, r, demoKey, "K11S", secretK11S, adminUser(), "office-other")
	if w.Code != http.StatusForbidden || payload[map[string]string](t, w)["reason"] != domain.ReasonKindMismatch {
		t.Fatalf("kind ไม่ตรงต้องปฏิเสธ: %d %s", w.Code, w.Body.String())
	}
}

// rotate: ใบเก่ายังใช้ได้จนกว่าจะ commit (R3 · AC-24)
func TestSecret_RotateKeepsOldUntilCommit(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/services/K11S/secret/rotate", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("rotate: %d %s", w.Code, w.Body.String())
	}
	newSecret, _ := payload[map[string]any](t, w)["secret_key"].(string)
	if !strings.HasPrefix(newSecret, domain.SecretPrefix) {
		t.Fatalf("ต้องได้ secret ใหม่ครั้งเดียว: %q", newSecret)
	}
	for _, s := range []string{secretK11S, newSecret} {
		if w := sessionReq(t, r, demoKey, "K11S", s, adminUser(), testKind); w.Code != http.StatusOK {
			t.Fatalf("ช่วงหมุน ใบเก่าและใบใหม่ต้องใช้ได้ทั้งคู่: %d", w.Code)
		}
	}
	do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/services/K11S/secret/commit", console: consoleToken})
	if w := sessionReq(t, r, demoKey, "K11S", secretK11S, adminUser(), testKind); w.Code != http.StatusForbidden {
		t.Fatalf("หลัง commit ใบเก่าต้องใช้ไม่ได้: %d", w.Code)
	}
	if w := sessionReq(t, r, demoKey, "K11S", newSecret, adminUser(), testKind); w.Code != http.StatusOK {
		t.Fatalf("ใบใหม่ต้องใช้ได้: %d", w.Code)
	}
}

// revoke มีผลทันทีกับตั๋วที่ออกไปแล้ว (ไม่ต้องรอตั๋วหมดอายุ)
func TestSecret_RevokeKillsExistingTickets(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	if code, _, _ := getBoot(t, r, demoKey, "K11S", tk); code != http.StatusOK {
		t.Fatalf("ก่อน revoke ต้องใช้ได้ %d", code)
	}
	w := do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/services/K11S/secret/revoke", console: consoleToken})
	if w.Code != http.StatusOK {
		t.Fatalf("revoke: %d", w.Code)
	}
	w = do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: tk})
	if w.Code != http.StatusForbidden || payload[map[string]string](t, w)["reason"] != domain.ReasonNoSecret {
		t.Fatalf("หลัง revoke ตั๋วเดิมต้องใช้ไม่ได้ทันที: %d %s", w.Code, w.Body.String())
	}
}

func TestSecret_ShownOnceNeverInOfficeJSON(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/offices/demo", console: consoleToken})
	body := w.Body.String()
	if strings.Contains(body, domain.HashSecret(secretK11S)) || strings.Contains(body, "secret_key_hash") {
		t.Fatalf("hash ของ secret ห้ามหลุดไปคอนโซล: %s", body)
	}
	if !strings.Contains(body, `"has_secret":true`) {
		t.Fatalf("คอนโซลต้องเห็นสถานะ has_secret: %s", body)
	}
	// ออกซ้ำตอนมีอยู่แล้วต้องไม่ได้ (ต้องใช้ rotate)
	w = do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/services/K11S/secret", console: consoleToken})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("มี secret แล้วออกซ้ำต้องไม่ได้: %d", w.Code)
	}
}

// ตั๋วหมดอายุ → 401 TICKET_EXPIRED (widget ขอใหม่เงียบ ๆ)
func TestTicket_Expired(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	_ = e
	issuer := auth.NewAccessTicketIssuer(ticketSecret)
	parsed, err := issuer.Verify(tk)
	if err != nil {
		t.Fatal(err)
	}
	parsed.IssuedAt = time.Now().Add(-2 * time.Hour)
	parsed.ExpiresAt = time.Now().Add(-time.Minute)
	old, _ := issuer.Issue(parsed)
	w := do(t, e.r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: old})
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "TICKET_EXPIRED") {
		t.Fatalf("ตั๋วหมดอายุต้อง 401 TICKET_EXPIRED ได้ %d %s", w.Code, w.Body.String())
	}
}

// ตั๋วห้ามอยู่นานกว่า grant ของ host
func TestSession_TicketNotLongerThanGrant(t *testing.T) {
	r := newRouter(t)
	u := adminUser()
	body := `{"kind":"` + testKind + `","user":{"id":"emp_1","username":"adm_ploy"},"grant":"` + fakeGrant(u.ID, "K11S", time.Now().Add(3*time.Minute)) + `"}`
	w := do(t, r, req{method: http.MethodPost, path: basePath(demoKey, "K11S") + "/session", body: body, headers: [][2]string{{"X-AI-Secret", secretK11S}}})
	res := payload[port.SessionResult](t, w)
	if w.Code != http.StatusOK || res.TTLSec > 181 {
		t.Fatalf("ตั๋วต้องหมดพร้อม grant (≤3 นาที) ได้ %d ttl=%d", w.Code, res.TTLSec)
	}
}

func TestBootstrap_UnknownKey(t *testing.T) {
	r := newRouter(t)
	w := sessionReq(t, r, "pk_no_such_key", "K11S", secretK11S, adminUser(), testKind)
	if w.Code != http.StatusNotFound {
		t.Fatalf("key มั่วต้อง 404 ได้ %d", w.Code)
	}
}

// key หลุดไปแล้วเอาไปแปะโดเมนอื่นต้องใช้ไม่ได้
func TestBootstrap_OriginNotRegistered(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: "http://evil.example", token: tk})
	if w.Code != http.StatusForbidden {
		t.Fatalf("โดเมนที่ไม่ได้ลงทะเบียนต้อง 403 ได้ %d %s", w.Code, w.Body.String())
	}
	w = do(t, r, req{method: http.MethodGet, path: "/api/ai/office/" + demoKey + "/page-config", origin: "http://evil.example"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("page-config จากโดเมนอื่นต้อง 403 ได้ %d", w.Code)
	}
}

// ปิดทั้ง office = ทุก service ปิดตาม (สวิตช์ฉุกเฉิน) รวมตั๋วที่ออกไปแล้ว
func TestBootstrap_OfficeKillSwitch(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	do(t, r, req{method: http.MethodPatch, path: "/api/ai/admin/offices/demo", console: consoleToken, body: `{"enabled":false}`})
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: tk})
	if w.Code != http.StatusForbidden || payload[map[string]string](t, w)["reason"] != domain.ReasonOfficeDisabled {
		t.Fatalf("ปิด office แล้วตั๋วเดิมต้องใช้ไม่ได้: %d %s", w.Code, w.Body.String())
	}
}

func TestBootstrap_NoTicket(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ไม่มีตั๋วต้อง 401 ได้ %d", w.Code)
	}
}

// ข้อมูลความปลอดภัยห้ามหลุดลงหน้าเว็บ
func TestBootstrap_NeverLeaksSecrets(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	_, _, raw := getBoot(t, r, demoKey, "K11S", tk)
	pc := do(t, r, req{method: http.MethodGet, path: "/api/ai/office/" + demoKey + "/page-config", origin: officeOrigin}).Body.String()
	for _, body := range []string{raw, pc} {
		for _, bad := range []string{`"allowlist"`, "allowed_origins", "backoffice_api_url", "secret", "W_VIEW", "127.0.0.1"} {
			if strings.Contains(body, bad) {
				t.Fatalf("ไม่ควรมี %q แต่เจอใน %s", bad, body)
			}
		}
	}
}

func TestPageConfig_KindAndPageAuth(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/office/" + demoKey + "/page-config", origin: officeOrigin})
	p := payload[map[string]any](t, w)
	if w.Code != http.StatusOK || p["kind"] != testKind || p["enabled"] != true {
		t.Fatalf("page-config ผิด: %d %s", w.Code, w.Body.String())
	}
	pa, _ := p["page_auth"].(map[string]any)
	tok, _ := pa["token"].(map[string]any)
	if tok["key"] != "tok" || pa["session_path"] != "/ai/session/{service}" {
		t.Fatalf("page_auth ต้องมาจาก host.yaml ของ connector: %v", pa)
	}
}

// office ที่ยังไม่ได้ตั้ง kind ใช้ widget ไม่ได้ (R1)
func TestPageConfig_NoKindRefused(t *testing.T) {
	o := demoOffice()
	o.Kind = ""
	r := newRouter(t, o)
	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/office/" + demoKey + "/page-config", origin: officeOrigin})
	if p := payload[map[string]any](t, w); p["enabled"] != false || p["reason"] != domain.ReasonKindNotSet {
		t.Fatalf("ไม่มี kind ต้องปฏิเสธ: %s", w.Body.String())
	}
	if w := sessionReq(t, r, demoKey, "K11S", secretK11S, adminUser(), testKind); w.Code != http.StatusForbidden {
		t.Fatalf("ไม่มี kind ต้องไม่ออกตั๋ว: %d", w.Code)
	}
}

// ตั๋วพกแค่ permission ที่ connector อ้างถึง (ตั๋วเล็ก · ไม่พกข้อมูลเกิน)
func TestSession_TicketKeepsOnlyRelevantPermissions(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser("W_VIEW", "SOMETHING_ELSE", "PHONE_SECRET"))
	parsed, _ := auth.NewAccessTicketIssuer(ticketSecret).Verify(tk)
	if len(parsed.User.Permissions) != 1 || parsed.User.Permissions[0] != "W_VIEW" {
		t.Fatalf("ตั๋วควรเหลือแค่ W_VIEW: %v", parsed.User.Permissions)
	}
	if strings.Contains(tk, "PHONE_SECRET") {
		t.Fatal("code ที่ไม่เกี่ยวต้องไม่อยู่ในตั๋ว")
	}
}
