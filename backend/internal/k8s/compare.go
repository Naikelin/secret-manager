package k8s

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// CompareSecretData compares a Kubernetes secret with Git data
// Returns true if they are identical, false if drift detected
func CompareSecretData(k8sSecret *corev1.Secret, gitData map[string]string) bool {
	if k8sSecret == nil {
		return false
	}

	// Normalize K8s secret first (filters out K8s-managed keys)
	k8sNormalized := NormalizeSecretData(k8sSecret)

	// Check if key counts match
	if len(k8sNormalized) != len(gitData) {
		return false
	}

	// Compare each key-value pair from Git side
	for key, gitValue := range gitData {
		k8sValue, exists := k8sNormalized[key]
		if !exists {
			return false // Key missing in K8s
		}

		if k8sValue != gitValue {
			return false // Value differs
		}
	}

	// Also check if K8s has extra keys not in Git
	for key := range k8sNormalized {
		if _, exists := gitData[key]; !exists {
			return false // Extra key in K8s
		}
	}

	return true
}

// NormalizeSecretData extracts and decodes base64 data from a Kubernetes secret
// Filters out Kubernetes-injected keys that should not be compared
func NormalizeSecretData(k8sSecret *corev1.Secret) map[string]string {
	if k8sSecret == nil {
		return make(map[string]string)
	}

	normalized := make(map[string]string)

	// Process base64-encoded data
	for key, value := range k8sSecret.Data {
		// Skip Kubernetes-injected service account keys
		if shouldSkipKey(key) {
			continue
		}

		// K8s secret data is already base64-decoded by the client-go library
		// Store as plain string for comparison
		normalized[key] = string(value)
	}

	// Process plaintext string data (if any)
	for key, value := range k8sSecret.StringData {
		if shouldSkipKey(key) {
			continue
		}
		normalized[key] = value
	}

	return normalized
}

// shouldSkipKey returns true if a key should be excluded from comparison
// These are Kubernetes-managed keys that should not trigger drift detection
// Only skip keys that are ALWAYS injected by Kubernetes, not user-defined keys
func shouldSkipKey(key string) bool {
	// Skip service account keys that are ALWAYS present in service account secrets
	// These are injected by Kubernetes for service account tokens
	if key == "ca.crt" || key == "namespace" {
		return true
	}

	// Skip keys with service-account prefix (these are always K8s-managed)
	if strings.HasPrefix(key, "service-account-") {
		return true
	}

	// Skip cert-manager managed TLS keys
	if strings.HasPrefix(key, "tls.") {
		return true
	}

	// Note: We do NOT skip "token" key alone because it might be a legitimate user secret
	// Service account tokens typically have the pattern "service-account-token" which is caught above

	return false
}

// CalculateSecretHash computes a deterministic SHA256 hash of secret data
// This is useful for quick comparison without decoding all values
func CalculateSecretHash(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}

	// Sort keys for deterministic hash
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build sorted key=value pairs
	var builder strings.Builder
	for i, key := range keys {
		if i > 0 {
			builder.WriteString("|")
		}
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(data[key])
	}

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(builder.String()))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// ComputeDiff returns a list of differences between Git and K8s secret data
func ComputeDiff(gitData, k8sData map[string]string) []string {
	var differences []string

	// Check for missing keys in K8s
	for gitKey := range gitData {
		if _, exists := k8sData[gitKey]; !exists {
			differences = append(differences, fmt.Sprintf("Key '%s' missing in K8s", gitKey))
		}
	}

	// Check for extra keys in K8s
	for k8sKey := range k8sData {
		if _, exists := gitData[k8sKey]; !exists {
			differences = append(differences, fmt.Sprintf("Key '%s' added in K8s (not in Git)", k8sKey))
		}
	}

	// Check for value differences
	for key, gitValue := range gitData {
		if k8sValue, exists := k8sData[key]; exists && gitValue != k8sValue {
			differences = append(differences, fmt.Sprintf("Key '%s' value differs", key))
		}
	}

	return differences
}
