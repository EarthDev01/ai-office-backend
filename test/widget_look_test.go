package test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	httpgin "ai-office-backend/internal/adapter/handler/gin"
)

// รูปผู้ช่วย · คำโปรย · พื้นหลัง — ตั้งจากคอนโซล ถึง widget ผ่าน bootstrap · ค่าอันตรายถูกปฏิเสธ
func TestServiceLook_PatchAndBootstrap(t *testing.T) {
	r := newRouter(t)
	code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S",
		`{"avatar_url":"asset:avatars/dara.png","tagline":"ผู้ช่วยดูแลลูกค้า · ตอบทันที 24 ชม.","background":"pattern:dots"}`)
	if code != http.StatusOK {
		t.Fatalf("patch = %d", code)
	}
	svc, _ := o.FindService("K11S")
	if svc.AvatarURL != "asset:avatars/dara.png" || svc.Background != "pattern:dots" || svc.Tagline == "" {
		t.Fatalf("ไม่ถูกบันทึก: %+v", svc)
	}
	w := do(t, r, req{method: "GET", path: bootPath("K11S"), origin: officeOrigin, token: adminJWT("adm_ploy", "K11S")})
	b := payload[struct {
		Tagline, Background string
	}](t, w)
	if b.Tagline == "" || b.Background != "pattern:dots" {
		t.Fatalf("bootstrap ไม่ส่งหน้าตาให้ widget: %s", w.Body.String())
	}

	for _, bad := range []string{
		`{"background":"https://x.example.com/a.png\");background:url(x"}`,
		`{"avatar_url":"javascript:alert(1)"}`,
		`{"avatar_url":"pattern:dots"}`,
		`{"tagline":"` + string(make([]rune, 0)) + `ยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมากยาวมาก"}`,
	} {
		if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S", bad); code != http.StatusBadRequest {
			t.Errorf("%s ต้อง 400 ได้ %d", bad, code)
		}
	}
}

