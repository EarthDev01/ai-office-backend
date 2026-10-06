package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type officeService struct {
	repo   port.OfficeRepository
	groups port.OfficeGroupRepository
	audit  port.AuditRecorder
	kinds  func(kind string) bool // kind ที่มี connector โหลดอยู่ · nil = ไม่ตรวจ
}

// OfficeOption ตั้งค่าเพิ่มของ office service
type OfficeOption func(*officeService)

// WithKinds = ตรวจ kind ที่ตั้งจากคอนโซลว่ามี connector จริง (ค่าว่างผ่านเสมอ = kind ตั้งต้นของระบบ)
func WithKinds(valid func(kind string) bool) OfficeOption {
	return func(s *officeService) { s.kinds = valid }
}

// audit เป็น nil ได้ (ไม่บันทึกประวัติ)
//
// บนหน้าจอเรียก Office ว่า "domain" (1 ตัว = 1 URL) และจัดเป็นกลุ่มด้วย groups
func NewOfficeService(repo port.OfficeRepository, groups port.OfficeGroupRepository, audit port.AuditRecorder, opts ...OfficeOption) port.OfficeService {
	s := &officeService{repo: repo, groups: groups, audit: auditOr(audit)}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *officeService) checkKind(kind string) (string, error) {
	kind = strings.TrimSpace(kind)
	if kind != "" && s.kinds != nil && !s.kinds(kind) {
		return "", fmt.Errorf("ไม่รู้จักชนิดหลังบ้าน %q", kind)
	}
	return kind, nil
}

// field ที่ไม่นับเป็น "การแก้ไข" ในประวัติ — services มีเหตุการณ์ของตัวเองแยก
var officeDiffSkip = []string{"updated_at", "updated_by", "created_at", "services"}

var (
	idPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)
	allowedThemes    = map[string]bool{"auto": true, "light": true, "dark": true}
	allowedPositions = map[string]bool{"bottom-right": true, "bottom-left": true}
	hexColorPattern  = regexp.MustCompile(`^#[0-9a-f]{6}$`)
)

func (s *officeService) List(ctx context.Context) ([]domain.Office, error) { return s.repo.List(ctx) }

func (s *officeService) Get(ctx context.Context, id string) (domain.Office, error) {
	return s.repo.Get(ctx, id)
}

func (s *officeService) Create(ctx context.Context, in domain.CreateOffice, actor string) (domain.Office, error) {
	id, label := strings.TrimSpace(in.ID), strings.TrimSpace(in.Label)
	if !idPattern.MatchString(id) {
		return domain.Office{}, fmt.Errorf("รหัสต้องเป็น a-z A-Z 0-9 _ - ยาวไม่เกิน 40 ตัว")
	}
	if _, err := s.repo.Get(ctx, id); err == nil {
		return domain.Office{}, domain.ErrConflict
	}
	if err := s.requireGroup(ctx, in.GroupID); err != nil {
		return domain.Office{}, err
	}
	origins, err := normalizeOrigins([]string{in.Origin})
	if err != nil {
		return domain.Office{}, err
	}
	if err := s.ensureOriginsFree(ctx, id, origins); err != nil {
		return domain.Office{}, err
	}
	kind, err := s.checkKind(in.Kind)
	if err != nil {
		return domain.Office{}, err
	}
	if label == "" {
		label = id
	}
	o := domain.NewOffice(id, label)
	o.Kind = kind
	o.GroupID = in.GroupID
	o.AllowedOrigins = origins
	o.UpdatedBy = actor
	if err := s.repo.Save(ctx, o); err != nil {
		return domain.Office{}, err
	}
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditOfficeCreate, TargetType: "office", TargetID: o.ID, TargetLabel: o.Label,
		Summary: fmt.Sprintf("สร้าง domain %q (%s)", o.Label, o.ID),
	})
	return o, nil
}

