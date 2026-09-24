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
	var list []domain.Office
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, o := range list {
		r.items[o.ID] = o
	}
	return nil
}

func (r *officeRepo) flush() error {
	list := make([]domain.Office, 0, len(r.items))
	for _, o := range r.items {
		list = append(list, o)
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
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
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

func (r *officeRepo) GetByOrigin(ctx context.Context, origin string) (domain.Office, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if origin == "" {
		return domain.Office{}, domain.ErrNotFound
	}
	// เรียง id ให้ผลนิ่ง เผื่อข้อมูลเก่ามีโดเมนซ้ำค้างอยู่ (ของใหม่ถูกกันไว้ที่ service แล้ว)
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
