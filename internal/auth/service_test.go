package auth

import "testing"

func TestLoginAndParseToken(t *testing.T) {
	svc := NewService("admin", "admin123", "secret")

	token, user, err := svc.Login("admin", "admin123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if user.Username != "admin" {
		t.Fatalf("unexpected username: %s", user.Username)
	}

	parsedUser, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}

	if parsedUser.Role != "admin" {
		t.Fatalf("unexpected role: %s", parsedUser.Role)
	}
}
