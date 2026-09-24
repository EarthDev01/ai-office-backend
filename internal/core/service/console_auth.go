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
	audit   port.AuditRecorder
	now     func() time.Time
}

func NewConsoleAuth(
	repo port.ConsoleUserRepository,
	tokens port.TokenIssuer,
	totp port.TOTPProvider,
	tickets port.TicketIssuer,
	perms *PermissionService,
	audit port.AuditRecorder, // nil ได้ (ไม่บันทึกประวัติ)
	now func() time.Time,
) *ConsoleAuth {
	return &ConsoleAuth{repo: repo, tokens: tokens, totp: totp, tickets: tickets, perms: perms, audit: auditOr(audit), now: now}
}

// recordAs บันทึกเหตุการณ์ในนามของ user ที่ระบุตรง ๆ — ใช้ในขั้น login ที่ยังไม่มี session ใน context
func (a *ConsoleAuth) recordAs(ctx context.Context, u domain.ConsoleUser, e domain.AuditEntry) {
	e.ActorID, e.Actor, e.ActorRole = u.ID, u.Username, string(u.Role)
	a.audit.Record(ctx, e)
}

// loginFailed บันทึก login ไม่สำเร็จ — user อาจไม่มีอยู่จริง (ID ว่าง) จึงใช้ username ที่พิมพ์มา
func (a *ConsoleAuth) loginFailed(ctx context.Context, u domain.ConsoleUser, reason, why string) {
	a.recordAs(ctx, u, domain.AuditEntry{
		Action: domain.AuditAuthLoginFailed, Status: domain.AuditFailure, Reason: reason,
		TargetType: "user", TargetID: u.ID, TargetLabel: u.Username,
		Summary: fmt.Sprintf("เข้าสู่ระบบไม่สำเร็จ (%s) — username %q", why, u.Username),
	})
}

// userLabel ใช้ใน summary ของ user management: "ชื่อ (username)"
func userLabel(u domain.ConsoleUser) string {
	if u.DisplayName != "" && u.DisplayName != u.Username {
		return fmt.Sprintf("%s (%s)", u.DisplayName, u.Username)
	}
	return u.Username
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
	a.recordAs(ctx, user, domain.AuditEntry{
		Action: domain.AuditAuthSetup, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		Summary: fmt.Sprintf("ตั้งค่าระบบครั้งแรก — สร้างผู้ดูแลคนแรก %s", userLabel(user)),
	})
	// 2FA บังคับ → เริ่ม enroll ทันที
	return a.beginEnroll(user)
}

