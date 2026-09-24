package gin

import (
	"context"
	"net/http"
	"strings"

	"ai-office-backend/internal/adapter/handler/gin/routes"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

type Deps struct {
	OfficeService port.OfficeService
	Connectors    port.ConnectorRegistry
	Tickets       port.AccessTicketIssuer
	Chat          *service.ChatService
	Admin         *service.AdminService
	Settings      *service.SettingsService
	Rollups       port.RollupRepository
	BundlePath    string
	Tokens        port.TokenIssuer
	Auth          *service.ConsoleAuth
	Permissions   *service.PermissionService
	ConsoleToken  string
	Audit         *service.AuditService // nil = ไม่บันทึก/ไม่เปิด endpoint ประวัติ
	// Metrics = handler ของ Prometheus (nil = ไม่เปิด /metrics) · MetricsToken ว่าง = ไม่ต้องใส่ token
	Metrics      http.Handler
	MetricsToken string
	// Ready = ตรวจ dependency (Mongo/Redis) สำหรับ /readyz · nil = ถือว่าพร้อม
	Ready func(ctx context.Context) error
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestMeta())

	// CORS เปิดทุก origin (ไม่ใช้ cookie) — ด่านจริงคือ origin ของ office ที่ตรวจใน handler/TicketAuth
	r.Use(CORS())

	r.GET("/healthz", func(c *gin.Context) {
		routes.ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		if d.Ready != nil {
			if err := d.Ready(c.Request.Context()); err != nil {
				routes.ResData(c, http.StatusServiceUnavailable, "NOT_READY", err.Error(), nil)
				return
			}
		}
		routes.ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"status": "ready"})
	})
	if d.Metrics != nil {
		r.GET("/metrics", func(c *gin.Context) {
			if d.MetricsToken != "" && strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ") != d.MetricsToken {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			d.Metrics.ServeHTTP(c.Writer, c.Request)
		})
	}

	// สิ่งเดียวที่ backend ส่งให้หน้า office คือไฟล์ js ไฟล์นี้
	// ไม่มี HTML ไหนออกจาก service นี้ — UI ของแชทถูกสร้างด้วย JS ในหน้า office เอง
	// ไฟล์เดียวกันทุก office · key ใน path เป็นแค่ป้าย (widget แกะไปใช้เรียก API)
	// "auto" = ให้ backend หา office จากโดเมนของหน้าเว็บ — snippet เดียวใช้ได้ทุกโดเมน (office-v10x)
	widget := routes.NewWidgetHandler(d.BundlePath)
	r.GET("/widget/v1/ai-office.js", widget.ServeBundle)
	r.GET("/widget/v1/:public_key/ai-office.js", widget.ServeBundle)

	w := &routes.WidgetAPI{Offices: d.OfficeService, Connectors: d.Connectors, ChatSvc: d.Chat}
	office := routes.NewOfficeHandler(d.OfficeService)

	// ---- ฝั่ง widget / host ----
	//
	//	public_key  ระบุว่า "หน้านี้เป็นของ office ไหน" (ไม่ใช่ความลับ)
	//	service_id  มาทาง path เท่านั้น · host เป็นผู้ยืนยันตอนออกตั๋ว · ตั๋วผูก office+service+user
	//
	// RejectTenantFields ปฏิเสธการยัด office/service ทาง body/query/header (AC-29)
	r.GET("/api/ai/office/:public_key/page-config", KeyFromOrigin(d.OfficeService), ResolveOffice(d.OfficeService), w.PageConfig)

	api := r.Group("/api/ai/office/:public_key/service/:service_id", RejectTenantFields())
	{
		// host → backend (server-to-server) ด้วย X-AI-Secret — contract-host-ai.md §2
		api.POST("/session", w.Session)
		// โหมด browser (connector mode: browser): widget ขอตั๋วเอง — ไม่ต้องแก้หลังบ้าน
		api.POST("/browser-session", KeyFromOrigin(d.OfficeService), w.BrowserSession)

		t := api.Group("", KeyFromOrigin(d.OfficeService), TicketAuth(d.Tickets, d.OfficeService))
		t.GET("/bootstrap", w.Bootstrap)
		t.POST("/chat", w.Chat)
		t.POST("/chat/relay/:id", w.ChatRelay)
		t.GET("/conversations", w.ListConversations)
		t.GET("/conversations/:id", w.GetConversation)
		t.POST("/conversations/:id/close", w.CloseConversation)
	}

	authHandler := routes.NewAuthHandler(d.Auth, d.Permissions)
	permHandler := routes.NewPermissionHandler(d.Permissions, d.Auth)
	adm := &routes.AdminHandler{Admin: d.Admin, Settings: d.Settings, Connectors: d.Connectors, Rollups: d.Rollups}

	// ---- ฝั่งคอนโซล : auth คนละชุดกับแอดมินเว็บ ----
	//
	// /auth/status, /auth/register, /auth/login, /auth/totp/verify ไม่ต้อง auth
	// (นี่คือทางเข้า login เอง) ส่วนที่เหลือทั้งหมดต้องผ่าน ConsoleAuth ก่อนเสมอ
	adminAuth := r.Group("/api/ai/admin/auth")
	{
		adminAuth.GET("/status", authHandler.Status)
		adminAuth.POST("/register", authHandler.Register)
		adminAuth.POST("/login", authHandler.Login)
		adminAuth.POST("/totp/verify", authHandler.VerifyTOTP)
	}

	admin := r.Group("/api/ai/admin", ConsoleAuth(d.Tokens, d.ConsoleToken))
	{
		// authenticated, any role
		admin.GET("/auth/me", authHandler.Me)
		admin.POST("/auth/change-password", authHandler.ChangePassword)
		admin.POST("/auth/logout", authHandler.Logout)

		// ส่ง *AuditService ที่เป็น nil เข้า interface ตรง ๆ จะได้ interface ที่ไม่ nil แล้ว panic ตอนเรียก
		var audit port.AuditRecorder = port.NopAudit{}
		if d.Audit != nil {
			audit = d.Audit
		}
		perm := func(p string) gin.HandlerFunc { return RequirePermission(p, d.Permissions, audit) }
		officeView := perm(domain.PermOfficeView)
		officeEdit := perm(domain.PermOfficeEdit)
		officeDelete := perm(domain.PermOfficeDelete)
		officeRotate := perm(domain.PermOfficeRotate)
		userManage := perm(domain.PermUserManage)

		admin.GET("/offices", officeView, office.List)
		admin.GET("/offices/:id", officeView, office.Get)
		admin.GET("/connectors", officeView, adm.ConnectorsList)

		officesWrite := admin.Group("/offices")
		{
			officesWrite.POST("", officeEdit, office.Create)
			officesWrite.PATCH("/:id", officeEdit, office.Patch)
			officesWrite.DELETE("/:id", officeDelete, office.Delete)
			officesWrite.POST("/:id/rotate-key", officeRotate, office.RotateKey)

			officesWrite.POST("/:id/services", officeEdit, office.AddService)
			officesWrite.PATCH("/:id/services/:sid", officeEdit, office.PatchService)
			officesWrite.DELETE("/:id/services/:sid", officeDelete, office.RemoveService)

			// secret_key ต่อ service (R3) — แสดงครั้งเดียว · หมุน 2 ใบ · ยกเลิกทันที
			sec := perm(domain.PermSecretManage)
			officesWrite.POST("/:id/services/:sid/secret", sec, office.IssueSecret)
			officesWrite.POST("/:id/services/:sid/secret/rotate", sec, office.RotateSecret)
			officesWrite.POST("/:id/services/:sid/secret/commit", sec, office.CommitSecret)
			officesWrite.POST("/:id/services/:sid/secret/revoke", sec, office.RevokeSecret)

			officesWrite.POST("/:id/services/:sid/quota/increase", perm(domain.PermQuotaManage), office.IncreaseQuota)
		}

		// ประวัติ/ตรวจคำตอบ — อ่านเนื้อความ = access_log ทุกครั้ง (AC-11)
		read := perm(domain.PermConversationRead)
		admin.GET("/conversations", read, adm.Conversations)
		admin.GET("/conversations/:id", read, adm.Conversation)
		admin.GET("/messages", read, adm.Messages)
		admin.GET("/messages/:id", read, adm.Message)

		verify := perm(domain.PermVerificationWrite)
		admin.GET("/verifications/queue", verify, adm.VerificationQueue)
		admin.POST("/verifications", verify, adm.Verify)
		admin.GET("/verifications/stats", perm(domain.PermQuotaView), adm.VerificationStats)

		admin.GET("/quotas", perm(domain.PermQuotaView), adm.Quotas)
		admin.GET("/rollups", perm(domain.PermQuotaView), adm.RollupsList)
		admin.GET("/access-log", perm(domain.PermAccessLogView), adm.AccessLog)

		del := perm(domain.PermDeletionManage)
		admin.POST("/deletion-requests", del, adm.CreateDeletion)
		admin.GET("/deletion-requests", del, adm.Deletions)

		admin.GET("/settings", officeView, adm.GetSettings)
		admin.PATCH("/settings", perm(domain.PermSettingsManage), adm.PatchSettings)

		// จัดการ console user + ตั้ง role→permission matrix ต้องมี user.manage เท่านั้น
		users := admin.Group("/users", userManage)
		{
			users.GET("", authHandler.ListUsers)
			users.POST("", authHandler.CreateUser)
			users.PATCH("/:id", authHandler.PatchUser)
			users.POST("/:id/reset-password", authHandler.ResetPassword)
			users.POST("/:id/reset-2fa", authHandler.Reset2FA)
			users.DELETE("/:id", authHandler.DeleteUser)
		}

		rolePerms := admin.Group("/role-permissions", userManage)
		{
			rolePerms.GET("", permHandler.Get)
			rolePerms.PUT("", permHandler.Put)
		}

		// role ทั้งชุดเป็น list ที่แอดมินแก้ได้ (เพิ่ม/เปลี่ยนชื่อ/ลบ) — ดู service/permission.go
		roles := admin.Group("/roles", userManage)
		{
			roles.POST("", permHandler.AddRole)
			roles.PATCH("/:key", permHandler.RenameRole)
			roles.DELETE("/:key", permHandler.DeleteRole)
		}

		// ประวัติการทำงานของผู้ใช้คอนโซล — อ่านอย่างเดียว
		if d.Audit != nil {
			auditHandler := routes.NewAuditHandler(d.Audit)
			logs := admin.Group("/audit-logs", perm(domain.PermAuditView))
			{
				logs.GET("", auditHandler.List)
				logs.GET("/actors", auditHandler.Actors)
			}
		}
	}

	return r
}

// NewTestRouter ใช้ใน test — gin.TestMode ไม่พิมพ์ log รก
func NewTestRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(d)
}
