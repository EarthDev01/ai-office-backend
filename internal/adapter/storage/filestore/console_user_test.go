package filestore

import (
	"context"
	"path/filepath"
	"testing"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

func newRepo(t *testing.T) port.ConsoleUserRepository {
	r, err := NewConsoleUserRepository(filepath.Join(t.TempDir(), "u.json"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCreateAndByUsername(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u := domain.ConsoleUser{ID: "1", Username: "earth24", Role: domain.RoleAdmin, Status: domain.StatusActive}
	if err := r.Create(ctx, u); err != nil {
		t.Fatal(err)
	}

	got, err := r.ByUsername(ctx, "earth24")
	if err != nil || got.ID != "1" {
		t.Fatalf("got %+v %v", got, err)
	}

	// ซ้ำ (แม้ต่าง case ควรถูกกันที่ service ด้วย normalize; ที่ repo กันตาม key ที่เก็บ)
	if err := r.Create(ctx, u); err != port.ErrUsernameTaken {
		t.Fatalf("dup should be ErrUsernameTaken, got %v", err)
	}
}

func TestPersistsSensitiveFieldsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "u.json")
	ctx := context.Background()

	r1, err := NewConsoleUserRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	u := domain.ConsoleUser{
		ID:             "1",
		Username:       "earth24",
		Role:           domain.RoleAdmin,
		Status:         domain.StatusActive,
		PasswordHash:   "bcrypt-hash-of-password",
		TOTPSecret:     "JBSWY3DPEHPK3PXP",
		TOTPEnrolled:   true,
		RecoveryHashes: []string{"rh1", "rh2"},
	}
	if err := r1.Create(ctx, u); err != nil {
		t.Fatal(err)
	}

	// เปิด repo ใหม่ชี้ไฟล์เดิม จำลอง restart process
	r2, err := NewConsoleUserRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r2.ByID(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordHash != u.PasswordHash {
		t.Errorf("PasswordHash lost across reload: got %q want %q", got.PasswordHash, u.PasswordHash)
	}
	if got.TOTPSecret != u.TOTPSecret {
		t.Errorf("TOTPSecret lost across reload: got %q want %q", got.TOTPSecret, u.TOTPSecret)
	}
	if !got.TOTPEnrolled {
		t.Errorf("TOTPEnrolled lost across reload")
	}
	if len(got.RecoveryHashes) != 2 {
		t.Errorf("RecoveryHashes lost across reload: got %v", got.RecoveryHashes)
	}
}

func TestCountAndNotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if n, _ := r.Count(ctx); n != 0 {
		t.Fatalf("count=%d", n)
	}
	if _, err := r.ByUsername(ctx, "nobody"); err != port.ErrUserNotFound {
		t.Fatalf("want ErrUserNotFound got %v", err)
	}
}
