// Package filestore เก็บ role config (รายชื่อ role + matrix) ลงไฟล์ JSON
//
// เป็น adapter ของ port.RoleConfigRepository — สลับไปใช้ MongoDB ได้โดยแก้
// _cmd/main.go บรรทัดเดียว ไม่ต้องแตะ service หรือ handler (มิเรอร์ console_user.go)
package filestore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type roleConfigRepo struct {
	mu   sync.RWMutex
	path string
}

func NewRoleMatrixRepository(path string) (port.RoleConfigRepository, error) {
	return &roleConfigRepo{path: path}, nil
}

// legacyRoleConfig รองรับไฟล์รูปแบบเก่า (มีแค่ matrix ไม่มี roles) จากก่อนที่ role จะกลาย
// เป็น list ที่แก้ได้ — Get ต้อง migrate ให้ใช้ได้ต่อ ไม่ใช่ error หรือทำข้อมูลหาย
type legacyRoleConfig struct {
	Roles  []domain.RoleDef    `json:"roles"`
	Matrix map[string][]string `json:"matrix"`
}

func (r *roleConfigRepo) Get(ctx context.Context) (domain.RoleConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return domain.DefaultRoleConfig(), nil
	}
	if err != nil {
		return domain.RoleConfig{}, err
	}
	var doc legacyRoleConfig
	if err := json.Unmarshal(b, &doc); err != nil {
		return domain.RoleConfig{}, err
	}
	if len(doc.Matrix) == 0 {
		return domain.DefaultRoleConfig(), nil
	}
	return migrateRoleConfig(doc.Roles, doc.Matrix), nil
}

// migrateRoleConfig: ถ้าไฟล์เก่ามี matrix แต่ไม่มี roles (รูปแบบก่อนหน้านี้) ให้สร้าง roles
// list ให้ตาม key ที่เจอใน matrix เอง (สังเคราะห์ label/builtin ตามค่า default ที่รู้จัก)
func migrateRoleConfig(roles []domain.RoleDef, matrix map[string][]string) domain.RoleConfig {
	if len(roles) > 0 {
		return domain.RoleConfig{Roles: roles, Matrix: matrix}
	}
	def := domain.DefaultRoleConfig()
	known := map[string]domain.RoleDef{}
	for _, r := range def.Roles {
		known[r.Key] = r
	}
	synth := make([]domain.RoleDef, 0, len(matrix))
	for key := range matrix {
		if rd, ok := known[key]; ok {
			synth = append(synth, rd)
			continue
		}
		synth = append(synth, domain.RoleDef{Key: key, Label: key, Builtin: key == string(domain.RoleAdmin)})
	}
	return domain.RoleConfig{Roles: synth, Matrix: matrix}
}

func (r *roleConfigRepo) Save(ctx context.Context, cfg domain.RoleConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, err := json.MarshalIndent(cfg, "", "  ")
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
