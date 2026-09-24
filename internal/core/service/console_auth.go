package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// Sentinel errors ของ console auth service (spec §4 + §9)
var (
	ErrSetupDone          = errors.New("SETUP_DONE")
	ErrInvalidCredentials = errors.New("INVALID_CREDENTIALS")
	ErrAccountLocked      = errors.New("ACCOUNT_LOCKED")
	ErrAccountDisabled    = errors.New("ACCOUNT_DISABLED")
	ErrInvalidPassword    = errors.New("INVALID_PASSWORD")
	ErrInvalid2FA         = errors.New("INVALID_2FA")
	ErrInvalidTicket      = errors.New("INVALID_TICKET")
	ErrLastAdmin          = errors.New("LAST_ADMIN")
	ErrCannotDeleteSelf   = errors.New("CANNOT_DELETE_SELF")
	ErrInvalidRole        = errors.New("INVALID_ROLE")
)

// LoginResult คือผลลัพธ์ของแต่ละขั้นใน login flow (2-step TOTP)
type LoginResult struct {
	Stage         string             // "enroll" | "totp" | "done"
	Ticket        string             // enroll/totp: short-lived ticket to pass to VerifyTOTP
	OTPAuthURI    string             // enroll only: for QR
	Secret        string             // enroll only: base32, for manual entry
	Token         string             // done only: session JWT
	User          domain.ConsoleUser // done only
	RecoveryCodes []string           // set once, right after enrollment completes
}

// ConsoleAuth ทำ password login + mandatory 2-step TOTP + จัดการ console user
type ConsoleAuth struct {
	repo    port.ConsoleUserRepository
	tokens  port.TokenIssuer
	totp    port.TOTPProvider
	tickets port.TicketIssuer
	perms   *PermissionService // role นี้ยังมีอยู่ไหม — เช็คตอน CreateUser/PatchUser
	now     func() time.Time
}

func NewConsoleAuth(
	repo port.ConsoleUserRepository,
	tokens port.TokenIssuer,
	totp port.TOTPProvider,
	tickets port.TicketIssuer,
	perms *PermissionService,
	now func() time.Time,
) *ConsoleAuth {
	return &ConsoleAuth{repo: repo, tokens: tokens, totp: totp, tickets: tickets, perms: perms, now: now}
}

// NeedsSetup: ยังไม่มี user เลย → ต้อง first-run register
func (a *ConsoleAuth) NeedsSetup(ctx context.Context) (bool, error) {
	n, err := a.repo.Count(ctx)
	return n == 0, err
}

