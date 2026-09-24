package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// unverifiedExp อ่าน exp ของ grant (JWT ของ host) โดยไม่ตรวจลายเซ็น
//
// ใช้แค่ "ไม่ให้ตั๋วอยู่นานกว่า grant" — ไม่ใช่การตัดสินความน่าเชื่อ (host ตรวจลายเซ็นเองตอนเราขอกุญแจดอกเล็ก)
func unverifiedExp(jwt string) (time.Time, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var c struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(b, &c) != nil || c.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(c.Exp, 0), true
}
