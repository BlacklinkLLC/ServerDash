// Package plugins loads ServerDash plugins: folders with a plugin.json that
// declare dashboard widgets and action buttons backed by shell commands.
//
// A plugin runs with ServerDash's own privileges, so only admins can enable
// or configure one. Every plugin starts disabled.
package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"serverdash/internal/store"
)

const (
	widgetTimeout = 20 * time.Second
	actionTimeout = 5 * time.Minute
	maxOutput     = 256 << 10
	minInterval   = 10
)

var (
	idRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,40}$`)
	keyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,40}$`)
)

// Setting is one value an admin fills in. It reaches the plugin's commands
// as the environment variable SETTING_<KEY>.
type Setting struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text | textarea | number | checkbox | password | select
	Default     string   `json:"default,omitempty"`
	Help        string   `json:"help,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"` // select
}

// Widget is a panel filled by a command's output.
//
//   - stats: {"items":[{"label":"…","value":"…","hint":"…","status":"ok|warn|crit"}]}
//   - table: {"columns":["…"],"rows":[["…"]],"status":[ "ok|warn|crit", … ]}
//   - text:  plain stdout
type Widget struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Format   string `json:"format"`
	Command  string `json:"command,omitempty"`
	Interval int    `json:"interval"` // seconds between refreshes
	Wide     bool   `json:"wide,omitempty"`
}

// Action is a button that runs a command.
type Action struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Command string `json:"command,omitempty"`
	Confirm string `json:"confirm,omitempty"` // ask before running, with this text
	Role    string `json:"role,omitempty"`    // admin (default) | operator
	Danger  bool   `json:"danger,omitempty"`
}

// ErrForbidden is returned when the caller's role can't run an action.
var ErrForbidden = errors.New("you don't have permission to run this action")

type Manifest struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Author      string    `json:"author,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	Requires    []string  `json:"requires,omitempty"` // commands that must be on PATH
	Settings    []Setting `json:"settings,omitempty"`
	Widgets     []Widget  `json:"widgets,omitempty"`
	Actions     []Action  `json:"actions,omitempty"`
}

// State is what an admin has chosen for a plugin; stored in the settings table.
type State struct {
	Enabled   bool              `json:"enabled"`
	Dashboard bool              `json:"dashboard"`
	Settings  map[string]string `json:"settings"`
}

type Plugin struct {
	Manifest
	Dir     string   `json:"-"`
	Source  string   `json:"source"`  // bundled | user
	Missing []string `json:"missing"` // required commands not found
}

// Result is a widget's latest output.
type Result struct {
	Format    string          `json:"format"`
	Data      json.RawMessage `json:"data,omitempty"`
	Text      string          `json:"text,omitempty"`
	Error     string          `json:"error,omitempty"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type cacheEntry struct {
	mu  sync.Mutex
	res *Result
}

type Manager struct {
	BundledDir string
	UserDir    string
	Env        []string // extra environment for every command

	store   *store.Store
	mu      sync.RWMutex
	plugins map[string]*Plugin
	cache   sync.Map // "id/widget" -> *cacheEntry
}

func New(st *store.Store, bundledDir, userDir string, env []string) *Manager {
	m := &Manager{BundledDir: bundledDir, UserDir: userDir, Env: env, store: st}
	if userDir != "" {
		_ = os.MkdirAll(userDir, 0o755)
	}
	m.Reload()
	return m
}

// Reload rescans both plugin folders. A user plugin with the same ID as a
// bundled one replaces it.
func (m *Manager) Reload() []error {
	found := map[string]*Plugin{}
	var errs []error
	for _, src := range []struct{ dir, name string }{{m.BundledDir, "bundled"}, {m.UserDir, "user"}} {
		if src.dir == "" {
			continue
		}
		entries, err := os.ReadDir(src.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(src.dir, e.Name())
			p, err := load(dir)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", dir, err))
				log.Printf("plugins: skipping %s: %v", dir, err)
				continue
			}
			p.Source = src.name
			found[p.ID] = p
		}
	}
	m.mu.Lock()
	m.plugins = found
	m.mu.Unlock()
	m.cache.Range(func(k, _ any) bool { m.cache.Delete(k); return true })
	return errs
}

