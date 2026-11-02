package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"serverdash/internal/auth"
	"serverdash/internal/cluster"
	"serverdash/internal/store"
	"serverdash/internal/workflow"
)

type workflowDTO struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	TriggerTime string           `json:"triggerTime"`
	TriggerTZ   string           `json:"triggerTz"`
	Trigger     workflow.Trigger `json:"trigger"`
	OnFailure   workflow.Alert   `json:"onFailure"`
	Summary     string           `json:"triggerSummary"`
	Blocks      json.RawMessage  `json:"blocks"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   string           `json:"createdAt"`
	UpdatedAt   string           `json:"updatedAt"`
}

func workflowResponse(wf store.Workflow) workflowDTO {
	blocks := wf.Blocks
	if blocks == "" {
		blocks = "[]"
	}
	opts := workflow.ParseOptions(wf.Options, wf.TriggerTime, wf.TriggerTZ)
	return workflowDTO{
		ID: wf.ID, Name: wf.Name, TriggerTime: wf.TriggerTime, TriggerTZ: wf.TriggerTZ,
		Trigger: opts.Trigger, OnFailure: opts.OnFailure, Summary: opts.Trigger.Describe(),
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
	Name        string            `json:"name"`
	TriggerTime string            `json:"triggerTime"`
	TriggerTZ   string            `json:"triggerTz"`
	Trigger     *workflow.Trigger `json:"trigger"`
	OnFailure   workflow.Alert    `json:"onFailure"`
	Blocks      json.RawMessage   `json:"blocks"`
	Enabled     bool              `json:"enabled"`
}

// options validates the request and returns the options JSON to store.
// existing is the saved workflow's options on update (so a webhook keeps
// its URL when edited), or "" on create.
func (req *workflowRequest) options(existing string) (string, error) {
	blocks, err := workflow.ParseBlocks(string(req.Blocks))
	if err != nil {
		return "", err
	}
	if err := workflow.Validate(blocks); err != nil {
		return "", err
	}
	if req.Trigger == nil {
		// Legacy clients send only triggerTime/triggerTz.
		req.Trigger = &workflow.Trigger{Type: workflow.TriggerDaily, Time: req.TriggerTime, TZ: req.TriggerTZ}
	}
	opts := workflow.Options{Trigger: *req.Trigger, OnFailure: req.OnFailure}
	if opts.Trigger.Type == workflow.TriggerWebhook && opts.Trigger.Token == "" {
		prev := workflow.ParseOptions(existing, "", "")
		if prev.Trigger.Token != "" {
			opts.Trigger.Token = prev.Trigger.Token
		} else {
			opts.Trigger.Token = randomHex(24)
		}
	}
	if err := workflow.ValidateOptions(opts); err != nil {
		return "", err
	}
	// Keep the legacy columns meaningful for daily/weekly triggers.
	if opts.Trigger.Time != "" {
		req.TriggerTime, req.TriggerTZ = opts.Trigger.Time, opts.Trigger.TZ
	}
	b, _ := json.Marshal(opts)
	return string(b), nil
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
	opts, err := req.options("")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var createdBy int64
	if user, ok := auth.UserFromContext(r.Context()); ok {
		createdBy = user.ID
	}

	wf, err := s.store.CreateWorkflow(r.Context(), req.Name, req.TriggerTime, req.TriggerTZ, string(req.Blocks), opts, req.Enabled, createdBy)
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
	existing, err := s.store.GetWorkflow(r.Context(), id)
	if err != nil {
		writeError(w, statusForStoreErr(err), err)
		return
	}
	opts, err := req.options(existing.Options)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.store.UpdateWorkflow(r.Context(), id, req.Name, req.TriggerTime, req.TriggerTZ, string(req.Blocks), opts, req.Enabled); err != nil {
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

type recentRunDTO struct {
	workflowRunDTO
	WorkflowName string `json:"workflowName"`
}

func (s *Server) handleRecentWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	runs, err := s.store.ListRecentWorkflowRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]recentRunDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, recentRunDTO{workflowRunDTO: workflowRunResponse(run.WorkflowRun), WorkflowName: run.WorkflowName})
	}
	writeJSON(w, http.StatusOK, out)
}

const maxWebhookPayload = 64 << 10

// handleWorkflowHook is public: the long random token in the URL is the
// credential, like a Discord or GitHub webhook URL. The run happens in
// the background so callers (CI, GitHub) get a fast 202.
func (s *Server) handleWorkflowHook(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	wfs, err := s.store.ListWorkflows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, wf := range wfs {
		t := workflow.ParseOptions(wf.Options, wf.TriggerTime, wf.TriggerTZ).Trigger
		if t.Type != workflow.TriggerWebhook || !cluster.SecretsEqual(t.Token, token) {
			continue
		}
		if !wf.Enabled {
			writeError(w, http.StatusConflict, errors.New("this workflow is disabled"))
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, maxWebhookPayload))
		wf := wf
		go s.automation.RunWorkflowWith(context.Background(), wf, workflow.Context{
			Trigger: "webhook",
			Event:   "webhook from " + clientIP(r),
			Payload: string(body),
		})
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "workflow": wf.Name})
		return
	}
	writeError(w, http.StatusNotFound, errors.New("no workflow uses this webhook"))
}

func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		ip, _, _ := strings.Cut(f, ",")
		return strings.TrimSpace(ip)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
