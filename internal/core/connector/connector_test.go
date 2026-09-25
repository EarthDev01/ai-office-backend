package connector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustLoad(t *testing.T) *Connector {
	t.Helper()
	c, err := Load(filepath.Join("testdata", "sample-kind"))
	if err != nil {
		t.Fatalf("load sample connector: %v", err)
	}
	return c
}

func jsonAny(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func bkk() *time.Location {
	l, _ := time.LoadLocation("Asia/Bangkok")
	return l
}

func TestLoad_SampleConnector(t *testing.T) {
	c := mustLoad(t)
	if c.Kind != "sample-kind" || len(c.Tools) != 2 {
		t.Fatalf("kind/tools ผิด: %s %d", c.Kind, len(c.Tools))
	}
	tool, ok := c.Tool("withdraw_pending_by_status")
	if !ok || tool.Line == 0 || tool.Questions[0] != "BQ-03" {
		t.Fatalf("tool meta ผิด: %+v", tool)
	}
}

// V5: connector ผิดรูปต้องบอกไฟล์/บรรทัด ไม่ใช่เงียบ
func TestLoad_BadConnectorReportsFileAndLine(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("testdata", "sample-kind")
	dst := filepath.Join(dir, "sample-kind")
	copyDir(t, src, dst)

	// field พิมพ์ผิด
	q := filepath.Join(dst, "questions", "withdraw.yaml")
	b, _ := os.ReadFile(q)
	os.WriteFile(q, []byte(strings.Replace(string(b), "freshness: summary", "freshnes: summary", 1)), 0o644)
	_, err := Load(dst)
	if err == nil || !strings.Contains(err.Error(), "withdraw.yaml") || !strings.Contains(err.Error(), "line") {
		t.Fatalf("ต้องบอกไฟล์+บรรทัด ได้ %v", err)
	}
}

func TestLoad_RejectsUnsafeTools(t *testing.T) {
	cases := map[string]struct{ from, to, want string }{
		"string input ไม่มี pattern": {`pattern: "^[A-Za-z0-9_.@-]{2,40}$"`, `max_len: 40`, "ต้องมี pattern"},
		"live แต่แคช":                {"freshness: live", "freshness: live\n    x_dummy: 1", ""},
		"path ไม่ผูก service":        {"/api/ai/read/member/{service}", "/api/ai/read/member/all", "{service}"},
		"method เขียน":               {"method: GET\n        path: /api/ai/read/member", "method: DELETE\n        path: /api/ai/read/member", "GET|POST"},
		"model_context ซ้ำชื่อการ์ด": {"name: has_pending, type: bool", "name: total_count, type: bool", "AC-14"},
		"ไม่มี link":                 {`link: {label: เปิดข้อมูลสมาชิก, path: "/member/{input.username}"}`, "", "link"},
		"text มีตัวเลข":              {"text: แสดงข้อมูลสมาชิกในการ์ดแล้ว", "text: ยอด 100 บาท", "ห้ามมีตัวเลข"},
		"cache เกิน 60":              {"cache_ttl: 60", "cache_ttl: 300", "0–60"},
	}
	for name, tc := range cases {
		if tc.want == "" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "sample-kind")
			copyDir(t, filepath.Join("testdata", "sample-kind"), dst)
			q := filepath.Join(dst, "questions", "withdraw.yaml")
			b, _ := os.ReadFile(q)
			s := string(b)
			if !strings.Contains(s, tc.from) {
				t.Fatalf("fixture ไม่มี %q", tc.from)
			}
			os.WriteFile(q, []byte(strings.Replace(s, tc.from, tc.to, 1)), 0o644)
			_, err := Load(dst)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("อยากได้ error มี %q ได้ %v", tc.want, err)
			}
		})
	}
}

