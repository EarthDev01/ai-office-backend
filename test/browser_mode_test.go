package test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-office-backend/internal/core/domain"
)

// โหมด browser (ทางที่ 1): หลังบ้านไม่ต้องแก้ — widget ขอตั๋วเอง แล้วยิง API เดิมของหลังบ้านให้ AI

const brwKey = "pk_brw"

func browserOffice() domain.Office {
	o := domain.NewOffice("brw", "หลังบ้านโหมด browser")
	o.PublicKey = brwKey
	o.Kind = "sample-browser"
	o.AllowedOrigins = []string{officeOrigin}
	k := domain.DefaultService("K11S", "เว็บ K11S")
	k.Enabled = true
	k.AllowAll = true
	o.Services = []domain.Service{k}
	return o
}

var fpA = strings.Repeat("a", 64)

func browserTicket(t *testing.T, r http.Handler, origin, fp string, user map[string]any) (int, string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"user": user, "token_fp": fp})
	w := do(t, r, req{method: http.MethodPost, path: basePath(brwKey, "K11S") + "/browser-session", origin: origin, body: string(b)})
	if w.Code != http.StatusOK {
		return w.Code, w.Body.String()
	}
	var res struct {
		Payload struct {
			Ticket string `json:"ticket"`
		} `json:"payload"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w.Code, res.Payload.Ticket
}

var brwUser = map[string]any{"id": "emp_1", "username": "adm_ploy", "permissions": []string{"W_VIEW"}}

func TestBrowser_SessionRules(t *testing.T) {
	e := newEnv(t, browserOffice())
	if code, _ := browserTicket(t, e.r, officeOrigin, fpA, brwUser); code != http.StatusOK {
		t.Fatalf("โดเมนที่ลงทะเบียน + ตัวตนครบ ต้องได้ตั๋ว: %d", code)
	}
	if code, _ := browserTicket(t, e.r, "", fpA, brwUser); code == http.StatusOK {
		t.Fatal("ไม่มี Origin ต้องไม่ได้ตั๋ว")
	}
	if code, _ := browserTicket(t, e.r, "http://evil.test", fpA, brwUser); code == http.StatusOK {
		t.Fatal("โดเมนอื่นต้องไม่ได้ตั๋ว")
	}
	if code, _ := browserTicket(t, e.r, officeOrigin, "short", brwUser); code != http.StatusBadRequest {
		t.Fatalf("ไม่มี token_fp ต้อง 400 ได้ %d", code)
	}
	// office โหมด host (demo) ขอตั๋วแบบ browser ไม่ได้
	e2 := newEnv(t, demoOffice())
	b, _ := json.Marshal(map[string]any{"user": brwUser, "token_fp": fpA})
	if w := do(t, e2.r, req{method: http.MethodPost, path: basePath(demoKey, "K11S") + "/browser-session", origin: officeOrigin, body: string(b)}); w.Code == http.StatusOK {
		t.Fatal("office โหมด host ต้องขอตั๋วแบบ browser ไม่ได้")
	}
}

// key "auto" = หา office จากโดเมน (snippet เดียวใช้ได้ทุกโดเมน)
func TestBrowser_AutoKeyFromOrigin(t *testing.T) {
	e := newEnv(t, browserOffice())
	w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/office/auto/page-config", origin: officeOrigin})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"mode":"browser"`) || !strings.Contains(w.Body.String(), `"identity"`) {
		t.Fatalf("page-config ของ auto ต้องบอก mode browser + identity: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, e.r, req{method: http.MethodGet, path: "/api/ai/office/auto/page-config", origin: "http://evil.test"}); w.Code != http.StatusForbidden {
		t.Fatalf("โดเมนที่ไม่ลงทะเบียนต้อง 403 ได้ %d", w.Code)
	}
}

