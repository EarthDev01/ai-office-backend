package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type OfficeDeps struct {
	Repo       port.OfficeRepository
	Connectors port.ConnectorRegistry
	Settings   *SettingsService
	Tickets    port.AccessTicketIssuer
	Sealer     port.GrantSealer
	Quota      *QuotaService
	Now        port.Clock
	Audit      port.AuditRecorder // nil ได้ (ไม่บันทึกประวัติ)
}

type officeService struct {
	OfficeDeps
}

func NewOfficeService(d OfficeDeps) port.OfficeService {
	if d.Now == nil {
		d.Now = time.Now
	}
	d.Audit = auditOr(d.Audit)
	return &officeService{OfficeDeps: d}
}

// field ที่ไม่นับเป็น "การแก้ไข" ในประวัติ — services มีเหตุการณ์ของตัวเองแยก
var officeDiffSkip = []string{"updated_at", "updated_by", "created_at", "services"}

var (
	idPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)
	allowedThemes    = map[string]bool{"auto": true, "light": true, "dark": true}
	allowedPositions = map[string]bool{"bottom-right": true, "bottom-left": true}
)

func (s *officeService) List(ctx context.Context) ([]domain.Office, error) { return s.Repo.List(ctx) }

func (s *officeService) Get(ctx context.Context, id string) (domain.Office, error) {
	return s.Repo.Get(ctx, id)
}

func (s *officeService) validKind(kind string) error {
	if kind == "" {
		return fmt.Errorf("ต้องเลือก kind (ชนิดหลังบ้าน)")
	}
	if s.Connectors == nil {
		return nil
	}
	if _, ok := s.Connectors.Get(kind); !ok {
		return fmt.Errorf("kind %q ยังไม่มี connector — มีแค่ %v", kind, s.Connectors.Kinds())
	}
	return nil
}

func (s *officeService) Create(ctx context.Context, id, label, kind, actor string) (domain.Office, error) {
	if !idPattern.MatchString(id) {
		return domain.Office{}, fmt.Errorf("id ต้องเป็น a-z A-Z 0-9 _ - ยาวไม่เกิน 40 ตัว")
	}
	if kind != "" {
		if err := s.validKind(kind); err != nil {
			return domain.Office{}, err
		}
	}
	if _, err := s.Repo.Get(ctx, id); err == nil {
		return domain.Office{}, domain.ErrConflict
	}
	if strings.TrimSpace(label) == "" {
		label = id
	}
	o := domain.NewOffice(id, label)
	o.Kind = kind
	o.UpdatedBy = actor
	if err := s.Repo.Save(ctx, o); err != nil {
		return domain.Office{}, err
	}
	s.Audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditOfficeCreate, TargetType: "office", TargetID: o.ID, TargetLabel: o.Label,
		Summary: fmt.Sprintf("สร้าง office %q (%s)", o.Label, o.ID),
	})
	return o, nil
}

