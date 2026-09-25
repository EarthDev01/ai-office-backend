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

func (r *chatRepo) TouchConversation(ctx context.Context, id string, at time.Time) error {
	_, err := r.convs.UpdateByID(ctx, id, bson.M{"$set": bson.M{"updated_at": at}})
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

func (r *chatRepo) CountAnswersSince(ctx context.Context, officeID, serviceID string, since time.Time) (int64, error) {
	return r.msgs.CountDocuments(ctx, bson.M{
		"office_id": officeID, "service_id": serviceID, "role": "assistant",
		"status": "ok", "created_at": bson.M{"$gte": since},
	})
}
