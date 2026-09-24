package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R1 · spec §5.3 กฎปลั๊ก: ห้ามมีของ kind ใดโดยเฉพาะในโค้ดกลาง (internal/core + widget ส่วนกลาง)
// ของเฉพาะ kind ต้องอยู่ใน connectors/<kind>/ (+ widget/src/hosts/<kind>.ts ถ้าจำเป็น) เท่านั้น
var forbidden = []string{
	"auth_token", "web-service", "list_service", "employees-byid", "t_account_office", "headertoken",
	"office-v10x", "office-abatech", "office-api-v10", "GOTOPOPOFFICE", "abaoffice",
}

func TestPluginLint_NoKindSpecificStringsInCore(t *testing.T) {
	roots := []string{filepath.Join("..", "internal", "core"), filepath.Join("..", "widget", "src")}
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if info != nil && info.IsDir() && (info.Name() == "testdata" || info.Name() == "hosts") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, ".ts") {
				return nil
			}
			b, _ := os.ReadFile(p)
			for i, line := range strings.Split(string(b), "\n") {
				for _, f := range forbidden {
					if strings.Contains(line, f) {
						t.Errorf("%s:%d มี %q — ของเฉพาะ kind ต้องอยู่ใน connectors/<kind>/", p, i+1, f)
					}
				}
			}
			return nil
		})
	}
}