func (s *officeService) Update(ctx context.Context, id string, p domain.UpdateOffice, actor string) (domain.Office, error) {
	o, err := s.Repo.Get(ctx, id)
	if err != nil {
		return domain.Office{}, err
	}
	before := o
	before.AllowedOrigins = append([]string(nil), o.AllowedOrigins...)
	if p.Label != nil {
		o.Label = *p.Label
	}
	if p.Kind != nil {
		if err := s.validKind(*p.Kind); err != nil {
			return domain.Office{}, err
		}
		o.Kind = *p.Kind
	}
	if p.AllowedOrigins != nil {
		clean, err := normalizeOrigins(*p.AllowedOrigins)
		if err != nil {
			return domain.Office{}, err
		}
		if err := s.ensureOriginsFree(ctx, o.ID, clean); err != nil {
			return domain.Office{}, err
		}
		o.AllowedOrigins = clean
	}
	if p.BackofficeAPIURL != nil {
		u := strings.TrimRight(strings.TrimSpace(*p.BackofficeAPIURL), "/")
		if u != "" && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return domain.Office{}, fmt.Errorf("backoffice_api_url ต้องขึ้นต้นด้วย http:// หรือ https://")
		}
		o.BackofficeAPIURL = u
	}
	if p.Enabled != nil {
		o.Enabled = *p.Enabled
	}
	if p.IsHidden != nil {
		o.IsHidden = *p.IsHidden
	}
	if p.Theme != nil {
		if !allowedThemes[*p.Theme] {
			return domain.Office{}, fmt.Errorf("theme ต้องเป็น auto, light หรือ dark เท่านั้น")
		}
		o.Theme = *p.Theme
	}
	if p.Placement != nil {
		if !allowedPositions[p.Placement.Position] {
			return domain.Office{}, fmt.Errorf("position ต้องเป็น bottom-right หรือ bottom-left เท่านั้น")
		}
		if p.Placement.OffsetX < 0 || p.Placement.OffsetX > 400 || p.Placement.OffsetY < 0 || p.Placement.OffsetY > 400 {
			return domain.Office{}, fmt.Errorf("offset ต้องอยู่ระหว่าง 0 ถึง 400")
		}
		o.Placement = *p.Placement
	}
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	if changes := domain.DiffFields(before, saved, officeDiffSkip...); len(changes) > 0 {
		s.Audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditOfficeUpdate, TargetType: "office", TargetID: saved.ID, TargetLabel: saved.Label,
			Summary: fmt.Sprintf("แก้ไข office %q (%s) %d รายการ: %s", saved.Label, saved.ID, len(changes), changedFieldNames(changes)),
			Changes: changes,
		})
	}
	return saved, nil
}

func (s *officeService) Delete(ctx context.Context, id string) error {
	// อ่านก่อนลบ เพื่อให้ประวัติยังบอกได้ว่าลบ office ชื่ออะไร มี service อะไรบ้าง
	o, getErr := s.Repo.Get(ctx, id)
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	label := id
	meta := map[string]string{}
	if getErr == nil {
		label = o.Label
		ids := make([]string, 0, len(o.Services))
		for _, svc := range o.Services {
			ids = append(ids, svc.ID)
		}
		meta["services"] = strings.Join(ids, ", ")
		meta["public_key"] = o.PublicKey
	}
	s.Audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditOfficeDelete, TargetType: "office", TargetID: id, TargetLabel: label,
		Summary: fmt.Sprintf("ลบ office %q (%s)", label, id),
		Meta:    meta,
	})
	return nil
}

// RotateKey ทำให้ snippet เดิมใช้ไม่ได้ทันที — คนเรียกต้องรู้ตัว
func (s *officeService) RotateKey(ctx context.Context, id, actor string) (domain.Office, error) {
	o, err := s.Repo.Get(ctx, id)
	if err != nil {
		return domain.Office{}, err
	}
	o.PublicKey = domain.NewPublicKey()
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	s.Audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditOfficeRotateKey, TargetType: "office", TargetID: saved.ID, TargetLabel: saved.Label,
		Summary: fmt.Sprintf("เปลี่ยน public key ของ office %q (%s) — snippet เดิมใช้ไม่ได้แล้ว", saved.Label, saved.ID),
	})
	return saved, nil
}

func (s *officeService) AddService(ctx context.Context, officeID, serviceID, label, actor string) (domain.Office, error) {
	o, err := s.Repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	if !idPattern.MatchString(serviceID) {
		return domain.Office{}, fmt.Errorf("service id ต้องเป็น a-z A-Z 0-9 _ - ยาวไม่เกิน 40 ตัว")
	}
	if _, ok := o.FindService(serviceID); ok {
		return domain.Office{}, domain.ErrConflict
	}
	if strings.TrimSpace(label) == "" {
		label = serviceID
	}
	svc := domain.DefaultService(serviceID, label)
	// service ใหม่ได้เพดานจำกัดเสมอ (P-C13) — ไม่ใช่ 0/ไม่จำกัด
	if s.Settings != nil {
		if st, err := s.Settings.Get(ctx); err == nil {
			svc.Quota.MonthlyLimit = st.DefaultMonthlyLimit
		}
	}
	o.Services = append(o.Services, svc)
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	s.Audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditServiceCreate, TargetType: "service", TargetID: serviceID, TargetLabel: label,
		Summary: fmt.Sprintf("เพิ่ม service %q (%s) ใน office %q", label, serviceID, saved.Label),
		Meta:    map[string]string{"office_id": saved.ID, "office_label": saved.Label},
	})
	return saved, nil
}

