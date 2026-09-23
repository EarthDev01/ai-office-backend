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

	httpgin "ai-office-backend/internal/adapter/handler/gin"
	httpreq "ai-office-backend/internal/adapter/handler/http_request"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

const (
	consoleToken = "test-console-token"
	officeOrigin = "http://office.test"
	demoKey      = "pk_demo_test"
)

// officeJWT สร้าง office-api JWT จริง (HS256, payload = {exp, result: EmployeeModel})
// แบบเดียวกับที่ office-v10x เก็บใน localStorage["auth_token"]
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
	if len(seed) == 0 {
		seed = []domain.Office{demoOffice()}
	}
	repo, err := filestore.NewOfficeRepository(filepath.Join(t.TempDir(), "offices.json"), seed)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return httpgin.NewTestRouter(httpgin.Deps{
		OfficeService: service.NewOfficeService(repo),
		Identity: service.NewIdentityResolver(
			httpreq.NewBackofficeClient(3*time.Second), 50*time.Millisecond, true),
		BundlePath:    filepath.Join(t.TempDir(), "missing.js"),
		ConsoleToken:  consoleToken,
		AllowedOrigin: []string{"http://localhost:5173"},
		DevMode:       true,
	})
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

func bootPath(key, serviceID string) string {
	return fmt.Sprintf("/api/ai/office/%s/service/%s/bootstrap", key, serviceID)
}

// devToken สร้าง token ปลอมสำหรับ office ที่ยังไม่ได้ตั้ง backoffice_api_url
func devToken(admin string, services ...string) string {
	out := "dev:" + admin + ":"
	for i, s := range services {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
