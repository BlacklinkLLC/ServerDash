package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

type Deployment struct {
	Context   string `json:"context"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
	Ready     int32  `json:"ready"`
	Available int32  `json:"available"`
	Image     string `json:"image"`
}

type rawDeploymentList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Replicas int32 `json:"replicas"`
			Template struct {
				Spec struct {
					Containers []struct {
						Image string `json:"image"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
		Status struct {
			ReadyReplicas     int32 `json:"readyReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	} `json:"items"`
}

func (c *Client) ListDeployments(ctx context.Context, kubeContext, namespace string) ([]Deployment, error) {
	if kubeContext != "" && !validName(kubeContext) {
		return nil, fmt.Errorf("invalid context name")
	}
	if namespace != "" && !validName(namespace) {
		return nil, fmt.Errorf("invalid namespace")
	}

	args := []string{"get", "deployments", "-o", "json"}
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
	var raw rawDeploymentList
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse kubectl output: %w", err)
	}

	items := make([]Deployment, 0, len(raw.Items))
	for _, item := range raw.Items {
		image := ""
		if len(item.Spec.Template.Spec.Containers) > 0 {
			image = item.Spec.Template.Spec.Containers[0].Image
		}
		items = append(items, Deployment{
			Context:   kubeContext,
			Namespace: item.Metadata.Namespace,
			Name:      item.Metadata.Name,
			Replicas:  item.Spec.Replicas,
			Ready:     item.Status.ReadyReplicas,
			Available: item.Status.AvailableReplicas,
			Image:     image,
		})
	}
	return items, nil
}

func (c *Client) ScaleDeployment(ctx context.Context, kubeContext, namespace, name string, replicas int32) error {
	if !validName(namespace) || !validName(name) {
		return fmt.Errorf("invalid namespace or deployment name")
	}
	if replicas < 0 || replicas > 1000 {
		return fmt.Errorf("replicas must be between 0 and 1000")
	}
	args := []string{"scale", "deployment", name, "--replicas", strconv.FormatInt(int64(replicas), 10), "-n", namespace}
	if kubeContext != "" {
		if !validName(kubeContext) {
			return fmt.Errorf("invalid context name")
		}
		args = append([]string{"--context", kubeContext}, args...)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) RestartDeployment(ctx context.Context, kubeContext, namespace, name string) error {
	if !validName(namespace) || !validName(name) {
		return fmt.Errorf("invalid namespace or deployment name")
	}
	args := []string{"rollout", "restart", "deployment", name, "-n", namespace}
	if kubeContext != "" {
		if !validName(kubeContext) {
			return fmt.Errorf("invalid context name")
		}
		args = append([]string{"--context", kubeContext}, args...)
	}
	_, err := c.run(ctx, args...)
	return err
}

type Service struct {
	Context   string   `json:"context"`
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	ClusterIP string   `json:"clusterIp"`
	Ports     []string `json:"ports"`
}

type rawServiceList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Type      string `json:"type"`
			ClusterIP string `json:"clusterIP"`
			Ports     []struct {
				Port     int32  `json:"port"`
				Protocol string `json:"protocol"`
			} `json:"ports"`
		} `json:"spec"`
	} `json:"items"`
}

func (c *Client) ListServices(ctx context.Context, kubeContext, namespace string) ([]Service, error) {
	if kubeContext != "" && !validName(kubeContext) {
		return nil, fmt.Errorf("invalid context name")
	}
	if namespace != "" && !validName(namespace) {
		return nil, fmt.Errorf("invalid namespace")
	}

	args := []string{"get", "services", "-o", "json"}
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
	var raw rawServiceList
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse kubectl output: %w", err)
	}

	items := make([]Service, 0, len(raw.Items))
	for _, item := range raw.Items {
		ports := make([]string, 0, len(item.Spec.Ports))
		for _, p := range item.Spec.Ports {
			port := strconv.FormatInt(int64(p.Port), 10)
			if p.Protocol != "" {
				port += "/" + p.Protocol
			}
			ports = append(ports, port)
		}
		items = append(items, Service{
			Context:   kubeContext,
			Namespace: item.Metadata.Namespace,
			Name:      item.Metadata.Name,
			Type:      item.Spec.Type,
			ClusterIP: item.Spec.ClusterIP,
			Ports:     ports,
		})
	}
	return items, nil
}
