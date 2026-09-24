package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

func cardsOf(evs []sseEvent) []map[string]any {
	var out []map[string]any
	for _, e := range eventsOf(evs, "card") {
		out = append(out, e.Data)
	}
	return out
}

func lastAssistant(t *testing.T, e *env) domain.Message {
	t.Helper()
	list, _, _ := e.msgs.Search(context.Background(), port.MessageFilter{Role: domain.RoleAssistant, Limit: 1})
	if len(list) == 0 {
		t.Fatal("ไม่มีข้อความของ AI ถูกบันทึก")
	}
	return list[0]
}

var digitsRe = regexp.MustCompile(`\d`)

// ครบวงจร 1 คำถาม (G2 ในเวอร์ชันจำลอง): ตั๋ว → tool → กุญแจดอกเล็ก → host → การ์ด → token → done → บันทึก
func TestChat_DataQuestionEndToEnd(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	evs := ask(t, e, tk, "มีรายการถอนค้างกี่รายการ")

	if evs[0].Type != "status" || evs[0].Data["conversation_id"] == "" {
		t.Fatalf("event แรกต้องเป็น status (ตอบสนอง ≤ 2 วิ · AC-2) ได้ %+v", evs[0])
	}
	cards := cardsOf(evs)
	if len(cards) != 1 || cards[0]["kind"] != "ok" {
		t.Fatalf("ต้องได้การ์ดข้อมูล 1 ใบ: %+v", cards)
	}
	tbl := cards[0]["table"].(map[string]any)
	rows := tbl["rows"].([]any)
	first := rows[0].([]any)
	if first[1].(map[string]any)["display"] != "7 รายการ" || first[2].(map[string]any)["display"] != "142,000.00" {
		t.Fatalf("ค่าบนการ์ดต้องเป็นค่าจาก host ตรงตัว (AC-14): %+v", first)
	}
	if cards[0]["fetched_at"] == "" || cards[0]["link"] == nil {
		t.Fatal("การ์ดต้องมีเวลาที่ดึง + ลิงก์หน้าจริงเสมอ")
	}
	done := eventsOf(evs, "done")
	if len(done) != 1 || done[0].Data["message_id"] == "" {
		t.Fatalf("ต้องจบด้วย done: %+v", evs)
	}
	m := lastAssistant(t, e)
	if len(m.Cards) != 1 || len(m.ToolCalls) != 1 || !m.ToolCalls[0].OK || m.Usage.Total() == 0 || m.Cost.Amount <= 0 || m.Category != "data" {
		t.Fatalf("บันทึกไม่ครบ (AC-11): %+v", m)
	}
	if m.ToolCalls[0].Endpoint != "/api/ai/read/withdraw-pending/{service}" {
		t.Fatalf("บันทึก endpoint ต้องเป็น template (ไม่มี PII): %q", m.ToolCalls[0].Endpoint)
	}
	if m.VerificationStatus != domain.VerifyPending {
		t.Fatalf("คำตอบใหม่ต้องเข้าคิวตรวจ: %q", m.VerificationStatus)
	}
}

// B-5: ห้ามมีข้อความของ AI ก่อนได้ข้อมูล — token ทุกตัวต้องมาหลังการ์ด
func TestChat_NoTokensBeforeData(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	evs := ask(t, e, tk, "ถอนค้างเท่าไหร่")
	lastCard, firstToken := -1, -1
	for i, ev := range evs {
		if ev.Type == "card" {
			lastCard = i
		}
		if ev.Type == "token" && firstToken < 0 {
			firstToken = i
		}
	}
	if firstToken < 0 || lastCard < 0 || firstToken < lastCard {
		t.Fatalf("token ต้องมาหลังการ์ดทั้งหมด (card=%d token=%d)", lastCard, firstToken)
	}
	// รอบเลือกเครื่องมือต้องบังคับ tool (ไม่มีข้อความหลุด) · รอบเขียนต้องไม่มี tool
	if e.llm.requests[0].ToolChoice != "any" || e.llm.requests[1].ToolChoice != "none" {
		t.Fatalf("tool_choice ผิด: %s / %s", e.llm.requests[0].ToolChoice, e.llm.requests[1].ToolChoice)
	}
}

