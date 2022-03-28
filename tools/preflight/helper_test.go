package preflight

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/trilioData/tvk-plugins/internal"
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

	Context("Fetch helm version from shell", func() {
		It("Should return helm version when correct binary name is provided", func() {
			_, err := GetHelmVersion(HelmBinaryName)
			Expect(err).To(BeNil())
		})

		It("Should return error when incorrect helm binary name is provided", func() {
			_, err := GetHelmVersion(invalidHelmBinaryName)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Fetch server preferred version for a API group on cluster", func() {

		It("Should return preferred version of valid group using a go-client", func() {
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

	Context("Fetch versions of a group on cluster", func() {

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

	Context("Check cluster has volume snapshot class", func() {

		It("Should return volume-snapshot-class object when snapshotclass name is provided and is present on cluster", func() {
			vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
			installVolumeSnapshotCRD(v1beta1K8sVersion, vsCRDsMap)
			installVolumeSnapshotClass(internal.V1BetaVersion, testDriver, testSnapshotClass)
			defer func() {
				deleteSnapshotClass(testSnapshotClass)
				deleteAllVolumeSnapshotCRD()
			}()
			_, err := clusterHasVolumeSnapshotClass(ctx, testSnapshotClass, contClient, k8sClient)
			Expect(err).To(BeNil())
		})

		It("Should return error when snapshot class does not exist on cluster", func() {
			_, err := clusterHasVolumeSnapshotClass(ctx, invalidSnapshotClass, contClient, k8sClient)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when runtime client object is nil", func() {
			_, err := clusterHasVolumeSnapshotClass(ctx, defaultSnapshotClass, nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				"runtime client object is nil, cannot fetch snapshot class from server"))
		})
	})

})
