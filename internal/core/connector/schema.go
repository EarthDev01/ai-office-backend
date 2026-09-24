package connector

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Schema = JSON Schema ชุดย่อยที่พอใช้ตรวจรูป response ของ host (P-11 · AC-30)
//
//	type        string | []string  (object array string number integer boolean null)
//	properties  map
//	required    []string
//	items       Schema
//	nullable    true = ยอม null เพิ่ม
//
// field ที่ไม่ได้ประกาศไว้ใน properties ยอมให้มีได้ (host เพิ่ม field ใหม่ต้องไม่ทำให้พัง)
// แต่ field ที่ประกาศว่า required หาย/ผิดชนิด = ผิด schema → tool ต้องตอบว่า "ดึงไม่ได้"
type Schema struct {
	Type       any                `yaml:"type"`
	Properties map[string]*Schema `yaml:"properties"`
	Required   []string           `yaml:"required"`
	Items      *Schema            `yaml:"items"`
	Nullable   bool               `yaml:"nullable"`
}

func (s *Schema) types() []string {
	switch t := s.Type.(type) {
	case string:
		return []string{t}
	case []any:
		out := []string{}
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	}
	return nil
}

var knownTypes = map[string]bool{"object": true, "array": true, "string": true, "number": true, "integer": true, "boolean": true, "null": true}

// check ตรวจรูปของ schema เอง (ตอนโหลด connector)
func (s *Schema) check(path string) error {
	if s == nil {
		return nil
	}
	for _, t := range s.types() {
		if !knownTypes[t] {
			return fmt.Errorf("schema %s: type %q ไม่รู้จัก", orRoot(path), t)
		}
	}
	for k, p := range s.Properties {
		if err := p.check(path + "." + k); err != nil {
			return err
		}
	}
	for _, r := range s.Required {
		if _, ok := s.Properties[r]; !ok {
			return fmt.Errorf("schema %s: required %q ไม่มีใน properties", orRoot(path), r)
		}
	}
	return s.Items.check(path + "[]")
}

// Validate คืน error แรกที่เจอ (บอก path) · nil schema = ผ่าน
func (s *Schema) Validate(v any) error {
	return s.validate(v, "")
}

func (s *Schema) validate(v any, path string) error {
	if s == nil {
		return nil
	}
	if v == nil {
		if s.Nullable || containsStr(s.types(), "null") || len(s.types()) == 0 {
			return nil
		}
		return fmt.Errorf("%s: เป็น null", orRoot(path))
	}
	ts := s.types()
	if len(ts) > 0 && !matchesAnyType(v, ts) {
		return fmt.Errorf("%s: ชนิด %s ไม่ตรง %s", orRoot(path), jsonType(v), strings.Join(ts, "|"))
	}
	if m, ok := v.(map[string]any); ok {
		for _, r := range s.Required {
			if _, ok := m[r]; !ok {
				return fmt.Errorf("%s: ไม่มี field %q", orRoot(path), r)
			}
		}
		keys := make([]string, 0, len(s.Properties))
		for k := range s.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if fv, ok := m[k]; ok {
				if err := s.Properties[k].validate(fv, path+"."+k); err != nil {
					return err
				}
			}
		}
	}
	if list, ok := v.([]any); ok && s.Items != nil {
		for i, it := range list {
			if err := s.Items.validate(it, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func matchesAnyType(v any, ts []string) bool {
	for _, t := range ts {
		switch t {
		case "object":
			if _, ok := v.(map[string]any); ok {
				return true
			}
		case "array":
			if _, ok := v.([]any); ok {
				return true
			}
		case "string":
			if _, ok := v.(string); ok {
				return true
			}
		case "number":
			if _, ok := v.(float64); ok {
				return true
			}
		case "integer":
			if f, ok := v.(float64); ok && f == math.Trunc(f) {
				return true
			}
		case "boolean":
			if _, ok := v.(bool); ok {
				return true
			}
		case "null":
			if v == nil {
				return true
			}
		}
	}
	return false
}

func jsonType(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func orRoot(p string) string {
	if p == "" {
		return "(root)"
	}
	return strings.TrimPrefix(p, ".")
}
