package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"ai-office-backend/docs"
	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/config"
	cryptobox "ai-office-backend/internal/adapter/crypto"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	llmrouter "ai-office-backend/internal/adapter/llm/router"
	"ai-office-backend/internal/adapter/storage/filestore"
	mongodb "ai-office-backend/internal/adapter/storage/mongodb"
	"ai-office-backend/internal/adapter/storage/mongodb/repository"
	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
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
	var credRepo port.LLMCredentialRepository
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
		credRepo = repository.NewLLMCredentialRepository(res.DB)
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
		credRepo = filestore.NewLLMCredentialRepository(filepath.Join(filepath.Dir(storePath), "llm_credentials.json"))
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
	// provider ใน .env ผิด = ไม่ยอมเปิด server · โมเดลและ API key ตั้งที่คอนโซล (ตั้งค่าระบบ)
	if err := cfg.LLM.Validate(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
	// API key เก็บใน DB แบบเข้ารหัสด้วย LLM_KEY_SECRET · ไม่มี secret = production ไม่เปิด (กันลืมแล้ว key ไม่ถูกเข้ารหัส)
	var keyBox port.SecretBox
	if cfg.LLM.KeySecret != "" {
		b, err := cryptobox.NewAESGCM(cfg.LLM.KeySecret, 1)
		if err != nil {
			log.Fatalf("[ERROR] %v", err)
		}
		keyBox = b
	} else if !cfg.App.IsDev() {
		log.Fatalf("[ERROR] production ต้องตั้ง LLM_KEY_SECRET (สร้างด้วย: openssl rand -base64 32)")
	} else {
		fmt.Println("[WARN] ยังไม่ตั้ง LLM_KEY_SECRET — ตั้ง API key จากคอนโซลไม่ได้ ใช้ key จาก .env แทน")
	}
	llmKeys := service.NewLLMKeyService(credRepo, keyBox, cfg.LLM.Keys, authSvc, auditSvc)
	if err := llmKeys.Load(ctx); err != nil {
		log.Fatalf("[ERROR] อ่าน API key จาก DB ไม่ได้: %v", err)
	}
	llmKeys.ImportEnv(ctx)
	for _, p := range domain.LLMProviderCatalog {
		k := llmKeys.Info(p.ID)
		state := "✘ ไม่มี key"
		switch {
		case k.Broken:
			state = "✘ ถอดรหัสไม่ได้ — ตั้งใหม่ในคอนโซล"
		case k.HasKey:
			state = "✔ " + k.Source
		}
		fmt.Printf("[INFO] LLM key %s: %s\n", p.ID, state)
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
	settingsSvc.SetLLMSeed(cfg.LLM.Seed())
	llm := llmrouter.New(llmKeys.Key, func() domain.LLMSettings { return settingsSvc.Current().LLM })
	settingsSvc.SetLLMKeyCheck(llm.HasKey)
	llmKeys.SetCurrent(func() domain.LLMSettings { return settingsSvc.Current().LLM })
	llmKeys.SetRecent(func(p string) (domain.LLMSettings, bool) {
		v, ok := settingsSvc.Current().LLMRecent[p]
		return v, ok
	})
	llmKeys.SetTester(llm.TestWithKey)
	if cur := settingsSvc.Current().LLM; !llm.Ready() {
		fmt.Printf("[WARN] โมเดลที่เลือก (%s · %s) ยังไม่มี key — ปิดแชท (/chat ตอบ 503) จนกว่าจะใส่ key หรือเปลี่ยนโมเดลในคอนโซล\n", cur.Provider, cur.Model)
	} else {
		fmt.Printf("[INFO] chat LLM: %s · model=%s ✔\n", cur.Provider, cur.Model)
	}
	usageSvc := service.NewUsageService(usageRepo)
	deletionSvc := service.NewDeletionService(chatRepo, verifRepo, accessRepo, deletionRepo, officeService, auditSvc)
	deletionSvc.RecoverStale(ctx)

	// อายุตั๋วตามตั้งค่าระบบ — widget ขอใหม่เองก่อนหมด
	chatTickets := auth.NewChatTicketIssuer(cfg.Console.ChatTicketSecret, func() time.Duration {
		return time.Duration(settingsSvc.Current().TicketTTLMin) * time.Minute
	})
	relay := service.NewRelay()
	loc, _ := conn.Location()
	chatSvc := service.NewChatService(llm, chatRepo, service.NewToolRunner(relay, settingsSvc), loc, settingsSvc, usageSvc)

	chatAdmin := service.NewChatAdminService(chatRepo, verifRepo, accessRepo, usageSvc)
	// ผู้ช่วยในคอนโซล — ใช้โมเดลเดียวกับแชท · เครื่องมืออ่านอย่างเดียวตามสิทธิ์คนถาม · เปิด/ปิดที่ตั้งค่าระบบ
	assistantSvc := service.NewAssistantService(llm, settingsSvc, permSvc, usageSvc,
		service.NewAssistantTools(officeService, usageSvc, chatAdmin, settingsSvc, llmKeys, auditSvc), docs.Guide)

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
		ChatAdmin:    chatAdmin,
		Settings:     settingsSvc,
		Usage:        usageSvc,
		Deletion:     deletionSvc,
		LLM:          llm,
		LLMKeys:      llmKeys,
		Assistant:    assistantSvc,
		Guide:        docs.Guide,
	})

	addr := ":" + cfg.HTTP.Port
	fmt.Printf("[INFO] listening on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[ERROR] server: %v", err)
	}
}
