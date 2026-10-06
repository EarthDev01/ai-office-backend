package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	httpgin "ai-office-backend/internal/adapter/handler/gin"
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
	d.Chat = service.NewChatService(llm, d.ChatRepo, service.NewToolRunner(d.Relay, d.Settings), time.UTC, d.Settings, d.Usage)
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
	return runChatIn(t, base, ticket, "", text, hostBody)
}

// runChatIn — conversationID ว่าง = ห้องใหม่
func runChatIn(t *testing.T, base, ticket, conversationID, text string, hostBody func(cmd service.FetchCommand) string) []sseEvent {
	t.Helper()
	req := map[string]string{"text": text}
	if conversationID != "" {
		req["conversation_id"] = conversationID
	}
	body, _ := json.Marshal(req)
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

// scriptedLLM — รอบเลือก tool ตอบตาม rounds ทีละรอบ (หมดแล้ว = ไม่เรียก tool) · รอบ stream ตอบ reply
type scriptedLLM struct {
	mu       sync.Mutex
	rounds   [][]port.ToolUse
	calls    int
	reply    string
	streamed port.LLMRequest
}

func (f *scriptedLLM) Complete(_ context.Context, _ port.LLMRequest) (port.LLMResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.calls
	f.calls++
	if i < len(f.rounds) {
		return port.LLMResponse{ToolUses: f.rounds[i], Usage: port.LLMUsage{InputTokens: 10, OutputTokens: 5}}, nil
	}
	return port.LLMResponse{Text: "พร้อม", Usage: port.LLMUsage{InputTokens: 10, OutputTokens: 1}}, nil
}

func (f *scriptedLLM) Stream(_ context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	f.mu.Lock()
	f.streamed = req
	f.mu.Unlock()
	return port.LLMUsage{InputTokens: 20, OutputTokens: 7}, onDelta(f.reply)
}

func lookup(id, user string) port.ToolUse {
	return port.ToolUse{ID: id, Name: "member_lookup", Input: map[string]any{"username": user}}
}

func TestChat_FollowUpRound_FetchesMoreThenStreams(t *testing.T) {
	llm := &scriptedLLM{
		rounds: [][]port.ToolUse{{lookup("t1", "somchai01")}, {lookup("t2", "somchai02")}},
		reply:  "ดูในการ์ดครับ",
	}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL, "MEMBER_INFO")

	var searched []string
	events := runChat(t, srv.URL, ticket, "เทียบยูส somchai01 กับ somchai02", func(cmd service.FetchCommand) string {
		searched = append(searched, cmd.Query["search"])
		return memberBody
	})
	if strings.Join(searched, ",") != "somchai01,somchai02" {
		t.Fatalf("ต้องดึง 2 รอบตามลำดับ: %v", searched)
	}
	if n := len(find(events, "card")); n != 2 {
		t.Fatalf("ต้องได้การ์ด 2 ใบ: %d", n)
	}
	// รอบเลือก 1 + ถามดึงเพิ่ม 2 (ได้ tool ใหม่ · ตอบพร้อม)
	if llm.calls != 3 {
		t.Fatalf("เรียกรอบเลือก tool %d ครั้ง", llm.calls)
	}
	// รอบ stream ต้องเห็นผล tool ของทั้ง 2 รอบ และห้ามเห็นคำว่า "พร้อม" ที่ทิ้งไปแล้ว
	var results int
	for _, m := range llm.streamed.Messages {
		results += len(m.ToolResults)
		if m.Role == "assistant" && strings.Contains(m.Text, "พร้อม") {
			t.Fatalf("ข้อความรอบถามดึงเพิ่มหลุดเข้าบทสนทนา: %+v", m)
		}
	}
	if results != 2 || llm.streamed.ToolChoice != port.ToolChoiceNone {
		t.Fatalf("รอบ stream เห็นผล tool %d ชิ้น · tool_choice=%s", results, llm.streamed.ToolChoice)
	}
	if len(find(events, "done")) != 1 {
		t.Fatalf("ต้องจบด้วย done: %v", events)
	}
}

