// Package filestore เก็บ office/service ลงไฟล์ JSON
//
// เป็น adapter ของ port.OfficeRepository — สลับไปใช้ MongoDB ได้โดยแก้
// _cmd/main.go บรรทัดเดียว ไม่ต้องแตะ service หรือ handler (02-SPEC §7)
package filestore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type officeRepo struct {
	mu    sync.RWMutex
	path  string
	items map[string]domain.Office
}

func NewOfficeRepository(path string, seed []domain.Office) (port.OfficeRepository, error) {
	r := &officeRepo{path: path, items: map[string]domain.Office{}}
	if err := r.load(); err != nil {
		return nil, err
	}
	if len(r.items) == 0 {
		for _, o := range seed {
			r.items[o.ID] = o
		}
		if err := r.flush(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *officeRepo) load() error {
	b, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []persistOffice
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, p := range list {
		o := p.toDomain()
		r.items[o.ID] = o
	}
	return nil
}

// persistOffice = รูปที่เขียนลงไฟล์ — domain ซ่อน hash ของ secret ด้วย json:"-" (ไม่ให้หลุดไปคอนโซล)
// แต่ไฟล์ต้องเก็บไว้ ไม่งั้นรีสตาร์ตแล้ว secret หาย
type persistOffice struct {
	domain.Office
	Services []persistService `json:"services"`
}

type persistService struct {
	domain.Service
	AllowAll          *bool  `json:"allow_all"` // ไม่มี field (ไฟล์รุ่นเก่า) = true ตามค่าเริ่มต้น D-74
	SecretKeyHash     string `json:"secret_key_hash,omitempty"`
	SecretKeyPrevHash string `json:"secret_key_prev_hash,omitempty"`
}

func fromDomain(o domain.Office) persistOffice {
	p := persistOffice{Office: o, Services: make([]persistService, len(o.Services))}
	for i, s := range o.Services {
		allow := s.AllowAll
		p.Services[i] = persistService{Service: s, AllowAll: &allow, SecretKeyHash: s.SecretKeyHash, SecretKeyPrevHash: s.SecretKeyPrevHash}
	}
	return p
}

func (p persistOffice) toDomain() domain.Office {
	o := p.Office
	o.Services = make([]domain.Service, len(p.Services))
	for i, ps := range p.Services {
		s := ps.Service
		s.AllowAll = ps.AllowAll == nil || *ps.AllowAll
		s.SecretKeyHash = ps.SecretKeyHash
		s.SecretKeyPrevHash = ps.SecretKeyPrevHash
		if s.Quota.TempIncreases == nil {
			s.Quota.TempIncreases = []domain.TempIncrease{}
		}
		o.Services[i] = s
	}
	return o
}

func (r *officeRepo) flush() error {
	list := make([]persistOffice, 0, len(r.items))
	for _, o := range r.items {
		list = append(list, fromDomain(o))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })

	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	// เขียนไฟล์ชั่วคราวก่อนแล้ว rename — กันไฟล์พังถ้าดับกลางคัน
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func (r *officeRepo) List(ctx context.Context) ([]domain.Office, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.Office, 0, len(r.items))
	for _, o := range r.items {
		list = append(list, o)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

func (r *officeRepo) Get(ctx context.Context, id string) (domain.Office, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.items[id]
	if !ok {
		return domain.Office{}, domain.ErrNotFound
	}
	return o, nil
}

func (r *officeRepo) GetByPublicKey(ctx context.Context, key string) (domain.Office, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if key == "" {
		return domain.Office{}, domain.ErrNotFound
	}
	for _, o := range r.items {
		if o.PublicKey == key {
			return o, nil
		}
	}
	return domain.Office{}, domain.ErrNotFound
}

// GetByOrigin — เรียง id ให้ผลนิ่ง เผื่อข้อมูลเก่ามีโดเมนซ้ำค้างอยู่ (ของใหม่ถูกกันไว้ที่ service แล้ว)
func (r *officeRepo) GetByOrigin(ctx context.Context, origin string) (domain.Office, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if origin == "" {
		return domain.Office{}, domain.ErrNotFound
	}
	ids := make([]string, 0, len(r.items))
	for id := range r.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		o := r.items[id]
		if o.AllowsOrigin(origin) {
			return o, nil
		}
	}
	return domain.Office{}, domain.ErrNotFound
}

func (r *officeRepo) Save(ctx context.Context, o domain.Office) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[o.ID] = o
	return r.flush()
}

func (r *officeRepo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return r.flush()
}

func (r *officeRepo) AllOrigins(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]bool{}
	out := []string{}
	for _, o := range r.items {
		for _, origin := range o.AllowedOrigins {
			if !seen[origin] {
				seen[origin] = true
				out = append(out, origin)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}
