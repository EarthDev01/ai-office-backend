package routes

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type WidgetHandler struct {
	bundlePath string
}

func NewWidgetHandler(bundlePath string) *WidgetHandler {
	return &WidgetHandler{bundlePath: bundlePath}
}

// ServeBundle ส่ง widget bundle — ไฟล์เดียวกันทุก officeลูกค้า
//
// ไม่ตรวจว่าใครขอ: ไฟล์เป็น static ล้วน ไม่มีข้อมูลของใครอยู่ข้างใน และ <script src> ก็ไม่ส่ง
// Origin มาให้ตรวจอยู่แล้ว · ด่านจริงอยู่ที่ /bootstrap ซึ่งเป็น fetch ข้ามโดเมนจึงมี Origin เสมอ
// (path แบบเก่าที่มี key ก็มาที่นี่ — key ไม่มีความหมายแล้ว)
func (h *WidgetHandler) ServeBundle(c *gin.Context) {
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
