	# Console Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เพิ่มระบบ login / register / authorization ให้ admin console โดยใช้ username + PIN 6 หลัก, JWT, 3 roles พร้อม UI

**Architecture:** คงรูป hexagonal ของ `ai-office-backend` — เพิ่ม `ConsoleUser` domain + port + repo (mongo/file) + service (bcrypt, lockout, JWT) + auth/user routes + middleware ที่แทน static-token ด้วย JWT (คง static token ไว้เป็น break-glass). ฝั่ง `ai-office-report` เพิ่ม auth store + router guard + Login/Setup/Users views.

**Tech Stack:** Go 1.2x, Gin, MongoDB driver, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt` · Vue 3 + Vite + TS + Pinia + ant-design-vue + Tailwind v4

**Spec:** `docs/superpowers/specs/2026-09-23-console-auth-design.md`

## Global Constraints

- PIN ต้องตรง `^\d{6}$` เท่านั้น — validate ที่ service ก่อน hash เสมอ
- ห้ามเก็บ/log PIN plaintext — เก็บ bcrypt hash เท่านั้น; response ห้ามมี `pin_hash`
- Roles: `admin` > `operator` > `viewer` (ลำดับสิทธิ์)
- JWT: HS256, claims `{sub, username, role, iat, exp}`, อายุ 8 ชม., เซ็นด้วย `CONSOLE_JWT_SECRET`
- Lockout: `failed_attempts >= 5` → `locked_until = now + 15m`, reset attempts; สำเร็จ → reset
- Login error รวมเป็น `INVALID_CREDENTIALS` (ไม่แยก user-not-found กับ pin-ผิด) กัน enumeration
- `ADMIN_CONSOLE_TOKEN` ยังใช้ได้เป็น break-glass = role `admin`, username `break-glass`
- Response envelope เดิม: `ResData(c, code, message, error, payload)` (routes/response.go)
- Username เทียบแบบ lowercase-trim; unique
- STORE_DRIVER มีทั้ง `mongo` และ `file` — ทุก repo ต้องมีทั้งสอง impl เหมือน office

## Review Focus

- **PIN รูปแบบผิด** (ตัวอักษร/5 หลัก/7 หลัก/ว่าง) ตอน register/create/reset/change → ต้อง 400 ไม่ใช่ 500 หรือ hash ขยะ → pin ในไฟล์ → Task 1 test
- **username ซ้ำ** (ต่าง case/มีช่องว่าง) ตอนสร้าง user → ต้อง 409 ไม่ใช่สร้างซ้อน → Task 2 (repo unique) + Task 5 test
- **JWT หมดอายุ / เซ็นผิด secret / role ปลอมใน token** → ต้อง 401 ไม่ผ่าน → Task 3 + Task 6 test
- **admin ลบตัวเอง / ลบ/ปลด admin คนสุดท้าย** → ต้องกัน (409) ไม่งั้นล็อกตัวเองออกจากระบบถาวร → Task 5 test
- **register รอบสองหลังมี user แล้ว** (หรือยิงพร้อมกัน) → ต้อง 409 SETUP_DONE ไม่สร้าง admin เกินตั้งใจ → Task 5 test

---

## Task 1: ConsoleUser domain + validation

**Files:**
- Create: `internal/core/domain/console_user.go`
- Test: `internal/core/domain/console_user_test.go`

**Interfaces:**
- Produces:
  - `type Role string` with consts `RoleAdmin="admin"`, `RoleOperator="operator"`, `RoleViewer="viewer"`
  - `func (r Role) Valid() bool`, `func (r Role) AtLeast(min Role) bool`
  - `type ConsoleUser struct { ID, Username, DisplayName, PINHash string; Role Role; Status string; FailedAttempts int; LockedUntil, LastLoginAt, CreatedAt, UpdatedAt time.Time; CreatedBy string }`
  - `const StatusActive="active"`, `StatusDisabled="disabled"`
  - `func NewConsoleUserID() string` (16-byte hex via crypto/rand, ตามแบบ `NewPublicKey`)
  - `func NormalizeUsername(s string) string` (lowercase+trim)
  - `func ValidPIN(pin string) bool` (`^\d{6}$`)
  - `func (u ConsoleUser) Locked(now time.Time) bool`

- [ ] **Step 1: Write failing tests**

```go
package domain

import (
	"testing"
	"time"
)