func load(dir string) (*Plugin, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var mf Manifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return nil, fmt.Errorf("plugin.json: %w", err)
	}
	if err := mf.validate(); err != nil {
		return nil, err
	}
	p := &Plugin{Manifest: mf, Dir: dir, Missing: []string{}}
	for _, c := range mf.Requires {
		if _, err := exec.LookPath(c); err != nil {
			p.Missing = append(p.Missing, c)
		}
	}
	return p, nil
}

func (mf *Manifest) validate() error {
	if !idRe.MatchString(mf.ID) {
		return fmt.Errorf("id %q must be 2-41 lowercase letters, digits or dashes", mf.ID)
	}
	if strings.TrimSpace(mf.Name) == "" {
		return errors.New("name is required")
	}
	seen := map[string]bool{}
	for i := range mf.Settings {
		s := &mf.Settings[i]
		if !keyRe.MatchString(s.Key) {
			return fmt.Errorf("setting key %q is invalid", s.Key)
		}
		if s.Type == "" {
			s.Type = "text"
		}
	}
	for i := range mf.Widgets {
		w := &mf.Widgets[i]
		if !idRe.MatchString(w.ID) || seen["w"+w.ID] {
			return fmt.Errorf("widget id %q is invalid or repeated", w.ID)
		}
		seen["w"+w.ID] = true
		switch w.Format {
		case "stats", "table", "text":
		case "":
			w.Format = "text"
		default:
			return fmt.Errorf("widget %s: format must be stats, table or text", w.ID)
		}
		if strings.TrimSpace(w.Command) == "" {
			return fmt.Errorf("widget %s: command is required", w.ID)
		}
		if w.Interval < minInterval {
			w.Interval = 60
		}
	}
	for i := range mf.Actions {
		a := &mf.Actions[i]
		if !idRe.MatchString(a.ID) || seen["a"+a.ID] {
			return fmt.Errorf("action id %q is invalid or repeated", a.ID)
		}
		seen["a"+a.ID] = true
		if strings.TrimSpace(a.Command) == "" {
			return fmt.Errorf("action %s: command is required", a.ID)
		}
		if a.Role != "operator" {
			a.Role = "admin"
		}
	}
	return nil
}

func (m *Manager) List() []*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *Manager) Get(id string) (*Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[id]
	return p, ok
}

func stateKey(id string) string { return "plugin:" + id }

func (m *Manager) State(ctx context.Context, id string) State {
	st := State{Settings: map[string]string{}}
	raw, ok, err := m.store.GetSetting(ctx, stateKey(id))
	if err == nil && ok {
		_ = json.Unmarshal([]byte(raw), &st)
		if st.Settings == nil {
			st.Settings = map[string]string{}
		}
	}
	return st
}

func (m *Manager) SetState(ctx context.Context, id string, st State) error {
	p, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no plugin %q", id)
	}
	clean := map[string]string{}
	for _, s := range p.Settings {
		if v, ok := st.Settings[s.Key]; ok {
			if len(v) > 8<<10 {
				return fmt.Errorf("%s is too long", s.Label)
			}
			clean[s.Key] = v
		}
	}
	st.Settings = clean
	raw, _ := json.Marshal(st)
	if err := m.store.SetSetting(ctx, stateKey(id), string(raw)); err != nil {
		return err
	}
	// New settings make cached widget output stale.
	for _, w := range p.Widgets {
		m.cache.Delete(id + "/" + w.ID)
	}
	return nil
}

