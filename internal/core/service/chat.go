package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ChatMetrics = จุดวัดผล (Prometheus) · nil ได้
type ChatMetrics interface {
	Message(officeID, serviceID, category string, latency time.Duration, u domain.Usage, guardHits int, failed bool)
	ToolCall(tool string, ok bool, cached bool, ms int64)
}

type ChatDeps struct {
	Connectors port.ConnectorRegistry
	Settings   *SettingsService
	Quota      *QuotaService
	LLM        port.LLM
	Runner     *ToolRunner
	Convs      port.ConversationRepository
	Msgs       port.MessageRepository
	Rollups    port.RollupRepository
	Sem        port.Semaphore
	Metrics    ChatMetrics
	Now        port.Clock
}

type ChatService struct{ ChatDeps }

func NewChatService(d ChatDeps) *ChatService {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &ChatService{ChatDeps: d}
}

// ChatEvent = 1 event ของ SSE (spec §7.2: status · card · token · done · error)
type ChatEvent struct {
	Type string
	Data any
}

type ChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Text           string `json:"text"`
}

// ข้อความ error ที่ผู้ใช้เห็น (spec §8)
var chatErrors = map[string]string{
	"busy":             "ตอนนี้มีคนใช้ผู้ช่วยพร้อมกันเต็มแล้ว กรุณาลองใหม่อีกครั้งในอีกสักครู่",
	"quota_exceeded":   "โควตาของเดือนนี้ใช้ครบแล้ว ติดต่อผู้ดูแลเพื่อเพิ่มโควตา",
	"llm_unavailable":  "ผู้ช่วยไม่ตอบกลับในเวลาที่กำหนด กรุณาลองใหม่อีกครั้ง",
	"bad_request":      "ข้อความว่างหรือยาวเกินไป",
	"not_found":        "ไม่พบห้องแชทนี้",
	"internal":         "เกิดข้อผิดพลาดภายในระบบผู้ช่วย กรุณาลองใหม่อีกครั้ง",
	"connector":        "ผู้ช่วยยังไม่พร้อมใช้งานกับหลังบ้านนี้",
	"service_disabled": "บริการ AI ของเว็บนี้ปิดใช้งานอยู่ ติดต่อผู้ดูแล",
}

func errEvent(code string) ChatEvent {
	return ChatEvent{Type: "error", Data: map[string]any{"code": code, "message": chatErrors[code]}}
}

const (
	maxQuestionRunes = 2000
	maxToolCalls     = 5
)

