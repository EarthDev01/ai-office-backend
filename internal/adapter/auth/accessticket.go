package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ตั๋วของ widget (AccessTicket) — คนละ secret กับ console session และคนละ typ
//
// ██ secret ห้าม log · production ต้องตั้ง TICKET_SECRET ยาว ≥ 32 ตัว (config ตรวจ)
type accessClaims struct {
	jwt.RegisteredClaims
	Typ   string   `json:"typ"`
	Usr   string   `json:"usr"`
	Name  string   `json:"name"`
	OID   string   `json:"oid"`
	SID   string   `json:"sid"`
	Kind  string   `json:"knd"`
	Perms []string `json:"prm"`
	Lvl   int      `json:"lvl"`
	Dept  string   `json:"dpt"`
	Grant string   `json:"g"`
	TFP   string   `json:"tfp,omitempty"`
}

const accessTyp = "ai-ticket"

type accessIssuer struct {
	secret []byte
	now    func() time.Time
}

func NewAccessTicketIssuer(secret string) port.AccessTicketIssuer {
	return &accessIssuer{secret: []byte(secret), now: time.Now}
}

func (i *accessIssuer) Issue(t domain.AccessTicket) (string, error) {
	c := accessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        t.ID,
			Subject:   t.User.ID,
			IssuedAt:  jwt.NewNumericDate(t.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(t.ExpiresAt),
		},
		Typ: accessTyp, Usr: t.User.Username, Name: t.User.DisplayName,
		OID: t.OfficeID, SID: t.ServiceID, Kind: t.Kind,
		Perms: t.User.Permissions, Lvl: t.User.Level, Dept: t.User.Dept, Grant: t.SealedGrant, TFP: t.TokenFP,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(i.secret)
}

func (i *accessIssuer) Verify(token string) (domain.AccessTicket, error) {
	var c accessClaims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("alg %v ไม่รับ", t.Header["alg"])
		}
		return i.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithTimeFunc(i.now))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return domain.AccessTicket{}, domain.ErrTicketExpired
		}
		return domain.AccessTicket{}, domain.ErrTicketInvalid
	}
	if c.Typ != accessTyp || c.OID == "" || c.SID == "" || c.Subject == "" {
		return domain.AccessTicket{}, domain.ErrTicketInvalid
	}
	return domain.AccessTicket{
		ID: c.ID, OfficeID: c.OID, ServiceID: c.SID, Kind: c.Kind,
		User:        domain.HostUser{ID: c.Subject, Username: c.Usr, DisplayName: c.Name, Permissions: c.Perms, Level: c.Lvl, Dept: c.Dept},
		SealedGrant: c.Grant, TokenFP: c.TFP, IssuedAt: c.IssuedAt.Time, ExpiresAt: c.ExpiresAt.Time,
	}, nil
}

// grantSealer = AES-256-GCM · key แยกจาก HMAC key ด้วย HKDF · AAD = office|service|user (สลับตั๋วใช้ grant ไม่ได้)
type grantSealer struct {
	aead cipher.AEAD
}

func NewGrantSealer(masterSecret string) (port.GrantSealer, error) {
	key, err := hkdf.Key(sha256.New, []byte(masterSecret), []byte("ai-office-backend"), "grant-seal-v1", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &grantSealer{aead: aead}, nil
}

func (s *grantSealer) Seal(grant, bind string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := s.aead.Seal(nil, nonce, []byte(grant), []byte(bind))
	return base64.RawURLEncoding.EncodeToString(append(nonce, ct...)), nil
}

func (s *grantSealer) Open(sealed, bind string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	n := s.aead.NonceSize()
	if len(raw) < n {
		return "", errors.New("sealed grant สั้นเกิน")
	}
	pt, err := s.aead.Open(nil, raw[:n], raw[n:], []byte(bind))
	if err != nil {
		return "", errors.New("grant ถูกแก้หรือไม่ใช่ของตั๋วนี้")
	}
	return string(pt), nil
}
