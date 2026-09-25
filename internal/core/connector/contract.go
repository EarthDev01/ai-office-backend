package connector

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"ai-office-backend/internal/core/domain"

	"gopkg.in/yaml.v3"
)

// Contract test (C10) = connectors/<kind>/contract-tests/*.yaml
//
// แต่ละไฟล์ = 1 tool · แต่ละ case = response ตัวอย่างของ host (ตามที่ catalog ของ host บอก) → การ์ดที่คาดหวัง
// วิ่งผ่าน DecodeResponse + Evaluate ตัวเดียวกับตอนรันจริง (ไม่มี I/O) — host เปลี่ยนรูป response แล้วลืมแก้ connector = test แดง
type ContractFile struct {
	Tool  string         `yaml:"tool"`
	Cases []ContractCase `yaml:"cases"`
	File  string         `yaml:"-"`
}

type ContractCase struct {
	Name      string                      `yaml:"name"`
	Now       string                      `yaml:"now"`     // RFC3339 (ไม่ใส่ = 2026-09-24T14:32:00+07:00)
	Service   string                      `yaml:"service"` // ไม่ใส่ = DEMO-SVC
	Input     map[string]any              `yaml:"input"`
	Responses map[string]ContractResponse `yaml:"responses"` // call id → response ของ host
	Expect    ContractExpect              `yaml:"expect"`
}

type ContractResponse struct {
	Status int    `yaml:"status"` // ไม่ใส่ = 200
	Body   any    `yaml:"body"`   // JSON ตามที่ host ตอบจริง (เขียนเป็น yaml ได้)
	Raw    string `yaml:"raw"`    // body ดิบ (ใช้ทดสอบ JSON พัง)
	Error  string `yaml:"error"`  // จำลองยิงไม่สำเร็จ: timeout | http_error
}

type ContractExpect struct {
	Status       string                `yaml:"status"` // ok | not_found | error | denied
	ErrorCode    string                `yaml:"error_code"`
	Fields       map[string]string     `yaml:"fields"`    // label → display (ต้องตรงตัว)
	NoFields     []string              `yaml:"no_fields"` // label ที่ต้องไม่ขึ้น
	TableRows    *int                  `yaml:"table_rows"`
	TableCells   [][]string            `yaml:"table_cells"` // display ทีละแถว (เทียบเฉพาะแถวที่ให้มา)
	ModelContext map[string]any        `yaml:"model_context"`
	Link         string                `yaml:"link"`
	Calls        map[string]ExpectCall `yaml:"calls"` // call id → request ที่ต้องยิง
}

type ExpectCall struct {
	Path  string            `yaml:"path"`
	Query map[string]string `yaml:"query"`
	Body  map[string]any    `yaml:"body"`
}

// Location = timezone + เวลาตัดวันของ connector (B-6)
func (c *Connector) Location() (*time.Location, time.Duration) {
	loc := domain.Bangkok()
	if c.Host.Timezone != "" {
		if l, err := time.LoadLocation(c.Host.Timezone); err == nil {
			loc = l
		}
	}
	var cutoff time.Duration
	if s := c.Host.DayCutoff; s != "" {
		if t, err := time.Parse("15:04", s); err == nil {
			cutoff = time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
		}
	}
	return loc, cutoff
}

// LoadContracts อ่าน contract-tests/*.yaml ของ connector (ไม่มีโฟลเดอร์ = ไม่มี test)
func (c *Connector) LoadContracts() ([]ContractFile, error) {
	files, _ := filepath.Glob(filepath.Join(c.Dir, "contract-tests", "*.yaml"))
	sort.Strings(files)
	var out []ContractFile
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var cf ContractFile
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		dec.KnownFields(true)
		if err := dec.Decode(&cf); err != nil {
			return nil, fmt.Errorf("%s: %w", rel(c.Dir, f), err)
		}
		cf.File = rel(c.Dir, f)
		out = append(out, cf)
	}
	return out, nil
}

