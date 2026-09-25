package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

const (
	ChatMaxTextRunes = 2000
	chatMaxToolUses  = 5
	round1MaxTokens  = 1024
)

// ChatError คือ error ที่ส่งให้ widget ผ่าน SSE event error
type ChatError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ChatError) Error() string { return e.Code + ": " + e.Message }

// ChatRequest — ConversationID ว่าง = เปิดห้องใหม่
type ChatRequest struct {
	ConversationID string
	Text           string
}

// ChatService คือ flow แชททั้งหมด: รอบ 1 เลือก tool → รัน tool (ยิงผ่าน widget) → รอบ 2 เขียนคำตอบ
//
// ค่าที่ปรับได้ (timeout, คนพร้อมกัน, ประวัติ ฯลฯ) อ่านจาก settings ทุกคำถาม — แก้ในคอนโซลแล้วมีผลทันที
type ChatService struct {
	llm      port.LLMClient
	repo     port.ChatRepository
	runner   *ToolRunner
	loc      *time.Location
	settings *SettingsService
	usage    *UsageService
	now      func() time.Time
	semMu    sync.Mutex
	sems     map[string]chan struct{}
}

func NewChatService(llm port.LLMClient, repo port.ChatRepository, runner *ToolRunner, loc *time.Location,
	settings *SettingsService, usage *UsageService) *ChatService {
	return &ChatService{llm: llm, repo: repo, runner: runner, loc: loc, settings: settings, usage: usage,
		now: time.Now, sems: map[string]chan struct{}{}}
}

// Enabled — ไม่มี LLM = ปิดแชท
func (s *ChatService) Enabled() bool { return s != nil && s.llm != nil }

// acquire — ขนาดเปลี่ยนตาม settings ได้: สร้างช่องใหม่ ส่วนคำถามที่กำลังตอบคืนช่องเดิมของตัวเอง
func (s *ChatService) acquire(key string, max int) (func(), bool) {
	s.semMu.Lock()
	sem, ok := s.sems[key]
	if !ok || cap(sem) != max {
		sem = make(chan struct{}, max)
		s.sems[key] = sem
	}
	s.semMu.Unlock()
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, true
	default: // เต็ม = busy ทันที ไม่เข้าคิว
		return nil, false
	}
}

// Handle ตอบคำถาม 1 ข้อ — ส่งทุกอย่างผ่าน emit · คืน *ChatError เมื่อควรแจ้ง widget
func (s *ChatService) Handle(ctx context.Context, office domain.Office, svc domain.Service, conn *connector.Connector,
	t domain.ChatTicket, req ChatRequest, emit Emitter) error {

	// 1. ตรวจข้อความ
	text := strings.TrimSpace(req.Text)
	if text == "" || utf8.RuneCountInString(text) > ChatMaxTextRunes {
		return &ChatError{"bad_request", fmt.Sprintf("ข้อความต้องไม่ว่างและยาวไม่เกิน %d ตัวอักษร", ChatMaxTextRunes)}
	}

	st := s.settings.Get(ctx)

	// 3. จำกัดคนใช้พร้อมกันต่อ service
	release, ok := s.acquire(office.ID+"|"+svc.ID, st.MaxConcurrent)
	if !ok {
		return &ChatError{"busy", "ผู้ช่วยกำลังตอบคนอื่นอยู่หลายคน ลองใหม่อีกครั้งในอีกสักครู่"}
	}
	defer release()

	now := s.now().In(s.loc)

	// 5. ห้องแชท + ประวัติ
	conv, history, isNew, err := s.openConversation(ctx, office, svc, t, req.ConversationID, text, st.HistoryTurns*2)
	if err != nil {
		return err
	}
	if err := s.repo.AppendMessage(ctx, domain.ChatMessage{
		ID: newID("msg"), ConversationID: conv.ID, OfficeID: office.ID, ServiceID: svc.ID,
		Role: "user", Text: text, CreatedAt: s.now(),
	}); err != nil {
		return fmt.Errorf("บันทึกข้อความ: %w", err)
	}

	// 6.
	if err := emit("status", map[string]any{"phase": "thinking", "text": "กำลังคิด…", "conversation_id": conv.ID}); err != nil {
		return err
	}

	ans := &domain.ChatMessage{
		ID: newID("msg"), ConversationID: conv.ID, OfficeID: office.ID, ServiceID: svc.ID, Role: "assistant",
	}
	err = s.answer(ctx, office, svc, conn, st, t, history, text, now, ans, emit)
	ans.CreatedAt = s.now()
	switch {
	case ctx.Err() != nil:
		// ผู้ใช้ปิด/ยกเลิก — บันทึกว่ายกเลิก ไม่ส่ง error
		ans.Status = "aborted"
		err = nil
	case err != nil:
		ans.Status = "error"
	default:
		ans.Status = "ok"
		ans.VerificationStatus = domain.VerifyPending // คำตอบที่ตอบสำเร็จเข้าคิวตรวจทุกข้อ
	}
	// ctx ของ request อาจถูกยกเลิกแล้ว — บันทึกด้วย ctx ใหม่ที่มีเวลาจำกัด
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if serr := s.repo.AppendMessage(saveCtx, *ans); serr != nil {
		log.Printf("[ERROR] บันทึกคำตอบ (office=%s service=%s): %v", office.ID, svc.ID, serr)
	}
	_ = s.repo.TouchConversation(saveCtx, conv.ID, s.now(), 2)
	s.usage.Record(saveCtx, office.ID, svc.ID, ans.CreatedAt, usageDelta(*ans, isNew))
	if err != nil {
		return err
	}
	return emit("done", map[string]any{})
}

