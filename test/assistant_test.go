package test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"ai-office-backend/docs"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

// assistantLLM — รอบแรกเรียกเครื่องมือตาม uses (ถ้ามี) · รอบเขียนคำตอบพ่น reply · จดคำขอไว้ตรวจ
type assistantLLM struct {
	mu       sync.Mutex
	uses     []port.ToolUse
	direct   string // ไม่มี uses = ตอบข้อความนี้จากรอบแรกเลย
	reply    []string
	requests []port.LLMRequest
	calls    int
}

func (f *assistantLLM) Complete(_ context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	f.calls++
	if f.calls == 1 && len(f.uses) > 0 {
		return port.LLMResponse{ToolUses: f.uses, Usage: port.LLMUsage{InputTokens: 100, OutputTokens: 10}}, nil
	}
	return port.LLMResponse{Text: f.direct, Usage: port.LLMUsage{InputTokens: 50, OutputTokens: 5}}, nil
}

func (f *assistantLLM) Stream(_ context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	for _, c := range f.reply {
		if err := onDelta(c); err != nil {
			return port.LLMUsage{}, err
		}
	}
	return port.LLMUsage{InputTokens: 120, OutputTokens: 30}, nil
}

func (f *assistantLLM) first() port.LLMRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[0]
}

// assistantServer — ผู้ช่วยปิดอยู่เป็นค่าเริ่มต้น · enable=true เปิดให้ผ่านหน้าตั้งค่าเหมือนผู้ใช้จริง
func assistantServer(t *testing.T, llm port.LLMClient, enable bool) http.Handler {
	t.Helper()
	d := newDeps(t)
	tools := service.NewAssistantTools(d.OfficeService, d.Usage, d.ChatAdmin, d.Settings, nil, d.Audit)
	d.Assistant = service.NewAssistantService(llm, d.Settings, d.Permissions, d.Usage, tools, docs.Guide)
	d.Guide = docs.Guide
	r := httpgin.NewTestRouter(d)
	if enable {
		since, _ := settingsSince(t, r)
		if code, body := consoleJSON(t, r, "PATCH", "/api/ai/admin/settings", `{"assistant_enabled":true,"updated_at":"`+since+`"}`, consoleToken); code != 200 {
			t.Fatalf("เปิดผู้ช่วย = %d %v", code, body)
		}
	}
	return r
}

func ask(t *testing.T, r http.Handler, token, body string) (int, []sseEvent) {
	t.Helper()
	w := do(t, r, req{method: http.MethodPost, path: "/api/ai/admin/assistant", console: token, body: body})
	if w.Code != 200 {
		return w.Code, nil
	}
	var events []sseEvent
	var name string
	sc := bufio.NewScanner(strings.NewReader(w.Body.String()))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			events = append(events, sseEvent{name: name, data: json.RawMessage(strings.TrimPrefix(line, "data: "))})
		}
	}
	return w.Code, events
}

func toolNames(defs []port.ToolDef) map[string]bool {
	m := map[string]bool{}
	for _, d := range defs {
		m[d.Name] = true
	}
	return m
}

const oneQuestion = `{"messages":[{"role":"user","text":"มี domain อะไรบ้าง"}],"page":"/offices"}`

func TestAssistant_DisabledByDefault(t *testing.T) {
	r := assistantServer(t, &assistantLLM{}, false)
	_, p := consoleJSON(t, r, "GET", "/api/ai/admin/assistant/status", "", consoleToken)
	if p["enabled"] != false {
		t.Fatalf("ค่าเริ่มต้นต้องปิด ได้ %v", p)
	}
	if code, _ := ask(t, r, consoleToken, oneQuestion); code != http.StatusServiceUnavailable {
		t.Fatalf("ปิดอยู่ต้อง 503 ได้ %d", code)
	}
}

func TestAssistant_AnswersWithToolsAndCountsUsage(t *testing.T) {
	llm := &assistantLLM{
		uses:  []port.ToolUse{{ID: "t1", Name: "list_domains", Input: map[string]any{}}},
		reply: []string{"มี domain ", "demo ครับ"},
	}
	r := assistantServer(t, llm, true)
	_, p := consoleJSON(t, r, "GET", "/api/ai/admin/assistant/status", "", consoleToken)
	if p["enabled"] != true || p["ready"] != true {
		t.Fatalf("status = %v", p)
	}

	code, events := ask(t, r, consoleToken, oneQuestion)
	if code != 200 || len(find(events, "done")) != 1 || len(find(events, "error")) != 0 {
		t.Fatalf("events = %d %+v", code, events)
	}
	var text string
	for _, e := range find(events, "token") {
		var d struct{ Text string }
		_ = json.Unmarshal(e.data, &d)
		text += d.Text
	}
	if text != "มี domain demo ครับ" {
		t.Fatalf("คำตอบ = %q", text)
	}

	// รอบเขียนคำตอบเห็นผลของเครื่องมือ (ข้อมูล domain จริง) และห้ามเรียกเครื่องมือเพิ่ม
	llm.mu.Lock()
	final := llm.requests[len(llm.requests)-1]
	llm.mu.Unlock()
	if final.ToolChoice != port.ToolChoiceNone {
		t.Fatal("รอบเขียนคำตอบต้องห้ามเรียกเครื่องมือ")
	}
	last := final.Messages[len(final.Messages)-1]
	if len(last.ToolResults) != 1 || !strings.Contains(last.ToolResults[0].Content, `"id":"demo"`) {
		t.Fatalf("ผลเครื่องมือ = %+v", last.ToolResults)
	}
	// คู่มืออยู่ใน system prompt
	if !strings.Contains(llm.first().System, "เริ่มต้นติดตั้ง") || !strings.Contains(llm.first().System, "/offices") {
		t.Fatal("system prompt ต้องมีคู่มือและหน้าที่เปิดอยู่")
	}

	// token นับแยกเป็นผู้ช่วยคอนโซล
	_, u := consoleJSON(t, r, "GET", "/api/ai/admin/usage", "", consoleToken)
	raw, _ := json.Marshal(u)
	if !strings.Contains(string(raw), `"office_id":"`+service.AssistantUsageOffice+`"`) {
		t.Fatalf("usage ไม่มีแถวผู้ช่วยคอนโซล: %s", raw)
	}
}