// AC-14 · R-17: โมเดลพิมพ์ตัวเลข/ชื่อเอง → ผู้ใช้ไม่เห็น (ไม่พึ่ง prompt)
func TestChat_OutputGuardMasksModelNumbers(t *testing.T) {
	e := newEnv(t)
	e.llm.answer = func([]string) string {
		return "เว็บ K11S มีถอนค้าง 7 รายการ รวม 142,000.00 บาท และ 18,500 บาท\n1. เปิดเมนู รายการถอน เพื่อดูต่อ"
	}
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	text := joinedTokens(ask(t, e, tk, "ถอนค้าง"))
	for _, bad := range []string{"142,000", "18,500", " 7 "} {
		if strings.Contains(text, bad) {
			t.Fatalf("ข้อความต้องไม่มี %q: %q", bad, text)
		}
	}
	if !strings.Contains(text, "K11S") || !strings.Contains(text, "1. เปิดเมนู รายการถอน") {
		t.Fatalf("ชื่อเว็บ/เลขข้อ/ชื่อเมนูต้องไม่ถูกตัด: %q", text)
	}
	if m := lastAssistant(t, e); m.GuardHits == 0 || strings.Contains(m.Text, "142,000") {
		t.Fatalf("ต้องบันทึกข้อความหลังตัด + นับ guard hit: %+v", m)
	}
}

// model_context ที่ส่งให้โมเดลต้องไม่มีตัวเลข/ค่าจากระบบ (B-4)
func TestChat_ModelNeverSeesSystemValues(t *testing.T) {
	e := newEnv(t)
	var seen []string
	e.llm.answer = func(results []string) string { seen = results; return "ตามการ์ดครับ" }
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	ask(t, e, tk, "ถอนค้าง")
	joined := strings.Join(seen, " ")
	if joined == "" {
		t.Fatal("โมเดลต้องได้ tool_result")
	}
	for _, bad := range []string{"142000", "142,000", "18500", "\"count\"", "\"amount\""} {
		if strings.Contains(joined, bad) {
			t.Fatalf("tool_result ที่โมเดลเห็นมี %q: %s", bad, joined)
		}
	}
	if !strings.Contains(joined, "รอโอน") {
		t.Fatalf("ป้ายสถานะ (ไม่ใช่ตัวเลข) ควรส่งให้โมเดลได้: %s", joined)
	}
	// ข้อความผู้ใช้ + ตั๋ว/secret/grant ต้องไม่ไปถึงโมเดล
	b, _ := json.Marshal(e.llm.requests)
	grantHeader := strings.Split(fakeGrant("x", "y", time.Now()), ".")[0] // ส่วนหัวของ JWT grant
	for _, bad := range []string{tk, secretK11S, "scoped-abc", grantHeader} {
		if strings.Contains(string(b), bad) {
			t.Fatalf("request ของโมเดลมี %q", bad)
		}
	}
}

// ไม่มีสิทธิ์ → ปฏิเสธที่ backend ก่อนยิง host + บอกว่าต้องให้ใครเปิด (AC-4 · B-12)
func TestChat_PermissionDeniedNoHostCall(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser("M_VIEW"))
	evs := ask(t, e, tk, "ถอนค้าง")
	cards := cardsOf(evs)
	if len(cards) != 1 || cards[0]["kind"] != "denied" || !strings.Contains(fmt.Sprint(cards[0]["note"]), "รายการถอน") {
		t.Fatalf("ต้องได้การ์ดสิทธิ์ไม่ถึงพร้อมชื่อเมนู: %+v", cards)
	}
	if e.host.readCalls != 0 {
		t.Fatalf("ไม่มีสิทธิ์ต้องไม่ยิง host เลย (ยิง %d ครั้ง)", e.host.readCalls)
	}
}

// host ปฏิเสธสิทธิ์เอง (ชั้นที่ 2) → การ์ด denied ไม่มีตัวเลข
func TestChat_HostDeniesToo(t *testing.T) {
	e := newEnv(t)
	e.host.set(func(h *mockHost) { h.denyRead = true })
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	cards := cardsOf(ask(t, e, tk, "ถอนค้าง"))
	if cards[0]["kind"] != "denied" {
		t.Fatalf("host 403 ต้องเป็น denied: %+v", cards[0])
	}
}