// Handle ตอบ 1 คำถาม · emit ถูกเรียกเรียงลำดับเสมอ (ไม่พร้อมกัน)
func (s *ChatService) Handle(ctx context.Context, office domain.Office, svc domain.Service, t domain.AccessTicket, req ChatRequest, emit func(ChatEvent)) {
	started := s.Now()
	text := strings.TrimSpace(req.Text)
	if text == "" || utf8.RuneCountInString(text) > maxQuestionRunes {
		emit(errEvent("bad_request"))
		return
	}
	conn, ok := s.Connectors.Get(office.Kind)
	if !ok {
		emit(errEvent("connector"))
		return
	}
	if conn.Host.IsBrowser() && s.Runner != nil {
		// โหมด browser: tool ให้ widget ยิง API เดิมของหลังบ้านแทน (ดู relay.go)
		ctx = WithRelay(ctx, NewRelay(s.Runner.Cache, t.ID, emit, s.Now))
	}
	st, _ := s.Settings.Get(ctx)

	// จำกัดพร้อมกันต่อ service — เกินลิมิตปฏิเสธทันที ไม่เข้าคิว (spec §8 · AC-16)
	if s.Sem != nil {
		ttl := time.Duration(st.LLMTimeoutSec*2+30) * time.Second
		release, ok, err := s.Sem.Acquire(ctx, "ai:conc:"+office.ID+":"+svc.ID, st.MaxConcurrent, ttl)
		if err != nil {
			emit(errEvent("internal"))
			return
		}
		if !ok {
			emit(errEvent("busy"))
			return
		}
		defer release()
	}
	if s.Quota != nil {
		if err := s.Quota.Allow(ctx, office, svc); err != nil {
			if errors.Is(err, domain.ErrQuotaExceeded) {
				emit(errEvent("quota_exceeded"))
			} else {
				emit(errEvent("internal"))
			}
			return
		}
	}

	conv, history, err := s.openConversation(ctx, office, svc, t, req.ConversationID, text, st)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			emit(errEvent("not_found"))
		} else {
			emit(errEvent("internal"))
		}
		return
	}
	emit(ChatEvent{Type: "status", Data: map[string]any{"phase": "thinking", "text": "กำลังตรวจสอบคำถาม…", "conversation_id": conv.ID}})

	userMsg := domain.Message{
		ID: domain.NewID(), OfficeID: office.ID, ServiceID: svc.ID, ConversationID: conv.ID,
		UserID: t.User.ID, UserName: t.User.Username, Role: domain.RoleUser, Text: text,
		Cards: []domain.Card{}, ToolCalls: []domain.ToolCall{}, CreatedAt: s.Now(),
	}
	_ = s.Msgs.Insert(ctx, userMsg)

	ans := &answer{
		msg: domain.Message{
			ID: domain.NewID(), OfficeID: office.ID, ServiceID: svc.ID, ConversationID: conv.ID,
			UserID: t.User.ID, UserName: t.User.Username, Role: domain.RoleAssistant,
			Cards: []domain.Card{}, ToolCalls: []domain.ToolCall{}, Model: st.Model,
			VerificationStatus: domain.VerifyPending,
		},
	}
	defer func() {
		s.finish(context.WithoutCancel(ctx), office, svc, conv, ans, started, st, text)
	}()

	loc, _ := conn.Location()
	system := systemPrompt(conn, svc, st.SupportMessage)
	tools := s.llmTools(conn)
	today := s.Now().In(loc).Format("2006-01-02")
	userTurn := port.LLMMessage{Role: "user", Blocks: []port.LLMBlock{
		{Type: port.BlockText, Text: text},
		{Type: port.BlockText, Text: "[ระบบ: วันนี้คือ " + today + " ตามเวลาไทย · " + round1Instruction + "]"},
	}}
	msgs := append(history, userTurn)

	// ---- รอบ 1: เลือกเครื่องมือ (บังคับเรียกอย่างน้อย 1 ตัว — ไม่มีข้อความหลุดก่อนได้ข้อมูล · B-5) ----
	llmCtx, cancel := context.WithTimeout(ctx, time.Duration(st.LLMTimeoutSec)*time.Second)
	r1, err := s.LLM.Complete(llmCtx, port.LLMRequest{
		Model: st.Model, System: system, Messages: msgs, Tools: tools, ToolChoice: "any",
		MaxTokens: 1024, Effort: st.Effort,
	})
	cancel()
	ans.msg.Usage = ans.msg.Usage.Add(r1.Usage)
	if err != nil {
		if ctx.Err() != nil {
			ans.msg.Aborted = true
			return
		}
		log.Printf("[WARN] LLM ล้ม (office=%s service=%s): %v", office.ID, svc.ID, err)
		ans.fail("llm_unavailable")
		emit(errEvent("llm_unavailable"))
		return
	}
	uses := toolUses(r1.Blocks)
	if len(uses) == 0 {
		// โมเดลไม่เรียก tool (เช่น refusal) → ตอบแบบไม่มีข้อมูล
		uses = []port.LLMBlock{{Type: port.BlockToolUse, ToolUseID: "auto_" + domain.NewID(), ToolName: "answer_directly", Input: map[string]any{"category": "other"}}}
	}
	if len(uses) > maxToolCalls {
		uses = uses[:maxToolCalls]
	}

	// ---- รัน tool ----
	results, cards, sensitive, category := s.runTools(ctx, office, conn, t, uses, st, text, emit, ans)
	ans.msg.Category = category
	for _, c := range cards {
		ans.msg.Cards = append(ans.msg.Cards, c)
		emit(ChatEvent{Type: "card", Data: c})
	}
	if ctx.Err() != nil {
		ans.msg.Aborted = true
		return
	}

	repeated := s.isRepeat(history, text)

	// ---- รอบ 2: เขียนคำตอบ (stream ผ่าน output guard) ----
	assistantTurn := port.LLMMessage{Role: "assistant", Blocks: uses}
	var resultBlocks []port.LLMBlock
	for _, u := range uses {
		b, _ := json.Marshal(results[u.ToolUseID])
		resultBlocks = append(resultBlocks, port.LLMBlock{Type: port.BlockToolResult, ToolUseID: u.ToolUseID, Content: string(b)})
	}
	note := "[ระบบ: ค่าตัวเลขและชื่อทั้งหมดอยู่ในการ์ดที่ผู้ใช้เห็นแล้ว ห้ามพิมพ์ซ้ำ · ตอบสั้น]"
	if repeated {
		note = "[ระบบ: ผู้ใช้ถามเรื่องเดิมซ้ำ ให้หยุดวน บอกว่าข้อมูลเป็นไปตามการ์ด และให้ช่องทางติดต่อ support: " + st.SupportMessage + "]"
	}
	resultBlocks = append(resultBlocks, port.LLMBlock{Type: port.BlockText, Text: note})
	msgs2 := append(append(msgs, assistantTurn), port.LLMMessage{Role: "user", Blocks: resultBlocks})

	allow := allowWords(conn, svc)
	allow = append(allow, strings.Fields(text)...) // คำที่ผู้ใช้พิมพ์เองพูดทวนได้
	first := time.Time{}
	guard := NewOutputGuard(allow, sensitive, func(chunk string) {
		if first.IsZero() {
			first = s.Now()
			emit(ChatEvent{Type: "status", Data: map[string]any{"phase": "writing", "text": "กำลังเรียบเรียงคำตอบ…"}})
		}
		emit(ChatEvent{Type: "token", Data: map[string]any{"text": chunk}})
	})
	llmCtx2, cancel2 := context.WithTimeout(ctx, time.Duration(st.LLMTimeoutSec)*time.Second)
	r2, err := s.LLM.Stream(llmCtx2, port.LLMRequest{
		Model: st.Model, System: system, Messages: msgs2, Tools: tools, ToolChoice: "none",
		MaxTokens: st.MaxOutputTokens, Effort: st.Effort,
	}, guard.Write)
	cancel2()
	guard.Flush()
	ans.msg.Usage = ans.msg.Usage.Add(r2.Usage)
	ans.msg.GuardHits = guard.Hits()
	ans.msg.Text = guard.Text()
	if !first.IsZero() {
		ans.msg.FirstTokenMs = first.Sub(started).Milliseconds()
	}
	if err != nil {
		if ctx.Err() != nil {
			ans.msg.Aborted = true
			return
		}
		log.Printf("[WARN] LLM ล้ม (office=%s service=%s): %v", office.ID, svc.ID, err)
		ans.fail("llm_unavailable")
		emit(errEvent("llm_unavailable"))
		return
	}
	if strings.TrimSpace(ans.msg.Text) == "" {
		fallback := "ข้อมูลอยู่ในการ์ดด้านบนครับ"
		if len(cards) == 0 {
			fallback = "ขออภัยครับ ตอนนี้ผมตอบคำถามนี้ไม่ได้ · " + st.SupportMessage
		}
		ans.msg.Text = fallback
		emit(ChatEvent{Type: "token", Data: map[string]any{"text": fallback}})
	}
	ans.done = true
	emit(ChatEvent{Type: "done", Data: map[string]any{
		"message_id": ans.msg.ID, "conversation_id": conv.ID,
		"usage":      map[string]any{"in": ans.msg.Usage.In + ans.msg.Usage.CacheRead + ans.msg.Usage.CacheWrite, "out": ans.msg.Usage.Out},
		"latency_ms": s.Now().Sub(started).Milliseconds(),
	}})
}

