package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/adapter/auth"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

func viewerToken(t *testing.T) string {
	tok, _ := auth.NewJWTIssuer(consoleJWTSecret, time.Hour).Issue(port.Claims{UserID: "u1", Username: "viewer1", Role: domain.RoleViewer})
	return tok
}

// ---- ตั้งค่าระบบ ----

func TestSettings_UpdateValidateConflictAudit(t *testing.T) {
	r := newRouter(t)
	code, p := consoleJSON(t, r, "GET", "/api/ai/admin/settings", "", consoleToken)
	if code != 200 {
		t.Fatalf("get = %d", code)
	}
	st := p["settings"].(map[string]any)
	if st["max_concurrent"].(float64) != 5 || p["ranges"] == nil {
		t.Fatalf("ค่าเริ่มต้น = %v", p)
	}
	since := st["updated_at"].(string)

	// ค่านอกช่วง = 400 บอกชื่อ field ไม่ปัดเป็นค่าเริ่มต้นเงียบ ๆ
	w := do(t, r, req{method: "PATCH", path: "/api/ai/admin/settings", console: consoleToken,
		body: `{"max_concurrent":51,"updated_at":"` + since + `"}`})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "max_concurrent") {
		t.Fatalf("นอกช่วง = %d %s", w.Code, w.Body.String())
	}

	code, saved := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings", `{"max_concurrent":8,"history_turns":0,"updated_at":"`+since+`"}`, consoleToken)
	if code != 200 || saved["max_concurrent"].(float64) != 8 || saved["history_turns"].(float64) != 0 {
		t.Fatalf("บันทึก = %d %v", code, saved)
	}

	// ส่ง updated_at เก่า (คนอื่นแก้ไปแล้ว) = 409
	if code, _ := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings", `{"max_concurrent":9,"updated_at":"`+since+`"}`, consoleToken); code != http.StatusConflict {
		t.Fatalf("ต้องได้ 409 ได้ %d", code)
	}

	page := queryAudit(t, r, consoleToken, url.Values{"action": {domain.AuditSettingsUpdate}})
	if page.Total != 1 || len(page.Items[0].Changes) != 2 {
		t.Fatalf("audit = %+v", page.Items)
	}
}

func TestSettings_ViewerReadsButCannotEdit(t *testing.T) {
	r := newRouter(t)
	tok := viewerToken(t)
	if code, _ := consoleJSON(t, r, "GET", "/api/ai/admin/settings", "", tok); code != 200 {
		t.Fatalf("viewer อ่านได้ ได้ %d", code)
	}
	if code, _ := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings", `{"max_concurrent":8}`, tok); code != http.StatusForbidden {
		t.Fatalf("viewer แก้ไม่ได้ ได้ %d", code)
	}
}

// ---- การใช้ token + สรุปรายวัน ----

