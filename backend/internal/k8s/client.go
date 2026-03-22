package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps the Kubernetes clientset for secret operations
type K8sClient struct {
	clientset kubernetes.Interface
	timeout   time.Duration
}

// NewK8sClient creates a new Kubernetes client
// If kubeconfig is empty, it uses in-cluster configuration
func NewK8sClient(kubeconfig string) (*K8sClient, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// In-cluster config (when running inside Kubernetes)
		config, err = rest.InClusterConfig()
	} else {
		// Out-of-cluster config (local development with kubeconfig file)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &K8sClient{
		clientset: clientset,
		timeout:   10 * time.Second, // Default timeout for operations
	}, nil
}

// GetSecret retrieves a single secret from Kubernetes
func (c *K8sClient) GetSecret(namespace, name string) (*corev1.Secret, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if name == "" {
		return nil, fmt.Errorf("secret name cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	secret, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("secret %s not found in namespace %s", name, namespace)
		}
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	return secret, nil
}

// ListSecrets retrieves all secrets in a namespace
func (c *K8sClient) ListSecrets(namespace string) ([]corev1.Secret, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	list, err := c.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	return list.Items, nil
}

// SecretExists checks if a secret exists in the specified namespace
func (c *K8sClient) SecretExists(namespace, name string) (bool, error) {
	if namespace == "" {
		return false, fmt.Errorf("namespace cannot be empty")
	}
	if name == "" {
		return false, fmt.Errorf("secret name cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil // Secret doesn't exist, not an error
		}
		return false, fmt.Errorf("failed to check secret existence: %w", err)
	}

	return true, nil
}

// DeleteSecret deletes a secret from Kubernetes (used for drift resolution)
func (c *K8sClient) DeleteSecret(namespace, name string) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	err := c.clientset.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Secret already deleted, not an error
			return nil
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}
