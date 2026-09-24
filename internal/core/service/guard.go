package service

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GuardMask = ข้อความที่ใส่แทนค่าที่โมเดลพิมพ์เอง
const GuardMask = "(ดูในการ์ด)"

// OutputGuard ตัดตัวเลข/ค่าที่อยู่บนการ์ดที่โมเดลพิมพ์เองออกจากข้อความ (B-4 · AC-14 · R-17)
//
// ██ ไม่พึ่ง prompt — ต่อให้โมเดลพิมพ์ยอดเงินออกมา ข้อความที่ถึงผู้ใช้ก็ไม่มีตัวเลขนั้น
//
// ทำงานแบบ stream: ถือท้ายข้อความที่อาจเป็นตัวเลข/ชื่อที่ยังพิมพ์ไม่จบไว้ก่อน แล้วค่อยปล่อย
//
// ยกเว้น (ไม่ตัด):
//   - คำใน allow (ชื่อเว็บ · ชื่อเมนู/แท็บ · ป้ายสถานะ · คำที่ผู้ใช้พิมพ์มาเอง)
//   - เลขข้อของรายการขั้นตอน "1." "2)" ต้นบรรทัด
//   - ตัวเลขที่ติดอยู่ในคำ (เช่น K11S, somchai99 — ไม่ใช่ตัวเลขเดี่ยว)
type OutputGuard struct {
	allow     []string
	sensitive []string
	pending   strings.Builder
	out       func(string)
	hits      int
	emitted   strings.Builder
}

func NewOutputGuard(allow, sensitive []string, out func(string)) *OutputGuard {
	g := &OutputGuard{out: out}
	g.allow = uniqLongFirst(allow, 2)
	g.sensitive = uniqLongFirst(sensitive, 1)
	return g
}

func uniqLongFirst(in []string, minRunes int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if utf8.RuneCountInString(s) < minRunes || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func (g *OutputGuard) Hits() int { return g.hits }

// Text = ข้อความทั้งหมดที่ปล่อยออกไปแล้ว (หลังตัด) — ใช้บันทึกลง messages
func (g *OutputGuard) Text() string { return g.emitted.String() }

func (g *OutputGuard) Write(chunk string) {
	g.pending.WriteString(chunk)
	s := g.pending.String()
	cut := safeCut(s)
	if cut == 0 {
		return
	}
	head := s[:cut]
	g.pending.Reset()
	g.pending.WriteString(s[cut:])
	g.emit(g.Mask(head))
}

func (g *OutputGuard) Flush() {
	s := g.pending.String()
	g.pending.Reset()
	if s != "" {
		g.emit(g.Mask(s))
	}
}

func (g *OutputGuard) emit(s string) {
	if s == "" {
		return
	}
	g.emitted.WriteString(s)
	if g.out != nil {
		g.out(s)
	}
}

// safeCut = ตำแหน่งที่ปล่อยได้อย่างปลอดภัย: ถือท้ายที่เป็นตัวอักษร "เสี่ยง" (ASCII/ตัวเลข/เครื่องหมายในตัวเลข)
// ไว้จนกว่าจะเจอตัวคั่น — กันตัวเลขถูกตัดครึ่งระหว่าง chunk
func safeCut(s string) int {
	i := len(s)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if isRisky(r) {
			i -= size
			continue
		}
		break
	}
	// ถ้าท้ายเป็นคำไทยที่อาจเป็นส่วนหนึ่งของคำใน allow/sensitive ยาว ๆ ไม่ต้องถือ — Mask ทำงานต่อชิ้นอยู่แล้ว
	return i
}

func isRisky(r rune) bool {
	return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_.,:%/@+-#", r))
}

// Mask ตัดค่าที่ห้ามพิมพ์ออกจากข้อความ 1 ชิ้น
func (g *OutputGuard) Mask(s string) string {
	if s == "" {
		return s
	}
	// 1) ป้องกันคำที่อนุญาต (แทนด้วย marker ชั่วคราว)
	var protected []string
	protect := func(text, word string) string {
		if word == "" || !strings.Contains(text, word) {
			return text
		}
		protected = append(protected, word)
		return strings.ReplaceAll(text, word, marker(len(protected)-1))
	}
	for _, w := range g.allow {
		s = protect(s, w)
	}
	// 2) ค่าบนการ์ด (ยาวก่อน) — ยกเว้นค่าที่สั้นมากและเป็นตัวเลขล้วน (ข้อ 3 จัดการ)
	for _, v := range g.sensitive {
		if isAllDigitsPunct(v) && utf8.RuneCountInString(v) < 3 {
			continue
		}
		if strings.Contains(s, v) {
			g.hits += strings.Count(s, v)
			s = strings.ReplaceAll(s, v, GuardMask)
		}
	}
	// 3) ตัวเลขเดี่ยว
	s = g.maskNumbers(s)
	// 4) คืนคำที่ป้องกันไว้
	for i, w := range protected {
		s = strings.ReplaceAll(s, marker(i), w)
	}
	return s
}

func marker(i int) string { return "\x00" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + "\x00" }

func isAllDigitsPunct(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && !strings.ContainsRune(",.%-", r) {
			return false
		}
	}
	return s != ""
}

// maskNumbers แทนตัวเลขเดี่ยว (ไม่ติดตัวอักษรละติน) ด้วย mask · เว้นเลขข้อต้นบรรทัด
func (g *OutputGuard) maskNumbers(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i := 0; i < len(rs); {
		r := rs[i]
		if !unicode.IsDigit(r) {
			b.WriteRune(r)
			i++
			continue
		}
		// ติดตัวอักษรละติน/ขีดล่างด้านหน้า = เป็นส่วนของคำ (K11S, user_99) → ไม่ใช่ตัวเลขเดี่ยว
		if i > 0 && isWordRune(rs[i-1]) {
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || isWordRune(rs[j])) {
				j++
			}
			b.WriteString(string(rs[i:j]))
			i = j
			continue
		}
		j := i
		for j < len(rs) && (unicode.IsDigit(rs[j]) || ((rs[j] == ',' || rs[j] == '.' || rs[j] == ':' || rs[j] == '/') && j+1 < len(rs) && unicode.IsDigit(rs[j+1]))) {
			j++
		}
		if j < len(rs) && rs[j] == '%' {
			j++
		}
		// ตามด้วยตัวอักษรละติน = ส่วนของคำ (เช่น 2FA)
		if j < len(rs) && isWordRune(rs[j]) {
			for j < len(rs) && (unicode.IsDigit(rs[j]) || isWordRune(rs[j])) {
				j++
			}
			b.WriteString(string(rs[i:j]))
			i = j
			continue
		}
		num := string(rs[i:j])
		if isListMarker(rs, i, j) {
			b.WriteString(num)
		} else {
			g.hits++
			b.WriteString(GuardMask)
		}
		i = j
	}
	return b.String()
}

func isWordRune(r rune) bool {
	return r < 128 && (unicode.IsLetter(r) || r == '_')
}

// "1." / "2)" ที่อยู่ต้นบรรทัด (หลังช่องว่าง) และตามด้วยช่องว่าง = เลขข้อ ไม่ใช่ข้อมูล
func isListMarker(rs []rune, start, end int) bool {
	if end-start > 2 {
		return false
	}
	k := start - 1
	for k >= 0 && (rs[k] == ' ' || rs[k] == '\t') {
		k--
	}
	if k >= 0 && rs[k] != '\n' {
		return false
	}
	if end+1 >= len(rs) {
		return false
	}
	return (rs[end] == '.' || rs[end] == ')') && (rs[end+1] == ' ' || rs[end+1] == '\t')
}