// B-11: ไม่พบ ≠ error ≠ 0
func TestChat_MemberNotFound(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	cards := cardsOf(ask(t, e, tk, "เช็กสมาชิก nobody99"))
	if len(cards) != 1 || cards[0]["kind"] != "not_found" || len(cards[0]["fields"].([]any)) != 0 {
		t.Fatalf("สมาชิกไม่มีต้องเป็น not_found ไม่มีตัวเลข: %+v", cards)
	}
	ok := cardsOf(ask(t, e, tk, "เช็กสมาชิก somchai99"))
	if ok[0]["kind"] != "ok" {
		t.Fatalf("สมาชิกที่มีต้องได้การ์ด: %+v", ok)
	}
	// host ตอบ HTTP 404 (body code 404) + connector ประกาศ not_found_codes: [404] → ยังเป็น "ไม่พบ" ไม่ใช่ "ดึงไม่ได้"
	cards = cardsOf(ask(t, e, tk, "เช็กสมาชิก gone404"))
	if len(cards) != 1 || cards[0]["kind"] != "not_found" {
		t.Fatalf("HTTP 404 ที่ประกาศไว้ต้องเป็น not_found: %+v", cards)
	}
}

// AC-9 · AC-30: host ล่ม / ผิด schema → ไม่มีตัวเลขการเงินใด ๆ
func TestChat_HostDownOrSchemaMismatchNoNumbers(t *testing.T) {
	for name, fn := range map[string]func(h *mockHost){
		"host ล่ม":    func(h *mockHost) { h.down = true },
		"ผิด schema": func(h *mockHost) { h.withdrawBody = `{"code":0,"data":{"rows":[{"status":"2","count":"many"}]}}` },
		"code ไม่ ok": func(h *mockHost) { h.withdrawBody = `{"code":5,"message":"error"}` },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.host.set(fn)
			tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
			evs := ask(t, e, tk, "ถอนค้าง")
			cards := cardsOf(evs)
			if len(cards) != 1 || cards[0]["kind"] != "error" || cards[0]["table"] != nil || len(cards[0]["fields"].([]any)) != 0 {
				t.Fatalf("ต้องเป็นการ์ด error ไม่มีตัวเลข: %+v", cards)
			}
			b, _ := json.Marshal(evs)
			if strings.Contains(string(b), "142,000") || strings.Contains(string(b), "142000") {
				t.Fatalf("ต้องไม่มีตัวเลขบางส่วนหลุด: %s", b)
			}
			if cards[0]["link"] == nil {
				t.Fatal("ดึงไม่ได้ต้องยังมีลิงก์หน้าจริง")
			}
		})
	}
}

// AC-32 · P-9: host เห็นแค่กุญแจดอกเล็ก — ไม่เคยได้ Authorization/Cookie/IP ของแอดมิน
func TestChat_HostNeverReceivesAdminCredentials(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	b, _ := json.Marshal(map[string]string{"text": "ถอนค้าง"})
	do(t, e.r, req{method: http.MethodPost, path: basePath(demoKey, "K11S") + "/chat", origin: officeOrigin, token: tk, body: string(b),
		headers: [][2]string{{"Cookie", "session=admin"}, {"CF-Connecting-IP", "1.2.3.4"}}})
	if len(e.host.seenHeaders) == 0 {
		t.Fatal("ต้องมีการยิง host")
	}
	for _, h := range e.host.seenHeaders {
		for _, k := range []string{"Authorization", "Cookie", "Cf-Connecting-Ip", "Headertoken", "X-Forwarded-For"} {
			if h.Get(k) != "" {
				t.Fatalf("host ได้ header %s = %q", k, h.Get(k))
			}
		}
		if strings.Contains(fmt.Sprint(h), tk) {
			t.Fatal("ตั๋วของ widget ต้องไม่ไปถึง host")
		}
	}
}

// host ไม่รับ grant แล้ว (เช่น ผู้ใช้ถูกปิด) → ดึงไม่ได้ ไม่ค้าง
func TestChat_GrantRejected(t *testing.T) {
	e := newEnv(t)
	e.host.set(func(h *mockHost) { h.grantRejected = true })
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	cards := cardsOf(ask(t, e, tk, "ถอนค้าง"))
	if cards[0]["kind"] != "error" {
		t.Fatalf("grant ถูกปฏิเสธต้องเป็น error: %+v", cards[0])
	}
}

// กุญแจดอกเล็กถูกขอครั้งเดียวแล้วจำไว้ในหน่วยความจำ (ไม่ขอทุกครั้ง)
func TestChat_ScopedTokenReused(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	ask(t, e, tk, "เช็กสมาชิก somchai99")
	ask(t, e, tk, "เช็กสมาชิก somchai99")
	if e.host.scopedCalls != 1 {
		t.Fatalf("ควรขอกุญแจ 1 ครั้งแล้วใช้ซ้ำ ได้ %d", e.host.scopedCalls)
	}
}