func TestEvaluate_TableCardAndContext(t *testing.T) {
	c := mustLoad(t)
	tool, _ := c.Tool("withdraw_pending_by_status")
	data := jsonAny(t, `{"rows":[{"status":2,"count":7,"amount":142000},{"status":10,"count":2,"amount":18500},{"status":99,"count":1,"amount":9000}]}`)
	rc := RenderContext{ServiceID: "K11S", Now: time.Now(), Loc: bkk()}
	out := c.Evaluate(tool, map[string]*CallResult{"list": {ID: "list", Data: data}}, rc, time.Now())

	if out.Status != StatusOK {
		t.Fatalf("status %s", out.Status)
	}
	if out.Card.Table == nil || len(out.Card.Table.Rows) != 3 {
		t.Fatalf("ตารางผิด: %+v", out.Card.Table)
	}
	first := out.Card.Table.Rows[0]
	if first[0].Display != "รอโอน" || first[1].Display != "7 รายการ" || first[2].Display != "142,000.00" {
		t.Fatalf("แถวแรกผิด (ต้องเรียงตามจำนวนมากไปน้อย): %+v", first)
	}
	if out.Card.Table.Rows[2][0].Display != "สถานะ 99 (ยังไม่มีคำอธิบาย)" {
		t.Fatalf("สถานะที่ไม่รู้จักต้องบอกตามจริง: %+v", out.Card.Table.Rows[2][0])
	}
	if out.Card.Fields[0].Display != "10 รายการ" || out.Card.Fields[1].Display != "169,500.00" {
		t.Fatalf("ผลรวมผิด: %+v", out.Card.Fields)
	}
	if out.Card.Link == nil || out.Card.Link.Path != "/withdraw?svc=SzExUw==" {
		t.Fatalf("link ผิด: %+v", out.Card.Link)
	}
	// โมเดลต้องไม่เห็นตัวเลขใด ๆ
	b, _ := json.Marshal(out.ModelContext)
	for _, bad := range []string{"142000", "169500", "7", "10"} {
		if strings.Contains(string(b), bad) {
			t.Fatalf("model_context มีตัวเลข %q: %s", bad, b)
		}
	}
	if out.ModelContext["has_pending"] != true {
		t.Fatalf("has_pending ผิด: %v", out.ModelContext)
	}
	if !containsStr(out.Sensitive, "142,000.00") {
		t.Fatalf("ค่าบนการ์ดต้องอยู่ใน sensitive list: %v", out.Sensitive)
	}
}

// B-11: ไม่พบ ≠ error ≠ 0
func TestEvaluate_EmptyIsNotFoundNotZero(t *testing.T) {
	c := mustLoad(t)
	tool, _ := c.Tool("withdraw_pending_by_status")
	out := c.Evaluate(tool, map[string]*CallResult{"list": {ID: "list", Data: jsonAny(t, `{"rows":[]}`)}},
		RenderContext{ServiceID: "K11S", Now: time.Now(), Loc: bkk()}, time.Now())
	if out.Status != StatusNotFound || out.Card.Note != "ไม่มีรายการถอนค้าง" || len(out.Card.Fields) != 0 {
		t.Fatalf("ต้องเป็น not_found ไม่มีตัวเลข: %+v", out)
	}
}

// AC-30: call ล้ม = ไม่มีตัวเลขบางส่วนโผล่
func TestEvaluate_ErrorHasNoNumbers(t *testing.T) {
	c := mustLoad(t)
	tool, _ := c.Tool("withdraw_pending_by_status")
	out := c.Evaluate(tool, map[string]*CallResult{"list": {ID: "list", Err: os.ErrDeadlineExceeded, ErrCode: "timeout"}},
		RenderContext{ServiceID: "K11S", Now: time.Now(), Loc: bkk()}, time.Now())
	if out.Status != StatusError || out.ErrorCode != "timeout" || len(out.Card.Fields) != 0 || out.Card.Table != nil {
		t.Fatalf("ต้องเป็น error ที่ไม่มีตัวเลข: %+v", out)
	}
	if out.Card.Link == nil {
		t.Fatal("การ์ด error ต้องยังมีลิงก์หน้าจริง")
	}
}

