package connector

import (
	"testing"
)

// TestConnectors โหลดทุก kind ใต้ connectors/ แล้วรัน contract-tests ทั้งหมด
func TestConnectors(t *testing.T) {
	all, err := LoadDir("../../../connectors")
	if err != nil {
		t.Fatalf("load connectors: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("ไม่พบ connector เลย")
	}
	for kind, c := range all {
		files, err := c.LoadContracts()
		if err != nil {
			t.Fatalf("%s contracts: %v", kind, err)
		}
		tested := map[string]bool{}
		for _, cf := range files {
			tested[cf.Tool] = true
			for _, e := range c.RunContract(cf) {
				t.Errorf("%s/%s: %v", kind, cf.Tool, e)
			}
		}
		for _, tool := range c.Tools {
			if !tested[tool.Name] {
				t.Errorf("%s: tool %s ไม่มี contract test", kind, tool.Name)
			}
		}
	}
}