func (s *ChatService) openConversation(ctx context.Context, office domain.Office, svc domain.Service, t domain.ChatTicket,
	id, text string, historyLimit int) (domain.Conversation, []domain.ChatMessage, bool, error) {
	if id != "" {
		c, err := s.repo.GetConversation(ctx, id)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return c, nil, false, fmt.Errorf("เปิดห้องแชท: %w", err)
		}
		// ห้องของคนอื่น/เว็บอื่น ถือว่าไม่มี
		if err == nil && c.OfficeID == office.ID && c.ServiceID == svc.ID && c.AdminID == t.AdminID {
			if historyLimit <= 0 {
				return c, nil, false, nil
			}
			h, err := s.repo.RecentMessages(ctx, c.ID, historyLimit)
			if err != nil {
				return c, nil, false, fmt.Errorf("โหลดประวัติ: %w", err)
			}
			return c, h, false, nil
		}
		return c, nil, false, &ChatError{"not_found", "ไม่พบห้องแชทนี้ เริ่มห้องใหม่ได้เลย"}
	}
	title := text
	if utf8.RuneCountInString(title) > 60 {
		title = string([]rune(title)[:60])
	}
	c := domain.Conversation{
		ID: newID("conv"), OfficeID: office.ID, ServiceID: svc.ID, AdminID: t.AdminID, Username: t.Username,
		Title: title, CreatedAt: s.now(), UpdatedAt: s.now(),
	}
	if err := s.repo.CreateConversation(ctx, c); err != nil {
		return c, nil, false, fmt.Errorf("สร้างห้องแชท: %w", err)
	}
	return c, nil, true, nil
}

// usageDelta คือสิ่งที่คำตอบ 1 ข้อบวกเข้าตัวนับรายเดือน/รายวัน
func usageDelta(ans domain.ChatMessage, newConversation bool) domain.RollupDelta {
	u := ans.Usage
	d := domain.RollupDelta{
		InputTokens: int64(u.InputTokens), OutputTokens: int64(u.OutputTokens),
		CacheRead: int64(u.CacheRead), CacheWrite: int64(u.CacheWrite), GuardHits: int64(ans.GuardHits),
	}
	if newConversation {
		d.Conversations = 1
	}
	if u.InputTokens+u.OutputTokens+u.CacheRead+u.CacheWrite > 0 {
		d.Questions = 1 // นับคำถามที่เรียก LLM จริง ไม่ว่ากี่รอบ
	}
	if refusalCategories[ans.Category] {
		d.Refusals = 1
	}
	for _, c := range ans.ToolCalls {
		if !c.OK {
			d.ToolErrors++
		}
	}
	return d
}

