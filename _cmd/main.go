package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	// timezone ของ connector (Asia/Bangkok) ต้องโหลดได้แม้ image ไม่มี tzdata — B-6 · AC-15
	_ "time/tzdata"

	"net/http"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/cache/memory"
	redisstore "ai-office-backend/internal/adapter/cache/redis"
	"ai-office-backend/internal/adapter/config"
	httpgin "ai-office-backend/internal/adapter/handler/gin"
	"ai-office-backend/internal/adapter/host/hostapi"
	"ai-office-backend/internal/adapter/llm/anthropic"
	"ai-office-backend/internal/adapter/llm/openaicompat"
	"ai-office-backend/internal/adapter/metrics"
	"ai-office-backend/internal/adapter/notify"
	"ai-office-backend/internal/adapter/storage/filestore"
	memstore "ai-office-backend/internal/adapter/storage/memory"
	mongodb "ai-office-backend/internal/adapter/storage/mongodb"
	"ai-office-backend/internal/adapter/storage/mongodb/repository"
	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"
)

// seedOffices ใช้เฉพาะ APP_MODE=dev ตอนฐานยังว่าง — production ไม่มี office ตัวอย่าง (P-16)
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
	fmt.Println("[INFO] ฐานยังว่าง — ใส่ office 'demo' ให้เริ่มต้น (dev เท่านั้น)")
	return nil
}

type stores struct {
	offices  port.OfficeRepository
	users    port.ConsoleUserRepository
	roles    port.RoleConfigRepository
	settings port.SettingsRepository
	convs    port.ConversationRepository
	msgs     port.MessageRepository
	verifs   port.VerificationRepository
	access   port.AccessLogRepository
	deletes  port.DeletionRepository
	quotas   port.QuotaRepository
	rollups  port.RollupRepository
	audit    port.AuditRepository
	ping     func(context.Context) error
	close    func()
}

func openStores(ctx context.Context, cfg *config.Container) (stores, error) {
	if cfg.Store.IsMongo() {
		res, err := mongodb.New(ctx, cfg.Store.URI, cfg.Store.DBName)
		if err != nil {
			return stores{}, err // ข้อความถูกกรอง URI ออกแล้วใน adapter
		}
		offices, err := repository.NewOfficeRepository(ctx, res.DB)
		if err != nil {
			return stores{}, fmt.Errorf("สร้าง index offices ไม่สำเร็จ: %w", err)
		}
		if err := repository.EnsureIndexes(ctx, res.DB); err != nil {
			return stores{}, fmt.Errorf("สร้าง index ไม่สำเร็จ: %w", err)
		}
		if n, err := repository.MigrateOffices(ctx, res.DB); err != nil {
			return stores{}, fmt.Errorf("migrate offices: %w", err)
		} else if n > 0 {
			fmt.Printf("[INFO] migrate: เติม allow_all=true ให้ %d office\n", n)
		}
		users, err := repository.NewConsoleUserRepository(ctx, res.DB)
		if err != nil {
			return stores{}, fmt.Errorf("console user store: %w", err)
		}
		roles, err := repository.NewRoleMatrixRepository(ctx, res.DB)
		if err != nil {
			return stores{}, fmt.Errorf("role permission store: %w", err)
		}
		auditRepo, err := repository.NewAuditRepository(ctx, res.DB)
		if err != nil {
			return stores{}, fmt.Errorf("audit log store: สร้าง index ไม่สำเร็จ: %w", err)
		}
		if cfg.App.IsDev() {
			if err := seedIfEmpty(ctx, offices); err != nil {
				return stores{}, err
			}
		}
		fmt.Printf("[INFO] store: MongoDB · db=%s ✔\n", cfg.Store.DBName)
		return stores{
			offices: offices, users: users, roles: roles,
			settings: repository.NewSettingsRepo(res.DB), convs: repository.NewConversationRepo(res.DB),
			msgs: repository.NewMessageRepo(res.DB), verifs: repository.NewVerificationRepo(res.DB),
			access: repository.NewAccessLogRepo(res.DB), deletes: repository.NewDeletionRepo(res.DB),
			quotas: repository.NewQuotaRepo(res.DB), rollups: repository.NewRollupRepo(res.DB), audit: auditRepo,
			ping:  func(ctx context.Context) error { return res.Client.Ping(ctx, nil) },
			close: res.Close,
		}, nil
	}

	// STORE_DRIVER=file = dev/ทดลองเท่านั้น (config บังคับ mongo บน production)
	const storePath = "./data/offices.json"
	var seed []domain.Office
	if cfg.App.IsDev() {
		seed = seedOffices()
	}
	offices, err := filestore.NewOfficeRepository(storePath, seed)
	if err != nil {
		return stores{}, fmt.Errorf("open office store: %w", err)
	}
	users, err := filestore.NewConsoleUserRepository(filepath.Join(filepath.Dir(storePath), "console_users.json"))
	if err != nil {
		return stores{}, fmt.Errorf("open console user store: %w", err)
	}
	roles, err := filestore.NewRoleMatrixRepository(filepath.Join(filepath.Dir(storePath), "role_permissions.json"))
	if err != nil {
		return stores{}, fmt.Errorf("open role permission store: %w", err)
	}
	auditRepo, err := filestore.NewAuditRepository(filepath.Join(filepath.Dir(storePath), "audit_logs.jsonl"))
	if err != nil {
		return stores{}, fmt.Errorf("open audit log store: %w", err)
	}
	fmt.Printf("[INFO] store: ไฟล์ %s + ประวัติในหน่วยความจำ (หายเมื่อรีสตาร์ต) · ตั้ง STORE_DRIVER=mongo เพื่อใช้ MongoDB\n", storePath)
	return stores{
		offices: offices, users: users, roles: roles,
		settings: &memstore.Settings{}, convs: memstore.NewConversations(), msgs: memstore.NewMessages(),
		verifs: memstore.NewVerifications(), access: &memstore.AccessLogs{}, deletes: &memstore.Deletions{},
		quotas: memstore.NewQuotas(), rollups: memstore.NewRollups(), audit: auditRepo,
		ping: func(context.Context) error { return nil }, close: func() {},
	}, nil
}