// B-7: สรุปแคชได้ ≤ 60 วิ · ยอดสด (live) ห้ามแคช · การ์ดบอกว่ามาจากแคช
func TestChat_CachePolicy(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	ask(t, e, tk, "ถอนค้าง")
	second := cardsOf(ask(t, e, tk, "ถอนค้าง"))
	if e.host.readCalls != 1 || second[0]["cached"] != true {
		t.Fatalf("สรุปต้องมาจากแคชครั้งที่ 2 (อ่าน host %d ครั้ง) %+v", e.host.readCalls, second[0])
	}
	ask(t, e, tk, "เช็กสมาชิก somchai99")
	ask(t, e, tk, "เช็กสมาชิก somchai99")
	if e.host.readCalls != 3 {
		t.Fatalf("live ต้องไม่แคช (อ่าน host ทั้งหมด %d ครั้ง อยากได้ 3)", e.host.readCalls)
	}
	// ผู้ใช้สิทธิ์ต่างกันไม่ใช้แคชร่วมกัน
	tk2 := ticketFor(t, e.r, demoKey, "K11S", secretK11S, hostUser{ID: "emp_1", Username: "adm_ploy", Permissions: []string{"W_VIEW"}})
	ask(t, e, tk2, "ถอนค้าง")
	if e.host.readCalls != 4 {
		t.Fatalf("ชุดสิทธิ์ต่างกันต้องไม่ใช้แคชร่วม (อ่าน %d)", e.host.readCalls)
	}
}

// AC-15: ถาม "วันนี้" ตอน 00:30 เวลาไทย ต้องได้วันใหม่
func TestChat_TodayIsBangkokDay(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	e.now.Set(time.Date(2026, 9, 23, 17, 30, 0, 0, time.UTC)) // = 24/09 00:30 ICT
	ask(t, e, tk, "ถอนค้างวันนี้")
	if e.host.lastQuery != "date=2026-09-24" {
		t.Fatalf("วันนี้ต้องเป็นวันไทย ได้ query %q", e.host.lastQuery)
	}
}

// ขอให้ทำรายการ → ปฏิเสธ + การ์ดพาไปเมนูที่ต้องทำเอง (AC-8 · BQ-51)
func TestChat_ActionRequestRefusedWithMenu(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	evs := ask(t, e, tk, "เพิ่มเครดิตให้ยูส somchai99 หน่อย 500")
	cards := cardsOf(evs)
	if len(cards) == 0 || cards[0]["kind"] != "reference" {
		t.Fatalf("ต้องมีการ์ดอ้างอิงเมนู: %+v", cards)
	}
	if m := lastAssistant(t, e); m.Category != "action_request" {
		t.Fatalf("ต้องนับเป็นการปฏิเสธ action_request ได้ %q", m.Category)
	}
	if e.host.readCalls != 0 {
		t.Fatal("ขอทำรายการต้องไม่ยิง host")
	}
}

func TestChat_ExplainStatusFromConnector(t *testing.T) {
	e := newEnv(t)
	var seen []string
	e.llm.answer = func(r []string) string { seen = r; return "สถานะ OTP ไม่ถูกต้อง ให้ทำรายการอีกครั้งครับ" }
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	text := joinedTokens(ask(t, e, tk, "สถานะ 10 แปลว่าอะไร"))
	if !strings.Contains(strings.Join(seen, ""), "ทำรายการอีกครั้ง") {
		t.Fatalf("ต้องได้ next_action จาก statuses.yaml: %v", seen)
	}
	if !strings.Contains(text, "OTP ไม่ถูกต้อง") {
		t.Fatalf("ป้ายสถานะต้องไม่ถูก guard ตัด: %q", text)
	}
}

// เต็มลิมิตพร้อมกัน → ปฏิเสธทันที ไม่เข้าคิว (spec §8 · AC-16)
func TestChat_BusyRejectedImmediately(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	for i := 0; i < 3; i++ {
		if _, ok, _ := e.cnt.Acquire(context.Background(), "ai:conc:demo:K11S", 3, time.Minute); !ok {
			t.Fatal("จอง slot ไม่ได้")
		}
	}
	start := time.Now()
	errs := eventsOf(ask(t, e, tk, "ถอนค้าง"), "error")
	if len(errs) != 1 || errs[0].Data["code"] != "busy" || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("ต้องปฏิเสธทันทีด้วย busy: %+v (%v)", errs, time.Since(start))
	}
}