// supportMessage — ของ connector ก่อน ไม่มีจึงใช้ของ settings
func supportMessage(conn *connector.Connector, st domain.Settings) string {
	if conn.Host.SupportMessage != "" {
		return conn.Host.SupportMessage
	}
	return st.SupportMessage
}

func (s *ChatService) llmFailed(office domain.Office, svc domain.Service, err error) error {
	// ข้อความเดียวกันทุกสาเหตุ — สาเหตุจริงอยู่ใน log นี้
	log.Printf("[WARN] LLM ล้ม (office=%s service=%s): %v", office.ID, svc.ID, err)
	return &ChatError{"llm_unavailable", "ผู้ช่วยไม่ตอบกลับในเวลาที่กำหนด ลองใหม่อีกครั้ง"}
}

func (s *ChatService) answer(ctx context.Context, office domain.Office, svc domain.Service, conn *connector.Connector,
	st domain.Settings, t domain.ChatTicket, history []domain.ChatMessage, text string, now time.Time, ans *domain.ChatMessage, emit Emitter) error {

	system := systemPrompt(office, svc, conn, t)
	msgs := historyMessages(history)
	msgs = append(msgs, port.LLMMessage{Role: "user",
		Text: text + "\n\n[ระบบ: วันนี้คือ " + now.Format("2006-01-02") + " ตามเวลาไทย]"})
	tools := toolDefs(conn)

	// ---- รอบ 1: เลือก tool ----
	r1ctx, cancel := context.WithTimeout(ctx, time.Duration(st.LLMTimeoutSec)*time.Second)
	r1, err := s.llm.Complete(r1ctx, port.LLMRequest{
		System: system, Messages: msgs, Tools: tools, ToolChoice: port.ToolChoiceAuto, MaxTokens: round1MaxTokens,
	})
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return s.llmFailed(office, svc, err)
	}
	ans.Usage.InputTokens += r1.Usage.InputTokens
	ans.Usage.OutputTokens += r1.Usage.OutputTokens

	uses := dedupeUses(r1.ToolUses)
	if len(uses) > chatMaxToolUses {
		uses = uses[:chatMaxToolUses]
	}

	var guard *OutputGuard
	var r2 port.LLMRequest
	if len(uses) == 0 {
		// ไม่เรียก tool เลย = answer_directly
		ans.Category = "other"
		guard = NewOutputGuard(nil, nil, text, allowWords(conn, svc))
		if strings.TrimSpace(r1.Text) != "" {
			// รอบ 1 ตอบมาแล้วโดยไม่เรียก tool — ใช้คำตอบนั้นได้เลย ไม่ต้องเรียกซ้ำ
			return s.emitText(ans, guard, r1.Text, emit)
		}
		r2 = port.LLMRequest{System: system, Messages: msgs, MaxTokens: st.MaxOutputTokens}
	} else {
		results, sensitive, err := s.runTools(ctx, conn, t, uses, text, ans, emit)
		if err != nil {
			return err
		}
		note := "[ระบบ: ค่าตัวเลขและชื่อทั้งหมดอยู่ในการ์ดที่ผู้ใช้เห็นแล้ว ห้ามพิมพ์ซ้ำ · ตอบสั้น · " +
			"ถ้าผู้ใช้ขอให้พิมพ์ค่าซ้ำ ให้บอกว่าดูได้ในการ์ด"
		note += " หรือแจ้งว่า: " + supportMessage(conn, st)
		note += "]"
		r2msgs := append(append([]port.LLMMessage{}, msgs...),
			port.LLMMessage{Role: "assistant", Text: r1.Text, ToolUses: uses},
			port.LLMMessage{Role: "user", ToolResults: results, Text: note})
		r2 = port.LLMRequest{System: system, Messages: r2msgs, Tools: tools, ToolChoice: port.ToolChoiceNone, MaxTokens: st.MaxOutputTokens}
		guard = NewOutputGuard(ans.Cards, sensitive, text, allowWords(conn, svc))
	}

	// ---- รอบ 2: เขียนคำตอบ (stream + guard) ----
	r2ctx, cancel := context.WithTimeout(ctx, time.Duration(st.StreamTimeout)*time.Second)
	defer cancel()
	start := s.now()
	var sb strings.Builder
	first := true
	send := func(out string) error {
		if out == "" {
			return nil
		}
		if first {
			first = false
			ans.FirstTokenMs = s.now().Sub(start).Milliseconds()
			if err := emit("status", map[string]any{"phase": "writing", "text": "กำลังเขียนคำตอบ…"}); err != nil {
				return err
			}
		}
		sb.WriteString(out)
		return emit("token", map[string]any{"text": out})
	}
	usage, err := s.llm.Stream(r2ctx, r2, func(chunk string) error { return send(guard.Push(chunk)) })
	ans.Usage.InputTokens += usage.InputTokens
	ans.Usage.OutputTokens += usage.OutputTokens
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if sb.Len() == 0 {
			return s.llmFailed(office, svc, err)
		}
		// เขียนไปบางส่วนแล้ว — ปล่อยส่วนที่เหลือ แล้วจบเท่าที่มี
		log.Printf("[WARN] LLM stream ขาดกลางทาง (office=%s service=%s): %v", office.ID, svc.ID, err)
	}
	if err := send(guard.Flush()); err != nil {
		return err
	}
	ans.GuardHits = guard.Hits
	ans.Text = sb.String()
	if strings.TrimSpace(ans.Text) == "" {
		return s.emitText(ans, nil, fallbackText(ans.Cards, supportMessage(conn, st)), emit)
	}
	return nil
}

