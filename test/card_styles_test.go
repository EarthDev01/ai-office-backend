package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

// แบบการ์ดต่อคำถามตั้งที่ domain · ค่าว่าง = กลับไปใช้แบบตั้งต้น · แบบที่ไม่รู้จัก = 400
func TestOffice_CardStyles(t *testing.T) {
	r := newRouter(t)
	code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"card_styles":{"deposit_find":"table","member_lookup":"single","withdraw_find":""}}`)
	if code != 200 || o.CardStyles["deposit_find"] != "table" || o.CardStyles["member_lookup"] != "single" {
		t.Fatalf("PATCH = %d %+v", code, o.CardStyles)
	}
	if _, ok := o.CardStyles["withdraw_find"]; ok {
		t.Fatal("ค่าว่างต้องไม่ถูกเก็บ")
	}
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"card_styles":{"deposit_find":"pie"}}`); code != 400 {
		t.Fatalf("แบบที่ไม่รู้จัก = %d อยากได้ 400", code)
	}
	// PATCH ที่ไม่ส่ง card_styles ต้องไม่ล้างของเดิม
	_, o = admin(t, r, http.MethodPatch, "/api/ai/admin/offices/demo", `{"label":"หลังบ้านจำลอง"}`)
	if o.CardStyles["deposit_find"] != "table" {
		t.Fatalf("ไม่ส่งแต่ถูกล้าง: %+v", o.CardStyles)
	}
}

// /kinds บอกคำถามของแต่ละชนิด + แบบตั้งต้น ให้คอนโซลทำตัวเลือก
func TestKinds_ListToolsWithStyles(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: "GET", path: "/api/ai/admin/kinds", console: consoleToken})
	body := w.Body.String()
	for _, want := range []string{`"name":"deposit_total"`, `"style":"stat"`, `"styles":["stat","list","table","single"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("ไม่มี %s ใน %s", want, body[:min(len(body), 400)])
		}
	}
}

// แบบที่ตั้งใน domain ทับแบบตั้งต้นของ connector ในการ์ดของคำตอบจริง · ไม่ตั้ง = แบบตั้งต้น
func TestChat_CardStyleFromOffice(t *testing.T) {
	for _, tc := range []struct{ override, want string }{{"", `"style":"single"`}, {"list", `"style":"list"`}} {
		o := demoOffice()
		if tc.override != "" {
			o.CardStyles = map[string]string{"member_lookup": tc.override}
		}
		d := newDeps(t, o)
		d.Chat = service.NewChatService(&fakeLLM{uses: []port.ToolUse{lookup("u1", "somchai01")}, reply: []string{"พบสมาชิกครับ"}},
			d.ChatRepo, service.NewToolRunner(d.Relay, d.Settings), time.UTC, d.Settings, d.Usage)
		srv := httptest.NewServer(httpgin.NewTestRouter(d))
		ev := runChat(t, srv.URL, openSession(t, srv.URL, "MEMBER_INFO"), "สมาชิก somchai01", func(service.FetchCommand) string { return memberBody })
		srv.Close()
		cards := find(ev, "card")
		if len(cards) == 0 || !strings.Contains(string(cards[0].data), tc.want) {
			t.Fatalf("override %q: การ์ด = %v อยากได้ %s", tc.override, cards, tc.want)
		}
	}
}
