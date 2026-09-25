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

func (h *OfficeHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"total": len(list), "data": list})
}

func (h *OfficeHandler) Get(c *gin.Context) {
	o, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", o)
}

// Create — POST /offices {id, label, group_id, origin}
func (h *OfficeHandler) Create(c *gin.Context) {
	var req domain.CreateOffice
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, err)
		return
	}
	o, err := h.svc.Create(c.Request.Context(), req, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusCreated, "SUCCESS", "", o)
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
	ResData(c, http.StatusOK, "SUCCESS", "", o)
}

func (h *OfficeHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
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
	ResData(c, http.StatusCreated, "SUCCESS", "", o)
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
	ResData(c, http.StatusOK, "SUCCESS", "", o)
}

func (h *OfficeHandler) RemoveService(c *gin.Context) {
	o, err := h.svc.RemoveService(c.Request.Context(), c.Param("id"), c.Param("sid"), actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", o)
}

// ---- กลุ่มของ domain ----

func (h *OfficeHandler) Groups(c *gin.Context) {
	list, err := h.svc.ListGroups(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"data": list})
}

type groupReq struct {
	Name string `json:"name"`
}

func (h *OfficeHandler) CreateGroup(c *gin.Context) {
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, err)
		return
	}
	g, err := h.svc.CreateGroup(c.Request.Context(), req.Name, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusCreated, "SUCCESS", "", g)
}

func (h *OfficeHandler) RenameGroup(c *gin.Context) {
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, err)
		return
	}
	g, err := h.svc.RenameGroup(c.Request.Context(), c.Param("gid"), req.Name, actorOf(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", g)
}

func (h *OfficeHandler) DeleteGroup(c *gin.Context) {
	if err := h.svc.DeleteGroup(c.Request.Context(), c.Param("gid"), actorOf(c)); err != nil {
		h.fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "SUCCESS", "", nil)
}
