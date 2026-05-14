package service

import (
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		username string
		wantErr  error
	}{
		{"too short", "Ab1!", "user", errPasswordTooShort},
		{"exactly 8 chars ok", "Abcdef1!", "user", nil},
		{"only lower+digit+special", "abc123!@", "user", nil},
		{"only upper+digit+special", "ABC123!@", "user", nil},
		{"only upper+lower+special", "ABCdef!@", "user", nil},
		{"only upper+lower+digit", "ABCdef12", "user", nil},
		{"two varieties weak", "Abcdefgh", "user", errPasswordWeak},
		{"one variety weak", "abcdefgh", "user", errPasswordWeak},
		{"contains username lower", "Abcdef1!user", "user", errPasswordContainsUsername},
		{"contains username upper", "Abcdef1!USER", "user", errPasswordContainsUsername},
		{"contains username mixed", "Abcdef1!UsEr", "user", errPasswordContainsUsername},
		{"empty username skip check", "Abcdef1!user", "", nil},
		{"strong password no username", "Abcdef1!xyz", "user", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password, tt.username)
			if tt.wantErr != nil {
				if !isErr(err, tt.wantErr) {
					t.Fatalf("ValidatePassword(%q, %q) error = %v, want %v", tt.password, tt.username, err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("ValidatePassword(%q, %q) unexpected error = %v", tt.password, tt.username, err)
			}
		})
	}
}

func TestIsWeakPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		username string
		want     bool
	}{
		{"weak", "abc", "user", true},
		{"strong", "Abcdef1!", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWeakPassword(tt.password, tt.username)
			if got != tt.want {
				t.Fatalf("IsWeakPassword(%q, %q) = %v, want %v", tt.password, tt.username, got, tt.want)
			}
		})
	}
}

func TestPasswordErrorCodes(t *testing.T) {
	// 确保错误码符合预期，便于上层断言
	if errPasswordTooShort.Code != errcode.ErrParam.Code {
		t.Fatalf("errPasswordTooShort code = %d, want %d", errPasswordTooShort.Code, errcode.ErrParam.Code)
	}
	if errPasswordWeak.Code != errcode.ErrWeakPassword.Code {
		t.Fatalf("errPasswordWeak code = %d, want %d", errPasswordWeak.Code, errcode.ErrWeakPassword.Code)
	}
	if errPasswordContainsUsername.Code != errcode.ErrParam.Code {
		t.Fatalf("errPasswordContainsUsername code = %d, want %d", errPasswordContainsUsername.Code, errcode.ErrParam.Code)
	}
}
