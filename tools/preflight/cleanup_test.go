package preflight

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Cleanup Unit Tests", func() {

	Context("Check cleanup of preflight resources", func() {
		It("Should clean all preflight resources in a given namespace", func() {
			_ = createPreflightResources()
			_ = createPreflightResources()
			_ = createPreflightResources()

			err := cleanupOps.CleanupPreflightResources(ctx)
			Expect(err).To(BeNil())
		})

		It("Should clean preflight resources of according to UID", func() {
			uid := createPreflightResources()
			cops := cleanupOps
			cops.UID = uid
			err := cops.CleanupPreflightResources(ctx)
			Expect(err).To(BeNil())
		})
	})

	Context("Clean individual resource", func() {
		It("Should clean resource when it exists on cluster", func() {
			resourceSuffix, err := CreateResourceNameSuffix()
			Expect(err).To(BeNil())
			pod := createTestPod(testPodPrefix+resourceSuffix, cleanupOps.Namespace, nil)
			err = cleanupOps.cleanResource(ctx, pod)
			Expect(err).To(BeNil())
		})

		It("Should return error when resource does not exist of cluster", func() {
			res := &unstructured.Unstructured{}
			res.SetName(testPodPrefix + testNameSuffix)
			res.SetNamespace(installNs)
			res.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))
			err := cleanupOps.cleanResource(ctx, res)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Delete resources irrespective of structured or unstructured type", func() {
		Context("Resources of type unstructured", func() {
			It("Should delete pod resource of unstructured type", func() {
				resourceSuffix, err := CreateResourceNameSuffix()
				Expect(err).To(BeNil())
				pod := createTestPod(testPodPrefix+resourceSuffix, cleanupOps.Namespace, nil)
				err = cleanupOps.cleanResource(ctx, pod)
				Expect(err).To(BeNil())
			})

			It("Should return error when resource does not exist in cluster", func() {
				res := &unstructured.Unstructured{}
				res.SetName(testPodPrefix + testNameSuffix)
				res.SetNamespace(installNs)
				res.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))
				err := cleanupOps.cleanResource(ctx, res)
				Expect(err).ToNot(BeNil())
			})
		})

		Context("Resources of type structured", func() {
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
				err = deleteK8sResource(ctx, pod)
				Expect(err).To(BeNil())
			})

			It("Should return error when pod resource of structured type does not exist on cluster", func() {
				pod := getPodTemplate(testPodPrefix+resourceSuffix, resourceSuffix, &runOps)
				err = deleteK8sResource(ctx, pod)
				Expect(err).ToNot(BeNil())
			})
		})
	})
})

func createPreflightResources() string {
	uid, err := CreateResourceNameSuffix()
	Expect(err).To(BeNil())
	pvc, _, err := runOps.createSourcePodAndPVC(ctx, uid)
	Expect(err).To(BeNil())
	_, err = runOps.createSnapshotFromPVC(ctx, testSnapPrefix+uid, defaultSnapshotClass, pvc.GetName(), uid)
	Expect(err).To(BeNil())

	return uid
}
