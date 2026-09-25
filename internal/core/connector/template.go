package connector

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Template ของ path / query / body / link · placeholder อยู่ใน {…}
//
//	{service}                 service ของตั๋ว (ไม่ใช่ค่าจากผู้ใช้)
//	{service_b64}             base64 ของ service (ลิงก์หน้า host ที่ใช้ ?service=base64)
//	{input.username}          ค่าที่โมเดลส่งมา (ผ่านการตรวจ input แล้ว)
//	{date:today}              YYYY-MM-DD ตาม timezone/day_cutoff ของ connector
//	{datetime:today}          YYYY-MM-DD 00:00:00  · {datetime_end:today} → 23:59:59
//	{month:today}             YYYY-MM · {unix:now} วินาที
//	spec: today yesterday tomorrow today+N today-N month_start month_end prev_month_start prev_month_end now input.<key>
//	a|b                       ใช้ a ถ้ามีค่า ไม่งั้น b (เช่น {date:input.date_from|date:today})

var placeholderRe = regexp.MustCompile(`\{([^{}]+)\}`)

// RenderContext = ค่าที่ template อ้างได้ · Now/Loc มาจากเวลาจริง + timezone ของ connector (B-6)
type RenderContext struct {
	ServiceID string
	Input     map[string]any
	Now       time.Time
	Loc       *time.Location
	Cutoff    time.Duration // เวลาตัดวันหลังเที่ยงคืน (ปกติ 0)
}

// Render แทนที่ทุก placeholder เป็น string
func (rc RenderContext) Render(tpl string) (string, error) {
	var firstErr error
	out := placeholderRe.ReplaceAllStringFunc(tpl, func(m string) string {
		v, err := rc.resolve(m[1 : len(m)-1])
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	})
	return out, firstErr
}

// RenderValue — ถ้า string เป็น placeholder ตัวเดียวล้วน คืนค่าตามชนิดจริง (เลขยังเป็นเลขใน body JSON)
func (rc RenderContext) RenderValue(v any) (any, error) {
	switch x := v.(type) {
	case string:
		t := strings.TrimSpace(x)
		if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") && strings.Count(t, "{") == 1 {
			return rc.resolve(t[1 : len(t)-1])
		}
		return rc.Render(x)
	case map[string]any:
		out := map[string]any{}
		for k, vv := range x {
			r, err := rc.RenderValue(vv)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			r, err := rc.RenderValue(vv)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}

func (rc RenderContext) resolve(expr string) (any, error) {
	for _, alt := range strings.Split(expr, "|") {
		v, err := rc.resolveOne(strings.TrimSpace(alt))
		if err != nil {
			return nil, err
		}
		if v != nil && fmt.Sprint(v) != "" {
			return v, nil
		}
	}
	return nil, nil
}

func (rc RenderContext) resolveOne(p string) (any, error) {
	switch {
	case p == "service":
		return rc.ServiceID, nil
	case p == "service_b64":
		return base64.StdEncoding.EncodeToString([]byte(rc.ServiceID)), nil
	case strings.HasPrefix(p, "input."):
		v, ok := rc.Input[strings.TrimPrefix(p, "input.")]
		if !ok {
			return nil, nil
		}
		return v, nil
	}
	fn, spec, ok := strings.Cut(p, ":")
	if !ok {
		return nil, fmt.Errorf("placeholder {%s} ไม่รู้จัก", p)
	}
	t, has, err := rc.day(spec)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	switch fn {
	case "date":
		return t.Format("2006-01-02"), nil
	case "datetime":
		if spec == "now" {
			return t.Format("2006-01-02 15:04:05"), nil
		}
		return t.Format("2006-01-02") + " 00:00:00", nil
	case "datetime_end":
		return t.Format("2006-01-02") + " 23:59:59", nil
	case "month":
		return t.Format("2006-01"), nil
	case "unix":
		return t.Unix(), nil
	}
	return nil, fmt.Errorf("placeholder {%s}: ฟังก์ชัน %q ไม่รู้จัก", p, fn)
}

// Today = วันทำการปัจจุบันตาม timezone + day_cutoff (B-6: "วันนี้" = 00:00–23:59 Asia/Bangkok)
func (rc RenderContext) Today() time.Time {
	loc := rc.Loc
	if loc == nil {
		loc = time.UTC
	}
	n := rc.Now.In(loc).Add(-rc.Cutoff)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

var offsetRe = regexp.MustCompile(`^today([+-])(\d{1,3})$`)

func (rc RenderContext) day(spec string) (time.Time, bool, error) {
	today := rc.Today()
	switch spec {
	case "today":
		return today, true, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), true, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), true, nil
	case "now":
		loc := rc.Loc
		if loc == nil {
			loc = time.UTC
		}
		return rc.Now.In(loc), true, nil
	case "month_start":
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location()), true, nil
	case "month_end":
		return time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()), true, nil
	case "prev_month_start":
		return time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, today.Location()), true, nil
	case "prev_month_end":
		return time.Date(today.Year(), today.Month(), 0, 0, 0, 0, 0, today.Location()), true, nil
	}
	if m := offsetRe.FindStringSubmatch(spec); m != nil {
		n, _ := strconv.Atoi(m[2])
		if m[1] == "-" {
			n = -n
		}
		return today.AddDate(0, 0, n), true, nil
	}
	if strings.HasPrefix(spec, "input.") {
		v, ok := rc.Input[strings.TrimPrefix(spec, "input.")]
		if !ok || v == nil || fmt.Sprint(v) == "" {
			return time.Time{}, false, nil
		}
		t, err := time.ParseInLocation("2006-01-02", fmt.Sprint(v), today.Location())
		if err != nil {
			return time.Time{}, false, fmt.Errorf("วันที่ %q ต้องเป็น YYYY-MM-DD", v)
		}
		return t, true, nil
	}
	return time.Time{}, false, fmt.Errorf("วันที่ %q ไม่รู้จัก", spec)
}

// placeholders คืนชื่อทุก placeholder ใน template (ใช้ตรวจตอนโหลด)
func placeholders(tpl string) []string {
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(tpl, -1) {
		for _, alt := range strings.Split(m[1], "|") {
			out = append(out, strings.TrimSpace(alt))
		}
	}
	return out
}

// checkPlaceholder ตรวจว่า placeholder ใช้ได้ และถ้าอ้าง input ต้องประกาศไว้
func checkPlaceholder(p string, inputs map[string]InputSpec) error {
	check := func(ref string) error {
		key := strings.TrimPrefix(ref, "input.")
		if _, ok := inputs[key]; !ok {
			return fmt.Errorf("placeholder {%s} อ้าง input %q ที่ไม่ได้ประกาศ", p, key)
		}
		return nil
	}
	switch {
	case p == "service" || p == "service_b64":
		return nil
	case strings.HasPrefix(p, "input."):
		return check(p)
	}
	fn, spec, ok := strings.Cut(p, ":")
	if !ok {
		return fmt.Errorf("placeholder {%s} ไม่รู้จัก", p)
	}
	switch fn {
	case "date", "datetime", "datetime_end", "month", "unix":
	default:
		return fmt.Errorf("placeholder {%s}: ฟังก์ชัน %q ไม่รู้จัก", p, fn)
	}
	if strings.HasPrefix(spec, "input.") {
		return check(spec)
	}
	rc := RenderContext{Now: time.Now(), Loc: time.UTC}
	_, _, err := rc.day(spec)
	return err
}
