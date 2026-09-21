// Package containers talks to the Docker/Podman Engine API to list and
// control containers. Podman's socket is Docker Engine API-compatible, so
// the same client works against either runtime unmodified.
package containers

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type Container struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Nickname string            `json:"nickname,omitempty"`
	Image    string            `json:"image"`
	State    string            `json:"state"`
	Status   string            `json:"status"`
	Created  int64             `json:"created"`
	Ports    []Port            `json:"ports"`
	Labels   map[string]string `json:"labels"`
}

// Unhealthy reports whether the Engine API's human-readable status string
// indicates a failing healthcheck — it's the only place this shows up in
// the container list API (there's no separate structured health field).
func (c Container) Unhealthy() bool {
	return strings.Contains(c.Status, "(unhealthy)")
}

type Port struct {
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort"`
	Type        string `json:"type"`
}

type Client struct {
	cli *client.Client
}

func New(host string) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to container runtime at %s: %w", host, err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) List(ctx context.Context) ([]Container, error) {
	raw, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	out := make([]Container, 0, len(raw))
	for _, rc := range raw {
		name := rc.ID[:12]
		if len(rc.Names) > 0 {
			name = rc.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		ports := make([]Port, 0, len(rc.Ports))
		for _, p := range rc.Ports {
			ports = append(ports, Port{PrivatePort: p.PrivatePort, PublicPort: p.PublicPort, Type: p.Type})
		}

		out = append(out, Container{
			ID:      rc.ID,
			Name:    name,
			Image:   rc.Image,
			State:   rc.State,
			Status:  rc.Status,
			Created: rc.Created,
			Ports:   ports,
			Labels:  rc.Labels,
		})
	}
	return out, nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Stop(ctx context.Context, id string) error {
	if err := c.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("stop container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Restart(ctx context.Context, id string) error {
	if err := c.cli.ContainerRestart(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("restart container %s: %w", id, err)
	}
	return nil
}

// Logs returns a stream of the container's combined stdout/stderr. The
// caller is responsible for closing it.
func (c *Client) Logs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       "200",
		Timestamps: true,
	})
	if err != nil {
		return nil, fmt.Errorf("stream logs for %s: %w", id, err)
	}
	return rc, nil
}

// Exec starts an interactive shell inside a container and returns the
// hijacked, bidirectional connection: writes go to the shell's stdin,
// reads come from its combined stdout/stderr (merged, since Tty is true).
// The caller closes the returned connection when done. Tries bash first,
// falling back to sh — most minimal images (alpine et al.) only have the
// latter, but bash is worth preferring where it exists.
func (c *Client) Exec(ctx context.Context, id string) (execID string, conn types.HijackedResponse, err error) {
	cmd := []string{"sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi"}
	created, err := c.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("create exec session for %s: %w", id, err)
	}

	attached, err := c.cli.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("attach exec session for %s: %w", id, err)
	}
	return created.ID, attached, nil
}

func (c *Client) ExecResize(ctx context.Context, execID string, height, width uint) error {
	return c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{Height: height, Width: width})
}

// RuntimeInfo identifies which container runtime is on the other end of
// the socket — Docker and Podman both speak the same Engine API, so this
// is the only way to tell them apart for display purposes.
type RuntimeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (c *Client) RuntimeInfo(ctx context.Context) (RuntimeInfo, error) {
	v, err := c.cli.ServerVersion(ctx)
	if err != nil {
		return RuntimeInfo{}, fmt.Errorf("query runtime version: %w", err)
	}
	name := v.Platform.Name
	if name == "" {
		name = "Docker Engine"
	}
	return RuntimeInfo{Name: name, Version: v.Version}, nil
}

// PruneReport summarizes what a prune pass removed, in bytes reclaimed and
// item counts, across containers, images, and volumes.
type PruneReport struct {
	ContainersDeleted []string `json:"containersDeleted"`
	ImagesDeleted     int      `json:"imagesDeleted"`
	VolumesDeleted    []string `json:"volumesDeleted"`
	SpaceReclaimed    uint64   `json:"spaceReclaimedBytes"`
}

// Prune removes stopped containers, dangling images, and unused volumes —
// the same scope as `docker system prune`, minus networks (rarely worth
// the churn) and minus the --all image flag (keeps images backing any
// stopped-but-not-removed container available for a quick restart).
func (c *Client) Prune(ctx context.Context) (PruneReport, error) {
	var report PruneReport

	cp, err := c.cli.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		return report, fmt.Errorf("prune containers: %w", err)
	}
	report.ContainersDeleted = cp.ContainersDeleted
	report.SpaceReclaimed += cp.SpaceReclaimed

	ip, err := c.cli.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return report, fmt.Errorf("prune images: %w", err)
	}
	report.ImagesDeleted = len(ip.ImagesDeleted)
	report.SpaceReclaimed += ip.SpaceReclaimed

	vp, err := c.cli.VolumesPrune(ctx, filters.Args{})
	if err != nil {
		return report, fmt.Errorf("prune volumes: %w", err)
	}
	report.VolumesDeleted = vp.VolumesDeleted
	report.SpaceReclaimed += vp.SpaceReclaimed

	return report, nil
}
