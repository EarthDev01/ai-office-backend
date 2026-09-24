package test

import (
	"sort"
	"strings"
	"testing"

	"ai-office-backend/internal/core/connector"
)

// connector จริงใน connectors/ — ประตูก่อนเปิดใช้ (spec §7.4 · AC-5 · AC-19 · C10)

var wantKinds = []string{"office-abatech", "office-v10x"}

// รอบแรก 31 ข้อ (spec §4 · ภาคผนวก ก) — หมวด E (BQ-52..56) ไม่ต้องมี tool จึงไม่อยู่ในรายการนี้
var firstRoundBQ = []string{
	"BQ-01", "BQ-02", "BQ-03", "BQ-04", "BQ-05", "BQ-06", "BQ-07", "BQ-08", "BQ-09", "BQ-10", "BQ-11", "BQ-12", "BQ-13",
	"BQ-29", "BQ-35",
	"BQ-41", "BQ-42", "BQ-43", "BQ-44", "BQ-45", "BQ-46", "BQ-47", "BQ-48", "BQ-49", "BQ-50", "BQ-51",
}

func loadReal(t *testing.T) map[string]*connector.Connector {
	t.Helper()
	reg, err := connector.LoadDir("../connectors")
	if err != nil {
		t.Fatalf("โหลด connectors/ ไม่ผ่าน:\n%v", err)
	}
	return reg
}

func TestRealConnectors_LoadBothKinds(t *testing.T) {
	reg := loadReal(t)
	for _, k := range wantKinds {
		if _, ok := reg[k]; !ok {
			t.Errorf("ไม่มี connector %q", k)
		}
	}
}

func TestRealConnectors_CoverFirstRound(t *testing.T) {
	for kind, c := range loadReal(t) {
		cov := c.Coverage()
		var missing []string
		for _, q := range firstRoundBQ {
			if len(cov[q]) == 0 {
				missing = append(missing, q)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s ยังไม่ครอบ %v", kind, missing)
		}
	}
}

// ทุก call ต้องไปที่ group /api/ai/read เท่านั้น (กุญแจดอกเล็กใช้ได้แค่ group นี้ · contract §4)
func TestRealConnectors_ReadGroupOnly(t *testing.T) {
	for kind, c := range loadReal(t) {
		if c.Host.IsBrowser() {
			continue // โหมด browser ใช้ API เดิมของหลังบ้าน (loader บังคับผูก {service} แทน)
		}
		if c.Host.HostAPI.ScopedTokenPath != "/api/ai/scoped-token" || c.Host.HostAPI.ScopedTokenHeader != "X-AI-Scoped-Token" {
			t.Errorf("%s: host_api ต้องตรง contract (scoped-token path/header)", kind)
		}
		if !strings.Contains(c.Host.PageAuth.SessionPath, "/ai/session/{service}") {
			t.Errorf("%s: session_path ต้องเป็น …/ai/session/{service}", kind)
		}
		for _, tool := range c.Tools {
			for _, cl := range tool.Calls {
				if !strings.HasPrefix(cl.Path, "/api/ai/read/") || !strings.HasSuffix(cl.Path, "/{service}") {
					t.Errorf("%s %s call %s: path %q ต้องเป็น /api/ai/read/<ชื่อ>/{service}", kind, tool.Name, cl.ID, cl.Path)
				}
			}
		}
	}
}

// C10: ทุก tool มี contract test · อย่างน้อย 1 case ที่ได้การ์ด และ 1 case ที่ไม่ใช่ ok (ไม่พบ/ดึงไม่ได้)
func TestRealConnectors_Contracts(t *testing.T) {
	for kind, c := range loadReal(t) {
		files, err := c.LoadContracts()
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		byTool := map[string][]connector.ContractFile{}
		for _, f := range files {
			byTool[f.Tool] = append(byTool[f.Tool], f)
			for _, e := range c.RunContract(f) {
				t.Errorf("%s: %v", kind, e)
			}
		}
		var noTest []string
		for _, tool := range c.Tools {
			fs := byTool[tool.Name]
			if len(fs) == 0 {
				noTest = append(noTest, tool.Name)
				continue
			}
			var ok, other bool
			for _, f := range fs {
				for _, cs := range f.Cases {
					if cs.Expect.Status == connector.StatusOK {
						ok = true
					} else if cs.Expect.Status != "" {
						other = true
					}
				}
			}
			if !ok || !other {
				t.Errorf("%s %s: contract ต้องมีทั้ง case ok และ case ไม่พบ/ดึงไม่ได้", kind, tool.Name)
			}
		}
		sort.Strings(noTest)
		if len(noTest) > 0 {
			t.Errorf("%s: tool ที่ยังไม่มี contract test %v", kind, noTest)
		}
	}
}
