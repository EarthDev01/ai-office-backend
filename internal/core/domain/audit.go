package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"
)

// AuditEntry คือ 1 เหตุการณ์ในประวัติการทำงานของคอนโซล — "ใคร ทำอะไร กับอะไร เมื่อไร จากที่ไหน"
//
// บันทึกแบบ append-only ไม่มี endpoint ให้แก้หรือลบ
// ██ ห้ามใส่ password / totp secret / recovery code / token ลงใน entry เด็ดขาด
type AuditEntry struct {
	ID string    `json:"id" bson:"_id"`
	At time.Time `json:"at" bson:"at"`

	// ผู้กระทำ — ตอน login ไม่สำเร็จ Actor คือ username ที่พิมพ์มา (อาจไม่มีอยู่จริง) ActorID จะว่าง
	ActorID   string `json:"actor_id" bson:"actor_id"`
	Actor     string `json:"actor" bson:"actor"`
	ActorRole string `json:"actor_role" bson:"actor_role"`

	Action   string `json:"action" bson:"action"`     // เช่น "office.update" — ดู Audit* ด้านล่าง
	Category string `json:"category" bson:"category"` // ส่วนหน้าของ Action ก่อนจุด (auth/office/service/user/role/access)

	TargetType  string `json:"target_type" bson:"target_type"` // office | service | user | role | role_matrix
	TargetID    string `json:"target_id" bson:"target_id"`
	TargetLabel string `json:"target_label" bson:"target_label"`

	Summary string            `json:"summary" bson:"summary"` // ประโยคภาษาไทยที่อ่านแล้วเข้าใจทันที
	Changes []FieldChange     `json:"changes" bson:"changes"`
	Meta    map[string]string `json:"meta" bson:"meta"` // ข้อมูลเสริม เช่น office_id ของ service

	Status string `json:"status" bson:"status"` // success | failure
	Reason string `json:"reason" bson:"reason"` // error code ตอน failure เช่น INVALID_CREDENTIALS

	IP        string `json:"ip" bson:"ip"`
	UserAgent string `json:"user_agent" bson:"user_agent"`
	Method    string `json:"method" bson:"method"`
	Path      string `json:"path" bson:"path"`
}

// FieldChange คือค่าก่อน/หลังของ field เดียว — Before/After เป็นค่าดิบ (string, bool, list, object)
type FieldChange struct {
	Field  string `json:"field" bson:"field"`
	Before any    `json:"before" bson:"before"`
	After  any    `json:"after" bson:"after"`
}

const (
	AuditSuccess = "success"
	AuditFailure = "failure"
)

// Action keys — FE ใช้ string พวกนี้ตรง ๆ เพื่อแปลเป็นป้ายภาษาไทย อย่าเปลี่ยนค่าโดยไม่แก้ FE
const (
	AuditAuthSetup           = "auth.setup"
	AuditAuthLogin           = "auth.login"
	AuditAuthLoginFailed     = "auth.login_failed"
	AuditAuthLocked          = "auth.account_locked"
	AuditAuth2FAEnrolled     = "auth.2fa_enrolled"
	AuditAuthRecoveryUsed    = "auth.recovery_code_used"
	AuditAuthLogout          = "auth.logout"
	AuditAuthPasswordChanged = "auth.password_changed"

	AuditOfficeCreate    = "office.create"
	AuditOfficeUpdate    = "office.update"
	AuditOfficeDelete    = "office.delete"
	AuditOfficeRotateKey = "office.rotate_key" // เลิกใช้แล้ว — คงไว้ให้ประวัติเก่ายังอ่านออก

	AuditServiceCreate = "service.create"
	AuditServiceUpdate = "service.update"
	AuditServiceDelete = "service.delete"

	AuditUserCreate        = "user.create"
	AuditUserUpdate        = "user.update"
	AuditUserResetPassword = "user.reset_password"
	AuditUserReset2FA      = "user.reset_2fa"
	AuditUserDelete        = "user.delete"

	AuditRoleCreate      = "role.create"
	AuditRoleRename      = "role.rename"
	AuditRoleDelete      = "role.delete"
	AuditRolePermissions = "role.permissions_update"

	AuditAccessDenied = "access.denied"
)

// AuditCategory คือส่วนหน้าของ action ก่อนจุดแรก
func AuditCategory(action string) string {
	if i := strings.IndexByte(action, '.'); i > 0 {
		return action[:i]
	}
	return action
}

func NewAuditID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ช่วงเวลาที่ค้นได้ต่อครั้ง — ประวัติไม่มีวันหมดอายุ จึงต้องจำกัดช่วงค้นแทน
// ไม่งั้นการค้นแบบ contains กับการนับจำนวนจะต้องไล่อ่านทั้ง collection
const (
	AuditDefaultRange = 30 * 24 * time.Hour // ไม่ระบุ from = ย้อนหลัง 30 วัน
	AuditMaxRange     = 62 * 24 * time.Hour // 2 เดือน (ครอบคลุม 2 เดือนปฏิทินใด ๆ)

	// AuditCountCap นับจำนวนที่ตรงเงื่อนไขอย่างมากเท่านี้ — เกินนี้ตอบว่า "10,000+"
	AuditCountCap = 10000
)

var (
	ErrAuditRangeInvalid = errors.New("AUDIT_RANGE_INVALID")
	ErrAuditRangeTooWide = errors.New("AUDIT_RANGE_TOO_WIDE")
)

