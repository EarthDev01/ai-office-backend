package test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

// openSessionWith ขอตั๋วด้วย token หลังบ้านที่กำหนดเอง
func openSessionWith(t *testing.T, base, officeToken string, perms ...string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"permissions": perms})
	res := post(t, base+"/api/ai/widget/service/K11S/browser-session", officeToken, string(body))
	defer res.Body.Close()
	var out struct {
		Payload struct {
			Ticket string `json:"ticket"`
		} `json:"payload"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 || out.Payload.Ticket == "" {
		t.Fatalf("browser-session = %d", res.StatusCode)
	}
	return out.Payload.Ticket
}

func get(t *testing.T, url, bearer string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Origin", officeOrigin)
	req.Header.Set("Authorization", "Bearer "+bearer)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func conversationOf(t *testing.T, events []sseEvent) string {
	t.Helper()
	var st struct {
		ConversationID string `json:"conversation_id"`
	}
	_ = json.Unmarshal(find(events, "status")[0].data, &st)
	if st.ConversationID == "" {
		t.Fatal("ไม่มี conversation_id")
	}
	return st.ConversationID
}

func TestHistory_OwnSessionOnly(t *testing.T) {
	llm := &fakeLLM{
		uses:  []port.ToolUse{{ID: "t1", Name: "member_lookup", Input: map[string]any{"username": "somchai01"}}},
		reply: []string{"ดูในการ์ดครับ"},
	}
	srv := chatServer(t, llm)
	exp := time.Now().Add(time.Hour).Unix()
	realToken := officeJWTExp("adm_ploy", map[string]bool{"K11S": true}, nil, exp)
	ticket := openSessionWith(t, srv.URL, realToken, "MEMBER_INFO")
	conv := conversationOf(t, runChat(t, srv.URL, ticket, "เช็คยูส somchai01", func(service.FetchCommand) string { return memberBody }))
	hist := srv.URL + "/api/ai/widget/service/K11S/conversations"

	// เจ้าของ (token เดิม แม้ขอตั๋วใบใหม่) เห็นห้อง + ข้อความ + การ์ด
	again := openSessionWith(t, srv.URL, realToken, "MEMBER_INFO")
	code, list := get(t, hist+"?days=7", again)
	rooms, _ := list["payload"].([]any)
	if code != 200 || len(rooms) != 1 || rooms[0].(map[string]any)["id"] != conv {
		t.Fatalf("เจ้าของต้องเห็นห้องตัวเอง: %d %v", code, list)
	}
	code, room := get(t, hist+"/"+conv, again)
	raw, _ := json.Marshal(room)
	if code != 200 || !strings.Contains(string(raw), "เช็คยูส somchai01") || !strings.Contains(string(raw), "สมชาย ใจดี") {
		t.Fatalf("เจ้าของต้องเปิดห้องเห็นคำถามและการ์ด: %d %s", code, raw)
	}
	if strings.Contains(string(raw), "tool_calls") || strings.Contains(string(raw), "session") {
		t.Fatalf("ประวัติฝั่ง widget ต้องไม่มี tool_calls/ลายนิ้วมือ: %s", raw)
	}

	// AC-33: JWT ปลอมที่อ้าง admin เดิม (ลายเซ็นไม่ถูกตรวจ แต่ token คนละค่า) → ไม่เห็นห้อง เปิด/ต่อห้องไม่ได้
	forged := openSessionWith(t, srv.URL, officeJWTExp("adm_ploy", map[string]bool{"K11S": true}, nil, exp+60), "MEMBER_INFO")
	if code, list := get(t, hist, forged); code != 200 || len(list["payload"].([]any)) != 0 {
		t.Fatalf("JWT ปลอมเห็นรายการห้อง: %d %v", code, list)
	}
	if code, _ := get(t, hist+"/"+conv, forged); code != http.StatusNotFound {
		t.Fatalf("JWT ปลอมเปิดห้องได้: %d", code)
	}
	events := runChatIn(t, srv.URL, forged, conv, "ต่อห้องเดิม", func(service.FetchCommand) string { return memberBody })
	if errs := find(events, "error"); len(errs) != 1 || !strings.Contains(string(errs[0].data), "not_found") {
		t.Fatalf("JWT ปลอมต่อห้องเดิมได้: %v", events)
	}

	// แอดมินคนอื่น → ไม่เห็น
	other := openSessionWith(t, srv.URL, adminJWT("adm_other", "K11S"))
	if code, _ := get(t, hist+"/"+conv, other); code != http.StatusNotFound {
		t.Fatalf("แอดมินอื่นเปิดห้องได้: %d", code)
	}

	// ตั๋วของเว็บนี้ใช้ดูประวัติเว็บอื่นไม่ได้
	if code, _ := get(t, srv.URL+"/api/ai/widget/service/PG99/conversations", again); code != http.StatusForbidden {
		t.Fatalf("ตั๋ว K11S ดูประวัติ PG99 ได้: %d", code)
	}
	// ไม่มีตั๋ว / token หลังบ้านแทนตั๋ว → 401
	if code, _ := get(t, hist, realToken); code != http.StatusUnauthorized {
		t.Fatalf("token หลังบ้านใช้แทนตั๋วได้: %d", code)
	}
}
