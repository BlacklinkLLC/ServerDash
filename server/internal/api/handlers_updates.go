package api

import (
	"net/http"

	"serverdash/internal/selfupdate"
)

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, selfupdate.CheckStatus(r.Context(), s.cfg.InstallDir))
}

func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if err := selfupdate.Apply(r.Context(), s.cfg.InstallDir, s.cfg.ComposeCmd); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "updating",
	})
}