// AC-25: โควตาเต็ม → เฉพาะเว็บนั้นหยุด · ข้อความชัด
func TestChat_QuotaExceededOnlyThatService(t *testing.T) {
	o := demoOffice()
	o.Services[1].Enabled = true
	o.Services[0].Quota.MonthlyLimit = 1000
	e := newEnv(t, o)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	ask(t, e, tk, "ถอนค้าง") // ใช้ไป ~2,330 token > 1,000
	errs := eventsOf(ask(t, e, tk, "ถอนค้าง"), "error")
	if len(errs) != 1 || errs[0].Data["code"] != "quota_exceeded" || !strings.Contains(fmt.Sprint(errs[0].Data["message"]), "โควตา") {
		t.Fatalf("ต้องได้ quota_exceeded: %+v", errs)
	}
	// โควตาเต็มยังออกตั๋วได้ — ผู้ใช้ต้องเห็นข้อความ ไม่ใช่ปุ่มหาย (spec §8) · bootstrap บอก notice
	tk2 := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	if w := do(t, e.r, req{method: http.MethodGet, path: bootPath(demoKey, "K11S"), origin: officeOrigin, token: tk2}); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"notice":"quota_exceeded"`) || !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("bootstrap ต้องบอก notice quota_exceeded: %d %s", w.Code, w.Body.String())
	}
	tkPG := ticketFor(t, e.r, demoKey, "PG99", secretPG99, adminUser())
	b, _ := json.Marshal(map[string]string{"text": "ถอนค้าง"})
	if errs := eventsOf(chatReq(t, e.r, demoKey, "PG99", tkPG, string(b)), "error"); len(errs) != 0 {
		t.Fatalf("เว็บอื่นต้องใช้ได้ปกติ: %+v", errs)
	}
	// เพิ่มชั่วคราว → ใช้ต่อได้ทันที + บันทึกผู้ทำ
	w := do(t, e.r, req{method: http.MethodPost, path: "/api/ai/admin/offices/demo/services/K11S/quota/increase", console: consoleToken,
		body: `{"amount":100000,"reason":"ทดสอบ"}`})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"by":"break-glass"`) {
		t.Fatalf("เพิ่มโควตาต้องบันทึกผู้ทำ: %d %s", w.Code, w.Body.String())
	}
	if errs := eventsOf(ask(t, e, tk, "ถอนค้าง"), "error"); len(errs) != 0 {
		t.Fatalf("เพิ่มโควตาแล้วต้องตอบต่อได้: %+v", errs)
	}
}

func TestChat_LLMDown(t *testing.T) {
	e := newEnv(t)
	e.llm.fail = true
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	errs := eventsOf(ask(t, e, tk, "ถอนค้าง"), "error")
	if len(errs) != 1 || errs[0].Data["code"] != "llm_unavailable" {
		t.Fatalf("โมเดลล่มต้องบอกตามจริง: %+v", errs)
	}
	if m := lastAssistant(t, e); m.Error != "llm_unavailable" {
		t.Fatalf("ต้องบันทึกข้อผิดพลาด: %+v", m)
	}
}

// ห้องเดิม + ถามซ้ำ = หยุดวน (B-13 · AC-12) · ประวัติส่งให้โมเดลโดยไม่มีค่าบนการ์ด
func TestChat_ConversationAndRepeat(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	evs := ask(t, e, tk, "ถอนค้างกี่รายการ")
	conv := fmt.Sprint(eventsOf(evs, "done")[0].Data["conversation_id"])

	var note string
	e.llm.answer = func(r []string) string { return "ok" }
	b, _ := json.Marshal(map[string]string{"text": "ถอนค้างกี่รายการ", "conversation_id": conv})
	chatReq(t, e.r, demoKey, "K11S", tk, string(b))
	last := e.llm.requests[len(e.llm.requests)-1]
	for _, blk := range last.Messages[len(last.Messages)-1].Blocks {
		if blk.Type == port.BlockText {
			note = blk.Text
		}
	}
	if !strings.Contains(note, "ถามเรื่องเดิมซ้ำ") {
		t.Fatalf("ถามซ้ำต้องสั่งให้หยุดวน: %q", note)
	}
	hist, _ := json.Marshal(last.Messages[:2])
	if !strings.Contains(string(hist), "การ์ดที่แสดงไปแล้ว") || strings.Contains(string(hist), "142,000") {
		t.Fatalf("ประวัติต้องมีแค่ชื่อการ์ด ไม่มีค่า: %s", hist)
	}
}

