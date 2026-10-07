package service

import (
	"testing"

	"ai-office-backend/internal/core/domain"
)

func exchange(q, category, status string) []domain.ChatMessage {
	return []domain.ChatMessage{
		{Role: "user", Text: q},
		{Role: "assistant", Text: "…", Category: category, Status: status},
	}
}

// B-13: ถามซ้ำเรื่องที่เพิ่งตอบไม่ได้ / ขอคุยกับคนซ้ำ → หยุดวน · ถามซ้ำเรื่องที่ตอบได้ → ถามใหม่ตามปกติ
func TestRepeatOfUnanswered(t *testing.T) {
	cases := []struct {
		name    string
		history []domain.ChatMessage
		text    string
		want    bool
	}{
		{"ขอคุยกับคนซ้ำ", exchange("ขอคุยกับคน", "human_handoff", "ok"), "ขอคุยกับคน", true},
		{"ขอคุยกับคนซ้ำ + ครับ", exchange("ขอคุยกับคน", "human_handoff", "ok"), "ขอคุยกับคนครับ", true},
		{"ขอคุยกับคนซ้ำ + ช่องว่าง", exchange("ขอคุยกับ คน", "human_handoff", "ok"), " ขอคุยกับคน ", true},
		{"ขอคุยกับคนซ้ำ + ?", exchange("ขอคุยกับคน", "human_handoff", "ok"), "ขอคุยกับคน??", true},
		{"ขอคุยกับคนซ้ำ + ค่ะ!", exchange("ขอคุยกับคนค่ะ", "human_handoff", "ok"), "ขอคุยกับคน ค่ะ!", true},
		{"ถามเทคนิคซ้ำ", exchange("database ใช้อะไร", "technical", "ok"), "Database ใช้อะไร", true},
		{"ไม่มีเครื่องมือซ้ำ", exchange("ยอดเล่นเกมรายค่าย", "no_tool", "ok"), "ยอดเล่นเกมรายค่าย", true},
		{"นอกขอบเขตซ้ำ", exchange("พยากรณ์อากาศ", "out_of_scope", "ok"), "พยากรณ์อากาศครับ", true},
		{"ขอทำรายการซ้ำ", exchange("เติมเครดิตให้ยูส a1", "action_request", "ok"), "เติมเครดิตให้ยูส a1", true},
		{"ขอข้อมูลเว็บอื่นซ้ำ", exchange("ยอดฝากเว็บ PG99", "cross_service", "ok"), "ยอดฝากเว็บ pg99", true},
		{"คำตอบล้มแล้วถามซ้ำ", exchange("ยอดฝากวันนี้", "data", "error"), "ยอดฝากวันนี้", true},
		{"ลงท้ายหน่อยนะ", exchange("ขอคุยกับคน", "human_handoff", "ok"), "ขอคุยกับคนหน่อยนะ", true},

		{"ถามยอดซ้ำที่ตอบได้ ต้องได้ค่าใหม่", exchange("ยอดฝากวันนี้", "data", "ok"), "ยอดฝากวันนี้", false},
		{"ถามเมนูซ้ำที่ตอบได้", exchange("เมนูถอนอยู่ไหน", "guide", "ok"), "เมนูถอนอยู่ไหน", false},
		{"ทักทายซ้ำ", exchange("สวัสดี", "greeting", "ok"), "สวัสดี", false},
		{"ปฏิเสธแล้วถามเรื่องอื่น", exchange("ขอคุยกับคน", "human_handoff", "ok"), "ยอดฝากวันนี้", false},
		{"ไม่มีประวัติ", nil, "ขอคุยกับคน", false},
		{"ประวัติมีแต่คำถามที่ยังไม่ได้ตอบ", []domain.ChatMessage{{Role: "user", Text: "ขอคุยกับคน"}}, "ขอคุยกับคน", false},
		{"ยกเลิกกลางทางไม่นับ", exchange("ยอดฝากวันนี้", "data", "aborted"), "ยอดฝากวันนี้", false},
		{"เทียบเฉพาะคู่ล่าสุด", append(exchange("ขอคุยกับคน", "human_handoff", "ok"), exchange("ยอดฝากวันนี้", "data", "ok")...), "ขอคุยกับคน", false},
	}
	for _, c := range cases {
		if got := repeatOfUnanswered(c.history, c.text); got != c.want {
			t.Errorf("%s: ได้ %v ต้องการ %v", c.name, got, c.want)
		}
	}
}

// B-3: คำตอบที่ตอบไม่ได้/ขอคุยกับคน ต้องจบด้วยช่องทาง support เสมอ แม้ LLM ลืมใส่ · ไม่ใส่ซ้ำถ้ามีแล้ว
func TestEnsureSupport(t *testing.T) {
	const support = "ติดต่อ support ที่ไลน์ @help"
	cases := []struct {
		category, text string
		appended       bool
	}{
		{"no_tool", "ยังตอบเรื่องนี้ไม่ได้ครับ", true},
		{"technical", "เรื่องนี้ตอบไม่ได้ครับ", true},
		{"out_of_scope", "นอกขอบเขตครับ", true},
		{"human_handoff", "ผมเป็นผู้ช่วยอัตโนมัติครับ", true},
		{"human_handoff", "ผมเป็นผู้ช่วยอัตโนมัติ " + support, false},
		{"data", "ดูในการ์ดครับ", false},
		{"greeting", "สวัสดีครับ", false},
		{"action_request", "ไปที่เมนูเติมเครดิตครับ", false}, // มีเมนูให้ไปทำเองแล้ว
	}
	s := &ChatService{}
	for _, c := range cases {
		ans := &domain.ChatMessage{Category: c.category, Text: c.text}
		var emitted []string
		err := s.ensureSupport(ans, support, func(_ string, data any) error {
			emitted = append(emitted, data.(map[string]any)["text"].(string))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(emitted) == 1; got != c.appended {
			t.Errorf("%s %q: ต่อท้าย=%v ต้องการ %v", c.category, c.text, got, c.appended)
		}
		if c.appended && ans.Text != c.text+"\n\n"+support {
			t.Errorf("%s: ข้อความที่บันทึก = %q", c.category, ans.Text)
		}
	}
}