// emitText ส่งข้อความที่ได้มาครบแล้ว (ไม่ได้ stream) ผ่าน guard แล้วตามด้วย token เดียว
func (s *ChatService) emitText(ans *domain.ChatMessage, guard *OutputGuard, text string, emit Emitter) error {
	if guard != nil {
		text = guard.Push(text) + guard.Flush()
		ans.GuardHits = guard.Hits
	}
	if strings.TrimSpace(text) == "" {
		text = "ขออภัยครับ ตอนนี้ยังตอบคำถามนี้ไม่ได้"
	}
	ans.Text = text
	if err := emit("status", map[string]any{"phase": "writing", "text": "กำลังเขียนคำตอบ…"}); err != nil {
		return err
	}
	return emit("token", map[string]any{"text": text})
}

func fallbackText(cards []domain.Card, support string) string {
	if len(cards) > 0 {
		return "ข้อมูลอยู่ในการ์ดด้านบนครับ"
	}
	return support
}

// runTools รันทุก tool ที่ LLM เลือก (tool ข้อมูลรันขนานกัน) · ส่งการ์ดให้ widget ตามลำดับที่ LLM เรียก
//
// คืน tool_result (สิ่งที่ LLM เห็น) และค่าทั้งหมดที่ขึ้นการ์ด (ให้ guard ตัดถ้า LLM พิมพ์ซ้ำ)
func (s *ChatService) runTools(ctx context.Context, conn *connector.Connector, t domain.ChatTicket, uses []port.ToolUse,
	question string, ans *domain.ChatMessage, emit Emitter) ([]port.ToolResult, []string, error) {

	results := make([]map[string]any, len(uses))
	cards := make([][]domain.Card, len(uses))
	var sensitive []string
	var categories []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, u := range uses {
		if _, ok := conn.Tool(u.Name); ok {
			if err := emit("status", map[string]any{"phase": "fetching", "text": "กำลังดึงข้อมูลจากระบบ…"}); err != nil {
				return nil, nil, err
			}
			break
		}
	}
	for i, u := range uses {
		switch u.Name {
		case "answer_directly":
			cat, _ := u.Input["category"].(string)
			if !containsString(answerCategories, cat) {
				cat = "other"
			}
			categories = append(categories, cat)
			res := map[string]any{"status": "ok", "category": cat}
			// ขอให้ทำรายการ / ถามสิ่งที่ไม่มีเครื่องมือ → บอกทางไปเมนูที่ทำเองได้
			if cat == "action_request" || cat == "no_tool" {
				if menus, card := lookupMenu(conn, t, question, s.now()); menus["status"] == "ok" {
					res["menu_hints"] = menus
					if card != nil {
						cards[i] = append(cards[i], *card)
					}
				}
			}
			results[i] = res
		case "lookup_menu":
			q, _ := u.Input["query"].(string)
			if !thaiQueryRe.MatchString(strings.TrimSpace(q)) {
				q = question
			}
			res, card := lookupMenu(conn, t, q, s.now())
			results[i] = res
			if card != nil {
				cards[i] = append(cards[i], *card)
			}
			categories = append(categories, "guide")
		case "explain_status":
			results[i] = explainStatus(conn, u.Input)
			categories = append(categories, "guide")
		case "host_facts":
			topic, _ := u.Input["topic"].(string)
			results[i] = hostFacts(conn, topic)
			categories = append(categories, "guide")
		default:
			tool, ok := conn.Tool(u.Name)
			if !ok {
				results[i] = map[string]any{"status": "error", "error": "unknown_tool"}
				continue
			}
			categories = append(categories, "data")
			wg.Add(1)
			go func(i int, u port.ToolUse) {
				defer wg.Done()
				run := s.runner.Run(ctx, conn, t, tool, u.Input, emit)
				mu.Lock()
				defer mu.Unlock()
				ans.ToolCalls = append(ans.ToolCalls, run.Calls...)
				res := map[string]any{}
				for k, v := range run.Outcome.ModelContext {
					res[k] = v
				}
				if !run.NoCard {
					res["card"] = map[string]any{"title": run.Outcome.Card.Title, "fields_shown": fieldLabels(run.Outcome.Card)}
					cards[i] = append(cards[i], run.Outcome.Card)
				}
				results[i] = res
				sensitive = append(sensitive, run.Outcome.Sensitive...)
			}(i, u)
		}
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	out := make([]port.ToolResult, len(uses))
	for i, u := range uses {
		for _, c := range cards[i] {
			ans.Cards = append(ans.Cards, c)
			if err := emit("card", c); err != nil {
				return nil, nil, err
			}
		}
		raw, _ := json.Marshal(results[i])
		out[i] = port.ToolResult{ToolUseID: u.ID, Name: u.Name, Content: string(raw)}
	}
	ans.Category = pickCategory(categories)
	return out, sensitive, nil
}

func pickCategory(categories []string) string {
	switch {
	case containsString(categories, "data"):
		return "data"
	case containsString(categories, "guide"):
		for _, c := range categories {
			if refusalCategories[c] {
				return c
			}
		}
		return "guide"
	}
	for _, c := range categories {
		if refusalCategories[c] {
			return c
		}
	}
	if len(categories) > 0 {
		return categories[0]
	}
	return "other"
}

// dedupeUses ตัด tool call ที่ซ้ำกันทั้งชื่อและ input (LLM บางตัวเรียกซ้ำ)
func dedupeUses(uses []port.ToolUse) []port.ToolUse {
	seen := map[string]bool{}
	out := make([]port.ToolUse, 0, len(uses))
	for _, u := range uses {
		in, _ := json.Marshal(u.Input)
		key := u.Name + string(in)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, u)
	}
	return out
}

