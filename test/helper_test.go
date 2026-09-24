package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/cache/memory"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/host/hostapi"
	"ai-office-backend/internal/adapter/storage/filestore"
	memstore "ai-office-backend/internal/adapter/storage/memory"
	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

const (
	consoleToken     = "test-console-token"
	officeOrigin     = "http://office.test"
	demoKey          = "pk_demo_test"
	consoleJWTSecret = "test-console-jwt-secret"
	ticketSecret     = "test-ticket-secret-0123456789abcdef-xyz"
	testKind         = "sample-kind"

	// secret_key ของ service ใน demoOffice (ตัวจริงอยู่ฝั่ง host · DB เก็บแค่ hash)
	secretK11S = "aisk_test_secret_k11s_0000000000000000000000000"
	secretPG99 = "aisk_test_secret_pg99_0000000000000000000000000"
)

// demoOffice: 1 office 2 service — K11S เปิดอยู่ (allowlist [adm_ploy], allow_all=false), PG99 ปิด
func demoOffice() domain.Office {
	o := domain.NewOffice("demo", "หลังบ้านจำลอง")
	o.PublicKey = demoKey
	o.Kind = testKind
	o.AllowedOrigins = []string{officeOrigin}

	k := domain.DefaultService("K11S", "เว็บ K11S")
	k.Enabled = true
	k.AllowAll = false
	k.Allowlist = []string{"adm_ploy"}
	k.SecretKeyHash = domain.HashSecret(secretK11S)

	pg := domain.DefaultService("PG99", "เว็บ PG99")
	pg.Allowlist = []string{"adm_ploy"}
	pg.AllowAll = false
	pg.SecretKeyHash = domain.HashSecret(secretPG99)

	o.Services = []domain.Service{k, pg}
	return o
}

// ---------- env ----------

type env struct {
	t       *testing.T
	r       *gin.Engine
	host    *mockHost
	llm     *fakeLLM
	msgs    *memstore.Messages
	convs   *memstore.Conversations
	access  *memstore.AccessLogs
	quotas  *memstore.Quotas
	rollups *memstore.Rollups
	offices port.OfficeRepository
	cnt     *memory.Store
	now     *clock
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.t.IsZero() {
		return time.Now()
	}
	return c.t
}

func (c *clock) Set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

func loadConnectors(t *testing.T) connector.Registry {
	t.Helper()
	conns, err := connector.LoadDir(filepath.Join("..", "internal", "core", "connector", "testdata"))
	if err != nil {
		t.Fatalf("load connectors: %v", err)
	}
	return connector.Registry(conns)
}

func newEnv(t *testing.T, seed ...domain.Office) *env {
	t.Helper()
	return newEnvWith(t, nil, seed...)
}