// B-15: ปิดห้อง (เปลี่ยน service) แล้วถามต่อ = ห้องใหม่ · ประวัติ 7 วันเห็นเฉพาะของตัวเอง
func TestChat_CloseAndHistory(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	conv := fmt.Sprint(eventsOf(ask(t, e, tk, "ถอนค้าง"), "done")[0].Data["conversation_id"])

	w := do(t, e.r, req{method: http.MethodGet, path: basePath(demoKey, "K11S") + "/conversations", origin: officeOrigin, token: tk})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), conv) {
		t.Fatalf("ต้องเห็นห้องของตัวเอง: %s", w.Body.String())
	}
	w = do(t, e.r, req{method: http.MethodGet, path: basePath(demoKey, "K11S") + "/conversations/" + conv, origin: officeOrigin, token: tk})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "142,000.00") {
		t.Fatalf("เปิดประวัติต้องเห็นการ์ดเดิม: %s", w.Body.String())
	}
	for _, leak := range []string{"tool_calls", "/api/ai/read", "usage", "cost", "model"} {
		if strings.Contains(w.Body.String(), `"`+leak) || strings.Contains(w.Body.String(), leak+`"`) {
			t.Fatalf("ประวัติฝั่ง widget ต้องไม่มี %q: %s", leak, w.Body.String())
		}
	}
	other := ticketFor(t, e.r, demoKey, "K11S", secretK11S, hostUser{ID: "emp_2", Username: "adm_ploy"})
	w = do(t, e.r, req{method: http.MethodGet, path: basePath(demoKey, "K11S") + "/conversations/" + conv, origin: officeOrigin, token: other})
	if w.Code != http.StatusNotFound {
		t.Fatalf("ผู้ใช้อื่นต้องอ่านห้องนี้ไม่ได้: %d", w.Code)
	}

	w = do(t, e.r, req{method: http.MethodPost, path: basePath(demoKey, "K11S") + "/conversations/" + conv + "/close", origin: officeOrigin, token: tk, body: `{"reason":"switch_service"}`})
	if w.Code != http.StatusOK {
		t.Fatalf("ปิดห้อง: %d %s", w.Code, w.Body.String())
	}
	b, _ := json.Marshal(map[string]string{"text": "สวัสดี", "conversation_id": conv})
	evs := chatReq(t, e.r, demoKey, "K11S", tk, string(b))
	if got := fmt.Sprint(eventsOf(evs, "done")[0].Data["conversation_id"]); got == conv {
		t.Fatal("ห้องที่ปิดแล้วต้องไม่ถูกใช้ต่อ")
	}
	c, _ := e.convs.Get(context.Background(), "demo", "K11S", conv)
	if c.ClosedReason != "switch_service" {
		t.Fatalf("ต้องบันทึกเหตุผลที่ปิด: %+v", c)
	}
}

func TestChat_EmptyAndHugeRejected(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	for _, text := range []string{"   ", strings.Repeat("ก", 2001)} {
		errs := eventsOf(ask(t, e, tk, text), "error")
		if len(errs) != 1 || errs[0].Data["code"] != "bad_request" {
			t.Fatalf("ข้อความว่าง/ยาวเกินต้องถูกปฏิเสธ: %+v", errs)
		}
	}
}

func TestChat_GreetingHasNoCardNoHost(t *testing.T) {
	e := newEnv(t)
	e.llm.answer = func([]string) string { return "สวัสดีครับ ผมเป็นระบบอัตโนมัติ ถามเรื่องยอดหรือรายการค้างได้เลยครับ" }
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	evs := ask(t, e, tk, "สวัสดี")
	if len(cardsOf(evs)) != 0 || e.host.readCalls != 0 || digitsRe.MatchString(joinedTokens(evs)) {
		t.Fatalf("ทักทายต้องไม่มีการ์ด/ไม่ยิง host: %+v", evs)
	}
	if m := lastAssistant(t, e); m.Category != "greeting" {
		t.Fatalf("category ผิด: %q", m.Category)
	}
}
