package routes

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// AdminHandler = API ฝั่งคอนโซลที่เพิ่มใน w-17 (V10) · operator = console user ที่ล็อกอินอยู่
type AdminHandler struct {
	Admin      *service.AdminService
	Settings   *service.SettingsService
	Connectors port.ConnectorRegistry
	Rollups    port.RollupRepository
}

func operator(c *gin.Context) string { return consoleUsername(c) }

func qInt(c *gin.Context, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil {
		return v
	}
	return def
}

// qTime รับ RFC3339 หรือ YYYY-MM-DD (ตีความเป็นเวลาไทย)
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

func (h *AdminHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบรายการนี้", nil)
	case errors.Is(err, service.ErrConfirmOld):
		ResData(c, http.StatusConflict, "CONFIRM_OLD_REQUIRED", "ข้อมูลเก่ากว่า 90 วัน ต้องกดยืนยันเพิ่มก่อนเปิดอ่าน", nil)
	default:
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	}
}

// ---- ประวัติแชท (K2) ----

func (h *AdminHandler) Conversations(c *gin.Context) {
	f := port.ConversationFilter{
		OfficeID: c.Query("office_id"), ServiceID: c.Query("service_id"), UserID: c.Query("user"),
		From: qTime(c, "from", false), To: qTime(c, "to", true), Text: c.Query("q"),
		Verification: c.Query("verification"),
		Limit:        qInt(c, "limit", 50), Offset: qInt(c, "offset", 0),
	}
	list, total, err := h.Admin.SearchConversations(c.Request.Context(), operator(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}

func (h *AdminHandler) Conversation(c *gin.Context) {
	v, err := h.Admin.ReadConversation(c.Request.Context(), operator(c), c.Param("id"), c.Query("confirm_old") == "1")
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", v)
}

// Messages = ค้นข้อความ (เช่น คำในบทสนทนา · สถานะตรวจ) — เนื้อความ → access_log
func (h *AdminHandler) Messages(c *gin.Context) {
	f := port.MessageFilter{
		OfficeID: c.Query("office_id"), ServiceID: c.Query("service_id"), UserID: c.Query("user"),
		Role: c.Query("role"), Verification: c.Query("verification"), Text: c.Query("q"),
		From: qTime(c, "from", false), To: qTime(c, "to", true),
		Limit: qInt(c, "limit", 50), Offset: qInt(c, "offset", 0),
	}
	list, total, err := h.Admin.SearchMessages(c.Request.Context(), operator(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}

func (h *AdminHandler) Message(c *gin.Context) {
	m, v, err := h.Admin.ReadMessage(c.Request.Context(), operator(c), c.Param("id"), c.Query("confirm_old") == "1")
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"message": m, "verification": v})
}

// ---- ตรวจคำตอบ (K3) ----

func (h *AdminHandler) VerificationQueue(c *gin.Context) {
	f := port.MessageFilter{
		OfficeID: c.Query("office_id"), ServiceID: c.Query("service_id"), Verification: c.Query("status"),
		From: qTime(c, "from", false), To: qTime(c, "to", true),
		Limit: qInt(c, "limit", 20), Offset: qInt(c, "offset", 0),
	}
	list, total, err := h.Admin.VerificationQueue(c.Request.Context(), operator(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list, "error_types": domain.VerificationErrorTypes})
}

func (h *AdminHandler) Verify(c *gin.Context) {
	var in service.VerifyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Admin.Verify(c.Request.Context(), operator(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", v)
}

func (h *AdminHandler) VerificationStats(c *gin.Context) {
	st, err := h.Admin.VerificationStats(c.Request.Context(), c.Query("office_id"), c.Query("service_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", st)
}

// ---- โควตา/ต้นทุน (K4) ----

func (h *AdminHandler) Quotas(c *gin.Context) {
	list, err := h.Admin.Quotas(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"data": list})
}

func (h *AdminHandler) RollupsList(c *gin.Context) {
	list, err := h.Rollups.List(c.Request.Context(), c.Query("office_id"), c.Query("service_id"), c.Query("from"), c.Query("to"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"data": list})
}

// ---- บันทึกการเข้าถึง ----

func (h *AdminHandler) AccessLog(c *gin.Context) {
	list, total, err := h.Admin.Access.List(c.Request.Context(), c.Query("office_id"), c.Query("service_id"), c.Query("operator"), qInt(c, "limit", 100), qInt(c, "offset", 0))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}

// ---- ลบตามคำขอ PDPA (K5) ----

func (h *AdminHandler) CreateDeletion(c *gin.Context) {
	var in service.DeletionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, err)
		return
	}
	req, err := h.Admin.Delete(c.Request.Context(), operator(c), in)
	if err != nil {
		if req.ID != "" {
			ResData(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error(), req)
			return
		}
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", req)
}

func (h *AdminHandler) Deletions(c *gin.Context) {
	list, total, err := h.Admin.Deletes.List(c.Request.Context(), qInt(c, "limit", 50), qInt(c, "offset", 0))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}

// ---- settings ระดับระบบ ----

func (h *AdminHandler) GetSettings(c *gin.Context) {
	st, err := h.Settings.Get(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", st)
}

func (h *AdminHandler) PatchSettings(c *gin.Context) {
	var p service.SettingsPatch
	if err := c.ShouldBindJSON(&p); err != nil {
		h.fail(c, err)
		return
	}
	st, err := h.Settings.Update(c.Request.Context(), p, operator(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", st)
}

// Connectors = kind ที่มีปลั๊ก + คำถามที่ครอบ (อ่านอย่างเดียว — แก้ผ่าน git/deploy เท่านั้น NG-21)
func (h *AdminHandler) ConnectorsList(c *gin.Context) {
	out := []gin.H{}
	for _, k := range h.Connectors.Kinds() {
		conn, _ := h.Connectors.Get(k)
		cov := conn.Coverage()
		qs := make([]string, 0, len(cov))
		for q := range cov {
			qs = append(qs, q)
		}
		sort.Strings(qs)
		tools := []gin.H{}
		for _, t := range conn.Tools {
			tools = append(tools, gin.H{"name": t.Name, "questions": t.Questions, "permission": t.Permission, "freshness": t.Freshness})
		}
		out = append(out, gin.H{"kind": k, "label": conn.Host.Label, "questions": qs, "tools": tools})
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"data": out})
}