func consoleGet(t *testing.T, base, path string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest("GET", base+path, nil)
	req.Header.Set("Authorization", "Bearer "+consoleToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out struct {
		Payload map[string]any `json:"payload"`
	}
	_ = json.Unmarshal(raw, &out)
	if res.StatusCode != 200 {
		t.Fatalf("%s = %d %s", path, res.StatusCode, raw)
	}
	return out.Payload
}

func consolePost(t *testing.T, base, path, body string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+consoleToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestUsage_RecordedPerAnswerAndVerification(t *testing.T) {
	llm := &fakeLLM{
		uses:  []port.ToolUse{{ID: "t1", Name: "member_lookup", Input: map[string]any{"username": "somchai01"}}},
		reply: []string{"ดูในการ์ดครับ"},
	}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL, "MEMBER_INFO")
	runChat(t, srv.URL, ticket, "เช็คยูส somchai01", func(service.FetchCommand) string { return memberBody })

	// fakeLLM: รอบ 1 = 10/5 · รอบ 2 = 20/7
	u := consoleGet(t, srv.URL, "/api/ai/admin/usage")
	rows := u["data"].([]any)
	if len(rows) != 1 {
		t.Fatalf("usage = %v", u)
	}
	row := rows[0].(map[string]any)
	if row["questions"].(float64) != 1 || row["input_tokens"].(float64) != 30 || row["output_tokens"].(float64) != 12 {
		t.Fatalf("ตัวนับรายเดือน = %v", row)
	}

	// ตรวจผิด แล้วเปลี่ยนเป็นถูก — สรุปรายวันต้องเหลือ ถูก 1 ผิด 0
	q := consoleGet(t, srv.URL, "/api/ai/admin/verifications/queue")
	id := q["data"].([]any)[0].(map[string]any)["answer"].(map[string]any)["id"].(string)
	if c := consolePost(t, srv.URL, "/api/ai/admin/verifications", `{"message_id":"`+id+`","status":"wrong","error_type":"wrong_menu","correct_answer":"x"}`); c != 200 {
		t.Fatalf("ตรวจผิด = %d", c)
	}
	if c := consolePost(t, srv.URL, "/api/ai/admin/verifications", `{"message_id":"`+id+`","status":"correct"}`); c != 200 {
		t.Fatalf("ตรวจถูก = %d", c)
	}
	day := consoleGet(t, srv.URL, "/api/ai/admin/rollups")["data"].([]any)[0].(map[string]any)
	if day["conversations"].(float64) != 1 || day["questions"].(float64) != 1 || day["correct"].(float64) != 1 || day["wrong"].(float64) != 0 {
		t.Fatalf("สรุปรายวัน = %v", day)
	}
}

// ---- ลบตามคำขอ ----

func seedDeletion(t *testing.T, chats port.ChatRepository) {
	ctx := context.Background()
	now := time.Now()
	conv := func(id, user string) {
		_ = chats.CreateConversation(ctx, domain.Conversation{ID: id, OfficeID: "demo", ServiceID: "K11S", AdminID: "emp_" + user, Username: user, CreatedAt: now.Add(-48 * time.Hour)})
	}
	msg := func(id, conv string, at time.Time) {
		_ = chats.AppendMessage(ctx, domain.ChatMessage{ID: id, ConversationID: conv, OfficeID: "demo", ServiceID: "K11S", Role: "user", Text: "x", CreatedAt: at})
	}
	conv("c_ploy", "adm_ploy")
	msg("m1", "c_ploy", now.Add(-48*time.Hour))
	msg("m2", "c_ploy", now.Add(-time.Hour))
	conv("c_ploy_recent", "adm_ploy")
	msg("m3", "c_ploy_recent", now.Add(-time.Hour))
	conv("c_other", "adm_other")
	msg("m4", "c_other", now.Add(-time.Hour))
}

func TestDeletion_ScopeAndResult(t *testing.T) {
	d := newDeps(t)
	seedDeletion(t, d.ChatRepo)
	r := httpgin.NewTestRouter(d)

	for name, body := range map[string]string{
		"ไม่มี office":     `{"user":"adm_ploy","reason":"PDPA #1"}`,
		"ไม่มีเหตุผล":      `{"office_id":"demo","user":"adm_ploy"}`,
		"ขอบเขตกว้างเกิน":  `{"office_id":"demo","reason":"PDPA #1"}`,
		"office ไม่มีจริง": `{"office_id":"nope","user":"adm_ploy","reason":"PDPA #1"}`,
	} {
		if code, _ := consoleJSON(t, r, "POST", "/api/ai/admin/deletion-requests", body, consoleToken); code != 400 {
			t.Errorf("%s: ต้องได้ 400 ได้ %d", name, code)
		}
	}

	// ลบของ adm_ploy เฉพาะ 1 วันล่าสุด (ระบุด้วย username) — ห้องที่ยังเหลือข้อความเก่าต้องอยู่
	from := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	code, p := consoleJSON(t, r, "POST", "/api/ai/admin/deletion-requests",
		`{"office_id":"demo","user":"adm_ploy","from":"`+from+`","reason":"PDPA #12"}`, consoleToken)
	if code != 200 || p["status"] != "done" {
		t.Fatalf("ลบ = %d %v", code, p)
	}
	res := p["result"].(map[string]any)
	if res["messages"].(float64) != 2 || res["conversations"].(float64) != 1 {
		t.Fatalf("ผลลบ = %v (ต้องลบ m2,m3 และห้อง c_ploy_recent)", res)
	}
	ctx := context.Background()
	if _, err := d.ChatRepo.GetConversation(ctx, "c_ploy"); err != nil {
		t.Fatal("ห้องที่ยังเหลือข้อความเก่าต้องไม่ถูกลบ")
	}
	if _, err := d.ChatRepo.GetConversation(ctx, "c_ploy_recent"); err == nil {
		t.Fatal("ห้องที่ไม่เหลือข้อความต้องถูกลบ")
	}
	if _, err := d.ChatRepo.GetMessage(ctx, "m4"); err != nil {
		t.Fatal("ข้อความของผู้ใช้อื่นต้องไม่ถูกลบ")
	}

	_, logs := consoleJSON(t, r, "GET", "/api/ai/admin/access-log", "", consoleToken)
	if logs["total"].(float64) != 1 || !strings.Contains(logs["data"].([]any)[0].(map[string]any)["detail"].(string), "messages=2") {
		t.Fatalf("access_log = %v", logs)
	}
	if page := queryAudit(t, r, consoleToken, url.Values{"action": {domain.AuditDeletionRequest}}); page.Total != 1 {
		t.Fatalf("ต้องมี audit การลบ 1 รายการ ได้ %d", page.Total)
	}
	_, list := consoleJSON(t, r, "GET", "/api/ai/admin/deletion-requests", "", consoleToken)
	if list["total"].(float64) != 1 {
		t.Fatalf("ใบรับรองการลบ = %v", list)
	}
	if code, _ := consoleJSON(t, r, "POST", "/api/ai/admin/deletion-requests", `{"office_id":"demo","user":"x","reason":"y"}`, viewerToken(t)); code != http.StatusForbidden {
		t.Fatalf("viewer ต้องลบไม่ได้ ได้ %d", code)
	}
}

func TestDeletion_RecoverStaleRunning(t *testing.T) {
	dir := t.TempDir()
	repo, err := filestore.NewDeletionRepository(filepath.Join(dir, "d.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = repo.Create(ctx, domain.DeletionRequest{ID: "r1", Status: "running", CreatedAt: time.Now()})
	svc := service.NewDeletionService(nil, nil, nil, repo, nil, nil)
	svc.RecoverStale(ctx)
	list, _, _ := repo.List(ctx, 10, 0)
	if list[0].Status != "failed" || list[0].CompletedAt == nil {
		t.Fatalf("คำขอที่ค้างต้องเป็น failed: %+v", list[0])
	}
}
