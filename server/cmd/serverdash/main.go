// Command serverdash runs ServerDash's Go backend: it collects host metrics
// and controls Docker/Podman containers over the Engine API, exposing both
// through an HTTP + WebSocket API for the web dashboard.
package main

import (
	"log"
	"net/http"

	"serverdash/internal/api"
	"serverdash/internal/config"
	"serverdash/internal/containers"
)

func main() {
	cfg := config.Load()

	dockerClient, err := containers.New(cfg.DockerHost)
	if err != nil {
		log.Fatalf("serverdash: %v", err)
	}
	defer dockerClient.Close()

	srv := api.New(cfg, dockerClient)

	log.Printf("serverdash API listening on %s (container runtime: %s, public host: %s)",
		cfg.ListenAddr, cfg.DockerHost, cfg.PublicHost)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Routes()); err != nil {
		log.Fatalf("serverdash: %v", err)
	}
}
