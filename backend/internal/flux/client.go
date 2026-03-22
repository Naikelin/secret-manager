package flux

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// FluxClient interacts with FluxCD resources in Kubernetes
type FluxClient struct {
	dynamicClient dynamic.Interface
	namespace     string // Default namespace for FluxCD (flux-system)
}

// KustomizationStatus represents the status of a FluxCD Kustomization
type KustomizationStatus struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	Ready             bool      `json:"ready"`
	LastAppliedCommit string    `json:"last_applied_commit"`
	LastSyncTime      time.Time `json:"last_sync_time"`
	Message           string    `json:"message,omitempty"`
}

// NewFluxClient creates a new FluxClient
// If kubeconfig is empty, it uses in-cluster configuration
func NewFluxClient(kubeconfig string) (*FluxClient, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// In-cluster config
		config, err = rest.InClusterConfig()
	} else {
		// Out-of-cluster config
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &FluxClient{
		dynamicClient: dynamicClient,
		namespace:     "flux-system", // Default FluxCD namespace
	}, nil
}

// GetKustomizationStatus retrieves the status of a specific Kustomization
func (c *FluxClient) GetKustomizationStatus(name, namespace string) (*KustomizationStatus, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	// Define the Kustomization GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	ctx := context.Background()
	result, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get kustomization %s: %w", name, err)
	}

	return parseKustomizationStatus(result)
}

// ListKustomizations retrieves all Kustomizations in a namespace
func (c *FluxClient) ListKustomizations(namespace string) ([]KustomizationStatus, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	// Define the Kustomization GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	ctx := context.Background()
	list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list kustomizations: %w", err)
	}

	statuses := make([]KustomizationStatus, 0, len(list.Items))
	for _, item := range list.Items {
		status, err := parseKustomizationStatus(&item)
		if err != nil {
			// Log error but continue processing other items
			continue
		}
		statuses = append(statuses, *status)
	}

	return statuses, nil
}

// parseKustomizationStatus extracts status information from an unstructured Kustomization object
func parseKustomizationStatus(obj *unstructured.Unstructured) (*KustomizationStatus, error) {
	status := &KustomizationStatus{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Extract status fields
	statusMap, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return status, nil // No status yet, return empty status
	}

	// Extract lastAppliedRevision (commit SHA)
	if lastAppliedRevision, found, _ := unstructured.NestedString(statusMap, "lastAppliedRevision"); found {
		status.LastAppliedCommit = extractCommitSHA(lastAppliedRevision)
	}

	// Extract conditions to determine readiness
	conditions, found, err := unstructured.NestedSlice(statusMap, "conditions")
	if err == nil && found {
		for _, condition := range conditions {
			condMap, ok := condition.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condMap, "type")
			condStatus, _, _ := unstructured.NestedString(condMap, "status")
			condMessage, _, _ := unstructured.NestedString(condMap, "message")
			condLastTransitionTime, _, _ := unstructured.NestedString(condMap, "lastTransitionTime")

			if condType == "Ready" {
				status.Ready = (condStatus == "True")
				status.Message = condMessage

				// Parse last transition time
				if condLastTransitionTime != "" {
					if t, err := time.Parse(time.RFC3339, condLastTransitionTime); err == nil {
						status.LastSyncTime = t
					}
				}
			}
		}
	}

	return status, nil
}

// extractCommitSHA extracts the commit SHA from a revision string
// FluxCD revision format: main@sha1:abc123 or main/abc123
func extractCommitSHA(revision string) string {
	// Try to extract SHA after "@sha1:" or after "/"
	if len(revision) == 0 {
		return ""
	}

	// Format: main@sha1:abc123
	for i := 0; i < len(revision); i++ {
		if i+6 < len(revision) && revision[i:i+6] == "@sha1:" {
			return revision[i+6:]
		}
	}

	// Format: main/abc123
	for i := len(revision) - 1; i >= 0; i-- {
		if revision[i] == '/' {
			return revision[i+1:]
		}
	}

	// If no special format, return as is (might be just the commit SHA)
	return revision
}
