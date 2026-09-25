package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// Sentinel errors ของการจัดการ role — AddRole/RenameRole/DeleteRole
//
// ErrInvalidRole ใช้ร่วมกับ console_auth.go (คนละ endpoint แต่ความหมายเดียวกัน: "role key นี้ใช้ไม่ได้")
var (
	ErrRoleExists = errors.New("ROLE_EXISTS")
	ErrRoleLocked = errors.New("ROLE_LOCKED")
)

// PermissionService ห่อ port.RoleConfigRepository พร้อม in-memory cache สั้น ๆ
//
// route gating ทุก request ต้องเรียก Can() — cache กันไม่ให้ยิง repo (โดยเฉพาะ mongo)
// ทุก request แต่ยัง invalidate ทันทีตอน mutate (SetMatrix/AddRole/RenameRole/DeleteRole)
// เพื่อไม่ให้ config ใหม่ค้าง
type PermissionService struct {
	repo  port.RoleConfigRepository
	audit port.AuditRecorder

	mu    sync.RWMutex
	cache *domain.RoleConfig // nil = ยังไม่ cache
}

// audit เป็น nil ได้ (ไม่บันทึกประวัติ)
func NewPermissionService(repo port.RoleConfigRepository, audit port.AuditRecorder) *PermissionService {
	return &PermissionService{repo: repo, audit: auditOr(audit)}
}

// Config คืน role config ทั้งชุด (roles list + matrix) — ผ่าน cache
func (s *PermissionService) Config(ctx context.Context) (domain.RoleConfig, error) {
	s.mu.RLock()
	if s.cache != nil {
		cfg := *s.cache
		s.mu.RUnlock()
		return cfg, nil
	}
	s.mu.RUnlock()

	cfg, err := s.repo.Get(ctx)
	if err != nil {
		return domain.RoleConfig{}, err
	}
	s.mu.Lock()
	s.cache = &cfg
	s.mu.Unlock()
	return cfg, nil
}

// invalidate ล้าง cache — เรียกท้าย mutation ทุกตัว
func (s *PermissionService) invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// Can เช็คว่า role มี permission key นี้ไหมตาม matrix ปัจจุบัน · admin ได้ทุกสิทธิ์เสมอ
func (s *PermissionService) Can(ctx context.Context, role domain.Role, perm string) (bool, error) {
	if role == domain.RoleAdmin {
		return true, nil
	}
	cfg, err := s.Config(ctx)
	if err != nil {
		return false, err
	}
	return cfg.Can(string(role), perm), nil
}

// For คืน permission key ทั้งหมดของ role นี้ (ใช้ตอบ /auth/me)
//
// admin ได้ทุกสิทธิ์ใน catalog เสมอ ไม่ว่า matrix ที่เก็บไว้จะมีอะไร — ตรงกับ RequirePermission ที่ให้ admin ผ่านทุกด่าน
// (matrix ที่บันทึกไว้ก่อนมีสิทธิ์ใหม่จะขาดสิทธิ์นั้น ถ้าอ่านจาก matrix เมนูของ admin จะหายทั้งที่ API ให้ผ่าน)
func (s *PermissionService) For(ctx context.Context, role domain.Role) ([]string, error) {
	if role == domain.RoleAdmin {
		all := []string{}
		for _, p := range domain.PermissionCatalog() {
			all = append(all, p.Key)
		}
		return all, nil
	}
	cfg, err := s.Config(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.For(string(role)), nil
}

// RoleExists เช็คว่า role นี้ยังอยู่ในรายชื่อ role ของระบบไหม — นี่คือความหมายใหม่ของ
// "role ถูกต้อง" แทนที่ domain.Role.Valid() แบบ fixed-enum เดิม (console user ใช้เช็คตอน
// CreateUser/PatchUser)
func (s *PermissionService) RoleExists(ctx context.Context, role domain.Role) (bool, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return false, err
	}
	return cfg.RoleExists(string(role)), nil
}

// RoleLabel คืนชื่อแสดงผลภาษาไทยของ role — break-glass ก็ผ่านทางนี้ได้เพราะมัน role="admin" เสมอ
func (s *PermissionService) RoleLabel(ctx context.Context, role domain.Role) (string, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	return cfg.Label(string(role)), nil
}

