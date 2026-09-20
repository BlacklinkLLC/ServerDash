// Package scripts executes user-authored shell scripts and records their
// output. This is deliberately arbitrary code execution — gated at the API
// layer to admins only — for cases the built-in automations don't cover.
package scripts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"serverdash/internal/store"
)

// maxRuntime bounds a single script run so a hung or runaway script can't
// occupy the scheduler indefinitely.
const maxRuntime = 10 * time.Minute

// Run executes content as a shell script and returns its exit code and
// combined stdout+stderr. A non-zero exit code is reported via the return
// value, not err — err is reserved for failures to start the process at
// all.
func Run(ctx context.Context, content string) (exitCode int, output string, err error) {
	ctx, cancel := context.WithTimeout(ctx, maxRuntime)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", content)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	output = buf.String()
	if runErr == nil {
		return 0, output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode(), output, nil
	}
	return -1, output, fmt.Errorf("run script: %w", runErr)
}

// RunAndRecord executes a stored script and persists the result as a
// script_runs row, for the run-history view in the admin panel.
func RunAndRecord(ctx context.Context, st *store.Store, script store.Script, triggeredBy string) error {
	runID, err := st.StartScriptRun(ctx, script.ID, triggeredBy)
	if err != nil {
		return fmt.Errorf("record script run start: %w", err)
	}

	exitCode, output, err := Run(ctx, script.Content)
	if err != nil {
		output = output + "\n" + err.Error()
		exitCode = -1
	}

	return st.FinishScriptRun(ctx, runID, exitCode, output)
}
