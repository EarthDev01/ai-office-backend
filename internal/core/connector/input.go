package connector

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// InputSchema แปลง input ของ tool เป็น JSON Schema ที่ส่งให้โมเดล
func (t *Tool) InputSchema() (properties map[string]any, required []string) {
	properties = map[string]any{}
	names := make([]string, 0, len(t.Input))
	for n := range t.Input {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		in := t.Input[n]
		p := map[string]any{"description": in.Description}
		switch in.Type {
		case "date":
			p["type"] = "string"
			p["pattern"] = `^\d{4}-\d{2}-\d{2}$`
			p["description"] = in.Description + " (รูปแบบ YYYY-MM-DD)"
		case "integer", "number", "boolean", "string":
			p["type"] = in.Type
		}
		if in.Pattern != "" {
			p["pattern"] = in.Pattern
		}
		if len(in.Enum) > 0 {
			p["enum"] = in.Enum
		}
		if in.MaxLen > 0 {
			p["maxLength"] = in.MaxLen
		}
		if in.Min != nil {
			p["minimum"] = *in.Min
		}
		if in.Max != nil {
			p["maximum"] = *in.Max
		}
		properties[n] = p
		if in.Required {
			required = append(required, n)
		}
	}
	return properties, required
}

// ValidateInput ตรวจค่าที่โมเดลส่งมา — ผู้ใช้ (ผ่านโมเดล) ส่งอะไรมาก็ได้ จึงถือเป็นข้อมูลไม่น่าเชื่อ (B-16)
//
// คืนค่าที่ทำความสะอาดแล้ว · field ที่ไม่ได้ประกาศถูกทิ้ง (กันยัด service/office ผ่านโมเดล)
func (t *Tool) ValidateInput(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for name, in := range t.Input {
		v, ok := raw[name]
		if !ok || v == nil || (fmt.Sprint(v) == "" && in.Type != "boolean") {
			if in.Required {
				return nil, fmt.Errorf("ต้องระบุ %s", name)
			}
			continue
		}
		switch in.Type {
		case "string":
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s ต้องเป็นข้อความ", name)
			}
			s = strings.TrimSpace(s)
			if in.MaxLen > 0 && len([]rune(s)) > in.MaxLen {
				return nil, fmt.Errorf("%s ยาวเกิน %d ตัว", name, in.MaxLen)
			}
			if in.Pattern != "" {
				re, err := regexp.Compile(in.Pattern)
				if err != nil || !re.MatchString(s) {
					return nil, fmt.Errorf("%s รูปแบบไม่ถูกต้อง", name)
				}
			}
			if len(in.Enum) > 0 && !inEnum(s, in.Enum) {
				return nil, fmt.Errorf("%s ต้องเป็นค่าใดค่าหนึ่งที่กำหนด", name)
			}
			out[name] = s
		case "integer", "number":
			f, ok := ToFloat(v)
			if !ok {
				return nil, fmt.Errorf("%s ต้องเป็นตัวเลข", name)
			}
			if in.Type == "integer" && f != float64(int64(f)) {
				return nil, fmt.Errorf("%s ต้องเป็นจำนวนเต็ม", name)
			}
			if in.Min != nil && f < *in.Min {
				return nil, fmt.Errorf("%s ต่ำกว่าที่กำหนด", name)
			}
			if in.Max != nil && f > *in.Max {
				return nil, fmt.Errorf("%s เกินที่กำหนด", name)
			}
			if len(in.Enum) > 0 && !inEnum(f, in.Enum) {
				return nil, fmt.Errorf("%s ต้องเป็นค่าใดค่าหนึ่งที่กำหนด", name)
			}
			if in.Type == "integer" {
				out[name] = int64(f)
			} else {
				out[name] = f
			}
		case "boolean":
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("%s ต้องเป็น true/false", name)
			}
			out[name] = b
		case "date":
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s ต้องเป็นวันที่ YYYY-MM-DD", name)
			}
			if _, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err != nil {
				return nil, fmt.Errorf("%s ต้องเป็นวันที่ YYYY-MM-DD", name)
			}
			out[name] = strings.TrimSpace(s)
		}
	}
	return out, nil
}

func inEnum(v any, enum []any) bool {
	for _, e := range enum {
		if looseEqual(v, e) {
			return true
		}
	}
	return false
}
