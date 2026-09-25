package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/config"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/handler/gin/routes"
	anthropicllm "ai-office-backend/internal/adapter/llm/anthropic"
	"ai-office-backend/internal/adapter/llm/gemini"
	"ai-office-backend/internal/adapter/llm/openaicompat"
	"ai-office-backend/internal/adapter/storage/filestore"
	mongodb "ai-office-backend/internal/adapter/storage/mongodb"
	"ai-office-backend/internal/adapter/storage/mongodb/repository"
	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

func main() {
	fmt.Println("[INFO] AI Office backend starting...")

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("[ERROR] load config: %v", err)
	}

	ctx := context.Background()

	// เลือก storage ด้วย STORE_DRIVER — service/handler ไม่รู้ว่าเบื้องหลังเป็นอะไร
	var repo port.OfficeRepository
	var groupRepo port.OfficeGroupRepository
	var cuRepo port.ConsoleUserRepository
	var rmRepo port.RoleConfigRepository
	var auditRepo port.AuditRepository
	var chatRepo port.ChatRepository
	var verifRepo port.VerificationRepository
	var accessRepo port.AccessLogRepository
	var settingsRepo port.SettingsRepository
	var usageRepo port.UsageRepository
	var deletionRepo port.DeletionRepository
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
		groupRepo = repository.NewOfficeGroupRepository(res.DB)
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

		chatRepo, err = repository.NewChatRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] chat store: สร้าง index ไม่สำเร็จ: %v", err)
		}
		verifRepo, err = repository.NewVerificationRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] verification store: สร้าง index ไม่สำเร็จ: %v", err)
		}
		accessRepo, err = repository.NewAccessLogRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] access log store: สร้าง index ไม่สำเร็จ: %v", err)
		}
		settingsRepo = repository.NewSettingsRepository(res.DB)
		usageRepo = repository.NewUsageRepository(res.DB)
		deletionRepo, err = repository.NewDeletionRepository(ctx, res.DB)
		if err != nil {
			log.Fatalf("[ERROR] deletion store: สร้าง index ไม่สำเร็จ: %v", err)
		}
	} else {
		var err error
		// path ของ file store — ใช้เฉพาะโหมด STORE_DRIVER=file (dev/ทดลอง) ไม่ได้มาจาก env
		const storePath = "./data/offices.json"
		repo, err = filestore.NewOfficeRepository(storePath, nil)
		if err != nil {
			log.Fatalf("[ERROR] open office store: %v", err)
		}
		fmt.Printf("[INFO] office store: ไฟล์ %s ✔ (ตั้ง STORE_DRIVER=mongo เพื่อใช้ MongoDB)\n", storePath)

		groupRepo, err = filestore.NewOfficeGroupRepository(filepath.Join(filepath.Dir(storePath), "office_groups.json"))
		if err != nil {
			log.Fatalf("[ERROR] open office group store: %v", err)
		}

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

		chatRepo, err = filestore.NewChatRepository(
			filepath.Join(filepath.Dir(storePath), "conversations.jsonl"),
			filepath.Join(filepath.Dir(storePath), "messages.jsonl"))
		if err != nil {
			log.Fatalf("[ERROR] open chat store: %v", err)
		}
		verifRepo, err = filestore.NewVerificationRepository(filepath.Join(filepath.Dir(storePath), "verifications.jsonl"))
		if err != nil {
			log.Fatalf("[ERROR] open verification store: %v", err)
		}
		accessRepo = filestore.NewAccessLogRepository(filepath.Join(filepath.Dir(storePath), "access_log.jsonl"))
		settingsRepo = filestore.NewSettingsRepository(filepath.Join(filepath.Dir(storePath), "settings.json"))
		usageRepo, err = filestore.NewUsageRepository(filepath.Join(filepath.Dir(storePath), "usage.json"))
		if err != nil {
			log.Fatalf("[ERROR] open usage store: %v", err)
		}
		deletionRepo, err = filestore.NewDeletionRepository(filepath.Join(filepath.Dir(storePath), "deletion_requests.json"))
		if err != nil {
			log.Fatalf("[ERROR] open deletion store: %v", err)
		}
	}

	auditSvc := service.NewAuditService(auditRepo, time.Now)
	officeService := service.NewOfficeService(repo, groupRepo, auditSvc)

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

	// ---- แชท ----
	//
	// provider ผิด = ไม่ยอมเปิด server ทุก mode · ไม่มี key = ปิดแชท แต่ระบบส่วนอื่นยังทำงานปกติ
	if err := cfg.LLM.Validate(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
	var llm port.LLMClient
	switch {
	case cfg.LLM.APIKey == "":
		fmt.Printf("[WARN] LLM_API_KEY ว่าง (provider=%s) — ปิดแชท (/chat ตอบ 503)\n", cfg.LLM.Provider)
	case cfg.LLM.Provider == "anthropic":
		llm = anthropicllm.New(cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Effort)
	case cfg.LLM.Provider == "gemini":
		llm = gemini.New(cfg.LLM.APIKey, cfg.LLM.Model)
	case cfg.LLM.Provider == "openai":
		// GLM ปิดการคิดได้ผ่าน field thinking — ปิดแล้วตอบไวกว่าราว 2 เท่า · provider อื่นไม่รู้จัก field นี้
		var thinking map[string]string
		if strings.Contains(cfg.LLM.BaseURL, "z.ai") || strings.Contains(cfg.LLM.BaseURL, "bigmodel") {
			thinking = map[string]string{"type": "disabled"}
		}
		llm = openaicompat.New(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, thinking)
	}
	if llm != nil {
		fmt.Printf("[INFO] chat LLM: %s · model=%s ✔\n", cfg.LLM.Provider, cfg.LLM.Model)
		if cfg.LLM.Provider != "anthropic" && !cfg.App.IsDev() {
			fmt.Printf("[WARN] production ควรใช้ LLM_PROVIDER=anthropic — %s ใช้ทดสอบเท่านั้น\n", cfg.LLM.Provider)
		}
	}

	// connector — ผิดแม้ไฟล์เดียวก็ไม่เปิด server · office ทุกเจ้าตอนนี้เป็น office-v10x
	const connectorsDir, connectorKind = "./connectors", "office-v10x"
	connectors, err := connector.LoadDir(connectorsDir)
	if err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
	conn, ok := connectors[connectorKind]
	if !ok {
		log.Fatalf("[ERROR] ไม่พบ connector %s ใน %s", connectorKind, connectorsDir)
	}
	if !conn.Host.IsBrowser() {
		// backend ยิงหลังบ้านเองไม่ได้ (Cloudflare + IP whitelist) — รองรับเฉพาะโหมด browser
		log.Fatalf("[ERROR] connector %s ต้องเป็น mode: browser", connectorKind)
	}
	fmt.Printf("[INFO] connector: %s · %d tool · %d เมนู ✔\n", connectorKind, len(conn.Tools), len(conn.Menus.Menus))

	// ตั๋วแชท ██ secret จริงต้องตั้งบน production (เหตุผลเดียวกับ CONSOLE_JWT_SECRET ข้างบน)
	if os.Getenv("CHAT_TICKET_SECRET") == "" {
		if !cfg.App.IsDev() {
			log.Fatal("[ERROR] CHAT_TICKET_SECRET ต้องตั้งค่าก่อนรันบน production")
		}
		fmt.Println("[WARN] CHAT_TICKET_SECRET ไม่ได้ตั้งค่า — ใช้ dev secret ชั่วคราว ██ ห้ามใช้บน production")
	}
	settingsSvc := service.NewSettingsService(settingsRepo, auditSvc)
	usageSvc := service.NewUsageService(usageRepo)
	deletionSvc := service.NewDeletionService(chatRepo, verifRepo, accessRepo, deletionRepo, officeService, auditSvc)
	deletionSvc.RecoverStale(ctx)

	// อายุตั๋วตามตั้งค่าระบบ — widget ขอใหม่เองก่อนหมด
	chatTickets := auth.NewChatTicketIssuer(cfg.Console.ChatTicketSecret, func() time.Duration {
		return time.Duration(settingsSvc.Current().TicketTTLMin) * time.Minute
	})
	relay := service.NewRelay()
	var chatSvc *service.ChatService
	if llm != nil {
		loc, _ := conn.Location()
		chatSvc = service.NewChatService(llm, chatRepo, service.NewToolRunner(relay, settingsSvc), loc, settingsSvc, usageSvc)
	}

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
		Chat:         chatSvc,
		Relay:        relay,
		Tickets:      chatTickets,
		Connector:    conn,
		ChatAdmin:    service.NewChatAdminService(chatRepo, verifRepo, accessRepo, usageSvc),
		Settings:     settingsSvc,
		Usage:        usageSvc,
		Deletion:     deletionSvc,
		LLMInfo:      routes.LLMInfo{Provider: cfg.LLM.Provider, Model: cfg.LLM.Model, Enabled: llm != nil},
	})

	addr := ":" + cfg.HTTP.Port
	fmt.Printf("[INFO] listening on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[ERROR] server: %v", err)
	}
}
