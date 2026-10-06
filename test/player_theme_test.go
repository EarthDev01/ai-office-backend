package test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"ai-office-backend/internal/core/domain"
)

const playerOrigin = "http://tangtem.test"

// playerJWT — token ผู้เล่นแบบ topupserie (service/authJWT.go): payload {username, session_id, exp} ไม่มีชื่อแบรนด์
func playerJWT(username string) string {
	b, _ := json.Marshal(map[string]any{"username": username, "session_id": "s1", "exp": time.Now().Add(time.Hour).Unix(), "iss": "auto"})
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	return head + "." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

// 1 โดเมนผู้เล่น = 1 office ชนิด theme-tangtem ที่มีเว็บเดียว
func tangtemOffice() domain.Office {
	o := domain.NewOffice("tangtem", "Tangtem")
	o.Kind = "theme-tangtem"
	o.AllowedOrigins = []string{playerOrigin}
	s := domain.DefaultService("TANGTEM", "Tangtem")
	s.Enabled = true
	o.Services = []domain.Service{s}
	return o
}

func TestPlayerTheme_PageConfigBootstrapSession(t *testing.T) {
	r := newRouter(t, demoOffice(), tangtemOffice())

	// page-config บอกเว็บของโดเมนนี้ (หน้าผู้เล่นไม่มีตัวเลือกเว็บ) + ที่เก็บ token แบบ @nuxtjs/auth
	w := do(t, r, req{method: "GET", path: "/api/ai/widget/page-config", origin: playerOrigin})
	cfg := payload[struct {
		Kind    string `json:"kind"`
		Service struct {
			Source string `json:"source"`
			Value  string `json:"value"`
		} `json:"service"`
		Token struct {
			Key     string `json:"key"`
			KeyFrom struct {
				Key string `json:"key"`
			} `json:"key_from"`
		} `json:"token"`
	}](t, w)
	if cfg.Kind != "theme-tangtem" || cfg.Service.Source != "office" || cfg.Service.Value != "TANGTEM" {
		t.Fatalf("page-config = %+v", cfg)
	}
	if cfg.Token.Key != "auth._token.{key_from}" || cfg.Token.KeyFrom.Key != "auth.strategy" {
		t.Fatalf("token source = %+v", cfg.Token)
	}

	// token ผู้เล่น (มีแค่ username) ผ่าน bootstrap และขอตั๋วได้
	tok := playerJWT("somchai01")
	w = do(t, r, req{method: "GET", path: bootPath("TANGTEM"), origin: playerOrigin, token: tok})
	if b := payload[struct{ Enabled bool }](t, w); w.Code != 200 || !b.Enabled {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body.String())
	}
	w = do(t, r, req{method: "POST", path: "/api/ai/widget/service/TANGTEM/browser-session", origin: playerOrigin, token: tok})
	if s := payload[struct{ Ticket string }](t, w); w.Code != 200 || s.Ticket == "" {
		t.Fatalf("browser-session = %d %s", w.Code, w.Body.String())
	}

	// โดเมนผู้เล่นขอเว็บอื่นไม่ได้
	w = do(t, r, req{method: "GET", path: bootPath("K11S"), origin: playerOrigin, token: tok})
	if b := payload[struct{ Enabled bool }](t, w); b.Enabled {
		t.Fatalf("โดเมนผู้เล่นเปิดเว็บอื่นได้: %s", w.Body.String())
	}
}
