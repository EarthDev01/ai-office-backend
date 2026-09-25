package routes

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// ChatAdminHandler — ประวัติแชทและตรวจคำตอบ ฝั่งคอนโซล
type ChatAdminHandler struct {
	svc *service.ChatAdminService
}

func NewChatAdminHandler(svc *service.ChatAdminService) *ChatAdminHandler {
	return &ChatAdminHandler{svc: svc}
}

func qInt(c *gin.Context, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil {
		return v
	}
	return def
}

// qTime รับ RFC3339 หรือ YYYY-MM-DD (ตีความเป็นเวลาไทย · endOfDay = สิ้นวันนั้น)
func qTime(c *gin.Context, key string, endOfDay bool) *time.Time {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	if t, err := time.ParseInLocation("2006-01-02", v, domain.Bangkok()); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		return &t
	}
	return nil
}

func (h *ChatAdminHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบรายการนี้", nil)
	case errors.Is(err, service.ErrConfirmOld):
		ResData(c, http.StatusConflict, "CONFIRM_OLD_REQUIRED", "ข้อมูลเก่ากว่า 90 วัน ต้องกดยืนยันก่อนเปิดอ่าน", nil)
	default:
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	}
}

// Conversations — GET /conversations?office_id&service_id&user&q&verification&from&to&limit&offset
func (h *ChatAdminHandler) Conversations(c *gin.Context) {
	f := port.ConversationFilter{
		OfficeID: c.Query("office_id"), ServiceID: c.Query("service_id"), User: c.Query("user"), Text: c.Query("q"),
		From: qTime(c, "from", false), To: qTime(c, "to", true),
		Limit: qInt(c, "limit", 50), Offset: qInt(c, "offset", 0),
	}
	list, total, err := h.svc.SearchConversations(c.Request.Context(), actorOf(c), f, c.Query("verification"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}

// Conversation — GET /conversations/:id?confirm_old=1
func (h *ChatAdminHandler) Conversation(c *gin.Context) {
	v, err := h.svc.ReadConversation(c.Request.Context(), actorOf(c), c.Param("id"), c.Query("confirm_old") == "1")
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", v)
}

// VerificationQueue — GET /verifications/queue?office_id&service_id&status&from&to&limit&offset
func (h *ChatAdminHandler) VerificationQueue(c *gin.Context) {
	f := port.MessageFilter{
		OfficeID: c.Query("office_id"), ServiceID: c.Query("service_id"), Verification: c.Query("status"),
		From: qTime(c, "from", false), To: qTime(c, "to", true),
		Limit: qInt(c, "limit", 20), Offset: qInt(c, "offset", 0),
	}
	list, total, err := h.svc.VerificationQueue(c.Request.Context(), actorOf(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list, "error_types": domain.VerificationErrorTypes})
}

// Verify — POST /verifications {message_id, status, error_type?, correct_answer?, note?}
func (h *ChatAdminHandler) Verify(c *gin.Context) {
	var in service.VerifyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.svc.Verify(c.Request.Context(), actorOf(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", v)
}

// VerificationStats — GET /verifications/stats?office_id&service_id
func (h *ChatAdminHandler) VerificationStats(c *gin.Context) {
	st, err := h.svc.VerificationStats(c.Request.Context(), c.Query("office_id"), c.Query("service_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", st)
}
