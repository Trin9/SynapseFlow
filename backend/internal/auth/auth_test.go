package auth

import (
	"strings"
	"testing"
	"time"
)

func TestIssueAndParseJWT(t *testing.T) {
	secret := []byte("test-secret-for-unit-test")
	id := &Identity{Subject: "alice", Role: RoleAdmin}

	token, err := IssueJWT(secret, id, time.Hour)
	if err != nil {
		t.Fatalf("IssueJWT error: %v", err)
	}
	if token == "" {
		t.Fatal("IssueJWT returned empty token")
	}

	parsed, err := ParseJWT(secret, token)
	if err != nil {
		t.Fatalf("ParseJWT error: %v", err)
	}
	if parsed.Subject != "alice" {
		t.Errorf("subject: want alice, got %s", parsed.Subject)
	}
	if parsed.Role != RoleAdmin {
		t.Errorf("role: want admin, got %s", parsed.Role)
	}
	if parsed.Mode != "jwt" {
		t.Errorf("mode: want jwt, got %s", parsed.Mode)
	}
}

func TestParseJWT_WrongSecret(t *testing.T) {
	secret := []byte("correct-secret")
	wrongSecret := []byte("wrong-secret")
	id := &Identity{Subject: "bob", Role: RoleViewer}

	token, err := IssueJWT(secret, id, time.Hour)
	if err != nil {
		t.Fatalf("IssueJWT error: %v", err)
	}

	_, err = ParseJWT(wrongSecret, token)
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret, got nil")
	}
}

func TestParseJWT_Expired(t *testing.T) {
	secret := []byte("test-secret")
	id := &Identity{Subject: "carol", Role: RoleOperator}

	// Issue a token that expired 2 minutes ago (beyond jwt/v5's default 1-min skew tolerance)
	token, err := IssueJWT(secret, id, -2*time.Minute)
	if err != nil {
		t.Fatalf("IssueJWT error: %v", err)
	}

	_, err = ParseJWT(secret, token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestIssueJWT_EmptySecret(t *testing.T) {
	_, err := IssueJWT(nil, &Identity{Subject: "x", Role: RoleViewer}, time.Hour)
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestParseToken_Legacy(t *testing.T) {
	id, err := ParseToken("admin:alice")
	if err != nil {
		t.Fatalf("ParseToken error: %v", err)
	}
	if id.Role != RoleAdmin || id.Subject != "alice" {
		t.Errorf("unexpected identity: %+v", id)
	}
}

func TestParseToken_InvalidFormat(t *testing.T) {
	cases := []string{"nocolon", ":nosubject", "admin:", "bad:role:extra"}
	for _, c := range cases {
		_, err := ParseToken(c)
		if err == nil {
			t.Errorf("ParseToken(%q): expected error, got nil", c)
		}
	}
}

func TestAllows(t *testing.T) {
	cases := []struct {
		role     Role
		required Role
		want     bool
	}{
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleOperator, true},
		{RoleAdmin, RoleViewer, true},
		{RoleOperator, RoleAdmin, false},
		{RoleOperator, RoleOperator, true},
		{RoleOperator, RoleViewer, true},
		{RoleViewer, RoleAdmin, false},
		{RoleViewer, RoleOperator, false},
		{RoleViewer, RoleViewer, true},
	}
	for _, c := range cases {
		got := Allows(c.role, c.required)
		if got != c.want {
			t.Errorf("Allows(%s, %s) = %v, want %v", c.role, c.required, got, c.want)
		}
	}
}

func TestJWTTokenStructure(t *testing.T) {
	secret := []byte("structure-test")
	id := &Identity{Subject: "dave", Role: RoleViewer}
	token, err := IssueJWT(secret, id, time.Hour)
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}
	// A valid HS256 JWT has exactly 3 dot-separated parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("JWT should have 3 parts, got %d: %s", len(parts), token)
	}
}
