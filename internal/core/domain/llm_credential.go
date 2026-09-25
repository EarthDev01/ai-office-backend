package domain

import (
	"fmt"
	"strings"
	"time"
)

// LLMCredential คือ API key ของ provider 1 ตัว เก็บแบบเข้ารหัส (AES-256-GCM) — ตัว key จริงไม่อยู่ใน DB
//
// ห้ามส่ง struct นี้ออก API — ใช้ LLMKeyInfo แทน (json tag มีไว้ให้ file store เก็บลงไฟล์ได้เท่านั้น)
type LLMCredential struct {
	Provider   string    `json:"provider"    bson:"_id"`
	Ciphertext []byte    `json:"ciphertext"  bson:"ciphertext"`
	Nonce      []byte    `json:"nonce"       bson:"nonce"`
	KeyVersion int       `json:"key_version" bson:"key_version"` // รุ่นของกุญแจหลัก — ไว้หมุนกุญแจภายหลัง
	Last4      string    `json:"last4"       bson:"last4"`
	UpdatedAt  time.Time `json:"updated_at"  bson:"updated_at"`
	UpdatedBy  string    `json:"updated_by"  bson:"updated_by"`
}

// LLMKeyInfo คือสิ่งเดียวที่หน้าเว็บเห็นเกี่ยวกับ key — ไม่มีตัว key
type LLMKeyInfo struct {
	HasKey    bool      `json:"has_key"`
	Last4     string    `json:"last4,omitempty"`
	Source    string    `json:"source,omitempty"` // db | env (env = ยังไม่ได้ตั้ง LLM_KEY_SECRET)
	Broken    bool      `json:"broken,omitempty"` // มีใน DB แต่ถอดรหัสไม่ได้ (กุญแจหลักเปลี่ยน)
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	UpdatedBy string    `json:"updated_by,omitempty"`
}

// KeyLast4 คืน 4 ตัวท้ายไว้แสดง/ลงประวัติ — key สั้นผิดปกติไม่โชว์อะไรเลย
func KeyLast4(key string) string {
	if len(key) < 12 {
		return ""
	}
	return key[len(key)-4:]
}

// MaskKey = "••••a1b2" สำหรับข้อความที่คนอ่าน
func MaskKey(last4 string) string {
	if last4 == "" {
		return "••••"
	}
	return "••••" + last4
}

// ValidateAPIKey ตรวจรูปแบบคร่าว ๆ ก่อนลองยิงจริง
func ValidateAPIKey(key string) error {
	if len(key) < 12 || len(key) > 500 {
		return fmt.Errorf("API key ต้องยาว 12–500 ตัวอักษร")
	}
	if strings.ContainsAny(key, " \t\r\n") {
		return fmt.Errorf("API key ต้องไม่มีช่องว่างหรือขึ้นบรรทัดใหม่")
	}
	return nil
}