// ถามจริง: backend ส่ง "fetch" ให้ widget → widget ยิงหลังบ้าน → ส่งผลกลับ → การ์ดจากข้อมูลนั้น
func TestBrowser_ChatRelaysThroughWidget(t *testing.T) {
	e := newEnv(t, browserOffice())
	_, tk := browserTicket(t, e.r, officeOrigin, fpA, brwUser)
	srv := httptest.NewServer(e.r)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "ถอนค้าง"})
	rq, _ := http.NewRequest(http.MethodPost, srv.URL+basePath(brwKey, "K11S")+"/chat", bytes.NewReader(body))
	rq.Header.Set("Authorization", "Bearer "+tk)
	rq.Header.Set("Origin", officeOrigin)
	rq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(rq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var fetches, cards []map[string]any
	var done bool
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	evType := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			evType = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			var data map[string]any
			_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data)
			switch evType {
			case "fetch":
				fetches = append(fetches, data)
				// จำลอง widget: ยิงหลังบ้านได้ผลแล้วส่งกลับ
				host := `{"code":0,"message":"ok","data":{"rows":[{"status":2,"count":7,"amount":142000},{"status":10,"count":2,"amount":18500}]}}`
				rb, _ := json.Marshal(map[string]any{"status": 200, "body": host})
				pr, _ := http.NewRequest(http.MethodPost, srv.URL+basePath(brwKey, "K11S")+"/chat/relay/"+fmt.Sprint(data["id"]), bytes.NewReader(rb))
				pr.Header.Set("Authorization", "Bearer "+tk)
				pr.Header.Set("Origin", officeOrigin)
				pr.Header.Set("Content-Type", "application/json")
				if r2, err := http.DefaultClient.Do(pr); err != nil || r2.StatusCode != http.StatusOK {
					t.Fatalf("ส่งผล relay ไม่ผ่าน: %v %v", err, r2)
				}
			case "card":
				cards = append(cards, data)
			case "done":
				done = true
			}
		}
	}
	if len(fetches) != 1 {
		t.Fatalf("ต้องมีคำขอให้ widget ยิง 1 ครั้ง: %+v", fetches)
	}
	f := fetches[0]
	if f["path"] != "/GetWithdrawList/K11S" || f["method"] != "GET" {
		t.Fatalf("คำขอต้องเป็น path ของ connector ที่ผูกเว็บของตั๋ว: %+v", f)
	}
	if !done || len(cards) != 1 || cards[0]["kind"] != "ok" {
		t.Fatalf("ต้องได้การ์ดจากข้อมูลที่ widget ส่งมา: done=%v %+v", done, cards)
	}
	if !strings.Contains(fmt.Sprint(cards[0]), "9 รายการ") {
		t.Fatalf("การ์ดต้องคำนวณจากผลที่ส่งมา: %+v", cards[0])
	}
	// backend ไม่เคยเรียก host เอง (ไม่มี scoped token / read)
	if e.host.readCalls != 0 || e.host.scopedCalls != 0 {
		t.Fatalf("โหมด browser backend ต้องไม่ยิงหลังบ้านเอง: read=%d scoped=%d", e.host.readCalls, e.host.scopedCalls)
	}
}

// ตัวตนในโหมด browser ไม่ได้ตรวจลายเซ็น — อ้างชื่อคนอื่น (คนละรอบล็อกอิน) ต้องเปิดประวัติของเขาไม่ได้
func TestBrowser_HistoryBoundToLoginSession(t *testing.T) {
	e := newEnv(t, browserOffice())
	_, tkA := browserTicket(t, e.r, officeOrigin, fpA, brwUser)
	evs := chatReq(t, e.r, brwKey, "K11S", tkA, `{"text":"สวัสดี"}`)
	if len(eventsOf(evs, "done")) != 1 {
		t.Fatalf("คุยได้: %+v", evs)
	}
	list := func(tk string) string {
		w := do(t, e.r, req{method: http.MethodGet, path: basePath(brwKey, "K11S") + "/conversations?days=7", origin: officeOrigin, token: tk})
		return w.Body.String()
	}
	if !strings.Contains(list(tkA), `"title":"สวัสดี"`) {
		t.Fatalf("เจ้าของต้องเห็นประวัติ: %s", list(tkA))
	}
	_, tkForged := browserTicket(t, e.r, officeOrigin, strings.Repeat("b", 64), brwUser)
	if strings.Contains(list(tkForged), "สวัสดี") {
		t.Fatalf("อ้างชื่อเดียวกันแต่คนละรอบล็อกอิน ต้องไม่เห็นประวัติ: %s", list(tkForged))
	}
}
