package test

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	httpgin "ai-office-backend/internal/adapter/handler/gin"
	llmrouter "ai-office-backend/internal/adapter/llm/router"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/service"
)

// llmServer — มี key เฉพาะ provider ที่ส่งมา (ค่าปลอม ไม่ยิงออกเน็ตในเทสนี้)
func llmServer(t *testing.T, keys map[string]string) http.Handler {
	t.Helper()
	d := newDeps(t)
	rt := llmrouter.New(llmrouter.StaticKeys(keys), func() domain.LLMSettings { return d.Settings.Current().LLM })
	d.Settings.SetLLMKeyCheck(rt.HasKey)
	d.LLM = rt
	return httpgin.NewTestRouter(d)
}

func settingsSince(t *testing.T, r http.Handler) (string, map[string]any) {
	t.Helper()
	code, p := consoleJSON(t, r, "GET", "/api/ai/admin/settings", "", consoleToken)
	if code != 200 {
		t.Fatalf("get settings = %d", code)
	}
	return p["settings"].(map[string]any)["updated_at"].(string), p
}

func TestLLMSettings_ChangeModelValidateAudit(t *testing.T) {
	r := llmServer(t, map[string]string{domain.LLMAnthropic: "k-test"})
	since, p := settingsSince(t, r)

	llm := p["settings"].(map[string]any)["llm"].(map[string]any)
	if llm["provider"] != domain.LLMAnthropic || llm["model"] == "" || p["llm_ready"] != true {
		t.Fatalf("ค่าเริ่มต้น = %v ready=%v", llm, p["llm_ready"])
	}
	provs := p["llm_providers"].([]any)
	if len(provs) != len(domain.LLMProviderCatalog) {
		t.Fatalf("providers = %v", provs)
	}
	for _, x := range provs {
		m := x.(map[string]any)
		if want := m["id"] == domain.LLMAnthropic; m["has_key"] != want {
			t.Fatalf("has_key ของ %v = %v", m["id"], m["has_key"])
		}
	}
	// สถานะ key ส่งได้ แต่ตัว key ห้ามหลุดออกหน้าเว็บ
	if w := do(t, r, req{method: "GET", path: "/api/ai/admin/settings", console: consoleToken}); strings.Contains(w.Body.String(), "k-test") {
		t.Fatal("ห้ามส่ง key ออกหน้าเว็บ")
	}

	// provider ที่ไม่มี key = 400 บอกชื่อตัวแปรที่ต้องใส่
	w := do(t, r, req{method: "PATCH", path: "/api/ai/admin/settings", console: consoleToken,
		body: `{"llm":{"provider":"gemini","model":"gemini-2.5-flash"},"updated_at":"` + since + `"}`})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "GEMINI_API_KEY") {
		t.Fatalf("ไม่มี key = %d %s", w.Code, w.Body.String())
	}
	// openai ไม่มี base_url = 400 (ตรวจรูปแบบก่อนตรวจ key)
	w = do(t, r, req{method: "PATCH", path: "/api/ai/admin/settings", console: consoleToken,
		body: `{"llm":{"provider":"openai","model":"glm-4.6"},"updated_at":"` + since + `"}`})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "base_url") {
		t.Fatalf("ไม่มี base_url = %d %s", w.Code, w.Body.String())
	}
	// effort นอกรายการ = 400
	w = do(t, r, req{method: "PATCH", path: "/api/ai/admin/settings", console: consoleToken,
		body: `{"llm":{"provider":"anthropic","model":"claude-opus-5-5","effort":"turbo"},"updated_at":"` + since + `"}`})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "effort") {
		t.Fatalf("effort ผิด = %d %s", w.Code, w.Body.String())
	}

	// เปลี่ยนโมเดล + ส่ง base_url ติดมา (provider นี้ไม่ใช้) → บันทึก และล้าง base_url ทิ้ง
	code, saved := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings",
		`{"llm":{"provider":"anthropic","model":" claude-opus-5-5 ","effort":"medium","base_url":"https://x"},"updated_at":"`+since+`"}`, consoleToken)
	got := saved["llm"].(map[string]any)
	if code != 200 || got["model"] != "claude-opus-5-5" || got["effort"] != "medium" || got["base_url"] != "" {
		t.Fatalf("บันทึก = %d %v", code, saved)
	}

	// audit แยกเป็นราย field ของ llm
	page := queryAudit(t, r, consoleToken, url.Values{"action": {domain.AuditSettingsUpdate}})
	fields := map[string]bool{}
	for _, c := range page.Items[0].Changes {
		fields[c.Field] = true
	}
	if page.Total != 1 || !fields["llm.model"] || !fields["llm.effort"] || len(fields) != 2 {
		t.Fatalf("audit = %+v", page.Items)
	}
}