// fieldLabels บอก LLM ว่าผู้ใช้เห็นอะไรบนการ์ด — ชื่อ field/คอลัมน์เท่านั้น ไม่มีค่า
func fieldLabels(c domain.Card) []string {
	out := []string{}
	for _, f := range c.Fields {
		out = append(out, f.Label)
	}
	if c.Table != nil {
		for _, col := range c.Table.Columns {
			out = append(out, col.Label)
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var thaiQueryRe = regexp.MustCompile(`^[\p{Thai}A-Za-z0-9 ._/()-]{1,80}$`)

func toolDefs(conn *connector.Connector) []port.ToolDef {
	defs := builtinTools(conn)
	for _, t := range conn.Tools {
		props, req := t.InputSchema()
		schema := map[string]any{"type": "object", "properties": props}
		if len(req) > 0 {
			schema["required"] = req
		}
		defs = append(defs, port.ToolDef{Name: t.Name, Description: t.Description, InputSchema: schema})
	}
	return defs
}

func historyMessages(h []domain.ChatMessage) []port.LLMMessage {
	out := make([]port.LLMMessage, 0, len(h))
	for _, m := range h {
		text := m.Text
		if m.Role == "assistant" && len(m.Cards) > 0 {
			titles := make([]string, 0, len(m.Cards))
			for _, c := range m.Cards {
				titles = append(titles, c.Title)
			}
			text = "[แสดงการ์ด: " + strings.Join(titles, ", ") + "]\n" + text
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		// provider ส่วนใหญ่ต้องการ user/assistant สลับกัน — รวมข้อความ role เดียวกันที่ติดกัน
		if n := len(out); n > 0 && out[n-1].Role == m.Role {
			out[n-1].Text += "\n\n" + text
			continue
		}
		out = append(out, port.LLMMessage{Role: m.Role, Text: text})
	}
	// ข้อความถัดไปเป็นของผู้ใช้เสมอ — ท้ายประวัติที่เป็น user (เช่นคำตอบที่ล้มไป) ต้องไม่ซ้อนกัน
	if n := len(out); n > 0 && out[n-1].Role == "user" {
		out = out[:n-1]
	}
	// เริ่มด้วย assistant ไม่ได้ในบาง provider
	for len(out) > 0 && out[0].Role != "user" {
		out = out[1:]
	}
	return out
}

func systemPrompt(office domain.Office, svc domain.Service, conn *connector.Connector, t domain.ChatTicket) string {
	name := svc.DisplayName
	if name == "" {
		name = "ผู้ช่วยหลังบ้าน"
	}
	label := svc.Label
	if label == "" {
		label = svc.ID
	}
	return fmt.Sprintf(`คุณคือ "%s" ผู้ช่วยในหลังบ้านของเว็บ %s ผู้ใช้ที่คุยด้วยคือแอดมิน %s
ตอบเป็นภาษาไทย สุภาพ กระชับ

วิธีทำงาน:
- ถ้าคำถามต้องใช้ข้อมูลในระบบ ให้เรียกเครื่องมือที่ตรงที่สุดเสมอ ห้ามตอบจากความจำหรือเดา
- ถามว่าเมนูอยู่ไหนหรือทำรายการอย่างไร ให้เรียก lookup_menu · ถามความหมายของสถานะ ให้เรียก explain_status
- ถามข้อเท็จจริงของหลังบ้าน (เช่น ปุ่มฉุกเฉิน ทำไมดูรายงานบางอย่างไม่ได้) ให้เรียก host_facts
- ถ้าไม่ต้องใช้ข้อมูล ให้เรียก answer_directly พร้อมระบุประเภทคำถาม
- ถ้าข้อมูลที่ต้องใช้ค้นยังไม่ครบ (เช่น ไม่มียูสเซอร์เนม) ให้เรียก answer_directly แล้วถามผู้ใช้กลับ
- ข้อมูลจริงจะแสดงให้ผู้ใช้เป็นการ์ด คุณจะเห็นเพียงสถานะสรุป ห้ามแต่งตัวเลข ชื่อ หรือรายละเอียดขึ้นเอง
- คุณทำรายการแทนผู้ใช้ไม่ได้ (เช่น แก้ไข อนุมัติ โอนเงิน) ทำได้แค่ดูข้อมูลและอธิบาย`,
		name, label, t.Username)
}

func newID(prefix string) string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
