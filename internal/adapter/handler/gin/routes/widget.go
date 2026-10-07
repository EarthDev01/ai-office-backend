package routes

import (
	"crypto/sha256"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"ai-office-backend/internal/core/domain"
)

type WidgetHandler struct {
	bundlePath string
}

func NewWidgetHandler(bundlePath string) *WidgetHandler {
	return &WidgetHandler{bundlePath: bundlePath}
}

// assetsDir = คลังรูปของ widget (รูปผู้ช่วย · พื้นหลัง) อยู่ข้างไฟล์ bundle: static/widget/assets/<โฟลเดอร์>/<ไฟล์>
func (h *WidgetHandler) assetsDir() string {
	return filepath.Join(filepath.Dir(h.bundlePath), "assets")
}

var assetTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif", ".svg": "image/svg+xml",
}

// ServeAsset — GET /widget/v1/assets/<โฟลเดอร์>/<ไฟล์> · เฉพาะไฟล์รูปในคลัง ไม่มีรายการไฟล์ ไม่ออกนอกโฟลเดอร์
func (h *WidgetHandler) ServeAsset(c *gin.Context) {
	name := strings.TrimPrefix(c.Param("path"), "/")
	if domain.ValidateLookValue("asset:"+name, false) != nil {
		c.Status(http.StatusNotFound)
		return
	}
	b, err := os.ReadFile(filepath.Join(h.assetsDir(), filepath.FromSlash(name)))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Content-Type-Options", "nosniff")
	if strings.HasSuffix(name, ".svg") {
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}
	c.Data(http.StatusOK, assetTypes[strings.ToLower(filepath.Ext(name))], b)
}

// ListAssets — คลังรูปให้คอนโซลเลือก: {avatars: [asset:avatars/…], backgrounds: [asset:backgrounds/…], patterns: [pattern:…]}
func (h *WidgetHandler) ListAssets(c *gin.Context) {
	ResData(c, http.StatusOK, "SUCCESS", "", gin.H{
		"avatars": h.assetList("avatars"), "backgrounds": h.assetList("backgrounds"), "patterns": patternValues(),
		"launchers": h.assetList("launchers"),
	})
}

func (h *WidgetHandler) assetList(dir string) []string {
	out := []string{}
	entries, _ := os.ReadDir(filepath.Join(h.assetsDir(), dir))
	for _, e := range entries {
		name := dir + "/" + e.Name()
		if !e.IsDir() && domain.ValidateLookValue("asset:"+name, false) == nil {
			out = append(out, "asset:"+name)
		}
	}
	sort.Strings(out)
	return out
}

func patternValues() []string {
	out := make([]string, 0, len(domain.WidgetPatterns))
	for _, p := range domain.WidgetPatterns {
		out = append(out, "pattern:"+p)
	}
	return out
}

// ResolveRandom แปลงค่า "random" เป็นรูปจริงจากคลัง (สุ่มใหม่ทุกครั้งที่ widget โหลด) · คลังว่าง = ”
func (h *WidgetHandler) ResolveRandom(b *domain.Bootstrap) {
	pick := func(pool []string) string {
		if len(pool) == 0 {
			return ""
		}
		return pool[rand.IntN(len(pool))]
	}
	if b.AvatarURL == domain.LookRandom {
		b.AvatarURL = pick(h.assetList("avatars"))
	}
	if b.Background == domain.LookRandom {
		b.Background = pick(append(patternValues(), h.assetList("backgrounds")...))
	}
	// ปุ่มเปิดแชทเป็นฟองแชทเสมอ — ไม่ได้เลือก = รูปแรกในคลัง · คลังว่าง = widget ใช้ไอคอนในตัว
	if b.LauncherIcon == "" {
		if l := h.assetList("launchers"); len(l) > 0 {
			b.LauncherIcon = l[0]
		}
	}
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
