# Console Auth — login / register / authorization ด้วย PIN 6 หลัก

วันที่: 2026-09-23
ขอบเขต: `ai-office-backend` (Go) + `ai-office-report` (Vue 3 console UI)

## 1. เป้าหมายและที่มา

ตอนนี้ admin console (`/api/ai/admin/*`) ป้องกันด้วย **static token ตัวเดียว** (`ADMIN_CONSOLE_TOKEN`)
ที่ต้องเอาไปแปะเองในช่อง header ของ `ai-office-report` — ไม่มีผู้ใช้หลายคน, ไม่มี login/register,
ไม่มี role, ไม่รู้ว่าใครทำอะไร

ต้องการ: ระบบ **login / register / authorization** สำหรับ console โดยใช้ **username + PIN 6 หลัก**
พร้อม UI ที่สวยและใช้งานได้จริง

### สิ่งที่ตกลงแล้ว (จาก brainstorming)
- **Login:** username + PIN 6 หลัก
- **Register:** คนแรกที่สมัคร = admin (super) · หลังจากนั้น admin เป็นคนสร้าง user + ตั้ง PIN ให้ (ไม่มี self-register อีก)
- **Roles:** 3 ระดับ — `admin` / `operator` / `viewer`
- **UI:** login (สวย) + first-run register + หน้าจัดการ user + guard + logout

### ค่า default ที่เลือกไว้ (ปรับได้)
- JWT อายุ **8 ชั่วโมง**
- Lockout: PIN ผิด **5 ครั้ง → ล็อก 15 นาที**
- คง `ADMIN_CONSOLE_TOKEN` ไว้เป็น **break-glass admin** (bootstrap/curl/ฉุกเฉิน)

### Non-goals (รอบนี้ไม่ทำ)
- reset PIN ผ่าน email/OTP (admin reset ให้แทน)
- 2FA/TOTP (เผื่ออนาคต — PIN entropy ต่ำ ชดเชยด้วย lockout + username)
- audit log ละเอียด (แค่ `created_by`, `last_login_at`)

## 2. สถาปัตยกรรม (คงรูป hexagonal เดิม)

ฝั่ง backend เพิ่มของใหม่ตามชั้นเดิม (domain → port → service → adapter):

```
domain/console_user.go        ConsoleUser, Role, สถานะ, validate PIN
port/console_user.go          ConsoleUserRepository (interface)
port/token.go                 TokenIssuer (mint/verify JWT) — เผื่อ mock ใน test
service/console_auth.go       Register, Login, Me, ChangePIN, CreateUser, ListUsers,
                              PatchUser, ResetPIN, Delete + logic lockout/hash
adapter/storage/mongodb/repository/console_user.go   collection "console_users"
adapter/storage/filestore/console_user.go            JSON store (dev)
adapter/handler/http_request หรือ crypto/…           bcrypt + golang-jwt (มาตรฐาน)
adapter/handler/gin/routes/auth.go                   handlers auth + users
adapter/handler/gin/middleware.go                    ConsoleAuth (JWT) + RequireRole
```

ต้องเพิ่ม dependency: `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`
(x/crypto มีอยู่แล้วแบบ indirect — เลื่อนเป็น direct)

## 3. Data model

```go
type Role string // "admin" | "operator" | "viewer"

type ConsoleUser struct {
    ID             string    // hex 16 ไบต์ (crypto/rand เหมือน public_key)
    Username       string    // unique, เทียบแบบ lowercase
    DisplayName    string
    PINHash        string    // bcrypt(PIN) — ไม่เก็บ plaintext เด็ดขาด
    Role           Role
    Status         string    // "active" | "disabled"
    FailedAttempts int
    LockedUntil    time.Time // zero = ไม่ถูกล็อก
    LastLoginAt    time.Time
    CreatedAt      time.Time
    CreatedBy      string    // username ผู้สร้าง หรือ "setup"/"break-glass"
    UpdatedAt      time.Time
}
```

- Mongo collection `console_users`, unique index บน `username` (lowercase)
- Filestore: JSON ที่ `CONFIG_STORE_PATH` ข้าง offices (เช่น `./data/console_users.json`)
- **PIN validate:** `^\d{6}$` เท่านั้น (ปฏิเสธ 6 ตัวซ้ำ/เรียง เช่น 000000, 123456 — optional เตือน ไม่บล็อก)

## 4. Auth mechanism

### JWT (HS256)
- config ใหม่ `CONSOLE_JWT_SECRET` (env) — ถ้าไม่ตั้งใน production ให้ backend refuse start
- claims: `{ sub: userID, username, role, exp, iat }` เซ็นด้วย secret
- ออกตอน login สำเร็จ, อายุ 8 ชม.