func TestSchema_Validate(t *testing.T) {
	c := mustLoad(t)
	tool, _ := c.Tool("withdraw_pending_by_status")
	s := tool.Calls[0].Schema
	if err := s.Validate(jsonAny(t, `{"rows":[{"status":2,"count":1,"amount":5}]}`)); err != nil {
		t.Fatalf("ควรผ่าน: %v", err)
	}
	for name, bad := range map[string]string{
		"ไม่มี rows":       `{"items":[]}`,
		"count เป็น str":   `{"rows":[{"status":2,"count":"1","amount":5}]}`,
		"status ทศนิยม":    `{"rows":[{"status":2.5,"count":1,"amount":5}]}`,
		"rows ไม่ใช่ list": `{"rows":{}}`,
	} {
		if err := s.Validate(jsonAny(t, bad)); err == nil {
			t.Errorf("%s: ต้องไม่ผ่าน", name)
		}
	}
}

func TestTemplate_BangkokDayBoundary(t *testing.T) {
	// 00:30 เวลาไทย = 17:30 UTC ของวันก่อน → "วันนี้" ต้องเป็นวันใหม่ของไทย (AC-15)
	now := time.Date(2026, 9, 23, 17, 30, 0, 0, time.UTC)
	rc := RenderContext{ServiceID: "K11S", Now: now, Loc: bkk(), Input: map[string]any{"d": "2026-09-01"}}
	cases := map[string]string{
		"{date:today}":                    "2026-09-24",
		"{date:yesterday}":                "2026-09-23",
		"{datetime:today}":                "2026-09-24 00:00:00",
		"{datetime_end:today}":            "2026-09-24 23:59:59",
		"{date:today-7}":                  "2026-09-17",
		"{date:month_start}":              "2026-09-01",
		"{date:prev_month_end}":           "2026-08-31",
		"{date:input.d|date:today}":       "2026-09-01",
		"{date:input.missing|date:today}": "2026-09-24",
		"{service_b64}":                   "SzExUw==",
	}
	for tpl, want := range cases {
		got, err := rc.Render(tpl)
		if err != nil || got != want {
			t.Errorf("%s = %q (%v) อยากได้ %q", tpl, got, err, want)
		}
	}
}

func TestExpr(t *testing.T) {
	env := map[string]any{"a": 3.0, "b": 4.0, "rows": []any{map[string]any{"n": 2.0}, map[string]any{"n": 5.0}}, "s": "x", "nothing": nil}
	cases := map[string]any{
		"a + b * 2":             11.0,
		"(a + b) * 2":           14.0,
		"a < b && !empty(rows)": true,
		"sum(rows, 'n')":        7.0,
		"count(rows) == 2":      true,
		"rows[1].n - rows[0].n": 3.0,
		"coalesce(nothing, a)":  3.0,
		"in(a, 1, 2, 3)":        true,
		"nothing + 1":           nil, // ค่าหาย ≠ 0
		"a / 0":                 nil,
		"s == 'x' || false":     true,
		"max(a, b)":             4.0,
		"round(10 / 3, 2)":      3.33,
	}
	for src, want := range cases {
		e, err := ParseExpr(src)
		if err != nil {
			t.Fatalf("%s: parse %v", src, err)
		}
		got, err := e.Eval(env)
		if err != nil || got != want {
			t.Errorf("%s = %v (%v) อยากได้ %v", src, got, err, want)
		}
	}
	for _, bad := range []string{"a +", "foo(1)", "a ^ b", "'unterminated"} {
		if _, err := ParseExpr(bad); err == nil {
			t.Errorf("%q ต้อง parse ไม่ผ่าน", bad)
		}
	}
}

func TestFormat(t *testing.T) {
	f := Formatter{Loc: bkk()}
	cases := []struct {
		v      any
		format string
		want   string
	}{
		{1284500.0, "money", "1,284,500.00"},
		{-5.5, "money", "-5.50"},
		{412.0, "int", "412"},
		{1234567.0, "int", "1,234,567"},
		{18.04, "percent", "18.0%"},
		{0.129, "ratio_percent", "12.9%"},
		{"2026-09-24T07:32:00Z", "datetime", "24/09/2026 14:32"},
		{1758699120.0, "datetime", "24/09/2025 14:32"},
		{nil, "money", "—"},
		{"abc", "", "abc"},
	}
	for _, c := range cases {
		if got := f.Format(c.v, c.format); got != c.want {
			t.Errorf("Format(%v,%s) = %q อยากได้ %q", c.v, c.format, got, c.want)
		}
	}
}

