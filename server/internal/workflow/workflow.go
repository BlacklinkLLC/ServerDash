// Package workflow implements ServerDash's block-based automation
// language: a small, fixed set of typed steps (git pull, rebuild via
// compose, restart a container, run a shell command, wait) that a workflow
// runs in order on a schedule. It's deliberately not a general scripting
// language — each block is a narrow, named operation, which is what makes
// it presentable as drag-free "blocks" in the UI rather than a code editor.
package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"serverdash/internal/containers"
)

type BlockType string

const (
	BlockGitPull          BlockType = "git_pull"
	BlockComposeUp        BlockType = "compose_up"
	BlockRestartContainer BlockType = "restart_container"
	BlockShell            BlockType = "shell"
	BlockWait             BlockType = "wait"
)

// Block is one step of a workflow. Only the fields relevant to its Type are
// used; the rest are left zero. Kept flat (rather than one struct per type)
// so the whole list round-trips through a single JSON array column.
type Block struct {
	Type      BlockType `json:"type"`
	Dir       string    `json:"dir,omitempty"`       // git_pull, compose_up: working directory
	Build     bool      `json:"build,omitempty"`     // compose_up: pass --build
	Container string    `json:"container,omitempty"` // restart_container: name or ID (prefix match)
	Command   string    `json:"command,omitempty"`   // shell: the command to run
	Seconds   int       `json:"seconds,omitempty"`   // wait: how long to pause
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

func Validate(blocks []Block) error {
	if len(blocks) == 0 {
		return fmt.Errorf("a workflow needs at least one block")
	}
	for i, b := range blocks {
		switch b.Type {
		case BlockGitPull, BlockComposeUp:
			if strings.TrimSpace(b.Dir) == "" {
				return fmt.Errorf("block %d (%s): dir is required", i+1, b.Type)
			}
		case BlockRestartContainer:
			if strings.TrimSpace(b.Container) == "" {
				return fmt.Errorf("block %d (%s): container is required", i+1, b.Type)
			}
		case BlockShell:
			if strings.TrimSpace(b.Command) == "" {
				return fmt.Errorf("block %d (%s): command is required", i+1, b.Type)
			}
		case BlockWait:
			if b.Seconds <= 0 {
				return fmt.Errorf("block %d (%s): seconds must be positive", i+1, b.Type)
			}
		default:
			return fmt.Errorf("block %d: unknown block type %q", i+1, b.Type)
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
)

// Executor runs blocks against this ServerDash instance's container
// runtime and shell.
type Executor struct {
	Docker     *containers.Client
	ComposeCmd []string // e.g. []string{"docker", "compose"} or []string{"podman-compose"}
}

// Run executes blocks in order, stopping at the first failure. It always
// returns a human-readable log of what happened, and ok reports whether
// every block succeeded.
func (e *Executor) Run(ctx context.Context, blocks []Block) (log string, ok bool) {
	ctx, cancel := context.WithTimeout(ctx, maxTotalRuntime)
	defer cancel()

	var buf bytes.Buffer
	for i, b := range blocks {
		fmt.Fprintf(&buf, "▶ %d/%d %s\n", i+1, len(blocks), describeBlock(b))

		out, err := e.runBlock(ctx, b)
		buf.WriteString(out)
		if !strings.HasSuffix(out, "\n") && out != "" {
			buf.WriteString("\n")
		}
		if err != nil {
			fmt.Fprintf(&buf, "✗ failed: %v\n", err)
			return buf.String(), false
		}
	}
	buf.WriteString("✓ workflow completed\n")
	return buf.String(), true
}

func describeBlock(b Block) string {
	switch b.Type {
	case BlockGitPull:
		return fmt.Sprintf("git_pull dir=%s", b.Dir)
	case BlockComposeUp:
		return fmt.Sprintf("compose_up dir=%s build=%v", b.Dir, b.Build)
	case BlockRestartContainer:
		return fmt.Sprintf("restart_container container=%s", b.Container)
	case BlockShell:
		return "shell " + b.Command
	case BlockWait:
		return fmt.Sprintf("wait %ds", b.Seconds)
	default:
		return string(b.Type)
	}
}

func (e *Executor) runBlock(ctx context.Context, b Block) (string, error) {
	switch b.Type {
	case BlockGitPull:
		// ServerDash's container runs as root while a bind-mounted project
		// directory is typically host-user-owned, which git's ownership
		// check refuses by default ("dubious ownership"). Scoped to this
		// one directory for this one invocation, rather than a blanket
		// `safe.directory = *` — the latter would trust every git repo
		// this process ever touches, not just the ones workflows name.
		return runCommand(ctx, b.Dir, "git", "-c", "safe.directory="+b.Dir, "-C", b.Dir, "pull")

	case BlockComposeUp:
		args := append(append([]string{}, e.ComposeCmd[1:]...), "up", "-d")
		if b.Build {
			args = append(args, "--build")
		}
		return runCommand(ctx, b.Dir, e.ComposeCmd[0], args...)

	case BlockRestartContainer:
		id, err := e.resolveContainer(ctx, b.Container)
		if err != nil {
			return "", err
		}
		if err := e.Docker.Restart(ctx, id); err != nil {
			return "", err
		}
		return "restarted " + id[:min(12, len(id))], nil

	case BlockShell:
		return runCommand(ctx, b.Dir, "/bin/sh", "-c", b.Command)

	case BlockWait:
		select {
		case <-time.After(time.Duration(b.Seconds) * time.Second):
			return fmt.Sprintf("waited %ds", b.Seconds), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}

	default:
		return "", fmt.Errorf("unknown block type %q", b.Type)
	}
}

func (e *Executor) resolveContainer(ctx context.Context, ref string) (string, error) {
	list, err := e.Docker.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list containers: %w", err)
	}
	for _, c := range list {
		if c.Name == ref || c.ID == ref || strings.HasPrefix(c.ID, ref) {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("no container matching %q", ref)
}

func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, maxBlockRuntime)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%s: %w", name, err)
	}
	return out.String(), nil
}
