package service

import (
	"strings"
	"testing"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ปุ่มถามต่อ: แทน {input.x} ด้วยค่าจริง · อ้าง input ที่ไม่ได้ส่ง = ข้าม · ไม่ซ้ำ · ไม่เกิน 3
func TestAddSuggestions(t *testing.T) {
	tool := &connector.Tool{FollowUps: []connector.FollowUp{
		{Label: "รายการถอนของ {input.username}", Ask: "รายการถอนของยูส {input.username}"},
		{Label: "ฝากวันที่ {input.date}"}, // ไม่ได้ส่ง date → ข้าม
		{Label: "ยอดฝากทั้งหมด"},
	}}
	ans := &domain.ChatMessage{}
	addSuggestions(ans, tool, map[string]any{"username": "abc123"})
	addSuggestions(ans, tool, map[string]any{"username": "abc123"}) // tool เดิมซ้ำ = ไม่เพิ่ม
	if len(ans.Suggestions) != 2 {
		t.Fatalf("ได้ %+v", ans.Suggestions)
	}
	if ans.Suggestions[0].Label != "รายการถอนของ abc123" || ans.Suggestions[0].Ask != "รายการถอนของยูส abc123" {
		t.Fatalf("แทนค่าผิด: %+v", ans.Suggestions[0])
	}
	if ans.Suggestions[1].Ask != "ยอดฝากทั้งหมด" {
		t.Fatalf("ไม่มี ask ต้องใช้ label: %+v", ans.Suggestions[1])
	}
	for i := 0; i < 5; i++ {
		addSuggestions(ans, &connector.Tool{FollowUps: []connector.FollowUp{{Label: "ข้อ " + string(rune('ก'+i))}}}, nil)
	}
	if len(ans.Suggestions) != chatMaxSuggestions {
		t.Fatalf("ต้องไม่เกิน %d ได้ %d", chatMaxSuggestions, len(ans.Suggestions))
	}
}

// การ์ด/ปุ่มของคำตอบเก่าเป็นหมายเหตุของระบบฝั่งคำถาม — ข้อความของผู้ช่วยต้องไม่มี "[แสดงการ์ด" ให้โมเดลลอก
func TestHistoryMessages_NoCardNoteInAssistantText(t *testing.T) {
	h := []domain.ChatMessage{
		{Role: "user", Text: "ยูส a ฝากเท่าไหร่"},
		{Role: "assistant", Text: "ฝากสำเร็จ 1 รายการครับ", Cards: []domain.Card{{Title: "รายการฝากของสมาชิก"}}, Actions: []domain.ChatAction{{Label: "ดูโปรโมชั่น"}}},
		{Role: "user", Text: "แล้วถอนล่ะ"},
	}
	msgs := historyMessages(h)
	if len(msgs) != 2 {
		t.Fatalf("ได้ %d ข้อความ", len(msgs))
	}
	if strings.Contains(msgs[1].Text, "[") {
		t.Fatalf("ข้อความผู้ช่วยมีหมายเหตุของระบบ: %q", msgs[1].Text)
	}
	if !strings.Contains(msgs[0].Text, "การ์ด รายการฝากของสมาชิก") || !strings.Contains(msgs[0].Text, "ปุ่ม ดูโปรโมชั่น") {
		t.Fatalf("คำถามต้องมีหมายเหตุว่าแสดงอะไรไป: %q", msgs[0].Text)
	}
}

func speedConn() *connector.Connector {
	c := &connector.Connector{Tools: []*connector.Tool{
		{Name: "deposit_find", Keywords: []string{"ฝาก"}, Answer: "รายการฝากของ {input.username}"},
		{Name: "withdraw_find", Keywords: []string{"ถอน"}, Chain: true},
		{Name: "member_lookup", Keywords: []string{"สมาชิก"}, Answer: "llm"},
		{Name: "bills", Card: connector.CardSpec{Title: "บิล"}}, // ไม่มี keyword = ส่งเสมอ
	}}
	c.Reindex()
	return c
}

func names(ts []*connector.Tool) string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return strings.Join(out, ",")
}

