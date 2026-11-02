// Package cluster links ServerDash instances together.
//
// One instance is the primary. It issues single-use join tokens; an
// instance that redeems one becomes a node of that primary. Nodes push a
// heartbeat with a status summary every HeartbeatInterval, which works even
// when the node is behind NAT. If the primary can also reach the node's URL,
// it can proxy dashboard requests to the node, so one browser session
// manages every server.
//
// A node joined with the "replica" role also mirrors the primary's users and
// appearance settings, delivered in heartbeat responses whenever they change,
// so the same logins and branding work on every replica.
package cluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"serverdash/internal/store"
)

const (
	HeartbeatInterval = 15 * time.Second
	// OfflineAfter is how long a primary waits without a heartbeat before
	// showing a node as offline: a few missed beats, not one slow one.
	OfflineAfter = 50 * time.Second

	tokenPrefix = "sdj1."
	// AuthHeader carries "nodeID:secret" on node→primary heartbeats and on
	// primary→node proxied requests.
	AuthHeader = "X-ServerDash-Cluster"
	// RoleHeader tells a node which role the primary's signed-in user has,
	// so proxied requests keep the same permissions.
	RoleHeader = "X-ServerDash-Cluster-Role"
	// UserHeader names that user, for logs on the node.
	UserHeader = "X-ServerDash-Cluster-User"
)

// ReplicatedSettings are the settings keys a replica mirrors. Node-local
// keys (cluster_*, logo files) are deliberately excluded.
var ReplicatedSettings = []string{
	"app_name", "accent_color", "theme", "density", "refresh_seconds",
	"warn_threshold", "crit_threshold", "overview_widgets", "footer_text",
	"dashboard_show_system", "dashboard_show_containers",
}

type JoinToken struct {
	PrimaryURL string `json:"u"`
	Token      string `json:"t"`
	Role       string `json:"r"`
}

func EncodeToken(t JoinToken) string {
	b, _ := json.Marshal(t)
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(b)
}

func DecodeToken(s string) (JoinToken, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, tokenPrefix) {
		return JoinToken{}, errors.New("that isn't a ServerDash join token (they start with " + tokenPrefix + ")")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, tokenPrefix))
	if err != nil {
		return JoinToken{}, errors.New("join token is corrupted; copy it again")
	}
	var t JoinToken
	if err := json.Unmarshal(raw, &t); err != nil || t.PrimaryURL == "" || t.Token == "" {
		return JoinToken{}, errors.New("join token is corrupted; copy it again")
	}
	return t, nil
}

// NormalizeURL checks that u is an absolute http(s) URL and trims any
// trailing slash, so paths can be appended safely.
func NormalizeURL(u string) (string, error) {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if u == "" {
		return "", nil
	}
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
		return "", fmt.Errorf("%q must be a full address like https://server.example.com", u)
	}
	return u, nil
}

func SecretsEqual(a, b string) bool {
	return len(a) > 0 && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Status is the summary a node reports in each heartbeat.
type Status struct {
	Hostname          string  `json:"hostname"`
	Platform          string  `json:"platform"`
	CPUPercent        float64 `json:"cpu"`
	MemPercent        float64 `json:"mem"`
	DiskPercent       float64 `json:"disk"`
	Load1             float64 `json:"load1"`
	UptimeSeconds     uint64  `json:"uptime"`
	Cores             int     `json:"cores"`
	ContainersRunning int     `json:"containersRunning"`
	ContainersTotal   int     `json:"containersTotal"`
	Deployment        string  `json:"deployment"`
	SyncedAt          string  `json:"syncedAt,omitempty"`
}

type Heartbeat struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Status   Status `json:"status"`
	SyncHash string `json:"syncHash"`
}

// Bundle is what a replica mirrors from its primary.
type Bundle struct {
	Users    []store.ReplicatedUser `json:"users"`
	Settings map[string]string      `json:"settings"`
}

func (b Bundle) Hash() string {
	j, _ := json.Marshal(b)
	sum := sha256.Sum256(j)
	return hex.EncodeToString(sum[:])
}

type HeartbeatResponse struct {
	Role        string  `json:"role"`
	PrimaryName string  `json:"primaryName"`
	Bundle      *Bundle `json:"bundle,omitempty"`
	BundleHash  string  `json:"bundleHash,omitempty"`
}

type RegisterRequest struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

type RegisterResponse struct {
	NodeID      string `json:"nodeId"`
	Secret      string `json:"secret"`
	Role        string `json:"role"`
	PrimaryName string `json:"primaryName"`
}

