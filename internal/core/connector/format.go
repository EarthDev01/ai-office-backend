package connector

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Formatter แปลงค่าจาก tool เป็นข้อความบนการ์ด (ค่าเดิมเก็บคู่ไว้ใน CardField.Value เสมอ)
type Formatter struct {
	Statuses Statuses
	Loc      *time.Location
}

var knownFormats = map[string]bool{
	"": true, "text": true, "int": true, "number": true, "money": true,
	"percent": true, "ratio_percent": true, "datetime": true, "date": true, "bool": true,
}

func validFormat(f string, st Statuses) error {
	if knownFormats[f] {
		return nil
	}
	if strings.HasPrefix(f, "status:") {
		t := strings.TrimPrefix(f, "status:")
		if _, ok := st.Tables[t]; !ok {
			return fmt.Errorf("format %q อ้างตารางสถานะ %q ที่ไม่มีใน statuses.yaml", f, t)
		}
		return nil
	}
	return fmt.Errorf("format %q ไม่รู้จัก", f)
}

// Format คืนข้อความแสดงผล · ค่าหาย = "—" (ไม่แสดง 0 ปลอม — B-11)
func (f Formatter) Format(v any, format string) string {
	if v == nil {
		return "—"
	}
	switch {
	case format == "int":
		if n, ok := ToFloat(v); ok {
			return groupThousands(strconv.FormatInt(int64(math.Round(n)), 10))
		}
	case format == "number":
		if n, ok := ToFloat(v); ok {
			return formatDecimal(n, 2, true)
		}
	case format == "money":
		if n, ok := ToFloat(v); ok {
			return formatDecimal(n, 2, false)
		}
	case format == "percent":
		if n, ok := ToFloat(v); ok {
			return strconv.FormatFloat(n, 'f', 1, 64) + "%"
		}
	case format == "ratio_percent":
		if n, ok := ToFloat(v); ok {
			return strconv.FormatFloat(n*100, 'f', 1, 64) + "%"
		}
	case format == "datetime":
		if t, ok := f.parseTime(v); ok {
			return t.In(f.loc()).Format("02/01/2006 15:04")
		}
	case format == "date":
		if t, ok := f.parseTime(v); ok {
			return t.In(f.loc()).Format("02/01/2006")
		}
	case format == "bool":
		if Truthy(v) {
			return "ใช่"
		}
		return "ไม่ใช่"
	case strings.HasPrefix(format, "status:"):
		return f.StatusLabel(strings.TrimPrefix(format, "status:"), v)
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == math.Trunc(x) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "ใช่"
		}
		return "ไม่ใช่"
	}
	return fmt.Sprint(v)
}

// StatusLabel แปลรหัสสถานะด้วยตารางของ connector · ไม่รู้จัก = บอกตามจริง ห้ามเดา (B-10)
func (f Formatter) StatusLabel(table string, v any) string {
	t, ok := f.Statuses.Tables[table]
	if !ok {
		return fmt.Sprint(v)
	}
	if n, ok := ToFloat(v); ok {
		if e, ok := t.Find(int(n)); ok {
			return e.Label
		}
	}
	return fmt.Sprintf("สถานะ %v (ยังไม่มีคำอธิบาย)", v)
}

func (f Formatter) loc() *time.Location {
	if f.Loc != nil {
		return f.Loc
	}
	return time.UTC
}

var timeLayouts = []string{
	time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
}

func (f Formatter) parseTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		for _, l := range timeLayouts {
			// เวลาที่ไม่มี timezone ถือว่าเป็นเวลาท้องถิ่นของ connector
			if t, err := time.ParseInLocation(l, s, f.loc()); err == nil {
				return t, true
			}
		}
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return unixToTime(n), true
		}
	case float64:
		return unixToTime(x), true
	case int64:
		return unixToTime(float64(x)), true
	}
	return time.Time{}, false
}

// unix อาจเป็นวินาทีหรือมิลลิวินาที
func unixToTime(n float64) time.Time {
	if n > 1e12 {
		return time.UnixMilli(int64(n))
	}
	return time.Unix(int64(n), 0)
}

func formatDecimal(n float64, decimals int, trimZeros bool) string {
	neg := n < 0
	s := strconv.FormatFloat(math.Abs(n), 'f', decimals, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	out := groupThousands(intPart)
	if frac != "" {
		if trimZeros {
			frac = strings.TrimRight(frac, "0")
		}
		if frac != "" {
			out += "." + frac
		}
	}
	if neg {
		out = "-" + out
	}
	return out
}

func groupThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// KnownStatus = รหัสนี้อยู่ในตารางสถานะของ connector
func (f Formatter) KnownStatus(table string, v any) bool {
	t, ok := f.Statuses.Tables[table]
	if !ok {
		return false
	}
	n, ok := ToFloat(v)
	if !ok {
		return false
	}
	_, found := t.Find(int(n))
	return found
}
