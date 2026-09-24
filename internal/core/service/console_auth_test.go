package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/domain"
)

func newAuth(t *testing.T) *ConsoleAuth {
	t.Helper()
	repo, err := filestore.NewConsoleUserRepository(filepath.Join(t.TempDir(), "u.json"))
	if err != nil {
		t.Fatal(err)
	}
	sec := "test-secret"
	rmRepo, err := filestore.NewRoleMatrixRepository(filepath.Join(t.TempDir(), "role_permissions.json"))
	if err != nil {
		t.Fatal(err)
	}
	permSvc := NewPermissionService(rmRepo)
	return NewConsoleAuth(
		repo,
		auth.NewJWTIssuer(sec, time.Hour),
		auth.NewTOTPProvider("AI Office Console Test"),
		auth.NewTicketIssuer(sec, 5*time.Minute),
		permSvc,
		time.Now,
	)
}

// enroll ครบ flow: register → verify enroll code → done + token + recovery codes
func enroll(t *testing.T, a *ConsoleAuth, user, password string) LoginResult {
	t.Helper()
	ctx := context.Background()
	reg, err := a.Register(ctx, user, "Display", password)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.Stage != "enroll" || reg.Secret == "" || reg.Ticket == "" {
		t.Fatalf("register should begin enroll: %+v", reg)
	}
	code, _ := totp.GenerateCode(reg.Secret, time.Now())
	done, err := a.VerifyTOTP(ctx, reg.Ticket, code)
	if err != nil || done.Stage != "done" || done.Token == "" {
		t.Fatalf("verify enroll: %+v %v", done, err)
	}
	if len(done.RecoveryCodes) != 8 {
		t.Fatalf("want 8 recovery codes, got %d", len(done.RecoveryCodes))
	}
	return done
}

func TestRegisterEnrollThenLoginWithTOTP(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	enroll(t, a, "Earth24", "secret12")

	// second register blocked
	if _, err := a.Register(ctx, "other", "x", "secret22"); err != ErrSetupDone {
		t.Fatalf("second register: want ErrSetupDone got %v", err)
	}

	// login now returns totp stage
	lr, err := a.Login(ctx, "earth24", "secret12")
	if err != nil || lr.Stage != "totp" || lr.Ticket == "" {
		t.Fatalf("login stage: %+v %v", lr, err)
	}
	// need the enrolled secret to make a code — re-derive via a fresh enroll is not possible;
	// instead verify wrong code fails and a recovery code works:
	if _, err := a.VerifyTOTP(ctx, lr.Ticket, "000000"); err != ErrInvalid2FA {
		t.Fatalf("bad totp: want ErrInvalid2FA got %v", err)
	}
}

func TestRecoveryCodeLoginOneTimeUse(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	done := enroll(t, a, "earth", "secret12")
	rc := done.RecoveryCodes[0]

	lr, _ := a.Login(ctx, "earth", "secret12")
	d2, err := a.VerifyTOTP(ctx, lr.Ticket, rc)
	if err != nil || d2.Stage != "done" {
		t.Fatalf("recovery login: %+v %v", d2, err)
	}
	// same recovery code cannot be reused
	lr2, _ := a.Login(ctx, "earth", "secret12")
	if _, err := a.VerifyTOTP(ctx, lr2.Ticket, rc); err != ErrInvalid2FA {
		t.Fatalf("reused recovery code: want ErrInvalid2FA got %v", err)
	}
}

func TestRegisterBadPassword(t *testing.T) {
	a := newAuth(t)
	if _, err := a.Register(context.Background(), "earth", "x", "short"); err != ErrInvalidPassword {
		t.Fatalf("want ErrInvalidPassword got %v", err)
	}
}

func TestLoginLockoutAfter5(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	enroll(t, a, "earth", "secret12")
	for i := 0; i < 5; i++ {
		if _, err := a.Login(ctx, "earth", "wrongpass"); err != ErrInvalidCredentials {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := a.Login(ctx, "earth", "secret12"); err != ErrAccountLocked {
		t.Fatalf("want ErrAccountLocked got %v", err)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	a := newAuth(t)
	enroll(t, a, "earth", "secret12")
	if _, err := a.Login(context.Background(), "ghost", "secret12"); err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials got %v", err)
	}
}

func TestLastAdminGuardAndSelfDelete(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	done := enroll(t, a, "earth", "secret12")
	adminID := done.User.ID
	op := domain.RoleOperator
	if _, err := a.PatchUser(ctx, adminID, nil, &op, nil); err != ErrLastAdmin {
		t.Fatalf("demote last admin: want ErrLastAdmin got %v", err)
	}
	if err := a.Delete(ctx, "someone", adminID); err != ErrLastAdmin {
		t.Fatalf("delete last admin: want ErrLastAdmin got %v", err)
	}
	if err := a.Delete(ctx, adminID, adminID); err != ErrCannotDeleteSelf {
		t.Fatalf("self delete: want ErrCannotDeleteSelf got %v", err)
	}
}

func TestChangePasswordAndReset2FA(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	done := enroll(t, a, "earth", "secret12")
	id := done.User.ID
	if err := a.ChangePassword(ctx, id, "wrongpass", "newsecret1"); err != ErrInvalidCredentials {
		t.Fatalf("wrong old password: %v", err)
	}
	if err := a.ChangePassword(ctx, id, "secret12", "newsecret1"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	// reset 2FA → next login returns enroll again
	if err := a.Reset2FA(ctx, id); err != nil {
		t.Fatalf("reset 2fa: %v", err)
	}
	lr, err := a.Login(ctx, "earth", "newsecret1")
	if err != nil || lr.Stage != "enroll" {
		t.Fatalf("after reset 2fa login should re-enroll: %+v %v", lr, err)
	}
}

func TestCreateUserByAdmin(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	enroll(t, a, "earth", "secret12")
	u, err := a.CreateUser(ctx, "earth", "Operator One", "Op", domain.RoleOperator, "opsecret1")
	if err != nil || u.Role != domain.RoleOperator || u.Username != "operator one" {
		t.Fatalf("create user: %+v %v", u, err)
	}
	// dup username (different case) rejected
	if _, err := a.CreateUser(ctx, "earth", "OPERATOR ONE", "X", domain.RoleViewer, "viewsecret"); err == nil {
		t.Fatal("duplicate username (case-variant) must be rejected")
	}
}

func TestMe(t *testing.T) {
	a := newAuth(t)
	ctx := context.Background()
	done := enroll(t, a, "earth", "secret12")

	u, err := a.Me(ctx, done.User.ID)
	if err != nil || u.Username != "earth" {
		t.Fatalf("me: %+v %v", u, err)
	}

	if _, err := a.Me(ctx, "no-such-id"); err == nil {
		t.Fatal("me with unknown id: want error")
	}
}
