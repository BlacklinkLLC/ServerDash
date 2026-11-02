package api

import (
	"errors"
	"net/http"

	"serverdash/internal/auth"
	"serverdash/internal/plugins"
	"serverdash/internal/store"
)

type pluginDTO struct {
	*plugins.Plugin
	Enabled   bool              `json:"enabled"`
	Dashboard bool              `json:"dashboard"`
	Values    map[string]string `json:"values,omitempty"` // admins only
}

// handleListPlugins: admins see every plugin with its settings; everyone
// else sees only enabled plugins, without commands or setting values.
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	admin := user.Role == store.RoleAdmin
	out := []pluginDTO{}
	for _, p := range s.plugins.List() {
		st := s.plugins.State(r.Context(), p.ID)
		if !admin && !st.Enabled {
			continue
		}
		dto := pluginDTO{Plugin: p, Enabled: st.Enabled, Dashboard: st.Dashboard}
		if admin {
			dto.Values = st.Settings
		} else {
			cp := *p
			cp.Settings = nil
			cp.Widgets = append([]plugins.Widget{}, p.Widgets...)
			for i := range cp.Widgets {
				cp.Widgets[i].Command = ""
			}
			cp.Actions = []plugins.Action{}
			for _, a := range p.Actions {
				if a.Role == "operator" && user.Role == store.RoleOperator {
					a.Command = ""
					cp.Actions = append(cp.Actions, a)
				}
			}
			dto.Plugin = &cp
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plugins":    out,
		"userDir":    s.plugins.UserDir,
		"bundledDir": s.plugins.BundledDir,
	})
}

func (s *Server) handleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	var req plugins.State
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.plugins.SetState(r.Context(), r.PathValue("id"), req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReloadPlugins(w http.ResponseWriter, r *http.Request) {
	errs := s.plugins.Reload()
	msgs := []string{}
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(s.plugins.List()), "errors": msgs})
}

func (s *Server) handlePluginWidget(w http.ResponseWriter, r *http.Request) {
	res, err := s.plugins.Widget(r.Context(), r.PathValue("id"), r.PathValue("widget"), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handlePluginAction(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	out, err := s.plugins.RunAction(r.Context(), r.PathValue("id"), r.PathValue("action"), user.Role)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, plugins.ErrForbidden) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]any{"ok": false, "error": err.Error(), "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}
