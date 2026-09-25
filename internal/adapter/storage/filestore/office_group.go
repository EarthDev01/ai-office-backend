package filestore

import (
	"context"
	"sort"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// officeGroupRepo เก็บกลุ่มของ domain เป็นไฟล์ JSON ไฟล์เดียว (เขียนทับทั้งไฟล์)
type officeGroupRepo struct {
	mu    sync.Mutex
	path  string
	items map[string]domain.OfficeGroup
}

func NewOfficeGroupRepository(path string) (port.OfficeGroupRepository, error) {
	r := &officeGroupRepo{path: path, items: map[string]domain.OfficeGroup{}}
	var list []domain.OfficeGroup
	if err := readJSON(path, &list); err != nil {
		return nil, err
	}
	for _, g := range list {
		r.items[g.ID] = g
	}
	return r, nil
}

func (r *officeGroupRepo) flush() error {
	list := make([]domain.OfficeGroup, 0, len(r.items))
	for _, g := range r.items {
		list = append(list, g)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return writeJSON(r.path, list)
}

func (r *officeGroupRepo) List(_ context.Context) ([]domain.OfficeGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.OfficeGroup, 0, len(r.items))
	for _, g := range r.items {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r *officeGroupRepo) Get(_ context.Context, id string) (domain.OfficeGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.items[id]
	if !ok {
		return g, domain.ErrNotFound
	}
	return g, nil
}

func (r *officeGroupRepo) Save(_ context.Context, g domain.OfficeGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[g.ID] = g
	return r.flush()
}

func (r *officeGroupRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return r.flush()
}
