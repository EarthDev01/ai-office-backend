package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-office-backend/internal/adapter/handler/gin/routes"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

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
// Role.ListService ของแอดมินคนนั้นก่อนเสมอ
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

// ResolveOffice หา office จากโดเมนที่เรียกเข้ามา (header Origin) แล้วแปะไว้ใน context
//
// officeลูกค้า อย่าง office-v10x ใช้ snippet เดียวกันทุกโดเมน โดเมนจึงเป็นตัวบอกว่าเป็นลูกค้าเจ้าไหน
// ต้องอยู่ก่อน ResolveCaller เพราะตัวตนที่อ่านได้ถูกผูกกับ office นี้ (caller.OfficeID)
func ResolveOffice(svc port.OfficeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		o, err := svc.ResolveByOrigin(c.Request.Context(), c.GetHeader("Origin"))
		switch err {
		case nil:
		case domain.ErrOriginRequired:
			routes.ResData(c, http.StatusForbidden, "ORIGIN_REQUIRED",
				"ไม่พบ Origin — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า", nil)
			c.Abort()
			return
		case domain.ErrOriginNotAllowed:
			routes.ResData(c, http.StatusForbidden, "ORIGIN_NOT_REGISTERED",
				"โดเมนนี้ยังไม่ได้ลงทะเบียนกับ office ไหน — เพิ่มที่ 'โดเมนที่อนุญาต' ในคอนโซล", nil)
			c.Abort()
			return
		default:
			routes.ResData(c, http.StatusInternalServerError, "INTERNAL_ERROR", "หา office ไม่สำเร็จ", nil)
			c.Abort()
			return
		}
		c.Set(OfficeKey, o)
		c.Next()
	}
}

