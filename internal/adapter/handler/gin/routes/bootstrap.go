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
//	office   มาจากโดเมนที่เรียกเข้ามา (ResolveOffice middleware)
//	service  มาจาก path เพราะหน้าเว็บเป็นคนรู้ว่าเปิด service ไหนอยู่
//	         (localStorage["web-service"]) — แล้วเราตรวจกับ Role.ListService ก่อนเสมอ
func (h *BootstrapHandler) Bootstrap(c *gin.Context, office domain.Office, caller domain.Caller) {
	b, err := h.svc.Bootstrap(c.Request.Context(), office, c.Param("service_id"), caller)

	switch err {
	case nil:
		ResData(c, http.StatusOK, "SUCCESS", "", b)
	case domain.ErrServiceNotAllowed:
		// บัญชีนี้ไม่มีสิทธิ์ใน service ที่หน้าเว็บอ้างมา — ไม่บอกว่ามี service นี้จริงหรือเปล่า
		ResData(c, http.StatusForbidden, "SERVICE_NOT_ALLOWED",
			"บัญชีนี้ไม่มีสิทธิ์ใน service ที่เลือกอยู่", nil)
	default:
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
	}
}
