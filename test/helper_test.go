package test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"ai-office-backend/internal/adapter/auth"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

const (
	consoleToken     = "test-console-token"
	officeOrigin     = "http://office.test"
	demoKey          = "pk_demo_test"
	consoleJWTSecret = "test-console-jwt-secret"
)

// officeJWT สร้าง JWT หลังบ้าน (HS256, payload = {exp, result: EmployeeModel})
// ที่หน้า office เก็บใน localStorage["auth_token"]
//
// backend อ่านตัวตนจาก payload นี้ตรง ๆ (ไม่ verify signature — ดู parseOfficeJWT)
// services: service id -> permission(true/false) ตาม Role.ListService
func officeJWT(admin string, services map[string]bool, perms []string) string {
	return officeJWTExp(admin, services, perms, time.Now().Add(time.Hour).Unix())
}

func officeJWTExp(admin string, services map[string]bool, perms []string, exp int64) string {
	list := make([]map[string]any, 0, len(services))
	for id, ok := range services {
		list = append(list, map[string]any{"service": id, "permission": ok})
	}
	permission := make([]map[string]any, 0, len(perms))
	for _, p := range perms {
		permission = append(permission, map[string]any{"code": p})
	}
	payload := map[string]any{
		"exp": exp,
		"result": map[string]any{
			"id": "emp_1", "username": admin,
			"role": map[string]any{
				"name": "admin", "level": 3,
				"permission":   permission,
				"list_service": list,
			},
		},
	}
	b, _ := json.Marshal(payload)
	seg := base64.RawURLEncoding.EncodeToString(b)
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	return head + "." + seg + ".sig"
}

// demoOffice: 1 office 2 service — K11S เปิดอยู่, PG99 ปิด
func demoOffice() domain.Office {
	o := domain.NewOffice("demo", "หลังบ้านจำลอง")
	o.PublicKey = demoKey
	o.AllowedOrigins = []string{officeOrigin}

	// Allowlist = ด่านคุมการปล่อยรอบแรกของเราเอง
	// คนละเรื่องกับ Role.ListService ที่บอกว่าแอดมินเข้า service ไหนได้
	k := domain.DefaultService("K11S", "เว็บ K11S")
	k.Enabled = true
	k.Allowlist = []string{"adm_ploy"}

	pg := domain.DefaultService("PG99", "เว็บ PG99")
	pg.Allowlist = []string{"adm_ploy"}

	o.Services = []domain.Service{k, pg}
	return o
}

func newRouter(t *testing.T, seed ...domain.Office) *gin.Engine {
	t.Helper()
	return httpgin.NewTestRouter(newDeps(t, seed...))
}

const chatTicketSecret = "test-chat-ticket-secret"

// newDeps ประกอบ Deps ของ router ทั้งชุด — แชทปิดอยู่ (Chat = nil) ให้ test ที่ต้องใช้ใส่เอง
func newDeps(t *testing.T, seed ...domain.Office) httpgin.Deps {
	t.Helper()
	if len(seed) == 0 {
		seed = []domain.Office{demoOffice()}
	}
	repo, err := filestore.NewOfficeRepository(filepath.Join(t.TempDir(), "offices.json"), seed)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cuRepo, err := filestore.NewConsoleUserRepository(filepath.Join(t.TempDir(), "console_users.json"))
	if err != nil {
		t.Fatalf("open console user store: %v", err)
	}
	rmRepo, err := filestore.NewRoleMatrixRepository(filepath.Join(t.TempDir(), "role_permissions.json"))
	if err != nil {
		t.Fatalf("open role permission store: %v", err)
	}
	auditRepo, err := filestore.NewAuditRepository(filepath.Join(t.TempDir(), "audit_logs.jsonl"))
	if err != nil {
		t.Fatalf("open audit store: %v", err)
	}
	auditSvc := service.NewAuditService(auditRepo, time.Now)
	permSvc := service.NewPermissionService(rmRepo, auditSvc)
	authSvc := service.NewConsoleAuth(
		cuRepo,
		auth.NewJWTIssuer(consoleJWTSecret, time.Hour),
		auth.NewTOTPProvider("AI Office Console Test"),
		auth.NewTicketIssuer(consoleJWTSecret, 5*time.Minute),
		permSvc,
		auditSvc,
		time.Now,
	)
	conn, err := connector.Load("../connectors/office-v10x")
	if err != nil {
		t.Fatalf("load connector: %v", err)
	}
	return httpgin.Deps{
		OfficeService: service.NewOfficeService(repo, auditSvc),
		Identity:      service.NewIdentityResolver(50 * time.Millisecond),
		BundlePath:    filepath.Join(t.TempDir(), "missing.js"),
		Tokens:        auth.NewJWTIssuer(consoleJWTSecret, time.Hour),
		Auth:          authSvc,
		Permissions:   permSvc,
		Audit:         auditSvc,
		ConsoleToken:  consoleToken,
		Relay:         service.NewRelay(),
		Tickets:       auth.NewChatTicketIssuer(chatTicketSecret, time.Hour),
		Connector:     conn,
	}
}

type req struct {
	method  string
	path    string
	body    string
	origin  string
	token   string // Bearer ของแอดมิน (ฝั่ง widget)
	console string // Bearer ของคอนโซล
	headers [][2]string
}

func do(t *testing.T, r http.Handler, in req) *httptest.ResponseRecorder {
	t.Helper()
	var httpReq *http.Request
	if in.body != "" {
		httpReq, _ = http.NewRequest(in.method, in.path, bytes.NewBufferString(in.body))
		httpReq.Header.Set("Content-Type", "application/json")
	} else {
		httpReq, _ = http.NewRequest(in.method, in.path, nil)
	}
	httpReq.RemoteAddr = "192.0.2.1:1234" // แบบเดียวกับ httptest.NewRequest — ให้ ClientIP() มีค่า
	if in.origin != "" {
		httpReq.Header.Set("Origin", in.origin)
	}
	if in.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+in.token)
	}
	if in.console != "" {
		httpReq.Header.Set("Authorization", "Bearer "+in.console)
	}
	for _, h := range in.headers {
		if h[0] != "" {
			httpReq.Header.Set(h[0], h[1])
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	return w
}

func payload[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Payload T `json:"payload"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("อ่าน payload ไม่ได้: %v · body=%s", err, w.Body.String())
	}
	return res.Payload
}

// bootPath — widget หา office จาก Origin แล้ว path จึงมีแค่ service
func bootPath(serviceID string) string {
	return fmt.Sprintf("/api/ai/widget/service/%s/bootstrap", serviceID)
}

// legacyBootPath — path ของ snippet รุ่นก่อนที่มี key (key ไม่ถูกใช้แล้ว)
func legacyBootPath(key, serviceID string) string {
	return fmt.Sprintf("/api/ai/office/%s/service/%s/bootstrap", key, serviceID)
}

// adminJWT สร้าง JWT ของแอดมิน officeลูกค้า ที่มีสิทธิ์ service ตามที่ระบุ (permission=true ทุกตัว)
func adminJWT(admin string, services ...string) string {
	m := make(map[string]bool, len(services))
	for _, s := range services {
		m[s] = true
	}
	return officeJWT(admin, m, nil)
}
