package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"serverdash/internal/containers"
)

type containerSpec struct {
	Name          string                `json:"name"`
	Image         string                `json:"image"`
	Env           []string              `json:"env,omitempty"`
	Ports         []containers.PortSpec `json:"ports,omitempty"`
	Volumes       []string              `json:"volumes,omitempty"`
	Command       []string              `json:"command,omitempty"`
	RestartPolicy string                `json:"restartPolicy,omitempty"`
	Labels        map[string]string     `json:"labels,omitempty"`
}

type managedDeployment struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Image  string `json:"image"`
	Action string `json:"action"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type managedContainer struct {
	containerSpec
	ID          string              `json:"id"`
	Enabled     bool                `json:"enabled"`
	ContainerID string              `json:"containerId,omitempty"`
	Status      string              `json:"status,omitempty"`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
	DeployedAt  string              `json:"deployedAt,omitempty"`
	History     []managedDeployment `json:"history,omitempty"`
}

var (
	containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
	imageRefPattern      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:/@-]{0,254}$`)
	envKeyPattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	volumePattern        = regexp.MustCompile(`^[^:\s]+:[^:\s]+(:(ro|rw))?$`)
	labelKeyPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	validRestartPolicies = map[string]bool{
		"no": true, "always": true, "unless-stopped": true, "on-failure": true,
	}
)

func (s *Server) requireContainerRuntime(w http.ResponseWriter, r *http.Request) bool {
	if _, err := s.docker.RuntimeInfo(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return false
	}
	return true
}

func (s *Server) requireNativeContainerHost(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.HostRoot != "" {
		writeError(w, http.StatusForbidden, errors.New("ServerDash Containers require a native ServerDash install"))
		return false
	}
	return s.requireContainerRuntime(w, r)
}

func validateContainerSpec(in containerSpec) (containers.CreateOptions, error) {
	name := strings.TrimSpace(in.Name)
	if !containerNamePattern.MatchString(name) {
		return containers.CreateOptions{}, errors.New("name must start with a letter or number and use only letters, numbers, dots, dashes, and underscores")
	}
	image := strings.TrimSpace(in.Image)
	if !imageRefPattern.MatchString(image) {
		return containers.CreateOptions{}, errors.New("image must be a valid image reference")
	}
	env := make([]string, 0, len(in.Env))
	for _, item := range in.Env {
		item = strings.TrimSpace(item)
		key, _, ok := strings.Cut(item, "=")
		if !ok || !envKeyPattern.MatchString(key) {
			return containers.CreateOptions{}, errors.New("environment entries must look like KEY=value")
		}
		env = append(env, item)
	}

	ports := make([]containers.PortSpec, 0, len(in.Ports))
	for _, p := range in.Ports {
		host, containerPort := strings.TrimSpace(p.HostPort), strings.TrimSpace(p.ContainerPort)
		for _, value := range []string{host, containerPort} {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 65535 {
				return containers.CreateOptions{}, errors.New("ports must be numbers between 1 and 65535")
			}
		}
		proto := strings.ToLower(strings.TrimSpace(p.Protocol))
		if proto == "" {
			proto = "tcp"
		}
		if proto != "tcp" && proto != "udp" {
			return containers.CreateOptions{}, errors.New("port protocol must be tcp or udp")
		}
		ports = append(ports, containers.PortSpec{HostPort: host, ContainerPort: containerPort, Protocol: proto})
	}

	volumes := make([]string, 0, len(in.Volumes))
	for _, v := range in.Volumes {
		v = strings.TrimSpace(v)
		if !volumePattern.MatchString(v) {
			return containers.CreateOptions{}, errors.New("volumes must look like /host/path:/container/path[:ro]")
		}
		volumes = append(volumes, v)
	}

	restart := strings.TrimSpace(in.RestartPolicy)
	if restart == "" {
		restart = "unless-stopped"
	}
	if !validRestartPolicies[restart] {
		return containers.CreateOptions{}, errors.New("unknown restart policy")
	}

	labels := map[string]string{"managed.by": "serverdash"}
	for k, v := range in.Labels {
		if !labelKeyPattern.MatchString(k) || len(v) > 200 {
			return containers.CreateOptions{}, errors.New("invalid container label")
		}
		labels[k] = v
	}

	return containers.CreateOptions{
		Name: name, Image: image, Env: env, Ports: ports, Volumes: volumes,
		Command: append([]string(nil), in.Command...), RestartPolicy: restart, Labels: labels,
	}, nil
}

