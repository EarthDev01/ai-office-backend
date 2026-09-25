package repository

import (
	"context"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// llmCredentialRepo — collection llm_credentials · 1 doc ต่อ provider (_id = provider) เก็บแต่ ciphertext
type llmCredentialRepo struct{ col *mongo.Collection }

func NewLLMCredentialRepository(db *mongo.Database) port.LLMCredentialRepository {
	return &llmCredentialRepo{col: db.Collection("llm_credentials")}
}

func (r *llmCredentialRepo) List(ctx context.Context) ([]domain.LLMCredential, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var out []domain.LLMCredential
	return out, cur.All(ctx, &out)
}

func (r *llmCredentialRepo) Save(ctx context.Context, c domain.LLMCredential) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": c.Provider}, c, options.Replace().SetUpsert(true))
	return err
}

func (r *llmCredentialRepo) Delete(ctx context.Context, provider string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": provider})
	return err
}
