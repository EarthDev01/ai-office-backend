package test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

	"github.com/gin-gonic/gin"
)

type failingAccess struct{}

func (failingAccess) Insert(context.Context, domain.AccessLog) error { return errors.New("db down") }
func (failingAccess) List(context.Context, port.AccessLogFilter) ([]domain.AccessLog, int64, error) {
	return nil, 0, errors.New("db down")
}

type chatFixture struct {
	r      *gin.Engine
	chats  port.ChatRepository
	access *countingAccess
}

type countingAccess struct {
	port.AccessLogRepository
	actions []string
}

func (a *countingAccess) Insert(ctx context.Context, e domain.AccessLog) error {
	a.actions = append(a.actions, e.Action)
	return a.AccessLogRepository.Insert(ctx, e)
}

// newChatAdmin — ห้อง conv_new (เพิ่งเปิด) มีถาม-ตอบ 1 รอบ · ห้อง conv_old เปิดมา 100 วันแล้ว
func newChatAdmin(t *testing.T, access port.AccessLogRepository) chatFixture {
	t.Helper()
	dir := t.TempDir()
	chats, err := filestore.NewChatRepository(filepath.Join(dir, "c.jsonl"), filepath.Join(dir, "m.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	verifs, err := filestore.NewVerificationRepository(filepath.Join(dir, "v.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	counting := &countingAccess{AccessLogRepository: filestore.NewAccessLogRepository(filepath.Join(dir, "a.jsonl"))}
	if access == nil {
		access = counting
	}
	ctx := context.Background()
	now := time.Now()
	seed := func(id string, opened time.Time, title string) {
		_ = chats.CreateConversation(ctx, domain.Conversation{ID: id, OfficeID: "demo", ServiceID: "K11S", AdminID: "emp_1",
			Username: "adm_ploy", Title: title, CreatedAt: opened, UpdatedAt: opened})
		_ = chats.AppendMessage(ctx, domain.ChatMessage{ID: id + "_q", ConversationID: id, OfficeID: "demo", ServiceID: "K11S",
			Role: "user", Text: title, CreatedAt: opened})
		_ = chats.AppendMessage(ctx, domain.ChatMessage{ID: id + "_a", ConversationID: id, OfficeID: "demo", ServiceID: "K11S",
			Role: "assistant", Text: "ดูในการ์ดครับ", Status: "ok", VerificationStatus: domain.VerifyPending,
			Cards: []domain.Card{{ID: "c1", Kind: "ok", Title: "ข้อมูลสมาชิก"}}, CreatedAt: opened.Add(time.Second)})
		_ = chats.TouchConversation(ctx, id, opened.Add(time.Second), 2)
	}
	seed("conv_new", now.Add(-time.Hour), "เช็คยูส somchai01")
	seed("conv_old", now.Add(-100*24*time.Hour), "ยอดเมื่อต้นปี")
	// คำตอบที่ล้มไม่เข้าคิว
	_ = chats.AppendMessage(ctx, domain.ChatMessage{ID: "err_a", ConversationID: "conv_new", OfficeID: "demo", ServiceID: "K11S",
		Role: "assistant", Status: "error", CreatedAt: now})

	d := newDeps(t)
	d.ChatAdmin = service.NewChatAdminService(chats, verifs, access, nil)
	return chatFixture{r: httpgin.NewTestRouter(d), chats: chats, access: counting}
}

func consoleJSON(t *testing.T, r http.Handler, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	w := do(t, r, req{method: method, path: path, body: body, console: token})
	var res struct {
		Payload map[string]any `json:"payload"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w.Code, res.Payload
}

func TestChatAdmin_SearchAndReadLogsAccess(t *testing.T) {
	f := newChatAdmin(t, nil)
	code, p := consoleJSON(t, f.r, "GET", "/api/ai/admin/conversations?office_id=demo&q=SOMCHAI", "", consoleToken)
	if code != 200 || p["total"].(float64) != 1 {
		t.Fatalf("ค้นหัวข้อ = %d %v", code, p)
	}
	code, p = consoleJSON(t, f.r, "GET", "/api/ai/admin/conversations/conv_new", "", consoleToken)
	if code != 200 || len(p["messages"].([]any)) != 3 {
		t.Fatalf("อ่านห้อง = %d %v", code, p)
	}
	if strings.Join(f.access.actions, ",") != "search,read_conversation" {
		t.Fatalf("access_log = %v", f.access.actions)
	}
}

func TestChatAdmin_OldConversationNeedsConfirm(t *testing.T) {
	f := newChatAdmin(t, nil)
	if code, _ := consoleJSON(t, f.r, "GET", "/api/ai/admin/conversations/conv_old", "", consoleToken); code != http.StatusConflict {
		t.Fatalf("ห้องเก่าต้องได้ 409 ได้ %d", code)
	}
	if code, _ := consoleJSON(t, f.r, "GET", "/api/ai/admin/conversations/conv_old?confirm_old=1", "", consoleToken); code != 200 {
		t.Fatalf("ยืนยันแล้วต้องเปิดได้ ได้ %d", code)
	}
	if strings.Join(f.access.actions, ",") != "read_old" {
		t.Fatalf("access_log = %v (409 ต้องไม่บันทึก)", f.access.actions)
	}
}

func TestChatAdmin_AccessLogFailClosed(t *testing.T) {
	f := newChatAdmin(t, failingAccess{})
	for _, path := range []string{"/api/ai/admin/conversations", "/api/ai/admin/conversations/conv_new", "/api/ai/admin/verifications/queue"} {
		code, p := consoleJSON(t, f.r, "GET", path, "", consoleToken)
		if code == 200 || p != nil {
			t.Errorf("%s: บันทึก access_log ไม่ได้ต้องไม่ให้อ่าน ได้ %d %v", path, code, p)
		}
	}
}

func TestChatAdmin_QueueVerifyStats(t *testing.T) {
	f := newChatAdmin(t, nil)
	code, p := consoleJSON(t, f.r, "GET", "/api/ai/admin/verifications/queue", "", consoleToken)
	if code != 200 || p["total"].(float64) != 2 {
		t.Fatalf("คิว = %d %v (ต้องไม่มีคำตอบที่ล้ม)", code, p)
	}
	first := p["data"].([]any)[0].(map[string]any)
	if first["question"] != "ยอดเมื่อต้นปี" || first["answer"].(map[string]any)["id"] != "conv_old_a" {
		t.Fatalf("คิวต้องเรียงเก่าสุดก่อนและจับคู่คำถาม: %v", first)
	}

	// ผิดแต่ไม่บอกประเภท/คำตอบที่ถูก = ไม่ผ่าน
	if code, _ := consoleJSON(t, f.r, "POST", "/api/ai/admin/verifications", `{"message_id":"conv_new_a","status":"wrong"}`, consoleToken); code != 400 {
		t.Fatalf("ผิดต้องมีประเภท ได้ %d", code)
	}
	// ข้อความของผู้ใช้ / คำตอบที่ล้ม ตรวจไม่ได้
	for _, id := range []string{"conv_new_q", "err_a"} {
		if code, _ := consoleJSON(t, f.r, "POST", "/api/ai/admin/verifications", `{"message_id":"`+id+`","status":"correct"}`, consoleToken); code != 400 {
			t.Fatalf("%s ต้องตรวจไม่ได้ ได้ %d", id, code)
		}
	}
	if code, _ := consoleJSON(t, f.r, "POST", "/api/ai/admin/verifications", `{"message_id":"conv_new_a","status":"correct"}`, consoleToken); code != 200 {
		t.Fatalf("ตรวจถูก ได้ %d", code)
	}
	// ตรวจซ้ำทับผลเดิม
	code, v := consoleJSON(t, f.r, "POST", "/api/ai/admin/verifications",
		`{"message_id":"conv_new_a","status":"wrong","error_type":"wrong_number","correct_answer":"ยอดต้องเป็น 500"}`, consoleToken)
	if code != 200 || v["verified_by"] != "break-glass" {
		t.Fatalf("ตรวจซ้ำ = %d %v", code, v)
	}

	_, st := consoleJSON(t, f.r, "GET", "/api/ai/admin/verifications/stats?office_id=demo", "", consoleToken)
	if st["wrong"].(float64) != 1 || st["pending"].(float64) != 1 || st["correct"].(float64) != 0 {
		t.Fatalf("สถิติ = %v", st)
	}
	if !strings.Contains(st["basis"].(string), "ยังไม่ตรวจ 1") {
		t.Fatalf("basis ต้องบอกจำนวนที่ยังไม่ตรวจ: %v", st["basis"])
	}
	_, conv := consoleJSON(t, f.r, "GET", "/api/ai/admin/conversations/conv_new", "", consoleToken)
	if conv["verifications"].(map[string]any)["conv_new_a"] == nil {
		t.Fatalf("อ่านห้องต้องเห็นผลตรวจ: %v", conv["verifications"])
	}
}

func TestChatAdmin_RequiresPermission(t *testing.T) {
	f := newChatAdmin(t, nil)
	viewer, _ := auth.NewJWTIssuer(consoleJWTSecret, time.Hour).Issue(port.Claims{UserID: "u1", Username: "viewer1", Role: domain.RoleViewer})
	for _, path := range []string{"/api/ai/admin/conversations", "/api/ai/admin/verifications/queue", "/api/ai/admin/verifications/stats"} {
		if code, _ := consoleJSON(t, f.r, "GET", path, "", viewer); code != http.StatusForbidden {
			t.Errorf("%s: viewer ต้องได้ 403 ได้ %d", path, code)
		}
	}
	if len(f.access.actions) != 0 {
		t.Fatalf("ถูกปฏิเสธต้องไม่ได้อ่าน: %v", f.access.actions)
	}
}
