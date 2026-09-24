package domain

import (
	"testing"
	"time"
)

func TestValidPassword(t *testing.T) {
	ok := []string{"secret12"}
	bad := []string{"short", ""}
	for _, p := range ok {
		if !ValidPassword(p) {
			t.Errorf("ValidPassword(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if ValidPassword(p) {
			t.Errorf("ValidPassword(%q) = true, want false", p)
		}
	}
}

func TestRoleAtLeast(t *testing.T) {
	if !RoleAdmin.AtLeast(RoleOperator) {
		t.Error("admin should be >= operator")
	}
	if RoleViewer.AtLeast(RoleOperator) {
		t.Error("viewer should be < operator")
	}
	if !RoleOperator.AtLeast(RoleOperator) {
		t.Error("operator should be >= operator")
	}
	if Role("nonsense").Valid() {
		t.Error("nonsense role must be invalid")
	}
}

func TestNormalizeUsername(t *testing.T) {
	if NormalizeUsername("  Earth24 ") != "earth24" {
		t.Errorf("got %q", NormalizeUsername("  Earth24 "))
	}
}

func TestLocked(t *testing.T) {
	now := time.Now()
	u := ConsoleUser{LockedUntil: now.Add(time.Minute)}
	if !u.Locked(now) {
		t.Error("should be locked")
	}
	if (ConsoleUser{}).Locked(now) {
		t.Error("zero LockedUntil = not locked")
	}
}

func TestNewConsoleUserID_UniqueHex(t *testing.T) {
	a, b := NewConsoleUserID(), NewConsoleUserID()
	if a == b || len(a) != 32 {
		t.Errorf("ids not unique/len: %q %q", a, b)
	}
}