// BuildBundle collects the users and replicated settings a replica mirrors.
func BuildBundle(ctx context.Context, st *store.Store) (Bundle, error) {
	users, err := st.ExportUsers(ctx)
	if err != nil {
		return Bundle{}, err
	}
	b := Bundle{Users: users, Settings: map[string]string{}}
	for _, k := range ReplicatedSettings {
		if v, ok, _ := st.GetSetting(ctx, k); ok {
			b.Settings[k] = v
		}
	}
	return b, nil
}

// --- node side ---

// Membership is this instance's own cluster configuration, persisted in
// the settings table.
type Membership struct {
	Mode        string `json:"mode"` // "standalone" | "primary" | "node"
	PrimaryURL  string `json:"primaryUrl,omitempty"`
	PrimaryName string `json:"primaryName,omitempty"`
	NodeID      string `json:"nodeId,omitempty"`
	Role        string `json:"role,omitempty"`
	Name        string `json:"name,omitempty"`
	SelfURL     string `json:"selfUrl,omitempty"`
	secret      string
}

func LoadMembership(ctx context.Context, st *store.Store) Membership {
	get := func(k string) string { v, _, _ := st.GetSetting(ctx, k); return v }
	m := Membership{
		Mode:        get("cluster_mode"),
		PrimaryURL:  get("cluster_primary_url"),
		PrimaryName: get("cluster_primary_name"),
		NodeID:      get("cluster_node_id"),
		Role:        get("cluster_role"),
		Name:        get("cluster_node_name"),
		SelfURL:     get("cluster_self_url"),
		secret:      get("cluster_secret"),
	}
	if m.Mode == "" {
		m.Mode = "standalone"
	}
	return m
}

func (m Membership) Secret() string { return m.secret }

