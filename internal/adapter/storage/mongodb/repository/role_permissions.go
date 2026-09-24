package repository

import (
	"context"
	"errors"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const roleMatrixCollection = "role_permissions"

// roleConfigDoc คือ record เดียวทั้งคอลเลกชัน (id ตายตัว "default")
type roleConfigDoc struct {
	ID     string              `bson:"_id"`
	Roles  []domain.RoleDef    `bson:"roles"`
	Matrix map[string][]string `bson:"matrix"`
}

type roleMatrixRepo struct {
	col *mongo.Collection
}

func NewRoleMatrixRepository(ctx context.Context, db *mongo.Database) (port.RoleConfigRepository, error) {
	return &roleMatrixRepo{col: db.Collection(roleMatrixCollection)}, nil
}

func (r *roleMatrixRepo) Get(ctx context.Context) (domain.RoleConfig, error) {
	var doc roleConfigDoc
	err := r.col.FindOne(ctx, bson.M{"_id": "default"}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.DefaultRoleConfig(), nil
	}
	if err != nil {
		return domain.RoleConfig{}, err
	}
	if len(doc.Matrix) == 0 {
		return domain.DefaultRoleConfig(), nil
	}
	return migrateRoleConfig(doc.Roles, doc.Matrix), nil
}

// migrateRoleConfig: ถ้า doc เก่ามี matrix แต่ไม่มี roles (รูปแบบก่อนหน้านี้) ให้สร้าง roles
// list ให้ตาม key ที่เจอใน matrix เอง (สังเคราะห์ label/builtin ตามค่า default ที่รู้จัก)
func migrateRoleConfig(roles []domain.RoleDef, matrix map[string][]string) domain.RoleConfig {
	if len(roles) > 0 {
		return domain.RoleConfig{Roles: roles, Matrix: matrix}
	}
	def := domain.DefaultRoleConfig()
	known := map[string]domain.RoleDef{}
	for _, rd := range def.Roles {
		known[rd.Key] = rd
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

func (r *roleMatrixRepo) Save(ctx context.Context, cfg domain.RoleConfig) error {
	doc := roleConfigDoc{ID: "default", Roles: cfg.Roles, Matrix: cfg.Matrix}
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": "default"}, doc, options.Replace().SetUpsert(true))
	return err
}
