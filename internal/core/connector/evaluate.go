package connector

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-office-backend/internal/core/domain"
)

// ผลของการยิง call 1 ครั้ง (หลังตัด envelope แล้ว)
type CallResult struct {
	ID       string
	Data     any
	NotFound bool
	Err      error
	ErrCode  string // timeout | http_error | host_error | schema_mismatch | denied | unauthorized | bad_json
	Status   int
	Ms       int64
	Cached   bool
}

const (
	StatusOK       = "ok"
	StatusNotFound = "not_found"
	StatusError    = "error"
	StatusDenied   = "denied"
)

// Outcome = ผลของ tool 1 ตัว: การ์ด (ไปหาผู้ใช้ตรง) + model_context (สิ่งเดียวที่โมเดลเห็น)
type Outcome struct {
	Status       string
	ErrorCode    string
	Message      string // ข้อความคงที่จาก connector (not_found / denied)
	Card         domain.Card
	ModelContext map[string]any
	// ค่าทุกตัวที่ขึ้นการ์ด (display + raw ที่เป็น string/ตัวเลข) — output guard ใช้ตัดถ้าโมเดลพิมพ์ซ้ำ
	Sensitive []string
}

// Evaluate สร้าง Outcome จากผลของทุก call · ไม่มี I/O
func (c *Connector) Evaluate(t *Tool, results map[string]*CallResult, rc RenderContext, fetchedAt time.Time) Outcome {
	fm := Formatter{Statuses: c.Statuses, Loc: rc.Loc}
	base := domain.Card{
		ID:        domain.NewID(),
		Tool:      t.Name,
		Title:     t.Card.Title,
		FetchedAt: fetchedAt,
		Fields:    []domain.CardField{},
	}
	if t.Card.Link != nil {
		p, _ := rc.Render(t.Card.Link.Path)
		base.Link = &domain.CardLink{Label: t.Card.Link.Label, Path: p}
	}
	cached := true
	for _, cl := range t.Calls {
		r := results[cl.ID]
		if r == nil || !r.Cached {
			cached = false
		}
	}
	base.Cached = cached

	// 1) call ที่ล้ม (ไม่ใช่ optional) = ทั้ง tool ดึงไม่ได้ — ห้ามมีตัวเลขบางส่วนโผล่ (B-5 · AC-30)
	for _, cl := range t.Calls {
		r := results[cl.ID]
		if r == nil {
			if cl.Optional {
				continue
			}
			return errorOutcome(base, "missing")
		}
		if r.Err != nil && !cl.Optional {
			if r.ErrCode == "denied" {
				base.Kind = StatusDenied
				return Outcome{Status: StatusDenied, ErrorCode: "denied", Card: base, ModelContext: map[string]any{"status": StatusDenied}}
			}
			return errorOutcome(base, r.ErrCode)
		}
	}
	for _, cl := range t.Calls {
		if r := results[cl.ID]; r != nil && r.NotFound && !cl.Optional {
			return notFoundOutcome(base, t)
		}
	}

	// 2) values ตามลำดับ
	env := map[string]any{"input": toAnyMap(rc.Input)}
	values := map[string]any{}
	for _, v := range t.Values {
		val, err := evalValue(v, results, env)
		if err != nil {
			return errorOutcome(base, "eval_error")
		}
		if val == nil && v.Default != nil {
			val = v.Default
		}
		values[v.Name] = val
		env[v.Name] = val
	}

	// 3) not_found ที่ประกาศไว้ (เช่น list ว่าง) — ไม่ใช่ error และไม่ใช่ 0 (B-11)
	if t.NotFound != nil {
		if e, err := ParseExpr(t.NotFound.When); err == nil {
			if r, err := e.Eval(env); err == nil && Truthy(r) {
				return notFoundOutcome(base, t)
			}
		}
	}

	// 4) การ์ด
	card := base
	card.Kind = StatusOK
	var sensitive []string
	for _, fs := range t.Card.Fields {
		if fs.When != "" {
			e, _ := ParseExpr(fs.When)
			if e == nil {
				continue
			}
			if r, _ := e.Eval(env); !Truthy(r) {
				continue
			}
		}
		raw := values[fs.Value]
		disp := fm.Format(raw, fs.Format)
		if raw != nil {
			disp = fs.Prefix + disp + fs.Suffix
		}
		card.Fields = append(card.Fields, domain.CardField{Label: fs.Label, Display: disp, Value: raw, Format: fs.Format})
		sensitive = appendSensitive(sensitive, raw, disp, fs.Format)
	}
	if ts := t.Card.Table; ts != nil {
		rows, _ := values[ts.Rows].([]any)
		max := ts.MaxRows
		if max <= 0 {
			max = 50
		}
		tbl := &domain.CardTable{Columns: make([]domain.CardColumn, len(ts.Columns)), Rows: [][]domain.CardCell{}}
		for i, col := range ts.Columns {
			tbl.Columns[i] = domain.CardColumn{Label: col.Label, Format: col.Format}
		}
		for i, row := range rows {
			if i >= max {
				break
			}
			cells := make([]domain.CardCell, len(ts.Columns))
			for j, col := range ts.Columns {
				raw := GetPath(row, col.Field)
				disp := fm.Format(raw, col.Format)
				if raw != nil {
					disp += col.Suffix
				}
				cells[j] = domain.CardCell{Display: disp, Value: raw}
				sensitive = appendSensitive(sensitive, raw, disp, col.Format)
			}
			tbl.Rows = append(tbl.Rows, cells)
		}
		card.Table = tbl
	}
	card.Note = t.Card.Note

	// 5) model_context — ประเภทจำกัด (bool / ป้ายจากตาราง / ข้อความคงที่) ตรวจแล้วตอนโหลด
	mc := map[string]any{"status": StatusOK}
	for _, cs := range t.ModelContext {
		switch cs.Type {
		case "bool":
			e, _ := ParseExpr(cs.Expr)
			if e != nil {
				r, _ := e.Eval(env)
				mc[cs.Name] = Truthy(r)
			}
		case "labels":
			list, _ := values[cs.From].([]any)
			seen := map[string]bool{}
			labels := []string{}
			table := strings.TrimPrefix(cs.Format, "status:")
			for _, it := range list {
				l := fm.StatusLabel(table, GetPath(it, cs.Pluck))
				if !fm.KnownStatus(table, GetPath(it, cs.Pluck)) {
					// รหัสที่ไม่มีในตาราง: การ์ดโชว์รหัสได้ แต่โมเดลต้องไม่เห็นตัวเลข (B-4)
					l = "สถานะที่ยังไม่มีคำอธิบายในระบบ"
				}
				if !seen[l] {
					seen[l] = true
					labels = append(labels, l)
				}
			}
			mc[cs.Name] = labels
		case "text":
			mc[cs.Name] = cs.Text
		}
	}
	return Outcome{Status: StatusOK, Card: card, ModelContext: mc, Sensitive: sensitive}
}

