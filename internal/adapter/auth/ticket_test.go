package auth

import (
	"testing"
	"time"

	"ai-office-backend/internal/core/port"
)

func TestTicketRoundtrip(t *testing.T) {
	ti := NewTicketIssuer("s3cr3t", 5*time.Minute)
	tok, err := ti.IssueTicket(port.Ticket{UserID: "u1", Stage: "enroll", PendingSecret: "ABC123"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ti.VerifyTicket(tok)
	if err != nil || got.UserID != "u1" || got.Stage != "enroll" || got.PendingSecret != "ABC123" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestTicketRejectsExpiredAndWrongSecret(t *testing.T) {
	exp, _ := NewTicketIssuer("s", -time.Minute).IssueTicket(port.Ticket{UserID: "u1", Stage: "totp"})
	if _, err := NewTicketIssuer("s", -time.Minute).VerifyTicket(exp); err == nil {
		t.Error("expired ticket must be rejected")
	}
	tok, _ := NewTicketIssuer("a", time.Minute).IssueTicket(port.Ticket{UserID: "u1", Stage: "totp"})
	if _, err := NewTicketIssuer("b", time.Minute).VerifyTicket(tok); err == nil {
		t.Error("wrong secret must be rejected")
	}
}
