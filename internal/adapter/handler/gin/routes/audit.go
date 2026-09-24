package routes

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// AuditHandler เปิดให้อ่านประวัติการทำงานอย่างเดียว — ไม่มี endpoint แก้/ลบโดยตั้งใจ
type AuditHandler struct {
	audit *service.AuditService
}

func NewAuditHandler(audit *service.AuditService) *AuditHandler {
	return &AuditHandler{audit: audit}
}

// GET /audit-logs?actor=&category=&action=&status=&target_type=&target_id=&q=&from=&to=&page=&page_size=
//
// from/to เป็น RFC3339 (to ไม่รวมตัวเอง) · ไม่ส่ง from = ย้อนหลัง 30 วัน · กว้างสุด 62 วัน
// page เริ่มที่ 1 · page_size สูงสุด 200 · total นับถึง 10,000 (เกินนี้ total_capped = true)
func (h *AuditHandler) List(c *gin.Context) {
	f := domain.AuditFilter{
		Actor:      c.Query("actor"),
		Category:   c.Query("category"),
		Action:     c.Query("action"),
		Status:     c.Query("status"),
		TargetType: c.Query("target_type"),
		TargetID:   c.Query("target_id"),
		Q:          c.Query("q"),
	}
	var ok bool
	if f.From, ok = parseTimeParam(c, "from"); !ok {
		return
	}
	if f.To, ok = parseTimeParam(c, "to"); !ok {
		return
	}
	f.Page, _ = strconv.Atoi(c.Query("page"))
	f.PageSize, _ = strconv.Atoi(c.Query("page_size"))
	f.Normalize()

	page, err := h.audit.Query(c.Request.Context(), f)
	switch {
	case errors.Is(err, domain.ErrAuditRangeTooWide):
		ResData(c, http.StatusBadRequest, "AUDIT_RANGE_TOO_WIDE", "ช่วงวันที่กว้างเกิน 2 เดือน", nil)
		return
	case errors.Is(err, domain.ErrAuditRangeInvalid):
		ResData(c, http.StatusBadRequest, "AUDIT_RANGE_INVALID", "วันเริ่มต้องมาก่อนวันสิ้นสุด", nil)
		return
	case err != nil:
		ResData(c, http.StatusInternalServerError, "INTERNAL", "อ่านประวัติไม่สำเร็จ", nil)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{
		"items":        page.Items,
		"total":        page.Total,
		"total_capped": page.TotalCapped,
		"page":         f.Page,
		"page_size":    f.PageSize,
	})
}

// GET /audit-logs/actors — รายชื่อผู้กระทำทั้งหมดที่เคยมีในประวัติ (ไว้ทำตัวกรอง)
//
// ไม่ใช้ /users เพราะต้องมี user.manage และจะไม่เห็นคนที่ถูกลบไปแล้ว
func (h *AuditHandler) Actors(c *gin.Context) {
	actors, err := h.audit.Actors(c.Request.Context())
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL", "อ่านรายชื่อไม่สำเร็จ", nil)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{"actors": actors})
}

func parseTimeParam(c *gin.Context, key string) (time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		ResData(c, http.StatusBadRequest, "ข้อมูลไม่ถูกต้อง", "BAD_REQUEST", gin.H{"field": key})
		return time.Time{}, false
	}
	return t, true
}
