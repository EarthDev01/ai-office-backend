package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

func accessRows(e *env) []domain.AccessLog {
	list, _, _ := e.access.List(context.Background(), "", "", "", 1000, 0)
	return list
}

// AC-11: เปิดอ่าน 1 ห้อง = 1 แถว access_log พร้อมชื่อผู้เปิด
func TestAdmin_ReadConversationWritesAccessLog(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	conv := fmt.Sprint(eventsOf(ask(t, e, tk, "ถอนค้าง"), "done")[0].Data["conversation_id"])

	before := len(accessRows(e))
	w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/conversations/" + conv, console: consoleToken})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "142,000.00") {
		t.Fatalf("อ่านห้อง: %d %s", w.Code, w.Body.String())
	}
	rows := accessRows(e)
	if len(rows) != before+1 || rows[0].Action != "read_conversation" || rows[0].Operator != "break-glass" || rows[0].ConversationID != conv {
		t.Fatalf("ต้องมี access_log 1 แถว: %+v", rows)
	}
	// ค้นรายการห้องก็ต้องบันทึก (หัวข้อคือคำถามของผู้ใช้)
	do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/conversations?office_id=demo", console: consoleToken})
	if rows := accessRows(e); rows[0].Action != "search" {
		t.Fatalf("ค้นห้องต้องมี access_log: %+v", rows[0])
	}
}

// เก่ากว่า 90 วันต้องกดยืนยันเพิ่ม
func TestAdmin_OldConversationNeedsConfirm(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	e.now.Set(time.Now().Add(-100 * 24 * time.Hour))
	conv := fmt.Sprint(eventsOf(ask(t, e, tk, "สวัสดี"), "done")[0].Data["conversation_id"])
	e.now.Set(time.Time{})

	w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/conversations/" + conv, console: consoleToken})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "CONFIRM_OLD_REQUIRED") {
		t.Fatalf("เก่ากว่า 90 วันต้องขอยืนยัน: %d %s", w.Code, w.Body.String())
	}
	w = do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/conversations/" + conv + "?confirm_old=1", console: consoleToken})
	if w.Code != http.StatusOK || accessRows(e)[0].Action != "read_old" {
		t.Fatalf("ยืนยันแล้วต้องเปิดได้ + บันทึก read_old: %d", w.Code)
	}
}

// AC-27 · AC-28: ตรวจคำตอบ ผิดต้องมีประเภท+คำตอบที่ถูก · % ถูกมาพร้อมจำนวนยังไม่ตรวจ
func TestAdmin_VerificationFlow(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	ask(t, e, tk, "ถอนค้าง")
	ask(t, e, tk, "เช็กสมาชิก somchai99")

	w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/verifications/queue?office_id=demo", console: consoleToken})
	var q struct {
		Total int64 `json:"total"`
		Data  []struct {
			Question string         `json:"question"`
			Answer   domain.Message `json:"answer"`
		} `json:"data"`
	}
	q = payload[struct {
		Total int64 `json:"total"`
		Data  []struct {
			Question string         `json:"question"`
			Answer   domain.Message `json:"answer"`
		} `json:"data"`
	}](t, w)
	if q.Total != 2 || q.Data[0].Question != "ถอนค้าง" {
		t.Fatalf("คิวต้องมี 2 ข้อความพร้อมคำถาม: %s", w.Body.String())
	}
	m1, m2 := q.Data[0].Answer.ID, q.Data[1].Answer.ID

	bad := do(t, e.r, req{method: http.MethodPost, path: "/api/ai/admin/verifications", console: consoleToken,
		body: `{"message_id":"` + m1 + `","status":"wrong"}`})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("ผิดโดยไม่บอกประเภท/คำตอบที่ถูกต้องไม่ผ่าน: %d", bad.Code)
	}
	ok := do(t, e.r, req{method: http.MethodPost, path: "/api/ai/admin/verifications", console: consoleToken,
		body: `{"message_id":"` + m1 + `","status":"correct"}`})
	wr := do(t, e.r, req{method: http.MethodPost, path: "/api/ai/admin/verifications", console: consoleToken,
		body: `{"message_id":"` + m2 + `","status":"wrong","error_type":"wrong_number","correct_answer":"วันสมัครคือ 05/01/2026"}`})
	if ok.Code != http.StatusOK || wr.Code != http.StatusOK {
		t.Fatalf("บันทึกผลตรวจ: %d %d %s", ok.Code, wr.Code, wr.Body.String())
	}
	if v := payload[domain.Verification](t, wr); v.VerifiedBy != "break-glass" || v.VerifiedAt.IsZero() {
		t.Fatalf("ต้องบันทึกผู้ตรวจ/เวลา: %+v", v)
	}
	ask(t, e, tk, "สวัสดี")
	w = do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/verifications/stats?office_id=demo", console: consoleToken})
	st := payload[map[string]any](t, w)
	if st["correct"] != 1.0 || st["wrong"] != 1.0 || st["pending"] != 1.0 || st["percent_correct"] != 50.0 || !strings.Contains(fmt.Sprint(st["basis"]), "ยังไม่ตรวจ 1") {
		t.Fatalf("%%ถูกต้องมาคู่กับจำนวนยังไม่ตรวจ: %+v", st)
	}
	// ประวัติกรองตามสถานะตรวจได้ (ห้องที่มีคำตอบผิด = 1 ห้อง)
	w = do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/conversations?office_id=demo&verification=wrong", console: consoleToken})
	if cv := payload[struct {
		Total int64 `json:"total"`
	}](t, w); cv.Total != 1 {
		t.Fatalf("กรองห้องที่ตอบผิดต้องได้ 1 ห้อง: %s", w.Body.String())
	}
	// rollup นับผลตรวจด้วย
	rs, _ := e.rollups.List(context.Background(), "demo", "K11S", "", "")
	if len(rs) == 0 || rs[0].Correct != 1 || rs[0].Wrong != 1 {
		t.Fatalf("rollup ต้องนับถูก/ผิด: %+v", rs)
	}
}