// ResolveCaller อ่าน Bearer token ของหน้า office ว่าเป็นใคร
func ResolveCaller(r port.IdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		office := OfficeFrom(c)

		cred := domain.Credential{
			Token: strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")),
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

const (
	ConsoleUserIDKey   = "console_user_id"
	ConsoleUsernameKey = "console_username"
	ConsoleRoleKey     = "console_role"
)

// ConsoleAuth คุม /api/ai/admin/* ซึ่งอ่านข้อมูลข้ามทุกเว็บ
//
// ตรวจด้วย JWT (คนละ secret กับแอดมินเว็บ เพราะแอดมินของเว็บต้องเข้าคอนโซลนี้ไม่ได้
// — 04-SPEC §7) ยกเว้น breakGlass token (ถ้าตั้งไว้) ที่ผ่านได้ทันทีในฐานะ admin
// ██ ห้าม log ทั้ง token และ breakGlass
func ConsoleAuth(tokens port.TokenIssuer, breakGlass string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if got == "" {
			routes.ResData(c, http.StatusUnauthorized, "UNAUTHORIZED", "ไม่พบ token", nil)
			c.Abort()
			return
		}

		if breakGlass != "" && got == breakGlass {
			c.Set(ConsoleRoleKey, domain.RoleAdmin)
			c.Set(ConsoleUsernameKey, "break-glass")
			c.Set(ConsoleUserIDKey, "")
			setAuditActor(c, domain.AuditActor{Username: "break-glass", Role: domain.RoleAdmin})
			c.Next()
			return
		}

		claims, err := tokens.Verify(got)
		if err != nil {
			routes.ResData(c, http.StatusUnauthorized, "UNAUTHORIZED", "console token ไม่ถูกต้อง", nil)
			c.Abort()
			return
		}

		c.Set(ConsoleUserIDKey, claims.UserID)
		c.Set(ConsoleUsernameKey, claims.Username)
		c.Set(ConsoleRoleKey, claims.Role)
		setAuditActor(c, domain.AuditActor{ID: claims.UserID, Username: claims.Username, Role: claims.Role})
		c.Next()
	}
}

// setAuditActor ใส่ผู้ใช้ที่ล็อกอินอยู่ลงใน request context ให้ service ชั้นล่างบันทึกประวัติได้เอง
func setAuditActor(c *gin.Context, a domain.AuditActor) {
	c.Request = c.Request.WithContext(domain.WithAuditActor(c.Request.Context(), a))
}

// RequestMeta เก็บ ip / user-agent / method / path ไว้ใน request context สำหรับประวัติการทำงาน
//
// ip มาจาก c.ClientIP() — ถ้า deploy หลัง reverse proxy ต้องตั้ง gin trusted proxies ให้ถูก
// ไม่งั้น client ปลอม X-Forwarded-For มาได้
func RequestMeta() gin.HandlerFunc {
	return func(c *gin.Context) {
		ua := c.Request.UserAgent()
		if len(ua) > 300 {
			ua = ua[:300]
		}
		c.Request = c.Request.WithContext(domain.WithRequestMeta(c.Request.Context(), domain.RequestMeta{
			IP:        c.ClientIP(),
			UserAgent: ua,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
		}))
		c.Next()
	}
}

// RequireRole ปฏิเสธ request ที่ role ต่ำกว่า min (ต้องมาหลัง ConsoleAuth เสมอ)
func RequireRole(min domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := ConsoleRoleFrom(c)
		if !role.AtLeast(min) {
			routes.ResData(c, http.StatusForbidden, "FORBIDDEN", "สิทธิ์ไม่พอ", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePermission ปฏิเสธ request ที่ role ปัจจุบันไม่มี permission key นี้ตาม RoleMatrix
// (ต้องมาหลัง ConsoleAuth เสมอ)
//
// break-glass เข้ามาเป็น role admin เสมอ (ดู ConsoleAuth) — admin ทุกคนจึงผ่านด่านนี้ได้ตรง ๆ
// โดยไม่ต้องเช็ค matrix เลย กันไม่ให้ config ผิดพลาดล็อกทางเข้าตั้งค่าตัวเอง (anti-lockout)
//
// ถูกปฏิเสธเมื่อไรจะบันทึก access.denied ไว้ด้วย — คนที่พยายามเข้าส่วนที่ไม่มีสิทธิ์ต้องเห็นในประวัติ
func RequirePermission(perm string, perms *service.PermissionService, audit port.AuditRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := ConsoleRoleFrom(c)
		if role == domain.RoleAdmin {
			c.Next()
			return
		}
		ok, err := perms.Can(c.Request.Context(), role, perm)
		if err != nil {
			routes.ResData(c, http.StatusInternalServerError, "INTERNAL", "ตรวจสิทธิ์ไม่สำเร็จ", nil)
			c.Abort()
			return
		}
		if !ok {
			if audit != nil {
				audit.Record(c.Request.Context(), domain.AuditEntry{
					Action: domain.AuditAccessDenied, Status: domain.AuditFailure, Reason: "FORBIDDEN",
					Summary: fmt.Sprintf("ถูกปฏิเสธ: ไม่มีสิทธิ์ %s (%s %s)", perm, c.Request.Method, c.Request.URL.Path),
					Meta:    map[string]string{"permission": perm},
				})
			}
			routes.ResData(c, http.StatusForbidden, "FORBIDDEN", "ไม่มีสิทธิ์ทำรายการนี้", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// ConsoleRoleFrom อ่าน role ของ console user จาก context (ต้องผ่าน ConsoleAuth มาก่อน)
func ConsoleRoleFrom(c *gin.Context) domain.Role {
	v, _ := c.Get(ConsoleRoleKey)
	role, _ := v.(domain.Role)
	return role
}

// CORS เปิดทุก origin — ตอบ Access-Control-Allow-Origin: *
// ให้ทุกคำขอ ไม่เช็ค allowlist
//
// ไม่ตั้ง Allow-Credentials: true เพราะเบราว์เซอร์ห้ามใช้คู่กับ "*" · คอนโซลส่ง
// token ผ่าน Authorization header (Bearer) อยู่แล้ว ไม่ได้พึ่ง cookie
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
