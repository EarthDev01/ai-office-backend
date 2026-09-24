package routes

import (
	"errors"
	"net/http"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"github.com/gin-gonic/gin"
)

type OfficeHandler struct {
	svc port.OfficeService
}

func NewOfficeHandler(svc port.OfficeService) *OfficeHandler {
	return &OfficeHandler{svc: svc}
}

// actorOf อ่าน username ของ console user ที่ล็อกอินอยู่ (ตั้งโดย ConsoleAuth middleware)
// ใช้ ctxConsoleUsername ตัวเดียวกับที่ auth.go ใช้ — ประกาศ const ซ้ำที่นั่นเพราะ
// import gin (middleware) กลับมาที่ routes จะเกิด import cycle
func actorOf(c *gin.Context) string {
	return consoleUsername(c)
}

func (h *OfficeHandler) fail(c *gin.Context, err error) {
	var taken *domain.OriginTakenError
	if errors.As(err, &taken) {
		ResData(c, http.StatusConflict, "ORIGIN_TAKEN", taken.Error(), gin.H{
			"origin": taken.Origin, "office_id": taken.OfficeID, "office_label": taken.OfficeLabel,
		})
		return
	}
	switch err {
	case domain.ErrNotFound:
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบรายการนี้", nil)
	case domain.ErrConflict:
		ResData(c, http.StatusConflict, "CONFLICT", "id นี้ถูกใช้ไปแล้ว", nil)
	default:
		ResData(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	}
}

// ok ตอบ office แบบซ่อน hash ของ secret เสมอ (เหลือแค่สถานะ has_secret/has_prev_secret)
func ok(c *gin.Context, code int, o domain.Office) {
	ResData(c, code, "SUCCESS", "", o.Redacted())
}

func (h *OfficeHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	out := make([]domain.Office, len(list))
	for i, o := range list {
		out[i] = o.Redacted()
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": len(out), "data": out})
}

func (h *OfficeHandler) Get(c *gin.Context) {
	o, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

type createOfficeReq struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

func (h *OfficeHandler) Create(c *gin.Context) {
	var req createOfficeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, err)
		return
	}
	o, err := h.svc.Create(c.Request.Context(), req.ID, req.Label, req.Kind, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusCreated, o)
}

func (h *OfficeHandler) Patch(c *gin.Context) {
	var patch domain.UpdateOffice
	if err := c.ShouldBindJSON(&patch); err != nil {
		h.fail(c, err)
		return
	}
	o, err := h.svc.Update(c.Request.Context(), c.Param("id"), patch, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

func (h *OfficeHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
}

// RotateKey ทำให้ snippet เดิมของ office นี้ใช้ไม่ได้ทันที
// คอนโซลต้องถามยืนยันก่อนเรียก
func (h *OfficeHandler) RotateKey(c *gin.Context) {
	o, err := h.svc.RotateKey(c.Request.Context(), c.Param("id"), actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

type createServiceReq struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (h *OfficeHandler) AddService(c *gin.Context) {
	var req createServiceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, err)
		return
	}
	o, err := h.svc.AddService(c.Request.Context(), c.Param("id"), req.ID, req.Label, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusCreated, o)
}

func (h *OfficeHandler) PatchService(c *gin.Context) {
	var patch domain.UpdateService
	if err := c.ShouldBindJSON(&patch); err != nil {
		h.fail(c, err)
		return
	}
	o, err := h.svc.UpdateService(c.Request.Context(), c.Param("id"), c.Param("sid"), patch, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

func (h *OfficeHandler) RemoveService(c *gin.Context) {
	o, err := h.svc.RemoveService(c.Request.Context(), c.Param("id"), c.Param("sid"), actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

// ---- secret_key ต่อ service (R3) · ค่า secret แสดงครั้งเดียวในคำตอบนี้เท่านั้น ----

func (h *OfficeHandler) secretResult(c *gin.Context, secret string, o domain.Office, err error) {
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{
		"secret_key": secret,
		"note":       "แสดงครั้งเดียว — คัดลอกไปใส่ AI_SERVICE_SECRETS ฝั่ง server ของหลังบ้าน (K8s Secret) ห้ามใส่ในหน้าเว็บ/ไฟล์ที่ git track",
		"office":     o.Redacted(),
	})
}

func (h *OfficeHandler) IssueSecret(c *gin.Context) {
	s, o, err := h.svc.IssueSecret(c.Request.Context(), c.Param("id"), c.Param("sid"), actorOf(c))
	h.secretResult(c, s, o, err)
}

func (h *OfficeHandler) RotateSecret(c *gin.Context) {
	s, o, err := h.svc.RotateSecret(c.Request.Context(), c.Param("id"), c.Param("sid"), actorOf(c))
	h.secretResult(c, s, o, err)
}

func (h *OfficeHandler) CommitSecret(c *gin.Context) {
	o, err := h.svc.CommitSecret(c.Request.Context(), c.Param("id"), c.Param("sid"), actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

func (h *OfficeHandler) RevokeSecret(c *gin.Context) {
	o, err := h.svc.RevokeSecret(c.Request.Context(), c.Param("id"), c.Param("sid"), actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}

func (h *OfficeHandler) IncreaseQuota(c *gin.Context) {
	var b struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&b); err != nil {
		h.fail(c, err)
		return
	}
	o, err := h.svc.IncreaseQuota(c.Request.Context(), c.Param("id"), c.Param("sid"), b.Amount, b.Reason, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusOK, o)
}