func (s *officeService) UpdateService(ctx context.Context, officeID, serviceID string, p domain.UpdateService, actor string) (domain.Office, error) {
	o, err := s.Repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	idx := o.ServiceIndex(serviceID)
	if idx < 0 {
		return domain.Office{}, domain.ErrNotFound
	}
	svc := &o.Services[idx]
	before := *svc
	before.Allowlist = append([]string(nil), svc.Allowlist...)
	if p.Label != nil {
		svc.Label = *p.Label
	}
	if p.Enabled != nil {
		svc.Enabled = *p.Enabled
	}
	if p.AllowAll != nil {
		svc.AllowAll = *p.AllowAll
	}
	if p.Allowlist != nil {
		clean := []string{}
		for _, x := range *p.Allowlist {
			if x = strings.TrimSpace(x); x != "" {
				clean = append(clean, x)
			}
		}
		svc.Allowlist = clean
	}
	if p.DisplayName != nil {
		svc.DisplayName = *p.DisplayName
	}
	if p.Greeting != nil {
		svc.Greeting = *p.Greeting
	}
	if p.AvatarURL != nil {
		u := strings.TrimSpace(*p.AvatarURL)
		if u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			return domain.Office{}, fmt.Errorf("avatar_url ต้องเป็น http(s)")
		}
		svc.AvatarURL = u
	}
	if p.MonthlyLimit != nil {
		if *p.MonthlyLimit <= 0 {
			return domain.Office{}, fmt.Errorf("monthly_limit ต้องมากกว่า 0 (ไม่มีแบบไม่จำกัด)")
		}
		svc.Quota.MonthlyLimit = *p.MonthlyLimit
	}
	after := *svc
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	if changes := domain.DiffFields(before, after); len(changes) > 0 {
		s.Audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditServiceUpdate, TargetType: "service", TargetID: after.ID, TargetLabel: after.Label,
			Summary: fmt.Sprintf("แก้ไข service %q (%s) ใน office %q %d รายการ: %s",
				after.Label, after.ID, saved.Label, len(changes), changedFieldNames(changes)),
			Changes: changes,
			Meta:    map[string]string{"office_id": saved.ID, "office_label": saved.Label},
		})
	}
	return saved, nil
}

func (s *officeService) RemoveService(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error) {
	o, err := s.Repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	out := make([]domain.Service, 0, len(o.Services))
	var removed domain.Service
	found := false
	for _, svc := range o.Services {
		if svc.ID == serviceID {
			removed = svc
			found = true
			continue
		}
		out = append(out, svc)
	}
	if !found {
		return domain.Office{}, domain.ErrNotFound
	}
	o.Services = out
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	s.Audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditServiceDelete, TargetType: "service", TargetID: removed.ID, TargetLabel: removed.Label,
		Summary: fmt.Sprintf("ลบ service %q (%s) ออกจาก office %q", removed.Label, removed.ID, saved.Label),
		Meta:    map[string]string{"office_id": saved.ID, "office_label": saved.Label},
	})
	return saved, nil
}

// ---------- secret_key (R3) ----------

func (s *officeService) withService(ctx context.Context, officeID, serviceID string, fn func(o *domain.Office, svc *domain.Service) error, actor string) (domain.Office, error) {
	o, err := s.Repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	idx := o.ServiceIndex(serviceID)
	if idx < 0 {
		return domain.Office{}, domain.ErrNotFound
	}
	if err := fn(&o, &o.Services[idx]); err != nil {
		return domain.Office{}, err
	}
	return s.touchSave(ctx, o, actor)
}

// auditService บันทึกเหตุการณ์ของ service (secret / โควตา) — ไม่ใส่ค่า secret ใด ๆ
func (s *officeService) auditService(ctx context.Context, action string, o domain.Office, serviceID, summary string, meta map[string]string) {
	label := serviceID
	if svc, ok := o.FindService(serviceID); ok {
		label = svc.Label
	}
	if meta == nil {
		meta = map[string]string{}
	}
	meta["office_id"], meta["office_label"] = o.ID, o.Label
	s.Audit.Record(ctx, domain.AuditEntry{
		Action: action, TargetType: "service", TargetID: serviceID, TargetLabel: label,
		Summary: fmt.Sprintf("%s — service %q (%s) ใน office %q", summary, label, serviceID, o.Label),
		Meta:    meta,
	})
}

