package api

import (
	"net/http"

	"serverdash/internal/system"
)

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	snap, err := system.Collect(s.cfg.HostRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
