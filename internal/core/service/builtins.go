package service

import (
	"sort"
	"strings"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// tool กลางที่ทุก kind ได้ — เนื้อหามาจาก menus.yaml / statuses.yaml / host.yaml ของ connector เท่านั้น
// ห้าม hardcode คำตอบในโค้ด · ชื่อเมนูมาจาก connector ของ kind นั้น

var answerCategories = []string{
	"greeting",       // ทักทาย/ถามว่าช่วยอะไรได้
	"identity",       // ถามว่าเป็นคนหรือบอท
	"human_handoff",  // ขอคุยกับคน/support
	"technical",      // เรื่องเทคนิคระบบ error/database/server
	"out_of_scope",   // นอกขอบเขตงานหลังบ้าน
	"action_request", // ขอให้ทำรายการ/แก้ข้อมูล/เพิ่มลดเครดิต
	"cross_service",  // ขอข้อมูลของเว็บอื่น
	"no_tool",        // ถามข้อมูลที่ไม่มีเครื่องมือรองรับ
	"other",
}

// refusal categories = คำถามที่ผู้ช่วยต้องปฏิเสธหรือตอบไม่ได้
var refusalCategories = map[string]bool{
	"technical": true, "out_of_scope": true, "action_request": true, "cross_service": true, "no_tool": true,
}

func builtinTools(conn *connector.Connector) []port.ToolDef {
	tables := make([]any, 0, len(conn.Statuses.Tables))
	names := make([]string, 0, len(conn.Statuses.Tables))
	for k := range conn.Statuses.Tables {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		tables = append(tables, k)
	}
	cats := make([]any, len(answerCategories))
	for i, c := range answerCategories {
		cats[i] = c
	}
	return []port.ToolDef{
		{
			Name:        "answer_directly",
			Description: "ใช้เมื่อคำถามไม่ต้องดึงข้อมูลจากระบบ: ทักทาย, ถามว่าเป็นใคร/เป็นคนไหม, ขอคุยกับคน, เรื่องเทคนิคของระบบ, นอกขอบเขต, ขอให้ทำรายการหรือแก้ข้อมูล (ต้องปฏิเสธ), ขอข้อมูลเว็บอื่น, หรือถามสิ่งที่ไม่มีเครื่องมือรองรับ",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"category": map[string]any{"type": "string", "enum": cats, "description": "ประเภทของคำถาม"},
			}, "required": []string{"category"}},
		},
		{
			Name:        "lookup_menu",
			Description: "ค้นเมนู/แท็บ/วิธีทำในหลังบ้านนี้ (ชื่อเมนูจริงของหลังบ้านนี้) — ใช้ตอบว่าเมนูหรือรายงานอยู่ตรงไหน และวิธีทำรายการที่ผู้ใช้ต้องกดเอง",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "คำค้นสั้น ๆ เช่น เพิ่มเครดิต, รายการถอน, จัดการโปรโมชั่น", "maxLength": 80},
			}, "required": []string{"query"}},
		},
		{
			Name:        "explain_status",
			Description: "อธิบายความหมายของสถานะรายการ (ฝาก/ถอน ฯลฯ) และสิ่งที่ต้องทำต่อ จากตารางสถานะของหลังบ้านนี้",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"table": map[string]any{"type": "string", "enum": tables, "description": "ชนิดของรายการ"},
				"code":  map[string]any{"type": "integer", "description": "รหัสสถานะ ถ้าผู้ใช้บอกเป็นตัวเลข"},
				"label": map[string]any{"type": "string", "description": "ชื่อสถานะตามที่ผู้ใช้เห็น ถ้าบอกเป็นข้อความ", "maxLength": 80},
			}, "required": []string{"table"}},
		},
		{
			Name:        "host_facts",
			Description: "ข้อเท็จจริงคงที่ของหลังบ้านนี้ เช่น มีปุ่มสถานการณ์ฉุกเฉินหรือไม่ และทำอะไร",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"topic": map[string]any{"type": "string", "description": "หัวข้อ เช่น ปุ่มฉุกเฉิน", "maxLength": 80},
			}, "required": []string{"topic"}},
		},
	}
}

func normSearch(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}