// Register: first-run only — สร้าง admin คนแรก แล้วเริ่ม enroll 2FA ทันที
func (a *ConsoleAuth) Register(ctx context.Context, username, display, password string) (LoginResult, error) {
	n, err := a.repo.Count(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	if n > 0 {
		return LoginResult{}, ErrSetupDone
	}
	if !domain.ValidPassword(password) {
		return LoginResult{}, ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return LoginResult{}, err
	}
	now := a.now()
	user := domain.ConsoleUser{
		ID:           domain.NewConsoleUserID(),
		Username:     domain.NormalizeUsername(username),
		DisplayName:  display,
		PasswordHash: string(hash),
		Role:         domain.RoleAdmin,
		Status:       domain.StatusActive,
		CreatedBy:    "setup",
		CreatedAt:    now,
		UpdatedAt:    now,
		TOTPEnrolled: false,
	}
	if err := a.repo.Create(ctx, user); err != nil {
		return LoginResult{}, err
	}
	// 2FA บังคับ → เริ่ม enroll ทันที
	return a.beginEnroll(user)
}

// Login: ตรวจ password + lockout, แล้วเข้าสู่ขั้น 2FA (enroll หรือ totp)
func (a *ConsoleAuth) Login(ctx context.Context, username, password string) (LoginResult, error) {
	uname := domain.NormalizeUsername(username)
	user, err := a.repo.ByUsername(ctx, uname)
	if err != nil {
		if errors.Is(err, port.ErrUserNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	now := a.now()
	if user.Locked(now) {
		return LoginResult{}, ErrAccountLocked
	}
	if user.Status == domain.StatusDisabled {
		return LoginResult{}, ErrAccountDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		user.FailedAttempts++
		if user.FailedAttempts >= 5 {
			user.LockedUntil = now.Add(15 * time.Minute)
			user.FailedAttempts = 0
		}
		user.UpdatedAt = now
		if err := a.repo.Update(ctx, user); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	// password ถูก → reset counters (ยังไม่ตั้ง LastLoginAt จนกว่าจะผ่าน 2FA)
	user.FailedAttempts = 0
	user.LockedUntil = time.Time{}
	user.UpdatedAt = now
	if err := a.repo.Update(ctx, user); err != nil {
		return LoginResult{}, err
	}
	if !user.TOTPEnrolled {
		return a.beginEnroll(user)
	}
	tkt, err := a.tickets.IssueTicket(port.Ticket{UserID: user.ID, Stage: "totp"})
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Stage: "totp", Ticket: tkt}, nil
}

// beginEnroll: สร้าง secret ใหม่ + ticket "enroll" (secret ยังไม่ผูกกับ user จนกว่าจะยืนยัน)
func (a *ConsoleAuth) beginEnroll(user domain.ConsoleUser) (LoginResult, error) {
	secret, uri, err := a.totp.GenerateSecret(user.Username)
	if err != nil {
		return LoginResult{}, err
	}
	tkt, err := a.tickets.IssueTicket(port.Ticket{UserID: user.ID, Stage: "enroll", PendingSecret: secret})
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Stage: "enroll", Ticket: tkt, OTPAuthURI: uri, Secret: secret}, nil
}

// VerifyTOTP: ขั้นที่สองของ login — ยืนยัน enroll code หรือ totp/recovery code
func (a *ConsoleAuth) VerifyTOTP(ctx context.Context, ticket, code string) (LoginResult, error) {
	t, err := a.tickets.VerifyTicket(ticket)
	if err != nil {
		return LoginResult{}, ErrInvalidTicket
	}
	user, err := a.repo.ByID(ctx, t.UserID)
	if err != nil {
		return LoginResult{}, ErrInvalidTicket
	}

	switch t.Stage {
	case "enroll":
		if !a.totp.Validate(t.PendingSecret, code) {
			return LoginResult{}, ErrInvalid2FA
		}
		user.TOTPSecret = t.PendingSecret
		user.TOTPEnrolled = true
		plain, hashes, err := generateRecoveryCodes()
		if err != nil {
			return LoginResult{}, err
		}
		user.RecoveryHashes = hashes
		token, err := a.finalize(ctx, &user)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{Stage: "done", Token: token, User: user, RecoveryCodes: plain}, nil

	case "totp":
		ok := a.totp.Validate(user.TOTPSecret, code)
		if !ok {
			// ลอง recovery code (one-time use)
			for i, h := range user.RecoveryHashes {
				if bcrypt.CompareHashAndPassword([]byte(h), []byte(code)) == nil {
					user.RecoveryHashes = append(user.RecoveryHashes[:i], user.RecoveryHashes[i+1:]...)
					ok = true
					break
				}
			}
		}
		if !ok {
			return LoginResult{}, ErrInvalid2FA
		}
		token, err := a.finalize(ctx, &user)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{Stage: "done", Token: token, User: user}, nil

	default:
		return LoginResult{}, ErrInvalidTicket
	}
}

// finalize: 2FA ผ่านแล้ว — ตั้ง LastLoginAt, persist, ออก session token
func (a *ConsoleAuth) finalize(ctx context.Context, user *domain.ConsoleUser) (string, error) {
	now := a.now()
	user.LastLoginAt = now
	user.UpdatedAt = now
	if err := a.repo.Update(ctx, *user); err != nil {
		return "", err
	}
	return a.tokens.Issue(port.Claims{UserID: user.ID, Username: user.Username, Role: user.Role})
}

// CreateUser (admin): สร้าง console user ใหม่
func (a *ConsoleAuth) CreateUser(ctx context.Context, actorUsername, username, display string, role domain.Role, password string) (domain.ConsoleUser, error) {
	ok, err := a.perms.RoleExists(ctx, role)
	if err != nil {
		return domain.ConsoleUser{}, err
	}
	if !ok {
		return domain.ConsoleUser{}, ErrInvalidRole
	}
	if !domain.ValidPassword(password) {
		return domain.ConsoleUser{}, ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.ConsoleUser{}, err
	}
	now := a.now()
	user := domain.ConsoleUser{
		ID:           domain.NewConsoleUserID(),
		Username:     domain.NormalizeUsername(username),
		DisplayName:  display,
		PasswordHash: string(hash),
		Role:         role,
		Status:       domain.StatusActive,
		CreatedBy:    domain.NormalizeUsername(actorUsername),
		CreatedAt:    now,
		UpdatedAt:    now,
		TOTPEnrolled: false,
	}
	if err := a.repo.Create(ctx, user); err != nil {
		return domain.ConsoleUser{}, err
	}
	return user, nil
}

// ListUsers: คืน console user ทั้งหมด
func (a *ConsoleAuth) ListUsers(ctx context.Context) ([]domain.ConsoleUser, error) {
	return a.repo.List(ctx)
}

// Me: คืนข้อมูล console user ปัจจุบัน (GET /auth/me)
func (a *ConsoleAuth) Me(ctx context.Context, id string) (domain.ConsoleUser, error) {
	return a.repo.ByID(ctx, id)
}

// PatchUser (admin): แก้ display/role/status พร้อม last-admin guard
func (a *ConsoleAuth) PatchUser(ctx context.Context, id string, display *string, role *domain.Role, status *string) (domain.ConsoleUser, error) {
	user, err := a.repo.ByID(ctx, id)
	if err != nil {
		return domain.ConsoleUser{}, err
	}
	if role != nil {
		ok, err := a.perms.RoleExists(ctx, *role)
		if err != nil {
			return domain.ConsoleUser{}, err
		}
		if !ok {
			return domain.ConsoleUser{}, ErrInvalidRole
		}
	}

	// last-admin guard: ถ้าจะ demote จาก admin หรือ disable admin คนสุดท้าย
	demoting := role != nil && user.Role == domain.RoleAdmin && *role != domain.RoleAdmin
	disabling := status != nil && *status == domain.StatusDisabled
	if demoting || disabling {
		if user.Role == domain.RoleAdmin && user.Status == domain.StatusActive {
			n, err := a.countActiveAdmins(ctx)
			if err != nil {
				return domain.ConsoleUser{}, err
			}
			if n <= 1 {
				return domain.ConsoleUser{}, ErrLastAdmin
			}
		}
	}

	if display != nil {
		user.DisplayName = *display
	}
	if role != nil {
		user.Role = *role
	}
	if status != nil {
		user.Status = *status
	}
	user.UpdatedAt = a.now()
	if err := a.repo.Update(ctx, user); err != nil {
		return domain.ConsoleUser{}, err
	}
	return user, nil
}

// ResetPassword (admin): ตั้ง password ใหม่ + ปลดล็อก
func (a *ConsoleAuth) ResetPassword(ctx context.Context, id, newPassword string) error {
	if !domain.ValidPassword(newPassword) {
		return ErrInvalidPassword
	}
	user, err := a.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	user.FailedAttempts = 0
	user.LockedUntil = time.Time{}
	user.UpdatedAt = a.now()
	return a.repo.Update(ctx, user)
}

// ChangePassword (self): เปลี่ยน password โดยต้องรู้ password เดิม
func (a *ConsoleAuth) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := a.repo.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		return ErrInvalidCredentials
	}
	if !domain.ValidPassword(newPassword) {
		return ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	user.UpdatedAt = a.now()
	return a.repo.Update(ctx, user)
}

// Reset2FA (admin): ล้าง 2FA — user จะต้อง enroll ใหม่ตอน login ครั้งถัดไป
func (a *ConsoleAuth) Reset2FA(ctx context.Context, id string) error {
	user, err := a.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	user.TOTPSecret = ""
	user.TOTPEnrolled = false
	user.RecoveryHashes = nil
	user.UpdatedAt = a.now()
	return a.repo.Update(ctx, user)
}

// Delete (admin): ลบ user พร้อม self-delete + last-admin guard
func (a *ConsoleAuth) Delete(ctx context.Context, actorID, id string) error {
	if actorID == id {
		return ErrCannotDeleteSelf
	}
	target, err := a.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if target.Role == domain.RoleAdmin && target.Status == domain.StatusActive {
		n, err := a.countActiveAdmins(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	return a.repo.Delete(ctx, id)
}

// countActiveAdmins: นับ admin ที่ status active เท่านั้น
func (a *ConsoleAuth) countActiveAdmins(ctx context.Context) (int, error) {
	users, err := a.repo.List(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range users {
		if u.Role == domain.RoleAdmin && u.Status == domain.StatusActive {
			n++
		}
	}
	return n, nil
}

// generateRecoveryCodes: 8 codes รูปแบบ xxxxx-xxxxx (10 lowercase-hex) + bcrypt hash
func generateRecoveryCodes() (plain []string, hashes []string, err error) {
	const count = 8
	plain = make([]string, count)
	hashes = make([]string, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 5)
		if _, err = rand.Read(b); err != nil {
			return nil, nil, err
		}
		h := hex.EncodeToString(b) // 10 hex chars
		code := fmt.Sprintf("%s-%s", h[:5], h[5:])
		plain[i] = code
		bh, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		hashes[i] = string(bh)
	}
	return plain, hashes, nil
}
