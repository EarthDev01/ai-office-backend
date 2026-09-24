package repository

import (
	"context"
	"errors"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// collection ของ ai_office (spec §7.5) — ทุกตัวมี office_id + service_id + created_at และ index นำด้วย 3 ตัวนี้
const (
	colSettings      = "settings"
	colConversations = "conversations"
	colMessages      = "messages"
	colVerifications = "verifications"
	colAccessLog     = "access_log"
	colDeletions     = "deletion_requests"
	colQuotaPeriods  = "quota_periods"
	colRollups       = "daily_rollups"
)

// ConversationTTL = 90 วัน (spec §7.5) · messages ไม่มี TTL จนกว่าจะมี ClickHouse (D-65)
const ConversationTTL = 90 * 24 * time.Hour

var tenantIndex = bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "created_at", Value: -1}}

// EnsureIndexes สร้าง index ครบทุก collection (idempotent)
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	type ix struct {
		col string
		m   []mongo.IndexModel
	}
	all := []ix{
		{colConversations, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
			{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "last_message_at", Value: -1}}, Options: options.Index().SetName("tenant_user_recent")},
			{Keys: bson.D{{Key: "opened_at", Value: 1}}, Options: options.Index().SetName("ttl_90d").SetExpireAfterSeconds(int32(ConversationTTL.Seconds()))},
		}},
		{colMessages, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
			{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: 1}}, Options: options.Index().SetName("conversation_created")},
			{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "verification_status", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("tenant_verification")},
			{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("tenant_user_created")},
		}},
		{colVerifications, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
			{Keys: bson.D{{Key: "message_id", Value: 1}}, Options: options.Index().SetName("uniq_message").SetUnique(true)},
		}},
		{colAccessLog, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
			{Keys: bson.D{{Key: "operator", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("operator_created")},
		}},
		{colDeletions, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
		}},
		{colQuotaPeriods, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
			{Keys: bson.D{{Key: "period", Value: 1}}, Options: options.Index().SetName("period")},
		}},
		{colRollups, []mongo.IndexModel{
			{Keys: tenantIndex, Options: options.Index().SetName("tenant_created")},
			{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "date", Value: 1}}, Options: options.Index().SetName("tenant_date")},
		}},
	}
	for _, x := range all {
		if _, err := db.Collection(x.col).Indexes().CreateMany(ctx, x.m); err != nil {
			return errors.New(x.col + ": " + err.Error())
		}
	}
	return nil
}

