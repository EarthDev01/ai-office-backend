package repository

import (
	"context"
	"errors"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---- settings ----

type settingsRepo struct{ col *mongo.Collection }

func NewSettingsRepository(db *mongo.Database) port.SettingsRepository {
	return &settingsRepo{col: db.Collection("settings")}
}

const settingsID = "global"

func (r *settingsRepo) Get(ctx context.Context) (domain.Settings, error) {
	var s domain.Settings
	err := r.col.FindOne(ctx, bson.M{"_id": settingsID}).Decode(&s)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return s, domain.ErrNotFound
	}
	return s, err
}

func (r *settingsRepo) Save(ctx context.Context, s domain.Settings) error {
	doc, err := bson.Marshal(s)
	if err != nil {
		return err
	}
	var m bson.M
	if err := bson.Unmarshal(doc, &m); err != nil {
		return err
	}
	_, err = r.col.ReplaceOne(ctx, bson.M{"_id": settingsID}, m, options.Replace().SetUpsert(true))
	return err
}

// ---- usage ----

type usageRepo struct {
	periods *mongo.Collection
	rollups *mongo.Collection
}

func NewUsageRepository(db *mongo.Database) port.UsageRepository {
	return &usageRepo{periods: db.Collection("usage_periods"), rollups: db.Collection("daily_rollups")}
}

func incOf(d domain.RollupDelta) bson.M {
	inc := bson.M{}
	for k, v := range map[string]int64{
		"conversations": d.Conversations, "questions": d.Questions,
		"input_tokens": d.InputTokens, "output_tokens": d.OutputTokens, "cache_read": d.CacheRead, "cache_write": d.CacheWrite,
		"refusals": d.Refusals, "tool_errors": d.ToolErrors, "guard_hits": d.GuardHits, "correct": d.Correct, "wrong": d.Wrong,
	} {
		if v != 0 {
			inc[k] = v
		}
	}
	return inc
}

func (r *usageRepo) AddPeriod(ctx context.Context, officeID, serviceID, period string, d domain.RollupDelta, at time.Time) error {
	inc := incOf(d)
	delete(inc, "conversations")
	for _, k := range []string{"refusals", "tool_errors", "guard_hits", "correct", "wrong"} {
		delete(inc, k)
	}
	update := bson.M{
		"$set":         bson.M{"updated_at": at},
		"$setOnInsert": bson.M{"office_id": officeID, "service_id": serviceID, "period": period},
	}
	if len(inc) > 0 {
		update["$inc"] = inc
	}
	_, err := r.periods.UpdateByID(ctx, officeID+"|"+serviceID+"|"+period, update, options.Update().SetUpsert(true))
	return err
}

func (r *usageRepo) AddRollup(ctx context.Context, officeID, serviceID, date string, d domain.RollupDelta) error {
	inc := incOf(d)
	if len(inc) == 0 {
		return nil
	}
	_, err := r.rollups.UpdateByID(ctx, officeID+"|"+serviceID+"|"+date, bson.M{
		"$inc":         inc,
		"$setOnInsert": bson.M{"office_id": officeID, "service_id": serviceID, "date": date},
	}, options.Update().SetUpsert(true))
	return err
}

func (r *usageRepo) ListPeriods(ctx context.Context, period string) ([]domain.UsagePeriod, error) {
	cur, err := r.periods.Find(ctx, bson.M{"period": period}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []domain.UsagePeriod{}
	return out, cur.All(ctx, &out)
}

func (r *usageRepo) ListRollups(ctx context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error) {
	filter := bson.M{"date": bson.M{"$gte": from, "$lte": to}}
	if officeID != "" {
		filter["office_id"] = officeID
	}
	if serviceID != "" {
		filter["service_id"] = serviceID
	}
	cur, err := r.rollups.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "date", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []domain.DailyRollup{}
	return out, cur.All(ctx, &out)
}

// ---- deletion_requests ----

type deletionRepo struct{ col *mongo.Collection }

func NewDeletionRepository(ctx context.Context, db *mongo.Database) (port.DeletionRepository, error) {
	col := db.Collection("deletion_requests")
	if _, err := col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: -1}}, Options: options.Index().SetName("created_desc"),
	}); err != nil {
		return nil, err
	}
	return &deletionRepo{col: col}, nil
}

func (r *deletionRepo) Create(ctx context.Context, d domain.DeletionRequest) error {
	_, err := r.col.InsertOne(ctx, d)
	return err
}

func (r *deletionRepo) Update(ctx context.Context, d domain.DeletionRequest) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": d.ID}, d)
	return err
}

func (r *deletionRepo) List(ctx context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error) {
	total, err := r.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}
	limit, offset = port.NormalizePage(limit, offset)
	cur, err := r.col.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(limit)).SetSkip(int64(offset)))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.DeletionRequest{}
	return out, total, cur.All(ctx, &out)
}

func (r *deletionRepo) FailRunning(ctx context.Context, reason string, at time.Time) (int64, error) {
	res, err := r.col.UpdateMany(ctx, bson.M{"status": "running"},
		bson.M{"$set": bson.M{"status": "failed", "error": reason, "completed_at": at}})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}