// SetMatrix บันทึก matrix ใหม่ (roles list ไม่เปลี่ยน — ใช้ AddRole/RenameRole/DeleteRole แก้ตรงนั้น)
//
// - กรอง permission key ที่ไม่อยู่ใน catalog ทิ้ง กัน typo/ค่าแปลกปลอมหลุดเข้าไป
// - กรอง role key ที่ไม่อยู่ใน roles list ทิ้ง (role ที่ยังไม่มี/ถูกลบไปแล้ว ตั้งสิทธิ์ไม่ได้)
// - บังคับ admin มี user.manage เสมอ — anti-lockout กันแอดมินล็อกตัวเองออกจากหน้าตั้งสิทธิ์
func (s *PermissionService) SetMatrix(ctx context.Context, m map[string][]string) error {
	cfg, err := s.Config(ctx)
	if err != nil {
		return err
	}

	validPerm := map[string]bool{}
	for _, p := range domain.PermissionCatalog() {
		validPerm[p.Key] = true
	}
	validRole := map[string]bool{}
	for _, r := range cfg.Roles {
		validRole[r.Key] = true
	}

	cleaned := make(map[string][]string, len(m))
	for role, perms := range m {
		if !validRole[role] {
			continue
		}
		kept := make([]string, 0, len(perms))
		seen := map[string]bool{}
		for _, p := range perms {
			if validPerm[p] && !seen[p] {
				kept = append(kept, p)
				seen[p] = true
			}
		}
		cleaned[role] = kept
	}

	// anti-lockout: admin ต้องมี user.manage เสมอ ไม่ว่า caller จะส่งอะไรมา
	adminKey := string(domain.RoleAdmin)
	adminPerms := cleaned[adminKey]
	hasUserManage := false
	for _, p := range adminPerms {
		if p == domain.PermUserManage {
			hasUserManage = true
			break
		}
	}
	if !hasUserManage {
		cleaned[adminKey] = append(adminPerms, domain.PermUserManage)
	}

	oldMatrix := cfg.Matrix
	cfg.Matrix = cleaned
	if err := s.repo.Save(ctx, cfg); err != nil {
		return err
	}
	s.invalidate()

	// บันทึกทีละ role ที่สิทธิ์เปลี่ยนจริง — Before/After เป็นรายการสิทธิ์เรียงแล้ว
	changes := []domain.FieldChange{}
	for _, r := range cfg.Roles {
		before, after := sortedCopy(oldMatrix[r.Key]), sortedCopy(cleaned[r.Key])
		if !sameStrings(before, after) {
			changes = append(changes, domain.FieldChange{Field: r.Key, Before: before, After: after})
		}
	}
	if len(changes) > 0 {
		names := make([]string, len(changes))
		for i, c := range changes {
			names[i] = cfg.Label(c.Field)
		}
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditRolePermissions, TargetType: "role_matrix", TargetID: "default", TargetLabel: "ตั้งค่า Permissions",
			Summary: fmt.Sprintf("แก้สิทธิ์ของ role %d ตัว: %s", len(changes), strings.Join(names, ", ")),
			Changes: changes,
		})
	}
	return nil
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AddRole สร้าง role ใหม่ในรายชื่อ role — เริ่มด้วยสิทธิ์ว่างเปล่า (แอดมินต้องมากดติ๊กเอง)
func (s *PermissionService) AddRole(ctx context.Context, key, label string) error {
	if !domain.ValidRoleKey(key) {
		return ErrInvalidRole
	}
	cfg, err := s.Config(ctx)
	if err != nil {
		return err
	}
	if key == string(domain.RoleAdmin) || cfg.RoleExists(key) {
		return ErrRoleExists
	}

	cfg.Roles = append(cfg.Roles, domain.RoleDef{Key: key, Label: label, Builtin: false})
	if cfg.Matrix == nil {
		cfg.Matrix = map[string][]string{}
	}
	cfg.Matrix[key] = []string{}

	if err := s.repo.Save(ctx, cfg); err != nil {
		return err
	}
	s.invalidate()
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditRoleCreate, TargetType: "role", TargetID: key, TargetLabel: label,
		Summary: fmt.Sprintf("สร้าง role %q (%s)", label, key),
	})
	return nil
}

// RenameRole เปลี่ยน label ของ role — ห้ามแตะ builtin (admin)
func (s *PermissionService) RenameRole(ctx context.Context, key, label string) error {
	cfg, err := s.Config(ctx)
	if err != nil {
		return err
	}
	idx := -1
	for i, r := range cfg.Roles {
		if r.Key == key {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrInvalidRole
	}
	if cfg.Roles[idx].Builtin {
		return ErrRoleLocked
	}
	oldLabel := cfg.Roles[idx].Label
	cfg.Roles[idx].Label = label

	if err := s.repo.Save(ctx, cfg); err != nil {
		return err
	}
	s.invalidate()
	if oldLabel != label {
		s.audit.Record(ctx, domain.AuditEntry{
			Action: domain.AuditRoleRename, TargetType: "role", TargetID: key, TargetLabel: label,
			Summary: fmt.Sprintf("เปลี่ยนชื่อ role %s จาก %q เป็น %q", key, oldLabel, label),
			Changes: []domain.FieldChange{{Field: "label", Before: oldLabel, After: label}},
		})
	}
	return nil
}

// DeleteRole ลบ role ออกจากรายชื่อ + matrix — ห้ามแตะ builtin (admin)
//
// การเช็ค "role นี้ยังมี user ใช้อยู่ไหม" เป็นหน้าที่ของ handler (ต้องรู้จัก console user
// repository ซึ่ง service นี้ไม่ผูกกับมันเลยตั้งใจ กันไม่ให้เกิด circular concern)
func (s *PermissionService) DeleteRole(ctx context.Context, key string) error {
	cfg, err := s.Config(ctx)
	if err != nil {
		return err
	}
	idx := -1
	for i, r := range cfg.Roles {
		if r.Key == key {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrInvalidRole
	}
	if cfg.Roles[idx].Builtin {
		return ErrRoleLocked
	}
	removed := cfg.Roles[idx]
	removedPerms := sortedCopy(cfg.Matrix[key])
	cfg.Roles = append(cfg.Roles[:idx], cfg.Roles[idx+1:]...)
	delete(cfg.Matrix, key)

	if err := s.repo.Save(ctx, cfg); err != nil {
		return err
	}
	s.invalidate()
	s.audit.Record(ctx, domain.AuditEntry{
		Action: domain.AuditRoleDelete, TargetType: "role", TargetID: key, TargetLabel: removed.Label,
		Summary: fmt.Sprintf("ลบ role %q (%s)", removed.Label, key),
		Changes: []domain.FieldChange{{Field: "permissions", Before: removedPerms, After: nil}},
	})
	return nil
}