// MigrateOffices เติม field ใหม่ให้ office เดิม (idempotent)
//
// allow_all ที่ยังไม่มี = true — คงพฤติกรรมเดิมของโค้ดทีม (ทุกคนที่ผ่าน host ใช้ได้) · D-74
func MigrateOffices(ctx context.Context, db *mongo.Database) (int64, error) {
	res, err := db.Collection(officeCollection).UpdateMany(ctx,
		bson.M{"services": bson.M{"$elemMatch": bson.M{"allow_all": bson.M{"$exists": false}}}},
		bson.M{"$set": bson.M{"services.$[s].allow_all": true}},
		options.Update().SetArrayFilters(options.ArrayFilters{Filters: []any{bson.M{"s.allow_all": bson.M{"$exists": false}}}}),
	)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func notFound(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	return err
}

func paging(limit, offset int) *options.FindOptions {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return options.Find().SetLimit(int64(limit)).SetSkip(int64(offset))
}

func textRegex(s string) bson.M {
	return bson.M{"$regex": regexp.QuoteMeta(s), "$options": "i"}
}

// ---------- settings ----------

type SettingsRepo struct{ col *mongo.Collection }

func NewSettingsRepo(db *mongo.Database) *SettingsRepo {
	return &SettingsRepo{col: db.Collection(colSettings)}
}

func (r *SettingsRepo) Get(ctx context.Context) (domain.Settings, error) {
	var s domain.Settings
	err := r.col.FindOne(ctx, bson.M{"_id": "global"}).Decode(&s)
	return s, notFound(err)
}

func (r *SettingsRepo) Save(ctx context.Context, s domain.Settings) error {
	doc, err := toDoc(s)
	if err != nil {
		return err
	}
	doc["_id"] = "global"
	_, err = r.col.ReplaceOne(ctx, bson.M{"_id": "global"}, doc, options.Replace().SetUpsert(true))
	return err
}

func toDoc(v any) (bson.M, error) {
	b, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m bson.M
	err = bson.Unmarshal(b, &m)
	return m, err
}

// ---------- conversations ----------

type ConversationRepo struct{ col *mongo.Collection }

func NewConversationRepo(db *mongo.Database) *ConversationRepo {
	return &ConversationRepo{col: db.Collection(colConversations)}
}

func (r *ConversationRepo) Create(ctx context.Context, c domain.Conversation) error {
	_, err := r.col.InsertOne(ctx, c)
	return err
}

func (r *ConversationRepo) Get(ctx context.Context, officeID, serviceID, id string) (domain.Conversation, error) {
	var c domain.Conversation
	err := r.col.FindOne(ctx, bson.M{"_id": id, "office_id": officeID, "service_id": serviceID}).Decode(&c)
	return c, notFound(err)
}

func (r *ConversationRepo) GetAny(ctx context.Context, id string) (domain.Conversation, error) {
	var c domain.Conversation
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	return c, notFound(err)
}

func (r *ConversationRepo) Touch(ctx context.Context, officeID, serviceID, id string, at time.Time, title string) error {
	set := bson.M{"last_message_at": at}
	if title != "" {
		set["title"] = title
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "office_id": officeID, "service_id": serviceID},
		bson.M{"$set": set, "$inc": bson.M{"message_count": 2}})
	return err
}