// IssueSecret ออกใบแรก · มีอยู่แล้วต้องใช้ rotate (กันกดซ้ำแล้ว host เดิมพังโดยไม่ตั้งใจ)
func (s *officeService) IssueSecret(ctx context.Context, officeID, serviceID, actor string) (string, domain.Office, error) {
	secret := domain.NewSecretKey()
	o, err := s.withService(ctx, officeID, serviceID, func(_ *domain.Office, svc *domain.Service) error {
		if svc.SecretKeyHash != "" {
			return fmt.Errorf("service นี้มี secret แล้ว — ใช้ rotate แทน")
		}
		now := s.Now()
		svc.SecretKeyHash = domain.HashSecret(secret)
		svc.SecretKeyPrevHash = ""
		svc.SecretCreatedAt = &now
		svc.SecretRotatedAt = nil
		return nil
	}, actor)
	if err != nil {
		return "", domain.Office{}, err
	}
	s.auditService(ctx, domain.AuditServiceSecretIssue, o, serviceID, "ออก secret_key ใบแรก", nil)
	return secret, o, nil
}

// RotateSecret = ใบใหม่เป็นปัจจุบัน ใบเก่ายังรับได้จนกว่าจะ commit (ไม่ล่มช่วงเปลี่ยน)
func (s *officeService) RotateSecret(ctx context.Context, officeID, serviceID, actor string) (string, domain.Office, error) {
	secret := domain.NewSecretKey()
	o, err := s.withService(ctx, officeID, serviceID, func(_ *domain.Office, svc *domain.Service) error {
		if svc.SecretKeyHash == "" {
			return fmt.Errorf("ยังไม่มี secret — ใช้ออกใบแรกก่อน")
		}
		now := s.Now()
		svc.SecretKeyPrevHash = svc.SecretKeyHash
		svc.SecretKeyHash = domain.HashSecret(secret)
		svc.SecretRotatedAt = &now
		return nil
	}, actor)
	if err != nil {
		return "", domain.Office{}, err
	}
	s.auditService(ctx, domain.AuditServiceSecretRotate, o, serviceID, "หมุน secret_key (ใบเก่ายังใช้ได้จนกว่าจะยืนยัน)", nil)
	return secret, o, nil
}

// CommitSecret ตัดใบเก่าหลัง host เปลี่ยนเป็นใบใหม่ครบแล้ว
func (s *officeService) CommitSecret(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error) {
	o, err := s.withService(ctx, officeID, serviceID, func(_ *domain.Office, svc *domain.Service) error {
		svc.SecretKeyPrevHash = ""
		return nil
	}, actor)
	if err == nil {
		s.auditService(ctx, domain.AuditServiceSecretCommit, o, serviceID, "ยืนยัน secret_key ใบใหม่ (ตัดใบเก่า)", nil)
	}
	return o, err
}

// RevokeSecret ยกเลิกทันทีทั้ง 2 ใบ — /session ปฏิเสธ + ตั๋วที่มีอยู่ใช้ต่อไม่ได้ (CheckTicket)
func (s *officeService) RevokeSecret(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error) {
	o, err := s.withService(ctx, officeID, serviceID, func(_ *domain.Office, svc *domain.Service) error {
		svc.SecretKeyHash = ""
		svc.SecretKeyPrevHash = ""
		svc.SecretRotatedAt = nil
		return nil
	}, actor)
	if err == nil {
		s.auditService(ctx, domain.AuditServiceSecretRevoke, o, serviceID, "ยกเลิก secret_key ทั้งหมด", nil)
	}
	return o, err
}