// K5 · P-13: ลบตามคำขอ → ค้นไม่พบ · มีบันทึกผู้ลบ/เหตุผล · เว็บอื่นไม่โดน
func TestAdmin_DeletionRequest(t *testing.T) {
	o := demoOffice()
	o.Services[1].Enabled = true
	e := newEnv(t, o)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	tkPG := ticketFor(t, e.r, demoKey, "PG99", secretPG99, adminUser())
	ask(t, e, tk, "ถอนค้าง")
	b, _ := json.Marshal(map[string]string{"text": "ถอนค้าง"})
	chatReq(t, e.r, demoKey, "PG99", tkPG, string(b))

	w := do(t, e.r, req{method: http.MethodPost, path: "/api/ai/admin/deletion-requests", console: consoleToken,
		body: `{"office_id":"demo","service_id":"K11S","user_id":"emp_1","reason":"คำขอ PDPA #12","approved_by":"หัวหน้า"}`})
	d := payload[domain.DeletionRequest](t, w)
	if w.Code != http.StatusOK || d.Status != "done" || d.Result.Messages != 2 || d.RequestedBy != "break-glass" || d.ApprovedBy != "หัวหน้า" {
		t.Fatalf("ลบ: %d %s", w.Code, w.Body.String())
	}
	left, _, _ := e.msgs.Search(context.Background(), port.MessageFilter{OfficeID: "demo", ServiceID: "K11S"})
	other, _, _ := e.msgs.Search(context.Background(), port.MessageFilter{OfficeID: "demo", ServiceID: "PG99"})
	if len(left) != 0 || len(other) != 2 {
		t.Fatalf("ต้องลบเฉพาะขอบเขต: เหลือ K11S %d · PG99 %d", len(left), len(other))
	}
	if rows := accessRows(e); rows[0].Action != "delete" {
		t.Fatalf("การลบต้องมี access_log: %+v", rows[0])
	}
	// ขอบเขตกว้างเกิน/ไม่มีเหตุผล → ไม่ลบ
	for _, body := range []string{`{"office_id":"demo","reason":"x"}`, `{"office_id":"demo","service_id":"K11S"}`} {
		if w := do(t, e.r, req{method: http.MethodPost, path: "/api/ai/admin/deletion-requests", console: consoleToken, body: body}); w.Code != http.StatusBadRequest {
			t.Fatalf("%s ต้องไม่ผ่าน: %d", body, w.Code)
		}
	}
}

