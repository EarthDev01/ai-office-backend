package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// AssistantTools คือเครื่องมืออ่านอย่างเดียวของผู้ช่วยคอนโซล — ไม่มีตัวไหนแก้ข้อมูลได้
//
// แต่ละตัวผูกสิทธิ์ไว้ · คนถามไม่มีสิทธิ์ = LLM ไม่เห็นเครื่องมือนั้นเลย และ Run ปฏิเสธซ้ำอีกชั้น
type AssistantTools struct {
	offices  port.OfficeService
	usage    *UsageService
	chats    *ChatAdminService // nil ได้ (ไม่มีผลตรวจคำตอบ)
	settings *SettingsService
	keys     *LLMKeyService // nil ได้
	audit    *AuditService  // nil ได้
}

func NewAssistantTools(offices port.OfficeService, usage *UsageService, chats *ChatAdminService,
	settings *SettingsService, keys *LLMKeyService, audit *AuditService) *AssistantTools {
	return &AssistantTools{offices: offices, usage: usage, chats: chats, settings: settings, keys: keys, audit: audit}
}

type assistantTool struct {
	def   port.ToolDef
	perms []string // มีข้อใดข้อหนึ่ง = ใช้ได้
	run   func(t *AssistantTools, ctx context.Context, in map[string]any) (any, error)
}

var assistantToolList = []assistantTool{
	{
		def: port.ToolDef{
			Name:        "list_domains",
			Description: "รายการกลุ่ม domain และ service ทั้งหมด พร้อม URL และสถานะเปิด/ปิด (domain ปิด = ทุก service ในนั้นไม่ทำงาน)",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		perms: []string{domain.PermOfficeView},
		run:   (*AssistantTools).listDomains,
	},
	{
		def: port.ToolDef{
			Name: "get_usage",
			Description: "การใช้ token และจำนวนคำถามของผู้ช่วย · mode=monthly สรุปรายเดือนราย service (period=YYYY-MM ว่าง=เดือนนี้) · " +
				"mode=daily สรุปรายวัน (from/to = YYYY-MM-DD ว่าง=30 วันล่าสุด) กรองด้วย office_id/service_id ได้ · " +
				"office_id \"_console\" คือผู้ช่วยในคอนโซลเอง",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"mode":       map[string]any{"type": "string", "enum": []string{"monthly", "daily"}},
				"period":     map[string]any{"type": "string", "description": "YYYY-MM"},
				"from":       map[string]any{"type": "string", "description": "YYYY-MM-DD"},
				"to":         map[string]any{"type": "string", "description": "YYYY-MM-DD"},
				"office_id":  map[string]any{"type": "string", "description": "รหัส domain"},
				"service_id": map[string]any{"type": "string"},
			}},
		},
		perms: []string{domain.PermUsageView},
		run:   (*AssistantTools).getUsage,
	},
	{
		def: port.ToolDef{
			Name:        "get_verification_stats",
			Description: "ผลตรวจคำตอบของผู้ช่วย: จำนวนถูก/ผิด/ยังไม่ตรวจ และ % ถูกต้อง · กรองด้วย office_id/service_id ได้ (ว่าง = ทั้งหมด)",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"office_id":  map[string]any{"type": "string"},
				"service_id": map[string]any{"type": "string"},
			}},
		},
		perms: []string{domain.PermUsageView, domain.PermVerificationWrite},
		run:   (*AssistantTools).getVerificationStats,
	},
	{
		def: port.ToolDef{
			Name:        "get_settings",
			Description: "ตั้งค่าระบบปัจจุบัน: โมเดล AI ที่ใช้, ผู้ให้บริการไหนมี API key แล้ว (ไม่มีตัว key), ค่าประสิทธิภาพ, สถานะผู้ช่วยคอนโซล",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		perms: []string{domain.PermOfficeView},
		run:   (*AssistantTools).getSettings,
	},
	{
		def: port.ToolDef{
			Name:        "recent_activity",
			Description: "ประวัติการทำงานล่าสุดในคอนโซล (ใคร ทำอะไร เมื่อไร) · กรองด้วย actor (username) หรือ q (คำค้น) ได้ · limit สูงสุด 20",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"actor": map[string]any{"type": "string"},
				"q":     map[string]any{"type": "string"},
				"limit": map[string]any{"type": "integer"},
			}},
		},
		perms: []string{domain.PermAuditView},
		run:   (*AssistantTools).recentActivity,
	},
}