func (s *officeService) IncreaseQuota(ctx context.Context, officeID, serviceID string, amount int64, reason, actor string) (domain.Office, error) {
	if amount <= 0 {
		return domain.Office{}, fmt.Errorf("จำนวนที่เพิ่มต้องมากกว่า 0")
	}
	if strings.TrimSpace(reason) == "" {
		return domain.Office{}, fmt.Errorf("ต้องระบุเหตุผล")
	}
	if actor == "" {
		return domain.Office{}, fmt.Errorf("ต้องรู้ว่าใครเป็นผู้เพิ่ม")
	}
	o, err := s.withService(ctx, officeID, serviceID, func(_ *domain.Office, svc *domain.Service) error {
		svc.Quota.TempIncreases = append(svc.Quota.TempIncreases, domain.TempIncrease{
			Period: domain.PeriodOf(s.Now()), Amount: amount, By: actor, Reason: strings.TrimSpace(reason), At: s.Now(),
		})
		return nil
	}, actor)
	if err == nil {
		s.auditService(ctx, domain.AuditServiceQuotaIncrease, o, serviceID, fmt.Sprintf("เพิ่มโควตาชั่วคราว %d token", amount),
			map[string]string{"amount": fmt.Sprint(amount), "reason": strings.TrimSpace(reason)})
	}
	return o, err
}

func (s *officeService) ResolveByPublicKey(ctx context.Context, publicKey string) (domain.Office, error) {
	return s.Repo.GetByPublicKey(ctx, publicKey)
}

// ---------- ตั๋ว (R4) ----------

// OpenSession: host (ถือ secret_key) ขอตั๋วให้ผู้ใช้ที่ตัวเองยืนยันแล้ว
//
// ตรวจครบ: secret (ใบปัจจุบันหรือใบเก่าช่วงหมุน) · office/service เปิด · kind ตรง · allow_all/allowlist · โควตา
// ██ public_key ไม่ใช่ความลับ — ห้ามใช้แทน secret (R-7 · AC-24)
func (s *officeService) OpenSession(ctx context.Context, req port.SessionRequest) (port.SessionResult, error) {
	o, err := s.Repo.GetByPublicKey(ctx, req.PublicKey)
	if err != nil {
		return port.SessionResult{}, err
	}
	idx := o.ServiceIndex(req.ServiceID)
	if idx < 0 {
		// ไม่บอกว่า service มีจริงไหม ก่อนผ่าน secret
		return port.SessionResult{}, domain.ErrSecretInvalid
	}
	svc := &o.Services[idx]
	which := svc.SecretMatches(req.Secret)
	if which == "" {
		return port.SessionResult{}, domain.ErrSecretInvalid
	}
	if !o.Enabled {
		return port.SessionResult{}, domain.Refuse(domain.ReasonOfficeDisabled)
	}
	if !svc.Enabled {
		return port.SessionResult{}, domain.Refuse(domain.ReasonServiceDisabled)
	}
	if o.Kind == "" {
		return port.SessionResult{}, domain.Refuse(domain.ReasonKindNotSet)
	}
	if req.Kind != o.Kind {
		return port.SessionResult{}, domain.Refuse(domain.ReasonKindMismatch)
	}
	if strings.TrimSpace(req.User.ID) == "" {
		return port.SessionResult{}, domain.ErrBadRequest
	}
	if strings.TrimSpace(req.Grant) == "" {
		return port.SessionResult{}, domain.ErrBadRequest
	}
	if !svc.AllowAll && !inAllowlist(req.User, svc.Allowlist) {
		return port.SessionResult{}, domain.Refuse(domain.ReasonNotInAllowlist)
	}
	// โควตาเต็ม "ไม่" ปฏิเสธตั๋ว — ผู้ใช้ต้องเห็นข้อความว่าโควตาเต็ม (spec §8) ไม่ใช่ปุ่มหายไปเฉย ๆ
	// bootstrap บอก notice ให้ widget แสดง · /chat ปฏิเสธเองอีกชั้น

	st := domain.DefaultSettings()
	if s.Settings != nil {
		if got, err := s.Settings.Get(ctx); err == nil {
			st = got
		}
	}
	now := s.Now()
	exp := now.Add(time.Duration(st.TicketTTLMin) * time.Minute)
	// ตั๋วห้ามอยู่นานกว่า grant ที่ host ให้ (กุญแจดอกเล็กจะขอไม่ได้แล้ว)
	if gexp, ok := unverifiedExp(req.Grant); ok && gexp.Before(exp) {
		exp = gexp
	}
	if !exp.After(now) {
		return port.SessionResult{}, domain.ErrBadRequest
	}

	user := req.User
	user.Permissions = s.relevantPermissions(o.Kind, user.Permissions)
	sealed, err := s.Sealer.Seal(req.Grant, TicketBinding(o.ID, svc.ID, user.ID))
	if err != nil {
		return port.SessionResult{}, err
	}
	ticket := domain.AccessTicket{
		ID:          domain.NewID(),
		OfficeID:    o.ID,
		ServiceID:   svc.ID,
		Kind:        o.Kind,
		User:        user,
		SealedGrant: sealed,
		IssuedAt:    now,
		ExpiresAt:   exp,
	}
	tok, err := s.Tickets.Issue(ticket)
	if err != nil {
		return port.SessionResult{}, err
	}

	// last-used ไม่ต้องแม่นระดับวินาที — เขียนเมื่อเก่ากว่า 5 นาที ลดการเขียน DB
	if svc.SecretLastUsedAt == nil || now.Sub(*svc.SecretLastUsedAt) > 5*time.Minute {
		svc.SecretLastUsedAt = &now
		_ = s.Repo.Save(ctx, o)
	}
	return port.SessionResult{Ticket: tok, ExpiresAt: exp.Unix(), TTLSec: int64(exp.Sub(now).Seconds())}, nil
}

