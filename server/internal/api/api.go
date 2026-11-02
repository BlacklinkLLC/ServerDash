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
	"serverdash/internal/cluster"
	"serverdash/internal/config"
	"serverdash/internal/containers"
	"serverdash/internal/kubernetes"
	"serverdash/internal/plugins"
	"serverdash/internal/store"
	"serverdash/internal/system"
)

type Server struct {
	cfg        config.Config
	docker     *containers.Client
	store      *store.Store
	kube       *kubernetes.Client
	automation *automation.Runner
	sampler    *system.Sampler
	agent      *cluster.Agent
	plugins    *plugins.Manager
	upgrader   websocket.Upgrader
}

func New(cfg config.Config, docker *containers.Client, st *store.Store, kube *kubernetes.Client, runner *automation.Runner, sampler *system.Sampler, agent *cluster.Agent, pm *plugins.Manager) *Server {
	return &Server{
		plugins:    pm,
		sampler:    sampler,
		agent:      agent,
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

// publicPaths bypass authentication entirely: health checks; the two
// endpoints that exist specifically to let an unauthenticated caller become
// authenticated (first-run setup, login); and branding (read-only), so the
// setup/login screens can render the operator's app name and logo before
// anyone has signed in. Keyed by "METHOD /path" (not bare path) so a route
// with both a public GET and an admin-only PUT/POST on the same path — like
// /api/settings — only exposes the read.
var publicPaths = map[string]bool{
	"GET /api/health":        true,
	"GET /api/setup/status":  true,
	"POST /api/setup":        true,
	"POST /api/auth/login":   true,
	"GET /api/settings":      true,
	"GET /api/branding/logo": true,
	// Cluster: authenticated by a single-use join token / node secret
	// instead of a session.
	"POST /api/cluster/register":  true,
	"POST /api/cluster/heartbeat": true,
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
	mux.HandleFunc("GET /api/system/history", s.handleSystemHistory)
	mux.HandleFunc("GET /api/host", s.handleHostInfo)
	// --- Host services / journal (native installs with systemd) ---
	mux.HandleFunc("GET /api/services", auth.RequireRole(s.handleListServices, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("POST /api/services/{name}/{action}", auth.RequireRole(s.handleServiceAction, store.RoleAdmin))
	mux.HandleFunc("GET /api/journal", auth.RequireRole(s.handleJournal, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("GET /api/system/interfaces", s.handleInterfaces)
	// Process command lines and socket owners can leak secrets (tokens in
	// argv), so these tools are limited to operators and admins; sending
	// signals is admin-only, matching the terminal.
	mux.HandleFunc("GET /api/system/processes", auth.RequireRole(s.handleProcesses, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("GET /api/system/ports", auth.RequireRole(s.handlePorts, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("POST /api/system/processes/{pid}/signal", auth.RequireRole(s.handleSignalProcess, store.RoleAdmin))

	// --- Containers (Docker/Podman) ---
	mux.HandleFunc("GET /api/containers", s.handleListContainers)
	mux.HandleFunc("POST /api/containers/{id}/start", auth.RequireRole(s.handleStart, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("POST /api/containers/{id}/stop", auth.RequireRole(s.handleStop, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("POST /api/containers/{id}/restart", auth.RequireRole(s.handleRestart, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("GET /api/containers/{id}/logs", s.handleLogs)
	mux.HandleFunc("PUT /api/containers/{id}/nickname", auth.RequireRole(s.handleSetContainerNickname, store.RoleAdmin, store.RoleOperator))
	mux.HandleFunc("DELETE /api/containers/{id}/nickname", auth.RequireRole(s.handleDeleteContainerNickname, store.RoleAdmin, store.RoleOperator))
	// A shell in a container is as powerful as Scripts (arbitrary code
	// execution), so it's gated the same way: admin only.
	mux.HandleFunc("GET /api/containers/{id}/terminal", auth.RequireRole(s.handleContainerTerminal, store.RoleAdmin))

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

	// --- Workflows: block-based scheduled deployment automations (admin) ---
	mux.HandleFunc("GET /api/workflows", auth.RequireRole(s.handleListWorkflows, store.RoleAdmin))
	mux.HandleFunc("POST /api/workflows", auth.RequireRole(s.handleCreateWorkflow, store.RoleAdmin))
	mux.HandleFunc("PUT /api/workflows/{id}", auth.RequireRole(s.handleUpdateWorkflow, store.RoleAdmin))
	mux.HandleFunc("DELETE /api/workflows/{id}", auth.RequireRole(s.handleDeleteWorkflow, store.RoleAdmin))
	mux.HandleFunc("POST /api/workflows/{id}/run", auth.RequireRole(s.handleRunWorkflow, store.RoleAdmin))
	mux.HandleFunc("GET /api/workflows/{id}/runs", auth.RequireRole(s.handleListWorkflowRuns, store.RoleAdmin))
	mux.HandleFunc("GET /api/workflow-runs", auth.RequireRole(s.handleRecentWorkflowRuns, store.RoleAdmin))
	mux.HandleFunc("POST /api/hooks/{token}", s.handleWorkflowHook)

	// --- Plugins: widgets for everyone signed in; managing them is admin-only,
	// and each action declares which role may run it.
	mux.HandleFunc("GET /api/plugins", s.handleListPlugins)
	mux.HandleFunc("POST /api/plugins/reload", auth.RequireRole(s.handleReloadPlugins, store.RoleAdmin))
	mux.HandleFunc("PUT /api/plugins/{id}", auth.RequireRole(s.handleUpdatePlugin, store.RoleAdmin))
	mux.HandleFunc("GET /api/plugins/{id}/widgets/{widget}", s.handlePluginWidget)
	mux.HandleFunc("POST /api/plugins/{id}/actions/{action}", auth.RequireRole(s.handlePluginAction, store.RoleAdmin, store.RoleOperator))

	// --- Cluster: primaries, nodes, replicas ---
	mux.HandleFunc("GET /api/cluster", s.handleClusterOverview)
	mux.HandleFunc("PUT /api/cluster/mode", auth.RequireRole(s.handleClusterMode, store.RoleAdmin))
	mux.HandleFunc("POST /api/cluster/tokens", auth.RequireRole(s.handleCreateJoinToken, store.RoleAdmin))
	mux.HandleFunc("POST /api/cluster/register", s.handleClusterRegister)
	mux.HandleFunc("POST /api/cluster/heartbeat", s.handleClusterHeartbeat)
	mux.HandleFunc("PUT /api/cluster/nodes/{id}", auth.RequireRole(s.handleUpdateClusterNode, store.RoleAdmin))
	mux.HandleFunc("DELETE /api/cluster/nodes/{id}", auth.RequireRole(s.handleDeleteClusterNode, store.RoleAdmin))
	mux.HandleFunc("/api/cluster/nodes/{id}/proxy/{path...}", s.handleClusterProxy)
	mux.HandleFunc("POST /api/cluster/join", auth.RequireRole(s.handleClusterJoin, store.RoleAdmin))
	mux.HandleFunc("POST /api/cluster/leave", auth.RequireRole(s.handleClusterLeave, store.RoleAdmin))
	mux.HandleFunc("PUT /api/cluster/self", auth.RequireRole(s.handleClusterSelf, store.RoleAdmin))

	// --- Settings / branding ---
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", auth.RequireRole(s.handleUpdateSettings, store.RoleAdmin))
	mux.HandleFunc("POST /api/settings/logo", auth.RequireRole(s.handleUploadLogo, store.RoleAdmin))
	mux.HandleFunc("DELETE /api/settings/logo", auth.RequireRole(s.handleDeleteLogo, store.RoleAdmin))
	mux.HandleFunc("GET /api/branding/logo", s.handleServeLogo)

	// --- Self-update (admin) ---
	mux.HandleFunc("GET /api/updates/status", auth.RequireRole(s.handleUpdateStatus, store.RoleAdmin))
	mux.HandleFunc("POST /api/updates/apply", auth.RequireRole(s.handleApplyUpdate, store.RoleAdmin))

	authMW := &auth.Middleware{
		Store:       s.store,
		PublicPaths: publicPaths,
		// Workflow webhooks: the random token in the path is the credential.
		PublicPrefixes: []string{"POST /api/hooks/"},
		ClusterAuth:    s.clusterAuth,
	}
	return withLogging(withStatic(s.cfg.WebDir, withCORS(s.cfg.PublicHost, authMW.Wrap(mux))))
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
