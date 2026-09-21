// Package selfupdate lets ServerDash check its own git checkout for
// upstream changes and, on request, pull and redeploy itself.
//
// The tricky part is that ServerDash runs inside the very container it
// would be replacing: a plain `git pull && docker compose up -d --build`
// run as a child of this process gets killed partway through when compose
// tears down the old container, since that child is still in this
// container's own process tree (even under pid: host, it's in this
// container's cgroup). Apply splits the two halves: the git pull runs
// synchronously here (safe — it only touches files, nothing gets torn
// down), then the actual rebuild+recreate is handed off to a detached
// sibling container, launched with `docker run -d`, which is independent
// of this container's lifecycle and keeps running (and finishes the
// recreate) even after this process is gone.
package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Status struct {
	Enabled       bool     `json:"enabled"`
	Branch        string   `json:"branch,omitempty"`
	CurrentCommit string   `json:"currentCommit,omitempty"`
	RemoteCommit  string   `json:"remoteCommit,omitempty"`
	CommitsBehind int      `json:"commitsBehind"`
	Log           []string `json:"log,omitempty"`
	CheckedAt     string   `json:"checkedAt,omitempty"`
	Error         string   `json:"error,omitempty"`
}

const gitTimeout = 30 * time.Second

// safeDir is passed to every git invocation: ServerDash's container runs
// as root while the mounted checkout is typically host-user-owned, which
// git's ownership check refuses by default. Scoped to this one directory
// rather than a blanket `safe.directory = *`.
func gitArgs(dir string, args ...string) []string {
	return append([]string{"-c", "safe.directory=" + dir, "-C", dir}, args...)
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, strings.TrimSpace(out.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// CheckStatus fetches from origin and reports how far the local checkout
// is behind it. dir == "" (SERVERDASH_INSTALL_DIR unset) means the feature
// isn't configured, reported as Enabled: false rather than an error.
func CheckStatus(ctx context.Context, dir string) Status {
	if dir == "" {
		return Status{Enabled: false}
	}
	status := Status{Enabled: true, CheckedAt: time.Now().Format(time.RFC3339)}

	if _, err := os.Stat(dir); err != nil {
		status.Error = fmt.Sprintf("install directory not accessible: %v", err)
		return status
	}

	branch, err := runGit(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Branch = branch

	if _, err := runGit(ctx, dir, "fetch", "--quiet", "origin", branch); err != nil {
		status.Error = err.Error()
		return status
	}

	current, err := runGit(ctx, dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.CurrentCommit = current

	remote, err := runGit(ctx, dir, "rev-parse", "--short", "origin/"+branch)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.RemoteCommit = remote

	countOut, err := runGit(ctx, dir, "rev-list", "--count", "HEAD.."+"origin/"+branch)
	if err == nil {
		status.CommitsBehind, _ = strconv.Atoi(countOut)
	}

	if status.CommitsBehind > 0 {
		logOut, err := runGit(ctx, dir, "log", "--oneline", "--max-count=20", "HEAD.."+"origin/"+branch)
		if err == nil && logOut != "" {
			status.Log = strings.Split(logOut, "\n")
		}
	}

	return status
}

// Apply pulls the latest commit, then hands off the rebuild+restart to a
// detached sibling container so it survives this container being replaced.
// It returns once the pull has succeeded and the rebuild has been handed
// off — not once the rebuild itself finishes, since by then this process
// may no longer exist to report back.
func Apply(ctx context.Context, dir string, composeCmd []string) error {
	if dir == "" {
		return fmt.Errorf("self-update isn't configured (SERVERDASH_INSTALL_DIR is unset)")
	}

	branch, err := runGit(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("determine current branch: %w", err)
	}
	if _, err := runGit(ctx, dir, "pull", "--ff-only", "origin", branch); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	image, err := ownImage(ctx)
	if err != nil {
		return fmt.Errorf("determine this container's image (needed to run the rebuild): %w", err)
	}

	shellCmd := strings.Join(composeCmd, " ") + " up -d --build"
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "docker", "run", "-d", "--rm",
		// On SELinux-enforcing hosts, container policy blocks a container's
		// own nested `docker` CLI calls against a mounted socket — the same
		// restriction documented for workflows' compose_up block. Scoped to
		// only this one-off, purpose-built helper container (not
		// ServerDash's own long-running one), which exists for seconds and
		// only to run this exact privileged rebuild.
		"--security-opt", "label=disable",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", dir+":"+dir,
		"-w", dir,
		"--entrypoint", "sh",
		image,
		"-c", shellCmd,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launch rebuild: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

// ownImage identifies the image this container is running from, so the
// detached rebuild container (which needs git + docker + compose, same as
// this one) doesn't need a separate image pulled just for that one-off job.
func ownImage(ctx context.Context) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}", hostname).Output()
	if err != nil {
		return "", err
	}
	image := strings.TrimSpace(string(out))
	if image == "" {
		return "", fmt.Errorf("empty image reference")
	}
	return image, nil
}