func (r *ConversationRepo) Close(ctx context.Context, officeID, serviceID, id string, at time.Time, reason string) error {
	res, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "office_id": officeID, "service_id": serviceID, "closed_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"closed_at": at, "closed_reason": reason}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		if _, err := r.Get(ctx, officeID, serviceID, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *ConversationRepo) ListForUser(ctx context.Context, officeID, serviceID, userID string, since time.Time, limit int) ([]domain.Conversation, error) {
	opts := paging(limit, 0).SetSort(bson.D{{Key: "last_message_at", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"office_id": officeID, "service_id": serviceID, "user_id": userID, "last_message_at": bson.M{"$gte": since}}, opts)
	if err != nil {
		return nil, err
	}
	out := []domain.Conversation{}
	return out, cur.All(ctx, &out)
}

func (r *ConversationRepo) Search(ctx context.Context, f port.ConversationFilter) ([]domain.Conversation, int64, error) {
	q := bson.M{}
	if f.OfficeID != "" {
		q["office_id"] = f.OfficeID
	}
	if f.ServiceID != "" {
		q["service_id"] = f.ServiceID
	}
	if f.UserID != "" {
		q["$or"] = bson.A{bson.M{"user_id": f.UserID}, bson.M{"user_name": f.UserID}}
	}
	rng := bson.M{}
	if f.From != nil {
		rng["$gte"] = *f.From
	}
	if f.To != nil {
		rng["$lte"] = *f.To
	}
	if len(rng) > 0 {
		q["opened_at"] = rng
	}
	if f.Text != "" {
		q["title"] = textRegex(f.Text)
	}
	if f.IDs != nil {
		q["_id"] = bson.M{"$in": f.IDs}
	}
	total, err := r.col.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.col.Find(ctx, q, paging(f.Limit, f.Offset).SetSort(bson.D{{Key: "last_message_at", Value: -1}}))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.Conversation{}
	return out, total, cur.All(ctx, &out)
}

func scopeFilter(s domain.DeletionScope, timeField string) bson.M {
	q := bson.M{"office_id": s.OfficeID}
	if s.ServiceID != "" {
		q["service_id"] = s.ServiceID
	}
	if s.UserID != "" {
		q["user_id"] = s.UserID
	}
	rng := bson.M{}
	if s.From != nil {
		rng["$gte"] = *s.From
	}
	if s.To != nil {
		rng["$lte"] = *s.To
	}
	if len(rng) > 0 {
		q[timeField] = rng
	}
	return q
}

func (r *ConversationRepo) DeleteScope(ctx context.Context, s domain.DeletionScope) (int64, error) {
	if s.OfficeID == "" {
		return 0, errors.New("ต้องระบุ office")
	}
	res, err := r.col.DeleteMany(ctx, scopeFilter(s, "opened_at"))
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// ---------- messages (append-only · ไม่มี TTL) ----------

type MessageRepo struct{ col *mongo.Collection }

func NewMessageRepo(db *mongo.Database) *MessageRepo {
	return &MessageRepo{col: db.Collection(colMessages)}
}

func (r *MessageRepo) Insert(ctx context.Context, m domain.Message) error {
	_, err := r.col.InsertOne(ctx, m)
	return err
}

func (r *MessageRepo) ListByConversation(ctx context.Context, officeID, serviceID, conversationID string, limit int) ([]domain.Message, error) {
	if limit <= 0 {
		limit = 200
	}
	cur, err := r.col.Find(ctx, bson.M{"office_id": officeID, "service_id": serviceID, "conversation_id": conversationID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []domain.Message{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (r *MessageRepo) Get(ctx context.Context, id string) (domain.Message, error) {
	var m domain.Message
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&m)
	return m, notFound(err)
}

func (r *MessageRepo) Search(ctx context.Context, f port.MessageFilter) ([]domain.Message, int64, error) {
	q := bson.M{}
	for k, v := range map[string]string{"office_id": f.OfficeID, "service_id": f.ServiceID, "conversation_id": f.ConversationID, "role": f.Role, "verification_status": f.Verification} {
		if v != "" {
			q[k] = v
		}
	}
	if f.UserID != "" {
		q["$or"] = bson.A{bson.M{"user_id": f.UserID}, bson.M{"user_name": f.UserID}}
	}
	rng := bson.M{}
	if f.From != nil {
		rng["$gte"] = *f.From
	}
	if f.To != nil {
		rng["$lte"] = *f.To
	}
	if len(rng) > 0 {
		q["created_at"] = rng
	}
	if f.Text != "" {
		q["text"] = textRegex(f.Text)
	}
	total, err := r.col.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	order := -1
	if f.Ascending {
		order = 1
	}
	cur, err := r.col.Find(ctx, q, paging(f.Limit, f.Offset).SetSort(bson.D{{Key: "created_at", Value: order}}))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.Message{}
	return out, total, cur.All(ctx, &out)
}

func (r *MessageRepo) SetVerification(ctx context.Context, id, status string) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"verification_status": status}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *MessageRepo) CountByVerification(ctx context.Context, officeID, serviceID string) (map[string]int64, error) {
	match := bson.M{"role": domain.RoleAssistant}
	if officeID != "" {
		match["office_id"] = officeID
	}
	if serviceID != "" {
		match["service_id"] = serviceID
	}
	cur, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{"_id": "$verification_status", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID string `bson:"_id"`
		N  int64  `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.ID] = r.N
	}
	return out, nil
}

func (r *MessageRepo) DeleteScope(ctx context.Context, s domain.DeletionScope) (int64, error) {
	if s.OfficeID == "" {
		return 0, errors.New("ต้องระบุ office")
	}
	res, err := r.col.DeleteMany(ctx, scopeFilter(s, "created_at"))
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// MessageIDsInScope ใช้ลบ verifications ที่ผูกกับข้อความที่ถูกลบ
func (r *MessageRepo) MessageIDsInScope(ctx context.Context, s domain.DeletionScope) ([]string, error) {
	cur, err := r.col.Find(ctx, scopeFilter(s, "created_at"), options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID string `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out, nil
}

func (r *MessageRepo) Estimate(ctx context.Context) (int64, error) {
	return r.col.EstimatedDocumentCount(ctx)
}

// ---------- verifications ----------

type VerificationRepo struct{ col *mongo.Collection }

func NewVerificationRepo(db *mongo.Database) *VerificationRepo {
	return &VerificationRepo{col: db.Collection(colVerifications)}
}

func (r *VerificationRepo) Upsert(ctx context.Context, v domain.Verification) error {
	set := bson.M{
		"office_id": v.OfficeID, "service_id": v.ServiceID, "conversation_id": v.ConversationID,
		"status": v.Status, "error_type": v.ErrorType, "correct_answer": v.CorrectAnswer, "note": v.Note,
		"verified_by": v.VerifiedBy, "verified_at": v.VerifiedAt,
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"message_id": v.MessageID},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": v.ID, "created_at": v.CreatedAt}},
		options.Update().SetUpsert(true))
	return err
}

func (r *VerificationRepo) GetByMessage(ctx context.Context, messageID string) (domain.Verification, error) {
	var v domain.Verification
	err := r.col.FindOne(ctx, bson.M{"message_id": messageID}).Decode(&v)
	return v, notFound(err)
}

func (r *VerificationRepo) List(ctx context.Context, officeID, serviceID, status string, limit, offset int) ([]domain.Verification, int64, error) {
	q := bson.M{}
	if officeID != "" {
		q["office_id"] = officeID
	}
	if serviceID != "" {
		q["service_id"] = serviceID
	}
	if status != "" {
		q["status"] = status
	}
	total, err := r.col.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.col.Find(ctx, q, paging(limit, offset).SetSort(bson.D{{Key: "verified_at", Value: -1}}))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.Verification{}
	return out, total, cur.All(ctx, &out)
}

func (r *VerificationRepo) DeleteScope(ctx context.Context, s domain.DeletionScope, messageIDs []string) (int64, error) {
	if len(messageIDs) == 0 {
		return 0, nil
	}
	res, err := r.col.DeleteMany(ctx, bson.M{"office_id": s.OfficeID, "message_id": bson.M{"$in": messageIDs}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// ---------- access log ----------

type AccessLogRepo struct{ col *mongo.Collection }

func NewAccessLogRepo(db *mongo.Database) *AccessLogRepo {
	return &AccessLogRepo{col: db.Collection(colAccessLog)}
}

func (r *AccessLogRepo) Insert(ctx context.Context, l domain.AccessLog) error {
	_, err := r.col.InsertOne(ctx, l)
	return err
}

func (r *AccessLogRepo) List(ctx context.Context, officeID, serviceID, operator string, limit, offset int) ([]domain.AccessLog, int64, error) {
	q := bson.M{}
	if officeID != "" {
		q["office_id"] = officeID
	}
	if serviceID != "" {
		q["service_id"] = serviceID
	}
	if operator != "" {
		q["operator"] = operator
	}
	total, err := r.col.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.col.Find(ctx, q, paging(limit, offset).SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.AccessLog{}
	return out, total, cur.All(ctx, &out)
}

// ---------- deletion requests ----------

type DeletionRepo struct{ col *mongo.Collection }

func NewDeletionRepo(db *mongo.Database) *DeletionRepo {
	return &DeletionRepo{col: db.Collection(colDeletions)}
}

func (r *DeletionRepo) Insert(ctx context.Context, d domain.DeletionRequest) error {
	_, err := r.col.InsertOne(ctx, d)
	return err
}

func (r *DeletionRepo) Update(ctx context.Context, d domain.DeletionRequest) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": d.ID}, d)
	return err
}

func (r *DeletionRepo) List(ctx context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error) {
	total, err := r.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.col.Find(ctx, bson.M{}, paging(limit, offset).SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.DeletionRequest{}
	return out, total, cur.All(ctx, &out)
}

// ---------- quota periods ----------

type QuotaRepo struct{ col *mongo.Collection }

func NewQuotaRepo(db *mongo.Database) *QuotaRepo {
	return &QuotaRepo{col: db.Collection(colQuotaPeriods)}
}

func (r *QuotaRepo) Get(ctx context.Context, officeID, serviceID, period string) (domain.QuotaPeriod, error) {
	var p domain.QuotaPeriod
	err := r.col.FindOne(ctx, bson.M{"_id": domain.QuotaPeriodID(officeID, serviceID, period)}).Decode(&p)
	return p, notFound(err)
}

func (r *QuotaRepo) upsertBase(officeID, serviceID, period string) bson.M {
	return bson.M{"office_id": officeID, "service_id": serviceID, "period": period, "created_at": time.Now()}
}

func (r *QuotaRepo) AddUsage(ctx context.Context, officeID, serviceID, period string, tokens, questions int64, cost float64) (domain.QuotaPeriod, error) {
	var p domain.QuotaPeriod
	err := r.col.FindOneAndUpdate(ctx, bson.M{"_id": domain.QuotaPeriodID(officeID, serviceID, period)},
		bson.M{
			"$inc":         bson.M{"used_tokens": tokens, "used_questions": questions, "cost_amount": cost},
			"$set":         bson.M{"updated_at": time.Now()},
			"$setOnInsert": r.upsertBase(officeID, serviceID, period),
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&p)
	return p, err
}

func (r *QuotaRepo) MarkAlert(ctx context.Context, officeID, serviceID, period, flag string) (bool, error) {
	id := domain.QuotaPeriodID(officeID, serviceID, period)
	var filter, set bson.M
	switch flag {
	case "alerted_80", "alerted_95":
		filter = bson.M{"_id": id, flag: bson.M{"$ne": true}}
		set = bson.M{flag: true}
	case "cut":
		filter = bson.M{"_id": id, "cut_at": bson.M{"$exists": false}}
		set = bson.M{"cut_at": time.Now()}
	default:
		return false, nil
	}
	res, err := r.col.UpdateOne(ctx, filter, bson.M{"$set": set})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

func (r *QuotaRepo) List(ctx context.Context, period string) ([]domain.QuotaPeriod, error) {
	q := bson.M{}
	if period != "" {
		q["period"] = period
	}
	cur, err := r.col.Find(ctx, q)
	if err != nil {
		return nil, err
	}
	out := []domain.QuotaPeriod{}
	return out, cur.All(ctx, &out)
}

// ---------- daily rollups ----------

type RollupRepo struct{ col *mongo.Collection }

func NewRollupRepo(db *mongo.Database) *RollupRepo {
	return &RollupRepo{col: db.Collection(colRollups)}
}

func (r *RollupRepo) Add(ctx context.Context, officeID, serviceID, date string, d domain.RollupDelta) error {
	inc := bson.M{
		"conversations": d.Conversations, "questions": d.Questions, "tokens_in": d.TokensIn, "tokens_out": d.TokensOut,
		"cost_amount": d.CostAmount, "refusals": d.Refusals, "tool_errors": d.ToolErrors, "guard_hits": d.GuardHits,
		"correct": d.Correct, "wrong": d.Wrong,
	}
	if d.LatencyMs > 0 {
		inc["latency_sum_ms"] = d.LatencyMs
		inc["latency_buckets."+domain.LatencyBucket(d.LatencyMs)] = 1
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": officeID + "|" + serviceID + "|" + date},
		bson.M{
			"$inc":         inc,
			"$set":         bson.M{"updated_at": time.Now()},
			"$setOnInsert": bson.M{"office_id": officeID, "service_id": serviceID, "date": date, "created_at": time.Now()},
		}, options.Update().SetUpsert(true))
	return err
}

func (r *RollupRepo) List(ctx context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error) {
	q := bson.M{}
	if officeID != "" {
		q["office_id"] = officeID
	}
	if serviceID != "" {
		q["service_id"] = serviceID
	}
	rng := bson.M{}
	if from != "" {
		rng["$gte"] = from
	}
	if to != "" {
		rng["$lte"] = to
	}
	if len(rng) > 0 {
		q["date"] = rng
	}
	cur, err := r.col.Find(ctx, q, options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []domain.DailyRollup{}
	return out, cur.All(ctx, &out)
}
