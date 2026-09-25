package routes

import (
	"errors"
	"net/http"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// OpsHandler — ตั้งค่าระบบ · การใช้ token · ลบตามคำขอ (PDPA) · บันทึกการเข้าถึง
type OpsHandler struct {
	settings *service.SettingsService
	usage    *service.UsageService
	deletion *service.DeletionService
	llm      LLMInfo
}

// LLMInfo คือโมเดลที่ใช้อยู่ (มาจาก .env) — หน้าตั้งค่าแสดงแบบอ่านอย่างเดียว
type LLMInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
}

func NewOpsHandler(settings *service.SettingsService, usage *service.UsageService, deletion *service.DeletionService, llm LLMInfo) *OpsHandler {
	return &OpsHandler{settings: settings, usage: usage, deletion: deletion, llm: llm}
}

// GetSettings — GET /settings
func (h *OpsHandler) GetSettings(c *gin.Context) {
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{
		"settings": h.settings.Get(c.Request.Context()),
		"ranges":   domain.SettingRanges(),
		"llm":      h.llm,
	})
}

// PatchSettings — PATCH /settings (ส่ง updated_at ที่เห็นล่าสุดมาด้วยเสมอ)
func (h *OpsHandler) PatchSettings(c *gin.Context) {
	var p service.SettingsPatch
	if err := c.ShouldBindJSON(&p); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง", nil)
		return
	}
	st, err := h.settings.Update(c.Request.Context(), p, actorOf(c))
	switch {
	case errors.Is(err, service.ErrSettingsConflict):
		ResData(c, http.StatusConflict, "SETTINGS_CONFLICT",
			"มีคนแก้ตั้งค่าไปก่อนแล้ว (โดย "+st.UpdatedBy+") — โหลดค่าล่าสุดแล้วแก้ใหม่", st)
	case err != nil:
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	default:
		ResData(c, http.StatusOK, "SUCCESS", "", st)
	}
}

// Usage — GET /usage?period=YYYY-MM (ว่าง = เดือนนี้)
func (h *OpsHandler) Usage(c *gin.Context) {
	period, list, err := h.usage.Periods(c.Request.Context(), c.Query("period"))
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"period": period, "data": list})
}

// Rollups — GET /rollups?office_id&service_id&from&to (YYYY-MM-DD · ว่าง = 30 วันล่าสุด)
func (h *OpsHandler) Rollups(c *gin.Context) {
	list, err := h.usage.Rollups(c.Request.Context(), c.Query("office_id"), c.Query("service_id"), c.Query("from"), c.Query("to"))
	if err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"data": list})
}

// CreateDeletion — POST /deletion-requests {office_id, service_id?, user?, from?, to? (RFC3339), reason}
func (h *OpsHandler) CreateDeletion(c *gin.Context) {
	var in service.DeletionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง (เวลาต้องเป็น RFC3339)", nil)
		return
	}
	req, err := h.deletion.Create(c.Request.Context(), actorOf(c), in)
	switch {
	case err != nil && req.ID == "":
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	case err != nil:
		// สร้างคำขอแล้วแต่ลบไม่ครบ — ส่งคำขอกลับไปให้เห็นว่าลบไปแล้วเท่าไหร่
		ResData(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error(), req)
	default:
		ResData(c, http.StatusOK, "SUCCESS", "", req)
	}
}

// Deletions — GET /deletion-requests?limit&offset
func (h *OpsHandler) Deletions(c *gin.Context) {
	list, total, err := h.deletion.List(c.Request.Context(), qInt(c, "limit", 50), qInt(c, "offset", 0))
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}

// AccessLog — GET /access-log?office_id&service_id&operator&limit&offset
func (h *OpsHandler) AccessLog(c *gin.Context) {
	list, total, err := h.deletion.AccessLog(c.Request.Context(), port.AccessLogFilter{
		OfficeID: c.Query("office_id"), ServiceID: c.Query("service_id"), Operator: c.Query("operator"),
		Limit: qInt(c, "limit", 50), Offset: qInt(c, "offset", 0),
	})
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": total, "data": list})
}
