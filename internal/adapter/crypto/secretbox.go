// Package crypto เข้ารหัสข้อมูลลับก่อนเก็บลง DB ด้วยกุญแจหลักจาก .env
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"ai-office-backend/internal/core/port"
)

// AESGCM — AES-256-GCM · กุญแจหลัก ██ secret ห้าม log
type AESGCM struct {
	aead    cipher.AEAD
	version int
}

var _ port.SecretBox = (*AESGCM)(nil)

// NewAESGCM รับกุญแจหลักแบบ base64 ยาว 32 byte (สร้างด้วย `openssl rand -base64 32`)
func NewAESGCM(secretB64 string, version int) (*AESGCM, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(secretB64))
	if err != nil {
		return nil, errors.New("LLM_KEY_SECRET ต้องเป็น base64 — สร้างด้วย: openssl rand -base64 32")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("LLM_KEY_SECRET ต้องยาว 32 byte หลังถอด base64 (ได้ %d) — สร้างด้วย: openssl rand -base64 32", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCM{aead: aead, version: version}, nil
}

func (b *AESGCM) Seal(plaintext, aad []byte) ([]byte, []byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, b.aead.Seal(nil, nonce, plaintext, aad), nil
}

func (b *AESGCM) Open(nonce, ciphertext, aad []byte) ([]byte, error) {
	if len(nonce) != b.aead.NonceSize() {
		return nil, errors.New("nonce ไม่ถูกต้อง")
	}
	return b.aead.Open(nil, nonce, ciphertext, aad)
}

func (b *AESGCM) Version() int { return b.version }