### middleware `ConsoleAuth` (แทนของเดิม)
1. อ่าน `Authorization: Bearer <token>`
2. ถ้า token == `ADMIN_CONSOLE_TOKEN` (break-glass) → set role=`admin`, username=`break-glass`
3. ไม่งั้น verify JWT (signature + exp) → set `console_user_id`, `console_username`, `console_role`
4. ล้มเหลว → 401 `UNAUTHORIZED`

### `RequireRole(min Role)` middleware
ลำดับสิทธิ์ `viewer < operator < admin` · gate ต่อ route ตามตาราง §5

### Login lockout (ใน service)
- ผิด → `failed_attempts++`; ถ้า ≥5 → `locked_until = now+15m`, reset attempts
- ถ้า `locked_until > now` → 423/401 `ACCOUNT_LOCKED` (บอกเวลาที่เหลือ)
- สำเร็จ → reset `failed_attempts`, set `last_login_at`
- ไม่บอกว่า "username ไม่มี" กับ "PIN ผิด" ต่างกัน (กัน enumeration) — ตอบ `INVALID_CREDENTIALS` เหมือนกัน

## 5. Routes (backend)

Base: `/api/ai/admin`

| Method/Path | Auth | Role | หน้าที่ |
|---|---|---|---|
| `GET  /auth/status` | public | — | `{needs_setup: bool}` (true ถ้ายังไม่มี user) |
| `POST /auth/register` | public* | — | first-run เท่านั้น: สร้าง user แรก = admin · ถ้ามี user แล้ว → 409 `SETUP_DONE` |
| `POST /auth/login` | public | — | `{username,pin}` → `{token, user}` หรือ 401/`ACCOUNT_LOCKED` |
| `GET  /auth/me` | JWT | any | ข้อมูล user ปัจจุบัน |
| `POST /auth/change-pin` | JWT | any | `{old_pin,new_pin}` เปลี่ยน PIN ตัวเอง |
| `GET    /users` | JWT | admin | list users (ไม่คืน pin_hash) |
| `POST   /users` | JWT | admin | สร้าง user `{username,display_name,role,pin}` |
| `PATCH  /users/:id` | JWT | admin | แก้ `display_name/role/status` |
| `POST   /users/:id/reset-pin` | JWT | admin | ตั้ง PIN ใหม่ให้ user |
| `DELETE /users/:id` | JWT | admin | ลบ user (กันลบตัวเอง / admin คนสุดท้าย) |

Office routes เดิม (`/offices...`) เปลี่ยน guard:
- `GET` (list/get) → ต้อง login (any role)
- `POST/PATCH/DELETE` office & service, `rotate-key` → `RequireRole(operator)`
- (user management → admin เท่านั้น ตามตาราง)

\* register public แต่ถูกล็อกด้วยเงื่อนไข "มี user 0 คน" — เปิดใช้ครั้งเดียวตอน setup

## 6. Frontend (`ai-office-report`)

### client (`src/services/api/client.ts`)
- เก็บ JWT ที่ `localStorage['ai_office_console_token']` (คีย์เดิม, reuse)
- เพิ่ม `src/services/api/auth.ts`: `status, register, login, me, changePin` + users CRUD
- 401 → เคลียร์ token → เด้งไป `/login`

### store (`src/stores/auth.ts` — Pinia ใหม่)
- `user, role, token`, `login(), logout(), loadMe()`
- getters: `isAdmin, canEdit` (admin|operator)

### router guard (`src/router/index.ts`)
- `beforeEach`:
  - ถ้า `/setup` แต่ setup เสร็จแล้ว → เด้ง `/login`
  - ถ้า route ต้อง auth และไม่มี token → เด้ง `/login` (หรือ `/setup` ถ้า `needs_setup`)
  - ถ้า route มี `meta.role: 'admin'` และไม่ใช่ admin → เด้ง `/offices`

### หน้าใหม่
- **`LoginView`** — การ์ดกลางจอ, brand, ช่อง username + **PIN 6 ช่องแบบ segmented** (โฟกัสไหลอัตโนมัติ, paste ได้, กด Enter ส่ง), แสดง error/lockout สวยงาม
- **`SetupView`** — first-run: username + display_name + PIN + ยืนยัน PIN → สร้าง admin คนแรก → login ต่อทันที
- **`UsersView`** (admin) — ตาราง users (username, ชื่อ, role, สถานะ, last login) + สร้าง/แก้ role/สถานะ/reset PIN (modal Antd) · reset PIN โชว์ PIN ใหม่ให้ครั้งเดียว
- **ConsoleLayout** — แทนช่อง token ด้วย **user menu** (ชื่อ + role badge + เปลี่ยน PIN + logout) · nav โชว์ "Users" เฉพาะ admin · viewer เห็นปุ่มแก้ไขแบบ disabled

