package auth

import (
	"github.com/pquerna/otp/totp"

	"ai-office-backend/internal/core/port"
)

type totpProvider struct {
	issuer string
}

// NewTOTPProvider สร้าง port.TOTPProvider โดยผูก issuer name (แสดงใน authenticator app)
func NewTOTPProvider(issuer string) port.TOTPProvider {
	return &totpProvider{issuer: issuer}
}

func (p *totpProvider) GenerateSecret(account string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      p.issuer,
		AccountName: account,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func (p *totpProvider) Validate(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}