func (t *AssistantTools) usable(tool assistantTool, allowed map[string]bool) bool {
	if tool.def.Name == "get_verification_stats" && t.chats == nil {
		return false
	}
	if tool.def.Name == "recent_activity" && t.audit == nil {
		return false
	}
	for _, p := range tool.perms {
		if allowed[p] {
			return true
		}
	}
	return false
}

// Defs คืนเฉพาะเครื่องมือที่คนถามมีสิทธิ์
func (t *AssistantTools) Defs(allowed map[string]bool) []port.ToolDef {
	out := []port.ToolDef{}
	for _, tool := range assistantToolList {
		if t.usable(tool, allowed) {
			out = append(out, tool.def)
		}
	}
	return out
}

// Run รันเครื่องมือแล้วคืนผลเป็น JSON (หรือข้อความ error ให้ LLM อ่าน) — ตรวจสิทธิ์ซ้ำเสมอ
func (t *AssistantTools) Run(ctx context.Context, name string, in map[string]any, allowed map[string]bool) string {
	for _, tool := range assistantToolList {
		if tool.def.Name != name {
			continue
		}
		if !t.usable(tool, allowed) {
			return `{"error":"ผู้ถามไม่มีสิทธิ์ดูข้อมูลนี้"}`
		}
		v, err := tool.run(t, ctx, in)
		if err != nil {
			return fmt.Sprintf(`{"error":%q}`, err.Error())
		}
		raw, _ := json.Marshal(v)
		return string(raw)
	}
	return `{"error":"ไม่มีเครื่องมือนี้"}`
}

func str(in map[string]any, k string) string {
	v, _ := in[k].(string)
	return strings.TrimSpace(v)
}

func (t *AssistantTools) listDomains(ctx context.Context, _ map[string]any) (any, error) {
	offices, err := t.offices.List(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := t.offices.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	type svc struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Enabled bool   `json:"enabled"`
	}
	type dom struct {
		ID       string `json:"id"`
		Label    string `json:"label"`
		URL      string `json:"url"`
		Enabled  bool   `json:"enabled"`
		Hidden   bool   `json:"floating_button_hidden"`
		Services []svc  `json:"services"`
	}
	byGroup := map[string][]dom{}
	for _, o := range offices {
		d := dom{ID: o.ID, Label: o.Label, Enabled: o.Enabled, Hidden: o.IsHidden, Services: []svc{}}
		if len(o.AllowedOrigins) > 0 {
			d.URL = o.AllowedOrigins[0]
		}
		for _, s := range o.Services {
			d.Services = append(d.Services, svc{ID: s.ID, Label: s.Label, Enabled: s.Enabled})
		}
		byGroup[o.GroupID] = append(byGroup[o.GroupID], d)
	}
	type grp struct {
		Name    string `json:"group"`
		Domains []dom  `json:"domains"`
	}
	out := []grp{}
	known := map[string]bool{}
	for _, g := range groups {
		known[g.ID] = true
		out = append(out, grp{Name: g.Name, Domains: append([]dom{}, byGroup[g.ID]...)})
	}
	var loose []dom
	for gid, ds := range byGroup {
		if !known[gid] {
			loose = append(loose, ds...)
		}
	}
	if len(loose) > 0 {
		out = append(out, grp{Name: "ยังไม่ได้จัดกลุ่ม", Domains: loose})
	}
	return out, nil
}

