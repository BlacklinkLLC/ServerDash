package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"serverdash/internal/auth"
	"serverdash/internal/store"
	"serverdash/internal/workflow"
)

type workflowDTO struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	TriggerTime string          `json:"triggerTime"`
	TriggerTZ   string          `json:"triggerTz"`
	Blocks      json.RawMessage `json:"blocks"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

func workflowResponse(wf store.Workflow) workflowDTO {
	blocks := wf.Blocks
	if blocks == "" {
		blocks = "[]"
	}
	return workflowDTO{
		ID: wf.ID, Name: wf.Name, TriggerTime: wf.TriggerTime, TriggerTZ: wf.TriggerTZ,
		Blocks: json.RawMessage(blocks), Enabled: wf.Enabled,
		CreatedAt: wf.CreatedAt.Format(timeFormat), UpdatedAt: wf.UpdatedAt.Format(timeFormat),
	}
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListWorkflows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]workflowDTO, 0, len(list))
	for _, wf := range list {
		out = append(out, workflowResponse(wf))
	}
	writeJSON(w, http.StatusOK, out)
}

type workflowRequest struct {
	Name        string          `json:"name"`
	TriggerTime string          `json:"triggerTime"`
	TriggerTZ   string          `json:"triggerTz"`
	Blocks      json.RawMessage `json:"blocks"`
	Enabled     bool            `json:"enabled"`
}

func (req workflowRequest) validate() ([]workflow.Block, error) {
	blocks, err := workflow.ParseBlocks(string(req.Blocks))
	if err != nil {
		return nil, err
	}
	if err := workflow.Validate(blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req workflowRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errWorkflowNameEmpty)
		return
	}
	if _, err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var createdBy int64
	if user, ok := auth.UserFromContext(r.Context()); ok {
		createdBy = user.ID
	}

	wf, err := s.store.CreateWorkflow(r.Context(), req.Name, req.TriggerTime, req.TriggerTZ, string(req.Blocks), createdBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, workflowResponse(wf))
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req workflowRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errWorkflowNameEmpty)
		return
	}
	if _, err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.store.UpdateWorkflow(r.Context(), id, req.Name, req.TriggerTime, req.TriggerTZ, string(req.Blocks), req.Enabled); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteWorkflow(r.Context(), id); err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	wf, err := s.store.GetWorkflow(r.Context(), id)
	if err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}

	// Synchronous, like scripts' run-now: the admin panel waits and shows
	// the result. Background context so navigating away doesn't kill a
	// git pull or compose build partway through.
	s.automation.RunWorkflow(context.Background(), wf, "manual")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	runs, err := s.store.ListWorkflowRuns(r.Context(), id, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]workflowRunDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, workflowRunResponse(run))
	}
	writeJSON(w, http.StatusOK, out)
}

type workflowRunDTO struct {
	ID          int64   `json:"id"`
	WorkflowID  int64   `json:"workflowId"`
	StartedAt   string  `json:"startedAt"`
	FinishedAt  *string `json:"finishedAt,omitempty"`
	Success     *bool   `json:"success,omitempty"`
	Log         string  `json:"log"`
	TriggeredBy string  `json:"triggeredBy"`
}

func workflowRunResponse(run store.WorkflowRun) workflowRunDTO {
	dto := workflowRunDTO{
		ID: run.ID, WorkflowID: run.WorkflowID, StartedAt: run.StartedAt.Format(timeFormat),
		Log: run.Log, TriggeredBy: run.TriggeredBy,
	}
	if run.FinishedAt.Valid {
		v := run.FinishedAt.Time.Format(timeFormat)
		dto.FinishedAt = &v
	}
	if run.Success.Valid {
		dto.Success = &run.Success.Bool
	}
	return dto
}
