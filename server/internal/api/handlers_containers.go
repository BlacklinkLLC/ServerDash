package api

import (
	"net/http"

	"github.com/gorilla/websocket"
)

const nicknameRuntimeContainer = "container"

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	list, err := s.docker.List(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	nicks, err := s.store.Nicknames(r.Context(), nicknameRuntimeContainer)
	if err == nil {
		for i := range list {
			list[i].Nickname = nicks[list[i].ID]
		}
	}

	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.docker.Start(r.Context(), id); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.docker.Stop(r.Context(), id); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.docker.Restart(r.Context(), id); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

// handleLogs upgrades to a WebSocket and streams the container's log output
// so the web dashboard can tail logs live without polling.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	stream, err := s.docker.Logs(r.Context(), id, true)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer stream.Close()

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			if werr := conn.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

type nicknameRequest struct {
	Nickname string `json:"nickname"`
}

func (s *Server) handleSetContainerNickname(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req nicknameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SetNickname(r.Context(), nicknameRuntimeContainer, id, req.Nickname); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteContainerNickname(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteNickname(r.Context(), nicknameRuntimeContainer, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRuntimeInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.docker.RuntimeInfo(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}