type answer struct {
	msg  domain.Message
	done bool
}

func (a *answer) fail(code string) {
	a.msg.Error = code
	if a.msg.Text == "" {
		a.msg.Text = chatErrors[code]
	}
}

// finish บันทึก 1 แถว + หักโควตา + rollup + metrics (ทำเสมอ แม้ผู้ใช้ปิดหน้าไปแล้ว)
func (s *ChatService) finish(ctx context.Context, office domain.Office, svc domain.Service, conv domain.Conversation, a *answer, started time.Time, st domain.Settings, question string) {
	now := s.Now()
	a.msg.CreatedAt = now
	a.msg.LatencyMs = now.Sub(started).Milliseconds()
	cost := domain.ComputeCost(a.msg.Usage, st.Pricing)
	a.msg.Cost = cost
	_ = s.Msgs.Insert(ctx, a.msg)
	_ = s.Convs.Touch(ctx, office.ID, svc.ID, conv.ID, now, "")
	if s.Quota != nil && a.msg.Usage.Total() > 0 {
		s.Quota.Record(ctx, office, svc, a.msg.Usage, cost.Amount)
	}
	if s.Rollups != nil {
		d := domain.RollupDelta{
			Questions: 1, TokensIn: a.msg.Usage.In + a.msg.Usage.CacheRead + a.msg.Usage.CacheWrite, TokensOut: a.msg.Usage.Out,
			CostAmount: cost.Amount, LatencyMs: a.msg.LatencyMs, GuardHits: int64(a.msg.GuardHits),
		}
		if conv.MessageCount == 0 {
			d.Conversations = 1
		}
		if refusalCategories[a.msg.Category] {
			d.Refusals = 1
		}
		for _, tc := range a.msg.ToolCalls {
			if !tc.OK {
				d.ToolErrors++
			}
		}
		_ = s.Rollups.Add(ctx, office.ID, svc.ID, domain.DateOf(now), d)
	}
	if s.Metrics != nil {
		s.Metrics.Message(office.ID, svc.ID, a.msg.Category, now.Sub(started), a.msg.Usage, a.msg.GuardHits, a.msg.Error != "")
	}
}

