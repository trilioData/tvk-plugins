package logcollector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PVC and PV Collection Helpers", func() {
	Describe("getPVCsUsedByPods", func() {
		It("should extract PVC names from pods with PVC volumes", func() {
			pod1 := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "pod1",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "data",
								"persistentVolumeClaim": map[string]interface{}{
									"claimName": "pvc1",
								},
							},
						},
					},
				},
			}

			pod2 := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "pod2",
						"namespace": "ns1",
					},
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "cache",
								"persistentVolumeClaim": map[string]interface{}{
									"claimName": "pvc2",
								},
							},
						},
					},
				},
			}

			pods := []unstructured.Unstructured{pod1, pod2}
			pvcSet := getPVCsUsedByPods(pods)

			Expect(pvcSet).To(HaveLen(2))
			Expect(pvcSet[types.NamespacedName{Name: "pvc1", Namespace: "default"}]).To(BeTrue())
			Expect(pvcSet[types.NamespacedName{Name: "pvc2", Namespace: "ns1"}]).To(BeTrue())
		})

		It("should handle pods without PVC volumes", func() {
			pod := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "pod1",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "config",
								"configMap": map[string]interface{}{
									"name": "config-map",
								},
							},
						},
					},
				},
			}

			pods := []unstructured.Unstructured{pod}
			pvcSet := getPVCsUsedByPods(pods)

			Expect(pvcSet).To(BeEmpty())
		})

		It("should handle empty pod list", func() {
			pvcSet := getPVCsUsedByPods([]unstructured.Unstructured{})
			Expect(pvcSet).To(BeEmpty())
		})
	})

	Describe("getPVNameFromPVC", func() {
		It("should extract PV name from bound PVC", func() {
			pvc := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "pvc1",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"volumeName": "pv1",
					},
				},
			}

			pvName := getPVNameFromPVC(pvc)
			Expect(pvName).To(Equal("pv1"))
		})

		It("should return empty string for unbound PVC", func() {
			pvc := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "pvc1",
						"namespace": "default",
					},
					"spec": map[string]interface{}{},
				},
			}

			pvName := getPVNameFromPVC(pvc)
			Expect(pvName).To(BeEmpty())
		})
	})

	Describe("isNFSPV", func() {
		It("should identify NFS PV", func() {
			pv := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pv1",
					},
					"spec": map[string]interface{}{
						"nfs": map[string]interface{}{
							"server": "nfs-server.example.com",
							"path":   "/exports/data",
						},
					},
				},
			}

			Expect(isNFSPV(pv)).To(BeTrue())
		})

		It("should return false for non-NFS PV", func() {
			pv := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pv1",
					},
					"spec": map[string]interface{}{
						"hostPath": map[string]interface{}{
							"path": "/data",
						},
					},
				},
			}

			Expect(isNFSPV(pv)).To(BeFalse())
		})

		It("should return false for PV without spec", func() {
			pv := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pv1",
					},
				},
			}

			Expect(isNFSPV(pv)).To(BeFalse())
		})
	})

	Describe("sanitizeNFSPV", func() {
		It("should remove NFS credentials from PV spec", func() {
			pv := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pv1",
					},
					"spec": map[string]interface{}{
						"nfs": map[string]interface{}{
							"server":   "nfs-server.example.com",
							"path":     "/exports/data",
							"readOnly": false,
						},
						"capacity": map[string]interface{}{
							"storage": "10Gi",
						},
					},
				},
			}

			sanitized := sanitizeNFSPV(pv)

			// Check that NFS fields are removed
			spec, found, _ := unstructured.NestedMap(sanitized.Object, "spec")
			Expect(found).To(BeTrue())

			nfs, found, _ := unstructured.NestedMap(spec, "nfs")
			Expect(found).To(BeTrue())
			Expect(nfs).ToNot(HaveKey("server"))
			Expect(nfs).ToNot(HaveKey("path"))
			Expect(nfs).ToNot(HaveKey("readOnly"))

			// Check that annotation is added
			annotations := sanitized.GetAnnotations()
			Expect(annotations).To(HaveKeyWithValue("log-collector.trilio.io/nfs-credentials-removed", "true"))

			// Check that other spec fields are preserved
			capacity, found, _ := unstructured.NestedMap(spec, "capacity")
			Expect(found).To(BeTrue())
			Expect(capacity).To(HaveKey("storage"))
		})

		It("should preserve non-NFS PVs unchanged", func() {
			pv := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pv1",
					},
					"spec": map[string]interface{}{
						"hostPath": map[string]interface{}{
							"path": "/data",
						},
					},
				},
			}

			sanitized := sanitizeNFSPV(pv)

			// Should be unchanged
			spec, found, _ := unstructured.NestedMap(sanitized.Object, "spec")
			Expect(found).To(BeTrue())
			Expect(spec).To(HaveKey("hostPath"))

			// Should not have annotation
			annotations := sanitized.GetAnnotations()
			if annotations != nil {
				Expect(annotations).ToNot(HaveKey("log-collector.trilio.io/nfs-credentials-removed"))
			}
		})
	})
})
