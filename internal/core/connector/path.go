package connector

import (
	"strconv"
	"strings"
)

// GetPath อ่านค่าจาก JSON ที่ decode แล้ว (map[string]any / []any) ตาม path
//
//	"a.b"      → field
//	"a[0]"     → index (ติดลบนับจากท้าย)
//	"a[*].b"   → ทุก item (ได้ []any)
//	""         → ทั้งก้อน
//
// ไม่เจอ = nil (ไม่ใช่ 0) — ผู้เรียกต้องแยก "ไม่มีค่า" ออกจาก "ศูนย์"
func GetPath(v any, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return v
	}
	segs := splitPath(path)
	return walk(v, segs)
}

type pathSeg struct {
	key   string
	index *int
	all   bool
}

func splitPath(path string) []pathSeg {
	var out []pathSeg
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		name := part
		rest := ""
		if i := strings.IndexByte(part, '['); i >= 0 {
			name, rest = part[:i], part[i:]
		}
		if name != "" {
			out = append(out, pathSeg{key: name})
		}
		for strings.HasPrefix(rest, "[") {
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				break
			}
			inner := rest[1:end]
			rest = rest[end+1:]
			if inner == "*" {
				out = append(out, pathSeg{all: true})
				continue
			}
			if n, err := strconv.Atoi(inner); err == nil {
				nn := n
				out = append(out, pathSeg{index: &nn})
			} else {
				out = append(out, pathSeg{key: strings.Trim(inner, `'"`)})
			}
		}
	}
	return out
}

func walk(v any, segs []pathSeg) any {
	for i, s := range segs {
		switch {
		case s.all:
			list, ok := v.([]any)
			if !ok {
				return nil
			}
			out := make([]any, 0, len(list))
			for _, it := range list {
				out = append(out, walk(it, segs[i+1:]))
			}
			return out
		case s.index != nil:
			list, ok := v.([]any)
			if !ok {
				return nil
			}
			k := *s.index
			if k < 0 {
				k = len(list) + k
			}
			if k < 0 || k >= len(list) {
				return nil
			}
			v = list[k]
		default:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v, ok = m[s.key]
			if !ok {
				return nil
			}
		}
	}
	return v
}
