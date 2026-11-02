package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"serverdash/internal/auth"
	"serverdash/internal/cluster"
	"serverdash/internal/store"
	"serverdash/internal/system"
)

// proxyablePrefixes are the API areas a primary may reach on a node: the
// server-scoped views (metrics, containers, pods, tools). Account,
// settings, automation, and cluster endpoints always stay local, so a
// proxied request can never change a node's users or settings.
var proxyablePrefixes = []string{
	"/api/system", "/api/runtime", "/api/host", "/api/containers",
	"/api/k8s", "/api/services", "/api/journal",
}

func proxyable(path string) bool {
	for _, p := range proxyablePrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// clusterAuth authenticates a request a primary proxied to this node.
func (s *Server) clusterAuth(r *http.Request) (store.User, bool, error) {
	h := r.Header.Get(cluster.AuthHeader)
	if h == "" || r.URL.Path == "/api/cluster/heartbeat" {
		return store.User{}, false, nil
	}
	m := cluster.LoadMembership(r.Context(), s.store)
	if m.Mode != "node" || !cluster.SecretsEqual(h, m.NodeID+":"+m.Secret()) {
		return store.User{}, false, errors.New("invalid cluster credentials")
	}
	if !proxyable(r.URL.Path) {
		return store.User{}, false, errors.New("this endpoint isn't available through the cluster")
	}
	role := store.Role(r.Header.Get(cluster.RoleHeader))
	if !role.Valid() {
		role = store.RoleViewer
	}
	name := r.Header.Get(cluster.UserHeader)
	return store.User{ID: -1, Username: "primary/" + name, Role: role}, true, nil
}

// collectStatus is the summary this server reports about itself, both in
// heartbeats (as a node) and for the "this server" card (as a primary).
func (s *Server) collectStatus(ctx context.Context) cluster.Status {
	st := cluster.Status{Deployment: "native"}
	if s.cfg.HostRoot != "" {
		st.Deployment = "container"
	}
	if snap, err := system.Collect(s.cfg.HostRoot); err == nil {
		st.Hostname, st.Platform = snap.Hostname, snap.Platform
		st.CPUPercent, st.MemPercent = snap.CPUPercent, snap.MemUsedPercent
		st.Load1, st.UptimeSeconds, st.Cores = snap.LoadAvg1, snap.UptimeSeconds, snap.CPUCores
		for _, d := range snap.Disks {
			if d.UsedPercent > st.DiskPercent {
				st.DiskPercent = d.UsedPercent
			}
		}
	}
	if list, err := s.docker.List(ctx); err == nil {
		st.ContainersTotal = len(list)
		for _, c := range list {
			if c.State == "running" {
				st.ContainersRunning++
			}
		}
	}
	return st
}

type nodeView struct {
	store.ClusterNode
	Online bool `json:"online"`
}

// handleClusterOverview is readable by every signed-in user: it powers the
// server switcher. Secrets never leave the server.
func (s *Server) handleClusterOverview(w http.ResponseWriter, r *http.Request) {
	m := cluster.LoadMembership(r.Context(), s.store)
	resp := map[string]any{
		"membership": m,
		"self":       s.collectStatus(r.Context()),
	}
	if m.Mode == "node" {
		resp["agent"] = s.agent.State()
	}
	nodes, err := s.store.ClusterNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	views := make([]nodeView, 0, len(nodes))
	for _, n := range nodes {
		views = append(views, nodeView{ClusterNode: n, Online: n.LastSeen != nil && time.Since(*n.LastSeen) < cluster.OfflineAfter})
	}
	resp["nodes"] = views
	writeJSON(w, http.StatusOK, resp)
}

type modeRequest struct {
	Mode string `json:"mode"`
}

func (s *Server) handleClusterMode(w http.ResponseWriter, r *http.Request) {
	var req modeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Mode != "standalone" && req.Mode != "primary" {
		writeError(w, http.StatusBadRequest, errors.New("mode must be standalone or primary (join a cluster to become a node)"))
		return
	}
	m := cluster.LoadMembership(r.Context(), s.store)
	if m.Mode == "node" {
		writeError(w, http.StatusConflict, errors.New("leave the current cluster first"))
		return
	}
	if req.Mode == "standalone" {
		if nodes, _ := s.store.ClusterNodes(r.Context()); len(nodes) > 0 {
			writeError(w, http.StatusConflict, errors.New("remove this primary's nodes first"))
			return
		}
	}
	if err := cluster.SetMode(r.Context(), s.store, req.Mode); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type tokenRequest struct {
	Role       string `json:"role"`
	PrimaryURL string `json:"primaryUrl"`
	TTLHours   int    `json:"ttlHours"`
}

func (s *Server) handleCreateJoinToken(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Role != "node" && req.Role != "replica" {
		writeError(w, http.StatusBadRequest, errors.New("role must be node or replica"))
		return
	}
	primary, err := cluster.NormalizeURL(req.PrimaryURL)
	if err != nil || primary == "" {
		writeError(w, http.StatusBadRequest, errors.New("enter the address nodes should use to reach this server"))
		return
	}
	if req.TTLHours <= 0 || req.TTLHours > 168 {
		req.TTLHours = 1
	}
	m := cluster.LoadMembership(r.Context(), s.store)
	if m.Mode == "node" {
		writeError(w, http.StatusConflict, errors.New("this server is a node; create join tokens on its primary"))
		return
	}
	tok, exp, err := s.store.CreateJoinToken(r.Context(), req.Role, time.Duration(req.TTLHours)*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if m.Mode != "primary" {
		_ = cluster.SetMode(r.Context(), s.store, "primary")
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":     cluster.EncodeToken(cluster.JoinToken{PrimaryURL: primary, Token: tok, Role: req.Role}),
		"expiresAt": exp,
	})
}

// handleClusterRegister is public: the single-use join token is the
// credential.
func (s *Server) handleClusterRegister(w http.ResponseWriter, r *http.Request) {
	var req cluster.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if cluster.LoadMembership(r.Context(), s.store).Mode == "node" {
		writeError(w, http.StatusConflict, errors.New("this server is itself a node, not a primary"))
		return
	}
	role, err := s.store.ConsumeJoinToken(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errors.New("join token is invalid, expired, or already used"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "node"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	nodeURL, err := cluster.NormalizeURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	n := store.ClusterNode{ID: randomHex(8), Name: name, URL: nodeURL, Role: role, Secret: randomHex(32)}
	if err := s.store.CreateClusterNode(r.Context(), n); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = cluster.SetMode(r.Context(), s.store, "primary")
	writeJSON(w, http.StatusCreated, cluster.RegisterResponse{NodeID: n.ID, Secret: n.Secret, Role: role, PrimaryName: s.instanceName(r.Context())})
}

func (s *Server) instanceName(ctx context.Context) string {
	if v, ok, _ := s.store.GetSetting(ctx, "app_name"); ok && v != "" {
		if h, err := os.Hostname(); err == nil {
			return v + " (" + h + ")"
		}
		return v
	}
	h, _ := os.Hostname()
	return h
}

// handleClusterHeartbeat is public at the session layer; the node's
// shared secret authenticates it.
func (s *Server) handleClusterHeartbeat(w http.ResponseWriter, r *http.Request) {
	id, secret, _ := strings.Cut(r.Header.Get(cluster.AuthHeader), ":")
	node, err := s.store.ClusterNode(r.Context(), id)
	if err != nil || !cluster.SecretsEqual(secret, node.Secret) {
		writeError(w, http.StatusUnauthorized, errors.New("unknown node"))
		return
	}
	var hb cluster.Heartbeat
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&hb); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	status, _ := json.Marshal(hb.Status)
	name := strings.TrimSpace(hb.Name)
	if name == "" {
		name = node.Name
	}
	nodeURL, err := cluster.NormalizeURL(hb.URL)
	if err != nil {
		nodeURL = node.URL
	}
	if err := s.store.RecordHeartbeat(r.Context(), node.ID, name, nodeURL, status); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := cluster.HeartbeatResponse{Role: node.Role, PrimaryName: s.instanceName(r.Context())}
	if node.Role == "replica" {
		b, err := cluster.BuildBundle(r.Context(), s.store)
		if err == nil {
			resp.BundleHash = b.Hash()
			if resp.BundleHash != hb.SyncHash {
				resp.Bundle = &b
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type nodeUpdateRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Role string `json:"role"`
}

func (s *Server) handleUpdateClusterNode(w http.ResponseWriter, r *http.Request) {
	var req nodeUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Role != "node" && req.Role != "replica" {
		writeError(w, http.StatusBadRequest, errors.New("role must be node or replica"))
		return
	}
	u, err := cluster.NormalizeURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 60 {
		writeError(w, http.StatusBadRequest, errors.New("name must be 1 to 60 characters"))
		return
	}
	if err := s.store.UpdateClusterNode(r.Context(), r.PathValue("id"), name, u, req.Role); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteClusterNode(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteClusterNode(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleClusterProxy forwards a server-scoped API request (including log
// and terminal WebSockets) to a node, as the signed-in user's role.
func (s *Server) handleClusterProxy(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	node, err := s.store.ClusterNode(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, errors.New("unknown node"))
		return
	}
	if node.URL == "" {
		writeError(w, http.StatusBadGateway, fmt.Errorf("%s has no address set, so it can only report status; set one under Admin → Cluster", node.Name))
		return
	}
	path := "/" + r.PathValue("path")
	if !proxyable(path) {
		writeError(w, http.StatusForbidden, errors.New("this endpoint isn't available through the cluster"))
		return
	}
	// Admin-only endpoints on the node (e.g. the terminal, signals) still
	// require the user to be an admin here: the role is forwarded as-is.
	target, err := url.Parse(node.URL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = strings.TrimRight(target.Path, "/") + path
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			// Never forward this server's session to another server.
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Del("Origin")
			pr.Out.Header.Set(cluster.AuthHeader, node.ID+":"+node.Secret)
			pr.Out.Header.Set(cluster.RoleHeader, string(user.Role))
			pr.Out.Header.Set(cluster.UserHeader, user.Username)
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadGateway, fmt.Errorf("couldn't reach %s at %s: %v", node.Name, node.URL, err))
		},
	}
	proxy.ServeHTTP(w, r)
}

// CollectStatus is collectStatus for the cluster agent's heartbeats.
func (s *Server) CollectStatus(ctx context.Context) cluster.Status { return s.collectStatus(ctx) }

// --- node side ---

type joinRequest struct {
	Token   string `json:"token"`
	Name    string `json:"name"`
	SelfURL string `json:"selfUrl"`
}

func (s *Server) handleClusterJoin(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name, _ = os.Hostname()
	}
	m, err := s.agent.Join(r.Context(), req.Token, name, req.SelfURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleClusterLeave(w http.ResponseWriter, r *http.Request) {
	if err := s.agent.Leave(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClusterSelf(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.agent.UpdateSelf(r.Context(), strings.TrimSpace(req.Name), req.SelfURL); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
