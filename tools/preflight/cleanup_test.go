package preflight

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Cleanup Unit Tests", func() {

	Context("Delete preflight resources of structured or unstructured type", func() {

		Context("When resource is of unstructured type", func() {

			It("Should delete pod resource of unstructured type", func() {
				resourceSuffix, err := CreateResourceNameSuffix()
				Expect(err).To(BeNil())
				pod := createTestPod(testPodPrefix+resourceSuffix, cleanupOps.Namespace, nil)
				err = cleanupOps.cleanResource(ctx, pod, contClient)
				Expect(err).To(BeNil())
			})

			It("Should return error when resource of unstructured type does not exist in cluster", func() {
				res := &unstructured.Unstructured{}
				res.SetName(testPodPrefix + testNameSuffix)
				res.SetNamespace(installNs)
				res.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))
				err := cleanupOps.cleanResource(ctx, res, contClient)
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When resource is of structured type", func() {
			var (
				resourceSuffix string
				err            error
			)
			BeforeEach(func() {
				resourceSuffix, err = CreateResourceNameSuffix()
				Expect(err).To(BeNil())
			})

			It("Should delete pod resource of structured type", func() {
				pod := createTestPodStructured(testPodPrefix+resourceSuffix, installNs)
				err = deleteK8sResource(ctx, pod, contClient)
				Expect(err).To(BeNil())
			})

			It("Should return error when pod resource of structured type does not exist on cluster", func() {
				pod := getPodTemplate(testPodPrefix+resourceSuffix, resourceSuffix, &runOps)
				err = deleteK8sResource(ctx, pod, contClient)
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Context("Fetch resource cleanup GVK list", func() {
		It("Should return list of API GVK to be cleaned", func() {
			gvkList, err := getCleanupResourceGVKList(k8sClient)
			Expect(err).To(BeNil())
			Expect(len(gvkList)).To(BeNumerically(">=", defaultCleanupGVKListLen))
		})

		It("Should return error when nil clientset object is provided", func() {
			_, err := getCleanupResourceGVKList(nil)
			Expect(err).ToNot(BeNil())
		})
	})
})