func errorOutcome(base domain.Card, code string) Outcome {
	base.Kind = StatusError
	base.Note = "ดึงข้อมูลไม่ได้ในตอนนี้ — เปิดหน้าจริงเพื่อตรวจสอบ"
	if code == "" {
		code = "error"
	}
	return Outcome{Status: StatusError, ErrorCode: code, Card: base, ModelContext: map[string]any{"status": StatusError, "error": code}}
}

func notFoundOutcome(base domain.Card, t *Tool) Outcome {
	msg := "ไม่พบข้อมูลที่ถาม"
	if t.NotFound != nil && t.NotFound.Message != "" {
		msg = t.NotFound.Message
	}
	base.Kind = StatusNotFound
	base.Note = msg
	return Outcome{Status: StatusNotFound, Message: msg, Card: base, ModelContext: map[string]any{"status": StatusNotFound, "message": msg}}
}

func appendSensitive(list []string, raw any, disp, format string) []string {
	// ป้ายจากตารางสถานะไม่ใช่ข้อมูลอ่อนไหว โมเดลพูดได้
	if strings.HasPrefix(format, "status:") || format == "bool" {
		return list
	}
	if disp != "" && disp != "—" {
		list = append(list, disp)
	}
	switch x := raw.(type) {
	case string:
		if strings.TrimSpace(x) != "" {
			list = append(list, x)
		}
	case float64:
		list = append(list, strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", x), "0"), "."))
	}
	return list
}

func toAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func evalValue(v ValueSpec, results map[string]*CallResult, env map[string]any) (any, error) {
	var cur any
	if v.From != "" {
		r := results[v.From]
		if r == nil || r.Err != nil {
			return nil, nil
		}
		cur = GetPath(r.Data, v.Path)
	}
	if v.Filter != "" {
		list, _ := cur.([]any)
		e, err := ParseExpr(v.Filter)
		if err != nil {
			return nil, err
		}
		out := []any{}
		for _, it := range list {
			local := copyEnv(env)
			local["item"] = it
			r, err := e.Eval(local)
			if err != nil {
				return nil, err
			}
			if Truthy(r) {
				out = append(out, it)
			}
		}
		cur = out
	}
	if v.GroupBy != "" {
		cur = groupBy(cur, v.GroupBy, v.Field)
	}
	if v.SortBy != "" {
		if list, ok := cur.([]any); ok {
			sorted := append([]any(nil), list...)
			sort.SliceStable(sorted, func(i, j int) bool {
				a, b := GetPath(sorted[i], v.SortBy), GetPath(sorted[j], v.SortBy)
				less := lessAny(a, b)
				if v.Order == "desc" {
					return lessAny(b, a)
				}
				return less
			})
			cur = sorted
		}
	}
	if v.Limit > 0 {
		if list, ok := cur.([]any); ok && len(list) > v.Limit {
			cur = list[:v.Limit]
		}
	}
	if v.Agg != "" {
		cur = aggregate(cur, v.Agg, v.Field)
	}
	if v.Expr != "" {
		e, err := ParseExpr(v.Expr)
		if err != nil {
			return nil, err
		}
		local := copyEnv(env)
		if v.From != "" {
			local["value"] = cur
		}
		r, err := e.Eval(local)
		if err != nil {
			return nil, err
		}
		cur = r
	}
	return cur, nil
}

func copyEnv(env map[string]any) map[string]any {
	out := make(map[string]any, len(env)+1)
	for k, v := range env {
		out[k] = v
	}
	return out
}

func groupBy(cur any, key, field string) any {
	list, ok := cur.([]any)
	if !ok {
		return []any{}
	}
	type bucket struct {
		key   any
		count float64
		sum   float64
	}
	var order []string
	buckets := map[string]*bucket{}
	for _, it := range list {
		k := GetPath(it, key)
		ks := fmt.Sprint(k)
		b, ok := buckets[ks]
		if !ok {
			b = &bucket{key: k}
			buckets[ks] = b
			order = append(order, ks)
		}
		b.count++
		if field != "" {
			if f, ok := ToFloat(GetPath(it, field)); ok {
				b.sum += f
			}
		}
	}
	out := make([]any, 0, len(order))
	for _, ks := range order {
		b := buckets[ks]
		row := map[string]any{"key": b.key, "count": b.count}
		if field != "" {
			row["sum"] = b.sum
		}
		out = append(out, row)
	}
	return out
}

func aggregate(cur any, agg, field string) any {
	list, ok := cur.([]any)
	if !ok {
		if agg == "count" {
			if cur == nil {
				return float64(0)
			}
			return float64(1)
		}
		return cur
	}
	pick := func(it any) any {
		if field == "" {
			return it
		}
		return GetPath(it, field)
	}
	switch agg {
	case "count":
		return float64(len(list))
	case "first":
		if len(list) == 0 {
			return nil
		}
		return pick(list[0])
	case "last":
		if len(list) == 0 {
			return nil
		}
		return pick(list[len(list)-1])
	}
	var nums []float64
	for _, it := range list {
		if f, ok := ToFloat(pick(it)); ok {
			nums = append(nums, f)
		}
	}
	if len(nums) == 0 {
		if agg == "sum" {
			return float64(0)
		}
		return nil
	}
	switch agg {
	case "sum":
		t := 0.0
		for _, n := range nums {
			t += n
		}
		return t
	case "avg":
		t := 0.0
		for _, n := range nums {
			t += n
		}
		return t / float64(len(nums))
	case "min":
		m := nums[0]
		for _, n := range nums {
			if n < m {
				m = n
			}
		}
		return m
	case "max":
		m := nums[0]
		for _, n := range nums {
			if n > m {
				m = n
			}
		}
		return m
	}
	return nil
}

func lessAny(a, b any) bool {
	af, aok := ToFloat(a)
	bf, bok := ToFloat(b)
	if aok && bok {
		return af < bf
	}
	return fmt.Sprint(a) < fmt.Sprint(b)
}
