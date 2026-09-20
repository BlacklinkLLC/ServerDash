package api

import (
	"errors"
	"net/http"
	"strconv"

	"serverdash/internal/auth"
	"serverdash/internal/store"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, userResponse(u))
	}
	writeJSON(w, http.StatusOK, out)
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateCredentials(req.Username, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	role := store.Role(req.Role)
	if !role.Valid() {
		writeError(w, http.StatusBadRequest, errors.New("role must be admin, operator, or viewer"))
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Username, req.Password, role)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, userResponse(user))
}

type setRoleRequest struct {
	Role string `json:"role"`
}

func (s *Server) handleSetUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if actor, ok := auth.UserFromContext(r.Context()); ok && actor.ID == id {
		writeError(w, http.StatusBadRequest, errors.New("cannot change your own role"))
		return
	}

	var req setRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	role := store.Role(req.Role)
	if !role.Valid() {
		writeError(w, http.StatusBadRequest, errors.New("role must be admin, operator, or viewer"))
		return
	}

	if err := s.store.SetUserRole(r.Context(), id, role); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type setPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleSetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req setPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, errors.New("password must be at least 8 characters"))
		return
	}
	if err := s.store.SetUserPassword(r.Context(), id, req.Password); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if actor, ok := auth.UserFromContext(r.Context()); ok && actor.ID == id {
		writeError(w, http.StatusBadRequest, errors.New("cannot delete your own account"))
		return
	}

	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func statusForStoreErr(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, store.ErrConflict) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
