package domain

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// Office คือ "การติดตั้ง 1 ชุด" — หลังบ้าน 1 ระบบที่เอา script ไปแปะ
//
// 1 office มีได้หลาย service (แบรนด์/เว็บย่อย) ที่แอดมินสลับไปมาในหน้าเดิม
// จึงต้องแยก 2 ชั้น:
//
//	public_key  อยู่ใน snippet  → บอกว่าหน้านี้เป็นของ office ไหน
//	service     มาจาก session   → บอกว่าตอนนี้กำลังดู service ไหน (เปลี่ยนได้ตลอดโดยไม่ต้องแก้ snippet)
type Office struct {
	ID               string    `json:"id"                 bson:"_id"`
	Label            string    `json:"label"              bson:"label"`
	PublicKey        string    `json:"public_key"         bson:"public_key"`
	AllowedOrigins   []string  `json:"allowed_origins"    bson:"allowed_origins"`
	BackofficeAPIURL string    `json:"backoffice_api_url" bson:"backoffice_api_url"`
	Enabled          bool      `json:"enabled"            bson:"enabled"`
	IsHidden         bool      `json:"is_hidden"          bson:"is_hidden"` // โหลด widget แต่ไม่โชว์ปุ่มลอย ให้ office เรียกเปิดเอง
	Theme            string    `json:"theme"              bson:"theme"`
	Placement        Placement `json:"placement"          bson:"placement"`
	Services         []Service `json:"services"           bson:"services"`
	CreatedAt        time.Time `json:"created_at"         bson:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"         bson:"updated_at"`
	UpdatedBy        string    `json:"updated_by"         bson:"updated_by"`
}

// Service คือแบรนด์/เว็บย่อยใต้ office หนึ่ง
//
// ค่าที่อยู่ตรงนี้คือค่าที่ "ต่างกันได้ระหว่างแบรนด์"
// ส่วน theme/placement อยู่ที่ระดับ office เพราะเป็นเรื่องหน้าตาของหลังบ้านชุดเดียวกัน
// ถ้าให้ต่างกันรายแบรนด์ ปุ่มจะเด้งไปมาตอนแอดมินสลับ service
type Service struct {
	ID          string   `json:"id"           bson:"id"`
	Label       string   `json:"label"        bson:"label"`
	Enabled     bool     `json:"enabled"      bson:"enabled"`
	Allowlist   []string `json:"allowlist"    bson:"allowlist"`
	DisplayName string   `json:"display_name" bson:"display_name"`
	Greeting    string   `json:"greeting"     bson:"greeting"`
	AvatarURL   string   `json:"avatar_url"   bson:"avatar_url"`
}

func (o *Office) FindService(id string) (Service, bool) {
	for _, s := range o.Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}

// AllowsOrigin — Origin ว่างถือว่าเป็น same-origin (เบราว์เซอร์ไม่ส่ง header นี้มา)
func (o *Office) AllowsOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	for _, a := range o.AllowedOrigins {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

func NewPublicKey() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "pk_" + hex.EncodeToString(b)
}

func DefaultService(id, label string) Service {
	return Service{
		ID:          id,
		Label:       label,
		Enabled:     false,
		Allowlist:   []string{},
		DisplayName: "ผู้ช่วยหลังบ้าน",
		Greeting:    "สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",
	}
}

func NewOffice(id, label string) Office {
	now := time.Now()
	return Office{
		ID:             id,
		Label:          label,
		PublicKey:      NewPublicKey(),
		AllowedOrigins: []string{},
		Enabled:        true,
		Theme:          "auto",
		Placement:      Placement{Position: "bottom-right", OffsetX: 12, OffsetY: 12},
		Services:       []Service{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