// scoreMatch: ยิ่งมีคำสำคัญของเมนูอยู่ในคำค้นมาก (หรือคำค้นอยู่ในชื่อเมนู) ยิ่งได้คะแนน
func scoreMatch(query string, words ...string) int {
	q := normSearch(query)
	if q == "" {
		return 0
	}
	score := 0
	for _, w := range words {
		n := normSearch(w)
		if n == "" {
			continue
		}
		if strings.Contains(q, n) {
			score += 2 + len([]rune(n))/4
		} else if strings.Contains(n, q) {
			score++
		}
	}
	return score
}

type menuHit struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Group       string   `json:"group,omitempty"`
	Description string   `json:"description,omitempty"`
	Tabs        []string `json:"tabs,omitempty"`
	CanAccess   bool     `json:"can_access"`
	NeedMenu    string   `json:"need_permission_for,omitempty"`
	GrantHint   string   `json:"grant_hint,omitempty"`
}

type guideHit struct {
	Title     string   `json:"title"`
	MenuName  string   `json:"menu"`
	MenuPath  string   `json:"menu_path"`
	Tab       string   `json:"tab,omitempty"`
	Steps     []string `json:"steps"`
	Fields    []string `json:"fields,omitempty"`
	Note      string   `json:"note,omitempty"`
	CanAccess bool     `json:"can_access"`
}

func canAccess(conn *connector.Connector, rule string, t domain.ChatTicket) (bool, connector.PermissionRule) {
	if rule == "" {
		return true, connector.PermissionRule{}
	}
	r, ok := conn.Rule(rule)
	if !ok {
		return false, r
	}
	return r.Allows(t.Permissions, int(t.Level)), r
}

// lookupMenu คืนเมนู/วิธีทำที่ตรงคำค้น (สูงสุด 3 + 2) · ไม่มีข้อมูลของระบบ (ไม่ต้องกัน PII)
func lookupMenu(conn *connector.Connector, t domain.ChatTicket, query string, now time.Time) (map[string]any, *domain.Card) {
	type scored struct {
		i, s int
	}
	var ms []scored
	for i, m := range conn.Menus.Menus {
		words := append([]string{m.Name, m.Group}, m.Keywords...)
		for _, tab := range m.Tabs {
			words = append(words, tab.Name)
		}
		if s := scoreMatch(query, words...); s > 0 {
			ms = append(ms, scored{i, s})
		}
	}
	sort.SliceStable(ms, func(a, b int) bool { return ms[a].s > ms[b].s })
	menuByID := map[string]connector.Menu{}
	for _, m := range conn.Menus.Menus {
		menuByID[m.ID] = m
	}
	// path ใน menus.yaml ใช้ {service} / {service_b64} ได้ (หน้าที่ต้องรู้เว็บ เช่น ?service=<base64>)
	loc, _ := conn.Location()
	rc := connector.RenderContext{ServiceID: t.ServiceID, Now: now, Loc: loc}
	menuPath := func(p string) string {
		if out, err := rc.Render(p); err == nil {
			return out
		}
		return p
	}
	var menus []menuHit
	for k, x := range ms {
		if k >= 3 {
			break
		}
		m := conn.Menus.Menus[x.i]
		ok, rule := canAccess(conn, m.Permission, t)
		h := menuHit{Name: m.Name, Path: menuPath(m.Path), Group: m.Group, Description: m.Description, CanAccess: ok}
		for _, tab := range m.Tabs {
			h.Tabs = append(h.Tabs, tab.Name)
		}
		if !ok {
			h.NeedMenu, h.GrantHint = rule.Menu, rule.GrantHint
		}
		menus = append(menus, h)
	}
	var gs []scored
	for i, g := range conn.Menus.Guides {
		words := append([]string{g.Title}, g.Keywords...)
		if s := scoreMatch(query, words...); s > 0 {
			gs = append(gs, scored{i, s})
		}
	}
	sort.SliceStable(gs, func(a, b int) bool { return gs[a].s > gs[b].s })
	var guides []guideHit
	for k, x := range gs {
		if k >= 2 {
			break
		}
		g := conn.Menus.Guides[x.i]
		m := menuByID[g.Menu]
		ok, _ := canAccess(conn, m.Permission, t)
		guides = append(guides, guideHit{Title: g.Title, MenuName: m.Name, MenuPath: menuPath(m.Path), Tab: g.Tab, Steps: g.Steps, Fields: g.Fields, Note: g.Note, CanAccess: ok})
	}
	res := map[string]any{"status": "ok", "menus": menus, "guides": guides}
	if len(menus) == 0 && len(guides) == 0 {
		res["status"] = "not_found"
		res["message"] = "ไม่พบเมนูที่ตรงกับคำค้นในหลังบ้านนี้"
		return res, nil
	}
	// การ์ดอ้างอิงพาไปหน้าเมนูจริง
	var name, path string
	if len(guides) > 0 && guides[0].MenuPath != "" {
		name, path = guides[0].MenuName, guides[0].MenuPath
	} else if len(menus) > 0 {
		name, path = menus[0].Name, menus[0].Path
	}
	if path == "" {
		return res, nil
	}
	card := &domain.Card{
		ID: domain.NewID(), Kind: "reference", Tool: "lookup_menu", Title: "เมนูที่เกี่ยวข้อง",
		Fields:    []domain.CardField{{Label: "เมนู", Display: name, Value: name}},
		FetchedAt: now,
		Link:      &domain.CardLink{Label: "เปิดเมนู " + name, Path: path},
	}
	return res, card
}

