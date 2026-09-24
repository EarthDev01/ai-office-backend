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

const officeCollection = "offices"

type officeRepo struct {
	col *mongo.Collection
}

// NewOfficeRepository สร้าง index ที่จำเป็นให้เลย
//
// allowed_origins คือทางที่ widget ใช้หา office ทุกครั้ง → ต้องมี index (multikey)
// ไม่ตั้ง unique ที่ DB เพราะข้อมูลเก่าที่มีโดเมนซ้ำจะทำให้ระบบเปิดไม่ขึ้น — กันซ้ำที่ service แทน
// uniq_public_key เป็น index เดิม ยังคงไว้ (public_key เลิกใช้ระบุ office แล้ว แต่ยังออก key สุ่มให้ทุก office)
func NewOfficeRepository(ctx context.Context, db *mongo.Database) (port.OfficeRepository, error) {
	col := db.Collection(officeCollection)
	_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "public_key", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_public_key")},
		{Keys: bson.D{{Key: "allowed_origins", Value: 1}}, Options: options.Index().SetName("allowed_origins")},
	})
	if err != nil {
		return nil, err
	}
	return &officeRepo{col: col}, nil
}

func (r *officeRepo) List(ctx context.Context) ([]domain.Office, error) {
	cur, err := r.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []domain.Office{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *officeRepo) Get(ctx context.Context, id string) (domain.Office, error) {
	return r.one(ctx, bson.M{"_id": id})
}

// GetByOrigin ค้นตรงตัว — ค่าที่เก็บถูก normalize ตั้งแต่ตอนบันทึก (ของเก่าที่ยังไม่ normalize
// จะถูกเตือนผ่าน OriginIssues ตอนเริ่มระบบ)
func (r *officeRepo) GetByOrigin(ctx context.Context, origin string) (domain.Office, error) {
	if origin == "" {
		return domain.Office{}, domain.ErrNotFound
	}
	var o domain.Office
	err := r.col.FindOne(ctx, bson.M{"allowed_origins": origin},
		options.FindOne().SetSort(bson.D{{Key: "_id", Value: 1}})).Decode(&o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Office{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Office{}, err
	}
	return o, nil
}

func (r *officeRepo) one(ctx context.Context, filter bson.M) (domain.Office, error) {
	var o domain.Office
	err := r.col.FindOne(ctx, filter).Decode(&o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Office{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Office{}, err
	}
	return o, nil
}

func (r *officeRepo) Save(ctx context.Context, o domain.Office) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": o.ID}, o, options.Replace().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrConflict
	}
	return err
}

func (r *officeRepo) Delete(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetByPublicKey — ทางของ host (server-to-server /session) และ path ของ widget (w-17)
func (r *officeRepo) GetByPublicKey(ctx context.Context, key string) (domain.Office, error) {
	if key == "" {
		return domain.Office{}, domain.ErrNotFound
	}
	return r.one(ctx, bson.M{"public_key": key})
}

// AllOrigins รวม origin ของทุก office (CORS)
func (r *officeRepo) AllOrigins(ctx context.Context) ([]string, error) {
	cur, err := r.col.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"allowed_origins": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		AllowedOrigins []string `bson:"allowed_origins"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, row := range rows {
		for _, o := range row.AllowedOrigins {
			if !seen[o] {
				seen[o] = true
				out = append(out, o)
			}
		}
	}
	return out, nil
}
