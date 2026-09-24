package test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

const auditBase = "/api/ai/admin/audit-logs"

type auditPage struct {
	Items    []domain.AuditEntry `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

func queryAudit(t *testing.T, r http.Handler, token string, params url.Values) auditPage {
	t.Helper()
	path := auditBase
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	w := do(t, r, req{method: http.MethodGet, path: path, console: token})
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, w.Code, w.Body.String())
	}
	return payload[auditPage](t, w)
}

func findAction(items []domain.AuditEntry, action string) (domain.AuditEntry, bool) {
	for _, e := range items {
		if e.Action == action {
			return e, true
		}
	}
	return domain.AuditEntry{}, false
}

func changeOf(e domain.AuditEntry, field string) (domain.FieldChange, bool) {
	for _, c := range e.Changes {
		if c.Field == field {
			return c, true
		}
	}
	return domain.FieldChange{}, false
}

// ตั้งค่าครั้งแรก + ยืนยัน 2FA ต้องได้ประวัติ setup / 2fa_enrolled / login ของคนนั้นพร้อม ip และ user-agent
func TestAudit_SetupAndLoginRecorded(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	page := queryAudit(t, r, vp.Token, url.Values{"category": {"auth"}})
	for _, action := range []string{domain.AuditAuthSetup, domain.AuditAuth2FAEnrolled, domain.AuditAuthLogin} {
		e, ok := findAction(page.Items, action)
		if !ok {
			t.Fatalf("missing %s in %+v", action, page.Items)
		}
		if e.Actor != "earth" || e.ActorID != vp.User.ID || e.Status != domain.AuditSuccess {
			t.Fatalf("%s: wrong actor/status: %+v", action, e)
		}
		if e.IP != "192.0.2.1" || e.Method != http.MethodPost || !strings.HasPrefix(e.Path, authBase) {
			t.Fatalf("%s: request meta missing: %+v", action, e)
		}
	}
	// ใหม่→เก่า: login มาหลัง setup จึงต้องอยู่ก่อนในรายการ
	if page.Items[0].Action != domain.AuditAuthLogin {
		t.Fatalf("want newest first (auth.login), got %s", page.Items[0].Action)
	}
}

// รหัสผ่านผิด / ไม่มี username ต้องบันทึกเป็น failure พร้อมเหตุผล และห้ามมีรหัสผ่านหลุดลงประวัติ
func TestAudit_FailedLoginRecordedWithoutPassword(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	do(t, r, req{method: http.MethodPost, path: authBase + "/login", body: `{"username":"earth","password":"wrong-pass-1"}`,
		headers: [][2]string{{"X-Forwarded-For", "203.0.113.7"}, {"User-Agent", "audit-test/1.0"}}})
	do(t, r, req{method: http.MethodPost, path: authBase + "/login", body: `{"username":"ghost","password":"wrong-pass-2"}`})

	page := queryAudit(t, r, vp.Token, url.Values{"action": {domain.AuditAuthLoginFailed}})
	if page.Total != 2 {
		t.Fatalf("want 2 failed logins, got %d: %+v", page.Total, page.Items)
	}
	reasons := map[string]string{}
	for _, e := range page.Items {
		if e.Status != domain.AuditFailure {
			t.Fatalf("failed login should be failure: %+v", e)
		}
		reasons[e.Actor] = e.Reason
		if e.Actor == "earth" && (e.IP != "203.0.113.7" || e.UserAgent != "audit-test/1.0") {
			t.Fatalf("ip/user-agent not recorded: %+v", e)
		}
		if strings.Contains(e.Summary, "wrong-pass") {
			t.Fatalf("password leaked into summary: %q", e.Summary)
		}
	}
	if reasons["earth"] != "INVALID_CREDENTIALS" || reasons["ghost"] != "UNKNOWN_USER" {
		t.Fatalf("reasons: %+v", reasons)
	}

	// status=failure กรองได้
	fails := queryAudit(t, r, vp.Token, url.Values{"status": {domain.AuditFailure}})
	if fails.Total != 2 {
		t.Fatalf("status filter: want 2, got %d", fails.Total)
	}
}

// ผิด 5 ครั้งติด → มีเหตุการณ์ account_locked แยกออกมาให้เห็นชัด
func TestAudit_AccountLockRecorded(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")
	for range 5 {
		do(t, r, req{method: http.MethodPost, path: authBase + "/login", body: `{"username":"earth","password":"nopenope"}`})
	}
	page := queryAudit(t, r, vp.Token, url.Values{"action": {domain.AuditAuthLocked}})
	if page.Total != 1 {
		t.Fatalf("want 1 lock event, got %d", page.Total)
	}
	if page.Items[0].Meta["locked_until"] == "" {
		t.Fatalf("lock event should carry locked_until: %+v", page.Items[0])
	}
}

// แก้ office ต้องเก็บค่าก่อน/หลังเฉพาะ field ที่เปลี่ยนจริง
func TestAudit_OfficeUpdateHasFieldDiff(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	w := do(t, r, req{
		method:  http.MethodPatch,
		path:    "/api/ai/admin/offices/demo",
		console: vp.Token,
		body:    `{"label":"หลังบ้านใหม่","theme":"dark","enabled":true}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch office: %d %s", w.Code, w.Body.String())
	}

	page := queryAudit(t, r, vp.Token, url.Values{"target_type": {"office"}, "target_id": {"demo"}})
	e, ok := findAction(page.Items, domain.AuditOfficeUpdate)
	if !ok {
		t.Fatalf("no office.update: %+v", page.Items)
	}
	if e.Actor != "earth" || e.ActorRole != "admin" {
		t.Fatalf("actor from session: %+v", e)
	}
	label, ok := changeOf(e, "label")
	if !ok || label.Before != "หลังบ้านจำลอง" || label.After != "หลังบ้านใหม่" {
		t.Fatalf("label diff: %+v", e.Changes)
	}
	if theme, ok := changeOf(e, "theme"); !ok || theme.Before != "auto" || theme.After != "dark" {
		t.Fatalf("theme diff: %+v", e.Changes)
	}
	// enabled ส่งมาค่าเดิม (true) → ไม่ใช่การแก้ไข
	if _, ok := changeOf(e, "enabled"); ok {
		t.Fatalf("unchanged field should not appear: %+v", e.Changes)
	}
	if len(e.Changes) != 2 {
		t.Fatalf("want exactly 2 changes, got %+v", e.Changes)
	}
}