func TestValidPIN(t *testing.T) {
	ok := []string{"000000", "123456", "999999"}
	bad := []string{"", "12345", "1234567", "12a456", "12 456", "abcdef"}
	for _, p := range ok {
		if !ValidPIN(p) {
			t.Errorf("ValidPIN(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if ValidPIN(p) {
			t.Errorf("ValidPIN(%q) = true, want false", p)
		}
	}
}

func TestRoleAtLeast(t *testing.T) {
	if !RoleAdmin.AtLeast(RoleOperator) {
		t.Error("admin should be >= operator")
	}
	if RoleViewer.AtLeast(RoleOperator) {
		t.Error("viewer should be < operator")
	}
	if !RoleOperator.AtLeast(RoleOperator) {
		t.Error("operator should be >= operator")
	}
	if Role("nonsense").Valid() {
		t.Error("nonsense role must be invalid")
	}
}

func TestNormalizeUsername(t *testing.T) {
	if NormalizeUsername("  Earth24 ") != "earth24" {
		t.Errorf("got %q", NormalizeUsername("  Earth24 "))
	}
}

func TestLocked(t *testing.T) {
	now := time.Now()
	u := ConsoleUser{LockedUntil: now.Add(time.Minute)}
	if !u.Locked(now) {
		t.Error("should be locked")
	}
	if (ConsoleUser{}).Locked(now) {
		t.Error("zero LockedUntil = not locked")
	}
}

func TestNewConsoleUserID_UniqueHex(t *testing.T) {
	a, b := NewConsoleUserID(), NewConsoleUserID()
	if a == b || len(a) != 32 {
		t.Errorf("ids not unique/len: %q %q", a, b)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL** (`go test ./internal/core/domain/ -run 'PIN|Role|Username|Locked|ConsoleUserID'`) — undefined symbols

- [ ] **Step 3: Implement `console_user.go`**

```go
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

var roleRank = map[Role]int{RoleViewer: 1, RoleOperator: 2, RoleAdmin: 3}

func (r Role) Valid() bool { _, ok := roleRank[r]; return ok }
func (r Role) AtLeast(min Role) bool { return roleRank[r] >= roleRank[min] && roleRank[r] > 0 }

type ConsoleUser struct {
	ID             string    `json:"id" bson:"_id"`
	Username       string    `json:"username" bson:"username"`
	DisplayName    string    `json:"display_name" bson:"display_name"`
	PINHash        string    `json:"-" bson:"pin_hash"`
	Role           Role      `json:"role" bson:"role"`
	Status         string    `json:"status" bson:"status"`
	FailedAttempts int       `json:"-" bson:"failed_attempts"`
	LockedUntil    time.Time `json:"-" bson:"locked_until"`
	LastLoginAt    time.Time `json:"last_login_at" bson:"last_login_at"`
	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
	CreatedBy      string    `json:"created_by" bson:"created_by"`
	UpdatedAt      time.Time `json:"updated_at" bson:"updated_at"`
}

func (u ConsoleUser) Locked(now time.Time) bool {
	return !u.LockedUntil.IsZero() && u.LockedUntil.After(now)
}

var pinRe = regexp.MustCompile(`^\d{6}$`)

func ValidPIN(pin string) bool { return pinRe.MatchString(pin) }

func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func NewConsoleUserID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

Note: `PINHash json:"-"` ensures it never leaks in responses (Global Constraint).

- [ ] **Step 4: Run tests — expect PASS**

- [ ] **Step 5: Commit** — `git add internal/core/domain/console_user*.go && git commit -m "feat(console): ConsoleUser domain + PIN/role validation"`

---

## Task 2: ConsoleUserRepository port + file & mongo repos

**Files:**
- Create: `internal/core/port/console_user.go`
- Create: `internal/adapter/storage/filestore/console_user.go`
- Create: `internal/adapter/storage/mongodb/repository/console_user.go`
- Test: `internal/adapter/storage/filestore/console_user_test.go`

**Interfaces:**
- Consumes: `domain.ConsoleUser`, `domain.NormalizeUsername`
- Produces `port.ConsoleUserRepository`:
  ```go
  type ConsoleUserRepository interface {
      Create(ctx context.Context, u domain.ConsoleUser) error // ErrUsernameTaken ถ้าซ้ำ
      ByUsername(ctx context.Context, username string) (domain.ConsoleUser, error) // ErrUserNotFound
      ByID(ctx context.Context, id string) (domain.ConsoleUser, error)
      List(ctx context.Context) ([]domain.ConsoleUser, error)
      Update(ctx context.Context, u domain.ConsoleUser) error
      Delete(ctx context.Context, id string) error
      Count(ctx context.Context) (int, error)
  }
  ```
- Produces sentinel errors ใน port: `ErrUsernameTaken`, `ErrUserNotFound`

**Follow the existing pattern:** `internal/adapter/storage/filestore/office.go` (mutex + JSON file) และ `internal/adapter/storage/mongodb/repository/office.go` (collection const + unique index).

- [ ] **Step 1: Write failing test (filestore)**

```go
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
	if err != nil { t.Fatal(err) }
	return r
}

func TestCreateAndByUsername(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u := domain.ConsoleUser{ID: "1", Username: "earth24", Role: domain.RoleAdmin, Status: domain.StatusActive}
	if err := r.Create(ctx, u); err != nil { t.Fatal(err) }

	got, err := r.ByUsername(ctx, "earth24")
	if err != nil || got.ID != "1" { t.Fatalf("got %+v %v", got, err) }

	// ซ้ำ (แม้ต่าง case ควรถูกกันที่ service ด้วย normalize; ที่ repo กันตาม key ที่เก็บ)
	if err := r.Create(ctx, u); err != port.ErrUsernameTaken {
		t.Fatalf("dup should be ErrUsernameTaken, got %v", err)
	}
}

func TestCountAndNotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if n, _ := r.Count(ctx); n != 0 { t.Fatalf("count=%d", n) }
	if _, err := r.ByUsername(ctx, "nobody"); err != port.ErrUserNotFound {
		t.Fatalf("want ErrUserNotFound got %v", err)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement port + filestore + mongo**

`port/console_user.go`:
```go
package port

import (
	"context"
	"errors"
	"ai-office-backend/internal/core/domain"
)

var (
	ErrUsernameTaken = errors.New("USERNAME_TAKEN")
	ErrUserNotFound  = errors.New("USER_NOT_FOUND")
)

type ConsoleUserRepository interface {
	Create(ctx context.Context, u domain.ConsoleUser) error
	ByUsername(ctx context.Context, username string) (domain.ConsoleUser, error)
	ByID(ctx context.Context, id string) (domain.ConsoleUser, error)
	List(ctx context.Context) ([]domain.ConsoleUser, error)
	Update(ctx context.Context, u domain.ConsoleUser) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
}
```

`filestore/console_user.go` — mirror `office.go`: struct with `sync.RWMutex`, `path`, in-memory `map[string]domain.ConsoleUser` keyed by ID, plus lookup by `NormalizeUsername`. `Create` returns `ErrUsernameTaken` if any existing user has same normalized username. Persist to JSON on each write (reuse office.go's load/save helpers style).

`mongodb/repository/console_user.go` — `const consoleUserCollection = "console_users"`; in constructor create unique index on `username`. `Create` maps a duplicate-key error to `port.ErrUsernameTaken`; `ByUsername` uses `NormalizeUsername`; `mongo.ErrNoDocuments` → `port.ErrUserNotFound`. Follow `office.go` mongo repo exactly for connection/context handling.

- [ ] **Step 4: Run filestore test — expect PASS**

- [ ] **Step 5: Commit** — `git commit -m "feat(console): ConsoleUserRepository port + file/mongo repos"`

---

## Task 3: TokenIssuer (JWT) + config

**Files:**
- Create: `internal/core/port/token.go`
- Create: `internal/adapter/auth/jwt.go`
- Modify: `internal/adapter/config/config.go` (add `Console` config)
- Test: `internal/adapter/auth/jwt_test.go`

**Interfaces:**
- Produces `port.TokenIssuer`:
  ```go
  type Claims struct { UserID, Username string; Role domain.Role }
  type TokenIssuer interface {
      Issue(c Claims) (string, error)
      Verify(token string) (Claims, error) // err ถ้า signature/exp ผิด
  }
  ```
- Produces `auth.NewJWTIssuer(secret string, ttl time.Duration) port.TokenIssuer`
- Config: add fields to config struct — `Console.JWTSecret string` (env `CONSOLE_JWT_SECRET`), `Console.TokenTTL` (default 8h), reuse `Admin.ConsoleToken` for break-glass.

Add dep: `go get github.com/golang-jwt/jwt/v5`

- [ ] **Step 1: Write failing test**

```go
package auth

import (
	"testing"
	"time"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

func TestIssueVerifyRoundtrip(t *testing.T) {
	iss := NewJWTIssuer("s3cr3t", time.Hour)
	tok, err := iss.Issue(port.Claims{UserID: "u1", Username: "earth24", Role: domain.RoleAdmin})
	if err != nil { t.Fatal(err) }
	c, err := iss.Verify(tok)
	if err != nil || c.UserID != "u1" || c.Role != domain.RoleAdmin {
		t.Fatalf("got %+v %v", c, err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, _ := NewJWTIssuer("a", time.Hour).Issue(port.Claims{UserID: "u1", Role: domain.RoleAdmin})
	if _, err := NewJWTIssuer("b", time.Hour).Verify(tok); err == nil {
		t.Fatal("must reject wrong secret")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	tok, _ := NewJWTIssuer("a", -time.Minute).Issue(port.Claims{UserID: "u1", Role: domain.RoleViewer})
	if _, err := NewJWTIssuer("a", -time.Minute).Verify(tok); err == nil {
		t.Fatal("must reject expired")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement `auth/jwt.go`** using `jwt/v5` `RegisteredClaims` + custom `Username`/`Role`, `SigningMethodHS256`, force method check in keyfunc:

```go
func (j *jwtIssuer) Verify(tokenStr string) (port.Claims, error) {
	var cl customClaims
	_, err := jwt.ParseWithClaims(tokenStr, &cl, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected alg")
		}
		return j.secret, nil
	})
	if err != nil { return port.Claims{}, err }
	return port.Claims{UserID: cl.Subject, Username: cl.Username, Role: domain.Role(cl.Role)}, nil
}
```

Add config fields; in `_cmd/main.go` read `CONSOLE_JWT_SECRET` (later task wires it).

- [ ] **Step 4: Run — expect PASS**

- [ ] **Step 5: Commit** — `git commit -m "feat(console): JWT TokenIssuer + console config"`

---

## Task 4: console_auth service — hashing, register, login, lockout

**Files:**
- Create: `internal/core/service/console_auth.go`
- Test: `internal/core/service/console_auth_test.go`

**Interfaces:**
- Consumes: `port.ConsoleUserRepository`, `port.TokenIssuer`, `domain.*`
- Produces `service.NewConsoleAuth(repo port.ConsoleUserRepository, tokens port.TokenIssuer, now func() time.Time) *ConsoleAuth` with methods:
  - `NeedsSetup(ctx) (bool, error)`
  - `Register(ctx, username, display, pin string) (domain.ConsoleUser, string, error)` — first-run only; ถ้า Count>0 → `ErrSetupDone`
  - `Login(ctx, username, pin string) (domain.ConsoleUser, string, error)` — lockout; รวม error เป็น `ErrInvalidCredentials`; ถ้า locked → `ErrAccountLocked`
  - `CreateUser(ctx, actor domain.ConsoleUser, username, display string, role domain.Role, pin string) (domain.ConsoleUser, error)`
  - `ListUsers(ctx) ([]domain.ConsoleUser, error)`
  - `PatchUser(ctx, id string, display *string, role *domain.Role, status *string) (domain.ConsoleUser, error)` — กันปลด/ปิด admin คนสุดท้าย
  - `ResetPIN(ctx, id, newPIN string) error`
  - `ChangePIN(ctx, userID, oldPIN, newPIN string) error`
  - `Delete(ctx, actorID, id string) error` — กันลบตัวเอง / admin คนสุดท้าย
- Sentinel errors: `ErrSetupDone, ErrInvalidCredentials, ErrAccountLocked, ErrInvalidPIN, ErrLastAdmin, ErrCannotDeleteSelf`
- Hashing: `bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)`, verify `bcrypt.CompareHashAndPassword`

Add dep: bcrypt already vendored indirectly; run `go get golang.org/x/crypto/bcrypt` to make direct.

- [ ] **Step 1: Write failing tests** (use filestore repo + real JWT issuer; inject fixed `now`)

```go
package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/adapter/storage/filestore"
	"ai-office-backend/internal/core/domain"
)

