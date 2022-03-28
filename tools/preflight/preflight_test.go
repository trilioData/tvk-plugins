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

func preflightCSITestcases(serverVersion string) {

	Describe("Preflight unit test cases", func() {

		BeforeEach(func() {
			run = &Run{CommonOptions: CommonOptions{Logger: logger}}
		})

		Describe("Preflight run command volume snapshot CRD test cases", func() {

			AfterEach(func() {
				deleteAllVolumeSnapshotCRD()
			})

			Context("When preflight run command executed with/without volume snapshot CRD on cluster", func() {

				It("Should skip installation if all volume snapshot CRDs are present", func() {
					vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
					installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
					Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, contClient)).To(BeNil())
					checkVolumeSnapshotCRDExists()
				})

				for i, crd := range VolumeSnapshotCRDs {
					vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
					It(fmt.Sprintf("Should install missing volume snapshot CRD %s when it is not present", crd), func() {
						vsCRDsMap[VolumeSnapshotCRDs[i]] = false
						installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
						Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, contClient)).To(BeNil())
						checkVolumeSnapshotCRDExists()
					})
				}

				It("Should install all volume snapshot CRDs when none of them are present", func() {
					deleteAllVolumeSnapshotCRD()
					Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, contClient)).To(BeNil())
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

			Context("When preflight run command executed without volume snapshot class flag", func() {

				It("Should skip installation if volume snapshot class is present", func() {
					installVolumeSnapshotClass(crVersion, dummyProvisioner, defaultVSCName)
					Expect(runOps.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion, contClient)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion, defaultVSCName)
				})

				It("Should install volume snapshot class with default name when volume snapshot class doesn't exists", func() {
					deleteAllVolumeSnapshotClass(crVersion)
					Expect(runOps.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion, contClient)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion, defaultVSCName)
				})

				It("Should install volume snapshot class with default name when volume snapshot class exists but with"+
					" a different driver", func() {
					installVolumeSnapshotClass(crVersion, "dummy-provisioner-2", "another-snapshot-class")
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion)
				})

			})

			Context("When preflight run command executed with volume snapshot class name on cluster", func() {

				It("Should skip installation if volume snapshot class with provided name is present", func() {
					run.SnapshotClass = defaultVSCName
					installVolumeSnapshotClass(crVersion, dummyProvisioner, defaultVSCName)
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion)
				})

				It("Should fail when volume snapshot class with provided name doesn't exist", func() {
					run.SnapshotClass = "abc"
					err = run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring("not found"))
				})

				It("Should create volume snapshot class when volume snapshot CRDs doesn't exist", func() {
					run.SnapshotClass = defaultVSCName
					deleteAllVolumeSnapshotCRD()
					Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
					checkVolumeSnapshotCRDExists()
					Expect(run.SnapshotClass).To(Equal(""))
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion)
				})

			})

		})

	})

}

