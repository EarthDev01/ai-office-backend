package port

// TOTPProvider สร้าง/ตรวจ TOTP secret (RFC 6238) ผ่าน authenticator app
type TOTPProvider interface {
	// GenerateSecret คืน base32 secret + otpauth:// URI (ไว้ทำ QR)
	GenerateSecret(account string) (secret string, otpauthURI string, err error)
	// Validate ตรวจ 6-digit code กับ secret (ยอม skew ±1 ช่วง)
	Validate(secret, code string) bool
}

// Ticket คือ payload ของ token อายุสั้นระหว่าง 2 ขั้นของ login
type Ticket struct {
	UserID        string
	Stage         string // "enroll" | "totp"
	PendingSecret string // เฉพาะ stage "enroll": secret ที่รอ user ยืนยัน
}

// TicketIssuer เซ็น/ตรวจ ticket (JWT อายุสั้น) — คนละอายุกับ session token
type TicketIssuer interface {
	IssueTicket(t Ticket) (string, error)
	VerifyTicket(token string) (Ticket, error) // err ถ้า signature/exp/alg ผิด
}
