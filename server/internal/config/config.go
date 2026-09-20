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
	// DBPath is where the SQLite database (users, sessions, nicknames,
	// automation rules, scripts) is stored.
	DBPath string
	// Kubeconfig is passed to kubectl as --kubeconfig; empty uses kubectl's
	// own default resolution (KUBECONFIG env, ~/.kube/config, in-cluster).
	Kubeconfig string
}

func Load() Config {
	return Config{
		ListenAddr: getEnv("SERVERDASH_LISTEN_ADDR", ":8080"),
		DockerHost: resolveDockerHost(),
		PublicHost: getEnv("SERVERDASH_PUBLIC_HOST", "nova.blacklink.net"),
		HostRoot:   getEnv("SERVERDASH_HOST_ROOT", ""),
		DBPath:     getEnv("SERVERDASH_DB_PATH", "./serverdash.db"),
		Kubeconfig: getEnv("SERVERDASH_KUBECONFIG", ""),
	}
}

// candidateDockerHosts is tried, in order, when SERVERDASH_DOCKER_HOST
// isn't set explicitly: standard Docker socket first, then Podman's
// rootless and rootful socket locations, so ServerDash works out of the
// box against either runtime without configuration.
func resolveDockerHost() string {
	if v, ok := os.LookupEnv("SERVERDASH_DOCKER_HOST"); ok && v != "" {
		return v
	}

	candidates := []string{"/var/run/docker.sock"}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates, runtimeDir+"/podman/podman.sock")
	}
	candidates = append(candidates, "/run/podman/podman.sock")

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return "unix://" + path
		}
	}
	return "unix:///var/run/docker.sock"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
