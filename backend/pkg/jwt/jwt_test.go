package jwt

import (
	"testing"
	"time"
)

func TestManagerIssueAndParse(t *testing.T) {
	mgr := NewManager("test-secret-key-123456789012345678901234567890", 3600, 86400)

	claims := &Claims{
		UserID:   42,
		Username: "alice",
		DeptID:   7,
		Roles:    []string{"admin", "editor"},
	}

	access, refresh, err := mgr.Issue(claims)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	if access == "" {
		t.Fatal("access token is empty")
	}
	if refresh == "" {
		t.Fatal("refresh token is empty")
	}

	parsed, err := mgr.Parse(access)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.UserID != 42 {
		t.Errorf("UserID = %d, want 42", parsed.UserID)
	}
	if parsed.Username != "alice" {
		t.Errorf("Username = %s, want alice", parsed.Username)
	}
	if parsed.DeptID != 7 {
		t.Errorf("DeptID = %d, want 7", parsed.DeptID)
	}
	if len(parsed.Roles) != 2 || parsed.Roles[0] != "admin" || parsed.Roles[1] != "editor" {
		t.Errorf("Roles = %v, want [admin editor]", parsed.Roles)
	}
}

func TestManagerParseExpiredToken(t *testing.T) {
	mgr := NewManager("test-secret-key-123456789012345678901234567890", -1, 86400)

	claims := &Claims{UserID: 1, Username: "bob"}
	access, _, err := mgr.Issue(claims)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	time.Sleep(2 * time.Second)
	_, err = mgr.Parse(access)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestManagerParseInvalidToken(t *testing.T) {
	mgr := NewManager("test-secret-key-123456789012345678901234567890", 3600, 86400)

	_, err := mgr.Parse("totally.invalid.token")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestManagerParseWrongSigningMethod(t *testing.T) {
	// Token signed with a different secret should fail
	mgr1 := NewManager("secret-one-12345678901234567890123456789012", 3600, 86400)
	mgr2 := NewManager("secret-two-12345678901234567890123456789012", 3600, 86400)

	claims := &Claims{UserID: 1, Username: "charlie"}
	access, _, err := mgr1.Issue(claims)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	_, err = mgr2.Parse(access)
	if err == nil {
		t.Fatal("expected error for token signed with different secret, got nil")
	}
}

func TestManagerRefreshTokenSubject(t *testing.T) {
	mgr := NewManager("test-secret-key-123456789012345678901234567890", 3600, 86400)

	claims := &Claims{UserID: 99, Username: "dave"}
	_, refresh, err := mgr.Issue(claims)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	parsed, err := mgr.Parse(refresh)
	if err != nil {
		t.Fatalf("Parse refresh token failed: %v", err)
	}
	if parsed.Subject != "refresh" {
		t.Errorf("refresh token Subject = %q, want refresh", parsed.Subject)
	}
}

func TestNewManagerDefaults(t *testing.T) {
	mgr := NewManager("short", 10, 20)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.accessExpiresIn != 10*time.Second {
		t.Errorf("accessExpiresIn = %v, want 10s", mgr.accessExpiresIn)
	}
	if mgr.refreshExpiresIn != 20*time.Second {
		t.Errorf("refreshExpiresIn = %v, want 20s", mgr.refreshExpiresIn)
	}
}
