package api

import (
	"errors"
	"net/http"
	"strconv"

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

func (s *Server) handleSystemHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.sampler.History())
}

func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	procs, err := system.Processes(r.URL.Query().Get("sort"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, procs)
}

type signalRequest struct {
	Signal string `json:"signal"`
}

func (s *Server) handleSignalProcess(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(r.PathValue("pid"), 10, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid pid"))
		return
	}
	var req signalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := system.Signal(int32(pid), req.Signal); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	ports, err := system.ListeningPorts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ports)
}

func (s *Server) handleInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := system.Interfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ifaces)
}
