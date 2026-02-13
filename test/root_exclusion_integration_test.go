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

package test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubevirt/virt-platform-autopilot/pkg/engine"
)

var _ = Describe("Root Exclusion Integration", func() {
	Describe("ParseDisabledResources", func() {
		It("should parse empty annotation", func() {
			result, err := engine.ParseDisabledResources("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should parse single resource", func() {
			yaml := `
- kind: ConfigMap
  namespace: default
  name: test-config
`
			result, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Kind).To(Equal("ConfigMap"))
			Expect(result[0].Namespace).To(Equal("default"))
			Expect(result[0].Name).To(Equal("test-config"))
		})

		It("should parse multiple resources with different namespaces", func() {
			yaml := `
- kind: ConfigMap
  namespace: openshift-cnv
  name: foo
- kind: Secret
  name: bar
- kind: Deployment
  namespace: default
  name: baz
`
			result, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(3))
			Expect(result[0].Kind).To(Equal("ConfigMap"))
			Expect(result[0].Namespace).To(Equal("openshift-cnv"))
			Expect(result[0].Name).To(Equal("foo"))
			Expect(result[1].Kind).To(Equal("Secret"))
			Expect(result[1].Namespace).To(Equal(""))
			Expect(result[1].Name).To(Equal("bar"))
		})

		It("should handle complex resource names", func() {
			yaml := `
- kind: KubeDescheduler
  name: cluster
- kind: MachineConfig
  name: 50-swap-enable
`
			result, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Kind).To(Equal("KubeDescheduler"))
			Expect(result[0].Name).To(Equal("cluster"))
			Expect(result[1].Kind).To(Equal("MachineConfig"))
			Expect(result[1].Name).To(Equal("50-swap-enable"))
		})

		It("should return error for invalid YAML", func() {
			yaml := "invalid yaml ["
			result, err := engine.ParseDisabledResources(yaml)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("FilterExcludedAssets", func() {
		var assets []*unstructured.Unstructured

		BeforeEach(func() {
			// Create test assets
			assets = []*unstructured.Unstructured{
				createUnstructuredResource("ConfigMap", "config-1", "default"),
				createUnstructuredResource("ConfigMap", "config-2", "default"),
				createUnstructuredResource("Secret", "secret-1", "default"),
				createUnstructuredResource("Deployment", "deploy-1", "default"),
			}
		})

		It("should return all assets when rules is empty", func() {
			var rules []engine.ExclusionRule
			result := engine.FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(4))
		})

		It("should exclude specified resource", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "config-1"},
			}
			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(3))

			// Verify the right one was excluded
			names := make([]string, 0)
			for _, asset := range result {
				key := asset.GetKind() + "/" + asset.GetName()
				names = append(names, key)
			}

			Expect(names).To(ConsistOf(
				"ConfigMap/config-2",
				"Secret/secret-1",
				"Deployment/deploy-1",
			))
		})

		It("should exclude multiple resources", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "config-1"},
				{Kind: "Secret", Namespace: "default", Name: "secret-1"},
			}
			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(2))

			names := make([]string, 0)
			for _, asset := range result {
				key := asset.GetKind() + "/" + asset.GetName()
				names = append(names, key)
			}

			Expect(names).To(ConsistOf(
				"ConfigMap/config-2",
				"Deployment/deploy-1",
			))
		})

		It("should exclude all when all are specified", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "config-1"},
				{Kind: "ConfigMap", Namespace: "default", Name: "config-2"},
				{Kind: "Secret", Namespace: "default", Name: "secret-1"},
				{Kind: "Deployment", Namespace: "default", Name: "deploy-1"},
			}
			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(BeEmpty())
		})

		It("should keep all when none match", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "nonexistent"},
				{Kind: "Service", Namespace: "default", Name: "test"},
			}
			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(4))
		})

		It("should handle nil rules", func() {
			result := engine.FilterExcludedAssets(assets, nil)
			Expect(result).To(HaveLen(4))
		})

		It("should be case-sensitive for Kind", func() {
			rules := []engine.ExclusionRule{
				{Kind: "configmap", Namespace: "default", Name: "config-1"}, // lowercase kind
			}
			result := engine.FilterExcludedAssets(assets, rules)

			// Should NOT exclude because Kind is case-sensitive
			Expect(result).To(HaveLen(4))
		})
	})

	Describe("IsResourceExcluded", func() {
		It("should return false for empty rules", func() {
			var rules []engine.ExclusionRule
			Expect(engine.IsResourceExcluded("ConfigMap", "default", "test", rules)).To(BeFalse())
		})

		It("should return true for excluded resource", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "test"},
			}
			Expect(engine.IsResourceExcluded("ConfigMap", "default", "test", rules)).To(BeTrue())
		})

		It("should return false for non-excluded resource", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "test"},
			}
			Expect(engine.IsResourceExcluded("ConfigMap", "default", "other", rules)).To(BeFalse())
			Expect(engine.IsResourceExcluded("Secret", "default", "test", rules)).To(BeFalse())
		})

		It("should be case-sensitive", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "test"},
			}
			Expect(engine.IsResourceExcluded("configmap", "default", "test", rules)).To(BeFalse())
			Expect(engine.IsResourceExcluded("ConfigMap", "default", "Test", rules)).To(BeFalse())
		})

		It("should handle nil rules", func() {
			Expect(engine.IsResourceExcluded("ConfigMap", "default", "test", nil)).To(BeFalse())
		})
	})

	Describe("Wildcard and namespace filtering", func() {
		It("should filter with name wildcards", func() {
			rules := []engine.ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-*"},
			}

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("ConfigMap", "virt-handler", "openshift-cnv"),
				createUnstructuredResource("ConfigMap", "virt-controller", "openshift-cnv"),
				createUnstructuredResource("ConfigMap", "other-config", "openshift-cnv"),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(1))
			Expect(result[0].GetName()).To(Equal("other-config"))
		})

		It("should filter with namespace wildcards", func() {
			rules := []engine.ExclusionRule{
				{Kind: "Service", Namespace: "prod-*", Name: "metrics"},
			}

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("Service", "metrics", "prod-us"),
				createUnstructuredResource("Service", "metrics", "prod-eu"),
				createUnstructuredResource("Service", "metrics", "dev-us"),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(1))
			Expect(result[0].GetNamespace()).To(Equal("dev-us"))
		})

		It("should match any namespace when rule namespace is empty", func() {
			rules := []engine.ExclusionRule{
				{Kind: "Secret", Name: "credentials"},
			}

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("Secret", "credentials", "default"),
				createUnstructuredResource("Secret", "credentials", "kube-system"),
				createUnstructuredResource("Secret", "other", "default"),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(1))
			Expect(result[0].GetName()).To(Equal("other"))
		})
	})

	Describe("Real-world scenarios", func() {
		It("should handle KubeDescheduler exclusion", func() {
			yaml := `
- kind: KubeDescheduler
  name: cluster
`
			rules, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("KubeDescheduler", "cluster", ""),
				createUnstructuredResource("HyperConverged", "kubevirt-hyperconverged", "openshift-cnv"),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(1))
			Expect(result[0].GetKind()).To(Equal("HyperConverged"))
		})

		It("should handle MachineConfig exclusion", func() {
			yaml := `
- kind: MachineConfig
  name: 50-swap-enable
`
			rules, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("MachineConfig", "50-swap-enable", ""),
				createUnstructuredResource("MachineConfig", "51-pci-passthrough", ""),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(1))
			Expect(result[0].GetName()).To(Equal("51-pci-passthrough"))
		})

		It("should handle multiple feature exclusions", func() {
			yaml := `
- kind: KubeDescheduler
  name: cluster
- kind: MachineConfig
  name: 50-swap-enable
- kind: PersesDataSource
  namespace: openshift-cnv
  name: virt-metrics
`
			rules, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("KubeDescheduler", "cluster", ""),
				createUnstructuredResource("MachineConfig", "50-swap-enable", ""),
				createUnstructuredResource("MachineConfig", "51-pci-passthrough", ""),
				createUnstructuredResource("PersesDataSource", "virt-metrics", "openshift-cnv"),
				createUnstructuredResource("HyperConverged", "kubevirt-hyperconverged", "openshift-cnv"),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			// Should have 2 resources left (MachineConfig and HyperConverged)
			Expect(result).To(HaveLen(2))

			names := make([]string, 0)
			for _, asset := range result {
				names = append(names, asset.GetName())
			}

			Expect(names).To(ConsistOf(
				"51-pci-passthrough",
				"kubevirt-hyperconverged",
			))
		})

		It("should handle wildcard exclusions for virt configs", func() {
			yaml := `
- kind: ConfigMap
  namespace: openshift-cnv
  name: virt-*
`
			rules, err := engine.ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())

			assets := []*unstructured.Unstructured{
				createUnstructuredResource("ConfigMap", "virt-handler", "openshift-cnv"),
				createUnstructuredResource("ConfigMap", "virt-controller", "openshift-cnv"),
				createUnstructuredResource("ConfigMap", "virt-api", "openshift-cnv"),
				createUnstructuredResource("ConfigMap", "other-config", "openshift-cnv"),
			}

			result := engine.FilterExcludedAssets(assets, rules)

			Expect(result).To(HaveLen(1))
			Expect(result[0].GetName()).To(Equal("other-config"))
		})
	})
})

// Helper function to create unstructured resources for testing
func createUnstructuredResource(kind, name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	obj.SetAPIVersion("v1")
	return obj
}