// relevantPermissions เก็บเฉพาะ code ที่ connector ของ kind นี้อ้างถึง — ตั๋วเล็ก ไม่พก code เกินจำเป็น
func (s *officeService) relevantPermissions(kind string, perms []string) []string {
	if s.Connectors == nil {
		return perms
	}
	c, ok := s.Connectors.Get(kind)
	if !ok {
		return perms
	}
	want := map[string]bool{}
	for _, r := range c.Permissions.Rules {
		for _, x := range r.AnyOf {
			want[x] = true
		}
		for _, x := range r.AllOf {
			want[x] = true
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, p := range perms {
		if want[p] && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func TicketBinding(officeID, serviceID, userID string) string {
	return officeID + "\x00" + serviceID + "\x00" + userID
}

func inAllowlist(u domain.HostUser, list []string) bool {
	for _, x := range list {
		if u.Matches(x) {
			return true
		}
	}
	return false
}

// CheckTicket ใช้ทุก request ฝั่งแชท: path ต้องตรงตั๋ว + สถานะล่าสุดยังเปิด + secret ยังไม่ถูก revoke
func (s *officeService) CheckTicket(ctx context.Context, publicKey, serviceID string, t domain.AccessTicket) (domain.Office, domain.Service, error) {
	o, err := s.Repo.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return domain.Office{}, domain.Service{}, err
	}
	if o.ID != t.OfficeID || serviceID != t.ServiceID || o.Kind != t.Kind {
		return domain.Office{}, domain.Service{}, domain.ErrTicketInvalid
	}
	svc, ok := o.FindService(serviceID)
	if !ok {
		return domain.Office{}, domain.Service{}, domain.ErrTicketInvalid
	}
	if !o.Enabled {
		return domain.Office{}, domain.Service{}, domain.Refuse(domain.ReasonOfficeDisabled)
	}
	if !svc.Enabled {
		return domain.Office{}, domain.Service{}, domain.Refuse(domain.ReasonServiceDisabled)
	}
	if !s.isBrowser(o.Kind) && !svc.HasActiveSecret() {
		return domain.Office{}, domain.Service{}, domain.Refuse(domain.ReasonNoSecret)
	}
	if !svc.AllowAll && !inAllowlist(t.User, svc.Allowlist) {
		return domain.Office{}, domain.Service{}, domain.Refuse(domain.ReasonNotInAllowlist)
	}
	return o, svc, nil
}

func (s *officeService) isBrowser(kind string) bool {
	if s.Connectors == nil {
		return false
	}
	c, ok := s.Connectors.Get(kind)
	return ok && c.Host.IsBrowser()
}

// OpenBrowserSession — โหมด browser: widget ขอตั๋วเองโดยไม่ผ่าน host
//
// ██ ไม่มี secret / grant · ตัวตนเชื่อตามที่หน้าเว็บบอก (ไม่ได้ตรวจลายเซ็น)
// ██ ปลอดภัยพอเพราะ backend ไม่เคยถือ token ของหลังบ้าน — ข้อมูลทุกชิ้นมาจาก widget ที่ยิงด้วย token ของแอดมินคนนั้นเอง
// ██ ประวัติแชทผูก TokenFP (รอบล็อกอิน) — อ้างชื่อคนอื่นแล้วเปิดประวัติของเขาไม่ได้
func (s *officeService) OpenBrowserSession(ctx context.Context, req port.BrowserSessionRequest) (port.SessionResult, error) {
	o, err := s.Repo.GetByPublicKey(ctx, req.PublicKey)
	if err != nil {
		return port.SessionResult{}, err
	}
	if !o.AllowsOrigin(req.Origin) {
		return port.SessionResult{}, domain.ErrOriginNotAllowed
	}
	if !o.Enabled {
		return port.SessionResult{}, domain.Refuse(domain.ReasonOfficeDisabled)
	}
	if o.Kind == "" {
		return port.SessionResult{}, domain.Refuse(domain.ReasonKindNotSet)
	}
	if !s.isBrowser(o.Kind) {
		return port.SessionResult{}, domain.Refuse(domain.ReasonKindMismatch)
	}
	svc, ok := o.FindService(req.ServiceID)
	if !ok {
		return port.SessionResult{}, domain.Refuse(domain.ReasonNoService)
	}
	if !svc.Enabled {
		return port.SessionResult{}, domain.Refuse(domain.ReasonServiceDisabled)
	}
	if strings.TrimSpace(req.User.ID) == "" || len(req.TokenFP) < 32 {
		return port.SessionResult{}, domain.ErrBadRequest
	}
	if !svc.AllowAll && !inAllowlist(req.User, svc.Allowlist) {
		return port.SessionResult{}, domain.Refuse(domain.ReasonNotInAllowlist)
	}
	st := domain.DefaultSettings()
	if s.Settings != nil {
		if got, err := s.Settings.Get(ctx); err == nil {
			st = got
		}
	}
	now := s.Now()
	exp := now.Add(time.Duration(st.TicketTTLMin) * time.Minute)
	user := req.User
	user.Permissions = s.relevantPermissions(o.Kind, user.Permissions)
	ticket := domain.AccessTicket{
		ID: domain.NewID(), OfficeID: o.ID, ServiceID: svc.ID, Kind: o.Kind, User: user,
		TokenFP: req.TokenFP, IssuedAt: now, ExpiresAt: exp,
	}
	tok, err := s.Tickets.Issue(ticket)
	if err != nil {
		return port.SessionResult{}, err
	}
	return port.SessionResult{Ticket: tok, ExpiresAt: exp.Unix(), TTLSec: int64(exp.Sub(now).Seconds())}, nil
}

// Bootstrap = หน้าตาของ widget สำหรับผู้ถือตั๋ว · origin ของหน้าเว็บต้องลงทะเบียนไว้
func (s *officeService) Bootstrap(ctx context.Context, publicKey, serviceID, origin string, t domain.AccessTicket) (domain.Bootstrap, error) {
	o, err := s.Repo.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return domain.Bootstrap{}, err
	}
	if !o.AllowsOrigin(origin) {
		return domain.Bootstrap{}, domain.ErrOriginNotAllowed
	}
	o, svc, err := s.CheckTicket(ctx, publicKey, serviceID, t)
	if err != nil {
		if r, ok := err.(*domain.RefusalError); ok {
			return domain.Bootstrap{Enabled: false, Reason: r.Reason}, nil
		}
		return domain.Bootstrap{}, err
	}
	notice := ""
	if s.Quota != nil {
		if qs, err := s.Quota.Status(ctx, o, svc); err == nil && qs.Cut {
			notice = domain.ReasonQuotaExceeded
		}
	}
	return domain.Bootstrap{
		Enabled:         true,
		Notice:          notice,
		OfficeID:        o.ID,
		ServiceID:       svc.ID,
		ServiceLabel:    svc.Label,
		Kind:            o.Kind,
		IsHidden:        o.IsHidden,
		AvatarURL:       svc.AvatarURL,
		DisplayName:     svc.DisplayName,
		Greeting:        svc.Greeting,
		Theme:           o.Theme,
		Placement:       o.Placement,
		TicketExpiresAt: t.ExpiresAt.Unix(),
	}, nil
}

func (s *officeService) touchSave(ctx context.Context, o domain.Office, actor string) (domain.Office, error) {
	o.UpdatedAt = s.Now()
	o.UpdatedBy = actor
	if err := s.Repo.Save(ctx, o); err != nil {
		return domain.Office{}, err
	}
	return o, nil
}

// ResolveByOrigin — หา office จากโดเมนที่เรียกเข้ามา (ใช้เมื่อ snippet เหมือนกันทุกโดเมน)
func (s *officeService) ResolveByOrigin(ctx context.Context, origin string) (domain.Office, error) {
	if strings.TrimSpace(origin) == "" {
		return domain.Office{}, domain.ErrOriginRequired
	}
	norm, err := domain.NormalizeOrigin(origin)
	if err != nil {
		return domain.Office{}, domain.ErrOriginNotAllowed
	}
	o, err := s.Repo.GetByOrigin(ctx, norm)
	if err == domain.ErrNotFound {
		return domain.Office{}, domain.ErrOriginNotAllowed
	}
	return o, err
}

// OriginIssues หาปัญหาในข้อมูลเดิมที่ทำให้หา office จากโดเมนผิดพลาดได้ — ใช้เตือนใน log ตอนเริ่มระบบ
func (s *officeService) OriginIssues(ctx context.Context) ([]string, error) {
	list, err := s.Repo.List(ctx)
	if err != nil {
		return nil, err
	}
	owner := map[string]string{}
	issues := []string{}
	for _, o := range list {
		for _, raw := range o.AllowedOrigins {
			norm, err := domain.NormalizeOrigin(raw)
			if err != nil {
				issues = append(issues, fmt.Sprintf("office %s: โดเมน %q รูปแบบไม่ถูกต้อง — จะไม่มีใครเข้าผ่านโดเมนนี้ได้", o.ID, raw))
				continue
			}
			if norm != raw {
				issues = append(issues, fmt.Sprintf("office %s: โดเมน %q ยังไม่อยู่ในรูปแบบมาตรฐาน (%s) — บันทึก office นี้ใหม่จากคอนโซลหนึ่งครั้ง", o.ID, raw, norm))
			}
			if prev, ok := owner[norm]; ok && prev != o.ID {
				issues = append(issues, fmt.Sprintf("โดเมน %s อยู่ทั้งใน office %s และ %s — ระบบจะเลือก %s", norm, prev, o.ID, prev))
				continue
			}
			owner[norm] = o.ID
		}
	}
	return issues, nil
}

// ensureOriginsFree กันไม่ให้โดเมนเดียวอยู่ 2 office — ไม่งั้นระบบแยกไม่ได้ว่าเป็นลูกค้าเจ้าไหน
func (s *officeService) ensureOriginsFree(ctx context.Context, selfID string, origins []string) error {
	list, err := s.Repo.List(ctx)
	if err != nil {
		return err
	}
	for _, o := range list {
		if o.ID == selfID {
			continue
		}
		for _, want := range origins {
			if o.AllowsOrigin(want) {
				return &domain.OriginTakenError{Origin: want, OfficeID: o.ID, OfficeLabel: o.Label}
			}
		}
	}
	return nil
}

// normalizeOrigins ทำทุกโดเมนให้เป็นรูปแบบมาตรฐาน ตัดบรรทัดว่างและตัวซ้ำทิ้ง
func normalizeOrigins(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		v, err := domain.NormalizeOrigin(raw)
		if err != nil {
			return nil, err
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, nil
}

// changedFieldNames รวมชื่อ field ที่เปลี่ยนไว้ใส่ใน summary ให้อ่านจากตารางได้เลยไม่ต้องกดขยาย
func changedFieldNames(changes []domain.FieldChange) string {
	names := make([]string, len(changes))
	for i, c := range changes {
		names[i] = c.Field
	}
	return strings.Join(names, ", ")
}
