package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ปุ่มใต้คำตอบ + guest ตรวจตอนโหลด — ใช้ connector จริงของ theme-tangtem แล้วแก้ host.yaml ให้ผิดทีละจุด
func TestLoad_PageActionsAndGuest(t *testing.T) {
	src := filepath.Join("..", "..", "..", "connectors", "theme-tangtem")
	c, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := c.PageAction("promotion"); !ok || a.Open.BVModal != "bv-modal-promotion" || a.AvailableTo(true) || !a.AvailableTo(false) {
		t.Fatalf("promotion = %+v", a)
	}
	if a, _ := c.PageAction("register"); !a.AvailableTo(true) || a.AvailableTo(false) {
		t.Fatalf("register ต้องเป็นของผู้ไม่ล็อกอินเท่านั้น: %+v", a)
	}
	if a, _ := c.PageAction("guide"); !a.AvailableTo(true) || !a.AvailableTo(false) {
		t.Fatalf("guide ต้องใช้ได้ทุกคน: %+v", a)
	}
	if c.Host.Guest == nil || c.Host.Guest.DailyLimit != 6 {
		t.Fatalf("guest = %+v", c.Host.Guest)
	}

	cases := map[string]struct{ from, to, want string }{
		"open ไม่ตั้ง":      {"open: {path: /guide}", "open: {}", "อย่างใดอย่างหนึ่ง"},
		"open ตั้ง 2 อย่าง": {"open: {path: /guide}", "open: {path: /guide, bv_modal: x}", "อย่างใดอย่างหนึ่ง"},
		"path นอกเว็บ":      {"open: {path: /guide}", "open: {path: //evil.test/x}", "เว็บเดียวกัน"},
		"selector แปลก":     {`open: {click: "#buttonFooterwheel a"}`, `open: {click: "a{color:red}"}`, "selector"},
		"modal id แปลก":     {"open: {bv_modal: bv-modal-history}", `open: {bv_modal: "x');alert(1)"}`, "bv_modal"},
		"id ซ้ำ":            {"id: event,", "id: history,", "ไม่ซ้ำ"},
		"when ผิด":          {"when: guest, open: {bv_modal: bv-modal-login}", "when: vip, open: {bv_modal: bv-modal-login}", "any|member|guest"},
		"ไม่มี about":       {`about: "หน้าคู่มือการใช้งานเว็บ"`, `about: ""`, "about"},
		"โควตาเกิน":         {"guest: {daily_limit: 6}", "guest: {daily_limit: 0}", "daily_limit"},
		"label ยาวเกิน":     {"label: คู่มือการใช้งาน,", "label: คู่มือการใช้งานแบบยาวมากเกินกว่าสามสิบตัวอักษรแน่นอน,", "label"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "theme-tangtem")
			copyDir(t, src, dst)
			p := filepath.Join(dst, "host.yaml")
			b, _ := os.ReadFile(p)
			if !strings.Contains(string(b), tc.from) {
				t.Fatalf("host.yaml ไม่มี %q", tc.from)
			}
			os.WriteFile(p, []byte(strings.Replace(string(b), tc.from, tc.to, 1)), 0o644)
			_, err := Load(dst)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("อยากได้ error มี %q ได้ %v", tc.want, err)
			}
		})
	}
}

// paths — ใช้ path แรกที่มีค่า (รูปข้อมูลของหลังบ้านเปลี่ยนตามการตั้งค่า)
func TestEvalValue_PathsFallback(t *testing.T) {
	v := ValueSpec{Name: "rows", From: "b", Paths: []string{"bonus", ""}, Filter: "item.id != 104"}
	for name, data := range map[string]string{
		"รายการตรง ๆ": `[{"id":1},{"id":104}]`,
		"data.bonus":  `{"balance_realcash":5,"bonus":[{"id":1},{"id":104}]}`,
	} {
		got, err := evalValue(v, map[string]*CallResult{"b": {Data: jsonAny(t, data)}}, map[string]any{})
		list, _ := got.([]any)
		if err != nil || len(list) != 1 {
			t.Fatalf("%s: ได้ %v %v", name, got, err)
		}
	}
}

// ทุกคำถามของทุก connector ต้องมีปุ่มถามต่อ — connector ใหม่ในอนาคตลืมใส่ test นี้จะเตือน
func TestConnectors_EveryToolHasFollowUps(t *testing.T) {
	root := filepath.Join("..", "..", "..", "connectors")
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := Load(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		for _, tool := range c.Tools {
			if len(tool.FollowUps) == 0 {
				t.Errorf("%s/%s: ไม่มี follow_ups (ปุ่มถามต่อ)", e.Name(), tool.Name)
			}
		}
		if len(c.Host.FollowUps) == 0 {
			t.Errorf("%s: host.yaml ไม่มี follow_ups (ปุ่มตั้งต้น)", e.Name())
		}
	}
}