func TestLLMSettings_RecentPerProviderSurvivesSwitch(t *testing.T) {
	r := llmServer(t, map[string]string{domain.LLMAnthropic: "k-a", domain.LLMOpenAI: "k-o"})
	since, _ := settingsSince(t, r)

	code, saved := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings",
		`{"llm":{"provider":"openai","model":"glm-5.3","base_url":"https://api.z.ai/api/coding/paas/v4/"},"updated_at":"`+since+`"}`, consoleToken)
	if code != 200 {
		t.Fatalf("บันทึก openai = %d %v", code, saved)
	}
	code, saved = consoleJSON(t, r, "PATCH", "/api/ai/admin/settings",
		`{"llm":{"provider":"anthropic","model":"claude-opus-5-5","effort":"high"},"updated_at":"`+saved["updated_at"].(string)+`"}`, consoleToken)
	if code != 200 {
		t.Fatalf("บันทึก anthropic = %d %v", code, saved)
	}

	// สลับไป anthropic แล้ว ค่า openai ที่เคยบันทึกยังอยู่ (normalize แล้ว) · ส่ง llm_recent มาใน patch ไม่มีผล
	_, p := settingsSince(t, r)
	recent := p["settings"].(map[string]any)["llm_recent"].(map[string]any)
	o := recent[domain.LLMOpenAI].(map[string]any)
	if o["model"] != "glm-5.3" || o["base_url"] != "https://api.z.ai/api/coding/paas/v4" {
		t.Fatalf("llm_recent.openai = %v", o)
	}
	if a := recent[domain.LLMAnthropic].(map[string]any); a["model"] != "claude-opus-5-5" || a["effort"] != "high" {
		t.Fatalf("llm_recent.anthropic = %v", a)
	}

	// audit ไม่นับ llm_recent เป็นการแก้ไข
	page := queryAudit(t, r, consoleToken, url.Values{"action": {domain.AuditSettingsUpdate}})
	for _, it := range page.Items {
		for _, c := range it.Changes {
			if strings.HasPrefix(c.Field, "llm_recent") {
				t.Fatalf("audit มี llm_recent: %+v", it.Changes)
			}
		}
	}
}

func TestLLMSettings_NoKeyStillEditsOtherFields(t *testing.T) {
	r := llmServer(t, map[string]string{})
	since, p := settingsSince(t, r)
	if p["llm_ready"] != false {
		t.Fatalf("ไม่มี key ต้อง ready=false ได้ %v", p["llm_ready"])
	}
	// ไม่ได้แตะ llm = ไม่ตรวจ key
	if code, _ := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings", `{"max_concurrent":7,"updated_at":"`+since+`"}`, consoleToken); code != 200 {
		t.Fatalf("แก้ค่าอื่นต้องได้ ได้ %d", code)
	}
}

func TestLLMSettings_TestEndpoint(t *testing.T) {
	r := llmServer(t, map[string]string{domain.LLMAnthropic: "k-test"})

	// ไม่มี key = บอกทันที ไม่ยิงออกไป
	w := do(t, r, req{method: "POST", path: "/api/ai/admin/settings/llm/test", console: consoleToken,
		body: `{"provider":"gemini","model":"gemini-2.5-flash"}`})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Gemini") {
		t.Fatalf("test ไม่มี key = %d %s", w.Code, w.Body.String())
	}
	// ค่าไม่ถูกรูปแบบ = 400
	w = do(t, r, req{method: "POST", path: "/api/ai/admin/settings/llm/test", console: consoleToken,
		body: `{"provider":"nope","model":"x"}`})
	if w.Code != 400 {
		t.Fatalf("provider ผิด = %d", w.Code)
	}
	// viewer ไม่มี settings.manage
	if code, _ := consoleJSON(t, r, "POST", "/api/ai/admin/settings/llm/test", `{"provider":"anthropic","model":"claude-sonnet-5"}`, viewerToken(t)); code != http.StatusForbidden {
		t.Fatalf("viewer ต้อง 403 ได้ %d", code)
	}
}

// ค่าจาก .env ใช้จนกว่าจะบันทึกจากคอนโซลครั้งแรก — หลังบันทึกอ่านจาก DB
func TestLLMSettings_SeedFromEnvUntilSaved(t *testing.T) {
	repo := filestore.NewSettingsRepository(filepath.Join(t.TempDir(), "settings.json"))
	svc := service.NewSettingsService(repo, nil)
	svc.SetLLMSeed(domain.LLMSettings{Provider: domain.LLMGemini, Model: "gemini-2.5-pro"})
	ctx := context.Background()
	if got := svc.Get(ctx).LLM; got.Provider != domain.LLMGemini || got.Model != "gemini-2.5-pro" {
		t.Fatalf("seed = %+v", got)
	}
	next := domain.LLMSettings{Provider: domain.LLMAnthropic, Model: "claude-sonnet-5"}
	if _, err := svc.Update(ctx, service.SettingsPatch{LLM: &next, UpdatedAt: svc.Get(ctx).UpdatedAt}, "admin"); err != nil {
		t.Fatal(err)
	}

	// เปิดใหม่ด้วย seed เดิม — ต้องได้ค่าที่บันทึกไว้ ไม่ใช่ seed
	again := service.NewSettingsService(repo, nil)
	again.SetLLMSeed(domain.LLMSettings{Provider: domain.LLMGemini, Model: "gemini-2.5-pro"})
	if got := again.Get(ctx).LLM; got.Provider != domain.LLMAnthropic || got.Effort != "low" {
		t.Fatalf("หลังบันทึก = %+v", got)
	}
}
