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

func TestCreateUserAndLogin(t *testing.T) {
	svc := NewService("admin", "admin123", "secret")

	created, err := svc.CreateUser("alice", "pw123", "operator")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.Username != "alice" || created.Role != "operator" {
		t.Fatalf("unexpected created user: %+v", created)
	}

	token, user, err := svc.Login("alice", "pw123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if user.Username != "alice" || user.Role != "operator" {
		t.Fatalf("unexpected user after login: %+v", user)
	}

	parsedUser, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if parsedUser.Username != "alice" || parsedUser.Role != "operator" {
		t.Fatalf("unexpected parsed user: %+v", parsedUser)
	}
}