func TestValidateInput(t *testing.T) {
	c := mustLoad(t)
	tool, _ := c.Tool("member_lookup")
	if _, err := tool.ValidateInput(map[string]any{}); err == nil {
		t.Fatal("ไม่ส่ง required ต้องไม่ผ่าน")
	}
	if _, err := tool.ValidateInput(map[string]any{"username": "a b; drop"}); err == nil {
		t.Fatal("pattern ไม่ตรงต้องไม่ผ่าน")
	}
	got, err := tool.ValidateInput(map[string]any{"username": " somchai99 ", "service": "PG99"})
	if err != nil || got["username"] != "somchai99" {
		t.Fatalf("ต้องผ่านและตัดช่องว่าง: %v %v", got, err)
	}
	if _, ok := got["service"]; ok {
		t.Fatal("field ที่ไม่ได้ประกาศ (เช่น service) ต้องถูกทิ้ง")
	}
}

func TestPermissionRule(t *testing.T) {
	r := PermissionRule{AnyOf: []string{"A", "B"}}
	if !r.Allows([]string{"B"}, 0) || r.Allows([]string{"C"}, 99) {
		t.Fatal("any_of ผิด")
	}
	if (PermissionRule{MinLevel: 7}).Allows(nil, 6) || !(PermissionRule{MinLevel: 7}).Allows(nil, 7) {
		t.Fatal("min_level ผิด")
	}
	if !(PermissionRule{LoggedIn: true}).Allows(nil, 0) {
		t.Fatal("logged_in ผิด")
	}
	if (PermissionRule{}).Allows([]string{"A"}, 99) {
		t.Fatal("rule ว่างต้อง fail closed")
	}
}

func TestGetPath(t *testing.T) {
	v := jsonAny(t, `{"a":{"b":[{"c":1},{"c":2}]},"x":null}`)
	if GetPath(v, "a.b[1].c") != 2.0 || GetPath(v, "a.b[-1].c") != 2.0 {
		t.Fatal("index ผิด")
	}
	all, _ := GetPath(v, "a.b[*].c").([]any)
	if len(all) != 2 || all[0] != 1.0 {
		t.Fatalf("wildcard ผิด: %v", all)
	}
	if GetPath(v, "a.zzz") != nil || GetPath(v, "x.y") != nil {
		t.Fatal("ไม่เจอต้องเป็น nil")
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// C10: contract test ของ connector ตัวอย่าง (โค้ดเดียวกับที่รันจริง)
func TestContracts_SampleKind(t *testing.T) {
	c, err := Load("testdata/sample-kind")
	if err != nil {
		t.Fatal(err)
	}
	files, err := c.LoadContracts()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("อยากได้ 2 ไฟล์ ได้ %d", len(files))
	}
	for _, f := range files {
		for _, e := range c.RunContract(f) {
			t.Error(e)
		}
	}
}

// runner ต้องจับได้จริงเมื่อ connector กับ response ไม่ตรงกัน
func TestContracts_DetectsMismatch(t *testing.T) {
	c, err := Load("testdata/sample-kind")
	if err != nil {
		t.Fatal(err)
	}
	cf := ContractFile{Tool: "withdraw_pending_by_status", File: "x", Cases: []ContractCase{{
		Name:      "ผิด",
		Responses: map[string]ContractResponse{"list": {Body: map[string]any{"code": 0, "data": map[string]any{"rows": []any{map[string]any{"status": 2, "count": 7, "amount": 142000}}}}}},
		Expect:    ContractExpect{Status: StatusOK, Fields: map[string]string{"รวมทั้งหมด": "8 รายการ"}},
	}}}
	if errs := c.RunContract(cf); len(errs) == 0 {
		t.Fatal("ต้องจับ field ที่ไม่ตรงได้")
	}
}
