package gin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"ai-office-backend/internal/adapter/handler/gin/routes"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"github.com/gin-gonic/gin"
)

const (
	CallerKey = "ai_office_caller"
	OfficeKey = "ai_office_office"
)

// คีย์ที่ห้าม client แอบยัดมาใน body/query/header
//
// ช่องทางเดียวที่ client บอก office/service ได้คือ "path" ซึ่งเป็นพารามิเตอร์ที่
// ประกาศไว้ชัดและถูกตรวจทุกครั้ง — ที่อื่นทั้งหมดถือว่าเป็นความพยายามเลี่ยงการตรวจ
//
// หมายเหตุ: service_id ไม่ใช่ "ค่าต้องห้าม" อีกต่อไป (หน้าเว็บเป็นคนรู้ว่าเปิด service ไหน
// อยู่ที่ localStorage["web-service"]) แต่ต้องมาทาง path เท่านั้น แล้วเราตรวจกับ
// Role.ListService ของแอดมินคนนั้นก่อนเสมอ — ดู backoffice-api-survey.md §3
var tenantKeys = map[string]bool{
	"service_id": true, "serviceid": true, "service": true,
	"website_id": true, "websiteid": true, "website": true,
	"business_id": true, "businessid": true,
	"tenant": true, "tenant_id": true, "tenantid": true,
	"office_id": true, "officeid": true,
}

// RejectTenantFields ปฏิเสธ request ที่ client พยายามบอกเองว่าอยู่เว็บไหน
//
// เลือก "reject" ไม่ใช่ "ignore" เพราะ ignore แปลว่ามีคนพยายามแล้วเราไม่รู้
// ดู 01-SPEC-SYSTEM.md §5 ชั้นที่ 1
func RejectTenantFields() gin.HandlerFunc {
	return func(c *gin.Context) {
		for k := range c.Request.URL.Query() {
			if tenantKeys[normKey(k)] {
				rejectTenant(c, "query:"+k)
				return
			}
		}
		for k := range c.Request.Header {
			if tenantKeys[normKey(strings.TrimPrefix(k, "X-"))] {
				rejectTenant(c, "header:"+k)
				return
			}
		}
		if c.Request.Body != nil {
			raw, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(raw)) // คืน body ให้ handler อ่านต่อได้
			if len(raw) > 0 {
				var body map[string]any
				if json.Unmarshal(raw, &body) == nil {
					if found, path := findTenantKey(body, ""); found {
						rejectTenant(c, "body:"+path)
						return
					}
				}
			}
		}
		c.Next()
	}
}

func normKey(k string) string {
	return strings.ToLower(strings.ReplaceAll(k, "-", "_"))
}

// findTenantKey เดินลงทุกชั้นของ JSON ไม่ใช่แค่ชั้นบนสุด
func findTenantKey(m map[string]any, prefix string) (bool, string) {
	for k, v := range m {
		if tenantKeys[normKey(k)] {
			return true, prefix + k
		}
		if child, ok := v.(map[string]any); ok {
			if found, p := findTenantKey(child, prefix+k+"."); found {
				return true, p
			}
		}
	}
	return false, ""
}

func rejectTenant(c *gin.Context, where string) {
	routes.ResData(c, http.StatusBadRequest, "UNEXPECTED_TENANT_FIELD",
		"office/service ต้องมาทาง path เท่านั้น — เจอที่ "+where, nil)
	c.Abort()
}

// ResolveOffice หา office จาก public_key ใน path แล้วแปะไว้ใน context
func ResolveOffice(svc port.OfficeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		o, err := svc.ResolveByPublicKey(c.Request.Context(), c.Param("public_key"))
		if err != nil {
			routes.ResData(c, http.StatusNotFound, "NOT_FOUND", "ไม่พบ office ของ key นี้", nil)
			c.Abort()
			return
		}
		c.Set(OfficeKey, o)
		c.Next()
	}
}

// ResolveCaller เอา Bearer token ไปถาม office-api-v10 ว่าเป็นใคร
//
// office-v10x เก็บ token ที่ localStorage["auth_token"] แล้วส่งเป็น Authorization: Bearer
// ไม่ใช่ cookie (backoffice-api-survey.md §2.1)
func ResolveCaller(r port.IdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		office := OfficeFrom(c)

		cred := domain.Credential{
			Token:     strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")),
			UserAgent: c.GetHeader("User-Agent"),
			ClientIP:  c.ClientIP(),
		}

		caller, err := r.Resolve(c.Request.Context(), office, cred)
		switch err {
		case nil:
		case domain.ErrNotAuthenticated:
			routes.ResData(c, http.StatusUnauthorized, "NOT_AUTHENTICATED", "ไม่พบ token", nil)
			c.Abort()
			return
		case domain.ErrSessionExpired:
			routes.ResData(c, http.StatusUnauthorized, "SESSION_EXPIRED", "token หมดอายุ", nil)
			c.Abort()
			return
		case domain.ErrUpstream:
			// AI ล่มต้องไม่ลากหลังบ้านลงไปด้วย และตรงกันข้ามก็เช่นกัน
			routes.ResData(c, http.StatusServiceUnavailable, "BACKOFFICE_UNAVAILABLE",
				"ตรวจสอบผู้ใช้กับหลังบ้านไม่ได้ชั่วคราว", nil)
			c.Abort()
			return
		default:
			routes.ResData(c, http.StatusUnauthorized, "NOT_AUTHENTICATED", err.Error(), nil)
			c.Abort()
			return
		}

		c.Set(CallerKey, caller)
		c.Next()
	}
}

func CallerFrom(c *gin.Context) domain.Caller {
	v, _ := c.Get(CallerKey)
	caller, _ := v.(domain.Caller)
	return caller
}

func OfficeFrom(c *gin.Context) domain.Office {
	v, _ := c.Get(OfficeKey)
	o, _ := v.(domain.Office)
	return o
}

// ConsoleAuth คุม /api/ai/admin/* ซึ่งอ่านข้อมูลข้ามทุกเว็บ
//
// ██ รอบนี้เป็น static token · ของจริงต้องเป็น JWT คนละ secret กับแอดมินเว็บ
// ██ เพราะแอดมินของเว็บต้องเข้าคอนโซลนี้ไม่ได้ (04-SPEC §7)
func ConsoleAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if got == "" || got != token {
			routes.ResData(c, http.StatusUnauthorized, "UNAUTHORIZED", "console token ไม่ถูกต้อง", nil)
			c.Abort()
			return
		}
		c.Set("console_actor", "console")
		c.Next()
	}
}

// CORS เปิดให้ origin ของคอนโซล (จาก env) + origin ของทุก office (จาก DB)
//
// ส่วนที่มาจาก DB ทำให้เพิ่มลูกค้าใหม่ได้โดยไม่ต้อง deploy
// อ่านใหม่ทุก request เพราะรายการเปลี่ยนได้ตลอดจากคอนโซล
func CORS(static []string, dynamic func() []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false
		if origin != "" {
			for _, o := range static {
				if o == origin {
					allowed = true
					break
				}
			}
			if !allowed && dynamic != nil {
				for _, o := range dynamic() {
					if o == origin {
						allowed = true
						break
					}
				}
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
