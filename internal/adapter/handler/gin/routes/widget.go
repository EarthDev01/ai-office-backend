package routes

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"github.com/gin-gonic/gin"
)

type WidgetHandler struct {
	bundlePath string
	offices    port.OfficeService
}

func NewWidgetHandler(bundlePath string, offices port.OfficeService) *WidgetHandler {
	return &WidgetHandler{bundlePath: bundlePath, offices: offices}
}

// ServeBundle ส่ง widget bundle ของ office ที่ key ชี้ถึง
//
// ตัวไฟล์เหมือนกันทุก office (ไม่มีข้อมูลใครอยู่ข้างใน) แต่บังคับให้ key ต้องมีจริง
// เพื่อให้ snippet ที่ยังแปะค้างอยู่หยุดทำงานทันทีเมื่อลบ office หรือ rotate key
//
// ██ ตรงนี้ตรวจ Origin ไม่ได้ เพราะ <script src> ไม่ส่ง Origin header มา
// ██ ด่านจริงอยู่ที่ /bootstrap ซึ่งเป็น fetch ข้าม origin จึงมี Origin เสมอ
// ██ ไฟล์นี้เป็น static ล้วน ไม่มีข้อมูลของใครให้รั่ว
func (h *WidgetHandler) ServeBundle(c *gin.Context) {
	key := c.Param("public_key")
	if _, err := h.offices.ResolveByPublicKey(c.Request.Context(), key); err != nil {
		if err == domain.ErrNotFound {
			ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบ office ของ key นี้", nil)
			return
		}
		ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	b, err := os.ReadFile(h.bundlePath)
	if err != nil {
		ResData(c, http.StatusNotFound, "NOT_FOUND",
			"ยังไม่ได้ build widget — รัน: cd widget && npm install && npm run build", nil)
		return
	}

	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(b))
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", b)
}
