package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

var (
	// ErrKeyStoreDisabled — ยังไม่ตั้ง LLM_KEY_SECRET จึงเก็บ key ลง DB ไม่ได้ (ใช้ key จาก .env อ่านอย่างเดียว)
	ErrKeyStoreDisabled = errors.New("LLM_KEY_STORE_DISABLED")
	// ErrStepUpRequired — บัญชีที่ไม่มีตัวตนจริง (break-glass) ยืนยัน 2FA ไม่ได้
	ErrStepUpRequired = errors.New("STEP_UP_REQUIRED")
	// ErrKeyInUse — ลบ key ของ provider ที่แชทใช้อยู่ = แชทล่มทันที ต้องเปลี่ยนโมเดลก่อน
	ErrKeyInUse = errors.New("LLM_KEY_IN_USE")
)

// StepUpVerifier ยืนยันตัวตนซ้ำด้วยรหัส 2FA (ConsoleAuth.ConfirmTOTP)
type StepUpVerifier interface {
	ConfirmTOTP(ctx context.Context, userID, code string) error
}

// LLMKeyTester ลองยิง LLM ด้วย key ที่ระบุ (ยังไม่บันทึก)
type LLMKeyTester func(ctx context.Context, cfg domain.LLMSettings, key string) (port.LLMTestResult, error)

// LLMKeyService ดูแล API key ของ LLM — เก็บเข้ารหัสใน DB · ถอดแล้วอยู่ใน memory ของ process เท่านั้น
//
// key ออกจากระบบได้ทางเดียวคือตอนส่งให้ provider: API คืนแค่ 4 ตัวท้าย · ประวัติบันทึกแค่ 4 ตัวท้าย
type LLMKeyService struct {
	repo    port.LLMCredentialRepository
	box     port.SecretBox    // nil = ยังไม่ตั้ง LLM_KEY_SECRET → ใช้ envKeys อ่านอย่างเดียว
	envKeys map[string]string // ██ secret
	stepUp  StepUpVerifier
	audit   port.AuditRecorder
	now     func() time.Time
	tester  LLMKeyTester
	current func() domain.LLMSettings
	recent  func(provider string) (domain.LLMSettings, bool) // ค่าล่าสุดที่เคยบันทึกของ provider นั้น

	mu     sync.RWMutex
	keys   map[string]string // provider → key ที่ถอดแล้ว ██ secret ห้าม log
	meta   map[string]domain.LLMCredential
	broken map[string]bool
}

func NewLLMKeyService(repo port.LLMCredentialRepository, box port.SecretBox, envKeys map[string]string,
	stepUp StepUpVerifier, audit port.AuditRecorder) *LLMKeyService {
	if envKeys == nil {
		envKeys = map[string]string{}
	}
	return &LLMKeyService{
		repo: repo, box: box, envKeys: envKeys, stepUp: stepUp, audit: auditOr(audit), now: time.Now,
		keys: map[string]string{}, meta: map[string]domain.LLMCredential{}, broken: map[string]bool{},
	}
}

// SetTester / SetCurrent ต่อทีหลังเพราะ router ของ LLM ต้องอ่าน key จาก service นี้ก่อน
func (s *LLMKeyService) SetTester(t LLMKeyTester)               { s.tester = t }
func (s *LLMKeyService) SetCurrent(f func() domain.LLMSettings) { s.current = f }
func (s *LLMKeyService) SetRecent(f func(provider string) (domain.LLMSettings, bool)) {
	s.recent = f
}

// Editable — ตั้ง/ลบ key จากหน้าเว็บได้ไหม
func (s *LLMKeyService) Editable() bool { return s.box != nil }

// Load อ่าน key ทั้งหมดจาก DB แล้วถอดรหัสเก็บใน memory · ถอดไม่ได้ = ถือว่าไม่มี key (ต้องตั้งใหม่)
func (s *LLMKeyService) Load(ctx context.Context) error {
	if s.box == nil {
		return nil
	}
	list, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range list {
		plain, err := s.box.Open(c.Nonce, c.Ciphertext, []byte(c.Provider))
		if err != nil {
			log.Printf("[WARN] ถอดรหัส API key ของ %s ไม่ได้ (LLM_KEY_SECRET เปลี่ยน?) — ต้องตั้ง key ใหม่ในคอนโซล", c.Provider)
			s.broken[c.Provider] = true
			s.meta[c.Provider] = c
			continue
		}
		s.keys[c.Provider] = string(plain)
		s.meta[c.Provider] = c
	}
	return nil
}

