package domain

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

var roleRank = map[Role]int{RoleViewer: 1, RoleOperator: 2, RoleAdmin: 3}

func (r Role) Valid() bool           { _, ok := roleRank[r]; return ok }
func (r Role) AtLeast(min Role) bool { return roleRank[r] >= roleRank[min] && roleRank[r] > 0 }

type ConsoleUser struct {
	ID             string    `json:"id" bson:"_id"`
	Username       string    `json:"username" bson:"username"`
	DisplayName    string    `json:"display_name" bson:"display_name"`
	PasswordHash   string    `json:"-" bson:"password_hash"`
	Role           Role      `json:"role" bson:"role"`
	Status         string    `json:"status" bson:"status"`
	FailedAttempts int       `json:"-" bson:"failed_attempts"`
	LockedUntil    time.Time `json:"-" bson:"locked_until"`
	LastLoginAt    time.Time `json:"last_login_at" bson:"last_login_at"`
	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
	CreatedBy      string    `json:"created_by" bson:"created_by"`
	UpdatedAt      time.Time `json:"updated_at" bson:"updated_at"`

	TOTPSecret     string   `json:"-" bson:"totp_secret"`
	TOTPEnrolled   bool     `json:"totp_enrolled" bson:"totp_enrolled"`
	RecoveryHashes []string `json:"-" bson:"recovery_hashes"`
}

func (u ConsoleUser) Locked(now time.Time) bool {
	return !u.LockedUntil.IsZero() && u.LockedUntil.After(now)
}

func ValidPassword(pw string) bool { return len(pw) >= 8 && len(pw) <= 72 }

func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func NewConsoleUserID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
