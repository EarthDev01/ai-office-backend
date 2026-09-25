package port

import (
	"context"

	"ai-office-backend/internal/core/domain"
)

// LLMCredentialRepository เก็บ key ที่เข้ารหัสแล้ว (collection llm_credentials) — แยกจาก settings เพื่อไม่ให้ diff/export ของ settings แตะ key
type LLMCredentialRepository interface {
	List(ctx context.Context) ([]domain.LLMCredential, error)
	Save(ctx context.Context, c domain.LLMCredential) error
	Delete(ctx context.Context, provider string) error
}

// SecretBox เข้า/ถอดรหัสแบบ authenticated · aad ผูกรหัสไว้กับบริบท (เช่นชื่อ provider) ย้ายไปใช้ที่อื่นแล้วถอดไม่ออก
type SecretBox interface {
	Seal(plaintext, aad []byte) (nonce, ciphertext []byte, err error)
	Open(nonce, ciphertext, aad []byte) ([]byte, error)
	Version() int
}
