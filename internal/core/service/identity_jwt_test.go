package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

// makeJWT สร้าง token 3 ส่วน (header.payload.signature) โดย payload = JSON ที่ให้มา
// signature เป็นค่าอะไรก็ได้ เพราะ parseOfficeJWT ไม่ verify signature
func makeJWT(payload map[string]any) string {
	b, _ := json.Marshal(payload)
	p := base64.RawURLEncoding.EncodeToString(b)
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	return h + "." + p + ".sig"
}

func officeResult(exp int64) map[string]any {
	return map[string]any{
		"exp": exp,
		"result": map[string]any{
			"id":       "665f0a1b2c3d4e5f60718293",
			"username": "adm_ploy",
			"role": map[string]any{
				"name":  "manager",
				"level": 3,
				"permission": []map[string]any{
					{"code": "DEPOSIT_VIEW"},
					{"code": ""}, // ต้องถูกข้าม
					{"code": "MEMBER_EDIT"},
				},
				"list_service": []map[string]any{
					{"service": "DEMO-STAGING", "permission": true},
					{"service": "SECRET-SVC", "permission": false}, // ต้องถูกข้าม
					{"service": "", "permission": true},            // ต้องถูกข้าม
				},
			},
		},
	}
}

func TestParseOfficeJWT_Valid(t *testing.T) {
	token := makeJWT(officeResult(time.Now().Add(time.Hour).Unix()))

	caller, err := parseOfficeJWT(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caller.AdminID != "665f0a1b2c3d4e5f60718293" {
		t.Errorf("AdminID = %q", caller.AdminID)
	}
	if caller.Username != "adm_ploy" {
		t.Errorf("Username = %q", caller.Username)
	}
	if caller.RoleName != "manager" {
		t.Errorf("RoleName = %q", caller.RoleName)
	}
	if caller.Level != 3 {
		t.Errorf("Level = %d", caller.Level)
	}
	if got := caller.Permissions; len(got) != 2 || got[0] != "DEPOSIT_VIEW" || got[1] != "MEMBER_EDIT" {
		t.Errorf("Permissions = %v (want [DEPOSIT_VIEW MEMBER_EDIT])", got)
	}
	// เอาเฉพาะ service ที่ permission==true และ service != ""
	if got := caller.Services; len(got) != 1 || got[0] != "DEMO-STAGING" {
		t.Errorf("Services = %v (want [DEMO-STAGING])", got)
	}
}

func TestParseOfficeJWT_Expired(t *testing.T) {
	token := makeJWT(officeResult(time.Now().Add(-time.Minute).Unix()))
	if _, err := parseOfficeJWT(token); err != domain.ErrSessionExpired {
		t.Errorf("err = %v (want ErrSessionExpired)", err)
	}
}

func TestParseOfficeJWT_Malformed(t *testing.T) {
	cases := map[string]string{
		"not a jwt":    "abc",
		"two parts":    "aaa.bbb",
		"bad base64":   "aaa.$$$.ccc",
		"empty result": makeJWT(map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "result": map[string]any{}}),
	}
	for name, tok := range cases {
		if _, err := parseOfficeJWT(tok); err != domain.ErrNotAuthenticated {
			t.Errorf("%s: err = %v (want ErrNotAuthenticated)", name, err)
		}
	}
}
