package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ai-office-backend/internal/core/domain"
)

func sampleTicket() domain.AccessTicket {
	return domain.AccessTicket{ID: "t1", OfficeID: "demo", ServiceID: "K11S", Kind: "k",
		User: domain.HostUser{ID: "u1", Username: "adm", Permissions: []string{"A"}, Level: 7}, SealedGrant: "sealed",
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
}

func TestAccessTicket_RoundTrip(t *testing.T) {
	iss := NewAccessTicketIssuer("0123456789abcdef0123456789abcdef")
	tok, err := iss.Issue(sampleTicket())
	if err != nil {
		t.Fatal(err)
	}
	got, err := iss.Verify(tok)
	if err != nil || got.OfficeID != "demo" || got.ServiceID != "K11S" || got.User.Level != 7 || got.User.Permissions[0] != "A" || got.SealedGrant != "sealed" {
		t.Fatalf("round trip ผิด: %+v %v", got, err)
	}
}

func TestAccessTicket_RejectsOtherKeyAlgAndTyp(t *testing.T) {
	iss := NewAccessTicketIssuer("0123456789abcdef0123456789abcdef")
	other := NewAccessTicketIssuer("ffffffffffffffffffffffffffffffff")
	tok, _ := other.Issue(sampleTicket())
	if _, err := iss.Verify(tok); err != domain.ErrTicketInvalid {
		t.Fatalf("key อื่นต้องไม่ผ่าน: %v", err)
	}
	// alg=none
	none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"typ": "ai-ticket", "oid": "demo", "sid": "K11S", "sub": "u1", "exp": time.Now().Add(time.Hour).Unix()})
	s, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := iss.Verify(s); err == nil {
		t.Fatal("alg=none ต้องไม่ผ่าน")
	}
	// typ อื่น (เช่น console session) ต้องไม่ผ่าน แม้ key เดียวกัน
	mc := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"typ": "console", "oid": "demo", "sid": "K11S", "sub": "u1", "exp": time.Now().Add(time.Hour).Unix()})
	s2, _ := mc.SignedString([]byte("0123456789abcdef0123456789abcdef"))
	if _, err := iss.Verify(s2); err == nil {
		t.Fatal("typ ผิดต้องไม่ผ่าน")
	}
	// ไม่มี exp ต้องไม่ผ่าน
	noexp := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"typ": "ai-ticket", "oid": "demo", "sid": "K11S", "sub": "u1"})
	s3, _ := noexp.SignedString([]byte("0123456789abcdef0123456789abcdef"))
	if _, err := iss.Verify(s3); err == nil {
		t.Fatal("ไม่มี exp ต้องไม่ผ่าน")
	}
}

func TestGrantSealer_BindsToTicket(t *testing.T) {
	s, _ := NewGrantSealer("0123456789abcdef0123456789abcdef")
	sealed, err := s.Seal("grant.jwt.value", "demo|K11S|u1")
	if err != nil || strings.Contains(sealed, "grant") {
		t.Fatalf("seal ผิด/อ่านออก: %q %v", sealed, err)
	}
	if g, err := s.Open(sealed, "demo|K11S|u1"); err != nil || g != "grant.jwt.value" {
		t.Fatalf("open ผิด: %q %v", g, err)
	}
	if _, err := s.Open(sealed, "demo|PG99|u1"); err == nil {
		t.Fatal("grant ย้ายไปใช้กับ service อื่นต้องไม่ได้")
	}
	other, _ := NewGrantSealer("ffffffffffffffffffffffffffffffff")
	if _, err := other.Open(sealed, "demo|K11S|u1"); err == nil {
		t.Fatal("key อื่นต้องเปิดไม่ได้")
	}
}