func (s *ChatService) llmTools(conn *connector.Connector) []port.LLMTool {
	out := builtinTools(conn)
	for _, t := range conn.Tools {
		props, req := t.InputSchema()
		out = append(out, port.LLMTool{Name: t.Name, Description: t.Description, Properties: props, Required: req})
	}
	return out
}

func toolUses(blocks []port.LLMBlock) []port.LLMBlock {
	var out []port.LLMBlock
	seen := map[string]bool{}
	for _, b := range blocks {
		if b.Type != port.BlockToolUse {
			continue
		}
		in, _ := json.Marshal(b.Input)
		key := b.ToolName + string(in)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, b)
	}
	return out
}

// runTools รันทุก tool ที่โมเดลเลือก (พร้อมกัน) · คืน tool_result (สำหรับโมเดล) + การ์ด (สำหรับผู้ใช้)
func (s *ChatService) runTools(ctx context.Context, office domain.Office, conn *connector.Connector, t domain.AccessTicket,
	uses []port.LLMBlock, st domain.Settings, question string, emit func(ChatEvent), a *answer) (map[string]any, []domain.Card, []string, string) {

	results := make(map[string]any, len(uses))
	cardsByIdx := make([][]domain.Card, len(uses))
	var sensitive []string
	var mu sync.Mutex
	categories := []string{}
	dataTools := 0
	for _, u := range uses {
		if _, ok := conn.Tool(u.ToolName); ok {
			dataTools++
		}
	}
	if dataTools > 0 {
		emit(ChatEvent{Type: "status", Data: map[string]any{"phase": "fetching", "text": "กำลังดึงข้อมูลจากระบบ…"}})
	}
	var wg sync.WaitGroup
	for i, u := range uses {
		switch u.ToolName {
		case "answer_directly":
			cat, _ := u.Input["category"].(string)
			if !containsString(answerCategories, cat) {
				cat = "other"
			}
			categories = append(categories, cat)
			res := map[string]any{"status": "ok", "category": cat}
			if cat == "action_request" || cat == "no_tool" {
				if menus, card := lookupMenu(conn, t, question, s.Now()); menus["status"] == "ok" {
					res["menu_hints"] = menus
					if card != nil {
						cardsByIdx[i] = append(cardsByIdx[i], *card)
					}
				}
			}
			results[u.ToolUseID] = res
		case "lookup_menu":
			q, _ := u.Input["query"].(string)
			if !thaiQueryRe.MatchString(strings.TrimSpace(q)) {
				q = question
			}
			res, card := lookupMenu(conn, t, q, s.Now())
			results[u.ToolUseID] = res
			if card != nil {
				cardsByIdx[i] = append(cardsByIdx[i], *card)
			}
			categories = append(categories, "guide")
		case "explain_status":
			results[u.ToolUseID] = explainStatus(conn, u.Input)
			categories = append(categories, "guide")
		case "host_facts":
			topic, _ := u.Input["topic"].(string)
			results[u.ToolUseID] = hostFacts(conn, topic)
			categories = append(categories, "guide")
		default:
			tool, ok := conn.Tool(u.ToolName)
			if !ok {
				results[u.ToolUseID] = map[string]any{"status": "error", "error": "unknown_tool"}
				continue
			}
			categories = append(categories, "data")
			wg.Add(1)
			go func(i int, u port.LLMBlock, tool *connector.Tool) {
				defer wg.Done()
				run := s.Runner.Run(ctx, office, conn, t, tool, u.Input, st)
				mu.Lock()
				defer mu.Unlock()
				a.msg.ToolCalls = append(a.msg.ToolCalls, run.Calls...)
				for _, c := range run.Calls {
					if s.Metrics != nil {
						s.Metrics.ToolCall(c.Tool, c.OK, c.Cached, c.Ms)
					}
				}
				res := map[string]any{}
				for k, v := range run.Outcome.ModelContext {
					res[k] = v
				}
				if !run.NoCard {
					res["card"] = map[string]any{"title": run.Outcome.Card.Title, "fields_shown": fieldLabels(run.Outcome.Card)}
					cardsByIdx[i] = append(cardsByIdx[i], run.Outcome.Card)
				}
				results[u.ToolUseID] = res
				sensitive = append(sensitive, run.Outcome.Sensitive...)
			}(i, u, tool)
		}
	}
	wg.Wait()
	var cards []domain.Card
	for _, cs := range cardsByIdx {
		cards = append(cards, cs...)
	}
	category := "other"
	switch {
	case containsString(categories, "data"):
		category = "data"
	case containsString(categories, "guide") && !containsAny(categories, refusalKeys()):
		category = "guide"
	case len(categories) > 0:
		category = categories[0]
		for _, c := range categories {
			if refusalCategories[c] {
				category = c
				break
			}
		}
	}
	return results, cards, sensitive, category
}

