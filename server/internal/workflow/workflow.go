// Package workflow implements ServerDash's block-based automation
// language: a fixed set of typed steps (git pull, rebuild via compose,
// container and service control, HTTP calls, notifications, checks, ...)
// that a workflow runs in order when its trigger fires. It's deliberately
// not a general scripting language: each block is a narrow, named
// operation, which is what makes it presentable as "blocks" in the UI
// rather than a code editor. The shell block is the escape hatch.
package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"serverdash/internal/containers"
	"serverdash/internal/scripts"
	"serverdash/internal/store"
	"serverdash/internal/system"
	"serverdash/internal/systemd"
)

type BlockType string

const (
	BlockGitPull          BlockType = "git_pull"
	BlockComposeUp        BlockType = "compose_up"
	BlockRestartContainer BlockType = "restart_container"
	BlockStartContainer   BlockType = "start_container"
	BlockStopContainer    BlockType = "stop_container"
	BlockShell            BlockType = "shell"
	BlockWait             BlockType = "wait"
	BlockHTTP             BlockType = "http_request"
	BlockNotify           BlockType = "notify"
	BlockService          BlockType = "service_action"
	BlockRunScript        BlockType = "run_script"
	BlockPrune            BlockType = "prune"
	BlockCheck            BlockType = "check"
)

// Check kinds for the check block ("only continue if ...").
const (
	CheckCPUAbove         = "cpu_above"
	CheckCPUBelow         = "cpu_below"
	CheckMemAbove         = "mem_above"
	CheckDiskAbove        = "disk_above"
	CheckContainerRunning = "container_running"
	CheckContainerStopped = "container_stopped"
	CheckHTTPOK           = "http_ok"
	CheckCommandOK        = "command_ok"
	CheckOutputContains   = "output_contains"
)

// Block is one step of a workflow. Only the fields relevant to its Type are
// used; the rest are left zero. Kept flat (rather than one struct per type)
// so the whole list round-trips through a single JSON array column.
type Block struct {
	Type      BlockType `json:"type"`
	Dir       string    `json:"dir,omitempty"`       // git_pull, compose_up, shell: working directory
	Build     bool      `json:"build,omitempty"`     // compose_up: pass --build
	Container string    `json:"container,omitempty"` // *_container, check: name or ID (prefix match)
	Command   string    `json:"command,omitempty"`   // shell, check(command_ok): the command to run
	Seconds   int       `json:"seconds,omitempty"`   // wait: how long to pause

	Method       string `json:"method,omitempty"`       // http_request
	URL          string `json:"url,omitempty"`          // http_request, notify, check(http_ok)
	Body         string `json:"body,omitempty"`         // http_request
	ExpectStatus int    `json:"expectStatus,omitempty"` // http_request: 0 = any 2xx

	Format  string `json:"format,omitempty"`  // notify: discord | slack | ntfy | json
	Message string `json:"message,omitempty"` // notify

	Service string `json:"service,omitempty"` // service_action
	Action  string `json:"action,omitempty"`  // service_action: start | stop | restart | reload

	ScriptID int64 `json:"scriptId,omitempty"` // run_script

	Check     string  `json:"check,omitempty"`     // check: one of the Check* kinds
	Threshold float64 `json:"threshold,omitempty"` // check: percent for the *_above/below kinds
	Text      string  `json:"text,omitempty"`      // check(output_contains)
	OnFalse   string  `json:"onFalse,omitempty"`   // check: "stop" (default, run still succeeds) | "fail"

	// ContinueOnError keeps the workflow going if this block fails; the
	// run is still reported as failed at the end.
	ContinueOnError bool   `json:"continueOnError,omitempty"`
	Label           string `json:"label,omitempty"` // optional display name in logs
}

func ParseBlocks(raw string) ([]Block, error) {
	if raw == "" {
		return nil, nil
	}
	var blocks []Block
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		return nil, fmt.Errorf("parse blocks: %w", err)
	}
	return blocks, nil
}

