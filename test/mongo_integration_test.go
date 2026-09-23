package test

import (
	"context"
	"os"
	"testing"
	"time"

	mongodb "ai-office-backend/internal/adapter/storage/mongodb"
	"ai-office-backend/internal/adapter/storage/mongodb/repository"
	"ai-office-backend/internal/core/domain"
)

// ทดสอบกับ MongoDB จริง — ข้ามถ้าไม่ได้ตั้ง env ไว้
//
//	AI_OFFICE_TEST_MONGO_URI=mongodb+srv://... go test ./test/ -run TestMongo -v
//
// ใช้ database แยกชื่อ ai_office_test เพื่อไม่ไปแตะของจริง
func TestMongoOfficeRepository(t *testing.T) {
	uri := os.Getenv("AI_OFFICE_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("ข้าม — ตั้ง AI_OFFICE_TEST_MONGO_URI ก่อนถึงจะรัน")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := mongodb.New(ctx, uri, "ai_office_test")
	if err != nil {
		t.Fatalf("เชื่อมต่อไม่ได้: %v", err)
	}
	defer res.Close()

	_ = res.DB.Collection("ai_offices").Drop(ctx)

	repo, err := repository.NewOfficeRepository(ctx, res.DB)
	if err != nil {
		t.Fatalf("สร้าง repo ไม่ได้: %v", err)
	}

	o := demoOffice()
	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Get(ctx, o.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PublicKey != o.PublicKey || len(got.Services) != 2 {
		t.Fatalf("อ่านกลับมาไม่ตรง: %+v", got)
	}
	if got.Services[0].ID != "K11S" || !got.Services[0].Enabled {
		t.Fatalf("service ฝังใน document ไม่ตรง: %+v", got.Services)
	}

	// public_key ต้องหาเจอ — เป็นเส้นทางที่ snippet ใช้
	byKey, err := repo.GetByPublicKey(ctx, o.PublicKey)
	if err != nil || byKey.ID != o.ID {
		t.Fatalf("หา by public key ไม่เจอ: %v %+v", err, byKey)
	}

	// key ซ้ำต้องถูกกัน ไม่งั้น key เดียวชี้ได้หลาย office
	dup := domain.NewOffice("acme", "ACME")
	dup.PublicKey = o.PublicKey
	if err := repo.Save(ctx, dup); err != domain.ErrConflict {
		t.Fatalf("public_key ซ้ำต้องถูกปฏิเสธ ได้ %v", err)
	}

	origins, err := repo.AllOrigins(ctx)
	if err != nil || len(origins) != 1 || origins[0] != officeOrigin {
		t.Fatalf("AllOrigins ผิด: %v %+v", err, origins)
	}

	if err := repo.Delete(ctx, o.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, o.ID); err != domain.ErrNotFound {
		t.Fatalf("ลบแล้วต้องหาไม่เจอ ได้ %v", err)
	}
	if _, err := repo.GetByPublicKey(ctx, o.PublicKey); err != domain.ErrNotFound {
		t.Fatalf("ลบแล้ว key ต้องใช้ไม่ได้ ได้ %v", err)
	}
}
