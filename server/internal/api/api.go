// Package api wires up ServerDash's HTTP and WebSocket endpoints.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"serverdash/internal/auth"
	"serverdash/internal/automation"
	"serverdash/internal/config"
	"serverdash/internal/containers"
	"serverdash/internal/kubernetes"
	"serverdash/internal/store"
)

type Server struct {
	cfg        config.Config
	docker     *containers.Client
	store      *store.Store
	kube       *kubernetes.Client
	automation *automation.Runner
	upgrader   websocket.Upgrader
}

func New(cfg config.Config, docker *containers.Client, st *store.Store, kube *kubernetes.Client, runner *automation.Runner) *Server {
	return &Server{
		cfg:        cfg,
		docker:     docker,
		store:      st,
		kube:       kube,
		automation: runner,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// publicPaths bypass authentication entirely: health checks, and the two
// endpoints that exist specifically to let an unauthenticated caller become
// authenticated (first-run setup, login).
var publicPaths = map[string]bool{
	"/api/health":       true,
	"/api/setup/status": true,
	"/api/setup":        true,
	"/api/auth/login":   true,
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)

	// --- Auth / setup ---
	mux.HandleFunc("GET /api/setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)

	// --- Users (admin) ---
	mux.HandleFunc("GET /api/users", auth.RequireRole(s.handleListUsers, store.RoleAdmin))
	mux.HandleFunc("POST /api/users", auth.RequireRole(s.handleCreateUser, store.RoleAdmin))
	mux.HandleFunc("PUT /api/users/{id}/role", auth.RequireRole(s.handleSetUserRole, store.RoleAdmin))
	mux.HandleFunc("PUT /api/users/{id}/password", auth.RequireRole(s.handleSetUserPassword, store.RoleAdmin))
	mux.HandleFunc("DELETE /api/users/{id}", auth.RequireRole(s.handleDeleteUser, store.RoleAdmin))

	// --- System / runtime ---
	mux.HandleFunc("GET /api/system", s.handleSystem)
	mux.HandleFunc("GET /api/runtime", s.handleRuntimeInfo)

	// --- Containers (Docker/Podman) ---
	mux.HandleFunc("GET /api/containers", s.handleListContainers)
	mux.HandleFunc("POST /api/containers/{id}/start", auth.RequireRole(s.handleStart, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("POST /api/containers/{id}/stop", auth.RequireRole(s.handleStop, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("POST /api/containers/{id}/restart", auth.RequireRole(s.handleRestart, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("GET /api/containers/{id}/logs", s.handleLogs)
	mux.HandleFunc("PUT /api/containers/{id}/nickname", auth.RequireRole(s.handleSetContainerNickname, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("DELETE /api/containers/{id}/nickname", auth.RequireRole(s.handleDeleteContainerNickname, store.RoleAdmin, store.RoleOperator))

	// --- Kubernetes ---
	mux.HandleFunc("GET /api/k8s/status", s.handleK8sStatus)
	mux.HandleFunc("GET /api/k8s/pods", s.handleK8sListPods)
	mux.HandleFunc("POST /api/k8s/pods/{namespace}/{name}/restart", auth.RequireRole(s.handleK8sRestartPod, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("GET /api/k8s/pods/{namespace}/{name}/logs", s.handleK8sPodLogs)
	mux.HandleFunc("PUT /api/k8s/pods/{namespace}/{name}/nickname", auth.RequireRole(s.handleSetK8sNickname, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("DELETE /api/k8s/pods/{namespace}/{name}/nickname", auth.RequireRole(s.handleDeleteK8sNickname, store.RoleAdmin, store.RoleOperator))

	// --- Automation (admin) ---
	mux.HandleFunc("GET /api/automation/rules", auth.RequireRole(s.handleListAutomationRules, store.RoleAdmin))
	mux.HandleFunc("PUT /api/automation/rules/{id}", auth.RequireRole(s.handleUpdateAutomationRule, store.RoleAdmin))
	mux.HandleFunc("POST /api/automation/prune", auth.RequireRole(s.handlePruneNow, store.RoleAdmin))

	// --- Scripts (admin) ---
	mux.HandleFunc("GET /api/scripts", auth.RequireRole(s.handleListScripts, store.RoleAdmin))
	mux.HandleFunc("POST /api/scripts", auth.RequireRole(s.handleCreateScript, store.RoleAdmin))
	mux.HandleFunc("PUT /api/scripts/{id}", auth.RequireRole(s.handleUpdateScript, store.RoleAdmin))
	mux.HandleFunc("DELETE /api/scripts/{id}", auth.RequireRole(s.handleDeleteScript, store.RoleAdmin))
	mux.HandleFunc("POST /api/scripts/{id}/run", auth.RequireRole(s.handleRunScript, store.RoleAdmin))
	mux.HandleFunc("GET /api/scripts/{id}/runs", auth.RequireRole(s.handleListScriptRuns, store.RoleAdmin))

	authMW := &auth.Middleware{Store: s.store, PublicPaths: publicPaths}
	return withCORS(s.cfg.PublicHost, withLogging(authMW.Wrap(mux)))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// withCORS allows the web dashboard's origin (its public hostname, over
// http/https, at any port) to call this API from the browser, with
// credentials (the session cookie) included.
func withCORS(publicHost string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && strings.Contains(origin, publicHost) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
