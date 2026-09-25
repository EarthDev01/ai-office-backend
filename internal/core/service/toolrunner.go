package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
)

// Emitter ส่ง SSE event ให้ widget — ต้องปลอดภัยเมื่อเรียกพร้อมกันหลาย goroutine
type Emitter func(event string, data any) error

const defaultCallTimeout = 20 * time.Second

// ToolRun คือผลของ tool 1 ตัว
//
// NoCard = input ไม่ผ่าน (ให้ LLM ถามผู้ใช้ใหม่) · Outcome.ModelContext คือสิ่งเดียวที่ LLM เห็นจากผลนี้
type ToolRun struct {
	Outcome connector.Outcome
	NoCard  bool
	Calls   []domain.ToolCall
}

// ToolRunner เช็คสิทธิ์ → ตรวจ input → render request จาก template → ยิงผ่าน relay (ขนานกันทุก call)
// → ถอดซอง/ตรวจ schema → cache (เฉพาะ tool ที่ไม่ใช่ live) → คำนวณการ์ดและ model_context
type ToolRunner struct {
	relay *Relay
	cache *ttlCache
	now   func() time.Time
}

func NewToolRunner(relay *Relay) *ToolRunner {
	return &ToolRunner{relay: relay, cache: newTTLCache(), now: time.Now}
}

func (r *ToolRunner) Run(ctx context.Context, conn *connector.Connector, t domain.ChatTicket, tool *connector.Tool,
	raw map[string]any, emit Emitter) ToolRun {

	// เช็คสิทธิ์ก่อนยิง — ไม่ผ่านไม่ยิง API เลย
	rule, ok := conn.Rule(tool.Permission)
	if !ok || !rule.Allows(t.Permissions, int(t.Level)) {
		return ToolRun{Outcome: deniedOutcome(tool, rule, r.now())}
	}

	input, err := tool.ValidateInput(raw)
	if err != nil {
		return ToolRun{NoCard: true, Outcome: connector.Outcome{Status: connector.StatusError, ErrorCode: "bad_input",
			ModelContext: map[string]any{"status": connector.StatusError, "error": "bad_input", "detail": err.Error()}}}
	}
	loc, cutoff := conn.Location()
	rc := connector.RenderContext{ServiceID: t.ServiceID, Input: input, Now: r.now(), Loc: loc, Cutoff: cutoff}

	results := make(map[string]*connector.CallResult, len(tool.Calls))
	calls := make([]domain.ToolCall, len(tool.Calls))
	fetched := r.now()
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range tool.Calls {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cl := tool.Calls[i]
			res, rec, at := r.call(ctx, conn, t, tool, cl, rc, emit)
			mu.Lock()
			defer mu.Unlock()
			results[cl.ID] = res
			calls[i] = rec
			if !at.IsZero() && at.Before(fetched) {
				fetched = at // การ์ดบอกเวลาของข้อมูลที่เก่าที่สุด (กรณีมาจาก cache)
			}
		}(i)
	}
	wg.Wait()
	return ToolRun{Outcome: conn.Evaluate(tool, results, rc, fetched), Calls: calls}
}

func deniedOutcome(tool *connector.Tool, rule connector.PermissionRule, now time.Time) connector.Outcome {
	hint := rule.GrantHint
	if hint == "" {
		hint = "ให้ผู้ดูแลระบบของหลังบ้านเปิดสิทธิ์ให้บัญชีของคุณ"
	}
	note := "บัญชีนี้ไม่มีสิทธิ์ดูข้อมูลส่วนนี้"
	if rule.Menu != "" {
		note += " (เมนู \"" + rule.Menu + "\")"
	}
	return connector.Outcome{
		Status:    connector.StatusDenied,
		ErrorCode: "denied",
		Card: domain.Card{ID: domain.NewID(), Kind: connector.StatusDenied, Tool: tool.Name, Title: tool.Card.Title,
			Fields: []domain.CardField{}, Note: note + " — " + hint, FetchedAt: now},
		ModelContext: map[string]any{"status": connector.StatusDenied, "menu": rule.Menu, "grant_hint": hint},
	}
}

type cachedData struct {
	data any
	at   time.Time
}

func (r *ToolRunner) call(ctx context.Context, conn *connector.Connector, t domain.ChatTicket, tool *connector.Tool,
	cl connector.Call, rc connector.RenderContext, emit Emitter) (*connector.CallResult, domain.ToolCall, time.Time) {

	rec := domain.ToolCall{Tool: tool.Name, Endpoint: cl.Path, Method: cl.Method} // template ไม่ใช่ค่าจริง (ไม่มี PII)
	res := &connector.CallResult{ID: cl.ID}
	fail := func(code string, err error) (*connector.CallResult, domain.ToolCall, time.Time) {
		res.Err, res.ErrCode = err, code
		rec.OK, rec.Error = false, code
		return res, rec, time.Time{}
	}

	req := RelayRequest{Method: cl.Method, Query: map[string]string{}}
	var err error
	if req.Path, err = rc.Render(cl.Path); err != nil {
		return fail("bad_input", err)
	}
	for k, v := range cl.Query {
		s, err := rc.Render(v)
		if err != nil {
			return fail("bad_input", err)
		}
		if s != "" {
			req.Query[k] = s
		}
	}
	if cl.Body != nil {
		if req.Body, err = rc.RenderValue(map[string]any(cl.Body)); err != nil {
			return fail("bad_input", err)
		}
	}

	// cache ผูกกับตั๋ว (ผู้ใช้ + รอบล็อกอิน + สิทธิ์) — คนอื่นไม่เห็นผลของกันและกัน
	key := ""
	if cl.CacheTTL > 0 && tool.Freshness != "live" {
		b, _ := json.Marshal(req) // map ถูกเรียง key ตอน marshal — key คงที่
		key = t.ID + "|" + tool.Name + "|" + cl.ID + "|" + string(b)
		if v, ok := r.cache.get(key); ok {
			c := v.(cachedData)
			res.Data, res.Cached = c.data, true
			rec.OK, rec.Cached = true, true
			return res, rec, c.at
		}
	}

	timeout := defaultCallTimeout
	if ms := firstPositive(cl.TimeoutMs, conn.Host.HostAPI.TimeoutMs); ms > 0 {
		timeout = time.Duration(ms) * time.Millisecond
	}
	started := r.now()
	out, err := r.relay.Fetch(ctx, t.ID, req, timeout, emit)
	rec.Ms = r.now().Sub(started).Milliseconds()
	rec.Status = out.Status
	switch {
	case errors.Is(err, ErrRelayTimeout):
		return fail("timeout", err)
	case err != nil:
		return fail("http_error", err)
	case out.Status == 0:
		return fail("http_error", errors.New("widget ยิงหลังบ้านไม่สำเร็จ"))
	}

	d := connector.DecodeResponse(conn.Host.HostAPI, cl, out.Status, []byte(out.Body))
	if d.Err != nil {
		return fail(d.ErrCode, d.Err)
	}
	rec.OK = true
	if d.NotFound {
		res.NotFound = true
		return res, rec, started
	}
	res.Data = d.Data
	if key != "" {
		r.cache.put(key, cachedData{data: d.Data, at: started}, time.Duration(cl.CacheTTL)*time.Second)
	}
	return res, rec, started
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}
