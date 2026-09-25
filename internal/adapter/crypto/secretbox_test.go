package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func newBox(t *testing.T) *AESGCM {
	t.Helper()
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	b, err := NewAESGCM(base64.StdEncoding.EncodeToString(k), 1)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAESGCM_RoundTripAndBinding(t *testing.T) {
	b := newBox(t)
	nonce, ct, err := b.Seal([]byte("sk-secret-value"), []byte("anthropic"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("sk-secret-value")) {
		t.Fatal("ciphertext ต้องไม่มีข้อความเดิม")
	}
	got, err := b.Open(nonce, ct, []byte("anthropic"))
	if err != nil || string(got) != "sk-secret-value" {
		t.Fatalf("open = %q %v", got, err)
	}
	// ย้ายไปใช้กับ provider อื่น = ถอดไม่ออก
	if _, err := b.Open(nonce, ct, []byte("gemini")); err == nil {
		t.Fatal("aad ต่างกันต้องถอดไม่ได้")
	}
	// กุญแจหลักคนละตัว = ถอดไม่ออก
	if _, err := newBox(t).Open(nonce, ct, []byte("anthropic")); err == nil {
		t.Fatal("กุญแจหลักต่างกันต้องถอดไม่ได้")
	}
}

func TestNewAESGCM_RejectsBadSecret(t *testing.T) {
	for _, s := range []string{"", "not-base64!!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := NewAESGCM(s, 1); err == nil {
			t.Fatalf("ต้อง error สำหรับ %q", s)
		}
	}
}
