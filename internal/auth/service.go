package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"adp/internal/model"
)

type Service struct {
	adminUsername string
	adminPassword string
	secret        []byte
}

type Claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Expiry  int64  `json:"exp"`
}

func NewService(adminUsername, adminPassword, secret string) *Service {
	return &Service{
		adminUsername: adminUsername,
		adminPassword: adminPassword,
		secret:        []byte(secret),
	}
}

func (s *Service) Login(username, password string) (string, model.User, error) {
	if username != s.adminUsername || password != s.adminPassword {
		return "", model.User{}, errors.New("invalid username or password")
	}

	user := model.User{
		Username: username,
		Role:     "admin",
	}

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