// RunContract รันทุก case ของไฟล์ · คืน error ทีละข้อที่ไม่ตรง
func (c *Connector) RunContract(cf ContractFile) []error {
	var errs []error
	t, ok := c.Tool(cf.Tool)
	if !ok {
		return []error{fmt.Errorf("%s: ไม่มี tool %q", cf.File, cf.Tool)}
	}
	if len(cf.Cases) == 0 {
		return []error{fmt.Errorf("%s: ไม่มี case", cf.File)}
	}
	for _, cs := range cf.Cases {
		for _, e := range c.runCase(t, cs) {
			errs = append(errs, fmt.Errorf("%s › %s: %w", cf.File, cs.Name, e))
		}
	}
	return errs
}

func (c *Connector) runCase(t *Tool, cs ContractCase) []error {
	var errs []error
	bad := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }

	now := time.Date(2026, 9, 24, 14, 32, 0, 0, domain.Bangkok())
	if cs.Now != "" {
		p, err := time.Parse(time.RFC3339, cs.Now)
		if err != nil {
			return []error{fmt.Errorf("now: %w", err)}
		}
		now = p
	}
	svc := cs.Service
	if svc == "" {
		svc = "DEMO-SVC"
	}
	input, err := t.ValidateInput(cs.Input)
	if err != nil {
		return []error{fmt.Errorf("input ไม่ผ่าน: %w", err)}
	}
	loc, cutoff := c.Location()
	rc := RenderContext{ServiceID: svc, Input: input, Now: now, Loc: loc, Cutoff: cutoff}

	results := map[string]*CallResult{}
	for _, cl := range t.Calls {
		if exp, ok := cs.Expect.Calls[cl.ID]; ok {
			checkRequest(rc, cl, exp, bad)
		}
		resp, ok := cs.Responses[cl.ID]
		if !ok {
			if !cl.Optional {
				bad("ไม่มี response ของ call %q", cl.ID)
			}
			continue
		}
		if resp.Error != "" {
			results[cl.ID] = &CallResult{ID: cl.ID, Err: errors.New(resp.Error), ErrCode: resp.Error}
			continue
		}
		body := []byte(resp.Raw)
		if resp.Raw == "" {
			body, err = json.Marshal(normalizeYAML(resp.Body))
			if err != nil {
				bad("body ของ %q แปลงเป็น JSON ไม่ได้: %v", cl.ID, err)
				continue
			}
		}
		status := resp.Status
		if status == 0 {
			status = 200
		}
		d := DecodeResponse(c.Host.HostAPI, cl, status, body)
		results[cl.ID] = &CallResult{ID: cl.ID, Data: d.Data, NotFound: d.NotFound, Err: d.Err, ErrCode: d.ErrCode, Status: status}
	}
	for id := range cs.Responses {
		found := false
		for _, cl := range t.Calls {
			found = found || cl.ID == id
		}
		if !found {
			bad("responses มี call %q ที่ tool ไม่มี", id)
		}
	}

	out := c.Evaluate(t, results, rc, now)
	ex := cs.Expect
	if ex.Status != "" && out.Status != ex.Status {
		bad("status = %q (error_code %q) อยากได้ %q", out.Status, out.ErrorCode, ex.Status)
	}
	if ex.ErrorCode != "" && out.ErrorCode != ex.ErrorCode {
		bad("error_code = %q อยากได้ %q", out.ErrorCode, ex.ErrorCode)
	}
	got := map[string]string{}
	for _, f := range out.Card.Fields {
		got[f.Label] = f.Display
	}
	for label, want := range ex.Fields {
		if g, ok := got[label]; !ok {
			bad("ไม่มี field %q บนการ์ด (มี %v)", label, keys(got))
		} else if g != want {
			bad("field %q = %q อยากได้ %q", label, g, want)
		}
	}
	for _, label := range ex.NoFields {
		if _, ok := got[label]; ok {
			bad("field %q ต้องไม่ขึ้น", label)
		}
	}
	rows := 0
	if out.Card.Table != nil {
		rows = len(out.Card.Table.Rows)
	}
	if ex.TableRows != nil && rows != *ex.TableRows {
		bad("ตารางมี %d แถว อยากได้ %d", rows, *ex.TableRows)
	}
	for i, want := range ex.TableCells {
		if out.Card.Table == nil || i >= len(out.Card.Table.Rows) {
			bad("ไม่มีแถวที่ %d", i+1)
			continue
		}
		row := out.Card.Table.Rows[i]
		for j, w := range want {
			if j >= len(row) || row[j].Display != w {
				g := ""
				if j < len(row) {
					g = row[j].Display
				}
				bad("ตาราง แถว %d คอลัมน์ %d = %q อยากได้ %q", i+1, j+1, g, w)
			}
		}
	}
	for k, want := range ex.ModelContext {
		if g, ok := out.ModelContext[k]; !ok || !reflect.DeepEqual(normalizeYAML(g), normalizeYAML(want)) {
			bad("model_context[%q] = %#v อยากได้ %#v", k, out.ModelContext[k], want)
		}
	}
	if ex.Link != "" && (out.Card.Link == nil || out.Card.Link.Path != ex.Link) {
		p := ""
		if out.Card.Link != nil {
			p = out.Card.Link.Path
		}
		bad("link = %q อยากได้ %q", p, ex.Link)
	}

	// ข้อบังคับที่ต้องจริงทุก case (ไม่ต้องเขียนใน yaml)
	if out.Status != StatusOK && len(out.Card.Fields) > 0 {
		bad("การ์ดสถานะ %q ห้ามมีตัวเลข/field (ห้ามแสดงบางส่วน · B-5)", out.Status)
	}
	if out.Card.Link == nil {
		bad("การ์ดต้องมีลิงก์หน้าจริงเสมอ")
	}
	checkModelContext(out, bad)
	return errs
}

