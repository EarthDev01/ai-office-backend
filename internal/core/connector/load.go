package connector

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ReservedToolNames = tool กลางที่ core ให้ทุก kind (อ่านจาก menus/statuses/facts ของ connector)
var ReservedToolNames = map[string]bool{
	"lookup_menu": true, "explain_status": true, "host_facts": true, "answer_directly": true,
}

var (
	toolNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,60}$`)
	ruleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)
	kindRe     = regexp.MustCompile(`^[a-z][a-z0-9-]{1,40}$`)
	cutoffRe   = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)
)

// LoadError บอกไฟล์ + บรรทัด (V5: connector ผิดรูป → boot fail บอกไฟล์/บรรทัด)
type LoadError struct {
	File string
	Line int
	Msg  string
}

func (e *LoadError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Msg)
}

// LoadDir โหลดทุก kind ใต้ root (1 โฟลเดอร์ = 1 kind ที่มี host.yaml)
func LoadDir(root string) (map[string]*Connector, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("อ่านโฟลเดอร์ connectors %q ไม่ได้: %w", root, err)
	}
	out := map[string]*Connector{}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "host.yaml")); err != nil {
			continue
		}
		c, err := Load(dir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out[c.Kind] = c
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

// Load โหลดและตรวจ connector 1 kind
func Load(dir string) (*Connector, error) {
	c := &Connector{Dir: dir, Kind: filepath.Base(dir), toolIndex: map[string]*Tool{}}
	var errs []error
	add := func(file string, line int, format string, a ...any) {
		errs = append(errs, &LoadError{File: rel(dir, file), Line: line, Msg: fmt.Sprintf(format, a...)})
	}

	hostPath := filepath.Join(dir, "host.yaml")
	if err := decodeStrict(hostPath, &c.Host); err != nil {
		return nil, err
	}
	for _, f := range []struct {
		name string
		dst  any
	}{
		{"permissions.yaml", &c.Permissions},
		{"statuses.yaml", &c.Statuses},
		{"menus.yaml", &c.Menus},
	} {
		if err := decodeStrict(filepath.Join(dir, f.name), f.dst); err != nil {
			return nil, err
		}
	}

	c.validateHost(hostPath, add)
	c.validatePermissions(filepath.Join(dir, "permissions.yaml"), add)
	c.validateStatuses(filepath.Join(dir, "statuses.yaml"), add)
	c.validateMenus(filepath.Join(dir, "menus.yaml"), add)

	files, _ := filepath.Glob(filepath.Join(dir, "questions", "*.yaml"))
	sort.Strings(files)
	for _, f := range files {
		tools, err := decodeQuestionFile(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, t := range tools {
			if _, dup := c.toolIndex[t.Name]; dup {
				add(f, t.Line, "tool %q ซ้ำกับไฟล์อื่น", t.Name)
				continue
			}
			c.validateTool(t, add)
			c.toolIndex[t.Name] = t
			c.Tools = append(c.Tools, t)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

func rel(dir, file string) string {
	if r, err := filepath.Rel(filepath.Dir(dir), file); err == nil {
		return filepath.ToSlash(r)
	}
	return file
}

// decodeStrict = field ที่ไม่รู้จัก = error พร้อมบรรทัด (กันพิมพ์ชื่อ field ผิดแล้วเงียบ)
func decodeStrict(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return &LoadError{File: path, Msg: "อ่านไฟล์ไม่ได้: " + err.Error()}
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(dst); err != nil {
		return &LoadError{File: path, Msg: err.Error()}
	}
	return nil
}

func decodeQuestionFile(path string) ([]*Tool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, &LoadError{File: path, Msg: err.Error()}
	}
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil, &LoadError{File: path, Msg: err.Error()}
	}
	var qf QuestionFile
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&qf); err != nil {
		return nil, &LoadError{File: path, Msg: err.Error()}
	}
	lines := toolLines(&root)
	for i, t := range qf.Tools {
		t.File = path
		if i < len(lines) {
			t.Line = lines[i]
		}
		if len(t.Questions) == 0 {
			t.Questions = qf.Questions
		}
	}
	return qf.Tools, nil
}

// toolLines หาเลขบรรทัดของแต่ละ tool ใน sequence "tools"
func toolLines(root *yaml.Node) []int {
	if root == nil || len(root.Content) == 0 {
		return nil
	}
	doc := root.Content[0]
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "tools" {
			var out []int
			for _, n := range doc.Content[i+1].Content {
				out = append(out, n.Line)
			}
			return out
		}
	}
	return nil
}

type addFn func(file string, line int, format string, a ...any)

func (c *Connector) validateHost(file string, add addFn) {
	h := &c.Host
	if !kindRe.MatchString(c.Kind) {
		add(file, 0, "ชื่อโฟลเดอร์ kind %q ต้องเป็น a-z 0-9 -", c.Kind)
	}
	if h.Kind != c.Kind {
		add(file, 0, "kind %q ต้องตรงกับชื่อโฟลเดอร์ %q", h.Kind, c.Kind)
	}
	pa := h.PageAuth
	switch pa.Token.Source {
	case "localStorage", "sessionStorage":
	default:
		add(file, 0, "page_auth.token.source ต้องเป็น localStorage|sessionStorage")
	}
	if pa.Token.Key == "" {
		add(file, 0, "page_auth.token.key ว่าง")
	}
	switch pa.Token.Format {
	case "raw":
	case "json-expiration":
		if pa.Token.ValueField == "" {
			add(file, 0, "page_auth.token.value_field ต้องมีเมื่อ format=json-expiration")
		}
	default:
		add(file, 0, "page_auth.token.format ต้องเป็น raw|json-expiration")
	}
	switch pa.Service.Source {
	case "localStorage", "sessionStorage", "query":
	default:
		add(file, 0, "page_auth.service.source ต้องเป็น localStorage|sessionStorage|query")
	}
	if pa.Service.Key == "" {
		add(file, 0, "page_auth.service.key ว่าง")
	}
	switch pa.Service.Encoding {
	case "", "none", "base64":
	default:
		add(file, 0, "page_auth.service.encoding ต้องเป็น none|base64")
	}
	if pa.HostAPIBase == "" {
		add(file, 0, "page_auth.host_api_base ว่าง")
	}
	switch h.Mode {
	case "", "host":
		if !strings.Contains(pa.SessionPath, "{service}") || !strings.HasPrefix(pa.SessionPath, "/") {
			add(file, 0, "page_auth.session_path ต้องขึ้นต้นด้วย / และมี {service}")
		}
		if !strings.HasPrefix(h.HostAPI.ScopedTokenPath, "/") {
			add(file, 0, "host_api.scoped_token_path ต้องขึ้นต้นด้วย /")
		}
		if h.HostAPI.ScopedTokenHeader == "" {
			add(file, 0, "host_api.scoped_token_header ว่าง")
		}
		if strings.EqualFold(h.HostAPI.ScopedTokenHeader, "Authorization") {
			add(file, 0, "host_api.scoped_token_header ห้ามเป็น Authorization (contract §4)")
		}
	case "browser":
		c.validateIdentity(file, pa.Identity, add)
		for _, hs := range pa.ExtraHeaders {
			if hs.Name == "" || strings.EqualFold(hs.Name, "Cookie") {
				add(file, 0, "page_auth.extra_headers: name ว่างหรือเป็น Cookie ไม่ได้")
			}
			switch hs.Source {
			case "token":
			case "localStorage", "sessionStorage":
				if hs.Key == "" {
					add(file, 0, "page_auth.extra_headers %q: ต้องมี key", hs.Name)
				}
			default:
				add(file, 0, "page_auth.extra_headers %q: source ต้องเป็น token|localStorage|sessionStorage", hs.Name)
			}
		}
	default:
		add(file, 0, "mode ต้องเป็น host|browser")
	}
	env := h.HostAPI.Envelope
	if env.CodeField == "" || env.DataField == "" || len(env.OkCodes) == 0 {
		add(file, 0, "host_api.envelope ต้องมี code_field, data_field, ok_codes")
	}
	if h.Timezone == "" {
		add(file, 0, "timezone ว่าง")
	} else if _, err := time.LoadLocation(h.Timezone); err != nil {
		add(file, 0, "timezone %q โหลดไม่ได้: %v", h.Timezone, err)
	}
	if h.DayCutoff != "" && !cutoffRe.MatchString(h.DayCutoff) {
		add(file, 0, "day_cutoff ต้องเป็น HH:MM")
	}
	seen := map[string]bool{}
	for _, f := range h.Facts {
		if f.ID == "" || f.Text == "" {
			add(file, 0, "facts ต้องมี id และ text")
		}
		if seen[f.ID] {
			add(file, 0, "fact %q ซ้ำ", f.ID)
		}
		seen[f.ID] = true
	}
}

func (c *Connector) validatePermissions(file string, add addFn) {
	if len(c.Permissions.Rules) == 0 {
		add(file, 0, "ต้องมีอย่างน้อย 1 rule")
	}
	for name, r := range c.Permissions.Rules {
		if !ruleNameRe.MatchString(name) {
			add(file, 0, "ชื่อ rule %q ต้องเป็น a-z 0-9 _", name)
		}
		if len(r.AnyOf) == 0 && len(r.AllOf) == 0 && r.MinLevel <= 0 && !r.LoggedIn {
			add(file, 0, "rule %q ว่าง — ต้องมี any_of/all_of/min_level/logged_in อย่างน้อย 1 อย่าง", name)
		}
	}
}

func (c *Connector) validateStatuses(file string, add addFn) {
	for name, t := range c.Statuses.Tables {
		if len(t.Entries) == 0 {
			add(file, 0, "ตารางสถานะ %q ว่าง", name)
		}
		seen := map[int]bool{}
		for _, e := range t.Entries {
			if seen[e.Code] {
				add(file, 0, "ตาราง %q: code %d ซ้ำ", name, e.Code)
			}
			seen[e.Code] = true
			if e.Label == "" {
				add(file, 0, "ตาราง %q: code %d ไม่มี label", name, e.Code)
			}
		}
	}
	if c.Statuses.UnknownMessage == "" {
		add(file, 0, "unknown_message ว่าง (ต้องบอกผู้ใช้ตามจริงเมื่อเจอสถานะที่ไม่รู้จัก — B-10)")
	}
}

func (c *Connector) validateMenus(file string, add addFn) {
	ids := map[string]bool{}
	for _, m := range c.Menus.Menus {
		if m.ID == "" || m.Name == "" {
			add(file, 0, "menu ต้องมี id และ name")
		}
		if ids[m.ID] {
			add(file, 0, "menu %q ซ้ำ", m.ID)
		}
		ids[m.ID] = true
		if m.Permission != "" {
			if _, ok := c.Permissions.Rules[m.Permission]; !ok {
				add(file, 0, "menu %q อ้าง permission %q ที่ไม่มี", m.ID, m.Permission)
			}
		}
		for _, t := range m.Tabs {
			if t.Permission != "" {
				if _, ok := c.Permissions.Rules[t.Permission]; !ok {
					add(file, 0, "menu %q tab %q อ้าง permission %q ที่ไม่มี", m.ID, t.Name, t.Permission)
				}
			}
		}
	}
	for _, g := range c.Menus.Guides {
		if g.ID == "" || g.Title == "" || len(g.Steps) == 0 {
			add(file, 0, "guide ต้องมี id, title, steps")
		}
		if g.Menu != "" && !ids[g.Menu] {
			add(file, 0, "guide %q อ้าง menu %q ที่ไม่มี", g.ID, g.Menu)
		}
	}
}

var validAgg = map[string]bool{"": true, "count": true, "sum": true, "min": true, "max": true, "avg": true, "first": true, "last": true}

func (c *Connector) validateTool(t *Tool, add addFn) {
	f, ln := t.File, t.Line
	if !toolNameRe.MatchString(t.Name) {
		add(f, ln, "ชื่อ tool %q ต้องเป็น a-z 0-9 _ ยาว 3-61", t.Name)
	}
	if ReservedToolNames[t.Name] {
		add(f, ln, "ชื่อ tool %q ชนกับ tool กลาง", t.Name)
	}
	if strings.TrimSpace(t.Description) == "" {
		add(f, ln, "tool %q ไม่มี description", t.Name)
	}
	if _, ok := c.Permissions.Rules[t.Permission]; !ok {
		add(f, ln, "tool %q อ้าง permission %q ที่ไม่มีใน permissions.yaml", t.Name, t.Permission)
	}
	switch t.Freshness {
	case "live", "summary":
	default:
		add(f, ln, "tool %q: freshness ต้องเป็น live|summary (B-7)", t.Name)
	}
	for name, in := range t.Input {
		if !ruleNameRe.MatchString(name) {
			add(f, ln, "tool %q: ชื่อ input %q ต้องเป็น a-z 0-9 _", t.Name, name)
		}
		switch in.Type {
		case "string":
			// ข้อความอิสระจากโมเดลห้ามวิ่งเข้า query ของ host โดยไม่มีกรอบ
			if in.Pattern == "" && len(in.Enum) == 0 {
				add(f, ln, "tool %q: input %q ชนิด string ต้องมี pattern หรือ enum", t.Name, name)
			}
		case "integer", "number", "boolean", "date":
		default:
			add(f, ln, "tool %q: input %q ชนิด %q ไม่รู้จัก", t.Name, name, in.Type)
		}
		if in.Pattern != "" {
			if _, err := regexp.Compile(in.Pattern); err != nil {
				add(f, ln, "tool %q: input %q pattern ผิด: %v", t.Name, name, err)
			}
		}
		if in.Description == "" {
			add(f, ln, "tool %q: input %q ไม่มี description", t.Name, name)
		}
	}
	if len(t.Calls) == 0 {
		add(f, ln, "tool %q ต้องมี calls อย่างน้อย 1", t.Name)
	}
	callIDs := map[string]bool{}
	for _, cl := range t.Calls {
		if cl.ID == "" || callIDs[cl.ID] {
			add(f, ln, "tool %q: call id %q ว่างหรือซ้ำ", t.Name, cl.ID)
		}
		callIDs[cl.ID] = true
		switch cl.Method {
		case "GET", "POST":
		default:
			add(f, ln, "tool %q call %q: method ต้องเป็น GET|POST (อ่านอย่างเดียว)", t.Name, cl.ID)
		}
		if !strings.HasPrefix(cl.Path, "/") {
			add(f, ln, "tool %q call %q: path ต้องขึ้นต้นด้วย /", t.Name, cl.ID)
		}
		tpls := []string{cl.Path}
		for _, v := range cl.Query {
			tpls = append(tpls, v)
		}
		collectStrings(cl.Body, &tpls)
		switch {
		case !c.Host.IsBrowser():
			if !strings.Contains(cl.Path, "{service}") {
				add(f, ln, "tool %q call %q: path ต้องมี {service} (host ผูก service กับกุญแจดอกเล็ก)", t.Name, cl.ID)
			}
		case cl.Scope == "office":
			// ข้อมูลระดับ office (ไม่ผูกเว็บ) เช่น บิล — ต้องประกาศชัดว่าตั้งใจ
		default:
			// โหมด browser: API เดิมรับ service ได้ทั้ง path/query/body — แต่ต้องผูกเว็บของตั๋วเสมอ
			bound := false
			for _, tpl := range tpls {
				if strings.Contains(tpl, "{service}") || strings.Contains(tpl, "{service_b64}") {
					bound = true
				}
			}
			if !bound {
				add(f, ln, "tool %q call %q: ต้องส่ง {service} ใน path/query/body (หรือประกาศ scope: office)", t.Name, cl.ID)
			}
		}
		for _, tpl := range tpls {
			for _, p := range placeholders(tpl) {
				if err := checkPlaceholder(p, t.Input); err != nil {
					add(f, ln, "tool %q call %q: %v", t.Name, cl.ID, err)
				}
			}
		}
		if cl.CacheTTL < 0 || cl.CacheTTL > 60 {
			add(f, ln, "tool %q call %q: cache_ttl ต้องอยู่ 0–60 วิ (ขยายต้องขออนุมัติ — B-7)", t.Name, cl.ID)
		}
		if t.Freshness == "live" && cl.CacheTTL != 0 {
			add(f, ln, "tool %q call %q: freshness=live ห้ามแคช", t.Name, cl.ID)
		}
		if cl.TimeoutMs < 0 || cl.TimeoutMs > 30000 {
			add(f, ln, "tool %q call %q: timeout_ms ต้องอยู่ 0–30000", t.Name, cl.ID)
		}
		switch cl.Envelope {
		case "", "raw":
		default:
			add(f, ln, "tool %q call %q: envelope ต้องเป็น \"\" หรือ raw", t.Name, cl.ID)
		}
		if cl.Schema == nil {
			add(f, ln, "tool %q call %q: ต้องมี schema (P-11)", t.Name, cl.ID)
		} else if err := cl.Schema.check(""); err != nil {
			add(f, ln, "tool %q call %q: %v", t.Name, cl.ID, err)
		}
	}

	valueNames := map[string]bool{}
	for i := range t.Values {
		v := &t.Values[i]
		if !ruleNameRe.MatchString(v.Name) || valueNames[v.Name] {
			add(f, ln, "tool %q: value %q ชื่อผิดหรือซ้ำ", t.Name, v.Name)
		}
		if v.From == "" && v.Expr == "" {
			add(f, ln, "tool %q value %q: ต้องมี from หรือ expr", t.Name, v.Name)
		}
		if v.From != "" && !callIDs[v.From] {
			add(f, ln, "tool %q value %q: from %q ไม่ใช่ call id", t.Name, v.Name, v.From)
		}
		if !validAgg[v.Agg] {
			add(f, ln, "tool %q value %q: agg %q ไม่รู้จัก", t.Name, v.Name, v.Agg)
		}
		switch v.Order {
		case "", "asc", "desc":
		default:
			add(f, ln, "tool %q value %q: order ต้องเป็น asc|desc", t.Name, v.Name)
		}
		for _, src := range []string{v.Filter, v.Expr} {
			if src == "" {
				continue
			}
			if _, err := ParseExpr(src); err != nil {
				add(f, ln, "tool %q value %q: %v", t.Name, v.Name, err)
			}
		}
		valueNames[v.Name] = true
	}
	if t.NotFound != nil {
		if _, err := ParseExpr(t.NotFound.When); err != nil {
			add(f, ln, "tool %q not_found.when: %v", t.Name, err)
		}
		if t.NotFound.Message == "" {
			add(f, ln, "tool %q not_found.message ว่าง", t.Name)
		}
	}

	card := t.Card
	if strings.TrimSpace(card.Title) == "" {
		add(f, ln, "tool %q: card.title ว่าง", t.Name)
	}
	if card.Link == nil || !strings.HasPrefix(card.Link.Path, "/") || card.Link.Label == "" {
		add(f, ln, "tool %q: การ์ดต้องมี link (label + path ขึ้นต้นด้วย /) ไปหน้าจริงเสมอ", t.Name)
	} else {
		for _, p := range placeholders(card.Link.Path) {
			if err := checkPlaceholder(p, t.Input); err != nil {
				add(f, ln, "tool %q card.link: %v", t.Name, err)
			}
		}
	}
	cardValues := map[string]bool{}
	for _, fs := range card.Fields {
		if !valueNames[fs.Value] {
			add(f, ln, "tool %q card field %q อ้าง value %q ที่ไม่มี", t.Name, fs.Label, fs.Value)
		}
		cardValues[fs.Value] = true
		if err := validFormat(fs.Format, c.Statuses); err != nil {
			add(f, ln, "tool %q card field %q: %v", t.Name, fs.Label, err)
		}
		if fs.When != "" {
			if _, err := ParseExpr(fs.When); err != nil {
				add(f, ln, "tool %q card field %q when: %v", t.Name, fs.Label, err)
			}
		}
	}
	if card.Table != nil {
		if !valueNames[card.Table.Rows] {
			add(f, ln, "tool %q card.table.rows อ้าง value %q ที่ไม่มี", t.Name, card.Table.Rows)
		}
		cardValues[card.Table.Rows] = true
		if len(card.Table.Columns) == 0 {
			add(f, ln, "tool %q card.table ไม่มี columns", t.Name)
		}
		for _, col := range card.Table.Columns {
			if err := validFormat(col.Format, c.Statuses); err != nil {
				add(f, ln, "tool %q column %q: %v", t.Name, col.Label, err)
			}
		}
	}
	if len(card.Fields) == 0 && card.Table == nil {
		add(f, ln, "tool %q: การ์ดต้องมี fields หรือ table", t.Name)
	}

	ctxNames := map[string]bool{}
	for _, cs := range t.ModelContext {
		if !ruleNameRe.MatchString(cs.Name) || ctxNames[cs.Name] {
			add(f, ln, "tool %q: model_context %q ชื่อผิดหรือซ้ำ", t.Name, cs.Name)
		}
		ctxNames[cs.Name] = true
		// AC-14 (static): model_context ห้ามชื่อเดียวกับค่าที่ขึ้นการ์ด — กันส่งค่าบนการ์ดให้โมเดลโดยไม่ตั้งใจ
		if cardValues[cs.Name] {
			add(f, ln, "tool %q: model_context %q ชื่อซ้ำกับค่าที่อยู่บนการ์ด (B-4 · AC-14)", t.Name, cs.Name)
		}
		switch cs.Type {
		case "bool":
			if _, err := ParseExpr(cs.Expr); err != nil {
				add(f, ln, "tool %q model_context %q: %v", t.Name, cs.Name, err)
			}
		case "labels":
			if !valueNames[cs.From] || cs.Pluck == "" || !strings.HasPrefix(cs.Format, "status:") {
				add(f, ln, "tool %q model_context %q: labels ต้องมี from (value) + pluck + format status:<ตาราง>", t.Name, cs.Name)
			} else if err := validFormat(cs.Format, c.Statuses); err != nil {
				add(f, ln, "tool %q model_context %q: %v", t.Name, cs.Name, err)
			}
		case "text":
			if strings.TrimSpace(cs.Text) == "" {
				add(f, ln, "tool %q model_context %q: text ว่าง", t.Name, cs.Name)
			}
			if containsDigit(cs.Text) {
				add(f, ln, "tool %q model_context %q: text ห้ามมีตัวเลข", t.Name, cs.Name)
			}
		default:
			add(f, ln, "tool %q model_context %q: type ต้องเป็น bool|labels|text", t.Name, cs.Name)
		}
	}
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func collectStrings(v any, out *[]string) {
	switch x := v.(type) {
	case string:
		*out = append(*out, x)
	case map[string]any:
		for _, vv := range x {
			collectStrings(vv, out)
		}
	case []any:
		for _, vv := range x {
			collectStrings(vv, out)
		}
	}
}

func (c *Connector) validateIdentity(file string, id *IdentitySpec, add addFn) {
	if id == nil {
		add(file, 0, "mode: browser ต้องมี page_auth.identity (อ่านตัวตนแอดมินจากหน้า)")
		return
	}
	switch id.Source {
	case "jwt":
	case "request":
		if !strings.HasPrefix(id.Path, "/") {
			add(file, 0, "page_auth.identity.path ต้องขึ้นต้นด้วย /")
		}
		if id.Method != "" && id.Method != "GET" && id.Method != "POST" {
			add(file, 0, "page_auth.identity.method ต้องเป็น GET|POST")
		}
	default:
		add(file, 0, "page_auth.identity.source ต้องเป็น jwt|request")
	}
	if id.ID == "" || id.Username == "" {
		add(file, 0, "page_auth.identity ต้องมี id และ username (path ใน object ผู้ใช้)")
	}
	if id.Permissions != nil && id.Permissions.Path == "" {
		add(file, 0, "page_auth.identity.permissions.path ว่าง")
	}
	if r := id.PermissionsRequest; r != "" {
		if !strings.HasPrefix(r, "/") || strings.HasPrefix(r, "//") || strings.Contains(r, "://") {
			add(file, 0, "page_auth.identity.permissions_request ต้องขึ้นต้นด้วย / และห้ามมี host")
		}
		if id.Permissions == nil {
			add(file, 0, "page_auth.identity.permissions_request ต้องมี permissions (path/pluck) คู่กัน")
		}
	}
}
