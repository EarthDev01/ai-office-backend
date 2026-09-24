package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"github.com/gin-gonic/gin"
)

// ChatHandler — ██ SPIKE: คุยกับ LLM ตรง ๆ ยังไม่มี tool / ไม่แตะข้อมูลลูกค้า / ไม่บันทึกประวัติแชท
//
// บทสนทนาทั้งห้องถูกส่งมาจาก widget ทุกครั้ง (ยังไม่มี session ฝั่ง server)
type ChatHandler struct {
	svc port.OfficeService
	llm port.LLMClient // nil = ยังไม่ได้ตั้ง LLM_API_KEY
}

func NewChatHandler(svc port.OfficeService, llm port.LLMClient) *ChatHandler {
	return &ChatHandler{svc: svc, llm: llm}
}

const (
	chatMaxTurns    = 40
	chatMaxTurnChar = 4000
)

type chatRequest struct {
	Messages []struct {
		Role string `json:"role"` // user | ai
		Text string `json:"text"`
	} `json:"messages"`
}

// Chat — POST …/service/:service_id/chat · ตอบเป็น SSE: event delta {text} · done {} · error {message}
//
// ด่านสิทธิ์ใช้ตัวเดียวกับ /bootstrap — widget ที่ไม่ควรโผล่ก็แชทไม่ได้
func (h *ChatHandler) Chat(c *gin.Context, office domain.Office, caller domain.Caller) {
	if h.llm == nil {
		ResData(c, http.StatusServiceUnavailable, "LLM_NOT_CONFIGURED", "ยังไม่ได้ตั้งค่า LLM_API_KEY", nil)
		return
	}

	b, err := h.svc.Bootstrap(c.Request.Context(), office, c.Param("service_id"), caller)
	switch {
	case err == domain.ErrServiceNotAllowed:
		ResData(c, http.StatusForbidden, "SERVICE_NOT_ALLOWED", "บัญชีนี้ไม่มีสิทธิ์ใน service ที่เลือกอยู่", nil)
		return
	case err != nil:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	case !b.Enabled:
		ResData(c, http.StatusForbidden, "CHAT_DISABLED", b.Reason, nil)
		return
	}

	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Messages) == 0 || len(req.Messages) > chatMaxTurns {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("messages ต้องมี 1–%d ข้อความ", chatMaxTurns), nil)
		return
	}
	turns := make([]port.ChatTurn, 0, len(req.Messages))
	for _, m := range req.Messages {
		text := strings.TrimSpace(m.Text)
		if (m.Role != "user" && m.Role != "ai") || text == "" || utf8.RuneCountInString(text) > chatMaxTurnChar {
			ResData(c, http.StatusBadRequest, "BAD_REQUEST", "ข้อความไม่ถูกต้อง", nil)
			return
		}
		turns = append(turns, port.ChatTurn{Role: m.Role, Text: text})
	}
	if turns[len(turns)-1].Role != "user" {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "ข้อความสุดท้ายต้องเป็นของผู้ใช้", nil)
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // กัน reverse proxy กักข้อความไว้จนจบ
	w.WriteHeader(http.StatusOK)

	send := func(event string, data any) error {
		raw, _ := json.Marshal(data)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
			return err
		}
		w.Flush()
		return nil
	}

	err = h.llm.Stream(c.Request.Context(), systemPrompt(b, caller), turns, func(text string) error {
		return send("delta", gin.H{"text": text})
	})
	if err != nil {
		if c.Request.Context().Err() == nil { // client ยังรออยู่ — บอกให้รู้ว่าพัง
			log.Printf("[ERROR] chat llm office=%s service=%s: %v", office.ID, b.ServiceID, err)
			_ = send("error", gin.H{"message": "ผู้ช่วยตอบไม่สำเร็จ ลองใหม่อีกครั้ง"})
		}
		return
	}
	_ = send("done", gin.H{})
}

func systemPrompt(b domain.Bootstrap, caller domain.Caller) string {
	name := b.DisplayName
	if name == "" {
		name = "ผู้ช่วย AI"
	}
	return fmt.Sprintf(`คุณคือ "%s" ผู้ช่วยในหลังบ้านของเว็บ %s ผู้ใช้ที่คุยด้วยคือแอดมินชื่อ %s (role: %s)
ตอบเป็นภาษาไทย สุภาพ กระชับ
ตอนนี้คุณยังเข้าถึงข้อมูลในระบบหลังบ้านไม่ได้ ถ้าถูกถามเรื่องข้อมูล ยอดเงิน จำนวนรายการ หรือสถานะใด ๆ ของระบบ
ให้บอกตรง ๆ ว่ายังดูข้อมูลนั้นไม่ได้ ห้ามเดาหรือแต่งตัวเลขขึ้นมาเอง`,
		name, b.ServiceLabel, caller.Username, caller.RoleName)
}
