package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

const authBase = "/api/ai/admin/auth"

type statusPayload struct {
	NeedsSetup bool `json:"needs_setup"`
}

type stagePayload struct {
	Stage      string `json:"stage"`
	Ticket     string `json:"ticket"`
	OTPAuthURI string `json:"otpauth_uri"`
	Secret     string `json:"secret"`
	Token      string `json:"token"`
}

type verifyPayload struct {
	Token         string             `json:"token"`
	User          domain.ConsoleUser `json:"user"`
	RecoveryCodes []string           `json:"recovery_codes"`
}

func mintJWT(t *testing.T, role domain.Role) string {
	t.Helper()
	issuer := auth.NewJWTIssuer(consoleJWTSecret, time.Hour)
	tok, err := issuer.Issue(port.Claims{UserID: "u-" + string(role), Username: string(role) + "1", Role: role})
	if err != nil {
		t.Fatalf("mint jwt: %v", err)
	}
	return tok
}

// registerAndEnroll ทำ register + verify enroll code ให้ครบ 1 admin คนแรก คืน token+ticket ที่ผ่านแล้ว
func registerAndEnroll(t *testing.T, r http.Handler, username, password string) verifyPayload {
	t.Helper()
	w := do(t, r, req{
		method: http.MethodPost,
		path:   authBase + "/register",
		body:   `{"username":"` + username + `","display_name":"Display","password":"` + password + `"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	sp := payload[stagePayload](t, w)
	if sp.Stage != "enroll" || sp.Secret == "" || sp.Ticket == "" {
		t.Fatalf("register should begin enroll: %+v", sp)
	}
	code, err := totp.GenerateCode(sp.Secret, time.Now())
	if err != nil {
		t.Fatalf("gen totp code: %v", err)
	}
	w2 := do(t, r, req{
		method: http.MethodPost,
		path:   authBase + "/totp/verify",
		body:   `{"ticket":"` + sp.Ticket + `","code":"` + code + `"}`,
	})
	if w2.Code != http.StatusOK {
		t.Fatalf("verify enroll: %d %s", w2.Code, w2.Body.String())
	}
	vp := payload[verifyPayload](t, w2)
	if vp.Token == "" || len(vp.RecoveryCodes) != 8 {
		t.Fatalf("verify enroll payload: %+v", vp)
	}
	return vp
}

func TestAuthStatus_NeedsSetupInitiallyTrue(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: authBase + "/status"})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	sp := payload[statusPayload](t, w)
	if !sp.NeedsSetup {
		t.Fatalf("want needs_setup=true, got %+v", sp)
	}
}

func TestAuthRegister_FirstOK_SecondSetupDone(t *testing.T) {
	r := newRouter(t)
	registerAndEnroll(t, r, "earth", "secret12")

	// status now false
	w := do(t, r, req{method: http.MethodGet, path: authBase + "/status"})
	sp := payload[statusPayload](t, w)
	if sp.NeedsSetup {
		t.Fatalf("want needs_setup=false after register, got %+v", sp)
	}

	// second register blocked
	w2 := do(t, r, req{
		method: http.MethodPost,
		path:   authBase + "/register",
		body:   `{"username":"other","display_name":"Other","password":"othersecret"}`,
	})
	if w2.Code != http.StatusConflict {
		t.Fatalf("second register: want 409, got %d %s", w2.Code, w2.Body.String())
	}
	var res struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.Error != "SETUP_DONE" {
		t.Fatalf("second register error code: want SETUP_DONE got %q", res.Error)
	}
}

func TestAuthLoginTotp_WrongCodeInvalid2FA(t *testing.T) {
	r := newRouter(t)
	registerAndEnroll(t, r, "earth", "secret12")

	w := do(t, r, req{
		method: http.MethodPost,
		path:   authBase + "/login",
		body:   `{"username":"earth","password":"secret12"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	sp := payload[stagePayload](t, w)
	if sp.Stage != "totp" || sp.Ticket == "" {
		t.Fatalf("login stage: %+v", sp)
	}

	w2 := do(t, r, req{
		method: http.MethodPost,
		path:   authBase + "/totp/verify",
		body:   `{"ticket":"` + sp.Ticket + `","code":"000000"}`,
	})
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("wrong totp code: want 401, got %d %s", w2.Code, w2.Body.String())
	}
	var res struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.Error != "INVALID_2FA" {
		t.Fatalf("wrong totp code error: want INVALID_2FA got %q", res.Error)
	}
}

func TestOfficeRoutes_RoleGates(t *testing.T) {
	r := newRouter(t)

	operatorTok := mintJWT(t, domain.RoleOperator)
	viewerTok := mintJWT(t, domain.RoleViewer)

	// operator can create office
	w := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/offices",
		body:    `{"id":"newoffice","label":"New Office"}`,
		console: operatorTok,
	})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("operator create office: %d %s", w.Code, w.Body.String())
	}

	// viewer forbidden to create office
	w2 := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/offices",
		body:    `{"id":"anotheroffice","label":"Another"}`,
		console: viewerTok,
	})
	if w2.Code != http.StatusForbidden {
		t.Fatalf("viewer create office: want 403, got %d %s", w2.Code, w2.Body.String())
	}

	// viewer can still list offices
	w3 := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/offices", console: viewerTok})
	if w3.Code != http.StatusOK {
		t.Fatalf("viewer list offices: want 200, got %d %s", w3.Code, w3.Body.String())
	}
}

