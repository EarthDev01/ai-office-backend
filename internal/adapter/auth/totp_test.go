package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func timeNow() time.Time { return time.Now() }

func TestTOTPGenerateAndValidate(t *testing.T) {
	p := NewTOTPProvider("AI Office Console")
	secret, uri, err := p.GenerateSecret("earth24")
	if err != nil || secret == "" {
		t.Fatalf("generate: %q %v", secret, err)
	}
	if uri == "" || uri[:10] != "otpauth://" {
		t.Fatalf("otpauth uri looks wrong: %q", uri)
	}
	// สร้าง code ปัจจุบันจาก secret แล้วต้อง validate ผ่าน
	code, err := totp.GenerateCode(secret, timeNow())
	if err != nil {
		t.Fatal(err)
	}
	if !p.Validate(secret, code) {
		t.Error("valid code should pass")
	}
	if p.Validate(secret, "000000") && code != "000000" {
		t.Error("wrong code should fail")
	}
	if p.Validate(secret, "") || p.Validate("", code) {
		t.Error("empty inputs must fail")
	}
}