func (t *AssistantTools) getUsage(ctx context.Context, in map[string]any) (any, error) {
	if str(in, "mode") == "daily" {
		rows, err := t.usage.Rollups(ctx, str(in, "office_id"), str(in, "service_id"), str(in, "from"), str(in, "to"))
		if err != nil {
			return nil, err
		}
		type day struct {
			Date                                string `json:"date"`
			OfficeID, ServiceID                 string
			Conversations, Questions, TokensIn  int64
			TokensOut, ToolErrors, Correct, Bad int64
		}
		out := make([]day, 0, len(rows))
		var q, tin, tout int64
		for _, r := range rows {
			in := r.InputTokens + r.CacheRead + r.CacheWrite
			out = append(out, day{Date: r.Date, OfficeID: r.OfficeID, ServiceID: r.ServiceID, Conversations: r.Conversations,
				Questions: r.Questions, TokensIn: in, TokensOut: r.OutputTokens, ToolErrors: r.ToolErrors, Correct: r.Correct, Bad: r.Wrong})
			q, tin, tout = q+r.Questions, tin+in, tout+r.OutputTokens
		}
		if len(out) > 62 {
			out = out[len(out)-62:]
		}
		return map[string]any{"total_questions": q, "total_tokens_in": tin, "total_tokens_out": tout, "days": out}, nil
	}
	period, list, err := t.usage.Periods(ctx, str(in, "period"))
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Questions > list[j].Questions })
	type row struct {
		OfficeID  string `json:"office_id"`
		ServiceID string `json:"service_id"`
		Questions int64  `json:"questions"`
		Tokens    int64  `json:"tokens_total"`
	}
	rows := make([]row, 0, len(list))
	var q, tok int64
	for _, u := range list {
		total := u.InputTokens + u.OutputTokens + u.CacheRead + u.CacheWrite
		rows = append(rows, row{OfficeID: u.OfficeID, ServiceID: u.ServiceID, Questions: u.Questions, Tokens: total})
		q, tok = q+u.Questions, tok+total
	}
	return map[string]any{"period": period, "total_questions": q, "total_tokens": tok, "by_service": rows}, nil
}

func (t *AssistantTools) getVerificationStats(ctx context.Context, in map[string]any) (any, error) {
	return t.chats.VerificationStats(ctx, str(in, "office_id"), str(in, "service_id"))
}

func (t *AssistantTools) getSettings(ctx context.Context, _ map[string]any) (any, error) {
	st := t.settings.Get(ctx)
	keys := map[string]bool{}
	for _, p := range domain.LLMProviderCatalog {
		keys[p.Label] = t.keys != nil && t.keys.HasKey(p.ID)
	}
	return map[string]any{
		"model":                map[string]any{"provider": st.LLM.Provider, "model": st.LLM.Model, "effort": st.LLM.Effort},
		"provider_has_api_key": keys,
		"console_assistant_on": st.AssistantEnabled,
		"max_concurrent":       st.MaxConcurrent,
		"ticket_ttl_min":       st.TicketTTLMin,
		"llm_timeout_sec":      st.LLMTimeoutSec,
		"stream_timeout_sec":   st.StreamTimeout,
		"max_output_tokens":    st.MaxOutputTokens,
		"history_turns":        st.HistoryTurns,
		"tool_timeout_ms":      st.ToolTimeoutMs,
		"support_message":      st.SupportMessage,
		"last_updated_by":      st.UpdatedBy,
	}, nil
}

func (t *AssistantTools) recentActivity(ctx context.Context, in map[string]any) (any, error) {
	limit := 10
	if v, ok := in["limit"].(float64); ok && v >= 1 {
		limit = min(int(v), 20)
	}
	page, err := t.audit.Query(ctx, domain.AuditFilter{Actor: str(in, "actor"), Q: str(in, "q"), Page: 1, PageSize: limit})
	if err != nil {
		return nil, err
	}
	type item struct {
		At      string `json:"at"`
		Actor   string `json:"actor"`
		Action  string `json:"action"`
		Status  string `json:"status"`
		Summary string `json:"summary"`
	}
	out := make([]item, 0, len(page.Items))
	for _, e := range page.Items {
		out = append(out, item{At: e.At.In(domain.Bangkok()).Format("2006-01-02 15:04"), Actor: e.Actor, Action: e.Action, Status: e.Status, Summary: e.Summary})
	}
	return out, nil
}
