package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// customClaims ห่อ RegisteredClaims (Subject = UserID, ExpiresAt ฯลฯ)
// พร้อม field เสริม Username/Role ของ console user
type customClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
	Role     string `json:"role"`
}

type jwtIssuer struct {
	secret []byte
	ttl    time.Duration
}

// NewJWTIssuer สร้าง port.TokenIssuer แบบ HS256
// secret ██ ห้าม log
func NewJWTIssuer(secret string, ttl time.Duration) port.TokenIssuer {
	return &jwtIssuer{secret: []byte(secret), ttl: ttl}
}

func (j *jwtIssuer) Issue(c port.Claims) (string, error) {
	now := time.Now()
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
		Username: c.Username,
		Role:     string(c.Role),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(j.secret)
}

func (j *jwtIssuer) Verify(tokenStr string) (port.Claims, error) {
	var cl customClaims
	_, err := jwt.ParseWithClaims(tokenStr, &cl, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return port.Claims{}, err
	}
	return port.Claims{
		UserID:   cl.Subject,
		Username: cl.Username,
		Role:     domain.Role(cl.Role),
	}, nil
}
