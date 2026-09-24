package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

func TestIssueVerifyRoundtrip(t *testing.T) {
	iss := NewJWTIssuer("s3cr3t", time.Hour)
	tok, err := iss.Issue(port.Claims{UserID: "u1", Username: "earth24", Role: domain.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	c, err := iss.Verify(tok)
	if err != nil || c.UserID != "u1" || c.Role != domain.RoleAdmin {
		t.Fatalf("got %+v %v", c, err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, _ := NewJWTIssuer("a", time.Hour).Issue(port.Claims{UserID: "u1", Role: domain.RoleAdmin})
	if _, err := NewJWTIssuer("b", time.Hour).Verify(tok); err == nil {
		t.Fatal("must reject wrong secret")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	tok, _ := NewJWTIssuer("a", -time.Minute).Issue(port.Claims{UserID: "u1", Role: domain.RoleViewer})
	if _, err := NewJWTIssuer("a", -time.Minute).Verify(tok); err == nil {
		t.Fatal("must reject expired")
	}
}

func TestVerifyRejectsNoneAlg(t *testing.T) {
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "u1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Username: "earth24",
		Role:     string(domain.RoleAdmin),
	}
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tok, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewJWTIssuer("a", time.Hour).Verify(tok); err == nil {
		t.Fatal("must reject non-HMAC (alg:none) signing method")
	}
}
