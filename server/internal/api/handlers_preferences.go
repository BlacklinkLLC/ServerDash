package api

import (
	"encoding/json"
	"net/http"

	"serverdash/internal/auth"
)

func (s *Server) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errNotAuthenticated)
		return
	}
	prefs := userPreferences{}
	raw, found, err := s.store.GetPreferences(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if found && raw != "" {
		if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
			prefs = userPreferences{}
		}
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errNotAuthenticated)
		return
	}
	var prefs userPreferences
	if err := decodeJSON(r, &prefs); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	catalog := s.widgetCatalog(r)
	if _, err := validateAppearancePatch(prefs.appearancePatch(), catalog); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	raw, _ := json.Marshal(prefs)
	if err := s.store.SetPreferences(r.Context(), user.ID, string(raw)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleResetPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errNotAuthenticated)
		return
	}
	if err := s.store.DeletePreferences(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