func refusalKeys() []string {
	out := make([]string, 0, len(refusalCategories))
	for k := range refusalCategories {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

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
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func containsAny(list, wants []string) bool {
	for _, w := range wants {
		if containsString(list, w) {
			return true
		}
	}
	return false
}

// openConversation เปิดห้องเดิม (ต้องเป็นของผู้ใช้คนนี้ใน service นี้ และยังไม่ปิด) หรือสร้างใหม่
func (s *ChatService) openConversation(ctx context.Context, office domain.Office, svc domain.Service, t domain.AccessTicket, id, text string, st domain.Settings) (domain.Conversation, []port.LLMMessage, error) {
	now := s.Now()
	if id != "" {
		c, err := s.Convs.Get(ctx, office.ID, svc.ID, id)
		if err != nil {
			return domain.Conversation{}, nil, err
		}
		if !ownsConversation(c, t) {
			return domain.Conversation{}, nil, domain.ErrNotFound
		}
		if c.ClosedAt == nil && now.Sub(c.OpenedAt) < 7*24*time.Hour {
			hist, err := s.history(ctx, office, svc, c, st)
			return c, hist, err
		}
	}
	c := domain.Conversation{
		ID: domain.NewID(), OfficeID: office.ID, ServiceID: svc.ID, Kind: office.Kind,
		UserID: t.User.ID, UserName: t.User.Username, TokenFP: t.TokenFP, Title: truncateRunes(text, 60),
		OpenedAt: now, LastMessageAt: now, CreatedAt: now,
	}
	if err := s.Convs.Create(ctx, c); err != nil {
		return domain.Conversation{}, nil, err
	}
	return c, nil, nil
}

// history = ข้อความก่อนหน้าแบบข้อความล้วน — การ์ดใส่แค่ชื่อ ไม่ใส่ค่า (B-4)
func (s *ChatService) history(ctx context.Context, office domain.Office, svc domain.Service, c domain.Conversation, st domain.Settings) ([]port.LLMMessage, error) {
	if st.HistoryTurns <= 0 {
		return nil, nil
	}
	list, err := s.Msgs.ListByConversation(ctx, office.ID, svc.ID, c.ID, st.HistoryTurns*2)
	if err != nil {
		return nil, err
	}
	var out []port.LLMMessage
	for _, m := range list {
		txt := strings.TrimSpace(m.Text)
		if m.Role == domain.RoleAssistant {
			var titles []string
			for _, cd := range m.Cards {
				titles = append(titles, cd.Title)
			}
			if len(titles) > 0 {
				txt += "\n[การ์ดที่แสดงไปแล้ว: " + strings.Join(titles, ", ") + "]"
			}
		}
		if txt == "" {
			continue
		}
		role := "user"
		if m.Role == domain.RoleAssistant {
			role = "assistant"
		}
		// API ต้องสลับ user/assistant — รวมข้อความติดกันที่ role เดียวกัน
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Blocks[0].Text += "\n" + txt
			continue
		}
		out = append(out, port.LLMMessage{Role: role, Blocks: []port.LLMBlock{{Type: port.BlockText, Text: txt}}})
	}
	// ต้องขึ้นต้นด้วย user และจบด้วย assistant ก่อนต่อคำถามใหม่
	for len(out) > 0 && out[0].Role != "user" {
		out = out[1:]
	}
	if n := len(out); n > 0 && out[n-1].Role == "user" {
		out = out[:n-1]
	}
	return out, nil
}

// isRepeat = คำถามเดิมซ้ำ (ครั้งที่ 2) → หยุดวนและบอกช่องทาง (B-13 · AC-12)
func (s *ChatService) isRepeat(history []port.LLMMessage, text string) bool {
	cur := normSearch(text)
	if utf8.RuneCountInString(cur) < 4 {
		return false
	}
	for _, m := range history {
		if m.Role != "user" {
			continue
		}
		for _, line := range strings.Split(m.Blocks[0].Text, "\n") {
			prev := normSearch(line)
			if prev == cur || similarity(prev, cur) >= 0.85 {
				return true
			}
		}
	}
	return false
}

// similarity = Jaccard ของ bigram ตัวอักษร (ภาษาไทยไม่มีเว้นวรรค)
func similarity(a, b string) float64 {
	ga, gb := bigrams(a), bigrams(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	inter := 0
	for g := range ga {
		if gb[g] {
			inter++
		}
	}
	union := len(ga) + len(gb) - inter
	return float64(inter) / float64(union)
}

func bigrams(s string) map[string]bool {
	r := []rune(s)
	out := map[string]bool{}
	for i := 0; i+1 < len(r); i++ {
		out[string(r[i:i+2])] = true
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
