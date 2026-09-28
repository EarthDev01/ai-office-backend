package test

import (
	"net/http"
	"testing"
)

// สีหลักของ widget ตั้งจากคอนโซลระดับ domain แล้วส่งถึง widget ทาง bootstrap
func TestOffice_AccentColor(t *testing.T) {
	r := newRouter(t)

	code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"accent_color":" #1E66F5 "}`)
	if code != http.StatusOK || o.Accent != "#1e66f5" {
		t.Fatalf("ต้องเก็บเป็นตัวพิมพ์เล็กที่ตัดช่องว่างแล้ว: %d %q", code, o.Accent)
	}
	_, b, raw := getBoot(t, r, "K11S", adminJWT("adm_ploy", "K11S"))
	if b.AccentColor != "#1e66f5" {
		t.Fatalf("bootstrap ต้องมีสี: %s", raw)
	}

	for _, bad := range []string{`"red"`, `"#12345"`, `"#1234567"`, `"url(x)"`, `"#12345g"`} {
		if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"accent_color":`+bad+`}`); code != http.StatusBadRequest {
			t.Errorf("สี %s ต้อง 400 ได้ %d", bad, code)
		}
	}

	// ว่าง = กลับไปใช้สีตั้งต้นของ widget
	if code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"accent_color":""}`); code != http.StatusOK || o.Accent != "" {
		t.Fatalf("ล้างสีไม่ได้: %d %q", code, o.Accent)
	}
	// ไม่ได้ส่งมา = ไม่แตะค่าเดิม
	admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"accent_color":"#aa3300"}`)
	if _, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"label":"x"}`); o.Accent != "#aa3300" {
		t.Fatalf("PATCH ที่ไม่ส่งสีต้องไม่ล้างสี: %q", o.Accent)
	}
}
