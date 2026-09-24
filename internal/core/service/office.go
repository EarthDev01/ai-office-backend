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

type officeService struct {
	repo port.OfficeRepository
}

func NewOfficeService(repo port.OfficeRepository) port.OfficeService {
	return &officeService{repo: repo}
}

var (
	idPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)
	allowedThemes    = map[string]bool{"auto": true, "light": true, "dark": true}
	allowedPositions = map[string]bool{"bottom-right": true, "bottom-left": true}
)

func (s *officeService) List(ctx context.Context) ([]domain.Office, error) { return s.repo.List(ctx) }

func (s *officeService) Get(ctx context.Context, id string) (domain.Office, error) {
	return s.repo.Get(ctx, id)
}

func (s *officeService) Create(ctx context.Context, id, label, actor string) (domain.Office, error) {
	if !idPattern.MatchString(id) {
		return domain.Office{}, fmt.Errorf("id ต้องเป็น a-z A-Z 0-9 _ - ยาวไม่เกิน 40 ตัว")
	}
	if _, err := s.repo.Get(ctx, id); err == nil {
		return domain.Office{}, domain.ErrConflict
	}
	if strings.TrimSpace(label) == "" {
		label = id
	}
	o := domain.NewOffice(id, label)
	o.UpdatedBy = actor
	if err := s.repo.Save(ctx, o); err != nil {
		return domain.Office{}, err
	}
	return o, nil
}

func (s *officeService) Update(ctx context.Context, id string, p domain.UpdateOffice, actor string) (domain.Office, error) {
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Office{}, err
	}

	if p.Label != nil {
		o.Label = *p.Label
	}
	if p.AllowedOrigins != nil {
		clean, err := normalizeOrigins(*p.AllowedOrigins)
		if err != nil {
			return domain.Office{}, err
		}
		o.AllowedOrigins = clean
	}
	if p.BackofficeAPIURL != nil {
		o.BackofficeAPIURL = strings.TrimRight(strings.TrimSpace(*p.BackofficeAPIURL), "/")
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

	return s.touchSave(ctx, o, actor)
}

func (s *officeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// RotateKey ทำให้ snippet เดิมใช้ไม่ได้ทันที — คนเรียกต้องรู้ตัว
func (s *officeService) RotateKey(ctx context.Context, id, actor string) (domain.Office, error) {
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Office{}, err
	}
	o.PublicKey = domain.NewPublicKey()
	return s.touchSave(ctx, o, actor)
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
	return s.touchSave(ctx, o, actor)
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
		svc.AvatarURL = *p.AvatarURL
	}
	return s.touchSave(ctx, o, actor)
}

func (s *officeService) RemoveService(ctx context.Context, officeID, serviceID, actor string) (domain.Office, error) {
	o, err := s.repo.Get(ctx, officeID)
	if err != nil {
		return domain.Office{}, err
	}
	out := o.Services[:0]
	found := false
	for _, svc := range o.Services {
		if svc.ID == serviceID {
			found = true
			continue
		}
		out = append(out, svc)
	}
	if !found {
		return domain.Office{}, domain.ErrNotFound
	}
	o.Services = out
	return s.touchSave(ctx, o, actor)
}

func (s *officeService) ResolveByPublicKey(ctx context.Context, publicKey string) (domain.Office, error) {
	return s.repo.GetByPublicKey(ctx, publicKey)
}

// Bootstrap ตัดสินให้เสร็จที่ server ว่า widget ควรโผล่ไหม
func (s *officeService) Bootstrap(ctx context.Context, publicKey, serviceID, origin string, caller domain.Caller) (domain.Bootstrap, error) {
	o, err := s.repo.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return domain.Bootstrap{}, err
	}
	// key ที่หลุดออกไปต้องใช้จากโดเมนอื่นไม่ได้
	if !o.AllowsOrigin(origin) {
		return domain.Bootstrap{}, domain.ErrOriginNotAllowed
	}
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
	if len(caller.Services) > 0 && !caller.CanAccessService(serviceID) {
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
		Theme:        o.Theme,
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

func normalizeOrigins(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimRight(strings.TrimSpace(raw), "/")
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			return nil, fmt.Errorf("origin ต้องขึ้นต้นด้วย http:// หรือ https:// — เจอ %q", raw)
		}
		if strings.Count(v, "/") != 2 {
			return nil, fmt.Errorf("origin ต้องมีแค่ scheme กับ host ห้ามมี path — เจอ %q", raw)
		}
		out = append(out, v)
	}
	return out, nil
}
