// Command serverdash runs ServerDash's Go backend: it collects host metrics,
// controls Docker/Podman containers over the Engine API and Kubernetes pods
// via kubectl, runs scheduled automation/scripts, and exposes all of it
// through an authenticated HTTP + WebSocket API for the web dashboard.
package main

import (
	"context"
	"log"
	"net/http"
	_ "time/tzdata" // embeds the IANA zone database in the binary: the Alpine base image doesn't ship it, and without this, a workflow's "CRON_TZ=America/Chicago ..." trigger silently fails to register

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

	log.Printf("serverdash: data (users, sessions, nicknames, settings, scripts, workflows) stored at %s", cfg.DBPath)
	if cfg.DBPath == "./serverdash.db" {
		log.Printf("serverdash: WARNING: using the default relative DB path — set SERVERDASH_DB_PATH to a location on a" +
			" persistent volume (docker-compose.yml's serverdash-data volume does this for you), or all data is lost" +
			" whenever this container is recreated, including by an update")
	}

	kubeClient := kubernetes.New(cfg.Kubeconfig)

	runner := automation.NewRunner(db, dockerClient, cfg.ComposeCmd)
	runner.Start(context.Background())
	defer runner.Stop()

	srv := api.New(cfg, dockerClient, db, kubeClient, runner)

	log.Printf("serverdash API listening on %s (container runtime: %s, kubernetes: %v, public host: %s)",
		cfg.ListenAddr, cfg.DockerHost, kubernetes.Available(), cfg.PublicHost)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Routes()); err != nil {
		log.Fatalf("serverdash: %v", err)
	}
}
