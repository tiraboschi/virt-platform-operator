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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Root Exclusion", func() {
	Describe("ParseDisabledResources", func() {
		It("should return nil for empty annotation", func() {
			result, err := ParseDisabledResources("")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should parse single rule", func() {
			yaml := `
- kind: ConfigMap
  namespace: openshift-cnv
  name: my-config
`
			result, err := ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Kind).To(Equal("ConfigMap"))
			Expect(result[0].Namespace).To(Equal("openshift-cnv"))
			Expect(result[0].Name).To(Equal("my-config"))
		})

		It("should parse multiple rules", func() {
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
			result, err := ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(3))
			Expect(result[0].Kind).To(Equal("ConfigMap"))
			Expect(result[0].Namespace).To(Equal("openshift-cnv"))
			Expect(result[0].Name).To(Equal("foo"))
			Expect(result[1].Kind).To(Equal("Secret"))
			Expect(result[1].Namespace).To(Equal(""))
			Expect(result[1].Name).To(Equal("bar"))
			Expect(result[2].Kind).To(Equal("Deployment"))
			Expect(result[2].Namespace).To(Equal("default"))
			Expect(result[2].Name).To(Equal("baz"))
		})

		It("should return error for invalid YAML", func() {
			yaml := "invalid yaml ["
			result, err := ParseDisabledResources(yaml)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return error for missing kind", func() {
			yaml := `
- namespace: openshift-cnv
  name: my-config
`
			result, err := ParseDisabledResources(yaml)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kind is required"))
			Expect(result).To(BeNil())
		})

		It("should return error for missing name", func() {
			yaml := `
- kind: ConfigMap
  namespace: openshift-cnv
`
			result, err := ParseDisabledResources(yaml)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name is required"))
			Expect(result).To(BeNil())
		})

		It("should handle whitespace correctly", func() {
			yaml := `
- kind: ConfigMap
  namespace: openshift-cnv
  name: my-config
`
			result, err := ParseDisabledResources(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Kind).To(Equal("ConfigMap"))
		})
	})

	Describe("IsResourceExcluded", func() {
		It("should return false for empty rules", func() {
			var rules []ExclusionRule
			Expect(IsResourceExcluded("ConfigMap", "default", "test", rules)).To(BeFalse())
		})

		It("should match exact kind/namespace/name", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "my-config"},
			}
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "my-config", rules)).To(BeTrue())
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "other", rules)).To(BeFalse())
			Expect(IsResourceExcluded("ConfigMap", "default", "my-config", rules)).To(BeFalse())
			Expect(IsResourceExcluded("Secret", "openshift-cnv", "my-config", rules)).To(BeFalse())
		})

		It("should match kind with name wildcard", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-*"},
			}
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "virt-handler", rules)).To(BeTrue())
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "virt-controller", rules)).To(BeTrue())
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "other", rules)).To(BeFalse())
		})

		It("should match kind with namespace wildcard", func() {
			rules := []ExclusionRule{
				{Kind: "Service", Namespace: "prod-*", Name: "metrics"},
			}
			Expect(IsResourceExcluded("Service", "prod-us", "metrics", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Service", "prod-eu", "metrics", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Service", "dev-us", "metrics", rules)).To(BeFalse())
		})

		It("should match kind with both wildcards", func() {
			rules := []ExclusionRule{
				{Kind: "Secret", Namespace: "prod-*", Name: "credentials-*"},
			}
			Expect(IsResourceExcluded("Secret", "prod-us", "credentials-db", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Secret", "prod-eu", "credentials-api", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Secret", "dev-us", "credentials-db", rules)).To(BeFalse())
			Expect(IsResourceExcluded("Secret", "prod-us", "other", rules)).To(BeFalse())
		})

		It("should match cluster-scoped resources (empty namespace)", func() {
			rules := []ExclusionRule{
				{Kind: "KubeDescheduler", Name: "cluster"},
			}
			Expect(IsResourceExcluded("KubeDescheduler", "", "cluster", rules)).To(BeTrue())
		})

		It("should match any namespace when rule namespace is empty", func() {
			rules := []ExclusionRule{
				{Kind: "Secret", Name: "credentials"},
			}
			Expect(IsResourceExcluded("Secret", "default", "credentials", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Secret", "kube-system", "credentials", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Secret", "", "credentials", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Secret", "default", "other", rules)).To(BeFalse())
		})

		It("should be case-sensitive for kind", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "test"},
			}
			Expect(IsResourceExcluded("ConfigMap", "default", "test", rules)).To(BeTrue())
			Expect(IsResourceExcluded("configmap", "default", "test", rules)).To(BeFalse())
		})

		It("should handle invalid wildcard patterns gracefully", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "default", Name: "test["},
			}
			// Invalid pattern should be skipped (fail-open)
			Expect(IsResourceExcluded("ConfigMap", "default", "test[", rules)).To(BeFalse())
		})

		It("should check multiple rules", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-*"},
				{Kind: "Secret", Name: "credentials"},
				{Kind: "Service", Namespace: "prod-*", Name: "metrics"},
			}
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "virt-handler", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Secret", "default", "credentials", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Service", "prod-us", "metrics", rules)).To(BeTrue())
			Expect(IsResourceExcluded("Deployment", "default", "test", rules)).To(BeFalse())
		})

		It("should match first matching rule", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-*"},
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-handler"},
			}
			// First rule should match
			Expect(IsResourceExcluded("ConfigMap", "openshift-cnv", "virt-handler", rules)).To(BeTrue())
		})
	})

	Describe("FilterExcludedAssets", func() {
		var assets []*unstructured.Unstructured

		BeforeEach(func() {
			// Create test assets
			assets = []*unstructured.Unstructured{
				createTestAsset("ConfigMap", "openshift-cnv", "virt-handler"),
				createTestAsset("ConfigMap", "openshift-cnv", "virt-controller"),
				createTestAsset("Secret", "default", "credentials"),
				createTestAsset("Deployment", "default", "test"),
				createTestAsset("KubeDescheduler", "", "cluster"),
			}
		})

		It("should return all assets for empty rules", func() {
			var rules []ExclusionRule
			result := FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(5))
		})

		It("should filter single resource", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-handler"},
			}
			result := FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(4))

			// Verify the right one was excluded
			for _, asset := range result {
				key := asset.GetKind() + "/" + asset.GetNamespace() + "/" + asset.GetName()
				Expect(key).NotTo(Equal("ConfigMap/openshift-cnv/virt-handler"))
			}
		})

		It("should filter with wildcards", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-*"},
			}
			result := FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(3))

			// Verify both ConfigMaps were excluded
			for _, asset := range result {
				Expect(asset.GetKind()).NotTo(Equal("ConfigMap"))
			}
		})

		It("should filter with namespace matching", func() {
			rules := []ExclusionRule{
				{Kind: "Secret", Name: "credentials"},
			}
			result := FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(4))

			// Verify Secret was excluded
			for _, asset := range result {
				if asset.GetKind() == "Secret" {
					Expect(asset.GetName()).NotTo(Equal("credentials"))
				}
			}
		})

		It("should handle nil rules", func() {
			result := FilterExcludedAssets(assets, nil)
			Expect(result).To(HaveLen(5))
		})

		It("should filter multiple resources", func() {
			rules := []ExclusionRule{
				{Kind: "ConfigMap", Namespace: "openshift-cnv", Name: "virt-*"},
				{Kind: "Secret", Name: "credentials"},
			}
			result := FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(2))

			// Verify only Deployment and KubeDescheduler remain
			Expect(result[0].GetKind()).To(Equal("Deployment"))
			Expect(result[1].GetKind()).To(Equal("KubeDescheduler"))
		})

		It("should filter cluster-scoped resources", func() {
			rules := []ExclusionRule{
				{Kind: "KubeDescheduler", Name: "cluster"},
			}
			result := FilterExcludedAssets(assets, rules)
			Expect(result).To(HaveLen(4))

			// Verify KubeDescheduler was excluded
			for _, asset := range result {
				Expect(asset.GetKind()).NotTo(Equal("KubeDescheduler"))
			}
		})
	})
})

// createTestAsset creates a test unstructured object
func createTestAsset(kind, namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetAPIVersion("v1")
	return obj
}
