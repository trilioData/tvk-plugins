package preflight

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal"
)

var _ = Describe("Delete Functionality Unit Tests", func() {
	var (
		err error
		pod = &unstructured.Unstructured{}
	)

	BeforeEach(func() {
		createTestPod(testPodName)
		pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))
		Eventually(func() error {
			err = testClient.RuntimeClient.Get(ctx, client.ObjectKey{
				Namespace: installNs,
				Name:      testPodName,
			}, pod)
			return err
		}, timeout, interval).ShouldNot(HaveOccurred())

		pod.SetFinalizers([]string{"kubernetes"})
		err = testClient.RuntimeClient.Update(ctx, pod)
		Eventually(func() bool {
			err = testClient.RuntimeClient.Get(ctx, client.ObjectKey{
				Namespace: installNs,
				Name:      testPodName,
			}, pod)
			Expect(err).To(BeNil())
			return len(pod.GetFinalizers()) == 1
		}, timeout, interval).Should(Equal(true))
	})

	Context("Delete preflight resources when exists on cluster", func() {

		It("Should delete pod resource successfully with finalizers set on the resource", func() {
			err = deleteK8sResource(ctx, pod, testClient.RuntimeClient)
			Expect(err).To(BeNil())
		})
	})

	Context("Remove finalizer from resource metadata", func() {

		It("Should remove finalizers", func() {
			err = removeFinalizer(ctx, pod, testClient.RuntimeClient)
			Expect(err).To(BeNil())

			Eventually(func() interface{} {
				gErr := testClient.RuntimeClient.Get(ctx, client.ObjectKey{Namespace: installNs, Name: testPodName}, pod)
				Expect(gErr).To(BeNil())
				return pod.GetFinalizers()
			}, timeout, interval).Should(BeNil())
		})
	})
})

var _ = Describe("Cleanup Resources GVK Unit Tests", func() {

	Context("Fetch resource cleanup GVK list", func() {

		It("Should return list of API GVK to be cleaned", func() {
			gvkList, err := getCleanupResourceGVKList(testClient.ClientSet)
			Expect(err).To(BeNil())
			Expect(len(gvkList)).To(BeNumerically(">=", defaultCleanupGVKListLen))
		})
	})
})