func TestChat_FollowUpRound_StopsAtMaxRounds(t *testing.T) {
	llm := &scriptedLLM{rounds: [][]port.ToolUse{
		{lookup("t1", "a1")}, {lookup("t2", "a2")}, {lookup("t3", "a3")}, {lookup("t4", "a4")},
	}, reply: "ดูในการ์ดครับ"}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL, "MEMBER_INFO")

	fetches := 0
	events := runChat(t, srv.URL, ticket, "ดูยูสหลายคน", func(service.FetchCommand) string {
		fetches++
		return memberBody
	})
	if fetches != 3 {
		t.Fatalf("ดึงได้สูงสุด 3 รอบ แต่ดึง %d", fetches)
	}
	if llm.calls != 3 {
		t.Fatalf("ครบ 3 รอบแล้วต้องไม่ถามดึงเพิ่มอีก: %d", llm.calls)
	}
	if len(find(events, "done")) != 1 {
		t.Fatalf("ต้องจบด้วย done: %v", events)
	}
}

// slowLLM — รอบเลือก tool ช้ากว่า 2 วิ (จำลองโมเดลคิดนาน)
type slowLLM struct{ fakeLLM }

func (f *slowLLM) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	select {
	case <-time.After(2500 * time.Millisecond):
	case <-ctx.Done():
	}
	return f.fakeLLM.Complete(ctx, req)
}

// AC-2 ฝั่งโค้ดเรา: สถานะ "กำลังคิด" ต้องถึง widget ทันที ไม่รอโมเดล · ส่วน ≤ 2 วิกับโมเดลจริงวัดจาก first_token_ms ตอนทดสอบบนเว็บจริง
func TestChat_StatusBeforeModel(t *testing.T) {
	srv := chatServer(t, &slowLLM{fakeLLM{reply: []string{"สวัสดีครับ"}}})
	ticket := openSession(t, srv.URL)

	start := time.Now()
	res := post(t, srv.URL+"/api/ai/widget/service/K11S/chat", ticket, `{"text":"สวัสดี"}`)
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") && strings.Contains(line, `"thinking"`) {
			if d := time.Since(start); d > 2*time.Second {
				t.Fatalf("สถานะกำลังคิดมาช้า %v", d)
			}
			return
		}
	}
	t.Fatal("ไม่ได้รับสถานะกำลังคิด")
}

func tokenText(events []sseEvent) string {
	var b strings.Builder
	for _, e := range find(events, "token") {
		var tok struct{ Text string }
		_ = json.Unmarshal(e.data, &tok)
		b.WriteString(tok.Text)
	}
	return b.String()
}

// B-13 ครบวงจร: ขอคุยกับคน → ได้ช่องทาง support (ต่อท้ายให้แม้ LLM ลืม) · ขอซ้ำในห้องเดิม → หยุดวน ไม่เรียก LLM
func TestChat_HumanHandoff_SupportThenStopLoop(t *testing.T) {
	llm := &scriptedLLM{
		rounds: [][]port.ToolUse{{{ID: "a1", Name: "answer_directly", Input: map[string]any{"category": "human_handoff"}}}},
		reply:  "ผมเป็นผู้ช่วยอัตโนมัติครับ",
	}
	srv := chatServer(t, llm)
	ticket := openSession(t, srv.URL)
	noFetch := func(service.FetchCommand) string { t.Fatal("ต้องไม่ยิงหลังบ้าน"); return "" }

	first := runChatIn(t, srv.URL, ticket, "", "ขอคุยกับคน", noFetch)
	var st struct {
		ConversationID string `json:"conversation_id"`
	}
	_ = json.Unmarshal(find(first, "status")[0].data, &st)
	calls := llm.calls

	second := runChatIn(t, srv.URL, ticket, st.ConversationID, "ขอคุยกับคนครับ", noFetch)
	if llm.calls != calls {
		t.Fatalf("ขอซ้ำต้องไม่เรียก LLM อีก (เรียกเพิ่ม %d ครั้ง)", llm.calls-calls)
	}
	again := tokenText(second)
	support := strings.TrimSpace(strings.TrimPrefix(again, "เรื่องนี้ผู้ช่วยยังช่วยไม่ได้ครับ"))
	if support == "" || support == again || len(find(second, "done")) != 1 {
		t.Fatalf("ขอซ้ำต้องได้ข้อความหยุดวน + ช่องทาง support แล้วจบ: %q", again)
	}
	if got := tokenText(first); !strings.Contains(got, "ผู้ช่วยอัตโนมัติ") || !strings.HasSuffix(got, support) {
		t.Fatalf("ครั้งแรกต้องมีคำตอบ LLM แล้วต่อท้ายด้วยช่องทาง support: %q", got)
	}
}
