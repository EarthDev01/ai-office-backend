package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// context keys ที่ middleware (package gin) แปะไว้ — ประกาศเป็น literal ที่นี่กัน import cycle
const (
	CtxOffice  = "ai_office_office"
	CtxTicket  = "ai_office_ticket"
	CtxService = "ai_office_service"
)

func officeFrom(c *gin.Context) domain.Office {
	v, _ := c.Get(CtxOffice)
	o, _ := v.(domain.Office)
	return o
}

func ticketFrom(c *gin.Context) domain.AccessTicket {
	v, _ := c.Get(CtxTicket)
	t, _ := v.(domain.AccessTicket)
	return t
}

func serviceFrom(c *gin.Context) domain.Service {
	v, _ := c.Get(CtxService)
	s, _ := v.(domain.Service)
	return s
}

type WidgetAPI struct {
	Offices    port.OfficeService
	Connectors port.ConnectorRegistry
	ChatSvc    *service.ChatService
}

// PageConfig — widget รู้ kind + วิธีอ่านล็อกอินบนหน้า (R2) · ไม่มีข้อมูลลูกค้า ไม่ต้อง auth
func (h *WidgetAPI) PageConfig(c *gin.Context) {
	o := officeFrom(c)
	if !o.AllowsOrigin(c.GetHeader("Origin")) {
		ResData(c, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "โดเมนนี้ไม่ได้ลงทะเบียนไว้กับ key นี้", nil)
		return
	}
	if !o.Enabled {
		ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"enabled": false, "reason": domain.ReasonOfficeDisabled})
		return
	}
	conn, ok := h.Connectors.Get(o.Kind)
	if o.Kind == "" || !ok {
		ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"enabled": false, "reason": domain.ReasonKindNotSet})
		return
	}
	pa := conn.Host.PageAuth
	if pa.AuthScheme == "" {
		pa.AuthScheme = "Bearer"
	}
	if pa.Service.Encoding == "" {
		pa.Service.Encoding = "none"
	}
	c.Header("Cache-Control", "public, max-age=60")
	mode := "host"
	if conn.Host.IsBrowser() {
		mode = "browser"
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"enabled": true, "kind": o.Kind, "mode": mode, "page_auth": pa, "version": 2})
}

type sessionBody struct {
	Kind  string          `json:"kind"`
	User  domain.HostUser `json:"user"`
	Grant string          `json:"grant"`
}

// Session — host ขอตั๋วให้ผู้ใช้ที่ host ยืนยันแล้ว (server-to-server · X-AI-Secret) — contract §2
func (h *WidgetAPI) Session(c *gin.Context) {
	var b sessionBody
	if err := c.ShouldBindJSON(&b); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ไม่ถูกต้อง", nil)
		return
	}
	res, err := h.Offices.OpenSession(c.Request.Context(), port.SessionRequest{
		PublicKey: c.Param("public_key"), ServiceID: c.Param("service_id"),
		Secret: strings.TrimSpace(c.GetHeader("X-AI-Secret")), Kind: b.Kind, User: b.User, Grant: b.Grant,
	})
	var ref *domain.RefusalError
	switch {
	case err == nil:
		c.Header("Cache-Control", "no-store")
		ResData(c, http.StatusOK, "SUCCESS", "", res)
	case errors.As(err, &ref):
		ResData(c, http.StatusForbidden, "SESSION_REFUSED", reasonText(ref.Reason), gin.H{"reason": ref.Reason})
	case errors.Is(err, domain.ErrSecretInvalid):
		// 403 ไม่ใช่ 401 — host ส่ง body นี้ต่อให้ browser ตรง ๆ และ UI หลังบ้านทั้ง 2 ตัว logout เมื่อเห็น code 401 (contract §2)
		ResData(c, http.StatusForbidden, "SECRET_INVALID", "secret ไม่ถูกต้อง หรือถูกยกเลิกแล้ว", nil)
	case errors.Is(err, domain.ErrBadRequest):
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "ต้องมี user.id และ grant", nil)
	case errors.Is(err, domain.ErrNotFound):
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบ office ของ key นี้", nil)
	default:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "ออกตั๋วไม่สำเร็จ", nil)
	}
}

type browserSessionBody struct {
	User    domain.HostUser `json:"user"`
	TokenFP string          `json:"token_fp"`
}

// BrowserSession — โหมด browser: widget ขอตั๋วเอง (ไม่มี host ออกให้) · ต้องมาจากโดเมนที่ลงทะเบียน
func (h *WidgetAPI) BrowserSession(c *gin.Context) {
	var b browserSessionBody
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)).Decode(&b); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ไม่ถูกต้อง", nil)
		return
	}
	res, err := h.Offices.OpenBrowserSession(c.Request.Context(), port.BrowserSessionRequest{
		PublicKey: c.Param("public_key"), ServiceID: c.Param("service_id"), Origin: c.GetHeader("Origin"),
		User: b.User, TokenFP: b.TokenFP,
	})
	var ref *domain.RefusalError
	switch {
	case err == nil:
		c.Header("Cache-Control", "no-store")
		ResData(c, http.StatusOK, "SUCCESS", "", res)
	case errors.As(err, &ref):
		ResData(c, http.StatusForbidden, "SESSION_REFUSED", reasonText(ref.Reason), gin.H{"reason": ref.Reason})
	case errors.Is(err, domain.ErrOriginNotAllowed):
		ResData(c, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "โดเมนนี้ไม่ได้ลงทะเบียนไว้กับ office นี้", nil)
	case errors.Is(err, domain.ErrBadRequest):
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "ต้องมี user.id และ token_fp", nil)
	case errors.Is(err, domain.ErrNotFound):
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบ office ของ key นี้", nil)
	default:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "ออกตั๋วไม่สำเร็จ", nil)
	}
}