// AuditFilter คือเงื่อนไขค้นประวัติ — ค่าว่าง = ไม่กรองช่องนั้น
type AuditFilter struct {
	Actor      string
	Category   string
	Action     string
	Status     string
	TargetType string
	TargetID   string
	Q          string // ค้นแบบ contains (ไม่สนตัวพิมพ์) ใน summary/target/actor
	From       time.Time
	To         time.Time
	Page       int // เริ่มที่ 1
	PageSize   int
}

// Normalize ใส่ค่า default ของหน้า/ขนาดหน้า และกันขนาดใหญ่เกินไป
func (f *AuditFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 50
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
	f.Q = strings.TrimSpace(f.Q)
	// actor ถูกบันทึกเป็น NormalizeUsername เสมอ — normalize ฝั่งค้นด้วยจะได้เทียบตรง ๆ ใช้ index ได้
	f.Actor = NormalizeUsername(f.Actor)
}

// ApplyRange ใส่ช่วงเวลา default และกันช่วงที่กว้างเกิน AuditMaxRange
//
// ไม่ระบุ to = ไม่มีขอบบน (เห็นรายการที่เพิ่งบันทึก) แต่ใช้ now เป็นปลายช่วงตอนวัดความกว้าง
func (f *AuditFilter) ApplyRange(now time.Time) error {
	end := f.To
	if end.IsZero() {
		end = now
	}
	if f.From.IsZero() {
		f.From = end.Add(-AuditDefaultRange)
	}
	if !f.From.Before(end) {
		return ErrAuditRangeInvalid
	}
	if end.Sub(f.From) > AuditMaxRange {
		return ErrAuditRangeTooWide
	}
	return nil
}

// AuditPage คือผลค้นประวัติ 1 หน้า — TotalCapped = จำนวนจริงมากกว่า Total (นับถึงแค่ AuditCountCap)
type AuditPage struct {
	Items       []AuditEntry
	Total       int
	TotalCapped bool
}

// Match ใช้กับ store ที่กรองในหน่วยความจำ (filestore) — Mongo แปลเป็น query เอง
func (f AuditFilter) Match(e AuditEntry) bool {
	if f.Actor != "" && !strings.EqualFold(e.Actor, f.Actor) {
		return false
	}
	if f.Category != "" && e.Category != f.Category {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
		return false
	}
	if f.Status != "" && e.Status != f.Status {
		return false
	}
	if f.TargetType != "" && e.TargetType != f.TargetType {
		return false
	}
	if f.TargetID != "" && e.TargetID != f.TargetID {
		return false
	}
	if !f.From.IsZero() && e.At.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && !e.At.Before(f.To) {
		return false
	}
	if f.Q != "" {
		q := strings.ToLower(f.Q)
		hay := strings.ToLower(e.Summary + "\n" + e.TargetID + "\n" + e.TargetLabel + "\n" + e.Actor + "\n" + e.IP)
		if !strings.Contains(hay, q) {
			return false
		}
	}
	return true
}

// ---- actor / request metadata ผ่าน context ----
//
// middleware ฝั่ง HTTP ใส่ค่าไว้ใน request context แล้ว service อ่านเอาตอนบันทึก
// service จึงไม่ต้องรับ ip/user-agent/actor เป็นพารามิเตอร์ทุกตัว

type AuditActor struct {
	ID       string
	Username string
	Role     Role
}

type RequestMeta struct {
	IP        string
	UserAgent string
	Method    string
	Path      string
}

type auditActorKey struct{}
type requestMetaKey struct{}

func WithAuditActor(ctx context.Context, a AuditActor) context.Context {
	return context.WithValue(ctx, auditActorKey{}, a)
}

func AuditActorFrom(ctx context.Context) (AuditActor, bool) {
	a, ok := ctx.Value(auditActorKey{}).(AuditActor)
	return a, ok
}

func WithRequestMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaKey{}, m)
}

func RequestMetaFrom(ctx context.Context) (RequestMeta, bool) {
	m, ok := ctx.Value(requestMetaKey{}).(RequestMeta)
	return m, ok
}

// ---- diff ----

// DiffFields เทียบ 2 struct ทีละ field (ตามชื่อ json) แล้วคืนเฉพาะที่เปลี่ยน เรียงตามชื่อ field
// skip = field ที่ไม่ต้องสนใจ (เช่น updated_at)
func DiffFields(before, after any, skip ...string) []FieldChange {
	b, a := toFieldMap(before), toFieldMap(after)
	skipSet := map[string]bool{}
	for _, s := range skip {
		skipSet[s] = true
	}
	keys := map[string]bool{}
	for k := range b {
		keys[k] = true
	}
	for k := range a {
		keys[k] = true
	}
	out := []FieldChange{}
	for k := range keys {
		if skipSet[k] {
			continue
		}
		if isEmptyValue(b[k]) && isEmptyValue(a[k]) {
			continue // nil กับ [] ถือว่าเท่ากัน ไม่ใช่การแก้ไขจริง
		}
		if !reflect.DeepEqual(b[k], a[k]) {
			out = append(out, FieldChange{Field: k, Before: b[k], After: a[k]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

func isEmptyValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// toFieldMap ผ่าน JSON เพื่อให้ได้ชื่อ field แบบเดียวกับที่ API ส่งออก และค่าที่เทียบกันได้ตรง ๆ
func toFieldMap(v any) map[string]any {
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m
}
