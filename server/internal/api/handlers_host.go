package api

import (
	"net/http"
	"strconv"

	"serverdash/internal/systemd"
)

// handleHostInfo tells the dashboard how ServerDash is deployed, so it can
// show or hide features that only work in one mode (systemd services and
// the journal need a native install; containers need a reachable runtime).
func (s *Server) handleHostInfo(w http.ResponseWriter, r *http.Request) {
	mode := "container"
	if s.cfg.HostRoot == "" {
		mode = "native"
	}
	_, dockerErr := s.docker.RuntimeInfo(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"deployment":        mode,
		"systemd":           systemd.Available(),
		"containersRuntime": dockerErr == nil,
		"selfUpdate":        s.cfg.InstallDir != "",
	})
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	if !systemd.Available() {
		writeJSON(w, http.StatusOK, []systemd.Unit{})
		return
	}
	units, err := systemd.List(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, units)
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	if err := systemd.Action(r.Context(), r.PathValue("name"), r.PathValue("action")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJournal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lines, _ := strconv.Atoi(q.Get("lines"))
	priority := -1
	if p := q.Get("priority"); p != "" {
		priority, _ = strconv.Atoi(p)
	}
	entries, err := systemd.Journal(r.Context(), q.Get("unit"), lines, priority, q.Get("grep"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