func newAuth(t *testing.T) *ConsoleAuth {
	repo, err := filestore.NewConsoleUserRepository(filepath.Join(t.TempDir(), "u.json"))
	if err != nil { t.Fatal(err) }
	return NewConsoleAuth(repo, auth.NewJWTIssuer("test", time.Hour), time.Now)
}

func TestRegisterFirstThenSetupDone(t *testing.T) {
	a := newAuth(t); ctx := context.Background()
	u, tok, err := a.Register(ctx, "Earth24", "Earth", "123456")
	if err != nil || tok == "" || u.Role != domain.RoleAdmin || u.Username != "earth24" {
		t.Fatalf("register: %+v %q %v", u, tok, err)
	}
	if _, _, err := a.Register(ctx, "other", "x", "222222"); err != ErrSetupDone {
		t.Fatalf("second register must be ErrSetupDone, got %v", err)
	}
}

func TestRegisterRejectsBadPIN(t *testing.T) {
	a := newAuth(t)
	if _, _, err := a.Register(context.Background(), "earth", "x", "12ab56"); err != ErrInvalidPIN {
		t.Fatalf("want ErrInvalidPIN got %v", err)
	}
}

func TestLoginWrongPINThenLockout(t *testing.T) {
	a := newAuth(t); ctx := context.Background()
	a.Register(ctx, "earth", "E", "123456")
	for i := 0; i < 5; i++ {
		if _, _, err := a.Login(ctx, "earth", "000000"); err != ErrInvalidCredentials {
			t.Fatalf("attempt %d: want ErrInvalidCredentials got %v", i, err)
		}
	}
	// ครั้งที่ 6 แม้ PIN ถูก ก็ต้องโดนล็อก
	if _, _, err := a.Login(ctx, "earth", "123456"); err != ErrAccountLocked {
		t.Fatalf("want ErrAccountLocked got %v", err)
	}
}