func newManagedID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}

func (s *Server) findContainerByName(r *http.Request, name string) (*containers.Container, error) {
	list, err := s.docker.List(r.Context())
	if err != nil {
		return nil, err
	}
	for i := range list {
		if strings.TrimPrefix(list[i].Name, "/") == name {
			return &list[i], nil
		}
	}
	return nil, nil
}

func managedContainerName(m managedContainer) string {
	return "sd-" + m.Name
}

func (s *Server) persistManaged(r *http.Request, m managedContainer) error {
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.store.SetManagedContainer(r.Context(), m.ID, m.Name, string(raw))
}

func (s *Server) loadAllManaged(r *http.Request) ([]managedContainer, error) {
	raw, err := s.store.ListManagedContainers(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]managedContainer, 0, len(raw))
	for _, item := range raw {
		var m managedContainer
		if json.Unmarshal([]byte(item), &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Server) loadManaged(r *http.Request, id string) (managedContainer, bool, error) {
	raw, found, err := s.store.GetManagedContainer(r.Context(), id)
	if err != nil || !found {
		return managedContainer{}, found, err
	}
	var m managedContainer
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return managedContainer{}, false, err
	}
	return m, true, nil
}

func (s *Server) enrichManaged(r *http.Request, m managedContainer) managedContainer {
	list, err := s.docker.List(r.Context())
	if err != nil {
		return m
	}
	for _, c := range list {
		if c.ID == m.ContainerID || strings.TrimPrefix(c.Name, "/") == managedContainerName(m) {
			m.ContainerID = c.ID
			m.Status = c.State
			return m
		}
	}
	if m.Status != "error" {
		m.Status = "missing"
	}
	return m
}

