// Package filestore เก็บ console user ลงไฟล์ JSON
//
// เป็น adapter ของ port.ConsoleUserRepository — สลับไปใช้ MongoDB ได้โดยแก้
// _cmd/main.go บรรทัดเดียว ไม่ต้องแตะ service หรือ handler (02-SPEC §7)
package filestore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type consoleUserRepo struct {
	mu    sync.RWMutex
	path  string
	items map[string]domain.ConsoleUser
}

// consoleUserDTO คือ struct แยกสำหรับ persist ลงไฟล์เท่านั้น
//
// domain.ConsoleUser ใช้ `json:"-"` กับ field อ่อนไหว (password_hash, failed_attempts,
// locked_until, totp_secret, recovery_hashes) เพื่อกันไม่ให้หลุดออกไปใน API response
// แต่ filestore เองต้อง persist field พวกนี้ด้วยไม่งั้น login/lockout/2FA จะหายหลัง restart
// เลยมี encoding ของตัวเองแยกจาก transport (JSON API) ไม่ต้องแตะ domain tags
type consoleUserDTO struct {
	ID             string      `json:"id"`
	Username       string      `json:"username"`
	DisplayName    string      `json:"display_name"`
	PasswordHash   string      `json:"password_hash"`
	Role           domain.Role `json:"role"`
	Status         string      `json:"status"`
	FailedAttempts int         `json:"failed_attempts"`
	LockedUntil    time.Time   `json:"locked_until"`
	LastLoginAt    time.Time   `json:"last_login_at"`
	CreatedAt      time.Time   `json:"created_at"`
	CreatedBy      string      `json:"created_by"`
	UpdatedAt      time.Time   `json:"updated_at"`
	TOTPSecret     string      `json:"totp_secret"`
	TOTPEnrolled   bool        `json:"totp_enrolled"`
	RecoveryHashes []string    `json:"recovery_hashes"`
}

func toDTO(u domain.ConsoleUser) consoleUserDTO {
	return consoleUserDTO{
		ID:             u.ID,
		Username:       u.Username,
		DisplayName:    u.DisplayName,
		PasswordHash:   u.PasswordHash,
		Role:           u.Role,
		Status:         u.Status,
		FailedAttempts: u.FailedAttempts,
		LockedUntil:    u.LockedUntil,
		LastLoginAt:    u.LastLoginAt,
		CreatedAt:      u.CreatedAt,
		CreatedBy:      u.CreatedBy,
		UpdatedAt:      u.UpdatedAt,
		TOTPSecret:     u.TOTPSecret,
		TOTPEnrolled:   u.TOTPEnrolled,
		RecoveryHashes: u.RecoveryHashes,
	}
}

func fromDTO(d consoleUserDTO) domain.ConsoleUser {
	return domain.ConsoleUser{
		ID:             d.ID,
		Username:       d.Username,
		DisplayName:    d.DisplayName,
		PasswordHash:   d.PasswordHash,
		Role:           d.Role,
		Status:         d.Status,
		FailedAttempts: d.FailedAttempts,
		LockedUntil:    d.LockedUntil,
		LastLoginAt:    d.LastLoginAt,
		CreatedAt:      d.CreatedAt,
		CreatedBy:      d.CreatedBy,
		UpdatedAt:      d.UpdatedAt,
		TOTPSecret:     d.TOTPSecret,
		TOTPEnrolled:   d.TOTPEnrolled,
		RecoveryHashes: d.RecoveryHashes,
	}
}

func NewConsoleUserRepository(path string) (port.ConsoleUserRepository, error) {
	r := &consoleUserRepo{path: path, items: map[string]domain.ConsoleUser{}}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *consoleUserRepo) load() error {
	b, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []consoleUserDTO
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, d := range list {
		u := fromDTO(d)
		r.items[u.ID] = u
	}
	return nil
}

func (r *consoleUserRepo) flush() error {
	list := make([]consoleUserDTO, 0, len(r.items))
	for _, u := range r.items {
		list = append(list, toDTO(u))
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

// findByUsername ต้องถือ lock (read หรือ write) อยู่แล้วก่อนเรียก
func (r *consoleUserRepo) findByUsername(username string) (domain.ConsoleUser, bool) {
	norm := domain.NormalizeUsername(username)
	for _, u := range r.items {
		if domain.NormalizeUsername(u.Username) == norm {
			return u, true
		}
	}
	return domain.ConsoleUser{}, false
}

func (r *consoleUserRepo) Create(ctx context.Context, u domain.ConsoleUser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.findByUsername(u.Username); ok {
		return port.ErrUsernameTaken
	}
	r.items[u.ID] = u
	return r.flush()
}

func (r *consoleUserRepo) ByUsername(ctx context.Context, username string) (domain.ConsoleUser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.findByUsername(username)
	if !ok {
		return domain.ConsoleUser{}, port.ErrUserNotFound
	}
	return u, nil
}

func (r *consoleUserRepo) ByID(ctx context.Context, id string) (domain.ConsoleUser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.items[id]
	if !ok {
		return domain.ConsoleUser{}, port.ErrUserNotFound
	}
	return u, nil
}

func (r *consoleUserRepo) List(ctx context.Context) ([]domain.ConsoleUser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.ConsoleUser, 0, len(r.items))
	for _, u := range r.items {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

func (r *consoleUserRepo) Update(ctx context.Context, u domain.ConsoleUser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[u.ID]; !ok {
		return port.ErrUserNotFound
	}
	if existing, ok := r.findByUsername(u.Username); ok && existing.ID != u.ID {
		return port.ErrUsernameTaken
	}
	r.items[u.ID] = u
	return r.flush()
}

func (r *consoleUserRepo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return port.ErrUserNotFound
	}
	delete(r.items, id)
	return r.flush()
}

func (r *consoleUserRepo) Count(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items), nil
}