// คลังรูป: เสิร์ฟเฉพาะไฟล์รูปในโฟลเดอร์ · รายการให้คอนโซลเลือก
func TestWidgetAssets_ServeAndList(t *testing.T) {
	d := newDeps(t)
	dir := filepath.Join(filepath.Dir(d.BundlePath), "assets")
	_ = os.MkdirAll(filepath.Join(dir, "avatars"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "avatars", "dara.png"), []byte("\x89PNG"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "avatars", "notes.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(filepath.Dir(d.BundlePath), "secret.png"), []byte("x"), 0o644)
	r := httpgin.NewTestRouter(d)

	w := do(t, r, req{method: "GET", path: "/widget/v1/assets/avatars/dara.png"})
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("รูปในคลัง = %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	for _, p := range []string{"/widget/v1/assets/avatars/notes.txt", "/widget/v1/assets/../secret.png", "/widget/v1/assets/avatars/../../secret.png", "/widget/v1/assets/avatars/"} {
		if w := do(t, r, req{method: "GET", path: p}); w.Code != http.StatusNotFound {
			t.Errorf("%s ต้อง 404 ได้ %d", p, w.Code)
		}
	}
	w = do(t, r, req{method: "GET", path: "/api/ai/admin/widget-assets", console: consoleToken})
	l := payload[struct{ Avatars, Backgrounds, Patterns, Launchers []string }](t, w)
	if len(l.Avatars) != 1 || l.Avatars[0] != "asset:avatars/dara.png" || len(l.Backgrounds) != 0 || len(l.Patterns) != 1 || l.Launchers == nil {
		t.Fatalf("รายการคลังรูป = %+v", l)
	}
}

// เลือกสีเอง = ไล่สี 2–4 สีเท่านั้น · ถึง widget ผ่าน bootstrap
func TestOfficeGradientColors(t *testing.T) {
	r := newRouter(t)
	for body, want := range map[string]int{
		`{"accent_colors":["#f5d76e","#2563eb"]}`:                     http.StatusOK,
		`{"accent_colors":["#f5d76e","#22c55e","#2563eb","#7c3aed"]}`: http.StatusOK,
		`{"accent_colors":["#f5d76e"]}`:                               http.StatusBadRequest,
		`{"accent_colors":["#1","#2","#3","#4","#5"]}`:                http.StatusBadRequest,
		`{"accent_colors":["#f5d76e","red"]}`:                         http.StatusBadRequest,
		`{"accent_colors":[],"color_source":"site"}`:                  http.StatusOK,
	} {
		if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", body); code != want {
			t.Errorf("%s = %d อยากได้ %d", body, code, want)
		}
	}
	admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"accent_colors":["#f5d76e","#22c55e","#2563eb"],"color_source":""}`)
	w := do(t, r, req{method: "GET", path: bootPath("K11S"), origin: officeOrigin, token: adminJWT("adm_ploy", "K11S")})
	b := payload[struct {
		AccentColors []string `json:"accent_colors"`
	}](t, w)
	if len(b.AccentColors) != 3 || b.AccentColors[2] != "#2563eb" {
		t.Fatalf("bootstrap ไม่ส่งสีไล่: %s", w.Body.String())
	}
}

// "สุ่ม" = bootstrap แปลงเป็นรูปจริงจากคลังทุกครั้ง · widget ไม่เคยเห็นคำว่า random
func TestServiceLook_RandomResolvedAtBootstrap(t *testing.T) {
	d := newDeps(t)
	dir := filepath.Join(filepath.Dir(d.BundlePath), "assets")
	_ = os.MkdirAll(filepath.Join(dir, "avatars"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "backgrounds"), 0o755)
	for _, f := range []string{"avatars/a1.jpg", "avatars/a2.jpg", "backgrounds/b1.jpg"} {
		_ = os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	r := httpgin.NewTestRouter(d)
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S", `{"avatar_url":"random","background":"random"}`); code != 200 {
		t.Fatalf("บันทึก random = %d", code)
	}
	seen := map[string]bool{}
	for i := 0; i < 30; i++ {
		w := do(t, r, req{method: "GET", path: bootPath("K11S"), origin: officeOrigin, token: adminJWT("adm_ploy", "K11S")})
		b := payload[struct {
			AvatarURL  string `json:"avatar_url"`
			Background string
		}](t, w)
		if b.AvatarURL != "asset:avatars/a1.jpg" && b.AvatarURL != "asset:avatars/a2.jpg" {
			t.Fatalf("avatar ต้องเป็นรูปในคลัง: %q", b.AvatarURL)
		}
		if b.Background != "pattern:glow" && b.Background != "asset:backgrounds/b1.jpg" {
			t.Fatalf("พื้นหลังต้องมาจากตัวเลือกที่มี: %q", b.Background)
		}
		seen[b.AvatarURL] = true
	}
	if len(seen) < 2 {
		t.Fatalf("สุ่ม 30 ครั้งได้รูปเดียว: %v", seen)
	}
}

// ปุ่มเปิดแชท: ไม่ได้เลือก = รูปแรกในคลัง · เลือกได้เฉพาะรูปในโฟลเดอร์ launchers
func TestServiceLook_LauncherIcon(t *testing.T) {
	d := newDeps(t)
	dir := filepath.Join(filepath.Dir(d.BundlePath), "assets", "launchers")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "bubble-1.png"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "bubble-2.png"), []byte("x"), 0o644)
	r := httpgin.NewTestRouter(d)
	boot := func() string {
		w := do(t, r, req{method: "GET", path: bootPath("K11S"), origin: officeOrigin, token: adminJWT("adm_ploy", "K11S")})
		return payload[struct {
			LauncherIcon string `json:"launcher_icon"`
		}](t, w).LauncherIcon
	}
	if got := boot(); got != "asset:launchers/bubble-1.png" {
		t.Fatalf("ค่าตั้งต้นต้องเป็นรูปแรก: %q", got)
	}
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S", `{"launcher_icon":"asset:launchers/bubble-2.png"}`); code != 200 {
		t.Fatalf("เลือกรูป = %d", code)
	}
	if got := boot(); got != "asset:launchers/bubble-2.png" {
		t.Fatalf("ต้องได้รูปที่เลือก: %q", got)
	}
	for _, bad := range []string{`{"launcher_icon":"asset:avatars/a.png"}`, `{"launcher_icon":"https://x.example.com/a.png"}`} {
		if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo/services/K11S", bad); code != http.StatusBadRequest {
			t.Errorf("%s ต้อง 400 ได้ %d", bad, code)
		}
	}
}
