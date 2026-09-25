package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// chatAudience กันไม่ให้ token ชนิดอื่นที่บังเอิญใช้ secret เดียวกันถูกรับเป็นตั๋วแชท
const chatAudience = "ai-office-chat"

type chatClaims struct {
	jwt.RegisteredClaims
	Office      string   `json:"office"`
	Service     string   `json:"service"`
	Username    string   `json:"username"`
	Level       int32    `json:"level"`
	Permissions []string `json:"perms,omitempty"`
}

type chatTicketIssuer struct {
	secret []byte
	ttl    time.Duration
}

// NewChatTicketIssuer — secret ██ ห้าม log
func NewChatTicketIssuer(secret string, ttl time.Duration) port.ChatTicketIssuer {
	return &chatTicketIssuer{secret: []byte(secret), ttl: ttl}
}

func (i *chatTicketIssuer) TTL() time.Duration { return i.ttl }

func (i *chatTicketIssuer) Issue(t domain.ChatTicket) (string, error) {
	if t.ID == "" {
		b := make([]byte, 12)
		_, _ = rand.Read(b)
		t.ID = hex.EncodeToString(b)
	}
	now := time.Now()
	claims := chatClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        t.ID,
			Subject:   t.AdminID,
			Audience:  jwt.ClaimStrings{chatAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
		Office: t.OfficeID, Service: t.ServiceID, Username: t.Username,
		Level: t.Level, Permissions: t.Permissions,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

func (i *chatTicketIssuer) Verify(token string) (domain.ChatTicket, error) {
	var cl chatClaims
	_, err := jwt.ParseWithClaims(token, &cl, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	}, jwt.WithAudience(chatAudience), jwt.WithExpirationRequired())
	if err != nil {
		return domain.ChatTicket{}, err
	}
	if cl.ID == "" || cl.Office == "" || cl.Service == "" {
		return domain.ChatTicket{}, fmt.Errorf("ตั๋วไม่ครบ")
	}
	return domain.ChatTicket{
		ID: cl.ID, OfficeID: cl.Office, ServiceID: cl.Service, AdminID: cl.Subject,
		Username: cl.Username, Level: cl.Level, Permissions: cl.Permissions,
		ExpiresAt: cl.ExpiresAt.Time,
	}, nil
}
