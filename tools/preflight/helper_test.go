package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Preflight Helper Funcs Unit Tests", func() {
	Context("Initialization of kube-client objects from kubeconfig", func() {
		It("Should initialize kube-client objects when valid kubeconfig file path is provided", func() {
			envKubeconfigVal := os.Getenv(internal.KubeconfigEnv)
			err := InitKubeEnv(envKubeconfigVal)
			Expect(err).To(BeNil())
		})

		It(fmt.Sprintf("Should read kubeconfig file path of env variable - %s when empty kubeconfig file path is provided",
			internal.KubeconfigEnv), func() {
			err := InitKubeEnv("")
			Expect(err).To(BeNil())
		})

		It(fmt.Sprintf("Should read kubeconfig file path of env variable - %s when kubeconfig file with empty data is provided",
			internal.KubeconfigEnv), func() {
			kcPath := filepath.Join(testDataDirRelPath, emptyFile)
			err := InitKubeEnv(kcPath)
			Expect(err).To(BeNil())
		})

		It("Should return error when non-existent kubeconfig file path is provided", func() {
			kcPath := filepath.Join(testDataDirRelPath, nonExistentFile)
			err := InitKubeEnv(kcPath)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when kubeconfig file contains invalid data", func() {
			kcPath := filepath.Join(testDataDirRelPath, invalidKubeconfigFile)
			err := InitKubeEnv(kcPath)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Get helm version", func() {
		It("Should return helm version when correct binary name is provided", func() {
			_, err := GetHelmVersion(HelmBinaryName)
			Expect(err).To(BeNil())
		})

		It("Should return error when incorrect helm binary name is provided", func() {
			_, err := GetHelmVersion(invalidHelmBinaryName)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Server preferred version for a group", func() {
		It("Should return prefer version of valid group using a go-client", func() {
			_, err := GetServerPreferredVersionForGroup(storageClassGroup, k8sClient)
			Expect(err).To(BeNil())
		})

		It("Should return error when no version for a group is found on cluster", func() {
			_, err := GetServerPreferredVersionForGroup(invalidGroup, k8sClient)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("no preferred version for group - %s found on cluster", invalidGroup)))
		})

		It("Should return error when nil go-client object is provided", func() {
			_, err := GetServerPreferredVersionForGroup(storageClassGroup, nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("client object is nil, cannot fetch versions of group - %s", storageClassGroup)))
		})
	})

	Context("Get versions of a group", func() {
		It("Should return non-zero length slice of version for a group existing on cluster", func() {
			vers, err := getVersionsOfGroup(storageClassGroup, k8sClient)
			Expect(err).To(BeNil())
			Expect(len(vers)).ToNot(Equal(0))
		})
		It("Should return zero length slice of version for a group not existing on cluster", func() {
			vers, err := getVersionsOfGroup(invalidGroup, k8sClient)
			Expect(err).To(BeNil())
			Expect(len(vers)).To(Equal(0))
		})
		It("Should return error if nil go-client object is provided", func() {
			_, err := getVersionsOfGroup(storageClassGroup, nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("client object is nil, cannot fetch versions of group - %s", storageClassGroup)))
		})
	})

	Context("Check cluster has snapshot class", func() {
		It("Should return volume-snapshot-class object when snapshotclass name is provided and is present on cluster", func() {
			_, err := clusterHasVolumeSnapshotClass(ctx, defaultSnapshotClass, contClient)
			Expect(err).To(BeNil())
		})

		It("Should return error when snapshot class does not exist on cluster", func() {
			_, err := clusterHasVolumeSnapshotClass(ctx, invalidSnapshotClass, contClient)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("volume snapshot class %s not found on cluster ::", invalidSnapshotClass)))
		})

		It("Should return error when runtime client object is nil", func() {
			_, err := clusterHasVolumeSnapshotClass(ctx, defaultSnapshotClass, nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				"runtime client object is nil, cannot fetch snapshot class from server"))
		})
	})

	Context("Waiting until pod reaches a particular condition", func() {
		var (
			resourceSuffix string
			err            error
		)
		BeforeEach(func() {
			resourceSuffix, err = CreateResourceNameSuffix()
		})

		It("Should wait until pod reaches ready condition with timeout limit", func() {
			pod := createTestPod(testPodPrefix+resourceSuffix, installNs, nil)
			defer deleteTestPod(pod.GetName(), pod.GetNamespace())
			wop := &wait.PodWaitOptions{
				Name:         pod.GetName(),
				Namespace:    pod.GetNamespace(),
				Timeout:      3 * time.Minute,
				PodCondition: corev1.PodReady,
				ClientSet:    k8sClient,
			}
			err = waitUntilPodCondition(ctx, wop)
			Expect(err).To(BeNil())
		})

		It("Should return error when does not reach required state within the given timeout period", func() {
			pod := createTestPod(testPodPrefix+resourceSuffix, installNs, map[string]string{
				"node-sel-key": "node-sel-val",
			})
			defer deleteTestPod(pod.GetName(), pod.GetNamespace())

			wop := &wait.PodWaitOptions{
				Name:         pod.GetName(),
				Namespace:    pod.GetNamespace(),
				Timeout:      3 * time.Minute,
				PodCondition: corev1.PodScheduled,
				ClientSet:    k8sClient,
			}
			err = waitUntilPodCondition(ctx, wop)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when pod does not exist on cluster", func() {
			pod := &unstructured.Unstructured{}
			pod.Object = map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      testPodPrefix + resourceSuffix,
					"namespace": installNs,
				},
			}
			wop := &wait.PodWaitOptions{
				Name:         pod.GetName(),
				Namespace:    pod.GetNamespace(),
				Timeout:      3 * time.Minute,
				PodCondition: corev1.PodScheduled,
				ClientSet:    k8sClient,
			}
			err = waitUntilPodCondition(ctx, wop)
			Expect(err).ToNot(BeNil())
		})
	})

})