func TestLoginUnknownUserSameError(t *testing.T) {
	a := newAuth(t); a.Register(context.Background(), "earth", "E", "123456")
	if _, _, err := a.Login(context.Background(), "ghost", "123456"); err != ErrInvalidCredentials {
		t.Fatalf("unknown user must be ErrInvalidCredentials got %v", err)
	}
}

func TestCannotRemoveLastAdmin(t *testing.T) {
	a := newAuth(t); ctx := context.Background()
	admin, _, _ := a.Register(ctx, "earth", "E", "123456")
	// demote admin คนเดียว → ต้องห้าม
	op := domain.RoleOperator
	if _, err := a.PatchUser(ctx, admin.ID, nil, &op, nil); err != ErrLastAdmin {
		t.Fatalf("demoting last admin must be ErrLastAdmin got %v", err)
	}
	if err := a.Delete(ctx, "someone_else", admin.ID); err != ErrLastAdmin {
		t.Fatalf("deleting last admin must be ErrLastAdmin got %v", err)
	}
}

func TestChangePIN(t *testing.T) {
	a := newAuth(t); ctx := context.Background()
	u, _, _ := a.Register(ctx, "earth", "E", "123456")
	if err := a.ChangePIN(ctx, u.ID, "000000", "999999"); err != ErrInvalidCredentials {
		t.Fatalf("wrong old pin must fail got %v", err)
	}
	if err := a.ChangePIN(ctx, u.ID, "123456", "999999"); err != nil {
		t.Fatalf("change pin: %v", err)
	}
	if _, _, err := a.Login(ctx, "earth", "999999"); err != nil {
		t.Fatalf("login with new pin: %v", err)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement `console_auth.go`** — each mutation validates PIN via `domain.ValidPIN`, normalizes username, hashes with bcrypt. `Login`: fetch by username; if not found → still `ErrInvalidCredentials`; if `Locked(now)` → `ErrAccountLocked`; compare hash; on fail `FailedAttempts++`, if `>=5` set `LockedUntil=now+15m`, `FailedAttempts=0`, persist, return `ErrInvalidCredentials`; on success reset counters, set `LastLoginAt`, persist, issue token. `PatchUser`/`Delete` count admins before demote/disable/delete; block if it would drop active-admin count to 0. `Delete` also blocks `actorID == id` → `ErrCannotDeleteSelf`.

- [ ] **Step 4: Run — expect PASS**

- [ ] **Step 5: Commit** — `git commit -m "feat(console): auth service — register/login/lockout/user mgmt"`

---

## Task 5: ConsoleAuth (JWT) + RequireRole middleware

**Files:**
- Modify: `internal/adapter/handler/gin/middleware.go` (replace `ConsoleAuth`, add `RequireRole`)
- Modify: `internal/adapter/handler/gin/gin.go` (`Deps` gets `Tokens port.TokenIssuer`, `ConsoleToken` stays for break-glass)
- Test: `internal/adapter/handler/gin/middleware_console_test.go`

**Interfaces:**
- Consumes: `port.TokenIssuer`, `domain.Role`
- Produces middleware:
  - `ConsoleAuth(tokens port.TokenIssuer, breakGlass string) gin.HandlerFunc` — sets ctx `console_role`, `console_user_id`, `console_username`
  - `RequireRole(min domain.Role) gin.HandlerFunc` — reads `console_role`, 403 `FORBIDDEN` if `!role.AtLeast(min)`
  - helper `ConsoleRoleFrom(c) domain.Role`

- [ ] **Step 1: Write failing test** — build a tiny gin router with `ConsoleAuth` + a `RequireRole(operator)` route; assert: no header → 401; break-glass token → admin passes; valid JWT viewer → 403 on operator route; valid JWT operator → 200; garbage JWT → 401. (Use httptest like existing test harness.)

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement** — `ConsoleAuth`: strip `Bearer `; if `== breakGlass && breakGlass != ""` → role admin, username `break-glass`; else `tokens.Verify` → set ctx; else 401. `RequireRole`: compare via `AtLeast`.

- [ ] **Step 4: Run — expect PASS**

- [ ] **Step 5: Commit** — `git commit -m "feat(console): JWT ConsoleAuth + RequireRole middleware"`

---

## Task 6: auth + users routes, gate office routes, wire main

**Files:**
- Create: `internal/adapter/handler/gin/routes/auth.go`
- Modify: `internal/adapter/handler/gin/gin.go` (register routes, apply role gates to office routes)
- Modify: `_cmd/main.go` (construct repo, issuer, service; read env)
- Test: `test/console_auth_test.go` (integration, httptest via `NewTestRouter`)

**Interfaces:**
- Consumes: `service.ConsoleAuth`, middleware from Task 5
- Routes per spec §5. Handlers return via `routes.ResData`. Map service errors → HTTP:
  `ErrSetupDone→409`, `ErrInvalidCredentials→401`, `ErrAccountLocked→423`, `ErrInvalidPIN→400`, `ErrUsernameTaken→409`, `ErrLastAdmin/ErrCannotDeleteSelf→409`, `ErrUserNotFound→404`.
- Office route gates: wrap existing group — GET allowed for any authenticated; add `RequireRole(domain.RoleOperator)` to POST/PATCH/DELETE office & service & rotate-key; user routes under `RequireRole(domain.RoleAdmin)`.

- [ ] **Step 1: Write failing integration tests** — extend the `test/` harness (helper builds router with a real filestore ConsoleUser repo + JWT issuer). Cases:
  - `GET /api/ai/admin/auth/status` → `{needs_setup:true}` initially
  - `POST /auth/register` first → 200 + token; second → 409
  - `POST /auth/login` good → token; wrong ×5 then locked → 423
  - operator token → `POST /offices` 200; viewer token → `POST /offices` 403; viewer → `GET /offices` 200
  - non-admin → `GET /users` 403; admin → 200
  - break-glass token (`ADMIN_CONSOLE_TOKEN`) still works on `POST /offices`

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement handlers + wire** — add to `gin.go` route group; construct in `_cmd/main.go`: pick repo by `STORE_DRIVER`, `auth.NewJWTIssuer(cfg.Console.JWTSecret, cfg.Console.TokenTTL)`, `service.NewConsoleAuth(...)`. If `CONSOLE_JWT_SECRET` empty and `APP_MODE!=dev` → `log.Fatal`.

- [ ] **Step 4: Run `go test ./...` — expect PASS**

- [ ] **Step 5: Commit** — `git commit -m "feat(console): auth/users routes + role-gated office routes"`

---

## Task 7: Frontend — auth client, Pinia store, router guard

**Files:** (`ai-office-report/`)
- Create: `src/services/api/auth.ts`
- Modify: `src/services/api/client.ts` (401 → clear token + redirect)
- Create: `src/stores/auth.ts`
- Modify: `src/router/index.ts` (routes + `beforeEach`)

**Interfaces:**
- `auth.ts`: `status(): Promise<{needs_setup:boolean}>`, `login(username,pin)`, `register(...)`, `me()`, `changePin(old,new)`, `listUsers()`, `createUser(...)`, `patchUser(...)`, `resetPin(id,pin)`, `deleteUser(id)` — all via existing `client.ts` fetch wrapper.
- `stores/auth.ts` (Pinia): state `token, user`; actions `login`, `logout`, `loadMe`, `ensureLoaded`; getters `isAuthed`, `isAdmin`, `canEdit`(admin||operator). Token in `localStorage['ai_office_console_token']`.
- Router: add `/login`, `/setup` (no auth); existing `/offices` requires auth; `/users` `meta:{role:'admin'}`. `beforeEach`: if target needs auth and `!token` → `/login`; call `status()` when no token to decide `/setup` vs `/login`; enforce role meta.

- [ ] **Step 1:** write a Vitest unit test for the store (`token set on login`, `logout clears`, `isAdmin` from role) if store logic is non-trivial; else a router-guard test. Run → FAIL.
- [ ] **Step 2:** Implement store + auth.ts + guard.
- [ ] **Step 3:** Run `npx vitest run` → PASS; `npm run type-check` clean.
- [ ] **Step 4: Commit** — `git commit -m "feat(console-ui): auth client + store + router guard"`

---

## Task 8: Frontend — LoginView + SetupView (PIN 6 segmented)

**Files:**
- Create: `src/views/auth/LoginView.vue`
- Create: `src/views/auth/SetupView.vue`
- Create: `src/components/PinInput.vue` (6-box segmented, auto-advance, paste, emits string)

**Interfaces:**
- `PinInput.vue`: props `modelValue:string`, emits `update:modelValue`; renders 6 numeric inputs; typing advances focus; backspace goes back; paste of 6 digits fills all; exposes filled=6 → emit `complete`.
- `LoginView`: brand card, `a-input` username + `PinInput`, submit → `authStore.login`; show `INVALID_CREDENTIALS`/`ACCOUNT_LOCKED` messages; on success → redirect `/offices` (or `redirect` query).
- `SetupView`: shown when `needs_setup`; fields username, display_name, PIN, confirm PIN; calls `register`; on success stores token → `/offices`. Guard: if `!needs_setup` → redirect `/login`.

Design tokens: reuse `--color-accent:#0f6e63`, IBM Plex Thai; centered card ~380px, soft shadow, generous spacing; PIN boxes 48px, rounded, focus ring accent.

- [ ] **Step 1:** Vitest component test for `PinInput` (typing 6 digits emits full string; paste fills). Run → FAIL.
- [ ] **Step 2:** Implement `PinInput`, `LoginView`, `SetupView`.
- [ ] **Step 3:** `npx vitest run` PASS; manual check `npm run dev` (task in Cursor terminal) — login/setup render, PIN works.
- [ ] **Step 4: Commit** — `git commit -m "feat(console-ui): Login + Setup views with 6-digit PIN input"`

---

## Task 9: Frontend — user menu/logout in layout + role-aware nav

**Files:**
- Modify: `src/layouts/ConsoleLayout.vue` (remove token box; add user menu: display name + role badge, dropdown → เปลี่ยน PIN (modal), logout; show "Users" nav only for `isAdmin`)
- Modify: `src/views/offices/OfficesView.vue` (disable mutation buttons when `!canEdit`)
- Create: `src/components/ChangePinModal.vue`

**Interfaces:** uses `stores/auth` getters. `ChangePinModal`: old PIN + new PIN + confirm → `auth.changePin`.

- [ ] **Step 1:** implement; type-check.
- [ ] **Step 2:** manual verify: viewer sees disabled edit; admin sees Users link; logout returns to /login.
- [ ] **Step 3: Commit** — `git commit -m "feat(console-ui): user menu, logout, change-PIN, role-aware office actions"`

---

## Task 10: Frontend — UsersView (admin)

**Files:**
- Create: `src/views/users/UsersView.vue`
- Modify: `src/router/index.ts` (already added `/users` in Task 7 — confirm nav link)

**Interfaces:** Antd table of users (username, display, role tag, status, last login). Actions: create (modal: username/display/role/PIN), edit role/status (modal), reset PIN (modal → shows the new PIN once), delete (confirm). All via `auth.ts` users calls. Reflect service errors (409 last-admin/self, 409 username taken) as `message.error`.

- [ ] **Step 1:** implement UsersView + modals; type-check.
- [ ] **Step 2:** manual verify against running backend: create operator, login as it, confirm role gating; reset PIN; try delete last admin → error shown.
- [ ] **Step 3: Commit** — `git commit -m "feat(console-ui): Users management screen (admin)"`

---

## Task 11: End-to-end polish + docs

**Files:**
- Modify: `.env.example` (both repos): add `CONSOLE_JWT_SECRET`, note lockout/TTL
- Modify: `README.md` (backend): document setup flow (first-run register, break-glass token, roles)
- Verify: `go test ./...` green; `npx vitest run` green (both FE test suites); `npm run type-check` clean; `go build ./...` clean

- [ ] **Step 1:** run full backend + widget + report test suites; fix any gaps.
- [ ] **Step 2:** manual smoke: fresh store → /setup → create admin → create operator/viewer → verify gating end to end against `localhost:6767`.
- [ ] **Step 3: Commit** — `git commit -m "docs(console): setup flow + env; final verification"`

---

## Self-review notes
- Spec coverage: §3 model→T1/T2; §4 JWT/lockout→T3/T4/T5; §5 routes→T6; §6 FE→T7–T10; §8 risks→Review Focus tests in T1/T2/T5/T6.
- Review Focus items each mapped to an owning task's test (bad PIN→T1/T4; dup username→T2/T6; JWT tampering→T3/T5; last-admin/self→T4; re-register→T4/T6).
- Break-glass token preserved throughout (no hard cutover) per Global Constraints.

---

## Plan revision R1 — TOTP 2FA (mandatory) + filestore persistence

อ้างอิง spec §9. Deltas ต่อ task (จะ fold เข้า dispatch ของแต่ละ task):

- **Task 1 (domain)** — เพิ่ม field: `TOTPSecret string json:"-" bson:"totp_secret"`, `TOTPEnrolled bool json:"totp_enrolled" bson:"totp_enrolled"`, `RecoveryHashes []string json:"-" bson:"recovery_hashes"` (ทำผ่าน Task 2 fix แล้ว)
- **Task 2 (repo)** — filestore serialize ผ่าน unexported persist DTO ที่มี tag ครบทุก field (pin_hash/totp_secret/recovery_hashes/failed/locked/...) กัน field หาย + test round-trip (ทำใน Task 2 fix)
- **Task 3 (JWT/config)** — ไม่เปลี่ยนโครง; TokenIssuer ตัวเดิมใช้ออก session JWT. ticket (stage=enroll/totp, อายุ 5 นาที) เป็น JWT อีกชนิดที่เซ็นด้วย secret เดียวกัน — จะทำใน Task 3.5
- **Task 3.5 (ใหม่) — TOTP service** `internal/core/service/totp.go` (+ dep `github.com/pquerna/otp/totp`):
  - `GenerateSecret() (secret, otpauthURI string)` (issuer="AI Office Console", account=username)
  - `ValidateCode(secret, code string) bool`
  - `GenerateRecoveryCodes() (plain []string, hashes []string)` (8 โค้ด, hash = bcrypt)
  - ticket sign/verify: `IssueTicket(userID, stage string, pendingSecret string) string` / `VerifyTicket(t) (userID, stage, pendingSecret, err)` (JWT อายุ 5 นาที, claim stage)
  - unit test: enroll secret → code ถูกต้อง validate ผ่าน; recovery code hash verify; ticket round-trip + หมดอายุ reject
- **Task 4 (auth service)** — login เป็น 2 ขั้น:
  - `Login(username,pin)` → เช็ค PIN+lockout → ถ้ายังไม่ enroll คืน `{Stage:"enroll", Ticket, OTPAuthURI, Secret}`; ถ้า enroll แล้วคืน `{Stage:"totp", Ticket}` (ยังไม่ออก session)
  - `VerifyTOTP(ticket, code)` → stage enroll: validate code กับ pendingSecret ใน ticket → set TOTPSecret+Enrolled + สร้าง recovery codes (คืน plain ครั้งเดียว) → ออก session JWT; stage totp: validate code กับ TOTPSecret หรือ recovery code (ใช้ครั้งเดียว ลบ hash) → ออก session JWT
  - `Reset2FA(id)` (admin) — ล้าง TOTPSecret+Enrolled+RecoveryHashes → user enroll ใหม่รอบหน้า
  - Register/CreateUser/ChangePIN/ResetPIN/lockout/last-admin ตามเดิม (ผู้ใช้ใหม่ยังไม่ enroll → ถูกบังคับ enroll ตอน login ครั้งแรก)
- **Task 6 (routes)** — login คืน stage+ticket (+otpauth_uri/secret ถ้า enroll) ไม่ใช่ token ตรงๆ; เพิ่ม `POST /auth/totp/verify {ticket,code}` (public) → `{token,user,recovery_codes?}`; เพิ่ม `POST /users/:id/reset-2fa` (admin). error map เพิ่ม: ticket ผิด/หมดอายุ→401, code ผิด→401 `INVALID_2FA`
- **Task 8/9 (FE)** — Login/Setup เป็น flow 2 ขั้น: หลังกรอก PIN ถ้า stage=enroll แสดง **หน้า Enroll** (QR จาก otpauth_uri + secret + ช่องกรอกโค้ดยืนยัน + แสดง recovery codes ครั้งเดียวให้จด), ถ้า stage=totp แสดงช่องกรอกโค้ด TOTP 6 หลัก (reuse PinInput). เก็บ ticket ชั่วคราวใน memory (ไม่ localStorage). dep FE: `qrcode` (สร้าง QR จาก otpauth URI)

Ruling: split auth service ไม่ทำ — คง Task 4 เป็นก้อนเดียวแต่ใหญ่ขึ้น (login 2 ขั้น + 2FA) เพราะ logic ผูกกันแน่น (ticket/stage) แยกแล้ว interface ยิบย่อย — ถ้าใหญ่ไป implementer แจ้ง DONE_WITH_CONCERNS ได้