### ดีไซน์
คงระบบเดิม (teal/green `#0f6e63`, IBM Plex Sans Thai/Anuphan/Mono, ant-design-vue, Tailwind v4)
เน้น login/setup ให้ดูพรีเมียม: การ์ดนุ่ม, spacing โปร่ง, PIN segmented, motion เล็กน้อย

## 7. ลำดับ build

1. **BE core:** domain `ConsoleUser` + validate + port + repo (mongo+file) + service (bcrypt, lockout, JWT issuer) + config (`CONSOLE_JWT_SECRET`, policy) + unit tests
2. **BE routes:** auth (status/register/login/me/change-pin) + users CRUD + `ConsoleAuth`(JWT)/`RequireRole` + gate office routes + integration tests · wire ใน `_cmd/main.go`
3. **FE auth:** client/auth.ts + Pinia store + router guard + LoginView + SetupView + user menu/logout
4. **FE users:** UsersView + role-based nav/disable + change-pin
5. **ทดสอบรวม + polish** (ดีไซน์ login/setup, error states, empty states)

## 8. ความเสี่ยง / ข้อควรระวัง
- **PIN 6 หลัก entropy ต่ำ (1M)** → ชดเชยด้วย lockout + username + bcrypt (ยอมรับได้สำหรับ internal console; ยกระดับด้วย TOTP ภายหลัง)
- **break-glass token** ต้องเก็บลับ — เป็น admin เต็มสิทธิ์ (คงไว้เพื่อ recover ตอนลืม PIN admin คนสุดท้าย)
- **first-run register** ต้องล็อกแน่นด้วยเงื่อนไข user==0 กัน race (สร้าง user แรกใน transaction/lock)
- migrate: ระบบเดิมที่ใช้ static token ยังทำงานได้ (break-glass) — ไม่ break ของเดิมทันที

---

## 9. Addendum R1 — TOTP 2FA (mandatory) + filestore persistence fix

### 9.1 เหตุผล
ให้ console เท่าเทียม office (office ใช้ `SECRET_KEY_2FA` = TOTP) → เพิ่มชั้นที่สองด้วย TOTP
บังคับทุก user (mandatory), enroll ครั้งแรกตอน login ครั้งแรก, มี recovery codes

### 9.2 Data model เพิ่ม (ConsoleUser)
- `TOTPSecret string` — base32 secret (`json:"-" bson:"totp_secret"`)
- `TOTPEnrolled bool` (`json:"totp_enrolled" bson:"totp_enrolled"`)
- `RecoveryHashes []string` — bcrypt hash ของ recovery codes (`json:"-" bson:"recovery_hashes"`)

### 9.3 Login flow 2 ขั้น
1. `POST /auth/login {username,pin}` → เช็ค PIN + lockout
   - ยังไม่ enroll → `{stage:"enroll", ticket, otpauth_uri, secret}` (secret โชว์ครั้งเดียวไว้ทำ QR)
   - enroll แล้ว → `{stage:"totp", ticket}`
   - `ticket` = JWT อายุสั้น (5 นาที) claim `{sub, stage, pending_secret(เฉพาะ enroll)}` เซ็นด้วย `CONSOLE_JWT_SECRET`
2. `POST /auth/totp/verify {ticket, code}` →
   - stage enroll: verify code กับ pending_secret ใน ticket → เซ็ต TOTPSecret+TOTPEnrolled + สร้าง recovery codes (คืนครั้งเดียว) → ออก JWT session
   - stage totp: verify code กับ TOTPSecret (หรือ recovery code 1 ครั้ง → ลบ hash) → ออก JWT session
- lockout เดิมนับที่ขั้น PIN; ขั้น TOTP ผิดหลายครั้งก็ invalidate ticket (ต้องเริ่ม login ใหม่)

### 9.4 lib / deps
`github.com/pquerna/otp/totp` (สร้าง secret, otpauth URI, validate code)

### 9.5 admin reset
- admin `reset-pin` เดิมคงไว้ · เพิ่ม admin `reset-2fa` (ล้าง TOTPSecret+Enrolled → user enroll ใหม่รอบหน้า)
- break-glass token ยัง bypass ได้ (admin เต็มสิทธิ์) — ไว้กู้ตอนหลุด 2FA

### 9.6 Filestore persistence fix (บั๊กที่เจอ)
`json:"-"` บน PINHash/FailedAttempts/LockedUntil ทำ filestore (ใช้ encoding/json) ทิ้ง field เหล่านี้
→ login พังหลัง restart เมื่อ STORE_DRIVER=file
**แก้:** filestore serialize ผ่าน struct ภายใน (persist DTO) ที่มี tag ครบทุก field รวม pin_hash/totp_secret/recovery_hashes/failed_attempts/locked_until — API safety ยังพึ่ง `json:"-"` บน domain เหมือนเดิม (handler marshal domain), แต่ storage มี encoding ของตัวเอง แยกจาก transport
