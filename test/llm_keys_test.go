package test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	cryptobox "ai-office-backend/internal/adapter/crypto"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	llmrouter "ai-office-backend/internal/adapter/llm/router"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

const (
	goodKey = "sk-test-good-key-0000a1b2"
	newKey  = "sk-test-new-key-00000c3d4"
	badKey  = "sk-test-bad-key-00000dead"
)

func newTestBox(t *testing.T) port.SecretBox {
	t.Helper()
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	b, err := cryptobox.NewAESGCM(base64.StdEncoding.EncodeToString(k), 1)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type keyEnv struct {
	r       http.Handler
	keys    *service.LLMKeyService
	file    string
	token   string // session ของ admin จริงที่มี 2FA
	secret  string // TOTP secret ของ admin คนนั้น
	testers int    // จำนวนครั้งที่ลองยิง provider
}

func (e *keyEnv) code(t *testing.T) string {
	t.Helper()
	c, err := totp.GenerateCode(e.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// keyServer — admin จริง (สมัคร + ตั้ง 2FA) · tester ปลอม: key ที่ขึ้นต้นด้วย badKey = provider ปฏิเสธ (และพ่น key กลับมาใน error)
func keyServer(t *testing.T) *keyEnv {
	t.Helper()
	d := newDeps(t)
	e := &keyEnv{file: filepath.Join(t.TempDir(), "llm_credentials.json")}
	e.keys = service.NewLLMKeyService(filestore.NewLLMCredentialRepository(e.file), newTestBox(t), nil, d.Auth, d.Audit)
	rt := llmrouter.New(e.keys.Key, func() domain.LLMSettings { return d.Settings.Current().LLM })
	d.Settings.SetLLMKeyCheck(rt.HasKey)
	e.keys.SetCurrent(func() domain.LLMSettings { return d.Settings.Current().LLM })
	e.keys.SetTester(func(_ context.Context, _ domain.LLMSettings, key string) (port.LLMTestResult, error) {
		e.testers++
		if key == badKey {
			return port.LLMTestResult{}, errors.New("401 invalid x-api-key: " + key)
		}
		return port.LLMTestResult{Reply: "OK"}, nil
	})
	d.LLM, d.LLMKeys = rt, e.keys
	e.r = httpgin.NewTestRouter(d)

	w := do(t, e.r, req{method: http.MethodPost, path: authBase + "/register",
		body: `{"username":"keyadmin","display_name":"Key Admin","password":"correct-horse-1"}`})
	sp := payload[stagePayload](t, w)
	e.secret = sp.Secret
	w = do(t, e.r, req{method: http.MethodPost, path: authBase + "/totp/verify",
		body: `{"ticket":"` + sp.Ticket + `","code":"` + e.code(t) + `"}`})
	e.token = payload[verifyPayload](t, w).Token
	if e.token == "" {
		t.Fatalf("login admin ไม่สำเร็จ: %s", w.Body.String())
	}
	return e
}

func (e *keyEnv) put(t *testing.T, provider, key, code, token string) (int, string) {
	t.Helper()
	w := do(t, e.r, req{method: http.MethodPut, path: "/api/ai/admin/settings/llm/keys/" + provider, console: token,
		body: `{"key":"` + key + `","code":"` + code + `"}`})
	return w.Code, w.Body.String()
}

func TestLLMKeys_SetRequires2FAAndNeverLeaks(t *testing.T) {
	e := keyServer(t)

	// ไม่มีรหัส / รหัสผิด = ไม่บันทึก และไม่ลองยิง provider
	if code, body := e.put(t, "anthropic", goodKey, "000000", e.token); code != http.StatusBadRequest || !strings.Contains(body, "INVALID_2FA") {
		t.Fatalf("รหัสผิดต้อง 400 INVALID_2FA ได้ %d %s", code, body)
	}
	if e.testers != 0 || e.keys.HasKey("anthropic") {
		t.Fatal("2FA ไม่ผ่านต้องไม่แตะ key")
	}

	code, body := e.put(t, "anthropic", goodKey, e.code(t), e.token)
	if code != 200 || strings.Contains(body, goodKey) || !strings.Contains(body, `"last4":"a1b2"`) {
		t.Fatalf("ตั้ง key = %d %s", code, body)
	}
	if e.keys.Key("anthropic") != goodKey {
		t.Fatal("แชทต้องใช้ key ใหม่ทันที")
	}

	// หน้าเว็บเห็นแค่ 4 ตัวท้าย
	w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/settings", console: e.token})
	if strings.Contains(w.Body.String(), goodKey) || !strings.Contains(w.Body.String(), `"last4":"a1b2"`) {
		t.Fatalf("settings เปิดเผย key หรือไม่มี last4: %s", w.Body.String())
	}
	// ไฟล์เก็บแต่ ciphertext
	raw, _ := os.ReadFile(e.file)
	if len(raw) == 0 || strings.Contains(string(raw), goodKey) {
		t.Fatal("ไฟล์ต้องไม่มี key แบบอ่านออก")
	}
	// ประวัติเห็นแค่ ••••a1b2
	page := queryAudit(t, e.r, e.token, url.Values{"action": {domain.AuditLLMKeySet}})
	if page.Total != 1 || !strings.Contains(page.Items[0].Summary, "••••a1b2") || strings.Contains(page.Items[0].Summary, goodKey) {
		t.Fatalf("audit = %+v", page.Items)
	}
}

func TestLLMKeys_BadKeyKeepsOldAndRedactsError(t *testing.T) {
	e := keyServer(t)
	if code, body := e.put(t, "anthropic", goodKey, e.code(t), e.token); code != 200 {
		t.Fatalf("ตั้ง key แรก = %d %s", code, body)
	}
	code, body := e.put(t, "anthropic", badKey, e.code(t), e.token)
	if code != 400 || strings.Contains(body, badKey) || !strings.Contains(body, "ยังไม่บันทึก") {
		t.Fatalf("key เสีย = %d %s", code, body)
	}
	if e.keys.Key("anthropic") != goodKey {
		t.Fatal("key เสียต้องไม่ทับ key เดิม")
	}
	// เปลี่ยนเป็น key ใหม่ที่ใช้ได้ — ประวัติบอก 4 ตัวท้ายเดิม → ใหม่
	if code, body := e.put(t, "anthropic", newKey, e.code(t), e.token); code != 200 {
		t.Fatalf("เปลี่ยน key = %d %s", code, body)
	}
	page := queryAudit(t, e.r, e.token, url.Values{"action": {domain.AuditLLMKeySet}})
	if !strings.Contains(page.Items[0].Summary, "••••a1b2 → ••••c3d4") {
		t.Fatalf("audit เปลี่ยน key = %+v", page.Items[0])
	}
}

func TestLLMKeys_WhoCanManage(t *testing.T) {
	e := keyServer(t)
	// break-glass ไม่มีตัวตนจริง ยืนยัน 2FA ไม่ได้
	if code, body := e.put(t, "anthropic", goodKey, e.code(t), consoleToken); code != http.StatusForbidden || !strings.Contains(body, "STEP_UP_REQUIRED") {
		t.Fatalf("break-glass = %d %s", code, body)
	}
	// viewer ไม่มีสิทธิ์ llm.key.manage
	if code, _ := e.put(t, "anthropic", goodKey, e.code(t), viewerToken(t)); code != http.StatusForbidden {
		t.Fatalf("viewer = %d", code)
	}
}

func TestLLMKeys_DeleteRules(t *testing.T) {
	e := keyServer(t)
	e.put(t, "anthropic", goodKey, e.code(t), e.token)
	e.put(t, "gemini", newKey, e.code(t), e.token)

	del := func(provider, code string) (int, string) {
		w := do(t, e.r, req{method: http.MethodDelete, path: "/api/ai/admin/settings/llm/keys/" + provider, console: e.token,
			body: `{"code":"` + code + `"}`})
		return w.Code, w.Body.String()
	}
	// anthropic คือโมเดลที่แชทใช้อยู่ (ค่าเริ่มต้น) — ลบไม่ได้
	if code, body := del("anthropic", e.code(t)); code != http.StatusConflict {
		t.Fatalf("ลบ key ที่ใช้อยู่ = %d %s", code, body)
	}
	if code, body := del("gemini", e.code(t)); code != 200 || e.keys.HasKey("gemini") {
		t.Fatalf("ลบ gemini = %d %s", code, body)
	}
}

func TestLLMKeys_WrongCodeFiveTimesLocks(t *testing.T) {
	e := keyServer(t)
	var last int
	for i := 0; i < 5; i++ {
		last, _ = e.put(t, "anthropic", goodKey, "000000", e.token)
	}
	if last != http.StatusLocked {
		t.Fatalf("ครั้งที่ 5 ต้องล็อก (423) ได้ %d", last)
	}
	// ล็อกแล้วรหัสถูกก็ไม่ผ่าน
	if code, _ := e.put(t, "anthropic", goodKey, e.code(t), e.token); code != http.StatusLocked {
		t.Fatalf("ระหว่างล็อกต้อง 423 ได้ %d", code)
	}
}

func TestLLMKeys_ImportEnvOnceAndBrokenSecret(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "llm_credentials.json")
	repo := filestore.NewLLMCredentialRepository(file)
	box := newTestBox(t)

	first := service.NewLLMKeyService(repo, box, map[string]string{"anthropic": goodKey}, nil, nil)
	_ = first.Load(ctx)
	first.ImportEnv(ctx)
	if first.Key("anthropic") != goodKey || first.Info("anthropic").Source != "db" {
		t.Fatalf("ย้ายจาก .env = %+v", first.Info("anthropic"))
	}

	// เปิดใหม่: .env เปลี่ยน แต่ DB มีแล้ว → ใช้ของ DB
	again := service.NewLLMKeyService(repo, box, map[string]string{"anthropic": newKey}, nil, nil)
	_ = again.Load(ctx)
	again.ImportEnv(ctx)
	if again.Key("anthropic") != goodKey {
		t.Fatal("DB มี key แล้วต้องไม่อ่าน .env")
	}

	// กุญแจหลักเปลี่ยน = ถอดไม่ได้ ถือว่าไม่มี key
	broken := service.NewLLMKeyService(repo, newTestBox(t), nil, nil, nil)
	_ = broken.Load(ctx)
	if info := broken.Info("anthropic"); !info.Broken || info.HasKey || broken.HasKey("anthropic") {
		t.Fatalf("กุญแจผิด = %+v", info)
	}
}

// openai ต้องมี base_url — ไม่มีต้องบอกทันที ก่อนยืนยัน 2FA (ไม่นับเป็นครั้งที่ผิด)
func TestLLMKeys_MissingBaseURLCheckedBefore2FA(t *testing.T) {
	e := keyServer(t)
	for i := 0; i < 6; i++ {
		code, body := e.put(t, "openai", goodKey, "000000", e.token)
		if code != http.StatusBadRequest || !strings.Contains(body, "base_url") {
			t.Fatalf("ครั้งที่ %d = %d %s", i+1, code, body)
		}
	}
	// ถ้านับเป็นครั้งที่ผิด บัญชีจะล็อกไปแล้ว — ต้องยังตั้ง key ได้เมื่อส่ง base_url มา
	w := do(t, e.r, req{method: http.MethodPut, path: "/api/ai/admin/settings/llm/keys/openai", console: e.token,
		body: `{"key":"` + goodKey + `","code":"` + e.code(t) + `","base_url":"https://api.example.com/v1"}`})
	if w.Code != 200 {
		t.Fatalf("ส่ง base_url แล้ว = %d %s", w.Code, w.Body.String())
	}
}