// AC-26: ต้นทุนต่อเว็บต่อเดือนด้วยเรต ณ ตอนบันทึก
func TestAdmin_QuotasAndCost(t *testing.T) {
	e := newEnv(t)
	tk := ticketFor(t, e.r, demoKey, "K11S", secretK11S, adminUser())
	ask(t, e, tk, "ถอนค้าง")
	w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/admin/quotas", console: consoleToken})
	var res struct {
		Data []domain.QuotaStatus `json:"data"`
	}
	res = payload[struct {
		Data []domain.QuotaStatus `json:"data"`
	}](t, w)
	var k domain.QuotaStatus
	for _, q := range res.Data {
		if q.ServiceID == "K11S" {
			k = q
		}
	}
	// fake LLM: รอบ 1 = 1000+50 · รอบ 2 = 1200+80 → 2,330 token · ราคา sonnet-5 $2/$10 ต่อ MTok
	wantCost := (2200*2.0 + 130*10.0) / 1_000_000
	if k.UsedTokens != 2330 || k.UsedQuestions != 1 || k.Limit != 1_000_000 || fmt.Sprintf("%.6f", k.CostAmount) != fmt.Sprintf("%.6f", wantCost) {
		t.Fatalf("โควตา/ต้นทุนผิด: %+v (อยากได้ cost %.6f)", k, wantCost)
	}
	m := lastAssistant(t, e)
	if m.Cost.RateSnapshot.InputPerMTok != 2.0 || m.Cost.Currency != "USD" {
		t.Fatalf("ต้องเก็บเรต ณ ตอนบันทึก: %+v", m.Cost)
	}
}

// NG-21: รายการ tool/สิทธิ์ดูได้แต่แก้จากคอนโซลไม่ได้
func TestAdmin_ConnectorsReadOnly(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodGet, path: "/api/ai/admin/connectors", console: consoleToken})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "withdraw_pending_by_status") || !strings.Contains(w.Body.String(), "BQ-03") {
		t.Fatalf("connectors: %d %s", w.Code, w.Body.String())
	}
	for _, m := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		if w := do(t, r, req{method: m, path: "/api/ai/admin/connectors", console: consoleToken, body: `{}`}); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s /connectors ต้องไม่มี: %d", m, w.Code)
		}
	}
}

// settings แก้ได้ แต่ค่าที่ทำให้ระบบพังถูกปรับกลับเป็นค่าเริ่มต้น
func TestAdmin_SettingsNormalized(t *testing.T) {
	r := newRouter(t)
	w := do(t, r, req{method: http.MethodPatch, path: "/api/ai/admin/settings", console: consoleToken,
		body: `{"max_concurrent":0,"support_message":"ติดต่อทีม support ห้อง X","ticket_ttl_min":9999}`})
	st := payload[domain.Settings](t, w)
	if w.Code != http.StatusOK || st.MaxConcurrent != 3 || st.TicketTTLMin != 30 || st.SupportMessage != "ติดต่อทีม support ห้อง X" || st.UpdatedBy != "break-glass" {
		t.Fatalf("settings: %d %+v", w.Code, st)
	}
}

// operator/viewer ถูกจำกัดสิทธิ์ของงานใหม่ตามค่าเริ่มต้น (least privilege)
func TestAdmin_NewPermissionsGated(t *testing.T) {
	r := newRouter(t)
	viewer := mintJWT(t, domain.RoleViewer)
	operatorTok := mintJWT(t, domain.RoleOperator)
	cases := []struct {
		tok    string
		method string
		path   string
		want   int
	}{
		{viewer, http.MethodGet, "/api/ai/admin/conversations", http.StatusForbidden},
		{viewer, http.MethodGet, "/api/ai/admin/quotas", http.StatusOK},
		{operatorTok, http.MethodGet, "/api/ai/admin/conversations", http.StatusOK},
		{operatorTok, http.MethodPost, "/api/ai/admin/offices/demo/services/K11S/secret/rotate", http.StatusForbidden},
		{operatorTok, http.MethodPost, "/api/ai/admin/deletion-requests", http.StatusForbidden},
		{operatorTok, http.MethodPatch, "/api/ai/admin/settings", http.StatusForbidden},
	}
	for _, c := range cases {
		if w := do(t, r, req{method: c.method, path: c.path, console: c.tok, body: `{}`}); w.Code != c.want {
			t.Errorf("%s %s = %d อยากได้ %d", c.method, c.path, w.Code, c.want)
		}
	}
}
