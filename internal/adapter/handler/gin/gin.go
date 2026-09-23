package gin

import (
	"context"
	"net/http"

	"ai-office-backend/internal/adapter/handler/gin/routes"
	"ai-office-backend/internal/core/port"

	"github.com/gin-gonic/gin"
)

type Deps struct {
	OfficeService port.OfficeService
	Identity      port.IdentityResolver
	BundlePath    string
	ConsoleToken  string
	AllowedOrigin []string
	DevMode       bool
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// origin ของ office ทุกเจ้าอ่านจาก DB — เพิ่มลูกค้าใหม่ไม่ต้อง deploy
	r.Use(CORS(d.AllowedOrigin, func() []string {
		return d.OfficeService.AllowedOrigins(context.Background())
	}))

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
	//	            แต่ต้องผ่านการตรวจกับ Role.ListService ที่ office-api ก่อนเสมอ
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

	// ---- ฝั่งคอนโซล : auth คนละชุดกับแอดมินเว็บ ----
	admin := r.Group("/api/ai/admin", ConsoleAuth(d.ConsoleToken))
	{
		admin.GET("/offices", office.List)
		admin.POST("/offices", office.Create)
		admin.GET("/offices/:id", office.Get)
		admin.PATCH("/offices/:id", office.Patch)
		admin.DELETE("/offices/:id", office.Delete)
		admin.POST("/offices/:id/rotate-key", office.RotateKey)

		admin.POST("/offices/:id/services", office.AddService)
		admin.PATCH("/offices/:id/services/:sid", office.PatchService)
		admin.DELETE("/offices/:id/services/:sid", office.RemoveService)
	}

	return r
}

// NewTestRouter ใช้ใน test — gin.TestMode ไม่พิมพ์ log รก
func NewTestRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(d)
}
