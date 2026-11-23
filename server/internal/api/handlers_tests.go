package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"serverdash/internal/scripts"
)

type testRunRequest struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Dir      string `json:"dir"`
	ScriptID int64  `json:"scriptId"`
}

type testRunResponse struct {
	Name       string `json:"name"`
	OK         bool   `json:"ok"`
	ExitCode   int    `json:"exitCode"`
	Output     string `json:"output"`
	DurationMs int64  `json:"durationMs"`
	Recorded   bool   `json:"recorded"`
	Error      string `json:"error,omitempty"`
}

func (s *Server) handleRunTest(w http.ResponseWriter, r *http.Request) {
	var req testRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.ScriptID > 0 {
		script, err := s.store.GetScript(r.Context(), req.ScriptID)
		if err != nil {
			writeError(w, statusForStoreErr(err), err)
			return
		}
		req.Command = script.Content
		if req.Name == "" {
			req.Name = script.Name
		}
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, errors.New("command is required"))
		return
	}

	dir := ""
	if req.Dir != "" {
		_, resolved, err := s.resolveHostPath(req.Dir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			writeError(w, http.StatusBadRequest, errors.New("working directory does not exist"))
			return
		}
		dir = resolved
	}

	ctx := context.Background()
	var runID int64
	if req.ScriptID > 0 {
		var err error
		runID, err = s.store.StartScriptRun(ctx, req.ScriptID, "test-runner")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	started := time.Now()
	exitCode, output, runErr := scripts.RunInDir(ctx, dir, req.Command)
	result := testRunResponse{
		Name:       req.Name,
		OK:         runErr == nil && exitCode == 0,
		ExitCode:   exitCode,
		Output:     output,
		DurationMs: time.Since(started).Milliseconds(),
	}
	if runErr != nil {
		result.Error = runErr.Error()
		result.Output += "\n" + runErr.Error()
	}
	if runID > 0 {
		if err := s.store.FinishScriptRun(ctx, runID, exitCode, result.Output); err != nil {
			result.Recorded = false
			if result.Error == "" {
				result.Error = err.Error()
			}
		} else {
			result.Recorded = true
		}
	}
	writeJSON(w, http.StatusOK, result)
}
