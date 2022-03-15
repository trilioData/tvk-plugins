package preflight

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

const (
	v1beta1K8sVersion = "v1.19.0"
	v1K8sVersion      = "v1.20.0"
	timeout           = time.Second * 60
	interval          = time.Second * 1
	vsClassCRD        = "volumesnapshotclasses." + StorageSnapshotGroup
	vsContentCRD      = "volumesnapshotcontents." + StorageSnapshotGroup
	vsCRD             = "volumesnapshots." + StorageSnapshotGroup
)

func preflightTestCases(serverVersion string) {

	Describe("Preflight unit test cases", func() {

		AfterEach(func() {
			deleteAllVolumeSnapshotCRD()
		})

		Context("Preflight run command volume snapshot CRD test cases", func() {

			It("Should skip installation if all volume snapshot CRDs are present", func() {
				vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
				installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
				Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
				checkVolumeSnapshotCRDExists()
			})

			for i, crd := range VolumeSnapshotCRDs {
				vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
				It(fmt.Sprintf("Should install missing volume snapshot CRD %s when it is not present", crd), func() {
					vsCRDsMap[VolumeSnapshotCRDs[i]] = false
					installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
					Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
					checkVolumeSnapshotCRDExists()
				})
			}

			It("Should install all volume snapshot CRDs when none of them are present", func() {
				deleteAllVolumeSnapshotCRD()
				Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
				checkVolumeSnapshotCRDExists()
			})

		})

	})

}

var _ = Context("Preflight Unit Tests", func() {
	preflightTestCases(v1beta1K8sVersion)
	preflightTestCases(v1K8sVersion)
})

func installVolumeSnapshotCRD(version string, volumeSnapshotCRDToInstall map[string]bool) {

	for i := range VolumeSnapshotCRDs {
		if volumeSnapshotCRDToInstall[VolumeSnapshotCRDs[i]] {
			crdObj := &apiextensions.CustomResourceDefinition{}
			dirVersion, err := getPrefVersionCRDObj(version)
			Expect(err).To(BeNil())

			fileBytes, readErr := ioutil.ReadFile(filepath.Join(volumeSnapshotCRDYamlDir, dirVersion, VolumeSnapshotCRDs[i]+".yaml"))
			Expect(readErr).To(BeNil())

			Expect(yaml.Unmarshal(fileBytes, crdObj)).To(BeNil())

			Expect(k8sClient.Create(ctx, crdObj)).To(BeNil())
			Eventually(func() error {
				volSnapCRDObj := &apiextensions.CustomResourceDefinition{}
				return k8sClient.Get(ctx, types.NamespacedName{Name: VolumeSnapshotCRDs[i]}, volSnapCRDObj)
			}, timeout, interval).ShouldNot(HaveOccurred())
		}
	}
}

func checkVolumeSnapshotCRDExists() {
	for _, crd := range VolumeSnapshotCRDs {
		crdObj := &apiextensions.CustomResourceDefinition{}
		crdObj.SetName(crd)
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
		}, timeout, interval).ShouldNot(HaveOccurred())
	}
}

func deleteAllVolumeSnapshotCRD() {
	for _, crd := range VolumeSnapshotCRDs {
		crdObj := &apiextensions.CustomResourceDefinition{}
		crdObj.SetName(crd)
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(k8sClient.Delete(ctx, crdObj)).To(BeNil())
			return false
		}, timeout, interval).Should(BeTrue())
	}
}
