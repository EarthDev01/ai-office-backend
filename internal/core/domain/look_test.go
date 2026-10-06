package domain

import "testing"

func TestValidateLookValue(t *testing.T) {
	ok := []struct {
		v       string
		pattern bool
	}{
		{"", false}, {"asset:avatars/dara.png", false}, {"asset:backgrounds/blue-1.webp", true},
		{"https://cdn.example.com/a/b.png?x=1", false}, {"http://localhost:6767/widget/v1/assets/avatars/a.png", false},
		{"pattern:dots", true}, {"pattern:glow", true},
	}
	for _, c := range ok {
		if err := ValidateLookValue(c.v, c.pattern); err != nil {
			t.Errorf("%q ต้องผ่าน: %v", c.v, err)
		}
	}
	bad := []struct {
		v       string
		pattern bool
	}{
		{"pattern:dots", false}, // รูปผู้ช่วยเป็นลายไม่ได้
		{"pattern:evil", true}, {"asset:../secret.png", false}, {"asset:avatars/a.exe", false}, {"asset:dara.png", false},
		{"http://evil.example.com/a.png", false}, {"javascript:alert(1)", false},
		{`https://x.example.com/a.png");background:url(x`, true}, {"https://x.example.com/a b.png", false},
		{"https://x.example.com/'a'.png", false},
	}
	for _, c := range bad {
		if err := ValidateLookValue(c.v, c.pattern); err == nil {
			t.Errorf("%q ต้องถูกปฏิเสธ", c.v)
		}
	}
}