// newEnvWith = newEnv ที่เลือกโมเดลได้ (nil = fakeLLM) — ใช้กับ eval ของจริง (eval_live_test.go)
func newEnvWith(t *testing.T, llm port.LLM, seed ...domain.Office) *env {
	t.Helper()
	if len(seed) == 0 {
		seed = []domain.Office{demoOffice()}
	}
	host := newMockHost(t)
	for i := range seed {
		if seed[i].BackofficeAPIURL == "" {
			seed[i].BackofficeAPIURL = host.srv.URL
		}
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

	e := &env{t: t, host: host, llm: newFakeLLM(), msgs: memstore.NewMessages(), convs: memstore.NewConversations(),
		access: &memstore.AccessLogs{}, quotas: memstore.NewQuotas(), rollups: memstore.NewRollups(), offices: repo,
		cnt: memory.New(), now: &clock{}}
	registry := loadConnectors(t)
	settings := service.NewSettingsService(&memstore.Settings{})
	quota := service.NewQuotaService(e.cnt, e.quotas, settings, nil, e.now.Now)
	tickets := auth.NewAccessTicketIssuer(ticketSecret)
	sealer, _ := auth.NewGrantSealer(ticketSecret)
	officeSvc := service.NewOfficeService(service.OfficeDeps{
		Repo: repo, Connectors: registry, Settings: settings, Tickets: tickets, Sealer: sealer, Quota: quota, Now: e.now.Now, Audit: auditSvc,
	})
	hc := hostapi.New()
	runner := &service.ToolRunner{Host: hc, Scoped: service.NewScopedTokenSource(hc, sealer, e.now.Now), Cache: e.cnt.Cache(), Now: e.now.Now}
	chat := service.NewChatService(service.ChatDeps{
		Connectors: registry, Settings: settings, Quota: quota, LLM: pickLLM(llm, e.llm), Runner: runner,
		Convs: e.convs, Msgs: e.msgs, Rollups: e.rollups, Sem: e.cnt, Now: e.now.Now,
	})
	admin := &service.AdminService{Offices: officeSvc, Convs: e.convs, Msgs: e.msgs, Verifs: memstore.NewVerifications(),
		Access: e.access, Deletes: &memstore.Deletions{}, Rollups: e.rollups, Quota: quota, Settings: settings, Now: e.now.Now}

	e.r = httpgin.NewTestRouter(httpgin.Deps{
		OfficeService: officeSvc,
		Connectors:    registry,
		Tickets:       tickets,
		Chat:          chat,
		Admin:         admin,
		Settings:      settings,
		Rollups:       e.rollups,
		BundlePath:    filepath.Join(t.TempDir(), "missing.js"),
		Tokens:        auth.NewJWTIssuer(consoleJWTSecret, time.Hour),
		Auth:          authSvc,
		Permissions:   permSvc,
		Audit:         auditSvc,
		ConsoleToken:  consoleToken,
	})
	return e
}

func pickLLM(real port.LLM, fake *fakeLLM) port.LLM {
	if real != nil {
		return real
	}
	return fake
}

// newRouter คง signature เดิมไว้ให้ test ของทีม (console/office)
func newRouter(t *testing.T, seed ...domain.Office) *gin.Engine {
	return newEnv(t, seed...).r
}

// ---------- http helper ----------

type req struct {
	method  string
	path    string
	body    string
	origin  string
	token   string // Bearer ตั๋ว (ฝั่ง widget)
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

func basePath(key, serviceID string) string {
	return fmt.Sprintf("/api/ai/office/%s/service/%s", key, serviceID)
}

func bootPath(key, serviceID string) string { return basePath(key, serviceID) + "/bootstrap" }

// ---------- ตั๋ว ----------

// fakeGrant = JWT รูปเดียวกับที่ host ออก (backend ไม่ตรวจลายเซ็น grant — host เป็นผู้ตรวจตอนออกกุญแจดอกเล็ก)
func fakeGrant(sub, svc string, exp time.Time) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	p, _ := json.Marshal(map[string]any{"typ": "ai-grant", "sub": sub, "svc": svc, "exp": exp.Unix()})
	return h + "." + base64.RawURLEncoding.EncodeToString(p) + ".sig"
}

type hostUser struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
	Level       int      `json:"level"`
}

func adminUser(perms ...string) hostUser {
	if perms == nil {
		perms = []string{"W_VIEW", "M_VIEW"}
	}
	return hostUser{ID: "emp_1", Username: "adm_ploy", DisplayName: "พลอย", Permissions: perms, Level: 5}
}

// sessionReq = host ยิง /session (server-to-server)
func sessionReq(t *testing.T, r http.Handler, key, sid, secret string, u hostUser, kind string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"kind": kind, "user": u, "grant": fakeGrant(u.ID, sid, time.Now().Add(30*time.Minute))})
	return do(t, r, req{method: http.MethodPost, path: basePath(key, sid) + "/session", body: string(body),
		headers: [][2]string{{"X-AI-Secret", secret}}})
}