// ImportEnv ย้าย key จาก .env เข้า DB ครั้งเดียว (เฉพาะ provider ที่ DB ยังไม่มี) — หลังจากนั้นไม่อ่าน .env อีก
func (s *LLMKeyService) ImportEnv(ctx context.Context) {
	if s.box == nil {
		return
	}
	for _, p := range domain.LLMProviderCatalog {
		key := strings.TrimSpace(s.envKeys[p.ID])
		if key == "" {
			continue
		}
		s.mu.RLock()
		_, exists := s.meta[p.ID]
		s.mu.RUnlock()
		if exists {
			log.Printf("[WARN] %s ยังอยู่ใน .env แต่ระบบใช้ key ใน DB แล้ว — ลบออกจาก .env ได้", p.EnvKey)
			continue
		}
		if _, err := s.store(ctx, p.ID, key, "system (.env)"); err != nil {
			log.Printf("[WARN] ย้าย %s เข้า DB ไม่สำเร็จ: %v", p.EnvKey, err)
			continue
		}
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditLLMKeySet, Actor: "system", TargetType: "llm_key", TargetID: p.ID, TargetLabel: p.Label,
			Summary: fmt.Sprintf("ย้าย API key ของ %s จาก .env เข้า DB (เข้ารหัสแล้ว) %s", p.Label, domain.MaskKey(domain.KeyLast4(key))),
		})
		log.Printf("[INFO] ย้าย %s จาก .env เข้า DB แล้ว — ลบออกจาก .env ได้", p.EnvKey)
	}
}

// Key คืน key ที่ใช้ยิง provider · ██ ห้าม log ค่าที่ได้
func (s *LLMKeyService) Key(provider string) string {
	if s.box == nil {
		return s.envKeys[provider]
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keys[provider]
}

func (s *LLMKeyService) HasKey(provider string) bool { return s.Key(provider) != "" }

// Info คือสิ่งที่หน้าเว็บเห็น — ไม่มีตัว key
func (s *LLMKeyService) Info(provider string) domain.LLMKeyInfo {
	if s.box == nil {
		k := s.envKeys[provider]
		return domain.LLMKeyInfo{HasKey: k != "", Last4: domain.KeyLast4(k), Source: "env"}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.meta[provider]
	if !ok {
		return domain.LLMKeyInfo{}
	}
	return domain.LLMKeyInfo{
		HasKey: s.keys[provider] != "", Last4: m.Last4, Source: "db", Broken: s.broken[provider],
		UpdatedAt: m.UpdatedAt, UpdatedBy: m.UpdatedBy,
	}
}

// SetLLMKey — Model/BaseURL ใช้ทดสอบ key ก่อนบันทึก
// (ว่าง = ค่าที่ใช้อยู่ถ้า provider เดียวกัน → ค่าล่าสุดที่เคยบันทึกของ provider นั้น → โมเดลแนะนำตัวแรก)
type SetLLMKey struct {
	Provider string
	Key      string // ██ secret
	Code     string // รหัส 2FA
	Model    string
	BaseURL  string
}

// KeyActor คือผู้ทำรายการ — ID ว่าง (break-glass) = ยืนยัน 2FA ไม่ได้
type KeyActor struct {
	ID       string
	Username string
}

// Set ยืนยัน 2FA → ทดสอบ key กับ provider จริง → เข้ารหัสแล้วบันทึก · ขั้นไหนไม่ผ่าน key เดิมยังใช้ต่อ
func (s *LLMKeyService) Set(ctx context.Context, in SetLLMKey, actor KeyActor) (domain.LLMKeyInfo, error) {
	if s.box == nil {
		return domain.LLMKeyInfo{}, ErrKeyStoreDisabled
	}
	info, ok := domain.LLMProviderByID(in.Provider)
	if !ok {
		return domain.LLMKeyInfo{}, fmt.Errorf("provider %q ไม่รู้จัก", in.Provider)
	}
	key := strings.TrimSpace(in.Key)
	if err := domain.ValidateAPIKey(key); err != nil {
		return domain.LLMKeyInfo{}, err
	}
	// ตรวจค่าที่ใช้ทดสอบก่อนยืนยัน 2FA — ค่าไม่ครบไม่ควรเผารหัสหรือนับเป็นครั้งที่ผิด
	cfg := s.testConfig(in)
	if s.tester != nil {
		if err := cfg.Validate(); err != nil {
			return domain.LLMKeyInfo{}, fmt.Errorf("ทดสอบ key ไม่ได้: %w", err)
		}
	}
	if err := s.confirm(ctx, actor, in.Code); err != nil {
		return domain.LLMKeyInfo{}, err
	}
	if s.tester != nil {
		if _, err := s.tester(ctx, cfg, key); err != nil {
			return domain.LLMKeyInfo{}, fmt.Errorf("key ใช้ไม่ได้ — ยังไม่บันทึก: %s", redactWith(err.Error(), key))
		}
	}
	before := s.Info(in.Provider)
	if _, err := s.store(ctx, in.Provider, key, actor.Username); err != nil {
		return domain.LLMKeyInfo{}, err
	}
	after := s.Info(in.Provider)
	summary := fmt.Sprintf("ตั้ง API key ของ %s %s", info.Label, domain.MaskKey(after.Last4))
	if before.HasKey || before.Broken {
		summary = fmt.Sprintf("เปลี่ยน API key ของ %s %s → %s", info.Label, domain.MaskKey(before.Last4), domain.MaskKey(after.Last4))
	}
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditLLMKeySet, TargetType: "llm_key", TargetID: in.Provider, TargetLabel: info.Label, Summary: summary,
		Changes: []domain.FieldChange{{Field: "key", Before: domain.MaskKey(before.Last4), After: domain.MaskKey(after.Last4)}},
	})
	return after, nil
}

