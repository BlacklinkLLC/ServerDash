package api

import (
	"encoding/json"
	"net/http"

	"serverdash/internal/store"
)

var automationRuleIDs = []string{store.RuleAutoRestartUnhealthy, store.RulePrune, store.RuleBackup}

func (s *Server) handleListAutomationRules(w http.ResponseWriter, r *http.Request) {
	rules := make([]store.AutomationRule, 0, len(automationRuleIDs))
	for _, id := range automationRuleIDs {
		rule, err := s.store.GetAutomationRule(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		rules = append(rules, rule)
	}
	writeJSON(w, http.StatusOK, rules)
}

type updateRuleRequest struct {
	Enabled  bool            `json:"enabled"`
	Config   json.RawMessage `json:"config"`
	Schedule string          `json:"schedule"`
}

func (s *Server) handleUpdateAutomationRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isKnownRule(id) {
		writeError(w, http.StatusNotFound, errUnknownAutomationRule)
		return
	}

	var req updateRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	config := "{}"
	if len(req.Config) > 0 {
		config = string(req.Config)
	}

	if err := s.store.UpsertAutomationRule(r.Context(), store.AutomationRule{
		ID: id, Enabled: req.Enabled, Config: config, Schedule: req.Schedule,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// The unhealthy-restart rule is polled by a ticker, not cron, so it
	// picks up the change on its own; the others need the schedule rebuilt.
	if err := s.automation.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePruneNow(w http.ResponseWriter, r *http.Request) {
	report, err := s.docker.Prune(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func isKnownRule(id string) bool {
	for _, known := range automationRuleIDs {
		if id == known {
			return true
		}
	}
	return false
}