func (s *Server) handleListManagedContainers(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	items, err := s.loadAllManaged(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for i := range items {
		items[i] = s.enrichManaged(r, items[i])
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateManagedContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	var in containerSpec
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := validateContainerSpec(in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	m := managedContainer{
		containerSpec: in,
		ID:            newManagedID(),
		Enabled:       true,
		Status:        "draft",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.persistManaged(r, m); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleUpdateManagedContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	m, found, err := s.loadManaged(r, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("managed container not found"))
		return
	}
	var in containerSpec
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := validateContainerSpec(in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	m.containerSpec = in
	if err := s.persistManaged(r, m); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteManagedContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	m, found, err := s.loadManaged(r, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("managed container not found"))
		return
	}
	if r.URL.Query().Get("removeContainer") == "true" {
		list, err := s.docker.List(r.Context())
		if err == nil {
			for _, c := range list {
				if c.ID == m.ContainerID || strings.TrimPrefix(c.Name, "/") == managedContainerName(m) {
					_ = s.docker.Remove(r.Context(), c.ID, true)
					break
				}
			}
		}
	}
	if err := s.store.DeleteManagedContainer(r.Context(), m.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deployManaged(r *http.Request, m *managedContainer) error {
	spec, err := validateContainerSpec(m.containerSpec)
	if err != nil {
		return err
	}
	if err := s.docker.PullImage(r.Context(), spec.Image); err != nil {
		return err
	}
	if m.ContainerID != "" {
		_ = s.docker.Remove(r.Context(), m.ContainerID, true)
	} else if existing, err := s.findContainerByName(r, managedContainerName(*m)); err == nil && existing != nil {
		_ = s.docker.Remove(r.Context(), existing.ID, true)
	}

	spec.Name = managedContainerName(*m)
	spec.Labels["serverdash.managed.id"] = m.ID
	id, err := s.docker.Create(r.Context(), spec)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	m.ContainerID = id
	m.Status = "running"
	m.DeployedAt = now
	m.Enabled = true
	m.History = append(m.History, managedDeployment{
		ID: newManagedID(), At: now, Image: spec.Image, Action: "deploy", Status: "success",
	})
	return s.persistManaged(r, *m)
}

func (s *Server) handleDeployManagedContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	m, found, err := s.loadManaged(r, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("managed container not found"))
		return
	}
	if err := s.deployManaged(r, &m); err != nil {
		now := time.Now().UTC().Format(time.RFC3339)
		m.Status = "error"
		m.History = append(m.History, managedDeployment{
			ID: newManagedID(), At: now, Image: m.Image, Action: "deploy", Status: "error", Error: err.Error(),
		})
		_ = s.persistManaged(r, m)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleManagedContainerAction(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	m, found, err := s.loadManaged(r, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("managed container not found"))
		return
	}
	action := r.PathValue("action")
	if action == "start" && (m.ContainerID == "" || m.Status == "missing") {
		if err := s.deployManaged(r, &m); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deployed"})
		return
	}
	switch action {
	case "start":
		err = s.docker.Start(r.Context(), m.ContainerID)
	case "stop":
		err = s.docker.Stop(r.Context(), m.ContainerID)
	case "restart":
		err = s.docker.Restart(r.Context(), m.ContainerID)
	default:
		writeError(w, http.StatusBadRequest, errors.New("unknown managed container action"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	switch action {
	case "start", "restart":
		m.Status = "running"
	case "stop":
		m.Status = "exited"
	}
	_ = s.persistManaged(r, m)
	writeJSON(w, http.StatusOK, map[string]string{"status": m.Status})
}

func (s *Server) handleRollbackManagedContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireNativeContainerHost(w, r) {
		return
	}
	m, found, err := s.loadManaged(r, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("managed container not found"))
		return
	}
	previous := ""
	for i := len(m.History) - 1; i >= 0; i-- {
		entry := m.History[i]
		if entry.Status == "success" && entry.Image != "" && entry.Image != m.Image {
			previous = entry.Image
			break
		}
	}
	if previous == "" {
		writeError(w, http.StatusBadRequest, errors.New("no earlier image to roll back to"))
		return
	}
	m.Image = previous
	if err := s.deployManaged(r, &m); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	if !s.requireContainerRuntime(w, r) {
		return
	}
	images, err := s.docker.ListImages(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, images)
}

func (s *Server) handlePullImage(w http.ResponseWriter, r *http.Request) {
	if !s.requireContainerRuntime(w, r) {
		return
	}
	var req struct {
		Image string `json:"image"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !imageRefPattern.MatchString(strings.TrimSpace(req.Image)) {
		writeError(w, http.StatusBadRequest, errors.New("image must be a valid image reference"))
		return
	}
	if err := s.docker.PullImage(r.Context(), strings.TrimSpace(req.Image)); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pulled", "image": req.Image})
}

func (s *Server) handleRemoveImage(w http.ResponseWriter, r *http.Request) {
	if !s.requireContainerRuntime(w, r) {
		return
	}
	if err := s.docker.RemoveImage(r.Context(), r.PathValue("id"), r.URL.Query().Get("force") == "true"); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireContainerRuntime(w, r) {
		return
	}
	var in containerSpec
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	spec, err := validateContainerSpec(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if existing, err := s.findContainerByName(r, spec.Name); err == nil && existing != nil {
		writeError(w, http.StatusConflict, errors.New("a container with that name already exists"))
		return
	}
	id, err := s.docker.Create(r.Context(), spec)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "created"})
}

func (s *Server) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	if !s.requireContainerRuntime(w, r) {
		return
	}
	if err := s.docker.Remove(r.Context(), r.PathValue("id"), r.URL.Query().Get("force") == "true"); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