func saveMembership(ctx context.Context, st *store.Store, m Membership) error {
	for k, v := range map[string]string{
		"cluster_mode": m.Mode, "cluster_primary_url": m.PrimaryURL, "cluster_primary_name": m.PrimaryName,
		"cluster_node_id": m.NodeID, "cluster_role": m.Role, "cluster_node_name": m.Name,
		"cluster_self_url": m.SelfURL, "cluster_secret": m.secret,
	} {
		if err := st.SetSetting(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

// SetMode switches between standalone and primary. Leaving node mode
// goes through Leave instead.
func SetMode(ctx context.Context, st *store.Store, mode string) error {
	return st.SetSetting(ctx, "cluster_mode", mode)
}

// AgentState is the node's view of its link to the primary.
type AgentState struct {
	LastHeartbeat string `json:"lastHeartbeat,omitempty"`
	LastSync      string `json:"lastSync,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

// Agent runs on a node, sending heartbeats and applying replica bundles.
type Agent struct {
	store   *store.Store
	collect func(context.Context) Status
	client  *http.Client

	mu    sync.Mutex
	state AgentState
	wake  chan struct{}
}

func NewAgent(st *store.Store, collect func(context.Context) Status) *Agent {
	return &Agent{store: st, collect: collect, client: &http.Client{Timeout: 15 * time.Second}, wake: make(chan struct{}, 1)}
}

func (a *Agent) State() AgentState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// Kick sends a heartbeat now instead of waiting for the next tick, e.g.
// right after joining.
func (a *Agent) Kick() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func (a *Agent) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(HeartbeatInterval)
		defer t.Stop()
		for {
			if m := LoadMembership(ctx, a.store); m.Mode == "node" {
				a.beat(ctx, m)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-a.wake:
			}
		}
	}()
}

func (a *Agent) setErr(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.state.LastError = err.Error()
	} else {
		a.state.LastError = ""
		a.state.LastHeartbeat = time.Now().Format(time.RFC3339)
	}
}

func (a *Agent) beat(ctx context.Context, m Membership) {
	syncHash, _, _ := a.store.GetSetting(ctx, "cluster_sync_hash")
	status := a.collect(ctx)
	status.SyncedAt = a.State().LastSync
	hb := Heartbeat{Name: m.Name, URL: m.SelfURL, Status: status, SyncHash: syncHash}
	body, _ := json.Marshal(hb)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, m.PrimaryURL+"/api/cluster/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(AuthHeader, m.NodeID+":"+m.secret)
	resp, err := a.client.Do(req)
	if err != nil {
		a.setErr(fmt.Errorf("can't reach the primary: %w", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		a.setErr(errors.New("the primary no longer recognizes this node (it was removed); leave the cluster and join again"))
		return
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		a.setErr(fmt.Errorf("primary answered %d: %s", resp.StatusCode, strings.TrimSpace(string(msg))))
		return
	}
	var hr HeartbeatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&hr); err != nil {
		a.setErr(fmt.Errorf("bad heartbeat response: %w", err))
		return
	}
	if hr.Role != m.Role || hr.PrimaryName != m.PrimaryName {
		_ = a.store.SetSetting(ctx, "cluster_role", hr.Role)
		_ = a.store.SetSetting(ctx, "cluster_primary_name", hr.PrimaryName)
	}
	if hr.Role == "replica" && hr.Bundle != nil {
		if err := a.apply(ctx, *hr.Bundle, hr.BundleHash); err != nil {
			a.setErr(fmt.Errorf("replication failed: %w", err))
			return
		}
	}
	a.setErr(nil)
}

func (a *Agent) apply(ctx context.Context, b Bundle, hash string) error {
	if err := a.store.ReplaceUsers(ctx, b.Users); err != nil {
		return err
	}
	for _, k := range ReplicatedSettings {
		if v, ok := b.Settings[k]; ok {
			if err := a.store.SetSetting(ctx, k, v); err != nil {
				return err
			}
		} else {
			_ = a.store.DeleteSetting(ctx, k)
		}
	}
	if err := a.store.SetSetting(ctx, "cluster_sync_hash", hash); err != nil {
		return err
	}
	a.mu.Lock()
	a.state.LastSync = time.Now().Format(time.RFC3339)
	a.mu.Unlock()
	log.Printf("cluster: replicated %d users and appearance settings from the primary", len(b.Users))
	return nil
}

// Join redeems a join token with its primary and stores the resulting
// membership. selfURL is how the primary should reach this node (may be
// empty: heartbeats still work, but the primary can't proxy to it).
func (a *Agent) Join(ctx context.Context, token, name, selfURL string) (Membership, error) {
	jt, err := DecodeToken(token)
	if err != nil {
		return Membership{}, err
	}
	primary, err := NormalizeURL(jt.PrimaryURL)
	if err != nil {
		return Membership{}, err
	}
	if selfURL, err = NormalizeURL(selfURL); err != nil {
		return Membership{}, err
	}
	if cur := LoadMembership(ctx, a.store); cur.Mode == "primary" {
		nodes, _ := a.store.ClusterNodes(ctx)
		if len(nodes) > 0 {
			return Membership{}, errors.New("this server is a primary with nodes of its own; remove them before joining another cluster")
		}
	}

	body, _ := json.Marshal(RegisterRequest{Token: jt.Token, Name: name, URL: selfURL})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, primary+"/api/cluster/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return Membership{}, fmt.Errorf("couldn't reach the primary at %s: %w", primary, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		return Membership{}, fmt.Errorf("the primary refused to add this node: %s", e.Error)
	}
	var rr RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return Membership{}, err
	}
	m := Membership{Mode: "node", PrimaryURL: primary, PrimaryName: rr.PrimaryName, NodeID: rr.NodeID, Role: rr.Role, Name: name, SelfURL: selfURL, secret: rr.Secret}
	if err := saveMembership(ctx, a.store, m); err != nil {
		return Membership{}, err
	}
	_ = a.store.DeleteSetting(ctx, "cluster_sync_hash")
	a.mu.Lock()
	a.state = AgentState{}
	a.mu.Unlock()
	a.Kick()
	return m, nil
}

// Leave forgets the primary. Replicated users and settings stay as they
// are, so the server keeps working on its own.
func (a *Agent) Leave(ctx context.Context) error {
	if err := saveMembership(ctx, a.store, Membership{Mode: "standalone"}); err != nil {
		return err
	}
	_ = a.store.DeleteSetting(ctx, "cluster_sync_hash")
	a.mu.Lock()
	a.state = AgentState{}
	a.mu.Unlock()
	return nil
}

// UpdateSelf changes how this node describes itself to the primary.
func (a *Agent) UpdateSelf(ctx context.Context, name, selfURL string) error {
	u, err := NormalizeURL(selfURL)
	if err != nil {
		return err
	}
	if err := a.store.SetSetting(ctx, "cluster_node_name", name); err != nil {
		return err
	}
	if err := a.store.SetSetting(ctx, "cluster_self_url", u); err != nil {
		return err
	}
	a.Kick()
	return nil
}
