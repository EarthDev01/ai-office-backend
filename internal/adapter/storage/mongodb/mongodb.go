// Package mongodb ถือ connection ของ MongoDB กลางของระบบ AI
//
// ██ ห้ามเอา URI หรือรหัสผ่านไป log / commit / เขียนลงไฟล์อื่นเด็ดขาด
// ██ ค่าจริงอยู่ใน .env ซึ่ง .gitignore ไว้แล้ว
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Resource struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func New(ctx context.Context, uri, dbName string) (*Resource, error) {
	if uri == "" {
		return nil, fmt.Errorf("DB_URI ว่าง")
	}
	if dbName == "" {
		return nil, fmt.Errorf("DB_NAME ว่าง")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetAppName("AI-OFFICE"))
	if err != nil {
		// ไม่ wrap err ดิบ เพราะ driver ชอบแปะ URI (มีรหัสผ่าน) มาในข้อความ
		return nil, fmt.Errorf("เชื่อมต่อ MongoDB ไม่สำเร็จ")
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping MongoDB ไม่สำเร็จ — ตรวจ DB_URI, network access list และรหัสผ่าน")
	}

	return &Resource{Client: client, DB: client.Database(dbName)}, nil
}

func (r *Resource) Close() {
	if r.Client != nil {
		_ = r.Client.Disconnect(context.Background())
	}
}