var _ = Context("Preflight Unit Tests", func() {
	preflightCSITestcases(v1beta1K8sVersion)
	preflightCSITestcases(v1K8sVersion)

	Describe("Preflight kubectl binary check test cases", func() {

		Context("Check whether kubectl binary is present on the system", func() {

			It("Should be able to find kubectl binary when correct binary name is provided", func() {
				err := runOps.checkKubectl(kubectlBinaryName)
				Expect(err).To(BeNil())
			})

			It("Should return error when invalid kubectl binary name is provided", func() {
				err := runOps.checkKubectl(invalidKubectlBinaryName)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(
					fmt.Sprintf("error finding '%s' binary in $PATH of the system ::", invalidKubectlBinaryName)))
			})
		})
	})

	Describe("Preflight helm binary check test cases", func() {

		Context("When valid/invalid helm binary name is provided", func() {

			Context("When helm binary does not satisfy minimum version requirement", func() {

				It("Should return error when helm version does not satisfy the minimum required helm version", func() {
					err := runOps.validateHelmVersion(invalidHelmVersion)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
						"helm does not meet minimum version requirement.\nUpgrade helm to minimum version - %s", MinHelmVersion)))
				})

			})

			It("Should pass helm check if correct binary name is provided", func() {
				err := runOps.checkHelmVersion(HelmBinaryName, k8sDiscClient)
				Expect(err).To(BeNil())
			})

			It("Should fail helm binary check if invalid binary name is provided", func() {
				err := runOps.checkHelmVersion(invalidHelmBinaryName, k8sDiscClient)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
					"error finding '%s' binary in $PATH of the system ::", invalidHelmBinaryName)))
			})
		})
	})

	Describe("Preflight kubernetes server version check test cases", func() {

		Context("When kubernetes server version satisfy/not satisfy minimum version requirement", func() {

			It("Should pass kubernetes server version check if minimum version provided is >= threshold minimum version", func() {
				err := runOps.checkKubernetesVersion(testMinK8sVersion, k8sClient)
				Expect(err).To(BeNil())
			})

			It("Should return error when kubernetes server version is less than the minimum required version", func() {
				err := runOps.checkKubernetesVersion(invalidK8sVersion, k8sClient)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("kubernetes server version does not meet minimum requirements"))
			})
		})
	})

	Context("Check rbac-API group and version on the cluster", func() {

		It("Should pass RBAC check when correct rbac-API group and version is provided", func() {
			err := runOps.checkKubernetesRBAC(RBACAPIGroup, RBACAPIVersion, k8sDiscClient)
			Expect(err).To(BeNil())
		})

		It("Should return error when rbac-API group and version is not present on server", func() {
			err := runOps.checkKubernetesRBAC(invalidRBACAPIGroup, RBACAPIVersion, k8sDiscClient)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("not enabled kubernetes RBAC"))
		})
	})

	Context("Generate a random and unique 6-length preflight UID", func() {
		It("Should create uid of length 6", func() {
			uid, err := CreateResourceNameSuffix()
			Expect(err).To(BeNil())
			Expect(len(uid)).To(Equal(6))
		})
	})
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

			Expect(contClient.Create(ctx, crdObj)).To(BeNil())
			Eventually(func() error {
				volSnapCRDObj := &apiextensions.CustomResourceDefinition{}
				return contClient.Get(ctx, types.NamespacedName{Name: VolumeSnapshotCRDs[i]}, volSnapCRDObj)
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
	Expect(contClient.Create(ctx, vscUnstrObj)).To(BeNil())
	Eventually(func() error {
		vscObj := &unstructured.Unstructured{}
		vscObj.SetGroupVersionKind(vscGVK)
		return contClient.Get(ctx, types.NamespacedName{Name: vscName}, vscObj)
	}, timeout, interval).ShouldNot(HaveOccurred())
}

func checkVolumeSnapshotCRDExists() {
	for _, crd := range VolumeSnapshotCRDs {
		crdObj := &apiextensions.CustomResourceDefinition{}
		crdObj.SetName(crd)
		Eventually(func() error {
			return contClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
		}, timeout, interval).ShouldNot(HaveOccurred())
	}
}

func checkVolumeSnapshotClassExists(version string) {
	vscUnstrObj := &unstructured.Unstructured{}
	vscUnstrObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: version,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	vscUnstrObj.SetName(defaultVSCName)
	Eventually(func() error {
		return k8sClient.Get(ctx, types.NamespacedName{Name: defaultVSCName}, vscUnstrObj)
	}, timeout, interval).ShouldNot(HaveOccurred())
}

func deleteAllVolumeSnapshotCRD() {
	for _, crd := range VolumeSnapshotCRDs {
		crdObj := &apiextensions.CustomResourceDefinition{}
		crdObj.SetName(crd)
		Eventually(func() bool {
			err := contClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(contClient.Delete(ctx, crdObj)).To(BeNil())
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
		return contClient.List(ctx, vscUnstrObjList)
	}, timeout, interval).ShouldNot(HaveOccurred())

	for _, vsc := range vscUnstrObjList.Items {
		vscUnstrObj := &unstructured.Unstructured{}
		vscUnstrObj.SetGroupVersionKind(vscGVK)
		Eventually(func() bool {
			err := contClient.Get(ctx, types.NamespacedName{Name: vsc.GetName()}, vscUnstrObj)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(contClient.Delete(ctx, vscUnstrObj)).To(BeNil())
			return false
		}, timeout, interval).Should(BeTrue())
	}
}
