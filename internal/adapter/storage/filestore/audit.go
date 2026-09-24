// Package filestore เก็บประวัติการทำงานลงไฟล์ JSON Lines (1 บรรทัด = 1 เหตุการณ์)
//
// ต่างจาก store อื่นในแพ็กเกจนี้ที่เขียนทับทั้งไฟล์ — ประวัติเป็น append-only
// จึงต่อท้ายไฟล์ทีละบรรทัด ไม่ต้องเขียนของเก่าซ้ำทุกครั้ง
package filestore

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type auditRepo struct {
	mu    sync.RWMutex
	path  string
	items []domain.AuditEntry // เรียงตามลำดับที่บันทึก (เก่า→ใหม่)
}

func NewAuditRepository(path string) (port.AuditRepository, error) {
	r := &auditRepo{path: path}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *auditRepo) load() error {
	f, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e domain.AuditEntry
		// บรรทัดที่พัง (เช่นเครื่องดับตอนเขียน) ข้ามไป ไม่ทำให้ทั้งระบบเปิดไม่ขึ้น
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		r.items = append(r.items, e)
	}
	return sc.Err()
}

func (r *auditRepo) Append(ctx context.Context, e domain.AuditEntry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	r.items = append(r.items, e)
	return nil
}

func (r *auditRepo) Query(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	f.Normalize()

	r.mu.RLock()
	matched := make([]domain.AuditEntry, 0)
	for _, e := range r.items {
		if f.Match(e) {
			matched = append(matched, e)
		}
	}
	r.mu.RUnlock()

	// ใหม่→เก่า · เวลาเท่ากันให้ตัวที่บันทึกทีหลังขึ้นก่อน (SliceStable บนลำดับเดิมที่กลับด้าน)
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].At.After(matched[j].At) })

	total := len(matched)
	start := (f.Page - 1) * f.PageSize
	if start >= total {
		return []domain.AuditEntry{}, total, nil
	}
	end := min(start+f.PageSize, total)
	return matched[start:end], total, nil
}

func (r *auditRepo) Actors(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]bool{}
	out := []string{}
	for _, e := range r.items {
		if e.Actor != "" && !seen[e.Actor] {
			seen[e.Actor] = true
			out = append(out, e.Actor)
		}
	}
	sort.Strings(out)
	return out, nil
}
