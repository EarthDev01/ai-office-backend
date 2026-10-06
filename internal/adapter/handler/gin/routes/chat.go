package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// ChatHandler — ช่องทางแชทของ widget (โหมด browser)
//
//	page-config       บอก widget ว่าจะยิง API หลังบ้านที่ไหน และอ่านสิทธิ์จากเส้นไหน
//	browser-session   ออกตั๋วแชท (ใช้ token ของหน้า office ยืนยันตัวตน)
//	chat              SSE: status / fetch / card / token / done / error
//	chat/relay/:id    widget ส่งผลของคำสั่ง fetch กลับ
type ChatHandler struct {
	offices port.OfficeService
	chat    *service.ChatService // Enabled() = false → ปิดแชท
	relay   *service.Relay
	tickets port.ChatTicketIssuer
	conns   connector.Set
}

func NewChatHandler(offices port.OfficeService, chat *service.ChatService, relay *service.Relay,
	tickets port.ChatTicketIssuer, conns connector.Set) *ChatHandler {
	return &ChatHandler{offices: offices, chat: chat, relay: relay, tickets: tickets, conns: conns}
}

// connFor = connector ของหลังบ้านชนิดที่ office นี้ตั้งไว้ · ไม่พบ = ตอบ 503 ให้แล้ว
func (h *ChatHandler) connFor(c *gin.Context, office domain.Office) (*connector.Connector, bool) {
	conn, ok := h.conns.For(office.Kind)
	if !ok {
		log.Printf("[ERROR] office %s ตั้ง kind %q ที่ไม่มี connector", office.ID, office.Kind)
		ResData(c, http.StatusServiceUnavailable, "KIND_NOT_SUPPORTED", "ยังไม่รองรับหลังบ้านชนิดนี้", nil)
		return nil, false
	}
	return conn, true
}

// pageConfig = วิธีอ่าน "ใครล็อกอิน + เปิดเว็บไหน" จากหน้าหลังบ้านชนิดนี้ (ชื่อช่อง storage ไม่ใช่ความลับ)
type pageConfig struct {
	Kind        string                  `json:"kind"`
	Mode        string                  `json:"mode"`
	HostAPIBase string                  `json:"host_api_base"`
	Token       connector.TokenSource   `json:"token"`
	Service     connector.ServiceSource `json:"service"`
	AuthScheme  string                  `json:"auth_scheme"`
	Identity    *connector.IdentitySpec `json:"identity,omitempty"`
}

// PageConfig — GET /api/ai/widget/page-config (office มาจาก Origin)
//
// ที่อยู่ API หลังบ้าน: ค่าที่ตั้งไว้ให้ office นี้ในคอนโซลก่อน ไม่มีจึงใช้ template ของ connector ({origin}/api)
func (h *ChatHandler) PageConfig(c *gin.Context, office domain.Office) {
	conn, ok := h.connFor(c, office)
	if !ok {
		return
	}
	pa := conn.Host.PageAuth
	origin, _ := domain.NormalizeOrigin(c.GetHeader("Origin"))
	base := office.HostAPIBase
	if base == "" {
		base = strings.ReplaceAll(pa.HostAPIBase, "{origin}", origin)
	}
	scheme := pa.AuthScheme
	if scheme == "" {
		scheme = "Bearer"
	}
	ResData(c, http.StatusOK, "SUCCESS", "", pageConfig{
		Kind:        conn.Kind,
		Mode:        conn.Host.Mode,
		HostAPIBase: base,
		Token:       pa.Token,
		Service:     pa.Service,
		AuthScheme:  scheme,
		Identity:    pa.Identity,
	})
}

var permCode = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)