func (m *Manager) env(p *Plugin, st State) []string {
	env := append(os.Environ(), m.Env...)
	env = append(env, "PLUGIN_DIR="+p.Dir, "PLUGIN_ID="+p.ID)
	if m.UserDir != "" {
		data := filepath.Join(filepath.Dir(m.UserDir), "plugin-data", p.ID)
		_ = os.MkdirAll(data, 0o755)
		env = append(env, "PLUGIN_DATA="+data)
	}
	for _, s := range p.Settings {
		v, ok := st.Settings[s.Key]
		if !ok {
			v = s.Default
		}
		env = append(env, "SETTING_"+strings.ToUpper(s.Key)+"="+v)
	}
	return env
}

func (m *Manager) run(ctx context.Context, p *Plugin, st State, command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = p.Dir
	cmd.Env = m.env(p, st)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limited{w: &stdout, n: maxOutput}
	cmd.Stderr = &limited{w: &stderr, n: 16 << 10}
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), fmt.Errorf("timed out after %s", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String() + stderr.String(), errors.New(lastLines(msg, 6))
	}
	return stdout.String(), nil
}

// Widget returns a widget's output, re-running its command when the cached
// result is older than the widget's interval (or force is set).
func (m *Manager) Widget(ctx context.Context, id, widgetID string, force bool) (*Result, error) {
	p, ok := m.Get(id)
	if !ok {
		return nil, fmt.Errorf("no plugin %q", id)
	}
	st := m.State(ctx, id)
	if !st.Enabled {
		return nil, fmt.Errorf("plugin %s is disabled", p.Name)
	}
	var w *Widget
	for i := range p.Widgets {
		if p.Widgets[i].ID == widgetID {
			w = &p.Widgets[i]
		}
	}
	if w == nil {
		return nil, fmt.Errorf("no widget %q", widgetID)
	}
	v, _ := m.cache.LoadOrStore(id+"/"+widgetID, &cacheEntry{})
	ce := v.(*cacheEntry)
	ce.mu.Lock()
	defer ce.mu.Unlock()
	if !force && ce.res != nil && time.Since(ce.res.UpdatedAt) < time.Duration(w.Interval)*time.Second {
		return ce.res, nil
	}
	// Detached from the request: a closed browser tab shouldn't kill a
	// half-finished command whose result other viewers are waiting on.
	out, err := m.run(context.WithoutCancel(ctx), p, st, w.Command, widgetTimeout)
	res := &Result{Format: w.Format, UpdatedAt: time.Now()}
	switch {
	case err != nil:
		res.Error = err.Error()
	case w.Format == "text":
		res.Text = out
	default:
		trimmed := bytes.TrimSpace([]byte(out))
		if !json.Valid(trimmed) {
			res.Error = "the widget command didn't print valid JSON: " + lastLines(out, 3)
		} else {
			res.Data = trimmed
		}
	}
	ce.res = res
	return res, nil
}

// RunAction runs one of a plugin's actions. role is the caller's role, or
// "admin" for workflows.
func (m *Manager) RunAction(ctx context.Context, id, actionID string, role store.Role) (string, error) {
	p, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("no plugin %q", id)
	}
	st := m.State(ctx, id)
	if !st.Enabled {
		return "", fmt.Errorf("plugin %s is disabled", p.Name)
	}
	for _, a := range p.Actions {
		if a.ID != actionID {
			continue
		}
		if role != store.RoleAdmin && !(a.Role == "operator" && role == store.RoleOperator) {
			return "", ErrForbidden
		}
		out, err := m.run(ctx, p, st, a.Command, actionTimeout)
		// Actions usually change what widgets show.
		for _, w := range p.Widgets {
			m.cache.Delete(id + "/" + w.ID)
		}
		return out, err
	}
	return "", fmt.Errorf("no action %q in %s", actionID, p.Name)
}

type limited struct {
	w *bytes.Buffer
	n int
}

func (l *limited) Write(b []byte) (int, error) {
	if room := l.n - l.w.Len(); room > 0 {
		if len(b) > room {
			l.w.Write(b[:room])
		} else {
			l.w.Write(b)
		}
	}
	return len(b), nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