// requireGroup — domain ทุกตัวต้องอยู่ในกลุ่มที่มีอยู่จริง
func (s *officeService) requireGroup(ctx context.Context, groupID string) error {
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("ต้องเลือกกลุ่ม")
	}
	if _, err := s.groups.Get(ctx, groupID); err != nil {
		return fmt.Errorf("ไม่พบกลุ่ม %s", groupID)
	}
	return nil
}

func (s *officeService) Update(ctx context.Context, id string, p domain.UpdateOffice, actor string) (domain.Office, error) {
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Office{}, err
	}
	before := o
	before.AllowedOrigins = append([]string(nil), o.AllowedOrigins...)

	if p.Label != nil {
		o.Label = *p.Label
	}
	if p.AllowedOrigins != nil {
		clean, err := normalizeOrigins(*p.AllowedOrigins)
		if err != nil {
			return domain.Office{}, err
		}
		if len(clean) > 1 {
			return domain.Office{}, fmt.Errorf("1 domain ใส่ URL ได้ 1 ตัว — URL อื่นให้เพิ่มเป็น domain ใหม่")
		}
		if err := s.ensureOriginsFree(ctx, o.ID, clean); err != nil {
			return domain.Office{}, err
		}
		o.AllowedOrigins = clean
	}
	if p.GroupID != nil {
		if err := s.requireGroup(ctx, *p.GroupID); err != nil {
			return domain.Office{}, err
		}
		o.GroupID = *p.GroupID
	}
	if p.HostAPIBase != nil {
		o.HostAPIBase = ""
		if strings.TrimSpace(*p.HostAPIBase) != "" {
			v, err := domain.NormalizeHostAPIBase(*p.HostAPIBase)
			if err != nil {
				return domain.Office{}, err
			}
			o.HostAPIBase = v
		}
	}
	if p.Kind != nil {
		kind, err := s.checkKind(*p.Kind)
		if err != nil {
			return domain.Office{}, err
		}
		o.Kind = kind
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
	if p.AccentColor != nil {
		c := strings.ToLower(strings.TrimSpace(*p.AccentColor))
		if c != "" && !hexColorPattern.MatchString(c) {
			return domain.Office{}, fmt.Errorf("สีต้องเป็นรหัส #rrggbb เช่น #0f6e63")
		}
		o.Accent = c
	}
	if p.AccentColors != nil {
		// เลือกเอง = ไล่สีเท่านั้น: 2–4 สี · ว่าง = ไม่ตั้ง (ใช้สีของเว็บ / สีตั้งต้น)
		in := *p.AccentColors
		if len(in) == 1 || len(in) > 4 {
			return domain.Office{}, fmt.Errorf("สีไล่ต้องมี 2–4 สี")
		}
		out := make([]string, 0, len(in))
		for _, c := range in {
			c = strings.ToLower(strings.TrimSpace(c))
			if !hexColorPattern.MatchString(c) {
				return domain.Office{}, fmt.Errorf("สีต้องเป็นรหัส #rrggbb เช่น #2563eb")
			}
			out = append(out, c)
		}
		o.AccentColors = out
	}
	if p.ColorSource != nil {
		switch *p.ColorSource {
		case "", "custom":
			o.ColorSource = ""
		case "site":
			o.ColorSource = "site"
		default:
			return domain.Office{}, fmt.Errorf("ที่มาของสีต้องเป็น custom หรือ site")
		}
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
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditOfficeUpdate, TargetType: "office", TargetID: saved.ID, TargetLabel: saved.Label,
			Summary: fmt.Sprintf("แก้ไข office %q (%s) %d รายการ: %s", saved.Label, saved.ID, len(changes), changedFieldNames(changes)),
			Changes: changes,
		})
	}
	return saved, nil
}