// OpenSession — POST …/service/:service_id/browser-session · body {permissions: [รหัสสิทธิ์]}
//
// รหัสสิทธิ์มาจากหน้า office (widget ยิงเส้นสิทธิ์ของหลังบ้านเอง) เพราะ JWT ไม่มีสิทธิ์ติดมา
// เชื่อได้ในระดับ "กั้นก่อนยิง" เท่านั้น — หลังบ้านตรวจซ้ำด้วย token จริงของแอดมินทุกครั้ง
func (h *ChatHandler) OpenSession(c *gin.Context, office domain.Office, caller domain.Caller) {
	b, err := h.offices.Bootstrap(c.Request.Context(), office, c.Param("service_id"), caller)
	switch {
	case errors.Is(err, domain.ErrServiceNotAllowed):
		ResData(c, http.StatusForbidden, "SERVICE_NOT_ALLOWED", "บัญชีนี้ไม่มีสิทธิ์ใน service ที่เลือกอยู่", nil)
		return
	case err != nil:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	case !b.Enabled:
		ResData(c, http.StatusForbidden, "CHAT_DISABLED", b.Reason, nil)
		return
	}

	var body struct {
		Permissions []string `json:"permissions"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&body); err != nil || len(body.Permissions) > 1000 {
			ResData(c, http.StatusBadRequest, "BAD_REQUEST", "permissions ต้องเป็น array ของรหัสสิทธิ์", nil)
			return
		}
	}
	seen := map[string]bool{}
	var perms []string
	for _, p := range append(append([]string{}, caller.Permissions...), body.Permissions...) {
		if permCode.MatchString(p) && !seen[p] {
			seen[p] = true
			perms = append(perms, p)
		}
	}

	t := domain.ChatTicket{
		OfficeID: office.ID, ServiceID: b.ServiceID, AdminID: caller.AdminID, Username: caller.Username,
		Level: caller.Level, Permissions: perms,
		Session: domain.SessionFingerprint(strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))),
	}
	tok, err := h.tickets.Issue(t)
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "ออกตั๋วไม่สำเร็จ", nil)
		return
	}
	ttl := h.tickets.TTL()
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{
		"ticket":     tok,
		"expires_in": int(ttl / time.Second),
		"expires_at": time.Now().Add(ttl).UTC().Format(time.RFC3339),
	})
}

// Conversations — GET …/service/:service_id/conversations?days=7 · ห้องของตัวเองใน session หลังบ้านนี้
func (h *ChatHandler) Conversations(c *gin.Context, office domain.Office, t domain.ChatTicket) {
	svc, ok := h.activeService(c, office, t)
	if !ok {
		return
	}
	days, _ := strconv.Atoi(c.Query("days"))
	list, err := h.chat.Conversations(c.Request.Context(), office.ID, svc.ID, t, days)
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "โหลดประวัติไม่สำเร็จ", nil)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, cv := range list {
		out = append(out, gin.H{"id": cv.ID, "title": cv.Title, "updated_at": cv.UpdatedAt, "message_count": cv.MessageCount})
	}
	ResData(c, http.StatusOK, "SUCCESS", "", out)
}

// ConversationMessages — GET …/service/:service_id/conversations/:id · ข้อความ + การ์ดของห้องตัวเอง
func (h *ChatHandler) ConversationMessages(c *gin.Context, office domain.Office, t domain.ChatTicket) {
	svc, ok := h.activeService(c, office, t)
	if !ok {
		return
	}
	cv, msgs, err := h.chat.ConversationMessages(c.Request.Context(), office.ID, svc.ID, t, c.Param("id"))
	if errors.Is(err, domain.ErrNotFound) {
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบห้องแชทนี้", nil)
		return
	}
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "โหลดประวัติไม่สำเร็จ", nil)
		return
	}
	// ส่งเฉพาะสิ่งที่ผู้ใช้เคยเห็นในห้อง — ไม่มี tool_calls/usage/ผลตรวจ
	out := make([]gin.H, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, gin.H{"role": m.Role, "text": m.Text, "cards": m.Cards, "status": m.Status, "created_at": m.CreatedAt})
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"id": cv.ID, "title": cv.Title, "messages": out})
}

// activeService ตรวจซ้ำตอนใช้ตั๋ว — office/service ที่ถูกปิดหลังออกตั๋วต้องใช้ต่อไม่ได้
func (h *ChatHandler) activeService(c *gin.Context, office domain.Office, t domain.ChatTicket) (domain.Service, bool) {
	if t.ServiceID != c.Param("service_id") {
		ResData(c, http.StatusForbidden, "TICKET_SERVICE_MISMATCH", "ตั๋วนี้ออกให้ service อื่น — ขอตั๋วใหม่", nil)
		return domain.Service{}, false
	}
	svc, ok := office.FindService(t.ServiceID)
	if !office.Enabled || !ok || !svc.Enabled {
		ResData(c, http.StatusForbidden, "SERVICE_DISABLED", "ผู้ช่วยของเว็บนี้ถูกปิดอยู่", nil)
		return domain.Service{}, false
	}
	return svc, true
}

// Chat — POST …/service/:service_id/chat · body {conversation_id, text} · ตอบเป็น SSE
func (h *ChatHandler) Chat(c *gin.Context, office domain.Office, t domain.ChatTicket) {
	if !h.chat.Enabled() {
		ResData(c, http.StatusServiceUnavailable, "LLM_NOT_CONFIGURED", "ยังไม่ได้ตั้งค่า LLM", nil)
		return
	}
	svc, ok := h.activeService(c, office, t)
	if !ok {
		return
	}
	var req struct {
		ConversationID string `json:"conversation_id"`
		Text           string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.ConversationID) > 64 {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ต้องเป็น {conversation_id, text}", nil)
		return
	}

	conn, ok := h.connFor(c, office)
	if !ok {
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // กัน reverse proxy กักข้อความไว้จนจบ
	w.WriteHeader(http.StatusOK)

	// tool หลายตัวส่ง fetch/card พร้อมกันได้ — เขียนทีละ event
	var mu sync.Mutex
	emit := func(event string, data any) error {
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
			return err
		}
		w.Flush()
		return nil
	}

	err := h.chat.Handle(c.Request.Context(), office, svc, conn, t,
		service.ChatRequest{ConversationID: req.ConversationID, Text: req.Text}, emit)
	if err == nil || c.Request.Context().Err() != nil {
		return
	}
	var ce *service.ChatError
	if !errors.As(err, &ce) {
		log.Printf("[ERROR] chat (office=%s service=%s): %v", office.ID, svc.ID, err)
		ce = &service.ChatError{Code: "internal", Message: "เกิดข้อผิดพลาด ลองใหม่อีกครั้ง"}
	}
	_ = emit("error", ce)
}

// Relay — POST …/service/:service_id/chat/relay/:id · body {status, body}
func (h *ChatHandler) Relay(c *gin.Context, office domain.Office, t domain.ChatTicket) {
	if _, ok := h.activeService(c, office, t); !ok {
		return
	}
	var req struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ต้องเป็น {status, body} และไม่เกิน 2 MB", nil)
		return
	}
	if err := h.relay.Deliver(t.ID, c.Param("id"), service.RelayResult{Status: req.Status, Body: req.Body}); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
}

type kindInfo struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Default bool   `json:"default"`
}

// ListKinds — GET /api/ai/admin/kinds · ชนิดหลังบ้านที่มี connector (ให้คอนโซลเลือกตอนตั้งค่า domain)
func ListKinds(c *gin.Context, conns connector.Set) {
	out := []kindInfo{}
	for _, k := range conns.Kinds() {
		conn, _ := conns.Get(k)
		label := conn.Host.Label
		if label == "" {
			label = k
		}
		out = append(out, kindInfo{Kind: k, Label: label, Default: k == conns.Default})
	}
	ResData(c, http.StatusOK, "SUCCESS", "", out)
}