// Login: ตรวจ password + lockout, แล้วเข้าสู่ขั้น 2FA (enroll หรือ totp)
func (a *ConsoleAuth) Login(ctx context.Context, username, password string) (LoginResult, error) {
	uname := domain.NormalizeUsername(username)
	user, err := a.repo.ByUsername(ctx, uname)
	if err != nil {
		if errors.Is(err, port.ErrUserNotFound) {
			a.loginFailed(ctx, domain.ConsoleUser{Username: uname}, "UNKNOWN_USER", "ไม่พบ username นี้")
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	now := a.now()
	if user.Locked(now) {
		a.loginFailed(ctx, user, "ACCOUNT_LOCKED", "บัญชีถูกล็อกชั่วคราว")
		return LoginResult{}, ErrAccountLocked
	}
	if user.Status == domain.StatusDisabled {
		a.loginFailed(ctx, user, "ACCOUNT_DISABLED", "บัญชีถูกปิดใช้งาน")
		return LoginResult{}, ErrAccountDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		user.FailedAttempts++
		attempt := user.FailedAttempts
		locked := false
		if user.FailedAttempts >= 5 {
			user.LockedUntil = now.Add(15 * time.Minute)
			user.FailedAttempts = 0
			locked = true
		}
		user.UpdatedAt = now
		if err := a.repo.Update(ctx, user); err != nil {
			return LoginResult{}, err
		}
		a.loginFailed(ctx, user, "INVALID_CREDENTIALS", fmt.Sprintf("รหัสผ่านผิด ครั้งที่ %d/5", attempt))
		if locked {
			a.recordAs(ctx, user, domain.AuditEntry{
				Action: domain.AuditAuthLocked, Status: domain.AuditFailure, Reason: "TOO_MANY_ATTEMPTS",
				TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
				Summary: fmt.Sprintf("บัญชี %s ถูกล็อก 15 นาที เพราะใส่รหัสผ่านผิด 5 ครั้งติด", userLabel(user)),
				Meta:    map[string]string{"locked_until": user.LockedUntil.Format(time.RFC3339)},
			})
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
			a.loginFailed(ctx, user, "INVALID_2FA", "รหัสยืนยัน 2FA ตอนตั้งค่าไม่ถูกต้อง")
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
		a.recordAs(ctx, user, domain.AuditEntry{
			Action: domain.AuditAuth2FAEnrolled, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
			Summary: fmt.Sprintf("%s ตั้งค่า 2FA สำเร็จ และได้รับ recovery code ชุดใหม่", userLabel(user)),
		})
		a.recordLogin(ctx, user, "2FA (ตั้งค่าครั้งแรก)")
		return LoginResult{Stage: "done", Token: token, User: user, RecoveryCodes: plain}, nil

	case "totp":
		ok := a.totp.Validate(user.TOTPSecret, code)
		usedRecovery := false
		if !ok {
			// ลอง recovery code (one-time use)
			for i, h := range user.RecoveryHashes {
				if bcrypt.CompareHashAndPassword([]byte(h), []byte(code)) == nil {
					user.RecoveryHashes = append(user.RecoveryHashes[:i], user.RecoveryHashes[i+1:]...)
					ok = true
					usedRecovery = true
					break
				}
			}
		}
		if !ok {
			a.loginFailed(ctx, user, "INVALID_2FA", "รหัส 2FA ไม่ถูกต้อง")
			return LoginResult{}, ErrInvalid2FA
		}
		token, err := a.finalize(ctx, &user)
		if err != nil {
			return LoginResult{}, err
		}
		method := "รหัส 2FA"
		if usedRecovery {
			method = "recovery code"
			a.recordAs(ctx, user, domain.AuditEntry{
				Action: domain.AuditAuthRecoveryUsed, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
				Summary: fmt.Sprintf("%s ใช้ recovery code เข้าสู่ระบบ — เหลืออีก %d รหัส", userLabel(user), len(user.RecoveryHashes)),
				Meta:    map[string]string{"remaining": fmt.Sprint(len(user.RecoveryHashes))},
			})
		}
		a.recordLogin(ctx, user, method)
		return LoginResult{Stage: "done", Token: token, User: user}, nil

	default:
		return LoginResult{}, ErrInvalidTicket
	}
}

func (a *ConsoleAuth) recordLogin(ctx context.Context, user domain.ConsoleUser, method string) {
	a.recordAs(ctx, user, domain.AuditEntry{
		Action: domain.AuditAuthLogin, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		Summary: fmt.Sprintf("%s เข้าสู่ระบบ (ยืนยันด้วย %s)", userLabel(user), method),
		Meta:    map[string]string{"method": method},
	})
}

// Logout บันทึกการออกจากระบบ — session เป็น JWT ไร้สถานะ ฝั่ง server จึงมีแค่การบันทึก
// (token เดิมยังใช้ได้จนหมดอายุ ถ้าต้องการ revoke จริงต้องมี denylist เพิ่ม)
func (a *ConsoleAuth) Logout(ctx context.Context) {
	actor, _ := domain.AuditActorFrom(ctx)
	a.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditAuthLogout, TargetType: "user", TargetID: actor.ID, TargetLabel: actor.Username,
		Summary: fmt.Sprintf("%s ออกจากระบบ", actor.Username),
	})
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
	a.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditUserCreate, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		Summary: fmt.Sprintf("เพิ่มผู้ใช้ %s บทบาท %s", userLabel(user), user.Role),
		Changes: []domain.FieldChange{
			{Field: "username", After: user.Username},
			{Field: "display_name", After: user.DisplayName},
			{Field: "role", After: string(user.Role)},
		},
	})
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

	before := user
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
	changes := []domain.FieldChange{}
	if before.DisplayName != user.DisplayName {
		changes = append(changes, domain.FieldChange{Field: "display_name", Before: before.DisplayName, After: user.DisplayName})
	}
	if before.Role != user.Role {
		changes = append(changes, domain.FieldChange{Field: "role", Before: string(before.Role), After: string(user.Role)})
	}
	if before.Status != user.Status {
		changes = append(changes, domain.FieldChange{Field: "status", Before: before.Status, After: user.Status})
	}
	if len(changes) > 0 {
		a.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditUserUpdate, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
			Summary: fmt.Sprintf("แก้ไขผู้ใช้ %s %d รายการ: %s", userLabel(user), len(changes), changedFieldNames(changes)),
			Changes: changes,
		})
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
	wasLocked := user.Locked(a.now())
	user.PasswordHash = string(hash)
	user.FailedAttempts = 0
	user.LockedUntil = time.Time{}
	user.UpdatedAt = a.now()
	if err := a.repo.Update(ctx, user); err != nil {
		return err
	}
	summary := fmt.Sprintf("ตั้งรหัสผ่านใหม่ให้ %s", userLabel(user))
	if wasLocked {
		summary += " และปลดล็อกบัญชี"
	}
	a.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditUserResetPassword, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		Summary: summary,
	})
	return nil
}

// ChangePassword (self): เปลี่ยน password โดยต้องรู้ password เดิม
func (a *ConsoleAuth) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := a.repo.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		a.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditAuthPasswordChanged, Status: domain.AuditFailure, Reason: "INVALID_CREDENTIALS",
			TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
			Summary: fmt.Sprintf("%s เปลี่ยนรหัสผ่านไม่สำเร็จ — รหัสผ่านเดิมไม่ถูกต้อง", userLabel(user)),
		})
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
	if err := a.repo.Update(ctx, user); err != nil {
		return err
	}
	a.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditAuthPasswordChanged, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		Summary: fmt.Sprintf("%s เปลี่ยนรหัสผ่านของตัวเอง", userLabel(user)),
	})
	return nil
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
	if err := a.repo.Update(ctx, user); err != nil {
		return err
	}
	a.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditUserReset2FA, TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		Summary: fmt.Sprintf("รีเซ็ต 2FA ของ %s — ต้องตั้งค่าใหม่ตอนเข้าสู่ระบบครั้งถัดไป", userLabel(user)),
	})
	return nil
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
	if err := a.repo.Delete(ctx, id); err != nil {
		return err
	}
	a.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditUserDelete, TargetType: "user", TargetID: target.ID, TargetLabel: target.Username,
		Summary: fmt.Sprintf("ลบผู้ใช้ %s (บทบาท %s)", userLabel(target), target.Role),
		Meta:    map[string]string{"display_name": target.DisplayName, "role": string(target.Role)},
	})
	return nil
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
