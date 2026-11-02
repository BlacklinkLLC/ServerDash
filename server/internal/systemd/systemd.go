// Package systemd manages host services through systemctl and reads the
// system journal through journalctl. It only works when ServerDash runs
// directly on a systemd host (the native install), not inside a container.
package systemd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Unit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Enabled     string `json:"enabled"`
}

// unitName restricts what reaches systemctl/journalctl to plain unit names,
// so a request can never smuggle in an extra flag.
var unitName = regexp.MustCompile(`^[A-Za-z0-9@._:\\-]+\.service$`)

const cmdTimeout = 20 * time.Second

// Available reports whether this process can talk to a running systemd:
// systemctl exists and /run/systemd/system is present (systemd's own
// documented "booted with systemd" check).
func Available() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), errors.New(msg)
	}
	return out.String(), nil
}

// List returns every loaded service unit plus installed-but-inactive ones,
// with each unit's enablement state.
func List(ctx context.Context) ([]Unit, error) {
	out, err := run(ctx, "systemctl", "list-units", "--type=service", "--all", "--plain", "--no-legend", "--no-pager")
	if err != nil {
		return nil, err
	}
	units := map[string]*Unit{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || !strings.HasSuffix(f[0], ".service") {
			continue
		}
		units[f[0]] = &Unit{Name: f[0], Load: f[1], Active: f[2], Sub: f[3], Description: strings.Join(f[4:], " ")}
	}

	files, err := run(ctx, "systemctl", "list-unit-files", "--type=service", "--plain", "--no-legend", "--no-pager")
	if err == nil {
		sc = bufio.NewScanner(strings.NewReader(files))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) < 2 {
				continue
			}
			u, ok := units[f[0]]
			if !ok {
				// Template units (foo@.service) can't be started bare.
				if strings.HasSuffix(f[0], "@.service") {
					continue
				}
				u = &Unit{Name: f[0], Load: "not-loaded", Active: "inactive", Sub: "dead"}
				units[f[0]] = u
			}
			u.Enabled = f[1]
		}
	}

	list := make([]Unit, 0, len(units))
	for _, u := range units {
		list = append(list, *u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

var actions = map[string]bool{"start": true, "stop": true, "restart": true, "reload": true, "enable": true, "disable": true}

// Action runs a systemctl verb against one service. ServerDash's own unit
// can't be stopped or disabled from its own UI (that would strand the
// operator with no dashboard); restarting it is allowed.
func Action(ctx context.Context, name, action string) error {
	if !unitName.MatchString(name) {
		return fmt.Errorf("invalid service name %q", name)
	}
	if !actions[action] {
		return fmt.Errorf("unsupported action %q", action)
	}
	if name == "serverdash.service" && (action == "stop" || action == "disable") {
		return errors.New("refusing to stop or disable ServerDash from its own dashboard")
	}
	_, err := run(ctx, "systemctl", action, "--no-ask-password", name)
	return err
}

type LogEntry struct {
	Time     string `json:"time"`
	Unit     string `json:"unit"`
	Priority int    `json:"priority"`
	Message  string `json:"message"`
}

// Journal returns the most recent lines (up to lines, max 2000) for one
// unit, or the whole system journal when unit is "". priority filters to
// that syslog level and more severe (0 emerg … 7 debug); -1 is no filter.
func Journal(ctx context.Context, unit string, lines, priority int, grep string) ([]LogEntry, error) {
	if lines <= 0 || lines > 2000 {
		lines = 300
	}
	args := []string{"--no-pager", "-o", "json", "-n", strconv.Itoa(lines),
		"--output-fields=MESSAGE,PRIORITY,_SYSTEMD_UNIT,SYSLOG_IDENTIFIER,__REALTIME_TIMESTAMP"}
	if unit != "" {
		if !unitName.MatchString(unit) {
			return nil, fmt.Errorf("invalid service name %q", unit)
		}
		args = append(args, "-u", unit)
	}
	if priority >= 0 && priority <= 7 {
		args = append(args, "-p", strconv.Itoa(priority))
	}
	if grep = strings.TrimSpace(grep); grep != "" {
		args = append(args, "--grep", grep, "--case-sensitive=false")
	}
	out, err := run(ctx, "journalctl", args...)
	if err != nil && out == "" {
		// journalctl exits 1 when --grep matches nothing.
		if grep != "" {
			return []LogEntry{}, nil
		}
		return nil, err
	}
	return parseJournal(out), nil
}
