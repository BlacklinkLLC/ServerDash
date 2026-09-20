// Command serverdash runs ServerDash's Go backend: it collects host metrics,
// controls Docker/Podman containers over the Engine API and Kubernetes pods
// via kubectl, runs scheduled automation/scripts, and exposes all of it
// through an authenticated HTTP + WebSocket API for the web dashboard.
package main

import (
	"context"
	"log"
	"net/http"

	"serverdash/internal/api"
	"serverdash/internal/automation"
	"serverdash/internal/config"
	"serverdash/internal/containers"
	"serverdash/internal/kubernetes"
	"serverdash/internal/store"
)

func main() {
	cfg := config.Load()

	dockerClient, err := containers.New(cfg.DockerHost)
	if err != nil {
		log.Fatalf("serverdash: %v", err)
	}
	defer dockerClient.Close()

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("serverdash: %v", err)
	}
	defer db.Close()

	kubeClient := kubernetes.New(cfg.Kubeconfig)

	runner := automation.NewRunner(db, dockerClient)
	runner.Start(context.Background())
	defer runner.Stop()

	srv := api.New(cfg, dockerClient, db, kubeClient, runner)

	log.Printf("serverdash API listening on %s (container runtime: %s, kubernetes: %v, public host: %s)",
		cfg.ListenAddr, cfg.DockerHost, kubernetes.Available(), cfg.PublicHost)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Routes()); err != nil {
		log.Fatalf("serverdash: %v", err)
	}
}
