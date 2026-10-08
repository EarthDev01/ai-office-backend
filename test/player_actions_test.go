package test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

const guestID = "g_4f3c2b1a0d9e8f7a6b5c"

// toolLLM — รอบเลือก tool ตอบ uses (ทุกรอบเหมือนกัน · ซ้ำแล้วไม่รันอีก) และจำว่าเห็น tool อะไรบ้าง
type toolLLM struct {
	mu       sync.Mutex
	uses     []port.ToolUse
	calls    int
	seen     []string
	streamed port.LLMRequest
}

func (f *toolLLM) Complete(_ context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.seen = f.seen[:0]
	for _, d := range req.Tools {
		f.seen = append(f.seen, d.Name)
	}
	return port.LLMResponse{ToolUses: f.uses, Usage: port.LLMUsage{InputTokens: 10, OutputTokens: 5}}, nil
}

func (f *toolLLM) Stream(_ context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	f.mu.Lock()
	f.streamed = req
	f.mu.Unlock()
	return port.LLMUsage{InputTokens: 20, OutputTokens: 7}, onDelta("กดปุ่มด้านล่างได้เลย")
}

func playerServer(t *testing.T, llm port.LLMClient) *httptest.Server {
	t.Helper()
	d := newDeps(t, demoOffice(), tangtemOffice())
	d.Chat = service.NewChatService(llm, d.ChatRepo, service.NewToolRunner(d.Relay, d.Settings), time.UTC, d.Settings, d.Usage)
	srv := httptest.NewServer(httpgin.NewTestRouter(d))
	t.Cleanup(srv.Close)
	return srv
}

