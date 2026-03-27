package flux

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yourorg/secret-manager/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// GitRepositoryStatus represents the status of a FluxCD GitRepository
type GitRepositoryStatus struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	Ready             bool      `json:"ready"`
	LastFetchedCommit string    `json:"last_fetched_commit"`
	LastFetchTime     time.Time `json:"last_fetch_time"`
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

// GetGitRepositoryStatus retrieves the status of a specific GitRepository
func (c *FluxClient) GetGitRepositoryStatus(name, namespace string) (*GitRepositoryStatus, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	// Define the GitRepository GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "gitrepositories",
	}

	ctx := context.Background()
	result, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get gitrepository %s: %w", name, err)
	}

	return parseGitRepositoryStatus(result)
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

// parseGitRepositoryStatus extracts status information from an unstructured GitRepository object
func parseGitRepositoryStatus(obj *unstructured.Unstructured) (*GitRepositoryStatus, error) {
	status := &GitRepositoryStatus{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Extract status fields
	statusMap, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return status, nil // No status yet, return empty status
	}

	// Extract artifact.revision (commit SHA)
	if artifactRevision, found, _ := unstructured.NestedString(statusMap, "artifact", "revision"); found {
		status.LastFetchedCommit = extractCommitSHA(artifactRevision)
	}

	// Extract artifact.lastUpdateTime
	if lastUpdateTime, found, _ := unstructured.NestedString(statusMap, "artifact", "lastUpdateTime"); found {
		if t, err := time.Parse(time.RFC3339, lastUpdateTime); err == nil {
			status.LastFetchTime = t
		}
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

			if condType == "Ready" {
				status.Ready = (condStatus == "True")
				status.Message = condMessage
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

// TriggerKustomizationReconciliation triggers immediate reconciliation of a Kustomization
// by patching it with the reconcile.fluxcd.io/requestedAt annotation
func (c *FluxClient) TriggerKustomizationReconciliation(ctx context.Context, name, namespace string) error {
	gvr := schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"reconcile.fluxcd.io/requestedAt": time.Now().Format(time.RFC3339Nano),
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = c.dynamicClient.Resource(gvr).Namespace(namespace).
		Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch Kustomization: %w", err)
	}

	logger.Info("Triggered Kustomization reconciliation", "name", name, "namespace", namespace)
	return nil
}

// TriggerGitRepositoryReconciliation triggers immediate reconciliation of a GitRepository
func (c *FluxClient) TriggerGitRepositoryReconciliation(ctx context.Context, name, namespace string) error {
	gvr := schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "gitrepositories",
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"reconcile.fluxcd.io/requestedAt": time.Now().Format(time.RFC3339Nano),
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = c.dynamicClient.Resource(gvr).Namespace(namespace).
		Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch GitRepository: %w", err)
	}

	logger.Info("Triggered GitRepository reconciliation", "name", name, "namespace", namespace)
	return nil
}

// WaitForKustomizationReconciliation polls Kustomization status until reconciliation completes or times out
func (c *FluxClient) WaitForKustomizationReconciliation(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error {
	deadline := time.Now().Add(timeout)

	logger.Info("Waiting for Kustomization reconciliation", "name", name, "timeout", timeout)

	for time.Now().Before(deadline) {
		status, err := c.GetKustomizationStatus(name, namespace)
		if err != nil {
			logger.Warn("Failed to get Kustomization status", "error", err)
			time.Sleep(pollInterval)
			continue
		}

		// Check if Ready condition is True
		if status.Ready {
			logger.Info("Kustomization reconciliation complete", "name", name, "revision", status.LastAppliedCommit)
			return nil
		}

		// Log progress
		logger.Debug("Kustomization not ready yet", "name", name, "ready", status.Ready)

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("timeout waiting for Kustomization reconciliation after %v", timeout)
}

// GetKustomizationGeneration retrieves the current generation and observedGeneration
// Used to verify reconciliation completion
func (c *FluxClient) GetKustomizationGeneration(ctx context.Context, name, namespace string) (generation int64, observedGeneration int64, err error) {
	gvr := schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	obj, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get Kustomization: %w", err)
	}

	generation = obj.GetGeneration()

	observedGen, found, err := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
	if err != nil || !found {
		return generation, 0, fmt.Errorf("observedGeneration not found in status")
	}

	return generation, observedGen, nil
}