// model_context ห้ามมีตัวเลข/ค่าบนการ์ด (B-4 · AC-14) — ตรวจจากผลจริงอีกชั้น
func checkModelContext(out Outcome, bad func(string, ...any)) {
	var walk func(k string, v any)
	walk = func(k string, v any) {
		switch x := v.(type) {
		case string:
			if strings.IndexFunc(x, unicode.IsDigit) >= 0 && k != "status" && k != "error" {
				bad("model_context[%q] มีตัวเลข %q", k, x)
			}
			for _, s := range out.Sensitive {
				if len([]rune(s)) >= 3 && strings.Contains(x, s) {
					bad("model_context[%q] มีค่าบนการ์ด %q", k, s)
				}
			}
		case float64, int, int64:
			bad("model_context[%q] เป็นตัวเลข", k)
		case []any:
			for _, it := range x {
				walk(k, it)
			}
		case []string:
			for _, it := range x {
				walk(k, it)
			}
		case map[string]any:
			for kk, it := range x {
				walk(k+"."+kk, it)
			}
		}
	}
	for k, v := range out.ModelContext {
		walk(k, v)
	}
}

func checkRequest(rc RenderContext, cl Call, exp ExpectCall, bad func(string, ...any)) {
	if exp.Path != "" {
		p, err := rc.Render(cl.Path)
		if err != nil || p != exp.Path {
			bad("call %q path = %q อยากได้ %q (%v)", cl.ID, p, exp.Path, err)
		}
	}
	for k, want := range exp.Query {
		tpl, ok := cl.Query[k]
		if !ok {
			bad("call %q ไม่มี query %q", cl.ID, k)
			continue
		}
		g, err := rc.Render(tpl)
		if err != nil || g != want {
			bad("call %q query %q = %q อยากได้ %q (%v)", cl.ID, k, g, want, err)
		}
	}
	if exp.Body != nil {
		g, err := rc.RenderValue(map[string]any(cl.Body))
		gj, _ := json.Marshal(normalizeYAML(g))
		wj, _ := json.Marshal(normalizeYAML(exp.Body))
		if err != nil || string(gj) != string(wj) {
			bad("call %q body = %s อยากได้ %s (%v)", cl.ID, gj, wj, err)
		}
	}
}

// yaml.v3 ให้ map[string]any แล้ว แต่ตัวเลขอาจเป็น int — ทำให้เทียบกับ JSON ได้ตรง
func normalizeYAML(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = normalizeYAML(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[fmt.Sprint(k)] = normalizeYAML(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = normalizeYAML(vv)
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = vv
		}
		return out
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return v
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
