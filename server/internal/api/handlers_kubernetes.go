package api

import (
	"io"
	"net/http"

	"github.com/gorilla/websocket"

	"serverdash/internal/kubernetes"
)

const nicknameRuntimeK8s = "k8s"

func (s *Server) handleK8sStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.kube.Status(r.Context()))
}

func (s *Server) handleK8sListPods(w http.ResponseWriter, r *http.Request) {
	kubeContext := r.URL.Query().Get("context")
	namespace := r.URL.Query().Get("namespace")

	pods, err := s.kube.ListPods(r.Context(), kubeContext, namespace)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	resolvedContext := kubeContext
	if resolvedContext == "" {
		resolvedContext = s.kube.Status(r.Context()).CurrentContext
	}

	nicks, err := s.store.Nicknames(r.Context(), nicknameRuntimeK8s)
	if err == nil {
		type podNickname struct {
			kubernetes.Pod
			Nickname string `json:"nickname,omitempty"`
		}
		out := make([]podNickname, 0, len(pods))
		for _, p := range pods {
			out = append(out, podNickname{Pod: p, Nickname: nicks[k8sNicknameKey(resolvedContext, p.Namespace, p.Name)]})
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	writeJSON(w, http.StatusOK, pods)
}

func (s *Server) handleK8sRestartPod(w http.ResponseWriter, r *http.Request) {
	namespace, name := r.PathValue("namespace"), r.PathValue("name")
	kubeContext := r.URL.Query().Get("context")

	if err := s.kube.DeletePod(r.Context(), kubeContext, namespace, name); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}

func (s *Server) handleK8sPodLogs(w http.ResponseWriter, r *http.Request) {
	namespace, name := r.PathValue("namespace"), r.PathValue("name")
	kubeContext := r.URL.Query().Get("context")

	stream, stop, err := s.kube.StreamLogs(r.Context(), kubeContext, namespace, name)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer stop()

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			if werr := conn.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				_ = conn.WriteMessage(websocket.TextMessage, []byte("\n["+err.Error()+"]\n"))
			}
			return
		}
	}
}

func k8sNicknameKey(kubeContext, namespace, name string) string {
	return kubeContext + "/" + namespace + "/" + name
}

func (s *Server) handleSetK8sNickname(w http.ResponseWriter, r *http.Request) {
	namespace, name := r.PathValue("namespace"), r.PathValue("name")
	kubeContext := r.URL.Query().Get("context")
	if kubeContext == "" {
		kubeContext = s.kube.Status(r.Context()).CurrentContext
	}

	var req nicknameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SetNickname(r.Context(), nicknameRuntimeK8s, k8sNicknameKey(kubeContext, namespace, name), req.Nickname); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteK8sNickname(w http.ResponseWriter, r *http.Request) {
	namespace, name := r.PathValue("namespace"), r.PathValue("name")
	kubeContext := r.URL.Query().Get("context")
	if kubeContext == "" {
		kubeContext = s.kube.Status(r.Context()).CurrentContext
	}
	if err := s.store.DeleteNickname(r.Context(), nicknameRuntimeK8s, k8sNicknameKey(kubeContext, namespace, name)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
