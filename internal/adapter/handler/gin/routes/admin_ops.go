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
	llm      port.LLMRouter
	keys     *service.LLMKeyService
}

func NewOpsHandler(settings *service.SettingsService, usage *service.UsageService, deletion *service.DeletionService,
	llm port.LLMRouter, keys *service.LLMKeyService) *OpsHandler {
	return &OpsHandler{settings: settings, usage: usage, deletion: deletion, llm: llm, keys: keys}
}

// GetSettings — GET /settings
func (h *OpsHandler) GetSettings(c *gin.Context) {
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{
		"settings":      h.settings.Get(c.Request.Context()),
		"ranges":        domain.SettingRanges(),
		"llm_providers": h.llmProviders(),
		"llm_ready":     h.llm != nil && h.llm.Ready(),
		"llm_key_store": h.keys != nil && h.keys.Editable(), // false = ยังไม่ตั้ง LLM_KEY_SECRET ตั้ง key จากหน้าเว็บไม่ได้
	})
}

// llmProvider คือ provider ในแคตตาล็อก + สถานะ key (มีไหม · 4 ตัวท้าย · ใครตั้ง) — ไม่มีตัว key
type llmProvider struct {
	domain.LLMProviderInfo
	HasKey bool              `json:"has_key"`
	Key    domain.LLMKeyInfo `json:"key"`
}

func (h *OpsHandler) llmProviders() []llmProvider {
	out := make([]llmProvider, 0, len(domain.LLMProviderCatalog))
	for _, p := range domain.LLMProviderCatalog {
		item := llmProvider{LLMProviderInfo: p, HasKey: h.llm != nil && h.llm.HasKey(p.ID)}
		if h.keys != nil {
			item.Key = h.keys.Info(p.ID)
		}
		out = append(out, item)
	}
	return out
}

// SetLLMKey — PUT /settings/llm/keys/:provider {key, code, model?, base_url?}
func (h *OpsHandler) SetLLMKey(c *gin.Context) {
	if h.keys == nil {
		ResData(c, http.StatusServiceUnavailable, "LLM_KEY_STORE_DISABLED", "ระบบนี้ไม่ได้เปิดการจัดการ key", nil)
		return
	}
	var body struct {
		Key     string `json:"key"`
		Code    string `json:"code"`
		Model   string `json:"model"`
		BaseURL string `json:"base_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง", nil)
		return
	}
	info, err := h.keys.Set(c.Request.Context(), service.SetLLMKey{
		Provider: c.Param("provider"), Key: body.Key, Code: body.Code, Model: body.Model, BaseURL: body.BaseURL,
	}, service.KeyActor{ID: consoleUserID(c), Username: consoleUsername(c)})
	if err != nil {
		h.keyFail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", info)
}

// DeleteLLMKey — DELETE /settings/llm/keys/:provider {code}
func (h *OpsHandler) DeleteLLMKey(c *gin.Context) {
	if h.keys == nil {
		ResData(c, http.StatusServiceUnavailable, "LLM_KEY_STORE_DISABLED", "ระบบนี้ไม่ได้เปิดการจัดการ key", nil)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&body)
	err := h.keys.Delete(c.Request.Context(), c.Param("provider"), body.Code,
		service.KeyActor{ID: consoleUserID(c), Username: consoleUsername(c)})
	if err != nil {
		h.keyFail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
}

// keyFail แปลง error เป็นคำตอบ — ข้อความผ่าน Redact เสมอ กัน key หลุดจาก error ของ provider
func (h *OpsHandler) keyFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrKeyStoreDisabled):
		ResData(c, http.StatusServiceUnavailable, "LLM_KEY_STORE_DISABLED",
			"ยังไม่ได้ตั้ง LLM_KEY_SECRET ใน .env ของหลังบ้าน ai — ตั้งแล้ว restart ก่อน", nil)
	case errors.Is(err, service.ErrStepUpRequired):
		ResData(c, http.StatusForbidden, "STEP_UP_REQUIRED", "ต้องล็อกอินด้วยบัญชีจริงที่ตั้ง 2FA แล้ว", nil)
	case errors.Is(err, service.ErrInvalid2FA):
		// 400 ไม่ใช่ 401 — คอนโซลถือว่า 401 = session หมด แล้วเด้งออกจากระบบ
		ResData(c, http.StatusBadRequest, "INVALID_2FA", "รหัส 2FA ไม่ถูกต้อง", nil)
	case errors.Is(err, service.ErrAccountLocked):
		ResData(c, http.StatusLocked, "ACCOUNT_LOCKED", "ใส่รหัส 2FA ผิดหลายครั้ง — บัญชีถูกล็อก 15 นาที", nil)
	case errors.Is(err, service.ErrAccountDisabled):
		ResData(c, http.StatusForbidden, "ACCOUNT_DISABLED", "บัญชีถูกปิดใช้งาน", nil)
	case errors.Is(err, service.ErrKeyInUse):
		ResData(c, http.StatusConflict, "LLM_KEY_IN_USE", "แชทใช้ผู้ให้บริการนี้อยู่ — เปลี่ยนโมเดลไปผู้ให้บริการอื่นก่อนลบ key", nil)
	default:
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", h.keys.Redact(err.Error()), nil)
	}
}

// TestLLM — POST /settings/llm/test ยิงข้อความสั้น 1 ครั้งด้วยค่าที่กรอก (ยังไม่บันทึก)
func (h *OpsHandler) TestLLM(c *gin.Context) {
	if h.llm == nil {
		ResData(c, http.StatusServiceUnavailable, "LLM_NOT_CONFIGURED", "ระบบนี้ไม่ได้เปิดตัวเลือกโมเดล", nil)
		return
	}
	var cfg domain.LLMSettings
	if err := c.ShouldBindJSON(&cfg); err != nil {
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง", nil)
		return
	}
	res, err := h.llm.Test(c.Request.Context(), cfg)
	if err != nil {
		msg := err.Error()
		if h.keys != nil {
			msg = h.keys.Redact(msg)
		}
		if rs := []rune(msg); len(rs) > 300 {
			msg = string(rs[:300]) + "…"
		}
		ResData(c, http.StatusBadRequest, "LLM_TEST_FAILED", "เชื่อมต่อไม่สำเร็จ: "+msg, nil)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", res)
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
