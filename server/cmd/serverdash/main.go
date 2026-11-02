// Command serverdash runs ServerDash's Go backend: it collects host metrics,
// controls Docker/Podman containers over the Engine API and Kubernetes pods
// via kubectl, runs scheduled automation/scripts, and exposes all of it
// through an authenticated HTTP + WebSocket API for the web dashboard.
package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"time"
	_ "time/tzdata" // embeds the IANA zone database in the binary: the Alpine base image doesn't ship it, and without this, a workflow's "CRON_TZ=America/Chicago ..." trigger silently fails to register

	"serverdash/internal/api"
	"serverdash/internal/automation"
	"serverdash/internal/cluster"
	"serverdash/internal/config"
	"serverdash/internal/containers"
	"serverdash/internal/kubernetes"
	"serverdash/internal/plugins"
	"serverdash/internal/store"
	"serverdash/internal/system"
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

	pluginEnv := []string{"SERVERDASH_DB_PATH=" + cfg.DBPath, "SERVERDASH_DATA_DIR=" + cfg.DataDir}
	if cfg.HostRoot != "" {
		pluginEnv = append(pluginEnv, "SERVERDASH_HOST_ROOT="+cfg.HostRoot)
	}
	pluginManager := plugins.New(db, cfg.PluginsDir, filepath.Join(cfg.DataDir, "plugins"), pluginEnv)
	log.Printf("serverdash: %d plugins found (bundled: %q, yours: %q)", len(pluginManager.List()), pluginManager.BundledDir, pluginManager.UserDir)

	runner := automation.NewRunner(db, dockerClient, cfg.ComposeCmd, cfg.HostRoot)
	runner.SetPlugins(pluginManager)
	runner.Start(context.Background())
	defer runner.Stop()

	// 5s samples × 720 = one hour of chart history for the overview.
	sampler := system.NewSampler(5*time.Second, 720)
	sampler.Start(context.Background())

	var srv *api.Server
	agent := cluster.NewAgent(db, func(ctx context.Context) cluster.Status { return srv.CollectStatus(ctx) })
	srv = api.New(cfg, dockerClient, db, kubeClient, runner, sampler, agent, pluginManager)
	// Started only once srv exists: the first heartbeat runs immediately
	// and calls back into it for this server's status.
	agent.Start(context.Background())

	log.Printf("serverdash API listening on %s (container runtime: %s, kubernetes: %v, public host: %s)",
		cfg.ListenAddr, cfg.DockerHost, kubernetes.Available(), cfg.PublicHost)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Routes()); err != nil {
		log.Fatalf("serverdash: %v", err)
	}
}
