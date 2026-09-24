package gin

import (
	"net/http"

	"ai-office-backend/internal/adapter/handler/gin/routes"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

type Deps struct {
	OfficeService port.OfficeService
	Identity      port.IdentityResolver
	BundlePath    string
	Tokens        port.TokenIssuer
	Auth          *service.ConsoleAuth
	Permissions   *service.PermissionService
	Audit         *service.AuditService // nil = ไม่บันทึก/ไม่เปิด endpoint ประวัติ
	ConsoleToken  string
	LLM           port.LLMClient // nil = ปิดแชท
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestMeta())

	// CORS เปิดทุก origin
	r.Use(CORS())

	r.GET("/healthz", func(c *gin.Context) {
		routes.ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"status": "ok"})
	})

	// สิ่งเดียวที่ backend ส่งให้หน้า office คือไฟล์ js ไฟล์นี้
	// ไม่มี HTML ไหนออกจาก service นี้ — UI ของแชทถูกสร้างด้วย JS ในหน้า office เอง
	// snippet ชุดเดียวใช้ได้ทุกโดเมน · แบบเก่าที่มี key ยังเสิร์ฟไฟล์เดียวกัน (ไม่สน key แล้ว)
	widget := routes.NewWidgetHandler(d.BundlePath)
	r.GET("/widget/v1/ai-office.js", widget.ServeBundle)
	r.GET("/widget/v1/:legacy_key/ai-office.js", widget.ServeBundle)

	boot := routes.NewBootstrapHandler(d.OfficeService)
	office := routes.NewOfficeHandler(d.OfficeService)

	// ---- ฝั่ง widget ----
	//
	//	Origin      ระบุว่า "เป็น officeลูกค้า เจ้าไหน" (ResolveOffice) — snippet เหมือนกันทุกโดเมน
	//	service_id  ระบุว่า "แอดมินกำลังเปิด service ไหน" — หน้าเว็บเป็นคนรู้ (localStorage)
	//	            แต่ต้องผ่านการตรวจกับ Role.ListService ของแอดมินก่อนเสมอ
	//
	// service มาทาง path เท่านั้น · RejectTenantFields ยังกันการยัดค่าเหล่านี้ทาง body/query/header
	// API ของ widget ในอนาคต (แชท ฯลฯ) ใส่ใต้ group นี้ทั้งหมด — ได้ office+service ที่ตรวจแล้วเสมอ
	widgetAPI := []gin.HandlerFunc{
		RejectTenantFields(),
		ResolveOffice(d.OfficeService),
		ResolveCaller(d.Identity),
	}
	bootstrap := func(c *gin.Context) { boot.Bootstrap(c, OfficeFrom(c), CallerFrom(c)) }

	chatHandler := routes.NewChatHandler(d.OfficeService, d.LLM)
	chat := func(c *gin.Context) { chatHandler.Chat(c, OfficeFrom(c), CallerFrom(c)) }

	api := r.Group("/api/ai/widget/service/:service_id", widgetAPI...)
	{
		api.GET("/bootstrap", bootstrap)
		api.POST("/chat", chat)
	}
	// ██ path เก่าที่มี key (snippet รุ่นก่อน) — key ไม่ถูกใช้แล้ว หา office จาก Origin เหมือนกัน
	// ลบได้เมื่อไม่มี officeลูกค้า ไหนใช้ snippet เก่าแล้ว
	legacy := r.Group("/api/ai/office/:legacy_key/service/:service_id", widgetAPI...)
	{
		legacy.GET("/bootstrap", bootstrap)
	}

	authHandler := routes.NewAuthHandler(d.Auth, d.Permissions)
	permHandler := routes.NewPermissionHandler(d.Permissions, d.Auth)

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

	// ส่ง *AuditService ที่เป็น nil เข้า interface ตรง ๆ จะได้ interface ที่ไม่ nil แล้ว panic ตอนเรียก
	var audit port.AuditRecorder = port.NopAudit{}
	if d.Audit != nil {
		audit = d.Audit
	}

	admin := r.Group("/api/ai/admin", ConsoleAuth(d.Tokens, d.ConsoleToken))
	{
		// authenticated, any role
		admin.GET("/auth/me", authHandler.Me)
		admin.POST("/auth/change-password", authHandler.ChangePassword)
		admin.POST("/auth/logout", authHandler.Logout)

		officeView := RequirePermission(domain.PermOfficeView, d.Permissions, audit)
		officeEdit := RequirePermission(domain.PermOfficeEdit, d.Permissions, audit)
		officeDelete := RequirePermission(domain.PermOfficeDelete, d.Permissions, audit)
		userManage := RequirePermission(domain.PermUserManage, d.Permissions, audit)
		auditView := RequirePermission(domain.PermAuditView, d.Permissions, audit)

		admin.GET("/offices", officeView, office.List)
		admin.GET("/offices/:id", officeView, office.Get)

		officesWrite := admin.Group("/offices")
		{
			officesWrite.POST("", officeEdit, office.Create)
			officesWrite.PATCH("/:id", officeEdit, office.Patch)
			officesWrite.DELETE("/:id", officeDelete, office.Delete)

			officesWrite.POST("/:id/services", officeEdit, office.AddService)
			officesWrite.PATCH("/:id/services/:sid", officeEdit, office.PatchService)
			officesWrite.DELETE("/:id/services/:sid", officeDelete, office.RemoveService)
		}

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

		// ประวัติการทำงาน — อ่านอย่างเดียว
		if d.Audit != nil {
			auditHandler := routes.NewAuditHandler(d.Audit)
			logs := admin.Group("/audit-logs", auditView)
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
