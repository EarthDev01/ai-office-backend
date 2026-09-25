package repository

import (
	"context"
	"errors"
	"regexp"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	conversationCollection = "conversations"
	messageCollection      = "messages"
)

type chatRepo struct {
	convs *mongo.Collection
	msgs  *mongo.Collection
}

// NewChatRepository สร้าง index ตามการใช้จริง: โหลดประวัติของห้อง และนับคำตอบรายเดือนต่อ service
func NewChatRepository(ctx context.Context, db *mongo.Database) (port.ChatRepository, error) {
	r := &chatRepo{convs: db.Collection(conversationCollection), msgs: db.Collection(messageCollection)}
	if _, err := r.convs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "updated_at", Value: -1}}, Options: options.Index().SetName("office_service_updated")},
		{Keys: bson.D{{Key: "admin_id", Value: 1}, {Key: "updated_at", Value: -1}}, Options: options.Index().SetName("admin_updated")},
	}); err != nil {
		return nil, err
	}
	if _, err := r.msgs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// ทิศเดียวกับ index ที่มีอยู่แล้วใน DB ชุดนี้ (ชื่อซ้ำแต่ทิศต่าง = สร้างไม่ได้) · เรียงใหม่→เก่าก็ยังใช้ index นี้ได้
		{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: 1}}, Options: options.Index().SetName("conversation_created")},
		{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "role", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("office_service_role_created")},
		// คิวตรวจคำตอบ — ชื่อ/ทิศเดียวกับ index ที่มีอยู่แล้วใน DB ชุดนี้
		{Keys: bson.D{{Key: "office_id", Value: 1}, {Key: "service_id", Value: 1}, {Key: "verification_status", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("tenant_verification")},
	}); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *chatRepo) CreateConversation(ctx context.Context, c domain.Conversation) error {
	_, err := r.convs.InsertOne(ctx, c)
	return err
}

func (r *chatRepo) GetConversation(ctx context.Context, id string) (domain.Conversation, error) {
	var c domain.Conversation
	err := r.convs.FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return c, domain.ErrNotFound
	}
	return c, err
}

func (r *chatRepo) TouchConversation(ctx context.Context, id string, at time.Time, addMessages int) error {
	_, err := r.convs.UpdateByID(ctx, id, bson.M{"$set": bson.M{"updated_at": at}, "$inc": bson.M{"message_count": addMessages}})
	return err
}

func (r *chatRepo) AppendMessage(ctx context.Context, m domain.ChatMessage) error {
	_, err := r.msgs.InsertOne(ctx, m)
	return err
}

func (r *chatRepo) RecentMessages(ctx context.Context, conversationID string, limit int) ([]domain.ChatMessage, error) {
	cur, err := r.msgs.Find(ctx, bson.M{"conversation_id": conversationID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	var out []domain.ChatMessage
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 { // ใหม่→เก่า เป็น เก่า→ใหม่
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func timeRange(field string, from, to *time.Time, filter bson.M) {
	if from == nil && to == nil {
		return
	}
	r := bson.M{}
	if from != nil {
		r["$gte"] = *from
	}
	if to != nil {
		r["$lte"] = *to
	}
	filter[field] = r
}

func (r *chatRepo) SearchConversations(ctx context.Context, f port.ConversationFilter) ([]domain.Conversation, int64, error) {
	filter := bson.M{}
	if f.OfficeID != "" {
		filter["office_id"] = f.OfficeID
	}
	if f.ServiceID != "" {
		filter["service_id"] = f.ServiceID
	}
	if f.User != "" {
		filter["$or"] = bson.A{bson.M{"admin_id": f.User}, bson.M{"username": f.User}}
	}
	if f.Text != "" {
		filter["title"] = bson.M{"$regex": regexp.QuoteMeta(f.Text), "$options": "i"}
	}
	if f.IDs != nil {
		filter["_id"] = bson.M{"$in": f.IDs}
	}
	timeRange("created_at", f.From, f.To, filter)

	total, err := r.convs.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	limit, offset := port.NormalizePage(f.Limit, f.Offset)
	cur, err := r.convs.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(int64(limit)).SetSkip(int64(offset)))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.Conversation{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *chatRepo) GetMessage(ctx context.Context, id string) (domain.ChatMessage, error) {
	var m domain.ChatMessage
	err := r.msgs.FindOne(ctx, bson.M{"_id": id}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return m, domain.ErrNotFound
	}
	return m, err
}

func (r *chatRepo) SearchMessages(ctx context.Context, f port.MessageFilter) ([]domain.ChatMessage, int64, error) {
	filter := bson.M{}
	if f.OfficeID != "" {
		filter["office_id"] = f.OfficeID
	}
	if f.ServiceID != "" {
		filter["service_id"] = f.ServiceID
	}
	if f.Role != "" {
		filter["role"] = f.Role
	}
	if f.Verification != "" {
		filter["verification_status"] = f.Verification
	}
	timeRange("created_at", f.From, f.To, filter)

	total, err := r.msgs.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	dir := -1
	if f.Ascending {
		dir = 1
	}
	limit, offset := port.NormalizePage(f.Limit, f.Offset)
	cur, err := r.msgs.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: dir}}).SetLimit(int64(limit)).SetSkip(int64(offset)))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.ChatMessage{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *chatRepo) SetVerification(ctx context.Context, messageID, status string) error {
	res, err := r.msgs.UpdateByID(ctx, messageID, bson.M{"$set": bson.M{"verification_status": status}})
	if err == nil && res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *chatRepo) CountByVerification(ctx context.Context, officeID, serviceID string) (map[string]int64, error) {
	match := bson.M{"role": "assistant", "verification_status": bson.M{"$in": bson.A{domain.VerifyPending, domain.VerifyCorrect, domain.VerifyWrong}}}
	if officeID != "" {
		match["office_id"] = officeID
	}
	if serviceID != "" {
		match["service_id"] = serviceID
	}
	cur, err := r.msgs.Aggregate(ctx, mongo.Pipeline{
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
	for _, row := range rows {
		out[row.ID] = row.N
	}
	return out, nil
}

// ---- verifications ----

type verificationRepo struct{ col *mongo.Collection }

func NewVerificationRepository(ctx context.Context, db *mongo.Database) (port.VerificationRepository, error) {
	col := db.Collection("verifications")
	// ชื่อ index เดียวกับที่มีอยู่แล้วใน DB ชุดนี้
	if _, err := col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "message_id", Value: 1}}, Options: options.Index().SetName("uniq_message").SetUnique(true),
	}); err != nil {
		return nil, err
	}
	return &verificationRepo{col: col}, nil
}

