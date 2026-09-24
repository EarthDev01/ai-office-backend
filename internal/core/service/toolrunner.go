package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ToolRunner เรียก endpoint ของ host ตาม connector ด้วยกุญแจดอกเล็ก (V7)
//
// ลำดับ: ตรวจสิทธิ์ (จากตั๋ว) → ตรวจ input → ขอกุญแจ → ยิง host (timeout ต่อ call) → ตรวจ schema
// → แคช (เฉพาะ summary ≤ 60 วิ) → คำนวณการ์ด/model_context
type ToolRunner struct {
	Host   port.HostClient
	Scoped port.ScopedTokenSource
	Cache  port.Cache
	Now    port.Clock
}

// ToolRun = ผลของการรัน tool 1 ครั้ง
type ToolRun struct {
	Name    string
	Input   map[string]any
	Outcome connector.Outcome
	NoCard  bool // input ผิด ฯลฯ — ไม่มีการ์ด (โมเดลถามผู้ใช้ใหม่)
	Calls   []domain.ToolCall
}

func (r *ToolRunner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *ToolRunner) Run(ctx context.Context, office domain.Office, conn *connector.Connector, t domain.AccessTicket, tool *connector.Tool, raw map[string]any, st domain.Settings) ToolRun {
	run := ToolRun{Name: tool.Name}
	rule, ok := conn.Rule(tool.Permission)
	if !ok || !rule.Allows(t.User.Permissions, t.User.Level) {
		run.Outcome = deniedOutcome(tool, rule, r.now())
		return run
	}
	input, err := tool.ValidateInput(raw)
	if err != nil {
		run.NoCard = true
		run.Outcome = connector.Outcome{Status: connector.StatusError, ErrorCode: "bad_input",
			ModelContext: map[string]any{"status": connector.StatusError, "error": "bad_input", "detail": err.Error()}}
		return run
	}
	run.Input = input
	loc, cutoff := conn.Location()
	rc := connector.RenderContext{ServiceID: t.ServiceID, Input: input, Now: r.now(), Loc: loc, Cutoff: cutoff}

	if !conn.Host.IsBrowser() && office.BackofficeAPIURL == "" {
		run.Outcome = conn.Evaluate(tool, failAll(tool, "host_not_configured"), rc, r.now())
		return run
	}

	results := make(map[string]*connector.CallResult, len(tool.Calls))
	var mu sync.Mutex
	var wg sync.WaitGroup
	calls := make([]domain.ToolCall, len(tool.Calls))
	fetched := r.now()
	for i := range tool.Calls {
		cl := tool.Calls[i]
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, rec, at := r.call(ctx, office, conn, t, tool, cl, rc, st)
			mu.Lock()
			results[cl.ID] = res
			calls[i] = rec
			if !at.IsZero() && at.Before(fetched) {
				fetched = at
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	run.Calls = calls
	run.Outcome = conn.Evaluate(tool, results, rc, fetched)
	return run
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

func failAll(tool *connector.Tool, code string) map[string]*connector.CallResult {
	out := map[string]*connector.CallResult{}
	for _, c := range tool.Calls {
		out[c.ID] = &connector.CallResult{ID: c.ID, Err: errors.New(code), ErrCode: code}
	}
	return out
}

type cacheEntry struct {
	Data      json.RawMessage `json:"data"`
	FetchedAt int64           `json:"fetched_at"`
}

func (r *ToolRunner) call(ctx context.Context, office domain.Office, conn *connector.Connector, t domain.AccessTicket,
	tool *connector.Tool, cl connector.Call, rc connector.RenderContext, st domain.Settings) (*connector.CallResult, domain.ToolCall, time.Time) {

	rec := domain.ToolCall{Tool: tool.Name, Endpoint: cl.Path, Method: cl.Method} // บันทึก template ไม่ใช่ค่าจริง (ไม่มี PII ใน log)
	res := &connector.CallResult{ID: cl.ID}
	fail := func(code string, err error) (*connector.CallResult, domain.ToolCall, time.Time) {
		res.Err, res.ErrCode = err, code
		rec.OK, rec.Error = false, code
		return res, rec, time.Time{}
	}

	path, err := rc.Render(cl.Path)
	if err != nil {
		return fail("bad_input", err)
	}
	query := map[string]string{}
	for k, v := range cl.Query {
		s, err := rc.Render(v)
		if err != nil {
			return fail("bad_input", err)
		}
		if s != "" {
			query[k] = s
		}
	}
	var body any
	if cl.Body != nil {
		b, err := rc.RenderValue(map[string]any(cl.Body))
		if err != nil {
			return fail("bad_input", err)
		}
		body = b
	}

	var cacheKey string
	if cl.CacheTTL > 0 && tool.Freshness != "live" && r.Cache != nil {
		cacheKey = toolCacheKey(t, tool.Name, cl.ID, cl.Method, path, query, body)
		if b, ok, _ := r.Cache.Get(ctx, cacheKey); ok {
			var ce cacheEntry
			if json.Unmarshal(b, &ce) == nil {
				var data any
				if json.Unmarshal(ce.Data, &data) == nil {
					res.Data, res.Cached = data, true
					rec.OK, rec.Cached = true, true
					return res, rec, time.UnixMilli(ce.FetchedAt)
				}
			}
		}
	}

	timeout := time.Duration(firstPositive(cl.TimeoutMs, conn.Host.HostAPI.TimeoutMs, st.ToolTimeoutMs)) * time.Millisecond
	started := r.now()
	var resp port.HostResponse
	if conn.Host.IsBrowser() {
		// โหมด browser: widget ยิง API เดิมของหลังบ้านด้วย token ของแอดมินเอง (backend ไม่ถือ token/กุญแจใด ๆ)
		relay := relayFrom(ctx)
		if relay == nil {
			return fail("relay_unavailable", errRelayUnavailable)
		}
		resp, err = relay(ctx, RelayRequest{Method: cl.Method, Path: path, Query: query, Body: body, Timeout: timeout})
		if errors.Is(err, errRelayTimeout) {
			return fail("timeout", err)
		}
	} else {
		resp, err = r.doWithToken(ctx, office, conn, t, cl.Method, office.BackofficeAPIURL+path, query, body, timeout)
	}
	rec.Ms = resp.Ms
	rec.Status = resp.Status
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "timeout") {
			return fail("timeout", err)
		}
		if errors.Is(err, errScopedToken) {
			return fail("scoped_token", err)
		}
		return fail("http_error", err)
	}
	d := connector.DecodeResponse(conn.Host.HostAPI, cl, resp.Status, resp.Body)
	if d.Err != nil {
		return fail(d.ErrCode, d.Err)
	}
	if d.NotFound {
		res.NotFound = true
		rec.OK = true
		return res, rec, started
	}
	data := d.Data
	res.Data = data
	rec.OK = true
	if cacheKey != "" {
		if raw, err := json.Marshal(data); err == nil {
			b, _ := json.Marshal(cacheEntry{Data: raw, FetchedAt: started.UnixMilli()})
			_ = r.Cache.Set(ctx, cacheKey, b, time.Duration(cl.CacheTTL)*time.Second)
		}
	}
	return res, rec, started
}

var errScopedToken = errors.New("scoped token unavailable")

// doWithToken ยิงพร้อมกุญแจดอกเล็ก · host ตอบ 401 = กุญแจหมด/ถูกยกเลิก → ขอใหม่แล้วลองอีก 1 ครั้ง
func (r *ToolRunner) doWithToken(ctx context.Context, office domain.Office, conn *connector.Connector, t domain.AccessTicket,
	method, fullURL string, query map[string]string, body any, timeout time.Duration) (port.HostResponse, error) {

	for attempt := 0; attempt < 2; attempt++ {
		tok, err := r.Scoped.Token(ctx, office, conn, t)
		if err != nil {
			return port.HostResponse{}, fmt.Errorf("%w: %v", errScopedToken, err)
		}
		resp, err := r.Host.Do(ctx, port.HostRequest{
			Method:  method,
			URL:     fullURL,
			Header:  map[string]string{conn.Host.HostAPI.ScopedTokenHeader: tok, "Accept": "application/json"},
			Query:   query,
			Body:    body,
			Timeout: timeout,
		})
		if err == nil && resp.Status == 401 && attempt == 0 {
			r.Scoped.Invalidate(office, t)
			continue
		}
		return resp, err
	}
	return port.HostResponse{}, errScopedToken
}

// key ของแคชผูก office+service+ชุดสิทธิ์ของผู้ใช้ — ผู้ใช้สิทธิ์ต่างกันไม่เห็นแคชของกันและกัน
func toolCacheKey(t domain.AccessTicket, tool, call, method, path string, query map[string]string, body any) string {
	perms := append([]string(nil), t.User.Permissions...)
	sort.Strings(perms)
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var q strings.Builder
	for _, k := range keys {
		q.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(query[k]) + "&")
	}
	b, _ := json.Marshal(body)
	// โหมด browser: ข้อมูลมาจาก token ของแอดมินคนนั้น — แคชแยกรายคน/รายรอบล็อกอิน
	who := ""
	if t.TokenFP != "" {
		who = "|u=" + t.User.ID + "|tfp=" + t.TokenFP
	}
	h := sha256.Sum256([]byte(method + " " + path + "?" + q.String() + "|" + string(b) + "|" + strings.Join(perms, ",") + fmt.Sprintf("|lvl=%d", t.User.Level) + who))
	return "ai:cache:" + t.OfficeID + ":" + t.ServiceID + ":" + tool + ":" + call + ":" + hex.EncodeToString(h[:12])
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 6000
}

var thaiQueryRe = regexp.MustCompile(`^[\p{Thai}A-Za-z0-9 ._/()-]{1,80}$`)
