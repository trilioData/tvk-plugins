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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
)

const (
	v1beta1K8sVersion = "v1.19.0"
	v1K8sVersion      = "v1.20.0"
	timeout           = time.Second * 60
	interval          = time.Second * 1
	vsClassCRD        = "volumesnapshotclasses." + StorageSnapshotGroup
	vsContentCRD      = "volumesnapshotcontents." + StorageSnapshotGroup
	vsCRD             = "volumesnapshots." + StorageSnapshotGroup
	dummyProvisioner  = "dummy-provisioner"
)

func preflightTestCases(serverVersion string) {

	Describe("Preflight unit test cases", func() {

		Describe("Preflight run command volume snapshot CRD test cases", func() {

			AfterEach(func() {
				deleteAllVolumeSnapshotCRD()
			})

			Context("When preflight run command executed with/without volume snapshot CRD on cluster", func() {

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

		Describe("Preflight run command volume snapshot class test cases", func() {

			var (
				crVersion string
				err       error
			)

			BeforeEach(func() {
				vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
				installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
				crVersion, err = getPrefSnapshotClassVersion(serverVersion)
				Expect(err).To(BeNil())
			})

			AfterEach(func() {
				deleteAllVolumeSnapshotClass(crVersion)
				deleteAllVolumeSnapshotCRD()
			})

			Context("When preflight run command executed with/without volume snapshot class on cluster", func() {

				It("Should skip installation if volume snapshot class is present", func() {
					installVolumeSnapshotClass(crVersion, dummyProvisioner, defaultVSCName)
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion, defaultVSCName)
				})

				It("Should install volume snapshot class with default name when volume snapshot class doesn't exists", func() {
					deleteAllVolumeSnapshotClass(crVersion)
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion, defaultVSCName)
				})

				It("Should install volume snapshot class with default name when volume snapshot class exists but with"+
					" a different driver", func() {
					installVolumeSnapshotClass(crVersion, "dummy-provisioner-2", "another-snapshot-class")
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion, defaultVSCName)
				})

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
			dirVersion, err := getPrefSnapshotClassVersion(version)
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

func installVolumeSnapshotClass(version, driver, vscName string) {
	vscUnstrObj := &unstructured.Unstructured{}
	vscUnstrObj.SetUnstructuredContent(map[string]interface{}{
		"driver":         driver,
		"deletionPolicy": "Delete",
	})
	vscGVK := schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: version,
		Kind:    internal.VolumeSnapshotClassKind,
	}
	vscUnstrObj.SetGroupVersionKind(vscGVK)
	vscUnstrObj.SetName(vscName)
	Expect(k8sClient.Create(ctx, vscUnstrObj)).To(BeNil())
	Eventually(func() error {
		vscObj := &unstructured.Unstructured{}
		vscObj.SetGroupVersionKind(vscGVK)
		return k8sClient.Get(ctx, types.NamespacedName{Name: vscName}, vscObj)
	}, timeout, interval).ShouldNot(HaveOccurred())
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

func checkVolumeSnapshotClassExists(version, vscName string) {
	vscUnstrObj := &unstructured.Unstructured{}
	vscUnstrObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: version,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	vscUnstrObj.SetName(vscName)
	Eventually(func() error {
		return k8sClient.Get(ctx, types.NamespacedName{Name: vscName}, vscUnstrObj)
	}, timeout, interval).ShouldNot(HaveOccurred())
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

func deleteAllVolumeSnapshotClass(version string) {
	var vscGVK = schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: version,
		Kind:    internal.VolumeSnapshotClassKind,
	}
	vscUnstrObjList := &unstructured.UnstructuredList{}
	vscUnstrObjList.SetGroupVersionKind(vscGVK)
	Eventually(func() error {
		return k8sClient.List(ctx, vscUnstrObjList)
	}, timeout, interval).ShouldNot(HaveOccurred())

	for _, vsc := range vscUnstrObjList.Items {
		vscUnstrObj := &unstructured.Unstructured{}
		vscUnstrObj.SetGroupVersionKind(vscGVK)
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: vsc.GetName()}, vscUnstrObj)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(k8sClient.Delete(ctx, vscUnstrObj)).To(BeNil())
			return false
		}, timeout, interval).Should(BeTrue())
	}
}
