// Package containers talks to the Docker/Podman Engine API to list and
// control containers. Podman's socket is Docker Engine API-compatible, so
// the same client works against either runtime unmodified.
package containers

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Container struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Created int64             `json:"created"`
	Ports   []Port            `json:"ports"`
	Labels  map[string]string `json:"labels"`
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