func ticketFor(t *testing.T, r http.Handler, key, sid, secret string, u hostUser) string {
	t.Helper()
	w := sessionReq(t, r, key, sid, secret, u, testKind)
	if w.Code != http.StatusOK {
		t.Fatalf("ขอตั๋วไม่ได้: %d %s", w.Code, w.Body.String())
	}
	return payload[port.SessionResult](t, w).Ticket
}

func getBoot(t *testing.T, r http.Handler, key, serviceID, ticket string) (int, domain.Bootstrap, string) {
	t.Helper()
	w := do(t, r, req{method: http.MethodGet, path: bootPath(key, serviceID), origin: officeOrigin, token: ticket})
	return w.Code, payload[domain.Bootstrap](t, w), w.Body.String()
}

// ---------- SSE ----------

type sseEvent struct {
	Type string
	Data map[string]any
}

func chatReq(t *testing.T, r http.Handler, key, sid, ticket, body string) []sseEvent {
	t.Helper()
	w := do(t, r, req{method: http.MethodPost, path: basePath(key, sid) + "/chat", origin: officeOrigin, token: ticket, body: body})
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("ต้องเป็น SSE ได้ %d %q %s", w.Code, ct, w.Body.String())
	}
	var out []sseEvent
	for _, block := range strings.Split(w.Body.String(), "\n\n") {
		var ev sseEvent
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				ev.Type = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev.Data)
			}
		}
		if ev.Type != "" {
			out = append(out, ev)
		}
	}
	return out
}

func ask(t *testing.T, e *env, ticket, text string) []sseEvent {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"text": text})
	return chatReq(t, e.r, demoKey, "K11S", ticket, string(b))
}

