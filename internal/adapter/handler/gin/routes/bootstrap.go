package routes

import (
	"net/http"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"github.com/gin-gonic/gin"
)

type BootstrapHandler struct {
	svc port.OfficeService
}

func NewBootstrapHandler(svc port.OfficeService) *BootstrapHandler {
	return &BootstrapHandler{svc: svc}
}

// Bootstrap — widget ถามว่า "ฉันควรโผล่ไหม และหน้าตาเป็นยังไง"
//
//	office   มาจาก public_key ใน path (ตัวที่อยู่ใน snippet)
//	service  มาจาก path เช่นกัน เพราะหน้าเว็บเป็นคนรู้ว่าเปิด service ไหนอยู่
//	         (localStorage["web-service"]) — แล้วเราตรวจกับ Role.ListService ก่อนเสมอ
func (h *BootstrapHandler) Bootstrap(c *gin.Context, caller domain.Caller) {
	b, err := h.svc.Bootstrap(
		c.Request.Context(),
		c.Param("public_key"),
		c.Param("service_id"),
		c.GetHeader("Origin"),
		caller,
	)

	switch err {
	case nil:
		ResData(c, http.StatusOK, "SUCCESS", "", b)
	case domain.ErrNotFound:
		ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบ office ของ key นี้", nil)
	case domain.ErrOriginNotAllowed:
		ResData(c, http.StatusForbidden, "ORIGIN_NOT_ALLOWED",
			"โดเมนนี้ไม่ได้ลงทะเบียนไว้กับ key นี้", nil)
	case domain.ErrServiceNotAllowed:
		// บัญชีนี้ไม่มีสิทธิ์ใน service ที่หน้าเว็บอ้างมา — ไม่บอกว่ามี service นี้จริงหรือเปล่า
		ResData(c, http.StatusForbidden, "SERVICE_NOT_ALLOWED",
			"บัญชีนี้ไม่มีสิทธิ์ใน service ที่เลือกอยู่", nil)
	default:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
	}
}