func TestAssistant_DirectAnswerUsesOneCall(t *testing.T) {
	llm := &assistantLLM{direct: "กดเมนูหลังบ้านลูกค้า แล้วกด + สร้างกลุ่มครับ"}
	r := assistantServer(t, llm, true)
	_, events := ask(t, r, consoleToken, `{"messages":[{"role":"user","text":"สร้างกลุ่มยังไง"}]}`)
	if len(find(events, "token")) != 1 || len(find(events, "done")) != 1 {
		t.Fatalf("events = %+v", events)
	}
	if llm.calls != 1 || len(llm.requests) != 1 {
		t.Fatalf("ไม่ต้องใช้ข้อมูล = เรียก LLM ครั้งเดียว ได้ %d", len(llm.requests))
	}
}

func TestAssistant_ToolsFollowPermissions(t *testing.T) {
	// viewer: office.view + usage.view — ไม่มี audit.view
	llm := &assistantLLM{
		uses:  []port.ToolUse{{ID: "t1", Name: "recent_activity", Input: map[string]any{}}},
		reply: []string{"ok"},
	}
	r := assistantServer(t, llm, true)
	ask(t, r, viewerToken(t), oneQuestion)

	names := toolNames(llm.first().Tools)
	if names["recent_activity"] || !names["list_domains"] || !names["get_usage"] {
		t.Fatalf("เครื่องมือของ viewer = %v", names)
	}
	// LLM ขอเครื่องมือที่ไม่ได้ให้ → ถูกปฏิเสธที่ฝั่ง server อีกชั้น
	llm.mu.Lock()
	final := llm.requests[len(llm.requests)-1]
	llm.mu.Unlock()
	res := final.Messages[len(final.Messages)-1].ToolResults
	if len(res) != 1 || !strings.Contains(res[0].Content, "ไม่มีสิทธิ์") {
		t.Fatalf("ต้องปฏิเสธ ได้ %+v", res)
	}
}

func TestAssistant_OnlyReadOnlyTools(t *testing.T) {
	llm := &assistantLLM{direct: "ok"}
	r := assistantServer(t, llm, true)
	ask(t, r, consoleToken, oneQuestion) // admin เห็นครบทุกตัว
	readOnly := map[string]bool{"list_domains": true, "get_usage": true, "get_verification_stats": true, "get_settings": true, "recent_activity": true}
	for n := range toolNames(llm.first().Tools) {
		if !readOnly[n] {
			t.Fatalf("มีเครื่องมือที่ไม่ได้อยู่ในรายการอ่านอย่างเดียว: %s", n)
		}
	}
	if len(llm.first().Tools) != len(readOnly) {
		t.Fatalf("admin ต้องเห็นครบ %d ตัว ได้ %d", len(readOnly), len(llm.first().Tools))
	}
}

func TestAssistant_NeedsQuestionLast(t *testing.T) {
	r := assistantServer(t, &assistantLLM{direct: "x"}, true)
	_, events := ask(t, r, consoleToken, `{"messages":[{"role":"user","text":"a"},{"role":"assistant","text":"b"}]}`)
	if len(find(events, "error")) != 1 {
		t.Fatalf("ไม่มีคำถามปิดท้ายต้อง error ได้ %+v", events)
	}
}

func TestGuide_ServedFromOneSource(t *testing.T) {
	r := assistantServer(t, &assistantLLM{}, false)
	code, p := consoleJSON(t, r, "GET", "/api/ai/admin/guide", "", viewerToken(t))
	md, _ := p["markdown"].(string)
	if code != 200 || md != docs.Guide || !strings.Contains(md, "## เริ่มต้นติดตั้ง {#start}") {
		t.Fatalf("guide = %d len=%d", code, len(md))
	}
	// ทุกหัวข้อหลักต้องมี id ไว้ทำสารบัญ
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") && !strings.Contains(line, "{#") {
			t.Fatalf("หัวข้อไม่มี id: %s", line)
		}
	}
}