type counters interface {
	port.Semaphore
	port.QuotaCounter
}

func main() {
	fmt.Println("[INFO] AI Office backend starting...")

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("[ERROR] config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// connector ผิดรูป = ไม่ boot (V5) · บอกไฟล์/บรรทัดใน error
	conns, err := connector.LoadDir(cfg.Connector.Dir)
	if err != nil {
		log.Fatalf("[ERROR] connectors:\n%v", err)
	}
	registry := connector.Registry(conns)
	fmt.Printf("[INFO] connectors: %v ✔\n", registry.Kinds())

	st, err := openStores(ctx, cfg)
	if err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
	defer st.close()

	var cnt counters
	var cache port.Cache
	readyRedis := func(context.Context) error { return nil }
	if cfg.Redis.URL != "" {
		rs, err := redisstore.New(cfg.Redis.URL)
		if err != nil {
			log.Fatalf("[ERROR] เชื่อมต่อ Redis ไม่สำเร็จ") // ไม่ log URL (อาจมีรหัสผ่าน)
		}
		defer rs.Close()
		cnt, cache, readyRedis = rs, rs.Cache(), rs.Ping
		fmt.Println("[INFO] counters: Redis ✔")
	} else {
		ms := memory.New()
		cnt, cache = ms, ms.Cache()
		fmt.Println("[WARN] ไม่ได้ตั้ง REDIS_URL — ตัวนับ/แคชอยู่ในหน่วยความจำ (dev เท่านั้น · หลาย replica จะนับแยกกัน)")
	}

	// console session JWT ██ ต้องมี secret จริงบน production (config.validate ตรวจแล้ว)
	if os.Getenv("CONSOLE_JWT_SECRET") == "" {
		fmt.Println("[WARN] CONSOLE_JWT_SECRET ไม่ได้ตั้งค่า — ใช้ dev secret ชั่วคราว ██ ห้ามใช้บน production")
	}
	if os.Getenv("TICKET_SECRET") == "" {
		fmt.Println("[WARN] TICKET_SECRET ไม่ได้ตั้งค่า — ใช้ dev secret ชั่วคราว ██ ห้ามใช้บน production")
	}
	fmt.Printf("[INFO] LLM: provider=%s model=%q (ว่าง = ตาม settings)\n", cfg.LLM.Provider, cfg.LLM.Model)
	if cfg.LLM.APIKey == "" && cfg.LLM.Provider == "anthropic" {
		fmt.Println("[WARN] ANTHROPIC_API_KEY ไม่ได้ตั้งค่า — แชทจะตอบว่า \"ผู้ช่วยไม่ตอบกลับ\" จนกว่าจะตั้งคีย์")
	}

	// session token อายุ 8 ชม. (ค่าคงที่ ไม่ได้มาจาก env)
	tokens := auth.NewJWTIssuer(cfg.Console.JWTSecret, 8*time.Hour)
	totpP := auth.NewTOTPProvider("AI Office Console")
	consoleTickets := auth.NewTicketIssuer(cfg.Console.JWTSecret, 5*time.Minute)
	auditSvc := service.NewAuditService(st.audit, time.Now)
	permSvc := service.NewPermissionService(st.roles, auditSvc)
	authSvc := service.NewConsoleAuth(st.users, tokens, totpP, consoleTickets, permSvc, auditSvc, time.Now)

	accessTickets := auth.NewAccessTicketIssuer(cfg.Ticket.Secret)
	sealer, err := auth.NewGrantSealer(cfg.Ticket.Secret)
	if err != nil {
		log.Fatalf("[ERROR] grant sealer: %v", err)
	}

	settingsSvc := service.NewSettingsService(st.settings)
	notifier := notify.NewTelegram(cfg.Notify.TelegramToken)
	quotaSvc := service.NewQuotaService(cnt, st.quotas, settingsSvc, notifier, time.Now)
	officeSvc := service.NewOfficeService(service.OfficeDeps{
		Repo: st.offices, Connectors: registry, Settings: settingsSvc,
		Tickets: accessTickets, Sealer: sealer, Quota: quotaSvc, Audit: auditSvc,
	})

	// widget หา office จากโดเมนได้ (key "auto") — โดเมนซ้ำข้าม office / รูปแบบเก่าทำให้หาผิดตัวได้
	if issues, err := officeSvc.OriginIssues(ctx); err != nil {
		fmt.Printf("[WARN] ตรวจโดเมนของ office ไม่สำเร็จ: %v\n", err)
	} else {
		for _, msg := range issues {
			fmt.Println("[WARN] " + msg)
		}
	}

	// office ที่ยังไม่ตั้ง kind จะใช้ widget ไม่ได้ (page-config ตอบ kind_not_set) — เตือนตอน boot
	if list, err := officeSvc.List(ctx); err == nil {
		for _, o := range list {
			if _, ok := registry.Get(o.Kind); !ok {
				fmt.Printf("[WARN] office %q ยังไม่ได้ตั้ง kind ที่มี connector (ตอนนี้ = %q) — ตั้งที่คอนโซลก่อนใช้งาน\n", o.ID, o.Kind)
			}
		}
	}

	host := hostapi.New()
	runner := &service.ToolRunner{Host: host, Scoped: service.NewScopedTokenSource(host, sealer, time.Now), Cache: cache}
	mx := metrics.New()
	chat := service.NewChatService(service.ChatDeps{
		Connectors: registry, Settings: settingsSvc, Quota: quotaSvc, LLM: newLLM(cfg.LLM),
		Runner: runner, Convs: st.convs, Msgs: st.msgs, Rollups: st.rollups, Sem: cnt, Metrics: mx,
	})
	adminSvc := &service.AdminService{
		Offices: officeSvc, Convs: st.convs, Msgs: st.msgs, Verifs: st.verifs, Access: st.access,
		Deletes: st.deletes, Rollups: st.rollups, Quota: quotaSvc, Settings: settingsSvc,
	}
	mx.WatchRows(ctx, st.msgs.Estimate)

	r := httpgin.NewRouter(httpgin.Deps{
		OfficeService: officeSvc,
		Connectors:    registry,
		Tickets:       accessTickets,
		Chat:          chat,
		Admin:         adminSvc,
		Settings:      settingsSvc,
		Rollups:       st.rollups,
		BundlePath:    "./static/widget/ai-office.v1.js",
		Tokens:        tokens,
		Auth:          authSvc,
		Permissions:   permSvc,
		Audit:         auditSvc,
		// break-glass token ปิดอยู่ (ค่าว่าง) — เข้าผ่าน login จริงเท่านั้น
		ConsoleToken: "",
		Metrics:      mx.Handler(),
		MetricsToken: cfg.Metrics.Token,
		Ready: func(ctx context.Context) error {
			if err := st.ping(ctx); err != nil {
				return errors.New("mongo")
			}
			if err := readyRedis(ctx); err != nil {
				return errors.New("redis")
			}
			return nil
		},
	})

	addr := ":" + cfg.HTTP.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// ไม่ตั้ง WriteTimeout: SSE ต้องค้างได้ (เวลาตอบถูกคุมด้วย llm_timeout ใน settings แทน)
		IdleTimeout: 120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	fmt.Printf("[INFO] listening on %s (mode=%s)\n", addr, cfg.App.Mode)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[ERROR] server: %v", err)
	}
}

// newLLM เลือกตัวต่อโมเดลตาม LLM_PROVIDER — core ใช้แค่ port.LLM ไม่ผูกค่ายใด
func newLLM(c *config.LLM) port.LLM {
	if c.Provider == "openai" {
		return openaicompat.New(c.BaseURL, c.APIKey, c.Model, c.ExtraBody)
	}
	return anthropic.NewWith(c.APIKey, c.BaseURL, c.Model)
}
