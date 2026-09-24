package service

import (
	"strings"
	"testing"
)

func guardRun(allow, sensitive []string, chunks ...string) (string, int) {
	var out strings.Builder
	g := NewOutputGuard(allow, sensitive, func(s string) { out.WriteString(s) })
	for _, c := range chunks {
		g.Write(c)
	}
	g.Flush()
	return out.String(), g.Hits()
}

func TestGuard_MasksStandaloneNumbers(t *testing.T) {
	got, hits := guardRun(nil, nil, "ยอดฝากวันนี้ 1,284,500.00 บาท ", "เพิ่มขึ้น 18.0% จากเมื่อวาน")
	if strings.ContainsAny(got, "0123456789") || hits != 2 {
		t.Fatalf("ต้องไม่มีตัวเลขเหลือ: %q (hits=%d)", got, hits)
	}
}

// ตัวเลขถูกตัดกลาง chunk ต้องยังถูกจับได้
func TestGuard_NumberSplitAcrossChunks(t *testing.T) {
	got, _ := guardRun(nil, nil, "รวม 1,2", "84,5", "00 บาท")
	if strings.ContainsAny(got, "0123456789") {
		t.Fatalf("ตัวเลขข้าม chunk หลุด: %q", got)
	}
}

func TestGuard_KeepsAllowedAndWords(t *testing.T) {
	allow := []string{"K11S", "รายการสมาชิก VIP 30 ลำดับ"}
	got, hits := guardRun(allow, nil, "เว็บ K11S ดูที่เมนู รายการสมาชิก VIP 30 ลำดับ\n1. เปิดเมนู\n2) กดค้นหา · ใช้ 2FA ได้ · ยูส somchai99")
	for _, want := range []string{"K11S", "VIP 30 ลำดับ", "1. เปิดเมนู", "2) กดค้นหา", "2FA", "somchai99"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q ต้องไม่ถูกตัด: %q", want, got)
		}
	}
	if hits != 0 {
		t.Fatalf("ไม่ควรมี hit: %d", hits)
	}
}

// ค่าบนการ์ด (เช่นชื่อผู้ดำเนินการ) ที่โมเดลพิมพ์ซ้ำต้องถูกตัด
func TestGuard_MasksCardValues(t *testing.T) {
	got, hits := guardRun(nil, []string{"admin_somsak", "7"}, "รายการนี้ทำโดย admin_somsak ครับ")
	if strings.Contains(got, "admin_somsak") || hits != 1 {
		t.Fatalf("ค่าบนการ์ดต้องถูกตัด: %q", got)
	}
}

func TestGuard_ListMarkerOnlyAtLineStart(t *testing.T) {
	got, _ := guardRun(nil, nil, "มี 3. รายการ")
	if strings.Contains(got, "3") {
		t.Fatalf("เลขกลางประโยคไม่ใช่เลขข้อ: %q", got)
	}
}
