package service

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"ai-office-backend/internal/core/domain"
)

const guardMask = "(ดูในการ์ด)"

// OutputGuard ลบค่าที่ LLM พิมพ์เองออกจากคำตอบ — ไม่พึ่ง prompt
//
//   - ค่าบนการ์ด (ชื่อ เบอร์ ยูสเซอร์ ฯลฯ) ที่ LLM พิมพ์ซ้ำ → แทนด้วย (ดูในการ์ด)
//   - ตัวเลขที่ยืนเดี่ยว ๆ → แทนเช่นกัน
//
// ยกเว้น: คำที่ผู้ใช้พิมพ์มาเอง · ชื่อเว็บ ชื่อเมนู ป้ายสถานะ (allow) · เลขข้อ "1." ต้นบรรทัด ·
// ตัวเลขที่ติดอยู่ในคำ (K11S) · ถ้าไม่มีการ์ดข้อมูลเลย เลข 1–2 หลักผ่านได้ (เช่น "ขั้นที่ 3")
//
// รับข้อความทีละ chunk แล้วกักท้ายไว้จนถึงช่องว่างถัดไป เพื่อไม่ให้ค่าที่ถูกตัดกลาง chunk หลุดไป
type OutputGuard struct {
	terms     []string // ค่าบนการ์ด เรียงยาว→สั้น
	userText  string
	allow     []string
	numbers   map[string]bool // ตัวเลขที่ผู้ใช้พิมพ์เอง หรืออยู่ในชื่อที่ยกเว้น (เทียบทั้งตัว)
	dataCards bool

	buf  string
	Hits int
}

var numberRun = regexp.MustCompile(`[0-9๐-๙][0-9๐-๙,.:/\-]*`)

// sensitive คือค่าดิบทุกตัวที่ขึ้นการ์ด (ที่ connector รวบรวมให้) — กันแม้ LLM พิมพ์ในรูปที่ต่างจากที่โชว์
func NewOutputGuard(cards []domain.Card, sensitive []string, userText string, allow []string) *OutputGuard {
	g := &OutputGuard{userText: strings.ToLower(userText), numbers: map[string]bool{}}
	for _, a := range allow {
		if a = strings.TrimSpace(a); a != "" {
			g.allow = append(g.allow, strings.ToLower(a))
		}
	}
	for _, src := range append([]string{userText}, allow...) {
		for _, n := range numberRun.FindAllString(src, -1) {
			g.numbers[strings.TrimRight(n, ",.:/-")] = true
		}
	}
	seen := map[string]bool{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if utf8.RuneCountInString(v) < 2 || v == "-" || seen[v] || g.allowed(v) {
			return
		}
		seen[v] = true
		g.terms = append(g.terms, v)
	}
	addValue := func(v string) {
		add(v)
		// ค่าหลายคำ (ชื่อ-นามสกุล) อาจถูกตัดตรงช่องว่าง — กันทีละคำด้วย
		for _, w := range strings.Fields(v) {
			if utf8.RuneCountInString(w) >= 3 {
				add(w)
			}
		}
	}
	for _, c := range cards {
		if c.Kind != "ok" {
			continue // การ์ดแดง/ไม่พบ/อ้างอิงเมนู ไม่มีค่าของระบบที่ต้องกัน
		}
		g.dataCards = true
		for _, f := range c.Fields {
			addValue(f.Display)
		}
		if c.Table != nil {
			for _, row := range c.Table.Rows {
				for _, cell := range row {
					addValue(cell.Display)
				}
			}
		}
	}
	for _, v := range sensitive {
		addValue(v)
	}
	sort.Slice(g.terms, func(i, j int) bool { return len(g.terms[i]) > len(g.terms[j]) })
	return g
}

// allowed — คำที่ผู้ใช้พิมพ์เอง หรืออยู่ในรายการยกเว้น
func (g *OutputGuard) allowed(v string) bool {
	lv := strings.ToLower(v)
	if strings.Contains(g.userText, lv) {
		return true
	}
	for _, a := range g.allow {
		if strings.Contains(a, lv) {
			return true
		}
	}
	return false
}

// Push รับ chunk แล้วคืนส่วนที่ปล่อยได้ (อาจว่าง)
func (g *OutputGuard) Push(chunk string) string {
	g.buf += chunk
	cut := strings.LastIndexFunc(g.buf, unicode.IsSpace)
	if cut < 0 {
		if utf8.RuneCountInString(g.buf) < 200 {
			return ""
		}
		cut = len(g.buf) - 1 // ข้อความยาวไม่มีช่องว่าง — ปล่อยเกือบทั้งหมด
	}
	out := g.buf[:cut+1]
	g.buf = g.buf[cut+1:]
	return g.filter(out)
}

// Flush ปล่อยที่เหลือตอนจบ
func (g *OutputGuard) Flush() string {
	out := g.filter(g.buf)
	g.buf = ""
	return out
}

func (g *OutputGuard) filter(s string) string {
	for _, t := range g.terms {
		if n := strings.Count(s, t); n > 0 {
			g.Hits += n
			s = strings.ReplaceAll(s, t, guardMask)
		}
	}

	idx := numberRun.FindAllStringIndex(s, -1)
	if len(idx) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range idx {
		start, end := m[0], m[1]
		tok := strings.TrimRight(s[start:end], ",.:/-")
		end = start + len(tok)
		b.WriteString(s[last:start])
		last = end
		if g.keepNumber(s, start, end, tok) {
			b.WriteString(tok)
			continue
		}
		g.Hits++
		b.WriteString(guardMask)
	}
	b.WriteString(s[last:])
	return b.String()
}

func (g *OutputGuard) keepNumber(s string, start, end int, tok string) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:start])
	after, _ := utf8.DecodeRuneInString(s[end:])
	// ติดอยู่ในคำ เช่น K11S, PG99
	if isASCIILetter(before) || isASCIILetter(after) {
		return true
	}
	// เลขข้อต้นบรรทัด "1." / "2)"
	lineStart := start == 0 || before == '\n'
	if lineStart && (after == '.' || after == ')') && len(tok) <= 2 {
		return true
	}
	if g.numbers[tok] {
		return true
	}
	return !g.dataCards && len(tok) <= 2
}

func isASCIILetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}