// playerPost — auth = ค่า Authorization ทั้งก้อน ("Guest <id>" / "Bearer <jwt|ticket>")
func playerPost(t *testing.T, url, auth, body string) *http.Response {
	t.Helper()
	r, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", playerOrigin)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func playerTicket(t *testing.T, base, auth string) string {
	t.Helper()
	res := playerPost(t, base+"/api/ai/widget/service/TANGTEM/browser-session", auth, "")
	defer res.Body.Close()
	var out struct {
		Payload struct{ Ticket string } `json:"payload"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 || out.Payload.Ticket == "" {
		t.Fatalf("browser-session (%s) = %d", auth, res.StatusCode)
	}
	return out.Payload.Ticket
}

// playerChat — ส่งคำถาม · เจอ fetch ตอบด้วย hostBody ผ่าน relay
func playerChat(t *testing.T, base, ticket, text string, hostBody func(cmd service.FetchCommand) string) []sseEvent {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"text": text})
	res := playerPost(t, base+"/api/ai/widget/service/TANGTEM/chat", "Bearer "+ticket, string(body))
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
				r := playerPost(t, base+"/api/ai/widget/service/TANGTEM/chat/relay/"+cmd.ID, "Bearer "+ticket, string(rb))
				r.Body.Close()
			}
		}
	}
	return events
}

func actionIDs(events []sseEvent) []string {
	var out []string
	for _, e := range find(events, "action") {
		var a domain.ChatAction
		_ = json.Unmarshal(e.data, &a)
		out = append(out, a.ID)
	}
	return out
}

func noHost(t *testing.T) func(service.FetchCommand) string {
	return func(cmd service.FetchCommand) string {
		t.Errorf("ไม่ควรยิงหลังบ้าน: %s", cmd.Path)
		return "{}"
	}
}

func TestPlayer_PageConfigHasActionsAndGuest(t *testing.T) {
	r := newRouter(t, tangtemOffice())
	w := do(t, r, req{method: "GET", path: "/api/ai/widget/page-config", origin: playerOrigin})
	cfg := payload[struct {
		Guest       bool `json:"guest"`
		PageActions []struct {
			ID    string         `json:"id"`
			About string         `json:"about"`
			Open  map[string]any `json:"open"`
		} `json:"page_actions"`
	}](t, w)
	if !cfg.Guest || len(cfg.PageActions) == 0 {
		t.Fatalf("page-config = %s", w.Body.String())
	}
	for _, a := range cfg.PageActions {
		if a.About != "" {
			t.Fatalf("about เป็นคำอธิบายให้โมเดล ไม่ต้องส่งลงหน้าเว็บ: %+v", a)
		}
	}
	// หลังบ้านไม่มีปุ่ม/guest
	w = do(t, newRouter(t), req{method: "GET", path: "/api/ai/widget/page-config", origin: officeOrigin})
	if strings.Contains(w.Body.String(), "page_actions") || strings.Contains(w.Body.String(), `"guest"`) {
		t.Fatalf("หลังบ้านไม่ควรได้ page_actions/guest: %s", w.Body.String())
	}
}

func TestPlayer_GuestCredential(t *testing.T) {
	srv := playerServer(t, &toolLLM{})
	for _, bad := range []string{"Guest short", "Guest " + guestID + "!", "Guest "} {
		res := playerPost(t, srv.URL+"/api/ai/widget/service/TANGTEM/browser-session", bad, "")
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("%q = %d อยากได้ 401", bad, res.StatusCode)
		}
	}
	// หลังบ้าน (connector ไม่เปิด guest) รับ Guest ไม่ได้
	r, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ai/widget/service/K11S/browser-session", nil)
	r.Header.Set("Origin", officeOrigin)
	r.Header.Set("Authorization", "Guest "+guestID)
	res2, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != 401 {
		var b strings.Builder
		_, _ = bufio.NewReader(res2.Body).WriteTo(&b)
		t.Fatalf("guest บนหลังบ้าน = %d %s", res2.StatusCode, b.String())
	}
}

// ผู้ไม่ล็อกอิน: ไม่เห็น tool ข้อมูล · ปุ่มของสมาชิกถูกทิ้ง · ปุ่ม guest แนบได้
func TestPlayer_GuestButtonsAndNoAccountTools(t *testing.T) {
	llm := &toolLLM{uses: []port.ToolUse{
		{ID: "u1", Name: "show_button", Input: map[string]any{"id": "register"}},
		{ID: "u2", Name: "show_button", Input: map[string]any{"id": "promotion"}}, // ของสมาชิก
		{ID: "u3", Name: "list_promotions", Input: map[string]any{}},              // ไม่มีให้ guest
	}}
	srv := playerServer(t, llm)
	tk := playerTicket(t, srv.URL, "Guest "+guestID)
	ev := playerChat(t, srv.URL, tk, "สมัครยังไง", noHost(t))

	if got := actionIDs(ev); len(got) != 1 || got[0] != "register" {
		t.Fatalf("ปุ่ม = %v อยากได้ [register]", got)
	}
	if len(find(ev, "card")) != 0 {
		t.Fatalf("ผู้ไม่ล็อกอินไม่ควรได้การ์ด: %v", find(ev, "card"))
	}
	for _, n := range llm.seen {
		if n == "list_promotions" || n == "my_balance" {
			t.Fatalf("ผู้ไม่ล็อกอินเห็น tool %s", n)
		}
	}
	if !strings.Contains(llm.streamed.System, "ยังไม่ได้เข้าสู่ระบบ") {
		t.Fatal("system prompt ต้องบอกว่าผู้ใช้ยังไม่ล็อกอิน")
	}
}

// ครบ 6 ข้อต่อวัน → ตอบเองไม่เรียก LLM + ปุ่มสมัคร/เข้าสู่ระบบ
func TestPlayer_GuestDailyLimit(t *testing.T) {
	llm := &toolLLM{uses: []port.ToolUse{{ID: "u1", Name: "answer_directly", Input: map[string]any{"category": "greeting"}}}}
	srv := playerServer(t, llm)
	tk := playerTicket(t, srv.URL, "Guest "+guestID)
	for i := 0; i < 6; i++ {
		playerChat(t, srv.URL, tk, "สวัสดี "+string(rune('ก'+i)), noHost(t))
	}
	calls := llm.calls
	ev := playerChat(t, srv.URL, tk, "ขออีกข้อ", noHost(t))
	if llm.calls != calls {
		t.Fatal("ครบโควตาแล้วยังเรียก LLM")
	}
	if !strings.Contains(tokenText(ev), "ครบ 6 ข้อ") {
		t.Fatalf("ข้อความ = %q", tokenText(ev))
	}
	if got := strings.Join(actionIDs(ev), ","); got != "register,login,forgot_password" {
		t.Fatalf("ปุ่ม = %s", got)
	}
	// guest คนอื่นยังถามได้
	other := playerTicket(t, srv.URL, "Guest x_"+guestID)
	ev = playerChat(t, srv.URL, other, "สวัสดี", noHost(t))
	if strings.Contains(tokenText(ev), "ครบ") {
		t.Fatal("โควตาต้องแยกต่อ guest_id")
	}
	// สมาชิกไม่โดนโควตา guest
	member := playerTicket(t, srv.URL, "Bearer "+playerJWT("somchai01"))
	for i := 0; i < 7; i++ {
		ev = playerChat(t, srv.URL, member, "สวัสดี "+string(rune('ก'+i)), noHost(t))
	}
	if strings.Contains(tokenText(ev), "ครบ") {
		t.Fatal("สมาชิกโดนโควตา guest")
	}
}

// สมาชิกถามโปร: ดึง /bonus (data.bonus) เป็นการ์ด + ปุ่มดูโปรโมชั่น · ประวัติคืนปุ่มเดิม · ไม่มีการ์ด "เมนูที่เกี่ยวข้อง"
func TestPlayer_MemberPromotionsWithButton(t *testing.T) {
	llm := &toolLLM{uses: []port.ToolUse{
		{ID: "u1", Name: "list_promotions", Input: map[string]any{}},
		{ID: "u2", Name: "show_button", Input: map[string]any{"id": "promotion"}},
		{ID: "u3", Name: "show_button", Input: map[string]any{"id": "register"}}, // ของ guest — ต้องถูกทิ้ง
		{ID: "u4", Name: "lookup_menu", Input: map[string]any{"query": "โปรโมชั่น"}},
	}}
	srv := playerServer(t, llm)
	tk := playerTicket(t, srv.URL, "Bearer "+playerJWT("somchai01"))
	ev := playerChat(t, srv.URL, tk, "มีโปรอะไรบ้าง", func(cmd service.FetchCommand) string {
		if cmd.Path != "/bonus" {
			t.Errorf("path = %s", cmd.Path)
		}
		return `{"status":{"code":200},"message":{"code":0,"message":"ok"},"data":{"balance_realcash":0,"bonus":[` +
			`{"bonus_name":"โบนัสทุกยอดฝาก","BonusID":7,"bonus_percent":10,"fix_deposit":100,"bonus_limit":1000,"fix_multiple":3},` +
			`{"bonus_name":"ไม่รับโบนัส","BonusID":104}]}}`
	})
	if got := actionIDs(ev); len(got) != 1 || got[0] != "promotion" {
		t.Fatalf("ปุ่ม = %v", got)
	}
	cards := find(ev, "card")
	if len(cards) != 1 || !strings.Contains(string(cards[0].data), "โบนัสทุกยอดฝาก") || strings.Contains(string(cards[0].data), "ไม่รับโบนัส") {
		t.Fatalf("การ์ด = %v", cards)
	}
	if strings.Contains(string(cards[0].data), "reference") {
		t.Fatal("หน้าเว็บผู้เล่นไม่ควรได้การ์ดเมนูที่เกี่ยวข้อง")
	}

	// ประวัติคืนปุ่มเดิม
	var st struct {
		ConversationID string `json:"conversation_id"`
	}
	_ = json.Unmarshal(find(ev, "status")[0].data, &st)
	hr, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/ai/widget/service/TANGTEM/conversations/"+st.ConversationID, nil)
	hr.Header.Set("Origin", playerOrigin)
	hr.Header.Set("Authorization", "Bearer "+tk)
	res, err := http.DefaultClient.Do(hr)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var h struct {
		Payload struct {
			Messages []struct {
				Role    string              `json:"role"`
				Actions []domain.ChatAction `json:"actions"`
			} `json:"messages"`
		} `json:"payload"`
	}
	_ = json.NewDecoder(res.Body).Decode(&h)
	found := false
	for _, m := range h.Payload.Messages {
		if m.Role == "assistant" && len(m.Actions) == 1 && m.Actions[0].Label == "ดูโปรโมชั่น" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ประวัติไม่มีปุ่ม: %+v", h.Payload.Messages)
	}
}
