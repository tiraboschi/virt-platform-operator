/*
Copyright 2026 The KubeVirt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	// DisabledResourcesAnnotation is the annotation key for root exclusion
	DisabledResourcesAnnotation = "platform.kubevirt.io/disabled-resources"
)

// ExclusionRule defines a single resource exclusion rule
type ExclusionRule struct {
	Kind      string `yaml:"kind"`      // Required: Resource kind (e.g., "ConfigMap")
	Namespace string `yaml:"namespace"` // Optional: Namespace (empty = all namespaces, supports wildcards)
	Name      string `yaml:"name"`      // Required: Resource name (supports wildcards)
}

// ParseDisabledResources parses the disabled-resources annotation as YAML
// Format: YAML array of ExclusionRule objects
// Returns: slice of ExclusionRule and error if parsing fails
func ParseDisabledResources(annotation string) ([]ExclusionRule, error) {
	trimmed := strings.TrimSpace(annotation)
	if trimmed == "" {
		return nil, nil // Empty is valid (no exclusions)
	}

	var rules []ExclusionRule
	if err := yaml.Unmarshal([]byte(trimmed), &rules); err != nil {
		return nil, fmt.Errorf("failed to parse disabled-resources annotation: %w", err)
	}

	// Validate rules
	for i, rule := range rules {
		if rule.Kind == "" {
			return nil, fmt.Errorf("rule %d: kind is required", i)
		}
		if rule.Name == "" {
			return nil, fmt.Errorf("rule %d: name is required", i)
		}
	}

	return rules, nil
}

// IsResourceExcluded checks if a specific resource matches any exclusion rule
func IsResourceExcluded(kind, namespace, name string, rules []ExclusionRule) bool {
	for _, rule := range rules {
		// Check kind (exact match, case-sensitive)
		if rule.Kind != kind {
			continue
		}

		// Check namespace
		if rule.Namespace != "" {
			// Rule has namespace specified - must match
			matched, _ := filepath.Match(rule.Namespace, namespace)
			if !matched {
				continue
			}
		}
		// If rule.Namespace is empty, it matches any namespace

		// Check name with wildcard support
		matched, err := filepath.Match(rule.Name, name)
		if err != nil {
			// Invalid pattern - skip this rule
			continue
		}

		if matched {
			return true // Exclusion matched
		}
	}

	return false
}

// FilterExcludedAssets removes disabled resources from asset list
// Returns a new slice with excluded assets removed
func FilterExcludedAssets(assets []*unstructured.Unstructured, rules []ExclusionRule) []*unstructured.Unstructured {
	if len(rules) == 0 {
		return assets
	}

	filtered := make([]*unstructured.Unstructured, 0, len(assets))
	for _, asset := range assets {
		if IsResourceExcluded(asset.GetKind(), asset.GetNamespace(), asset.GetName(), rules) {
			continue // Skip excluded asset
		}
		filtered = append(filtered, asset)
	}

	return filtered
}