// เพิ่ม/แก้/ลบ service + ลบ office ต้องมีครบ และลบแล้วยังรู้ชื่อเดิม
func TestAudit_ServiceLifecycleAndOfficeDelete(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")
	admin := "/api/ai/admin/offices/demo"

	steps := []req{
		{method: http.MethodPost, path: admin + "/services", body: `{"id":"NEW1","label":"เว็บใหม่"}`},
		{method: http.MethodPatch, path: admin + "/services/NEW1", body: `{"enabled":true,"greeting":"hi"}`},
		{method: http.MethodDelete, path: admin + "/services/NEW1"},
		{method: http.MethodDelete, path: admin},
	}
	for _, s := range steps {
		s.console = vp.Token
		if w := do(t, r, s); w.Code >= 300 {
			t.Fatalf("%s %s: %d %s", s.method, s.path, w.Code, w.Body.String())
		}
	}

	page := queryAudit(t, r, vp.Token, url.Values{"category": {"service"}})
	if page.Total != 3 {
		t.Fatalf("want 3 service events, got %d", page.Total)
	}
	upd, _ := findAction(page.Items, domain.AuditServiceUpdate)
	if c, ok := changeOf(upd, "enabled"); !ok || c.Before != false || c.After != true {
		t.Fatalf("service enabled diff: %+v", upd.Changes)
	}
	if upd.Meta["office_id"] != "demo" {
		t.Fatalf("service event should know its office: %+v", upd.Meta)
	}

	del, ok := findAction(queryAudit(t, r, vp.Token, nil).Items, domain.AuditOfficeDelete)
	if !ok || del.TargetLabel != "หลังบ้านจำลอง" || !strings.Contains(del.Meta["services"], "K11S") {
		t.Fatalf("office delete should keep label + services: %+v", del)
	}
}

// จัดการผู้ใช้: สร้าง / เปลี่ยน role / ลบ ต้องมีประวัติพร้อมค่าก่อน-หลัง
func TestAudit_UserManagement(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	w := do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/users", console: vp.Token,
		body: `{"username":"ploy","display_name":"พลอย","role":"viewer","password":"secret12"}`})
	if w.Code != http.StatusOK {
		t.Fatalf("create user: %d %s", w.Code, w.Body.String())
	}
	id := payload[struct {
		User domain.ConsoleUser `json:"user"`
	}](t, w).User.ID

	do(t, r, req{method: http.MethodPatch, path: "/api/ai/admin/users/" + id, console: vp.Token, body: `{"role":"operator"}`})
	do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/users/" + id + "/reset-password", console: vp.Token, body: `{"password":"newsecret1"}`})
	do(t, r, req{method: http.MethodDelete, path: "/api/ai/admin/users/" + id, console: vp.Token})

	page := queryAudit(t, r, vp.Token, url.Values{"target_type": {"user"}, "target_id": {id}})
	want := []string{domain.AuditUserDelete, domain.AuditUserResetPassword, domain.AuditUserUpdate, domain.AuditUserCreate}
	if len(page.Items) != len(want) {
		t.Fatalf("want %d events for user, got %+v", len(want), page.Items)
	}
	for i, a := range want {
		if page.Items[i].Action != a {
			t.Fatalf("event %d: want %s got %s", i, a, page.Items[i].Action)
		}
	}
	if c, ok := changeOf(page.Items[2], "role"); !ok || c.Before != "viewer" || c.After != "operator" {
		t.Fatalf("role diff: %+v", page.Items[2].Changes)
	}
	for _, e := range page.Items {
		if strings.Contains(e.Summary, "secret") {
			t.Fatalf("password leaked: %q", e.Summary)
		}
	}
}