func (s *officeService) Delete(ctx context.Context, id string) error {
	// อ่านก่อนลบ เพื่อให้ประวัติยังบอกได้ว่าลบ office ชื่ออะไร มี service อะไรบ้าง
	o, getErr := s.repo.Get(ctx, id)
	if err := s.repo.Delete(ctx, id); err != nil {
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
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditOfficeDelete, TargetType: "office", TargetID: id, TargetLabel: label,
		Summary: fmt.Sprintf("ลบ office %q (%s)", label, id),
		Meta:    meta,
	})
	return nil
}

func (s *officeService) AddService(ctx context.Context, officeID, serviceID, label, actor string) (domain.Office, error) {
	o, err := s.repo.Get(ctx, officeID)
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
	o.Services = append(o.Services, domain.DefaultService(serviceID, label))
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditServiceCreate, TargetType: "service", TargetID: serviceID, TargetLabel: label,
		Summary: fmt.Sprintf("เพิ่ม service %q (%s) ใน office %q", label, serviceID, saved.Label),
		Meta:    map[string]string{"office_id": saved.ID, "office_label": saved.Label},
	})
	return saved, nil
}

func (s *officeService) UpdateService(ctx context.Context, officeID, serviceID string, p domain.UpdateService, actor string) (domain.Office, error) {
	o, err := s.repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	idx := -1
	for i := range o.Services {
		if o.Services[i].ID == serviceID {
			idx = i
			break
		}
	}
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
	if p.Allowlist != nil {
		svc.Allowlist = *p.Allowlist
	}
	if p.DisplayName != nil {
		svc.DisplayName = *p.DisplayName
	}
	if p.Greeting != nil {
		svc.Greeting = *p.Greeting
	}
	if p.AvatarURL != nil {
		v := strings.TrimSpace(*p.AvatarURL)
		if err := domain.ValidateLookValue(v, false); err != nil {
			return domain.Office{}, fmt.Errorf("รูปผู้ช่วย: %v", err)
		}
		svc.AvatarURL = v
	}
	if p.Tagline != nil {
		v := strings.TrimSpace(*p.Tagline)
		if utf8.RuneCountInString(v) > 80 {
			return domain.Office{}, fmt.Errorf("คำโปรยยาวเกิน 80 ตัวอักษร")
		}
		svc.Tagline = v
	}
	if p.LauncherIcon != nil {
		v := strings.TrimSpace(*p.LauncherIcon)
		if v != "" && !strings.HasPrefix(v, "asset:launchers/") {
			return domain.Office{}, fmt.Errorf("รูปปุ่มเปิดแชทต้องเลือกจากคลังรูป")
		}
		if err := domain.ValidateLookValue(v, false); err != nil {
			return domain.Office{}, fmt.Errorf("รูปปุ่มเปิดแชท: %v", err)
		}
		svc.LauncherIcon = v
	}
	if p.Background != nil {
		v := strings.TrimSpace(*p.Background)
		if err := domain.ValidateLookValue(v, true); err != nil {
			return domain.Office{}, fmt.Errorf("พื้นหลัง: %v", err)
		}
		svc.Background = v
	}
	after := *svc
	saved, err := s.touchSave(ctx, o, actor)
	if err != nil {
		return domain.Office{}, err
	}
	if changes := domain.DiffFields(before, after); len(changes) > 0 {
		s.audit.Record(ctx, domain.AuditEntry{
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
	o, err := s.repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	out := o.Services[:0]
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
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditServiceDelete, TargetType: "service", TargetID: removed.ID, TargetLabel: removed.Label,
		Summary: fmt.Sprintf("ลบ service %q (%s) ออกจาก office %q", removed.Label, removed.ID, saved.Label),
		Meta:    map[string]string{"office_id": saved.ID, "office_label": saved.Label},
	})
	return saved, nil
}

// ResolveByOrigin — โดเมนที่เรียกเข้ามาคือตัวบอกว่าเป็นลูกค้าเจ้าไหน
//
// fetch ข้ามโดเมนจากเบราว์เซอร์ส่ง Origin มาเสมอ และ script ในหน้าเว็บปลอมค่านี้ไม่ได้
func (s *officeService) ResolveByOrigin(ctx context.Context, origin string) (domain.Office, error) {
	if strings.TrimSpace(origin) == "" {
		return domain.Office{}, domain.ErrOriginRequired
	}
	norm, err := domain.NormalizeOrigin(origin)
	if err != nil {
		return domain.Office{}, domain.ErrOriginNotAllowed
	}
	o, err := s.repo.GetByOrigin(ctx, norm)
	if err == domain.ErrNotFound {
		return domain.Office{}, domain.ErrOriginNotAllowed
	}
	return o, err
}

// OriginIssues หาปัญหาในข้อมูลเดิมที่ทำให้หา office จากโดเมนผิดพลาดได้ — ใช้เตือนใน log ตอนเริ่มระบบ
func (s *officeService) OriginIssues(ctx context.Context) ([]string, error) {
	list, err := s.repo.List(ctx)
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
	list, err := s.repo.List(ctx)
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

// Bootstrap ตัดสินให้เสร็จที่ server ว่า widget ควรโผล่ไหม
//
// office มาจาก ResolveByOrigin แล้ว (middleware) — ตรงนี้ตรวจต่อเรื่อง office เปิด / token / service
func (s *officeService) Bootstrap(ctx context.Context, o domain.Office, serviceID string, caller domain.Caller) (domain.Bootstrap, error) {
	if !o.Enabled {
		return domain.Bootstrap{Enabled: false, Reason: "office_disabled"}, nil
	}
	// token ของ office อื่นเอามาใช้กับ key นี้ไม่ได้
	if caller.OfficeID != o.ID {
		return domain.Bootstrap{Enabled: false, Reason: "wrong_office"}, nil
	}

	// ██ การตรวจ service — 2 ด่าน
	//
	// หน้าเว็บเป็นคนบอกว่ากำลังเปิด service ไหน (localStorage["web-service"])
	// ไม่มี session ฝั่ง server ที่จำไว้ — เราจึงต้องตรวจเอง
	if serviceID == "" {
		return domain.Bootstrap{Enabled: false, Reason: "no_service"}, nil
	}

	// ด่าน 1: Role.ListService ของแอดมินคนนั้น
	//
	// ██ ข้อมูลจริงของ demo-staging_office: list_service ว่างทั้ง 9 employee และทั้ง 4 role
	// ██ เพราะ office ที่มี service เดียวไม่มีอะไรให้จำกัด → ว่าง = ไม่จำกัด
	// ██ แต่ถ้ามีค่า (office ที่มีหลาย service) ต้องบังคับตามนั้น
	if !caller.ServiceAllowed(serviceID) {
		return domain.Bootstrap{}, domain.ErrServiceNotAllowed
	}

	// ด่าน 2: service ต้องมีอยู่ใน office นี้จริง
	// กันการชี้ไป service ของ office อื่น แม้ ListService จะว่าง
	svc, ok := o.FindService(serviceID)
	if !ok {
		return domain.Bootstrap{Enabled: false, Reason: "service_not_in_office"}, nil
	}
	if !svc.Enabled {
		return domain.Bootstrap{Enabled: false, Reason: "service_disabled"}, nil
	}
	// ██ ไม่มี allowlist แล้ว — ใครก็ตามที่ล็อกอิน office สำเร็จ + มีสิทธิ์ service นี้
	// ██ (Role.ListService ตรวจข้างบนแล้ว) ใช้ AI ได้เลย เปิดให้ทุกคนที่ล็อกอิน

	return domain.Bootstrap{
		Enabled:      true,
		OfficeID:     o.ID,
		ServiceID:    svc.ID,
		ServiceLabel: svc.Label,
		IsHidden:     o.IsHidden,
		AvatarURL:    svc.AvatarURL,
		DisplayName:  svc.DisplayName,
		Greeting:     svc.Greeting,
		Tagline:      svc.Tagline,
		LauncherIcon: svc.LauncherIcon,
		Background:   svc.Background,
		Theme:        o.Theme,
		AccentColor:  o.Accent,
		AccentColors: o.AccentColors,
		ColorSource:  o.ColorSource,
		Placement:    o.Placement,
	}, nil
}

func (s *officeService) touchSave(ctx context.Context, o domain.Office, actor string) (domain.Office, error) {
	o.UpdatedAt = time.Now()
	o.UpdatedBy = actor
	if err := s.repo.Save(ctx, o); err != nil {
		return domain.Office{}, err
	}
	return o, nil
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

// ---- กลุ่มของ domain ----

func (s *officeService) ListGroups(ctx context.Context) ([]domain.OfficeGroup, error) {
	return s.groups.List(ctx)
}

func (s *officeService) groupNameTaken(ctx context.Context, name, selfID string) error {
	list, err := s.groups.List(ctx)
	if err != nil {
		return err
	}
	for _, g := range list {
		if g.ID != selfID && strings.EqualFold(g.Name, name) {
			return fmt.Errorf("มีกลุ่มชื่อ %q อยู่แล้ว", name)
		}
	}
	return nil
}

func (s *officeService) CreateGroup(ctx context.Context, name, actor string) (domain.OfficeGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 60 {
		return domain.OfficeGroup{}, fmt.Errorf("ชื่อกลุ่มต้องไม่ว่างและยาวไม่เกิน 60 ตัวอักษร")
	}
	if err := s.groupNameTaken(ctx, name, ""); err != nil {
		return domain.OfficeGroup{}, err
	}
	now := time.Now()
	g := domain.OfficeGroup{ID: domain.NewID(), Name: name, CreatedAt: now, UpdatedAt: now, UpdatedBy: actor}
	if err := s.groups.Save(ctx, g); err != nil {
		return domain.OfficeGroup{}, err
	}
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditGroupCreate, TargetType: "group", TargetID: g.ID, TargetLabel: g.Name,
		Summary: fmt.Sprintf("สร้างกลุ่ม %q", g.Name),
	})
	return g, nil
}

func (s *officeService) RenameGroup(ctx context.Context, id, name, actor string) (domain.OfficeGroup, error) {
	g, err := s.groups.Get(ctx, id)
	if err != nil {
		return g, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 60 {
		return g, fmt.Errorf("ชื่อกลุ่มต้องไม่ว่างและยาวไม่เกิน 60 ตัวอักษร")
	}
	if err := s.groupNameTaken(ctx, name, id); err != nil {
		return g, err
	}
	before := g.Name
	g.Name, g.UpdatedAt, g.UpdatedBy = name, time.Now(), actor
	if err := s.groups.Save(ctx, g); err != nil {
		return g, err
	}
	if before != name {
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditGroupUpdate, TargetType: "group", TargetID: g.ID, TargetLabel: g.Name,
			Summary: fmt.Sprintf("เปลี่ยนชื่อกลุ่ม %q เป็น %q", before, name),
			Changes: []domain.FieldChange{{Field: "name", Before: before, After: name}},
		})
	}
	return g, nil
}

func (s *officeService) DeleteGroup(ctx context.Context, id, actor string) error {
	g, err := s.groups.Get(ctx, id)
	if err != nil {
		return err
	}
	list, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	n := 0
	for _, o := range list {
		if o.GroupID == id {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("กลุ่ม %q ยังมี %d domain — ย้ายหรือลบ domain ออกก่อน", g.Name, n)
	}
	if err := s.groups.Delete(ctx, id); err != nil {
		return err
	}
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditGroupDelete, TargetType: "group", TargetID: g.ID, TargetLabel: g.Name,
		Summary: fmt.Sprintf("ลบกลุ่ม %q", g.Name),
	})
	return nil
}