type relayBody struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// ChatRelay — widget ส่งผลการยิง API หลังบ้านกลับมา (โหมด browser) · ผูกกับตั๋วที่ถืออยู่
func (h *WidgetAPI) ChatRelay(c *gin.Context) {
	var b relayBody
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, (2<<20)+(64<<10))).Decode(&b); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ไม่ถูกต้องหรือใหญ่เกิน", nil)
		return
	}
	if err := h.ChatSvc.DeliverRelay(c.Request.Context(), ticketFrom(c), c.Param("id"), b.Status, []byte(b.Body)); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "รับผลไม่ได้", nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
}

func reasonText(r string) string {
	switch r {
	case domain.ReasonOfficeDisabled, domain.ReasonServiceDisabled:
		return "บริการ AI ของเว็บนี้ปิดใช้งานอยู่"
	case domain.ReasonNotInAllowlist:
		return "บัญชีนี้ยังไม่อยู่ในรายชื่อที่ใช้ AI ได้"
	case domain.ReasonQuotaExceeded:
		return "โควตาของเดือนนี้ใช้ครบแล้ว"
	case domain.ReasonKindMismatch, domain.ReasonKindNotSet:
		return "office นี้ตั้งชนิดหลังบ้านไม่ตรง"
	case domain.ReasonNoSecret:
		return "เว็บนี้ยังไม่มี secret_key"
	}
	return r
}

// Bootstrap — หน้าตา widget สำหรับผู้ถือตั๋ว
func (h *WidgetAPI) Bootstrap(c *gin.Context) {
	b, err := h.Offices.Bootstrap(c.Request.Context(), c.Param("public_key"), c.Param("service_id"), c.GetHeader("Origin"), ticketFrom(c))
	switch {
	case err == nil:
		ResData(c, http.StatusOK, "SUCCESS", "", b)
	case errors.Is(err, domain.ErrOriginNotAllowed):
		ResData(c, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "โดเมนนี้ไม่ได้ลงทะเบียนไว้กับ key นี้", nil)
	case errors.Is(err, domain.ErrNotFound):
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบ office ของ key นี้", nil)
	case errors.Is(err, domain.ErrTicketInvalid):
		ResData(c, http.StatusUnauthorized, "TICKET_INVALID", "ตั๋วไม่ตรงกับเว็บนี้", nil)
	default:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "", nil)
	}
}

// Chat — SSE (spec §7.2): status · card · token · done · error
//
// ██ ingress ต้องไม่บัฟเฟอร์ (X-Accel-Buffering: no) + timeout ≥ 60 วิ (PRE-6)
func (h *WidgetAPI) Chat(c *gin.Context) {
	var req service.ChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)).Decode(&req); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ไม่ถูกต้อง", nil)
		return
	}
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	var mu sync.Mutex
	write := func(event string, data any) {
		b, _ := json.Marshal(data)
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		w.Flush()
	}
	// ping ทุก 15 วิ กัน proxy ตัดสายระหว่างรอ
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-c.Request.Context().Done():
				return
			case <-t.C:
				mu.Lock()
				fmt.Fprint(w, ": ping\n\n")
				w.Flush()
				mu.Unlock()
			}
		}
	}()
	h.ChatSvc.Handle(c.Request.Context(), officeFrom(c), serviceFrom(c), ticketFrom(c), req, func(ev service.ChatEvent) {
		if c.Request.Context().Err() != nil {
			return
		}
		write(ev.Type, ev.Data)
	})
	close(stop)
}

func (h *WidgetAPI) ListConversations(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	list, err := h.ChatSvc.ListHistory(c.Request.Context(), officeFrom(c), serviceFrom(c), ticketFrom(c), days)
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "", nil)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, cv := range list {
		out = append(out, gin.H{"id": cv.ID, "title": cv.Title, "opened_at": cv.OpenedAt, "last_message_at": cv.LastMessageAt, "closed": cv.ClosedAt != nil})
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"data": out})
}

func (h *WidgetAPI) GetConversation(c *gin.Context) {
	cv, msgs, err := h.ChatSvc.GetHistory(c.Request.Context(), officeFrom(c), serviceFrom(c), ticketFrom(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบห้องแชทนี้", nil)
			return
		}
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "", nil)
		return
	}
	// ให้ widget เห็นแค่ที่ต้องแสดง — ไม่ส่ง tool_calls (path ของ host) / usage / cost / model ออกไป browser
	out := make([]gin.H, 0, len(msgs))
	for _, m := range msgs {
		cards := m.Cards
		if cards == nil {
			cards = []domain.Card{}
		}
		out = append(out, gin.H{"id": m.ID, "role": m.Role, "text": m.Text, "cards": cards, "created_at": m.CreatedAt})
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{
		"conversation": gin.H{"id": cv.ID, "title": cv.Title, "opened_at": cv.OpenedAt, "closed": cv.ClosedAt != nil},
		"messages":     out,
	})
}

func (h *WidgetAPI) CloseConversation(c *gin.Context) {
	var b struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&b)
	if err := h.ChatSvc.CloseConversation(c.Request.Context(), officeFrom(c), serviceFrom(c), ticketFrom(c), c.Param("id"), b.Reason); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบห้องแชทนี้", nil)
			return
		}
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
}
