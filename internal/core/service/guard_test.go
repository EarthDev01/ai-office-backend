package service

import (
	"testing"

	"ai-office-backend/internal/core/domain"
)

func guardAll(g *OutputGuard, chunks ...string) string {
	out := ""
	for _, c := range chunks {
		out += g.Push(c)
	}
	return out + g.Flush()
}

func TestGuard_MasksCardValuesAcrossChunks(t *testing.T) {
	cards := []domain.Card{{Kind: "ok", Fields: []domain.CardField{{Label: "ชื่อ", Display: "สมชาย ใจดี"}, {Label: "ยอด", Display: "12,500"}}}}
	g := NewOutputGuard(cards, nil, "ดูยูส abc", nil)
	got := guardAll(g, "ชื่อ สมช", "าย ใจดี ยอด 12,5", "00 บาท")
	want := "ชื่อ (ดูในการ์ด) ยอด (ดูในการ์ด) บาท"
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestGuard_Exceptions(t *testing.T) {
	cards := []domain.Card{{Kind: "ok", Fields: []domain.CardField{{Label: "สถานะ", Display: "ยืนยันแล้ว"}}}}
	g := NewOutputGuard(cards, nil, "ยูส user99 ฝากไป 300 ไหม", []string{"K11S", "ยืนยันแล้ว"})
	got := guardAll(g, "1. เว็บ K11S\n2. ยูส user99 สถานะ ยืนยันแล้ว ยอด 300 ไม่ใช่ 450\n")
	want := "1. เว็บ K11S\n2. ยูส user99 สถานะ ยืนยันแล้ว ยอด 300 ไม่ใช่ (ดูในการ์ด)\n"
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestGuard_NoDataCardsAllowsSmallNumbers(t *testing.T) {
	g := NewOutputGuard(nil, nil, "เมนูอยู่ไหน", nil)
	got := guardAll(g, "ไปที่ขั้นที่ 3 แล้วกด ส่วนเลข 12345 ไม่ควรมี")
	want := "ไปที่ขั้นที่ 3 แล้วกด ส่วนเลข (ดูในการ์ด) ไม่ควรมี"
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}