// Delete ยืนยัน 2FA แล้วลบ · ไม่ยอมลบ key ของ provider ที่แชทใช้อยู่
func (s *LLMKeyService) Delete(ctx context.Context, provider, code string, actor KeyActor) error {
	if s.box == nil {
		return ErrKeyStoreDisabled
	}
	info, ok := domain.LLMProviderByID(provider)
	if !ok {
		return fmt.Errorf("provider %q ไม่รู้จัก", provider)
	}
	if s.current != nil && s.current().Provider == provider {
		return ErrKeyInUse
	}
	if err := s.confirm(ctx, actor, code); err != nil {
		return err
	}
	before := s.Info(provider)
	if err := s.repo.Delete(ctx, provider); err != nil {
		return fmt.Errorf("ลบ key ไม่สำเร็จ: %w", err)
	}
	s.mu.Lock()
	delete(s.keys, provider)
	delete(s.meta, provider)
	delete(s.broken, provider)
	s.mu.Unlock()
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditLLMKeyDelete, TargetType: "llm_key", TargetID: provider, TargetLabel: info.Label,
		Summary: fmt.Sprintf("ลบ API key ของ %s %s", info.Label, domain.MaskKey(before.Last4)),
		Changes: []domain.FieldChange{{Field: "key", Before: domain.MaskKey(before.Last4), After: ""}},
	})
	return nil
}

// Redact ลบ key ทุกตัวที่รู้จักออกจากข้อความ ก่อนเขียน log หรือส่งกลับหน้าเว็บ
func (s *LLMKeyService) Redact(msg string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.keys {
		msg = redactWith(msg, k)
	}
	for _, k := range s.envKeys {
		msg = redactWith(msg, k)
	}
	return msg
}

func redactWith(msg, key string) string {
	if key == "" {
		return msg
	}
	return strings.ReplaceAll(msg, key, domain.MaskKey(domain.KeyLast4(key)))
}

func (s *LLMKeyService) confirm(ctx context.Context, actor KeyActor, code string) error {
	if actor.ID == "" || s.stepUp == nil {
		return ErrStepUpRequired
	}
	return s.stepUp.ConfirmTOTP(ctx, actor.ID, strings.TrimSpace(code))
}

func (s *LLMKeyService) testConfig(in SetLLMKey) domain.LLMSettings {
	cfg := domain.LLMSettings{Provider: in.Provider, Model: in.Model, BaseURL: in.BaseURL}
	fill := func(from domain.LLMSettings) {
		if cfg.Model == "" {
			cfg.Model = from.Model
		}
		if cfg.BaseURL == "" {
			cfg.BaseURL = from.BaseURL
		}
		if cfg.Effort == "" {
			cfg.Effort = from.Effort
		}
	}
	if s.current != nil {
		if cur := s.current(); cur.Provider == in.Provider {
			fill(cur)
		}
	}
	if s.recent != nil {
		if prev, ok := s.recent(in.Provider); ok {
			fill(prev)
		}
	}
	if cfg.Model == "" {
		if p, ok := domain.LLMProviderByID(in.Provider); ok && len(p.Models) > 0 {
			cfg.Model = p.Models[0]
		}
	}
	cfg.Normalize()
	return cfg
}

func (s *LLMKeyService) store(ctx context.Context, provider, key, actor string) (domain.LLMCredential, error) {
	nonce, ct, err := s.box.Seal([]byte(key), []byte(provider))
	if err != nil {
		return domain.LLMCredential{}, fmt.Errorf("เข้ารหัส key ไม่สำเร็จ: %w", err)
	}
	c := domain.LLMCredential{
		Provider: provider, Ciphertext: ct, Nonce: nonce, KeyVersion: s.box.Version(),
		Last4: domain.KeyLast4(key), UpdatedAt: s.now().Truncate(time.Millisecond), UpdatedBy: actor,
	}
	if err := s.repo.Save(ctx, c); err != nil {
		return c, fmt.Errorf("บันทึก key ไม่สำเร็จ: %w", err)
	}
	s.mu.Lock()
	s.keys[provider] = key
	s.meta[provider] = c
	delete(s.broken, provider)
	s.mu.Unlock()
	return c, nil
}
