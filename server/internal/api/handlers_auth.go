package api

import (
	"net/http"

	"serverdash/internal/auth"
	"serverdash/internal/store"
)

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": count == 0})
}

type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleSetup creates the first admin account. It's only ever valid once —
// as soon as any user exists, this endpoint refuses, so it can't be used to
// mint a second admin without going through normal authenticated user
// management.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, errSetupAlreadyDone)
		return
	}

	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateCredentials(req.Username, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Username, req.Password, store.RoleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	sess, err := s.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	auth.SetSessionCookie(w, sess)
	writeJSON(w, http.StatusCreated, userResponse(user))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.store.VerifyPassword(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errInvalidCredentials)
		return
	}

	sess, err := s.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	auth.SetSessionCookie(w, sess)
	writeJSON(w, http.StatusOK, userResponse(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		_ = s.store.DeleteSession(r.Context(), cookie.Value)
	}
	auth.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errNotAuthenticated)
		return
	}
	writeJSON(w, http.StatusOK, userResponse(user))
}

type userDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

func userResponse(u store.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: string(u.Role), CreatedAt: u.CreatedAt.Format(timeFormat)}
}
