package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"adp/internal/domain/model"
)

// UserStore defines the persistence interface for user operations.
// Implemented by db.Repository for PostgreSQL and by the in-memory fallback.
type UserStore interface {
	CreateUser(username, passwordHash, role string) (model.User, error)
	GetUser(username string) (passwordHash string, user model.User, found bool, err error)
	ListUsers() ([]model.User, error)
	DeleteUser(username string) error
	UpdatePassword(username, newHash string) error
}

type Service struct {
	adminUsername string
	adminPassword string
	secret        []byte
	mu            sync.RWMutex
	users         map[string]storedUser
	userStore     UserStore // optional: database-backed user store
}

type storedUser struct {
	password string
	user     model.User
}

func (s storedUser) GetPassword() string { return s.password }
func (s storedUser) GetUser() model.User { return s.user }

type Claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Expiry  int64  `json:"exp"`
}

func NewService(adminUsername, adminPassword, secret string) *Service {
	svc := &Service{
		adminUsername: adminUsername,
		adminPassword: adminPassword,
		secret:        []byte(secret),
		users:         make(map[string]storedUser),
	}

	// Seed admin user with bcrypt hash.
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	passwordHash := adminPassword
	if err == nil {
		passwordHash = string(hash)
	}
	svc.users[adminUsername] = storedUser{
		password: passwordHash,
		user: model.User{
			Username: adminUsername,
			Role:     "admin",
		},
	}
	return svc
}

// SetUserStore sets an optional database-backed user store.
// When set, all user operations delegate to the store in addition to in-memory.
func (s *Service) SetUserStore(store UserStore) {
	s.userStore = store
}

// SeedUserStore syncs the in-memory admin user to the database.
func (s *Service) SeedUserStore() error {
	if s.userStore == nil {
		return nil
	}

	_, _, found, err := s.userStore.GetUser(s.adminUsername)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	_, err = s.userStore.CreateUser(s.adminUsername, s.users[s.adminUsername].password, "admin")
	return err
}

func (s *Service) Login(username, password string) (string, model.User, error) {
	// Try database first if available.
	if s.userStore != nil {
		passwordHash, user, found, err := s.userStore.GetUser(username)
		if err != nil {
			return "", model.User{}, fmt.Errorf("login: %w", err)
		}
		if found {
			if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
				// Fallback: try plaintext comparison for migrated users.
				if password != passwordHash {
					return "", model.User{}, errors.New("invalid username or password")
				}
			}
			return s.generateToken(user)
		}
	}

	// Fall back to in-memory.
	s.mu.RLock()
	record, ok := s.users[username]
	s.mu.RUnlock()

	// Try bcrypt compare first, then fall back to plaintext for backward compat.
	if ok {
		if bcrypt.CompareHashAndPassword([]byte(record.password), []byte(password)) == nil {
			return s.generateToken(record.user)
		}
	}

	// Plaintext comparison for backward compatibility.
	if ok && password == record.password {
		return s.generateToken(record.user)
	}

	return "", model.User{}, errors.New("invalid username or password")
}

func (s *Service) generateToken(user model.User) (string, model.User, error) {
	token, err := s.sign(Claims{
		Subject: user.Username,
		Role:    user.Role,
		Expiry:  time.Now().Add(12 * time.Hour).Unix(),
	})
	if err != nil {
		return "", model.User{}, err
	}
	return token, user, nil
}

func (s *Service) CreateUser(username, password, role string) (model.User, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	role = strings.TrimSpace(role)
	if username == "" || password == "" {
		return model.User{}, errors.New("username and password are required")
	}
	if role == "" {
		role = "operator"
	}
	if role != "admin" && role != "operator" {
		return model.User{}, errors.New("role must be admin or operator")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}
	passwordHash := string(hash)

	user := model.User{
		Username: username,
		Role:     role,
	}

	// Persist to database if available.
	if s.userStore != nil {
		if _, err := s.userStore.CreateUser(username, passwordHash, role); err != nil {
			return model.User{}, err
		}
	}

	// Also keep in-memory.
	s.mu.Lock()
	if _, exists := s.users[username]; exists {
		s.mu.Unlock()
		return model.User{}, errors.New("user already exists")
	}
	s.users[username] = storedUser{
		password: passwordHash,
		user:     user,
	}
	s.mu.Unlock()

	return user, nil
}

func (s *Service) ListUsers() []model.User {
	// Use database if available.
	if s.userStore != nil {
		users, err := s.userStore.ListUsers()
		if err == nil {
			return users
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]model.User, 0, len(s.users))
	for _, record := range s.users {
		users = append(users, record.user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})
	return users
}

// DeleteUser removes a user. Only admin can perform this action.
func (s *Service) DeleteUser(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	if username == s.adminUsername {
		return errors.New("cannot delete the initial admin user")
	}

	// Delete from database if available.
	if s.userStore != nil {
		if err := s.userStore.DeleteUser(username); err != nil {
			return err
		}
	}

	// Delete from in-memory.
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[username]; !exists {
		return errors.New("user not found")
	}
	delete(s.users, username)
	return nil
}

// ChangePassword changes a user's password.
func (s *Service) ChangePassword(username, newPassword string) error {
	username = strings.TrimSpace(username)
	newPassword = strings.TrimSpace(newPassword)
	if username == "" || newPassword == "" {
		return errors.New("username and new password are required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	passwordHash := string(hash)

	// Update database if available.
	if s.userStore != nil {
		if err := s.userStore.UpdatePassword(username, passwordHash); err != nil {
			return err
		}
	}

	// Update in-memory.
	s.mu.Lock()
	defer s.mu.Unlock()
	record, exists := s.users[username]
	if !exists {
		return errors.New("user not found")
	}
	record.password = passwordHash
	s.users[username] = record
	return nil
}

func (s *Service) ParseToken(token string) (model.User, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return model.User{}, errors.New("invalid token format")
	}

	expectedSignature := s.signRaw(parts[0] + "." + parts[1])
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return model.User{}, errors.New("invalid token signature")
	}

	payload, err := decodeSegment(parts[1])
	if err != nil {
		return model.User{}, err
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return model.User{}, err
	}

	if time.Now().Unix() > claims.Expiry {
		return model.User{}, errors.New("token expired")
	}

	return model.User{
		Username: claims.Subject,
		Role:     claims.Role,
	}, nil
}

func (s *Service) sign(claims Claims) (string, error) {
	headerBytes, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	header := encodeSegment(headerBytes)
	payload := encodeSegment(payloadBytes)
	signingInput := header + "." + payload
	signature := s.signRaw(signingInput)

	return fmt.Sprintf("%s.%s", signingInput, signature), nil
}

func (s *Service) signRaw(signingInput string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(signingInput))
	return encodeSegment(mac.Sum(nil))
}

func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSegment(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
