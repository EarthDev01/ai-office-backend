package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// AssistantHandler — ผู้ช่วย AI ในคอนโซล + คู่มือการใช้งาน
//
//	GET  /guide              คู่มือ (Markdown) ทุกคนที่ล็อกอิน
//	GET  /assistant/status   ปุ่มผู้ช่วยควรโชว์ไหม
//	POST /assistant          ถาม 1 ข้อ ตอบเป็น SSE: status / token / done / error (ชุดเดียวกับแชทของ widget)
type AssistantHandler struct {
	svc   *service.AssistantService // nil = ไม่เปิดผู้ช่วย
	guide string
}

func NewAssistantHandler(svc *service.AssistantService, guide string) *AssistantHandler {
	return &AssistantHandler{svc: svc, guide: guide}
}

const ctxConsoleRole = "console_role"

func consoleRole(c *gin.Context) domain.Role {
	v, _ := c.Get(ctxConsoleRole)
	r, _ := v.(domain.Role)
	return r
}

func (h *AssistantHandler) Guide(c *gin.Context) {
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"markdown": h.guide})
}

func (h *AssistantHandler) Status(c *gin.Context) {
	enabled, ready := false, false
	if h.svc != nil {
		enabled, ready = h.svc.Status(c.Request.Context())
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"enabled": enabled, "ready": ready})
}

func (h *AssistantHandler) Ask(c *gin.Context) {
	if h.svc == nil {
		ResData(c, http.StatusServiceUnavailable, "ASSISTANT_DISABLED", "ผู้ช่วยในคอนโซลปิดอยู่", nil)
		return
	}
	enabled, ready := h.svc.Status(c.Request.Context())
	if !enabled {
		ResData(c, http.StatusServiceUnavailable, "ASSISTANT_DISABLED", "ผู้ช่วยในคอนโซลปิดอยู่ — เปิดได้ที่ตั้งค่าระบบ", nil)
		return
	}
	if !ready {
		ResData(c, http.StatusServiceUnavailable, "LLM_NOT_CONFIGURED", "โมเดลที่เลือกไว้ยังไม่มี API key", nil)
		return
	}
	var req service.AssistantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "body ต้องเป็น {messages, page}", nil)
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
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

	user := service.AssistantUser{Username: consoleUsername(c), Role: consoleRole(c)}
	err := h.svc.Handle(c.Request.Context(), user, req, emit)
	if err == nil || c.Request.Context().Err() != nil {
		return
	}
	var ce *service.ChatError
	switch {
	case errors.As(err, &ce):
	case errors.Is(err, service.ErrAssistantDisabled):
		ce = &service.ChatError{Code: "disabled", Message: "ผู้ช่วยในคอนโซลปิดอยู่"}
	case errors.Is(err, service.ErrAssistantNoLLM):
		ce = &service.ChatError{Code: "llm_not_configured", Message: "โมเดลที่เลือกไว้ยังไม่มี API key"}
	default:
		log.Printf("[ERROR] ผู้ช่วยคอนโซล (%s): %v", user.Username, err)
		ce = &service.ChatError{Code: "internal", Message: "เกิดข้อผิดพลาด ลองใหม่อีกครั้ง"}
	}
	_ = emit("error", ce)
}
