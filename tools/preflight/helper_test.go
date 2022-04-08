package preflight

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
			_, err := GetServerPreferredVersionForGroup(storageClassGroup, testClient.ClientSet)
			Expect(err).To(BeNil())
		})

		It("Should return error when no version for a group is found on cluster", func() {
			_, err := GetServerPreferredVersionForGroup(invalidGroup, testClient.ClientSet)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("no preferred version for group - %s found on cluster", invalidGroup)))
		})
	})

	Context("Fetch versions of a group on cluster", func() {

		It("Should return non-zero length slice of version for a group existing on cluster", func() {
			vers, err := getVersionsOfGroup(storageClassGroup, testClient.ClientSet)
			Expect(err).To(BeNil())
			Expect(len(vers)).ToNot(Equal(0))
		})
		It("Should return zero length slice of version for a group not existing on cluster", func() {
			vers, err := getVersionsOfGroup(invalidGroup, testClient.ClientSet)
			Expect(err).To(BeNil())
			Expect(len(vers)).To(Equal(0))
		})
	})

	Context("Check cluster has volume snapshot class", func() {

		It("Should return volume-snapshot-class object when snapshotclass name is provided and is present on cluster", func() {
			var (
				prefVersion string
				vsc         *unstructured.Unstructured
				err         error
			)

			vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
			installVolumeSnapshotCRD(v1beta1K8sVersion, vsCRDsMap)
			installVolumeSnapshotClass(internal.V1BetaVersion, testDriver, testSnapshotClass)
			defer func() {
				deleteSnapshotClass(testSnapshotClass)
				deleteAllVolumeSnapshotCRD()
			}()
			prefVersion, err = GetServerPreferredVersionForGroup(StorageSnapshotGroup, testClient.ClientSet)
			Expect(err).To(BeNil())
			vsc, err = clusterHasVolumeSnapshotClass(ctx, testSnapshotClass, testClient.ClientSet, testClient.RuntimeClient)
			Expect(err).To(BeNil())
			Expect(vsc.GroupVersionKind()).To(Equal(schema.GroupVersionKind{
				Group:   StorageSnapshotGroup,
				Version: prefVersion,
				Kind:    internal.VolumeSnapshotClassKind,
			}))
		})

		It("Should return error when snapshot class does not exist on cluster", func() {
			_, err := clusterHasVolumeSnapshotClass(ctx, invalidSnapshotClass, testClient.ClientSet, testClient.RuntimeClient)
			Expect(err).ToNot(BeNil())
		})
	})

})