func TestUsersRoutes_AdminOnly(t *testing.T) {
	r := newRouter(t)

	viewerTok := mintJWT(t, domain.RoleViewer)
	adminTok := mintJWT(t, domain.RoleAdmin)

	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/users", console: viewerTok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer list users: want 403, got %d %s", w.Code, w.Body.String())
	}

	w2 := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/users", console: adminTok})
	if w2.Code != http.StatusOK {
		t.Fatalf("admin list users: want 200, got %d %s", w2.Code, w2.Body.String())
	}
}

type rolePermPayload struct {
	Matrix map[string][]string `json:"matrix"`
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// TestRolePermissions_ConfigurableRBAC ครอบ 3 พฤติกรรมหลักของ RBAC ที่ตั้งค่าได้:
//  1. default matrix: operator ทำ office.edit ได้แต่ user.manage ไม่ได้
//  2. PUT /role-permissions ให้ viewer มี office.edit เพิ่ม แล้ว viewer JWT เดิม POST /offices ได้ทันที
//  3. PUT ที่ไม่ได้ใส่ admin.user.manage มา ก็ยังถูกบังคับใส่ให้อัตโนมัติ (anti-lockout)
func TestRolePermissions_ConfigurableRBAC(t *testing.T) {
	r := newRouter(t)

	operatorTok := mintJWT(t, domain.RoleOperator)
	viewerTok := mintJWT(t, domain.RoleViewer)
	adminTok := mintJWT(t, domain.RoleAdmin)

	// 1. default matrix: operator can office.edit but not user.manage
	w := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/offices",
		body:    `{"id":"rbac-op","label":"RBAC Op"}`,
		console: operatorTok,
	})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("operator office.edit: want 2xx, got %d %s", w.Code, w.Body.String())
	}
	w2 := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/users", console: operatorTok})
	if w2.Code != http.StatusForbidden {
		t.Fatalf("operator user.manage: want 403, got %d %s", w2.Code, w2.Body.String())
	}

	// viewer starts unable to create office (only office.view by default)
	wv := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/offices",
		body:    `{"id":"rbac-viewer-before","label":"Before"}`,
		console: viewerTok,
	})
	if wv.Code != http.StatusForbidden {
		t.Fatalf("viewer before grant: want 403, got %d %s", wv.Code, wv.Body.String())
	}

	// 2. admin grants viewer office.edit, and (deliberately) leaves admin without user.manage
	putBody := `{"matrix":{` +
		`"admin":["office.view"],` +
		`"operator":["office.view","office.edit","office.delete","office.rotate"],` +
		`"viewer":["office.view","office.edit"]` +
		`}}`
	wp := do(t, r, req{
		method:  http.MethodPut,
		path:    "/api/ai/admin/role-permissions",
		body:    putBody,
		console: adminTok,
	})
	if wp.Code != http.StatusOK {
		t.Fatalf("put role-permissions: %d %s", wp.Code, wp.Body.String())
	}
	putResp := payload[rolePermPayload](t, wp)
	if !contains(putResp.Matrix["admin"], "user.manage") {
		t.Fatalf("PUT response should have force-added admin.user.manage, got %+v", putResp.Matrix)
	}

	// viewer JWT (unchanged) can now POST /offices
	wv2 := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/offices",
		body:    `{"id":"rbac-viewer-after","label":"After"}`,
		console: viewerTok,
	})
	if wv2.Code != http.StatusCreated && wv2.Code != http.StatusOK {
		t.Fatalf("viewer after grant office.edit: want 2xx, got %d %s", wv2.Code, wv2.Body.String())
	}

	// 3. anti-lockout: stored matrix still has admin.user.manage even though PUT body omitted it
	wg := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/role-permissions", console: adminTok})
	if wg.Code != http.StatusOK {
		t.Fatalf("admin get role-permissions: want 200, got %d %s", wg.Code, wg.Body.String())
	}
	getResp := payload[rolePermPayload](t, wg)
	if !contains(getResp.Matrix["admin"], "user.manage") {
		t.Fatalf("anti-lockout failed: admin matrix missing user.manage: %+v", getResp.Matrix)
	}
}

func TestBreakGlass_StillWorksForOfficeWrite(t *testing.T) {
	r := newRouter(t)

	w := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/offices",
		body:    `{"id":"bgoffice","label":"BG Office"}`,
		console: consoleToken,
	})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("break-glass create office: %d %s", w.Code, w.Body.String())
	}
}

