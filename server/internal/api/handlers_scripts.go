package api

import (
	"context"
	"net/http"
	"strconv"

	"serverdash/internal/auth"
	"serverdash/internal/scripts"
	"serverdash/internal/store"
)

type scriptDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Schedule  string `json:"schedule"`
	Enabled   bool   `json:"enabled"`
	CreatedBy *int64 `json:"createdBy,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func scriptResponse(sc store.Script) scriptDTO {
	dto := scriptDTO{
		ID: sc.ID, Name: sc.Name, Content: sc.Content, Schedule: sc.Schedule, Enabled: sc.Enabled,
		CreatedAt: sc.CreatedAt.Format(timeFormat), UpdatedAt: sc.UpdatedAt.Format(timeFormat),
	}
	if sc.CreatedBy.Valid {
		dto.CreatedBy = &sc.CreatedBy.Int64
	}
	return dto
}

type scriptRunDTO struct {
	ID          int64   `json:"id"`
	ScriptID    int64   `json:"scriptId"`
	StartedAt   string  `json:"startedAt"`
	FinishedAt  *string `json:"finishedAt,omitempty"`
	ExitCode    *int64  `json:"exitCode,omitempty"`
	Output      string  `json:"output"`
	TriggeredBy string  `json:"triggeredBy"`
}

func scriptRunResponse(run store.ScriptRun) scriptRunDTO {
	dto := scriptRunDTO{
		ID: run.ID, ScriptID: run.ScriptID, StartedAt: run.StartedAt.Format(timeFormat),
		Output: run.Output, TriggeredBy: run.TriggeredBy,
	}
	if run.FinishedAt.Valid {
		s := run.FinishedAt.Time.Format(timeFormat)
		dto.FinishedAt = &s
	}
	if run.ExitCode.Valid {
		dto.ExitCode = &run.ExitCode.Int64
	}
	return dto
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func (s *Server) handleListScripts(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListScripts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]scriptDTO, 0, len(list))
	for _, sc := range list {
		out = append(out, scriptResponse(sc))
	}
	writeJSON(w, http.StatusOK, out)
}

type scriptRequest struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Schedule string `json:"schedule"`
	Enabled  bool   `json:"enabled"`
}

func (s *Server) handleCreateScript(w http.ResponseWriter, r *http.Request) {
	var req scriptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, errScriptNameOrContentEmpty)
		return
	}

	var createdBy int64
	if user, ok := auth.UserFromContext(r.Context()); ok {
		createdBy = user.ID
	}

	sc, err := s.store.CreateScript(r.Context(), req.Name, req.Content, req.Schedule, createdBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, scriptResponse(sc))
}

func (s *Server) handleUpdateScript(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req scriptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, errScriptNameOrContentEmpty)
		return
	}

	if err := s.store.UpdateScript(r.Context(), id, req.Name, req.Content, req.Schedule, req.Enabled); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteScript(r.Context(), id); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRunScript(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc, err := s.store.GetScript(r.Context(), id)
	if err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}

	// Runs synchronously and returns once complete: the admin panel shows a
	// spinner while it waits, simpler than a job-polling API for scripts
	// that are expected to run in seconds, not minutes. Uses a background
	// context (not r.Context()) so closing the browser tab doesn't kill a
	// script that's still running server-side.
	if err := scripts.RunAndRecord(context.Background(), s.store, sc, "manual"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListScriptRuns(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	runs, err := s.store.ListScriptRuns(r.Context(), id, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]scriptRunDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, scriptRunResponse(run))
	}
	writeJSON(w, http.StatusOK, out)
}
