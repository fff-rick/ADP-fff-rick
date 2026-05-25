package api

import (
	"adp/internal/model"
	"context"
	"errors"
	"net/http"
)

type contextKey string

const userContextKey contextKey = "user"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	token, user, err := s.authService.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token: token,
		User:  user,
	})
}

func (s *Server) withUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, errors.New("missing bearer token"))
			return
		}

		user, err := s.authService.ParseToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	}
}

func currentUser(r *http.Request) model.User {
	user, _ := r.Context().Value(userContextKey).(model.User)
	return user
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		writeError(w, http.StatusForbidden, errors.New("admin role required"))
		return
	}

	writeJSON(w, http.StatusOK, s.authService.ListUsers())
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		writeError(w, http.StatusForbidden, errors.New("admin role required"))
		return
	}

	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.authService.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	actor := currentUser(r)
	s.recordAudit("user", actor.Username, "user.created", "user", user.Username, map[string]any{
		"role": user.Role,
	})

	writeJSON(w, http.StatusCreated, user)
}
