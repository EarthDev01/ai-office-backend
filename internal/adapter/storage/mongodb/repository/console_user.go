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

const consoleUserCollection = "console_users"

type consoleUserRepo struct {
	col *mongo.Collection
}

// NewConsoleUserRepository สร้าง index ที่จำเป็นให้เลย
//
// username ต้อง unique เพราะเป็นตัวที่ใช้ login — กันไม่ให้สร้างซ้ำ
func NewConsoleUserRepository(ctx context.Context, db *mongo.Database) (port.ConsoleUserRepository, error) {
	col := db.Collection(consoleUserCollection)
	_, err := col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_username"),
	})
	if err != nil {
		return nil, err
	}
	return &consoleUserRepo{col: col}, nil
}

func (r *consoleUserRepo) Create(ctx context.Context, u domain.ConsoleUser) error {
	_, err := r.col.InsertOne(ctx, u)
	if mongo.IsDuplicateKeyError(err) {
		return port.ErrUsernameTaken
	}
	return err
}

func (r *consoleUserRepo) ByUsername(ctx context.Context, username string) (domain.ConsoleUser, error) {
	return r.one(ctx, bson.M{"username": domain.NormalizeUsername(username)})
}

func (r *consoleUserRepo) ByID(ctx context.Context, id string) (domain.ConsoleUser, error) {
	return r.one(ctx, bson.M{"_id": id})
}

func (r *consoleUserRepo) one(ctx context.Context, filter bson.M) (domain.ConsoleUser, error) {
	var u domain.ConsoleUser
	err := r.col.FindOne(ctx, filter).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ConsoleUser{}, port.ErrUserNotFound
	}
	if err != nil {
		return domain.ConsoleUser{}, err
	}
	return u, nil
}

func (r *consoleUserRepo) List(ctx context.Context) ([]domain.ConsoleUser, error) {
	cur, err := r.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []domain.ConsoleUser{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *consoleUserRepo) Update(ctx context.Context, u domain.ConsoleUser) error {
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": u.ID}, u)
	if mongo.IsDuplicateKeyError(err) {
		return port.ErrUsernameTaken
	}
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return port.ErrUserNotFound
	}
	return nil
}

func (r *consoleUserRepo) Delete(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return port.ErrUserNotFound
	}
	return nil
}

func (r *consoleUserRepo) Count(ctx context.Context) (int, error) {
	n, err := r.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
