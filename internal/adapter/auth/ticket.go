package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ai-office-backend/internal/core/port"
)

// ticketClaims ห่อ RegisteredClaims (Subject = UserID, ExpiresAt ฯลฯ)
// พร้อม field เสริม Stage/PendingSecret ของ 2FA ticket
type ticketClaims struct {
	jwt.RegisteredClaims
	Stage         string `json:"stage"`
	PendingSecret string `json:"pending_secret,omitempty"`
}

type ticketIssuer struct {
	secret []byte
	ttl    time.Duration
}

// NewTicketIssuer สร้าง port.TicketIssuer แบบ HS256 อายุสั้น (คนละ secret/อายุกับ session token)
// secret ██ ห้าม log
func NewTicketIssuer(secret string, ttl time.Duration) port.TicketIssuer {
	return &ticketIssuer{secret: []byte(secret), ttl: ttl}
}

func (i *ticketIssuer) IssueTicket(t port.Ticket) (string, error) {
	now := time.Now()
	claims := ticketClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   t.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
		Stage:         t.Stage,
		PendingSecret: t.PendingSecret,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(i.secret)
}

func (i *ticketIssuer) VerifyTicket(tokenStr string) (port.Ticket, error) {
	var cl ticketClaims
	_, err := jwt.ParseWithClaims(tokenStr, &cl, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		return port.Ticket{}, err
	}
	return port.Ticket{
		UserID:        cl.Subject,
		Stage:         cl.Stage,
		PendingSecret: cl.PendingSecret,
	}, nil
}
