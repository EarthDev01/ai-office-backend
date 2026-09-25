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

type officeGroupRepo struct{ col *mongo.Collection }

func NewOfficeGroupRepository(db *mongo.Database) port.OfficeGroupRepository {
	return &officeGroupRepo{col: db.Collection("office_groups")}
}

func (r *officeGroupRepo) List(ctx context.Context) ([]domain.OfficeGroup, error) {
	cur, err := r.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []domain.OfficeGroup{}
	return out, cur.All(ctx, &out)
}

func (r *officeGroupRepo) Get(ctx context.Context, id string) (domain.OfficeGroup, error) {
	var g domain.OfficeGroup
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&g)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return g, domain.ErrNotFound
	}
	return g, err
}

func (r *officeGroupRepo) Save(ctx context.Context, g domain.OfficeGroup) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": g.ID}, g, options.Replace().SetUpsert(true))
	return err
}

func (r *officeGroupRepo) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
