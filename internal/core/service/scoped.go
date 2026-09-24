package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// scopedTokens ขอกุญแจดอกเล็กจาก host ด้วย grant ที่ปิดผนึกอยู่ในตั๋ว (D-86) และจำไว้ในหน่วยความจำ
//
// ██ ไม่เก็บลง DB/Redis (spec §7.3) · replica แต่ละตัวขอของตัวเอง
// ██ backend ไม่ถือ secret ของ host เลย — grant หมดอายุพร้อมตั๋ว
type scopedTokens struct {
	host   port.HostClient
	sealer port.GrantSealer
	now    port.Clock

	mu sync.Mutex
	m  map[string]scopedEntry
}

type scopedEntry struct {
	token string
	exp   time.Time
}

func NewScopedTokenSource(host port.HostClient, sealer port.GrantSealer, now port.Clock) port.ScopedTokenSource {
	if now == nil {
		now = time.Now
	}
	return &scopedTokens{host: host, sealer: sealer, now: now, m: map[string]scopedEntry{}}
}

func scopedKey(o domain.Office, t domain.AccessTicket) string {
	return o.ID + "|" + t.ServiceID + "|" + t.User.ID + "|" + t.ID
}

var (
	ErrGrantRejected  = errors.New("GRANT_REJECTED")
	ErrUserNotAllowed = errors.New("USER_NOT_ALLOWED")
)

func (s *scopedTokens) Token(ctx context.Context, o domain.Office, conn *connector.Connector, t domain.AccessTicket) (string, error) {
	key := scopedKey(o, t)
	s.mu.Lock()
	if e, ok := s.m[key]; ok && s.now().Add(30*time.Second).Before(e.exp) {
		s.mu.Unlock()
		return e.token, nil
	}
	s.mu.Unlock()

	grant, err := s.sealer.Open(t.SealedGrant, TicketBinding(o.ID, t.ServiceID, t.User.ID))
	if err != nil {
		return "", fmt.Errorf("เปิด grant ไม่ได้: %w", err)
	}
	if o.BackofficeAPIURL == "" {
		return "", fmt.Errorf("office ยังไม่ได้ตั้ง backoffice_api_url")
	}
	timeout := time.Duration(firstPositive(conn.Host.HostAPI.TimeoutMs, 5000)) * time.Millisecond
	resp, err := s.host.Do(ctx, port.HostRequest{
		Method:  "POST",
		URL:     strings.TrimRight(o.BackofficeAPIURL, "/") + conn.Host.HostAPI.ScopedTokenPath,
		Header:  map[string]string{"Accept": "application/json"},
		Body:    map[string]any{"grant": grant},
		Timeout: timeout,
	})
	if err != nil {
		return "", err
	}
	switch resp.Status {
	case 200:
	case 401:
		return "", ErrGrantRejected
	case 403:
		return "", ErrUserNotAllowed
	default:
		return "", fmt.Errorf("host /ai/scoped-token ตอบ %d", resp.Status)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Token     string `json:"token"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil || body.Code != 0 || body.Data.Token == "" {
		return "", fmt.Errorf("response ของ /ai/scoped-token ผิดรูป")
	}
	exp := time.Unix(body.Data.ExpiresAt, 0)
	if body.Data.ExpiresAt <= 0 || exp.Sub(s.now()) > 10*time.Minute {
		// host ต้องออกอายุ 5 นาที — ถ้ามาแปลก ๆ จำไว้แค่ 4 นาที
		exp = s.now().Add(4 * time.Minute)
	}
	s.mu.Lock()
	if len(s.m) > 2000 {
		now := s.now()
		for k, e := range s.m {
			if now.After(e.exp) {
				delete(s.m, k)
			}
		}
	}
	s.m[key] = scopedEntry{token: body.Data.Token, exp: exp}
	s.mu.Unlock()
	return body.Data.Token, nil
}

func (s *scopedTokens) Invalidate(o domain.Office, t domain.AccessTicket) {
	s.mu.Lock()
	delete(s.m, scopedKey(o, t))
	s.mu.Unlock()
}
