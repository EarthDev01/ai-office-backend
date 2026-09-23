package main

import (
	"context"
	"fmt"
	"log"

	"ai-office-backend/internal/adapter/config"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	httpreq "ai-office-backend/internal/adapter/handler/http_request"
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
	} else {
		var err error
		repo, err = filestore.NewOfficeRepository(cfg.Store.ConfigPath, seedOffices())
		if err != nil {
			log.Fatalf("[ERROR] open office store: %v", err)
		}
		fmt.Printf("[INFO] office store: ไฟล์ %s ✔ (ตั้ง STORE_DRIVER=mongo เพื่อใช้ MongoDB)\n", cfg.Store.ConfigPath)
	}

	officeService := service.NewOfficeService(repo)

	// ตัวตนมาจาก office-api-v10 จริง — cache สั้น ๆ กันไปกิน rate limit ของเขา
	backoffice := httpreq.NewBackofficeClient(cfg.Backoffice.Timeout)
	identity := service.NewIdentityResolver(backoffice, cfg.Backoffice.IdentityTTL, cfg.App.IsDev())

	r := httpgin.NewRouter(httpgin.Deps{
		OfficeService: officeService,
		Identity:      identity,
		BundlePath:    "./static/widget/ai-office.v1.js",
		ConsoleToken:  cfg.Admin.ConsoleToken,
		AllowedOrigin: cfg.HTTP.AllowedOrigins,
		DevMode:       cfg.App.IsDev(),
	})

	if cfg.App.IsDev() {
		fmt.Println("[WARN] APP_MODE=dev — office ที่ยังไม่ตั้ง backoffice_api_url จะรับ token ปลอม 'dev:...' ได้ · ห้ามใช้ค่านี้บน production")
	}

	addr := ":" + cfg.HTTP.Port
	fmt.Printf("[INFO] listening on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[ERROR] server: %v", err)
	}
}