// คัด tool: keyword ในคำถาม · tool ที่เพิ่งตอบ (ถามต่อ) · ไม่มี keyword ส่งเสมอ · ไม่เข้าเลย = ส่งทั้งหมด
func TestRelevantTools(t *testing.T) {
	c := speedConn()
	if got := names(relevantTools(c, "ยูส a ฝากเท่าไหร่", nil)); got != "deposit_find,bills" {
		t.Fatalf("ฝาก → %s", got)
	}
	h := []domain.ChatMessage{{Role: "assistant", Cards: []domain.Card{{Tool: "withdraw_find"}}}}
	if got := names(relevantTools(c, "แล้วฝากล่ะ", h)); got != "deposit_find,withdraw_find,bills" {
		t.Fatalf("ถามต่อ → %s", got)
	}
	if got := names(relevantTools(c, "สวัสดี", nil)); got != "deposit_find,withdraw_find,member_lookup,bills" {
		t.Fatalf("ไม่เข้าเลยต้องส่งทั้งหมด → %s", got)
	}
}

// ประโยคสำเร็จรูป: ข้อมูลล้วน = ไม่ต้องเรียก LLM · tool ที่ตั้ง answer: llm / ผิดพลาด = ให้ LLM เขียน
func TestTemplatedReply(t *testing.T) {
	c := speedConn()
	uses := []port.ToolUse{{Name: "deposit_find", Input: map[string]any{"username": "abc"}}, {Name: "show_button"}}
	got, ok := templatedReply(c, uses, []domain.Card{{Tool: "deposit_find", Kind: "ok", Title: "รายการฝากของสมาชิก"}})
	if !ok || got != "รายการฝากของ abc" {
		t.Fatalf("ได้ %q %v", got, ok)
	}
	got, ok = templatedReply(c, uses, []domain.Card{{Tool: "deposit_find", Kind: "not_found", Note: "ไม่พบรายการฝาก"}})
	if !ok || got != "ไม่พบรายการฝาก" {
		t.Fatalf("ไม่พบ → %q %v", got, ok)
	}
	if _, ok := templatedReply(c, uses, []domain.Card{{Tool: "deposit_find", Kind: "error"}}); ok {
		t.Fatal("ผิดพลาดต้องให้ LLM อธิบาย")
	}
	if _, ok := templatedReply(c, []port.ToolUse{{Name: "member_lookup"}}, []domain.Card{{Tool: "member_lookup", Kind: "ok"}}); ok {
		t.Fatal("answer: llm ต้องให้ LLM เขียน")
	}
	if _, ok := templatedReply(c, []port.ToolUse{{Name: "lookup_menu"}}, nil); ok {
		t.Fatal("ไม่มี tool ข้อมูลต้องให้ LLM เขียน")
	}
	if anyChain(c, uses) || !anyChain(c, []port.ToolUse{{Name: "withdraw_find"}}) {
		t.Fatal("chain ผิด")
	}
}

// ปุ่มตั้งต้น: กรองตามสถานะผู้ใช้ · ไม่เอาปุ่มที่ถามซ้ำกับคำถามนี้ · ไม่เกิน 3
func TestAddHostSuggestions(t *testing.T) {
	c := &connector.Connector{Host: connector.HostConfig{FollowUps: []connector.HostFollowUp{
		{Label: "สมัครยังไง", When: "guest"},
		{Label: "เครดิตคงเหลือ", When: "member"},
		{Label: "มีโปรอะไรบ้าง"},
		{Label: "เว็บทำอะไรได้บ้าง"},
		{Label: "ติดต่อแอดมิน"},
	}}}
	ans := &domain.ChatMessage{}
	addHostSuggestions(ans, c, domain.ChatTicket{Guest: true}, "มีโปรอะไรบ้างคะ")
	var got []string
	for _, s := range ans.Suggestions {
		got = append(got, s.Label)
	}
	if strings.Join(got, ",") != "สมัครยังไง,เว็บทำอะไรได้บ้าง,ติดต่อแอดมิน" {
		t.Fatalf("ได้ %v", got)
	}
}
