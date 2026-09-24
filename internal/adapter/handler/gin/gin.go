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
	ConsoleToken  string
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS เปิดทุก origin
	r.Use(CORS())

	r.GET("/healthz", func(c *gin.Context) {
		routes.ResData(c, http.StatusOK, "SUCCESS", "", gin.H{"status": "ok"})
	})

	// สิ่งเดียวที่ backend ส่งให้หน้า office คือไฟล์ js ไฟล์นี้
	// ไม่มี HTML ไหนออกจาก service นี้ — UI ของแชทถูกสร้างด้วย JS ในหน้า office เอง
	widget := routes.NewWidgetHandler(d.BundlePath, d.OfficeService)
	r.GET("/widget/v1/:public_key/ai-office.js", widget.ServeBundle)

	boot := routes.NewBootstrapHandler(d.OfficeService)
	office := routes.NewOfficeHandler(d.OfficeService)

	// ---- ฝั่ง widget ----
	//
	//	public_key  ระบุว่า "หน้านี้เป็นของ office ไหน"
	//	service_id  ระบุว่า "แอดมินกำลังเปิด service ไหน" — หน้าเว็บเป็นคนรู้ (localStorage)
	//	            แต่ต้องผ่านการตรวจกับ Role.ListService ของแอดมินก่อนเสมอ
	//
	// ทั้งสองมาทาง path เท่านั้น · RejectTenantFields ยังกันการยัดค่าเหล่านี้ทาง body/query/header
	api := r.Group("/api/ai/office/:public_key/service/:service_id",
		RejectTenantFields(),
		ResolveOffice(d.OfficeService),
		ResolveCaller(d.Identity),
	)
	{
		api.GET("/bootstrap", func(c *gin.Context) {
			boot.Bootstrap(c, CallerFrom(c))
		})
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

	admin := r.Group("/api/ai/admin", ConsoleAuth(d.Tokens, d.ConsoleToken))
	{
		// authenticated, any role
		admin.GET("/auth/me", authHandler.Me)
		admin.POST("/auth/change-password", authHandler.ChangePassword)

		officeView := RequirePermission(domain.PermOfficeView, d.Permissions)
		officeEdit := RequirePermission(domain.PermOfficeEdit, d.Permissions)
		officeDelete := RequirePermission(domain.PermOfficeDelete, d.Permissions)
		officeRotate := RequirePermission(domain.PermOfficeRotate, d.Permissions)
		userManage := RequirePermission(domain.PermUserManage, d.Permissions)

		admin.GET("/offices", officeView, office.List)
		admin.GET("/offices/:id", officeView, office.Get)

		officesWrite := admin.Group("/offices")
		{
			officesWrite.POST("", officeEdit, office.Create)
			officesWrite.PATCH("/:id", officeEdit, office.Patch)
			officesWrite.DELETE("/:id", officeDelete, office.Delete)
			officesWrite.POST("/:id/rotate-key", officeRotate, office.RotateKey)

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
	}

	return r
}

// NewTestRouter ใช้ใน test — gin.TestMode ไม่พิมพ์ log รก
func NewTestRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(d)
}