func explainStatus(conn *connector.Connector, input map[string]any) map[string]any {
	table, _ := input["table"].(string)
	t, ok := conn.Statuses.Tables[table]
	if !ok {
		return map[string]any{"status": "not_found", "message": conn.Statuses.UnknownMessage}
	}
	var entry *connector.StatusEntry
	if f, ok := connector.ToFloat(input["code"]); ok {
		if e, ok := t.Find(int(f)); ok {
			entry = &e
		}
	}
	if entry == nil {
		if label, _ := input["label"].(string); label != "" {
			n := normSearch(label)
			for i := range t.Entries {
				e := t.Entries[i]
				cands := append([]string{e.Label}, e.Aliases...)
				for _, c := range cands {
					if normSearch(c) == n || (len([]rune(n)) >= 3 && strings.Contains(normSearch(c), n)) {
						entry = &e
						break
					}
				}
				if entry != nil {
					break
				}
			}
		}
	}
	if entry == nil {
		// ไม่ระบุสถานะ = ขอทั้งตาราง ("สถานะนี้แปลว่าอะไร" แบบรวม)
		if input["code"] == nil && input["label"] == nil {
			all := []map[string]any{}
			for _, e := range t.Entries {
				all = append(all, map[string]any{"label": e.Label, "meaning": e.Meaning, "next_action": e.NextAction})
			}
			return map[string]any{"status": "ok", "table": t.Label, "entries": all}
		}
		return map[string]any{"status": "not_found", "message": conn.Statuses.UnknownMessage}
	}
	return map[string]any{"status": "ok", "table": t.Label, "label": entry.Label, "meaning": entry.Meaning,
		"next_action": entry.NextAction, "final": entry.Final}
}

func hostFacts(conn *connector.Connector, topic string) map[string]any {
	var hits []map[string]any
	for _, f := range conn.Host.Facts {
		if scoreMatch(topic, append([]string{f.Title, f.ID}, f.Keywords...)...) > 0 {
			hits = append(hits, map[string]any{"title": f.Title, "text": f.Text})
		}
	}
	if len(hits) == 0 {
		return map[string]any{"status": "not_found", "message": "ไม่มีข้อมูลเรื่องนี้ของหลังบ้านนี้"}
	}
	return map[string]any{"status": "ok", "facts": hits}
}

// allowWords = คำที่โมเดลพูดได้ (ไม่ถูก guard ตัด): ชื่อเมนู/แท็บ/สถานะ/วิธีทำ ของ connector + ชื่อเว็บ
func allowWords(conn *connector.Connector, svc domain.Service) []string {
	out := []string{svc.ID, svc.Label, svc.DisplayName, conn.Host.Label}
	for _, m := range conn.Menus.Menus {
		out = append(out, m.Name, m.Group)
		for _, t := range m.Tabs {
			out = append(out, t.Name)
		}
	}
	for _, g := range conn.Menus.Guides {
		out = append(out, g.Title, g.Tab, g.Note)
		out = append(out, g.Steps...)
		out = append(out, g.Fields...)
	}
	for _, t := range conn.Statuses.Tables {
		out = append(out, t.Label)
		for _, e := range t.Entries {
			out = append(out, e.Label, e.Meaning, e.NextAction)
			out = append(out, e.Aliases...)
		}
	}
	for _, f := range conn.Host.Facts {
		out = append(out, f.Title, f.Text)
	}
	for _, r := range conn.Permissions.Rules {
		out = append(out, r.Menu, r.GrantHint)
	}
	return out
}
