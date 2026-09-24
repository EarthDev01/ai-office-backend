package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/config"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/storage/filestore"
	mongodb "ai-office-backend/internal/adapter/storage/mongodb"
	"ai-office-backend/internal/adapter/storage/mongodb/repository"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

// seedOffices ใช้เฉพาะตอนไฟล์ยังว่าง — ของจริง office ถูกสร้างจากคอนโซล
func seedOffices() []domain.Office {
	o := domain.NewOffice("demo", "หลังบ้านจำลอง")
	o.PublicKey = "pk_demo_local"
	o.AllowedOrigins = []string{"http://localhost:5174"}
	o.Services = []domain.Service{
		domain.DefaultService("K11S", "เว็บ K11S"),
		domain.DefaultService("PG99", "เว็บ PG99"),
	}
	return []domain.Office{o}
}

// seedIfEmpty ใส่ office ตัวอย่างให้เฉพาะตอนฐานยังว่าง — ไม่ทับของที่มีอยู่
func seedIfEmpty(ctx context.Context, repo port.OfficeRepository) error {
	list, err := repo.List(ctx)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return nil
	}
	for _, o := range seedOffices() {
		if err := repo.Save(ctx, o); err != nil {
			return err
		}
	}
	fmt.Println("[INFO] ฐานยังว่าง — ใส่ office 'demo' ให้เริ่มต้น")
	return nil
}

func main() {
	fmt.Println("[INFO] AI Office backend starting...")

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("[ERROR] load config: %v", err)
	}

	ctx := context.Background()

	// เลือก storage ด้วย STORE_DRIVER — service/handler ไม่รู้ว่าเบื้องหลังเป็นอะไร
	var repo port.OfficeRepository
	var cuRepo port.ConsoleUserRepository
	var rmRepo port.RoleConfigRepository
	var auditRepo port.AuditRepository
	if cfg.Store.IsMongo() {
		res, err := mongodb.New(ctx, cfg.Store.URI, cfg.Store.DBName)
		if err != nil {
			log.Fatalf("[ERROR] %v", err) // ข้อความถูกกรอง URI ออกแล้วใน adapter
		}
		defer res.Close()

		repo, err = repository.NewOfficeRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] สร้าง index ไม่สำเร็จ: %v", err)
		}
		if err := seedIfEmpty(ctx, repo); err != nil {
			log.Fatalf("[ERROR] seed: %v", err)
		}
		fmt.Printf("[INFO] office store: MongoDB · db=%s ✔\n", cfg.Store.DBName)

		cuRepo, err = repository.NewConsoleUserRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] console user store: สร้าง index ไม่สำเร็จ: %v", err)
		}

		rmRepo, err = repository.NewRoleMatrixRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] role permission store: %v", err)
		}

		auditRepo, err = repository.NewAuditRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] audit log store: สร้าง index ไม่สำเร็จ: %v", err)
		}
	} else {
		var err error
		// path ของ file store — ใช้เฉพาะโหมด STORE_DRIVER=file (dev/ทดลอง) ไม่ได้มาจาก env
		const storePath = "./data/offices.json"
		repo, err = filestore.NewOfficeRepository(storePath, seedOffices())
		if err != nil {
			log.Fatalf("[ERROR] open office store: %v", err)
		}
		fmt.Printf("[INFO] office store: ไฟล์ %s ✔ (ตั้ง STORE_DRIVER=mongo เพื่อใช้ MongoDB)\n", storePath)

		cuPath := filepath.Join(filepath.Dir(storePath), "console_users.json")
		cuRepo, err = filestore.NewConsoleUserRepository(cuPath)
		if err != nil {
			log.Fatalf("[ERROR] open console user store: %v", err)
		}

		rmPath := filepath.Join(filepath.Dir(storePath), "role_permissions.json")
		rmRepo, err = filestore.NewRoleMatrixRepository(rmPath)
		if err != nil {
			log.Fatalf("[ERROR] open role permission store: %v", err)
		}

		auditPath := filepath.Join(filepath.Dir(storePath), "audit_logs.jsonl")
		auditRepo, err = filestore.NewAuditRepository(auditPath)
		if err != nil {
			log.Fatalf("[ERROR] open audit log store: %v", err)
		}
	}

	auditSvc := service.NewAuditService(auditRepo, time.Now)
	officeService := service.NewOfficeService(repo, auditSvc)

	// widget หา office จากโดเมน — โดเมนซ้ำข้าม office / รูปแบบเก่าที่ยังไม่ normalize ทำให้หาผิดตัวได้
	if issues, err := officeService.OriginIssues(ctx); err != nil {
		fmt.Printf("[WARN] ตรวจโดเมนของ office ไม่สำเร็จ: %v\n", err)
	} else {
		for _, msg := range issues {
			fmt.Println("[WARN] " + msg)
		}
	}

	// cache ttl เป็นค่าคงที่ (ไม่ได้มาจาก env)
	identity := service.NewIdentityResolver(60 * time.Second)

	// console session JWT ██ ต้องมี secret จริงบน production — ห้ามปล่อยให้ใช้ dev secret หลุดขึ้นจริง
	//
	// ██ ต้องเช็ค os.Getenv ตรง ๆ ไม่ใช่ cfg.Console.JWTSecret — config.env() ใส่ default
	// "dev-console-jwt-secret" ให้เสมอเมื่อ env ว่าง เช็คผ่าน cfg ที่ default แล้วจะไม่มีวัน "" จริง
	// (กลายเป็น dead code) ทำให้ production ที่ลืมตั้ง CONSOLE_JWT_SECRET เซ็น JWT ด้วย secret
	// ที่มี plaintext อยู่ในโค้ดแบบเงียบ ๆ โดยไม่มี fatal/warn ใด ๆ
	if os.Getenv("CONSOLE_JWT_SECRET") == "" {
		if !cfg.App.IsDev() {
			log.Fatal("[ERROR] CONSOLE_JWT_SECRET ต้องตั้งค่าก่อนรันบน production")
		}
		fmt.Println("[WARN] CONSOLE_JWT_SECRET ไม่ได้ตั้งค่า — ใช้ dev secret ชั่วคราว ██ ห้ามใช้บน production")
	}

	// session token อายุ 8 ชม. (ค่าคงที่ ไม่ได้มาจาก env)
	tokens := auth.NewJWTIssuer(cfg.Console.JWTSecret, 8*time.Hour)
	totpP := auth.NewTOTPProvider("AI Office Console")
	tickets := auth.NewTicketIssuer(cfg.Console.JWTSecret, 5*time.Minute)
	permSvc := service.NewPermissionService(rmRepo, auditSvc)
	authSvc := service.NewConsoleAuth(cuRepo, tokens, totpP, tickets, permSvc, auditSvc, time.Now)

	r := httpgin.NewRouter(httpgin.Deps{
		OfficeService: officeService,
		Identity:      identity,
		BundlePath:    "./static/widget/ai-office.v1.js",
		Tokens:        tokens,
		Auth:          authSvc,
		Permissions:   permSvc,
		Audit:         auditSvc,
		// break-glass token ปิดอยู่ (ค่าว่าง) — เข้าผ่าน login จริงเท่านั้น
		ConsoleToken: "",
	})

	addr := ":" + cfg.HTTP.Port
	fmt.Printf("[INFO] listening on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[ERROR] server: %v", err)
	}
}
