package auth

import (
	"testing"
	"time"
)

func TestChallengeLifecycle(t *testing.T) {
	mgr := NewManager("secret")
	id, code, expires, err := mgr.CreateChallenge("user@example.com")
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	if expires.Before(time.Now()) {
		t.Fatalf("expires in past")
	}
	email, err := mgr.VerifyChallenge(id, code)
	if err != nil {
		t.Fatalf("VerifyChallenge: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("unexpected email %s", email)
	}
	if _, err := mgr.VerifyChallenge(id, code); err == nil {
		t.Fatalf("expected error after challenge consumed")
	}
}

func TestTokenValidation(t *testing.T) {
	mgr := NewManager("secret")
	token, err := mgr.IssueToken("user@example.com", time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	email, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("unexpected email %s", email)
	}
}

func TestExpiredToken(t *testing.T) {
	mgr := NewManager("secret")
	token, err := mgr.IssueToken("user@example.com", -time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := mgr.ValidateToken(token); err == nil {
		t.Fatalf("expected expiration error")
	}
}
