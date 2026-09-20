// Package kubernetes shells out to the kubectl CLI to list and control
// pods. This deliberately avoids the client-go SDK: kubectl already
// understands whatever kubeconfig/contexts/auth plugins the operator has
// set up, so shelling out reuses that for free instead of reimplementing
// auth plumbing.
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"time"
)

type Client struct {
	kubeconfig string
}

func New(kubeconfig string) *Client {
	return &Client{kubeconfig: kubeconfig}
}

// Available reports whether the kubectl binary is on PATH at all, so the
// API can report Kubernetes support as absent rather than erroring on
// every call when no cluster is configured.
func Available() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

// k8sName matches valid Kubernetes object/context names (RFC 1123 label,
// loosely: contexts additionally allow a handful of punctuation chars used
// in kubeconfig-generated names). Validated before use in exec args as
// defense in depth against argument injection, even though exec.Command
// never invokes a shell.
var k8sName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_.:@-]{0,251}[A-Za-z0-9])?$`)

func validName(s string) bool {
	return s != "" && k8sName.MatchString(s)
}

func (c *Client) baseArgs() []string {
	if c.kubeconfig != "" {
		return []string{"--kubeconfig", c.kubeconfig}
	}
	return nil
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", append(c.baseArgs(), args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl %v: %w: %s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// --- Contexts ---

type Status struct {
	Available      bool     `json:"available"`
	CurrentContext string   `json:"currentContext"`
	Contexts       []string `json:"contexts"`
	Error          string   `json:"error,omitempty"`
}

func (c *Client) Status(ctx context.Context) Status {
	if !Available() {
		return Status{Available: false, Error: "kubectl not found on PATH"}
	}

	out, err := c.run(ctx, "config", "get-contexts", "-o", "name")
	if err != nil {
		return Status{Available: false, Error: err.Error()}
	}
	contexts := []string{}
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		if len(line) > 0 {
			contexts = append(contexts, string(line))
		}
	}
	if len(contexts) == 0 {
		return Status{Available: false, Contexts: contexts, Error: "no kubeconfig contexts found"}
	}

	current, _ := c.run(ctx, "config", "current-context")
	return Status{
		Available:      true,
		CurrentContext: string(bytes.TrimSpace(current)),
		Contexts:       contexts,
	}
}

// --- Pods ---

type Pod struct {
	Context      string `json:"context"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Ready        string `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	Image        string `json:"image"`
	Node         string `json:"node"`
	StartTime    string `json:"startTime"`
}

// rawPodList mirrors just the fields of `kubectl get pods -o json` that
// the dashboard needs.
type rawPodList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			NodeName   string `json:"nodeName"`
			Containers []struct {
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			Phase             string `json:"phase"`
			StartTime         string `json:"startTime"`
			ContainerStatuses []struct {
				Ready        bool  `json:"ready"`
				RestartCount int32 `json:"restartCount"`
			} `json:"containerStatuses"`
		} `json:"status"`
	} `json:"items"`
}

func (c *Client) ListPods(ctx context.Context, kubeContext, namespace string) ([]Pod, error) {
	if kubeContext != "" && !validName(kubeContext) {
		return nil, fmt.Errorf("invalid context name")
	}
	if namespace != "" && !validName(namespace) {
		return nil, fmt.Errorf("invalid namespace")
	}

	args := []string{"get", "pods", "-o", "json"}
	if kubeContext != "" {
		args = append([]string{"--context", kubeContext}, args...)
	}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "-A")
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var raw rawPodList
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse kubectl output: %w", err)
	}

	pods := make([]Pod, 0, len(raw.Items))
	for _, item := range raw.Items {
		ready, total := 0, len(item.Status.ContainerStatuses)
		var restarts int32
		for _, cs := range item.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}
		image := ""
		if len(item.Spec.Containers) > 0 {
			image = item.Spec.Containers[0].Image
		}
		pods = append(pods, Pod{
			Context:      kubeContext,
			Namespace:    item.Metadata.Namespace,
			Name:         item.Metadata.Name,
			Phase:        item.Status.Phase,
			Ready:        fmt.Sprintf("%d/%d", ready, total),
			RestartCount: restarts,
			Image:        image,
			Node:         item.Spec.NodeName,
			StartTime:    item.Status.StartTime,
		})
	}
	return pods, nil
}

// DeletePod deletes the pod, which — for anything managed by a Deployment,
// StatefulSet, DaemonSet etc. — is how you "restart" it: the controller
// recreates it immediately.
func (c *Client) DeletePod(ctx context.Context, kubeContext, namespace, name string) error {
	if !validName(namespace) || !validName(name) {
		return fmt.Errorf("invalid namespace or pod name")
	}
	args := []string{"delete", "pod", name, "-n", namespace}
	if kubeContext != "" {
		if !validName(kubeContext) {
			return fmt.Errorf("invalid context name")
		}
		args = append([]string{"--context", kubeContext}, args...)
	}
	_, err := c.run(ctx, args...)
	return err
}

// StreamLogs runs `kubectl logs -f` and returns its stdout pipe for the
// caller to relay (e.g. over a WebSocket). The returned cancel func must be
// called once the caller is done reading to stop the subprocess.
func (c *Client) StreamLogs(ctx context.Context, kubeContext, namespace, name string) (io.ReadCloser, func(), error) {
	if !validName(namespace) || !validName(name) {
		return nil, nil, fmt.Errorf("invalid namespace or pod name")
	}
	args := []string{"logs", "-f", "--tail=200", name, "-n", namespace}
	if kubeContext != "" {
		if !validName(kubeContext) {
			return nil, nil, fmt.Errorf("invalid context name")
		}
		args = append([]string{"--context", kubeContext}, args...)
	}

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, "kubectl", append(c.baseArgs(), args...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, err
	}

	stop := func() {
		cancel()
		_ = cmd.Wait()
	}
	return stdout, stop, nil
}