func eventsOf(evs []sseEvent, typ string) []sseEvent {
	var out []sseEvent
	for _, e := range evs {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

func joinedTokens(evs []sseEvent) string {
	var b strings.Builder
	for _, e := range eventsOf(evs, "token") {
		b.WriteString(fmt.Sprint(e.Data["text"]))
	}
	return b.String()
}

// ---------- fake LLM (สคริปต์ · ไม่ยิง API จริง) ----------

type fakeLLM struct {
	mu        sync.Mutex
	decide    func(text string) []port.LLMBlock // รอบ 1
	answer    func(results []string) string      // รอบ 2 (ได้ tool_result JSON)
	fail      bool
	requests  []port.LLMRequest
	streamed  int
}

func newFakeLLM() *fakeLLM {
	return &fakeLLM{
		decide: func(text string) []port.LLMBlock {
			switch {
			case strings.Contains(text, "ถอน"):
				return []port.LLMBlock{toolUse("withdraw_pending_by_status", nil)}
			case strings.Contains(text, "สมาชิก"):
				name := strings.TrimSpace(text[strings.LastIndex(text, " ")+1:])
				return []port.LLMBlock{toolUse("member_lookup", map[string]any{"username": name})}
			case strings.Contains(text, "เพิ่มเครดิต"):
				return []port.LLMBlock{toolUse("answer_directly", map[string]any{"category": "action_request"}), toolUse("lookup_menu", map[string]any{"query": "เพิ่มเครดิต"})}
			case strings.Contains(text, "สถานะ"):
				return []port.LLMBlock{toolUse("explain_status", map[string]any{"table": "withdraw", "code": 10.0})}
			default:
				return []port.LLMBlock{toolUse("answer_directly", map[string]any{"category": "greeting"})}
			}
		},
		answer: func([]string) string { return "ตามการ์ดด้านบนครับ" },
	}
}

var toolSeq int

func toolUse(name string, in map[string]any) port.LLMBlock {
	toolSeq++
	if in == nil {
		in = map[string]any{}
	}
	return port.LLMBlock{Type: port.BlockToolUse, ToolUseID: fmt.Sprintf("tu_%d", toolSeq), ToolName: name, Input: in}
}

func (f *fakeLLM) lastUserText(req port.LLMRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		m := req.Messages[i]
		if m.Role == "user" && len(m.Blocks) > 0 && m.Blocks[0].Type == port.BlockText {
			return m.Blocks[0].Text
		}
	}
	return ""
}

func (f *fakeLLM) Complete(_ context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	fail := f.fail
	f.mu.Unlock()
	if fail {
		return port.LLMResponse{}, port.ErrLLMUnavailable
	}
	return port.LLMResponse{Blocks: f.decide(f.lastUserText(req)), StopReason: "tool_use", Usage: domain.Usage{In: 1000, Out: 50}}, nil
}

func (f *fakeLLM) Stream(_ context.Context, req port.LLMRequest, onText func(string)) (port.LLMResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.streamed++
	f.mu.Unlock()
	var results []string
	last := req.Messages[len(req.Messages)-1]
	for _, b := range last.Blocks {
		if b.Type == port.BlockToolResult {
			results = append(results, b.Content)
		}
	}
	text := f.answer(results)
	// ส่งทีละ 3 ตัวอักษร จำลอง stream (ตัวเลขอาจถูกตัดกลาง chunk)
	r := []rune(text)
	for i := 0; i < len(r); i += 3 {
		j := i + 3
		if j > len(r) {
			j = len(r)
		}
		onText(string(r[i:j]))
	}
	return port.LLMResponse{StopReason: "end_turn", Usage: domain.Usage{In: 1200, Out: 80}}, nil
}

// ---------- mock host (contract-host-ai.md §3–§4) ----------

type mockHost struct {
	t   *testing.T
	srv *httptest.Server
	mu  sync.Mutex

	scopedCalls   int
	readCalls     int
	seenHeaders   []http.Header
	grantRejected bool
	down          bool
	slow          time.Duration
	withdrawBody  string
	denyRead      bool
	lastQuery     string
}

func newMockHost(t *testing.T) *mockHost {
	h := &mockHost{t: t, withdrawBody: `{"code":0,"message":"ok","data":{"rows":[{"status":2,"count":7,"amount":142000},{"status":10,"count":2,"amount":18500}]}}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/scoped-token", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.scopedCalls++
		h.seenHeaders = append(h.seenHeaders, r.Header.Clone())
		rejected := h.grantRejected
		h.mu.Unlock()
		var b struct{ Grant string }
		_ = json.NewDecoder(r.Body).Decode(&b)
		if rejected || b.Grant == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"GRANT_INVALID"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"code":0,"message":"SUCCESS","data":{"token":"scoped-abc","expires_at":%d}}`, time.Now().Add(5*time.Minute).Unix())
	})
	read := func(w http.ResponseWriter, r *http.Request, body string) {
		h.mu.Lock()
		h.readCalls++
		h.seenHeaders = append(h.seenHeaders, r.Header.Clone())
		down, slow, deny := h.down, h.slow, h.denyRead
		h.mu.Unlock()
		if slow > 0 {
			time.Sleep(slow)
		}
		if down {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if r.Header.Get("X-AI-Scoped-Token") != "scoped-abc" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"SCOPED_TOKEN_REQUIRED"}`))
			return
		}
		if deny {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":403,"message":"PERMISSION_DENIED"}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}
	mux.HandleFunc("/api/ai/read/withdraw-pending/", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		body := h.withdrawBody
		h.lastQuery = r.URL.RawQuery
		h.mu.Unlock()
		read(w, r, body)
	})
	mux.HandleFunc("/api/ai/read/member/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") == "somchai99" {
			read(w, r, `{"code":0,"message":"ok","data":{"username":"somchai99","date_regis":"2026-01-05T03:00:00Z"}}`)
			return
		}
		if r.URL.Query().Get("username") == "gone404" {
			// host จริงทั้งสองตอบ "ไม่พบ" เป็น HTTP 404 + body code 404
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":404,"message":"NOT_FOUND","data":null}`))
			return
		}
		read(w, r, `{"code":1,"message":"ไม่พบข้อมูล"}`)
	})
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)
	return h
}

func (h *mockHost) set(fn func(h *mockHost)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fn(h)
}
