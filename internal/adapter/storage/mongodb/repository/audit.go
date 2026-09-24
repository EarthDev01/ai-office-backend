package repository

import (
	"context"
	"regexp"
	"sort"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const auditCollection = "audit_logs"

type auditRepo struct {
	col *mongo.Collection
}

// NewAuditRepository สร้าง index ตามรูปแบบการค้นที่หน้าประวัติใช้จริง
// (ทั้งหมดเรียงเวลาใหม่→เก่า) — ไม่ตั้ง TTL: ประวัติต้องอยู่จนกว่าจะมีนโยบายเก็บที่ชัดเจน
func NewAuditRepository(ctx context.Context, db *mongo.Database) (port.AuditRepository, error) {
	col := db.Collection(auditCollection)
	_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "at", Value: -1}}, Options: options.Index().SetName("at_desc")},
		{Keys: bson.D{{Key: "actor", Value: 1}, {Key: "at", Value: -1}}, Options: options.Index().SetName("actor_at")},
		{Keys: bson.D{{Key: "category", Value: 1}, {Key: "at", Value: -1}}, Options: options.Index().SetName("category_at")},
		{Keys: bson.D{{Key: "target_type", Value: 1}, {Key: "target_id", Value: 1}, {Key: "at", Value: -1}}, Options: options.Index().SetName("target_at")},
	})
	if err != nil {
		return nil, err
	}
	return &auditRepo{col: col}, nil
}

func (r *auditRepo) Append(ctx context.Context, e domain.AuditEntry) error {
	_, err := r.col.InsertOne(ctx, e)
	return err
}

func (r *auditRepo) Query(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	f.Normalize()
	filter := bson.M{}
	if f.Actor != "" {
		// เทียบตรง ๆ ได้เพราะทั้งฝั่งบันทึกและฝั่งค้นผ่าน NormalizeUsername — ใช้ index actor_at ได้เต็ม
		filter["actor"] = f.Actor
	}
	if f.Category != "" {
		filter["category"] = f.Category
	}
	if f.Action != "" {
		filter["action"] = f.Action
	}
	if f.Status != "" {
		filter["status"] = f.Status
	}
	if f.TargetType != "" {
		filter["target_type"] = f.TargetType
	}
	if f.TargetID != "" {
		filter["target_id"] = f.TargetID
	}
	if !f.From.IsZero() || !f.To.IsZero() {
		at := bson.M{}
		if !f.From.IsZero() {
			at["$gte"] = f.From
		}
		if !f.To.IsZero() {
			at["$lt"] = f.To
		}
		filter["at"] = at
	}
	if f.Q != "" {
		// QuoteMeta กัน regex injection จากช่องค้นหา
		rx := bson.M{"$regex": regexp.QuoteMeta(f.Q), "$options": "i"}
		filter["$or"] = bson.A{
			bson.M{"summary": rx}, bson.M{"target_id": rx}, bson.M{"target_label": rx},
			bson.M{"actor": rx}, bson.M{"ip": rx},
		}
	}

	// นับแค่พอรู้ว่าเกิน cap ไหม — นับทั้งหมดบน collection ใหญ่ช้าเกินไปสำหรับทุกหน้า
	total, err := r.col.CountDocuments(ctx, filter, options.Count().SetLimit(int64(domain.AuditCountCap+1)))
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "at", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((f.Page - 1) * f.PageSize)).
		SetLimit(int64(f.PageSize))
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	out := []domain.AuditEntry{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	for i := range out {
		for j := range out[i].Changes {
			out[i].Changes[j].Before = plain(out[i].Changes[j].Before)
			out[i].Changes[j].After = plain(out[i].Changes[j].After)
		}
	}
	return out, int(total), nil
}

// plain แปลงค่าที่ driver decode ลง any (primitive.D / primitive.A) กลับเป็น map/slice ธรรมดา
// ไม่งั้น object อย่าง placement จะออก JSON เป็น [{"Key":..,"Value":..}] แทน {"position":..}
func plain(v any) any {
	switch x := v.(type) {
	case primitive.D:
		m := make(map[string]any, len(x))
		for _, e := range x {
			m[e.Key] = plain(e.Value)
		}
		return m
	case primitive.M:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = plain(e)
		}
		return m
	case primitive.A:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = plain(e)
		}
		return out
	}
	return v
}

func (r *auditRepo) Actors(ctx context.Context) ([]string, error) {
	vals, err := r.col.Distinct(ctx, "actor", bson.M{"actor": bson.M{"$ne": ""}})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out, nil
}
