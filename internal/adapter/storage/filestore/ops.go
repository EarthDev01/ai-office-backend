package filestore

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// writeJSON เขียนทับทั้งไฟล์ (ไฟล์ชั่วคราวแล้ว rename กันไฟล์พังครึ่งทาง)
func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// ---- settings ----

type settingsRepo struct {
	mu   sync.Mutex
	path string
}

func NewSettingsRepository(path string) port.SettingsRepository { return &settingsRepo{path: path} }

func (r *settingsRepo) Get(_ context.Context) (domain.Settings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		return domain.Settings{}, domain.ErrNotFound
	}
	var s domain.Settings
	return s, readJSON(r.path, &s)
}

func (r *settingsRepo) Save(_ context.Context, s domain.Settings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return writeJSON(r.path, s)
}

// ---- usage ----

type usageRepo struct {
	mu   sync.Mutex
	path string
	data struct {
		Periods map[string]domain.UsagePeriod `json:"periods"`
		Rollups map[string]domain.DailyRollup `json:"rollups"`
	}
}

func NewUsageRepository(path string) (port.UsageRepository, error) {
	r := &usageRepo{path: path}
	if err := readJSON(path, &r.data); err != nil {
		return nil, err
	}
	if r.data.Periods == nil {
		r.data.Periods = map[string]domain.UsagePeriod{}
	}
	if r.data.Rollups == nil {
		r.data.Rollups = map[string]domain.DailyRollup{}
	}
	return r, nil
}

func (r *usageRepo) AddPeriod(_ context.Context, officeID, serviceID, period string, d domain.RollupDelta, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := officeID + "|" + serviceID + "|" + period
	p := r.data.Periods[id]
	p.ID, p.OfficeID, p.ServiceID, p.Period, p.UpdatedAt = id, officeID, serviceID, period, at
	p.Questions += d.Questions
	p.InputTokens += d.InputTokens
	p.OutputTokens += d.OutputTokens
	p.CacheRead += d.CacheRead
	p.CacheWrite += d.CacheWrite
	r.data.Periods[id] = p
	return writeJSON(r.path, r.data)
}

func (r *usageRepo) AddRollup(_ context.Context, officeID, serviceID, date string, d domain.RollupDelta) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := officeID + "|" + serviceID + "|" + date
	x := r.data.Rollups[id]
	x.ID, x.OfficeID, x.ServiceID, x.Date = id, officeID, serviceID, date
	x.Conversations += d.Conversations
	x.Questions += d.Questions
	x.InputTokens += d.InputTokens
	x.OutputTokens += d.OutputTokens
	x.CacheRead += d.CacheRead
	x.CacheWrite += d.CacheWrite
	x.Refusals += d.Refusals
	x.ToolErrors += d.ToolErrors
	x.GuardHits += d.GuardHits
	x.Correct += d.Correct
	x.Wrong += d.Wrong
	r.data.Rollups[id] = x
	return writeJSON(r.path, r.data)
}

func (r *usageRepo) ListPeriods(_ context.Context, period string) ([]domain.UsagePeriod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.UsagePeriod{}
	for _, p := range r.data.Periods {
		if p.Period == period {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *usageRepo) ListRollups(_ context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.DailyRollup{}
	for _, x := range r.data.Rollups {
		if x.Date < from || x.Date > to || (officeID != "" && x.OfficeID != officeID) || (serviceID != "" && x.ServiceID != serviceID) {
			continue
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// ---- deletion_requests ----

type deletionRepo struct {
	mu    sync.Mutex
	path  string
	items []domain.DeletionRequest
}

func NewDeletionRepository(path string) (port.DeletionRepository, error) {
	r := &deletionRepo{path: path}
	return r, readJSON(path, &r.items)
}

func (r *deletionRepo) Create(_ context.Context, d domain.DeletionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, d)
	return writeJSON(r.path, r.items)
}

func (r *deletionRepo) Update(_ context.Context, d domain.DeletionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.items {
		if r.items[i].ID == d.ID {
			r.items[i] = d
		}
	}
	return writeJSON(r.path, r.items)
}

func (r *deletionRepo) List(_ context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]domain.DeletionRequest(nil), r.items...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return page(out, limit, offset), int64(len(out)), nil
}

func (r *deletionRepo) FailRunning(_ context.Context, reason string, at time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for i := range r.items {
		if r.items[i].Status == "running" {
			r.items[i].Status, r.items[i].Error, r.items[i].CompletedAt = "failed", reason, &at
			n++
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, writeJSON(r.path, r.items)
}