// Upsert ทับผลเดิมของข้อความเดียวกัน แต่คง _id และ created_at ของครั้งแรกไว้
func (r *verificationRepo) Upsert(ctx context.Context, v domain.Verification) error {
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

func (r *verificationRepo) GetByMessage(ctx context.Context, messageID string) (domain.Verification, error) {
	var v domain.Verification
	err := r.col.FindOne(ctx, bson.M{"message_id": messageID}).Decode(&v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return v, domain.ErrNotFound
	}
	return v, err
}

// ---- access_log ----

type accessLogRepo struct{ col *mongo.Collection }

// NewAccessLogRepository — ไม่ตั้ง TTL: บันทึกการเข้าถึงต้องอยู่จนกว่าจะมีนโยบายเก็บที่ชัดเจน
func NewAccessLogRepository(ctx context.Context, db *mongo.Database) (port.AccessLogRepository, error) {
	col := db.Collection("access_log")
	if _, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "at", Value: -1}}, Options: options.Index().SetName("at_desc")},
		{Keys: bson.D{{Key: "operator", Value: 1}, {Key: "at", Value: -1}}, Options: options.Index().SetName("operator_at")},
	}); err != nil {
		return nil, err
	}
	return &accessLogRepo{col: col}, nil
}

func (r *accessLogRepo) Insert(ctx context.Context, e domain.AccessLog) error {
	_, err := r.col.InsertOne(ctx, e)
	return err
}

// ---- ลบตามคำขอ ----

// convFilter — ห้องในขอบเขต office/service/ผู้ใช้ (ไม่ดูเวลา)
func convFilter(s domain.DeletionScope) bson.M {
	f := bson.M{"office_id": s.OfficeID}
	if s.ServiceID != "" {
		f["service_id"] = s.ServiceID
	}
	if s.User != "" {
		f["$or"] = bson.A{bson.M{"admin_id": s.User}, bson.M{"username": s.User}}
	}
	return f
}

// msgFilter — ข้อความไม่ได้เก็บผู้ใช้ไว้ ถ้าระบุผู้ใช้จึงหาจากห้องของผู้ใช้ก่อน
func (r *chatRepo) msgFilter(ctx context.Context, s domain.DeletionScope) (bson.M, error) {
	f := bson.M{"office_id": s.OfficeID}
	if s.ServiceID != "" {
		f["service_id"] = s.ServiceID
	}
	if s.User != "" {
		ids, err := r.convs.Distinct(ctx, "_id", convFilter(s))
		if err != nil {
			return nil, err
		}
		f["conversation_id"] = bson.M{"$in": ids}
	}
	timeRange("created_at", s.From, s.To, f)
	return f, nil
}

func (r *chatRepo) MessageIDsInScope(ctx context.Context, s domain.DeletionScope) ([]string, error) {
	f, err := r.msgFilter(ctx, s)
	if err != nil {
		return nil, err
	}
	raw, err := r.msgs.Distinct(ctx, "_id", f)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if id, ok := v.(string); ok {
			out = append(out, id)
		}
	}
	return out, nil
}

func (r *chatRepo) DeleteMessagesInScope(ctx context.Context, s domain.DeletionScope) (int64, error) {
	f, err := r.msgFilter(ctx, s)
	if err != nil {
		return 0, err
	}
	res, err := r.msgs.DeleteMany(ctx, f)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (r *chatRepo) DeleteEmptyConversations(ctx context.Context, s domain.DeletionScope) (int64, error) {
	ids, err := r.convs.Distinct(ctx, "_id", convFilter(s))
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	used, err := r.msgs.Distinct(ctx, "conversation_id", bson.M{"conversation_id": bson.M{"$in": ids}})
	if err != nil {
		return 0, err
	}
	res, err := r.convs.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": ids, "$nin": used}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (r *verificationRepo) DeleteByMessages(ctx context.Context, messageIDs []string) (int64, error) {
	if len(messageIDs) == 0 {
		return 0, nil
	}
	res, err := r.col.DeleteMany(ctx, bson.M{"message_id": bson.M{"$in": messageIDs}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (r *accessLogRepo) List(ctx context.Context, f port.AccessLogFilter) ([]domain.AccessLog, int64, error) {
	filter := bson.M{}
	if f.OfficeID != "" {
		filter["office_id"] = f.OfficeID
	}
	if f.ServiceID != "" {
		filter["service_id"] = f.ServiceID
	}
	if f.Operator != "" {
		filter["operator"] = f.Operator
	}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	limit, offset := port.NormalizePage(f.Limit, f.Offset)
	cur, err := r.col.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "at", Value: -1}}).SetLimit(int64(limit)).SetSkip(int64(offset)))
	if err != nil {
		return nil, 0, err
	}
	out := []domain.AccessLog{}
	return out, total, cur.All(ctx, &out)
}