// แก้ matrix สิทธิ์ต้องบอกว่า role ไหนได้/เสียสิทธิ์อะไร
func TestAudit_RolePermissionsDiff(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	w := do(t, r, req{method: http.MethodPut, path: "/api/ai/admin/role-permissions", console: vp.Token,
		body: `{"matrix":{"admin":["office.view","office.edit","office.delete","user.manage","audit.view"],"operator":["office.view","office.edit","office.delete"],"viewer":["office.view","audit.view"]}}`})
	if w.Code != http.StatusOK {
		t.Fatalf("put matrix: %d %s", w.Code, w.Body.String())
	}
	page := queryAudit(t, r, vp.Token, url.Values{"action": {domain.AuditRolePermissions}})
	if page.Total != 1 {
		t.Fatalf("want 1 matrix event, got %d", page.Total)
	}
	e := page.Items[0]
	if len(e.Changes) != 1 || e.Changes[0].Field != "viewer" {
		t.Fatalf("only viewer changed: %+v", e.Changes)
	}
}

// คนไม่มีสิทธิ์ audit.view เปิดประวัติไม่ได้ และความพยายามนั้นถูกบันทึกไว้
func TestAudit_ViewerDeniedAndRecorded(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	w := do(t, r, req{method: http.MethodGet, path: auditBase, console: mintJWT(t, domain.RoleViewer)})
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer audit: want 403 got %d", w.Code)
	}
	page := queryAudit(t, r, vp.Token, url.Values{"action": {domain.AuditAccessDenied}})
	if page.Total != 1 || page.Items[0].Actor != "viewer1" || page.Items[0].Meta["permission"] != "audit.view" {
		t.Fatalf("access.denied: %+v", page.Items)
	}
}

// แบ่งหน้า + ค้นข้อความ + รายชื่อผู้กระทำ + from/to ผิดรูปแบบ
func TestAudit_PaginationSearchActors(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")
	for _, l := range []string{"a1", "a2", "a3", "a4", "a5"} {
		do(t, r, req{method: http.MethodPatch, path: "/api/ai/admin/offices/demo", console: vp.Token, body: `{"label":"` + l + `"}`})
	}

	p1 := queryAudit(t, r, vp.Token, url.Values{"category": {"office"}, "page_size": {"2"}})
	p3 := queryAudit(t, r, vp.Token, url.Values{"category": {"office"}, "page_size": {"2"}, "page": {"3"}})
	if p1.Total != 5 || len(p1.Items) != 2 || len(p3.Items) != 1 {
		t.Fatalf("pagination: total=%d p1=%d p3=%d", p1.Total, len(p1.Items), len(p3.Items))
	}
	if c, _ := changeOf(p1.Items[0], "label"); c.After != "a5" {
		t.Fatalf("newest first: %+v", p1.Items[0].Changes)
	}

	q := queryAudit(t, r, vp.Token, url.Values{"q": {"ตั้งค่าระบบครั้งแรก"}})
	if q.Total != 1 || q.Items[0].Action != domain.AuditAuthSetup {
		t.Fatalf("search: %+v", q.Items)
	}

	w := do(t, r, req{method: http.MethodGet, path: auditBase + "/actors", console: vp.Token})
	actors := payload[struct {
		Actors []string `json:"actors"`
	}](t, w).Actors
	if len(actors) != 1 || actors[0] != "earth" {
		t.Fatalf("actors: %+v", actors)
	}

	bad := do(t, r, req{method: http.MethodGet, path: auditBase + "?from=yesterday", console: vp.Token})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad from: want 400 got %d", bad.Code)
	}
}

// logout ต้องบันทึกในนามคนที่ถือ token
func TestAudit_Logout(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")
	w := do(t, r, req{method: http.MethodPost, path: authBase + "/logout", console: vp.Token})
	if w.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", w.Code, w.Body.String())
	}
	page := queryAudit(t, r, vp.Token, url.Values{"action": {domain.AuditAuthLogout}})
	if page.Total != 1 || page.Items[0].Actor != "earth" {
		t.Fatalf("logout event: %+v", page.Items)
	}
}

// ช่วงวันที่: กว้างเกิน 2 เดือน / from หลัง to → 400 · ค้น actor ไม่สนตัวพิมพ์
func TestAudit_DateRangeLimitAndActorCase(t *testing.T) {
	r := newRouter(t)
	vp := registerAndEnroll(t, r, "earth", "secret12")

	now := time.Now().UTC()
	for _, tc := range []struct {
		name     string
		from, to time.Time
	}{
		{"too wide", now.AddDate(0, 0, -63), now},
		{"inverted", now, now.Add(-time.Hour)},
	} {
		path := auditBase + "?" + url.Values{
			"from": {tc.from.Format(time.RFC3339)}, "to": {tc.to.Format(time.RFC3339)},
		}.Encode()
		if w := do(t, r, req{method: http.MethodGet, path: path, console: vp.Token}); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: want 400 got %d", tc.name, w.Code)
		}
	}

	p := queryAudit(t, r, vp.Token, url.Values{"actor": {"EARTH"}})
	if p.Total == 0 {
		t.Fatal("ค้น actor ตัวพิมพ์ใหญ่ต้องเจอรายการของ earth")
	}
}
