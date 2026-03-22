package flux

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// KustomizationManifest represents a FluxCD Kustomization resource
type KustomizationManifest struct {
	APIVersion string                `yaml:"apiVersion"`
	Kind       string                `yaml:"kind"`
	Metadata   KustomizationMetadata `yaml:"metadata"`
	Spec       KustomizationSpec     `yaml:"spec"`
}

// KustomizationMetadata represents metadata for Kustomization resource
type KustomizationMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// KustomizationSpec represents the spec for Kustomization resource
type KustomizationSpec struct {
	Interval   string                   `yaml:"interval"`
	Path       string                   `yaml:"path"`
	Prune      bool                     `yaml:"prune"`
	SourceRef  KustomizationSourceRef   `yaml:"sourceRef"`
	Decryption *KustomizationDecryption `yaml:"decryption,omitempty"`
}

// KustomizationSourceRef references the GitRepository
type KustomizationSourceRef struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

// KustomizationDecryption configures SOPS decryption
type KustomizationDecryption struct {
	Provider  string                 `yaml:"provider"`
	SecretRef KustomizationSecretRef `yaml:"secretRef"`
}

// KustomizationSecretRef references the SOPS key secret
type KustomizationSecretRef struct {
	Name string `yaml:"name"`
}

// GenerateKustomization creates a FluxCD Kustomization manifest for a namespace
func GenerateKustomization(namespace string) ([]byte, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	kustomization := KustomizationManifest{
		APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Kind:       "Kustomization",
		Metadata: KustomizationMetadata{
			Name:      fmt.Sprintf("secrets-%s", namespace),
			Namespace: "flux-system",
		},
		Spec: KustomizationSpec{
			Interval: "1m",
			Path:     fmt.Sprintf("./namespaces/%s/secrets", namespace),
			Prune:    true,
			SourceRef: KustomizationSourceRef{
				Kind: "GitRepository",
				Name: "secrets-repo",
			},
			Decryption: &KustomizationDecryption{
				Provider: "sops",
				SecretRef: KustomizationSecretRef{
					Name: "sops-age",
				},
			},
		},
	}

	yamlData, err := yaml.Marshal(&kustomization)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kustomization to YAML: %w", err)
	}

	return yamlData, nil
}
