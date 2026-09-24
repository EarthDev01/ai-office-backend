package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// SecretPrefix ทำให้สแกนหา secret ที่หลุดใน log/repo ได้ง่าย (เช่น secret scanning)
const SecretPrefix = "aisk_"

// NewSecretKey สุ่ม secret_key 32 bytes (256 bit) — แสดงให้คนเห็นครั้งเดียวตอนออก/หมุน
func NewSecretKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return SecretPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// HashSecret — secret มี entropy สูง (256 bit) จึงใช้ SHA-256 ได้ ไม่ต้อง bcrypt
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

// SecretMatches เทียบแบบ constant-time กับทั้งใบปัจจุบันและใบเก่าช่วงหมุน (rotate 2 ใบ)
// คืน "current" | "prev" | ""
func (s *Service) SecretMatches(secret string) string {
	if secret == "" {
		return ""
	}
	h := HashSecret(secret)
	if s.SecretKeyHash != "" && subtle.ConstantTimeCompare([]byte(h), []byte(s.SecretKeyHash)) == 1 {
		return "current"
	}
	if s.SecretKeyPrevHash != "" && subtle.ConstantTimeCompare([]byte(h), []byte(s.SecretKeyPrevHash)) == 1 {
		return "prev"
	}
	return ""
}