var validNotifyFormats = map[string]bool{"discord": true, "slack": true, "ntfy": true, "json": true}
var validChecks = map[string]bool{
	CheckCPUAbove: true, CheckCPUBelow: true, CheckMemAbove: true, CheckDiskAbove: true,
	CheckContainerRunning: true, CheckContainerStopped: true, CheckHTTPOK: true,
	CheckCommandOK: true, CheckOutputContains: true,
}

func requireURL(i int, b Block) error {
	if !strings.HasPrefix(b.URL, "http://") && !strings.HasPrefix(b.URL, "https://") {
		return fmt.Errorf("block %d (%s): url must start with http:// or https://", i+1, b.Type)
	}
	return nil
}

func Validate(blocks []Block) error {
	if len(blocks) == 0 {
		return fmt.Errorf("a workflow needs at least one block")
	}
	if len(blocks) > 50 {
		return fmt.Errorf("a workflow can have at most 50 blocks")
	}
	for i, b := range blocks {
		req := func(field, v string) error {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("block %d (%s): %s is required", i+1, b.Type, field)
			}
			return nil
		}
		var err error
		switch b.Type {
		case BlockGitPull, BlockComposeUp:
			err = req("dir", b.Dir)
		case BlockRestartContainer, BlockStartContainer, BlockStopContainer:
			err = req("container", b.Container)
		case BlockShell:
			err = req("command", b.Command)
		case BlockWait:
			if b.Seconds <= 0 || b.Seconds > 3600 {
				err = fmt.Errorf("block %d (%s): seconds must be between 1 and 3600", i+1, b.Type)
			}
		case BlockHTTP:
			err = requireURL(i, b)
			if err == nil && b.Method != "" && !regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE|HEAD)$`).MatchString(b.Method) {
				err = fmt.Errorf("block %d (%s): unsupported method %q", i+1, b.Type, b.Method)
			}
		case BlockNotify:
			err = requireURL(i, b)
			if err == nil {
				err = req("message", b.Message)
			}
			if err == nil && b.Format != "" && !validNotifyFormats[b.Format] {
				err = fmt.Errorf("block %d (%s): format must be discord, slack, ntfy, or json", i+1, b.Type)
			}
		case BlockService:
			err = req("service", b.Service)
			if err == nil && b.Action != "start" && b.Action != "stop" && b.Action != "restart" && b.Action != "reload" {
				err = fmt.Errorf("block %d (%s): action must be start, stop, restart, or reload", i+1, b.Type)
			}
		case BlockRunScript:
			if b.ScriptID <= 0 {
				err = fmt.Errorf("block %d (%s): choose a script", i+1, b.Type)
			}
		case BlockPrune:
		case BlockCheck:
			if !validChecks[b.Check] {
				err = fmt.Errorf("block %d (%s): unknown check %q", i+1, b.Type, b.Check)
				break
			}
			switch b.Check {
			case CheckContainerRunning, CheckContainerStopped:
				err = req("container", b.Container)
			case CheckHTTPOK:
				err = requireURL(i, b)
			case CheckCommandOK:
				err = req("command", b.Command)
			case CheckOutputContains:
				err = req("text", b.Text)
			}
		default:
			err = fmt.Errorf("block %d: unknown block type %q", i+1, b.Type)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// maxTotalRuntime bounds an entire workflow run; maxBlockRuntime bounds any
// single external command within it, so one hung git remote or compose
// build can't wedge the run forever.
const (
	maxTotalRuntime = 30 * time.Minute
	maxBlockRuntime = 10 * time.Minute
	maxCapture      = 8 << 10 // bytes of an HTTP response kept for {{output}}
)

// Executor runs blocks against this ServerDash instance's container
// runtime, shell, and services.
type Executor struct {
	Docker     *containers.Client
	ComposeCmd []string // e.g. []string{"docker", "compose"} or []string{"podman-compose"}
	Store      *store.Store
	HostRoot   string
	HTTP       *http.Client
}

// Context describes why a run is happening; its fields are available to
// blocks as {{variables}}.
type Context struct {
	Workflow string
	Trigger  string // e.g. "schedule", "manual", "webhook", "event"
	Event    string // human-readable detail for event triggers, e.g. "CPU at 93%"
	Payload  string // webhook request body
}

// runState threads values between blocks.
type runState struct {
	rc     Context
	output string // last block's output, as {{output}}
	failed []string
}

// ErrStop is returned by a check block whose condition isn't met: the run
// ends early, successfully.
type stopRun struct{ reason string }

func (s stopRun) Error() string { return s.reason }

// Run executes blocks in order. It always returns a human-readable log of
// what happened; ok reports whether every block succeeded (blocks marked
// continueOnError still count as failures).
func (e *Executor) Run(ctx context.Context, blocks []Block, rc Context) (log string, ok bool) {
	ctx, cancel := context.WithTimeout(ctx, maxTotalRuntime)
	defer cancel()

	st := &runState{rc: rc}
	var buf bytes.Buffer
	if rc.Event != "" {
		fmt.Fprintf(&buf, "⚡ triggered by %s: %s\n", rc.Trigger, rc.Event)
	}
	for i, b := range blocks {
		fmt.Fprintf(&buf, "▶ %d/%d %s\n", i+1, len(blocks), describeBlock(b))
		start := time.Now()

		out, err := e.runBlock(ctx, b, st)
		buf.WriteString(out)
		if !strings.HasSuffix(out, "\n") && out != "" {
			buf.WriteString("\n")
		}
		if stop, isStop := err.(stopRun); isStop {
			fmt.Fprintf(&buf, "■ stopped: %s\n", stop.reason)
			buf.WriteString("✓ workflow completed (stopped early by a check)\n")
			return buf.String(), len(st.failed) == 0
		}
		if err != nil {
			fmt.Fprintf(&buf, "✗ failed after %s: %v\n", time.Since(start).Round(time.Millisecond), err)
			st.failed = append(st.failed, fmt.Sprintf("block %d", i+1))
			if b.ContinueOnError {
				buf.WriteString("  continuing (continue on error is on)\n")
				continue
			}
			return buf.String(), false
		}
		st.output = out
	}
	if len(st.failed) > 0 {
		fmt.Fprintf(&buf, "✗ workflow finished with failures in %s\n", strings.Join(st.failed, ", "))
		return buf.String(), false
	}
	buf.WriteString("✓ workflow completed\n")
	return buf.String(), true
}

func describeBlock(b Block) string {
	if b.Label != "" {
		return b.Label
	}
	switch b.Type {
	case BlockGitPull:
		return fmt.Sprintf("git pull in %s", b.Dir)
	case BlockComposeUp:
		return fmt.Sprintf("compose up in %s (build=%v)", b.Dir, b.Build)
	case BlockRestartContainer, BlockStartContainer, BlockStopContainer:
		return fmt.Sprintf("%s %s", strings.TrimSuffix(string(b.Type), "_container"), b.Container)
	case BlockShell:
		return "shell: " + b.Command
	case BlockWait:
		return fmt.Sprintf("wait %ds", b.Seconds)
	case BlockHTTP:
		return fmt.Sprintf("%s %s", methodOr(b.Method), b.URL)
	case BlockNotify:
		return fmt.Sprintf("notify (%s)", formatOr(b.Format))
	case BlockService:
		return fmt.Sprintf("%s service %s", b.Action, b.Service)
	case BlockRunScript:
		return fmt.Sprintf("run script #%d", b.ScriptID)
	case BlockPrune:
		return "prune unused containers, images, and volumes"
	case BlockCheck:
		return "only continue if " + describeCheck(b)
	default:
		return string(b.Type)
	}
}

func describeCheck(b Block) string {
	switch b.Check {
	case CheckCPUAbove:
		return fmt.Sprintf("CPU > %.0f%%", b.Threshold)
	case CheckCPUBelow:
		return fmt.Sprintf("CPU < %.0f%%", b.Threshold)
	case CheckMemAbove:
		return fmt.Sprintf("memory > %.0f%%", b.Threshold)
	case CheckDiskAbove:
		return fmt.Sprintf("any disk > %.0f%%", b.Threshold)
	case CheckContainerRunning:
		return b.Container + " is running"
	case CheckContainerStopped:
		return b.Container + " is stopped"
	case CheckHTTPOK:
		return b.URL + " responds OK"
	case CheckCommandOK:
		return "`" + b.Command + "` succeeds"
	case CheckOutputContains:
		return fmt.Sprintf("the previous output contains %q", b.Text)
	}
	return b.Check
}

func methodOr(m string) string {
	if m == "" {
		return "GET"
	}
	return m
}

func formatOr(f string) string {
	if f == "" {
		return "json"
	}
	return f
}

// expand fills {{variables}} in user-supplied text.
func (e *Executor) expand(s string, st *runState) string {
	return e.expandWith(s, st, func(v string) string { return v })
}

// expandShell fills {{variables}} in a shell command, single-quoting each
// value so event text or a webhook payload is always data, never code:
// `echo {{payload}}` can't be turned into `echo; rm -rf /`.
func (e *Executor) expandShell(s string, st *runState) string {
	return e.expandWith(s, st, shellQuote)
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

func (e *Executor) expandWith(s string, st *runState, quote func(string) string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	host, _ := os.Hostname()
	vars := []string{
		"{{workflow}}", st.rc.Workflow,
		"{{trigger}}", st.rc.Trigger,
		"{{event}}", st.rc.Event,
		"{{payload}}", st.rc.Payload,
		"{{output}}", strings.TrimSpace(st.output),
		"{{host}}", host,
		"{{time}}", time.Now().Format(time.RFC1123),
		"{{date}}", time.Now().Format("2006-01-02"),
	}
	if strings.Contains(s, "{{cpu}}") || strings.Contains(s, "{{mem}}") || strings.Contains(s, "{{disk}}") || strings.Contains(s, "{{load}}") {
		if snap, err := system.Collect(e.HostRoot); err == nil {
			vars = append(vars,
				"{{cpu}}", fmt.Sprintf("%.0f%%", snap.CPUPercent),
				"{{mem}}", fmt.Sprintf("%.0f%%", snap.MemUsedPercent),
				"{{disk}}", fmt.Sprintf("%.0f%%", maxDisk(snap)),
				"{{load}}", fmt.Sprintf("%.2f", snap.LoadAvg1),
			)
		}
	}
	for i := 1; i < len(vars); i += 2 {
		vars[i] = quote(vars[i])
	}
	return strings.NewReplacer(vars...).Replace(s)
}

func maxDisk(snap system.Snapshot) float64 {
	m := 0.0
	for _, d := range snap.Disks {
		if d.UsedPercent > m {
			m = d.UsedPercent
		}
	}
	return m
}

func (e *Executor) runBlock(ctx context.Context, b Block, st *runState) (string, error) {
	switch b.Type {
	case BlockGitPull:
		// ServerDash's container runs as root while a bind-mounted project
		// directory is typically host-user-owned, which git's ownership
		// check refuses by default ("dubious ownership"). Scoped to this
		// one directory for this one invocation, rather than a blanket
		// `safe.directory = *`.
		return runCommand(ctx, b.Dir, nil, "git", "-c", "safe.directory="+b.Dir, "-C", b.Dir, "pull")

	case BlockComposeUp:
		args := append(append([]string{}, e.ComposeCmd[1:]...), "up", "-d")
		if b.Build {
			args = append(args, "--build")
		}
		return runCommand(ctx, b.Dir, nil, e.ComposeCmd[0], args...)

	case BlockRestartContainer, BlockStartContainer, BlockStopContainer:
		id, err := e.resolveContainer(ctx, e.expand(b.Container, st))
		if err != nil {
			return "", err
		}
		short := id[:min(12, len(id))]
		switch b.Type {
		case BlockStartContainer:
			return "started " + short, e.Docker.Start(ctx, id)
		case BlockStopContainer:
			return "stopped " + short, e.Docker.Stop(ctx, id)
		default:
			return "restarted " + short, e.Docker.Restart(ctx, id)
		}

	case BlockShell:
		env := []string{
			"SERVERDASH_WORKFLOW=" + st.rc.Workflow,
			"SERVERDASH_TRIGGER=" + st.rc.Trigger,
			"SERVERDASH_EVENT=" + st.rc.Event,
			"SERVERDASH_PAYLOAD=" + st.rc.Payload,
			"SERVERDASH_OUTPUT=" + st.output,
		}
		return runCommand(ctx, b.Dir, env, "/bin/sh", "-c", e.expandShell(b.Command, st))

	case BlockWait:
		select {
		case <-time.After(time.Duration(b.Seconds) * time.Second):
			return fmt.Sprintf("waited %ds", b.Seconds), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}

	case BlockHTTP:
		status, body, err := e.httpDo(ctx, methodOr(b.Method), e.expand(b.URL, st), e.expand(b.Body, st), "")
		if err != nil {
			return "", err
		}
		if (b.ExpectStatus == 0 && (status < 200 || status > 299)) || (b.ExpectStatus != 0 && status != b.ExpectStatus) {
			return body, fmt.Errorf("got HTTP %d", status)
		}
		return body, nil

	case BlockNotify:
		msg := e.expand(b.Message, st)
		payload, contentType := notifyPayload(formatOr(b.Format), msg, st.rc)
		status, body, err := e.httpDo(ctx, http.MethodPost, e.expand(b.URL, st), payload, contentType)
		if err != nil {
			return "", err
		}
		if status < 200 || status > 299 {
			return body, fmt.Errorf("notification endpoint answered HTTP %d", status)
		}
		return "sent: " + msg, nil

	case BlockService:
		if !systemd.Available() {
			return "", fmt.Errorf("systemd isn't available here (service actions need a native install)")
		}
		name := e.expand(b.Service, st)
		if !strings.HasSuffix(name, ".service") {
			name += ".service"
		}
		if err := systemd.Action(ctx, name, b.Action); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s: done", b.Action, name), nil

	case BlockRunScript:
		sc, err := e.Store.GetScript(ctx, b.ScriptID)
		if err != nil {
			return "", fmt.Errorf("script #%d: %w", b.ScriptID, err)
		}
		code, out, err := scripts.Run(ctx, sc.Content)
		if err != nil {
			return out, err
		}
		if code != 0 {
			return out, fmt.Errorf("script %q exited with code %d", sc.Name, code)
		}
		return out, nil

	case BlockPrune:
		rep, err := e.Docker.Prune(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("reclaimed %s (%d containers, %d images, %d volumes)",
			humanBytes(rep.SpaceReclaimed), len(rep.ContainersDeleted), rep.ImagesDeleted, len(rep.VolumesDeleted)), nil

	case BlockCheck:
		pass, detail, err := e.evalCheck(ctx, b, st)
		if err != nil && b.Check != CheckHTTPOK && b.Check != CheckCommandOK {
			return "", err
		}
		if pass {
			return "condition met: " + detail, nil
		}
		if b.OnFalse == "fail" {
			return "", fmt.Errorf("condition not met: %s", detail)
		}
		return "", stopRun{reason: "condition not met: " + detail}

	default:
		return "", fmt.Errorf("unknown block type %q", b.Type)
	}
}

func (e *Executor) evalCheck(ctx context.Context, b Block, st *runState) (bool, string, error) {
	switch b.Check {
	case CheckCPUAbove, CheckCPUBelow, CheckMemAbove, CheckDiskAbove:
		snap, err := system.Collect(e.HostRoot)
		if err != nil {
			return false, "", err
		}
		switch b.Check {
		case CheckCPUAbove:
			return snap.CPUPercent > b.Threshold, fmt.Sprintf("CPU is %.0f%%", snap.CPUPercent), nil
		case CheckCPUBelow:
			return snap.CPUPercent < b.Threshold, fmt.Sprintf("CPU is %.0f%%", snap.CPUPercent), nil
		case CheckMemAbove:
			return snap.MemUsedPercent > b.Threshold, fmt.Sprintf("memory is %.0f%%", snap.MemUsedPercent), nil
		default:
			d := maxDisk(snap)
			return d > b.Threshold, fmt.Sprintf("fullest disk is %.0f%%", d), nil
		}
	case CheckContainerRunning, CheckContainerStopped:
		list, err := e.Docker.List(ctx)
		if err != nil {
			return false, "", err
		}
		ref := e.expand(b.Container, st)
		for _, c := range list {
			if c.Name == ref || strings.TrimPrefix(c.Name, "/") == ref || strings.HasPrefix(c.ID, ref) {
				running := c.State == "running"
				if b.Check == CheckContainerRunning {
					return running, fmt.Sprintf("%s is %s", ref, c.State), nil
				}
				return !running, fmt.Sprintf("%s is %s", ref, c.State), nil
			}
		}
		return b.Check == CheckContainerStopped, ref + " doesn't exist", nil
	case CheckHTTPOK:
		status, _, err := e.httpDo(ctx, http.MethodGet, e.expand(b.URL, st), "", "")
		if err != nil {
			return false, err.Error(), err
		}
		return status >= 200 && status < 400, fmt.Sprintf("HTTP %d", status), nil
	case CheckCommandOK:
		_, err := runCommand(ctx, "", nil, "/bin/sh", "-c", e.expandShell(b.Command, st))
		if err != nil {
			return false, err.Error(), err
		}
		return true, "exited 0", nil
	case CheckOutputContains:
		return strings.Contains(st.output, b.Text), fmt.Sprintf("looked for %q", b.Text), nil
	}
	return false, "", fmt.Errorf("unknown check %q", b.Check)
}

func notifyPayload(format, msg string, rc Context) (string, string) {
	var v any
	switch format {
	case "discord":
		v = map[string]any{"username": "ServerDash", "content": msg}
	case "slack":
		v = map[string]any{"text": msg}
	case "ntfy":
		return msg, "text/plain; charset=utf-8"
	default:
		host, _ := os.Hostname()
		v = map[string]any{"message": msg, "workflow": rc.Workflow, "trigger": rc.Trigger, "event": rc.Event, "host": host, "time": time.Now().Format(time.RFC3339)}
	}
	b, _ := json.Marshal(v)
	return string(b), "application/json"
}

func (e *Executor) httpDo(ctx context.Context, method, url, body, contentType string) (int, string, error) {
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return 0, "", err
	}
	if contentType == "" && body != "" {
		contentType = "application/json"
		if !json.Valid([]byte(body)) {
			contentType = "text/plain; charset=utf-8"
		}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", "ServerDash-Workflow/1")
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxCapture))
	return resp.StatusCode, fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, strings.TrimSpace(string(data))), nil
}

func (e *Executor) resolveContainer(ctx context.Context, ref string) (string, error) {
	list, err := e.Docker.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list containers: %w", err)
	}
	for _, c := range list {
		if c.Name == ref || strings.TrimPrefix(c.Name, "/") == ref || c.ID == ref || strings.HasPrefix(c.ID, ref) {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("no container matching %q", ref)
}

func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, maxBlockRuntime)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%s: %w", name, err)
	}
	return out.String(), nil
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// SendAlert posts a failure notification for a run. Errors are logged by
// the caller's log only; an alert failing must never mask the run result.
func (e *Executor) SendAlert(ctx context.Context, a Alert, workflowName string, rc Context, runLog string) {
	tail := runLog
	if len(tail) > 1200 {
		tail = "…" + tail[len(tail)-1200:]
	}
	host, _ := os.Hostname()
	msg := fmt.Sprintf("❌ Workflow \"%s\" failed on %s (trigger: %s)\n```\n%s\n```", workflowName, host, rc.Trigger, strings.TrimSpace(tail))
	if a.Format == "ntfy" || a.Format == "json" {
		msg = fmt.Sprintf("Workflow \"%s\" failed on %s (trigger: %s)\n%s", workflowName, host, rc.Trigger, strings.TrimSpace(tail))
	}
	payload, ct := notifyPayload(formatOr(a.Format), msg, rc)
	_, _, _ = e.httpDo(ctx, http.MethodPost, a.URL, payload, ct)
}
