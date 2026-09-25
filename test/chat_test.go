package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

// fakeLLM — รอบ 1 เรียก tool ตาม uses · รอบ 2 พ่นข้อความ reply เป็นชิ้น ๆ
type fakeLLM struct {
	mu    sync.Mutex
	uses  []port.ToolUse
	reply []string
	r2    port.LLMRequest // คำขอรอบ 2 ที่ได้รับ (ไว้ตรวจว่า LLM เห็นอะไร)
}

func (f *fakeLLM) Complete(_ context.Context, _ port.LLMRequest) (port.LLMResponse, error) {
	return port.LLMResponse{ToolUses: f.uses, Usage: port.LLMUsage{InputTokens: 10, OutputTokens: 5}}, nil
}

func (f *fakeLLM) Stream(_ context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	f.mu.Lock()
	f.r2 = req
	f.mu.Unlock()
	for _, c := range f.reply {
		if err := onDelta(c); err != nil {
			return port.LLMUsage{}, err
		}
	}
	return port.LLMUsage{InputTokens: 20, OutputTokens: 7}, nil
}

type sseEvent struct {
	name string
	data json.RawMessage
}

// chatServer เปิด server จริง (SSE ต้องอ่านไปพร้อมกับ POST relay)
func chatServer(t *testing.T, llm port.LLMClient) *httptest.Server {
	t.Helper()
	d := newDeps(t)
	repo, err := filestore.NewChatRepository(filepath.Join(t.TempDir(), "c.jsonl"), filepath.Join(t.TempDir(), "m.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Bangkok")
	d.Chat = service.NewChatService(llm, repo, service.NewToolRunner(d.Relay), loc, service.ChatSettings{})
	srv := httptest.NewServer(httpgin.NewTestRouter(d))
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, url, bearer, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", officeOrigin)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func openSession(t *testing.T, base string, perms ...string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"permissions": perms})
	res := post(t, base+"/api/ai/widget/service/K11S/browser-session", adminJWT("adm_ploy", "K11S"), string(body))
	defer res.Body.Close()
	var out struct {
		Payload struct {
			Ticket string `json:"ticket"`
		} `json:"payload"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 || out.Payload.Ticket == "" {
		t.Fatalf("browser-session = %d", res.StatusCode)
	}
	return out.Payload.Ticket
}

// runChat ส่งคำถามแล้วเล่นบทเป็น widget: เจอ fetch → ตอบด้วย hostBody ผ่าน relay
func runChat(t *testing.T, base, ticket, text string, hostBody func(cmd service.FetchCommand) string) []sseEvent {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"text": text})
	res := post(t, base+"/api/ai/widget/service/K11S/chat", ticket, string(body))
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("chat = %d", res.StatusCode)
	}
	var events []sseEvent
	sc := bufio.NewScanner(res.Body)
	var name string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			ev := sseEvent{name: name, data: json.RawMessage(strings.TrimPrefix(line, "data: "))}
			events = append(events, ev)
			if name == "fetch" {
				var cmd service.FetchCommand
				_ = json.Unmarshal(ev.data, &cmd)
				rb, _ := json.Marshal(map[string]any{"status": 200, "body": hostBody(cmd)})
				r := post(t, base+"/api/ai/widget/service/K11S/chat/relay/"+cmd.ID, ticket, string(rb))
				r.Body.Close()
				if r.StatusCode != 200 {
					t.Fatalf("relay = %d", r.StatusCode)
				}
			}
		}
	}
	return events
}

func find(events []sseEvent, name string) []sseEvent {
	var out []sseEvent
	for _, e := range events {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

const memberBody = `{"code":0,"message":"Success","data":[{"username":"somchai01","full_name":"สมชาย ใจดี","phone_number":"089xxxx123","active":1,"topup_status":1,"date_regis":"2026-03-05T10:30:00Z"}]}`

func TestChat_ToolViaRelay_CardAndGuard(t *testing.T) {
	llm := &fakeLLM{
		uses:  []port.ToolUse{{ID: "t1", Name: "member_lookup", Input: map[string]any{"username": "somchai01"}}},
		reply: []string{"สมาชิก somchai01 ชื่อ สมช", "าย ใจดี ยืนยันแล้ว ยอด 1500 บาท"},
	}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL, "MEMBER_INFO")

	var gotPath string
	events := runChat(t, srv.URL, ticket, "เช็คยูส somchai01 ให้หน่อย", func(cmd service.FetchCommand) string {
		gotPath = cmd.Path + "?search=" + cmd.Query["search"]
		return memberBody
	})

	if gotPath != "/get-member-list-v3/K11S?search=somchai01" {
		t.Fatalf("widget ถูกสั่งให้ยิง %q", gotPath)
	}
	cards := find(events, "card")
	if len(cards) != 1 || !strings.Contains(string(cards[0].data), "สมชาย ใจดี") {
		t.Fatalf("ต้องได้การ์ดข้อมูลสมาชิก 1 ใบ: %v", cards)
	}
	var text strings.Builder
	for _, e := range find(events, "token") {
		var tok struct{ Text string }
		_ = json.Unmarshal(e.data, &tok)
		text.WriteString(tok.Text)
	}
	got := text.String()
	for _, leak := range []string{"สมชาย", "ใจดี", "1500"} {
		if strings.Contains(got, leak) {
			t.Errorf("guard ปล่อย %q หลุด: %s", leak, got)
		}
	}
	if !strings.Contains(got, "somchai01") || !strings.Contains(got, "ยืนยันแล้ว") {
		t.Errorf("คำที่ผู้ใช้พิมพ์เองและป้ายสถานะต้องผ่าน: %s", got)
	}
	if len(find(events, "done")) != 1 {
		t.Fatalf("ต้องจบด้วย done: %v", events)
	}

	// LLM รอบ 2 ต้องไม่เห็นค่าดิบ — เห็นแค่ป้ายและชื่อ field
	var seen bytes.Buffer
	for _, m := range llm.r2.Messages {
		for _, r := range m.ToolResults {
			seen.WriteString(r.Content)
		}
	}
	if strings.Contains(seen.String(), "สมชาย") || strings.Contains(seen.String(), "089") {
		t.Errorf("tool_result มีค่าดิบ: %s", seen.String())
	}
	if !strings.Contains(seen.String(), `"verified":true`) {
		t.Errorf("tool_result ต้องมีสถานะสรุป: %s", seen.String())
	}
}

func TestChat_PermissionDenied_NoFetch(t *testing.T) {
	llm := &fakeLLM{
		uses:  []port.ToolUse{{ID: "t1", Name: "member_lookup", Input: map[string]any{"username": "somchai01"}}},
		reply: []string{"ไม่มีสิทธิ์ครับ"},
	}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL) // ไม่มีสิทธิ์ MEMBER_INFO

	events := runChat(t, srv.URL, ticket, "เช็คยูส somchai01", func(service.FetchCommand) string {
		t.Fatal("ไม่มีสิทธิ์ต้องไม่ยิง API เลย")
		return ""
	})
	cards := find(events, "card")
	if len(cards) != 1 || !strings.Contains(string(cards[0].data), `"kind":"denied"`) {
		t.Fatalf("ต้องได้การ์ดไม่มีสิทธิ์: %v", cards)
	}
}

func TestChat_TicketBoundToService(t *testing.T) {
	srv := chatServer(t, &fakeLLM{})
	ticket := openSession(t, srv.URL, "MEMBER_INFO")
	res := post(t, srv.URL+"/api/ai/widget/service/PG99/chat", ticket, `{"text":"hi"}`)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("ตั๋วของ K11S ใช้กับ PG99 ได้: %d", res.StatusCode)
	}
}

func TestChat_RelayRejectsOtherTicket(t *testing.T) {
	d := newDeps(t)
	relay := d.Relay
	// ผลที่ส่งมาด้วยตั๋วใบอื่นต้องไม่ถูกหยิบไปใช้
	if err := relay.Deliver("ticket-B", "abc", service.RelayResult{Status: 200, Body: "{}"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := relay.Fetch(ctx, "ticket-A", service.RelayRequest{Method: "GET", Path: "/x"}, 50*time.Millisecond, func(string, any) error { return nil })
	if err == nil {
		t.Fatal("ต้องไม่ได้ผลของตั๋วอื่น")
	}
}

func TestChat_TicketRequired(t *testing.T) {
	srv := chatServer(t, &fakeLLM{})
	// token ของหน้า office ใช้แทนตั๋วแชทไม่ได้
	res := post(t, srv.URL+"/api/ai/widget/service/K11S/chat", adminJWT("adm_ploy", "K11S"), `{"text":"hi"}`)
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestPageConfig(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: "GET", path: "/api/ai/widget/page-config", origin: officeOrigin})
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	cfg := payload[struct {
		Kind        string `json:"kind"`
		HostAPIBase string `json:"host_api_base"`
	}](t, w)
	if cfg.Kind != "office-v10x" || cfg.HostAPIBase != fmt.Sprintf("%s/api", officeOrigin) {
		t.Fatalf("page-config = %+v", cfg)
	}
}

func TestChat_LookupMenu_ReferenceCardNoFetch(t *testing.T) {
	llm := &fakeLLM{
		uses:  []port.ToolUse{{ID: "m1", Name: "lookup_menu", Input: map[string]any{"query": "รายการถอน"}}},
		reply: []string{"ไปที่เมนูรายการถอนครับ"},
	}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL, "TRANSECTION_WITHDRAW")
	events := runChat(t, srv.URL, ticket, "รายการถอนอยู่เมนูไหน", func(service.FetchCommand) string {
		t.Fatal("lookup_menu ต้องไม่ยิง API หลังบ้าน")
		return ""
	})
	cards := find(events, "card")
	if len(cards) != 1 || !strings.Contains(string(cards[0].data), `"kind":"reference"`) {
		t.Fatalf("ต้องได้การ์ดอ้างอิงเมนู 1 ใบ: %v", cards)
	}
	if len(find(events, "done")) != 1 {
		t.Fatalf("ต้องจบด้วย done: %v", events)
	}
}
