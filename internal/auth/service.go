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

	"adp/internal/model"
)

type Service struct {
	adminUsername string
	adminPassword string
	secret        []byte
	mu            sync.RWMutex
	users         map[string]storedUser
}

type storedUser struct {
	password string
	user     model.User
}

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
	svc.users[adminUsername] = storedUser{
		password: adminPassword,
		user: model.User{
			Username: adminUsername,
			Role:     "admin",
		},
	}
	return svc
}

func (s *Service) Login(username, password string) (string, model.User, error) {
	s.mu.RLock()
	record, ok := s.users[username]
	s.mu.RUnlock()
	if !ok || password != record.password {
		return "", model.User{}, errors.New("invalid username or password")
	}

	token, err := s.sign(Claims{
		Subject: record.user.Username,
		Role:    record.user.Role,
		Expiry:  time.Now().Add(12 * time.Hour).Unix(),
	})
	if err != nil {
		return "", model.User{}, err
	}

	return token, record.user, nil
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

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[username]; exists {
		return model.User{}, errors.New("user already exists")
	}

	user := model.User{
		Username: username,
		Role:     role,
	}
	s.users[username] = storedUser{
		password: password,
		user:     user,
	}

	return user, nil
}

func (s *Service) ListUsers() []model.User {
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
