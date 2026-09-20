// Package config loads ServerDash's runtime configuration from environment variables.
package config

import "os"

type Config struct {
	// ListenAddr is the address the Go API server binds to, e.g. ":8080".
	ListenAddr string
	// DockerHost is the Docker/Podman Engine API socket to connect to.
	// Podman's socket is API-compatible with Docker's, so the same client works for both.
	DockerHost string
	// PublicHost is the externally reachable hostname the dashboard is served from,
	// used for CORS allow-listing and links generated in API responses.
	PublicHost string
	// HostRoot is where the host's root filesystem is bind-mounted, if at
	// all (e.g. "/rootfs" in the container deployment). Empty when running
	// directly on the host.
	HostRoot string
}

func Load() Config {
	return Config{
		ListenAddr: getEnv("SERVERDASH_LISTEN_ADDR", ":8080"),
		DockerHost: getEnv("SERVERDASH_DOCKER_HOST", "unix:///var/run/docker.sock"),
		PublicHost: getEnv("SERVERDASH_PUBLIC_HOST", "nova.blacklink.net"),
		HostRoot:   getEnv("SERVERDASH_HOST_ROOT", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