type roleDefPayload struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Builtin bool   `json:"builtin"`
}

type roleConfigPayload struct {
	Roles   []roleDefPayload    `json:"roles"`
	Matrix  map[string][]string `json:"matrix"`
	Catalog []map[string]string `json:"catalog"`
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var res struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode error code: %v · body=%s", err, w.Body.String())
	}
	return res.Error
}

func hasRole(roles []roleDefPayload, key string) bool {
	for _, r := range roles {
		if r.Key == key {
			return true
		}
	}
	return false
}

// TestDynamicRoles_AddRoleThenAssignToUser ครอบ happy path หลักของ role แบบ dynamic:
// แอดมินสร้าง role ใหม่ผ่าน POST /roles แล้วเอา role นั้นไปสร้าง console user ได้ทันที
// (role validity ใหม่ = "อยู่ใน roles list" ไม่ใช่ fixed enum เดิม)
func TestDynamicRoles_AddRoleThenAssignToUser(t *testing.T) {
	r := newRouter(t)
	adminTok := mintJWT(t, domain.RoleAdmin)

	wAdd := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/roles",
		body:    `{"key":"support","label":"ฝ่ายสนับสนุน"}`,
		console: adminTok,
	})
	if wAdd.Code != http.StatusCreated {
		t.Fatalf("add role: want 201, got %d %s", wAdd.Code, wAdd.Body.String())
	}
	addResp := payload[roleConfigPayload](t, wAdd)
	if !hasRole(addResp.Roles, "support") {
		t.Fatalf("new role missing from roles list: %+v", addResp.Roles)
	}

	wUser := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/users",
		body:    `{"username":"supp1","display_name":"Supp","role":"support","password":"supportpw1"}`,
		console: adminTok,
	})
	if wUser.Code != http.StatusOK {
		t.Fatalf("create user with new role: want 200, got %d %s", wUser.Code, wUser.Body.String())
	}
}

// TestDynamicRoles_DeleteRoleInUse: ลบ role ที่ยังมี user ใช้อยู่ต้องโดน 409 ROLE_IN_USE
// ก่อนจะไปถึงขั้นเช็ค builtin ด้วยซ้ำ
func TestDynamicRoles_DeleteRoleInUse(t *testing.T) {
	r := newRouter(t)
	adminTok := mintJWT(t, domain.RoleAdmin)

	wAdd := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/roles",
		body:    `{"key":"support","label":"ฝ่ายสนับสนุน"}`,
		console: adminTok,
	})
	if wAdd.Code != http.StatusCreated {
		t.Fatalf("add role: %d %s", wAdd.Code, wAdd.Body.String())
	}
	wUser := do(t, r, req{
		method:  http.MethodPost,
		path:    "/api/ai/admin/users",
		body:    `{"username":"supp2","display_name":"Supp2","role":"support","password":"supportpw1"}`,
		console: adminTok,
	})
	if wUser.Code != http.StatusOK {
		t.Fatalf("create user: %d %s", wUser.Code, wUser.Body.String())
	}

	wDel := do(t, r, req{
		method:  http.MethodDelete,
		path:    "/api/ai/admin/roles/support",
		console: adminTok,
	})
	if wDel.Code != http.StatusConflict {
		t.Fatalf("delete role in use: want 409, got %d %s", wDel.Code, wDel.Body.String())
	}
	if code := errCode(t, wDel); code != "ROLE_IN_USE" {
		t.Fatalf("delete role in use error code: want ROLE_IN_USE got %q", code)
	}
}

// TestDynamicRoles_AdminBuiltinLocked: admin เปลี่ยนชื่อ/ลบไม่ได้เด็ดขาด (builtin)
func TestDynamicRoles_AdminBuiltinLocked(t *testing.T) {
	r := newRouter(t)
	adminTok := mintJWT(t, domain.RoleAdmin)

	wRename := do(t, r, req{
		method:  http.MethodPatch,
		path:    "/api/ai/admin/roles/admin",
		body:    `{"label":"หัวหน้าใหญ่"}`,
		console: adminTok,
	})
	if wRename.Code != http.StatusForbidden {
		t.Fatalf("rename admin: want 403, got %d %s", wRename.Code, wRename.Body.String())
	}
	if code := errCode(t, wRename); code != "ROLE_LOCKED" {
		t.Fatalf("rename admin error code: want ROLE_LOCKED got %q", code)
	}

	wDel := do(t, r, req{
		method:  http.MethodDelete,
		path:    "/api/ai/admin/roles/admin",
		console: adminTok,
	})
	if wDel.Code != http.StatusForbidden {
		t.Fatalf("delete admin: want 403, got %d %s", wDel.Code, wDel.Body.String())
	}
	if code := errCode(t, wDel); code != "ROLE_LOCKED" {
		t.Fatalf("delete admin error code: want ROLE_LOCKED got %q", code)
	}
}
