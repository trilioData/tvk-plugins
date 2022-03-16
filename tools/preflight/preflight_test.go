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

			Context("When preflight run command executed without volume snapshot class flag", func() {

				It("Should skip installation if volume snapshot class is present", func() {
					installVolumeSnapshotClass(crVersion, dummyProvisioner, defaultVSCName)
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion)
				})

				It("Should install volume snapshot class with default name when volume snapshot class doesn't exists", func() {
					deleteAllVolumeSnapshotClass(crVersion)
					Expect(run.checkStorageSnapshotClass(ctx, dummyProvisioner, crVersion)).To(BeNil())
					checkVolumeSnapshotClassExists(crVersion)
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

var _ = Describe("Preflight Unit Tests", func() {

	Context("Check kubectl", func() {
		It("Should be able to find kubectl binary", func() {
			err := runOps.checkKubectl(kubectlBinaryName)
			Expect(err).To(BeNil())
		})

		It("Should return err for invalid kubectl binary name", func() {
			err := runOps.checkKubectl(invalidKubectlBinaryName)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("error finding '%s' binary in $PATH of the system ::", invalidKubectlBinaryName)))
		})
	})

	// TODO
	Context("Check cluster access", func() {})

	Context("Check helm binary and version", func() {
		It("Should pass helm check", func() {
			err := runOps.checkHelmVersion(HelmBinaryName)
			Expect(err).To(BeNil())
		})

		It("Should fail helm binary check if invalid binary name is provided", func() {
			err := runOps.checkHelmVersion(invalidHelmBinaryName)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"error finding '%s' binary in $PATH of the system ::", invalidHelmBinaryName)))
		})

		It("Should return error when helm version does not satisfy minimum required helm version", func() {
			err := runOps.validateHelmVersion(invalidHelmVersion)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"helm does not meet minimum version requirement.\nUpgrade helm to minimum version - %s", MinHelmVersion)))
		})
	})

	Context("Check kubernetes server version", func() {
		It("Should pass kubernetes server version check", func() {
			err := runOps.checkKubernetesVersion(MinK8sVersion)
			Expect(err).To(BeNil())
		})

		It("Should return error when kubernetes server version is less than minimum required version", func() {
			err := runOps.checkKubernetesVersion(invalidK8sVersion)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("kubernetes server version does not meet minimum requirements"))
		})
	})

	Context("Check rbac-API group and version", func() {
		It("Should pass RBAC check", func() {
			err := runOps.checkKubernetesRBAC(RBACAPIGroup, RBACAPIVersion)
			Expect(err).To(BeNil())
		})

		It("Should return error when rbac-API group and version is not present on server", func() {
			err := runOps.checkKubernetesRBAC(invalidRBACAPIGroup, RBACAPIVersion)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("not enabled kubernetes RBAC"))
		})
	})

	Context("Check storage and snapshot class", func() {
		//It("Should return err when storage class is not present on cluster", func() {
		//	ops := runOps
		//	ops.StorageClass = invalidStorageClass
		//	err := ops.checkStorageSnapshotClass(ctx)
		//	Expect(err).ToNot(BeNil())
		//	Expect(err.Error()).To(ContainSubstring(
		//		fmt.Sprintf("not found storageclass - %s on cluster", invalidStorageClass)))
		//})

		//It("Should return error when storage class provisioner has no matching snapshot class driver", func() {
		//	createStorageClass(testStorageClass, testProvisioner)
		//	defer deleteStorageClass(testStorageClass)
		//	ops := runOps
		//	ops.StorageClass = testStorageClass
		//	err := ops.checkStorageSnapshotClass(ctx)
		//	Expect(err).ToNot(BeNil())
		//})

		//It("Should return error when given snapshot class driver does not match with storage class provisioner", func() {
		//	createStorageClass(testStorageClass, testProvisioner)
		//	defer deleteStorageClass(testStorageClass)
		//	createSnapshotClass(testDriver)
		//	defer deleteSnapshotClass()
		//	ops := runOps
		//	ops.StorageClass = testStorageClass
		//	ops.SnapshotClass = testSnapshotClass
		//	err := ops.checkStorageSnapshotClass(ctx)
		//	Expect(err).ToNot(BeNil())
		//	Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
		//		"volume snapshot class - %s driver does not match with given StorageClass's provisioner=%s",
		//		testSnapshotClass, testProvisioner)))
		//})

		//It("Should pass storage-snapshot check when given storage class provisioner and snapshot class driver match", func() {
		//	createStorageClass(testStorageClass, testProvisioner)
		//	defer deleteStorageClass(testStorageClass)
		//	createSnapshotClass(testProvisioner)
		//	defer deleteSnapshotClass()
		//	ops := runOps
		//	ops.StorageClass = testStorageClass
		//	ops.SnapshotClass = testSnapshotClass
		//	err := ops.checkStorageSnapshotClass(ctx)
		//	Expect(err).To(BeNil())
		//})

		//It("Should pass storage-snapshot checks when storage class provisioner matches with snapshot class driver on cluster", func() {
		//	err := runOps.checkStorageSnapshotClass(ctx)
		//	Expect(err).To(BeNil())
		//})
	})

	Context("Check snapshot class against a provisioner", func() {
		//It("Should find snapshot class with the given provisioner", func() {
		//	createSnapshotClass(testDriver)
		//	defer deleteSnapshotClass()
		//	ops := runOps
		//	ops.SnapshotClass = testSnapshotClass
		//	sscName, err := ops.checkSnapshotclassForProvisioner(ctx, testDriver)
		//	Expect(err).To(BeNil())
		//	Expect(sscName).To(Equal(testSnapshotClass))
		//})

		//It("Should return error when no snapshot class is found against a provisioner", func() {
		//	createSnapshotClass(testDriver)
		//	defer deleteSnapshotClass()
		//	ops := runOps
		//	ops.SnapshotClass = testSnapshotClass
		//	_, err := ops.checkSnapshotclassForProvisioner(ctx, testProvisioner)
		//	Expect(err).ToNot(BeNil())
		//	Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
		//		"no matching volume snapshot class having driver same as provisioner - %s found on cluster", testProvisioner)))
		//})
	})

	// TODO
	Context("Check CSI APIs", func() {
	})

	Context("Check DNS resolution", func() {
		BeforeEach(func() {
			createService(testService, installNs)
		})

		It("Should be able to resolve service on the cluster", func() {
			err := runOps.checkDNSResolution(ctx, execTestServiceCmd, testNameSuffix)
			Expect(err).To(BeNil())
		})

		It("Should return err if not able to resolve a service", func() {
			execCmd := []string{"nslookup", fmt.Sprintf("%s.%s", invalidService, installNs)}
			err := runOps.checkDNSResolution(ctx, execCmd, testNameSuffix)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("not able to resolve DNS '%s' service inside pods", execCmd[1])))
		})

		AfterEach(func() {
			deleteService(testService, installNs)
			deleteTestPod(dnsUtils+testNameSuffix, installNs)
		})
	})

	Context("DNS resolution, namespace testcases", func() {
		It("Should return err when namespace does not exist on cluster", func() {
			ops := runOps
			ops.Namespace = invalidNamespace
			err := ops.checkDNSResolution(ctx, execTestServiceCmd, testNameSuffix)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Check volume snapshot", func() {
		var (
			resourceSuffix string
			err            error
		)

		BeforeEach(func() {
			resourceSuffix, err = CreateResourceNameSuffix()
			Expect(err).To(BeNil())
		})

		It("Should pass volume snapshot check when correct inputs are provided", func() {
			err := runOps.checkVolumeSnapshot(ctx, resourceSuffix)
			Expect(err).To(BeNil())
		})

		It("Should return error when namespace does not exist on cluster", func() {
			ops := runOps
			ops.Namespace = invalidNamespace
			err := ops.checkVolumeSnapshot(ctx, resourceSuffix)
			Expect(err).ToNot(BeNil())
		})

		AfterEach(func() {
			cops := cleanupOps
			cops.UID = resourceSuffix
			cops.CleanupPreflightResources(ctx)
		})
	})

	Context("Create source pod and pvc", func() {
		var (
			resourceSuffix string
			err            error
		)

		BeforeEach(func() {
			resourceSuffix, err = CreateResourceNameSuffix()
			Expect(err).To(BeNil())
		})

		It("Should create source pod and pvc when correct inputs are provided", func() {
			_, _, err = runOps.createSourcePodAndPVC(ctx, resourceSuffix)
			Expect(err).To(BeNil())
		})

		It("Should return error when namespace does not exist on cluster", func() {
			ops := runOps
			ops.Namespace = invalidNamespace
			_, _, err = ops.createSourcePodAndPVC(ctx, resourceSuffix)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when storage class does not exist on cluster", func() {
			ops := runOps
			ops.StorageClass = invalidStorageClass
			_, _, err = ops.createSourcePodAndPVC(ctx, resourceSuffix)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when resource name suffix is empty", func() {
			_, _, err = runOps.createSourcePodAndPVC(ctx, "")
			Expect(err).ToNot(BeNil())
		})

		AfterEach(func() {
			cops := cleanupOps
			cops.UID = resourceSuffix
			cops.CleanupPreflightResources(ctx)
		})
	})

	Context("Create snapshot from pvc", func() {
		It("Should pass snapshot check when correct pvc is provided", func() {
			resourceSuffix, err := CreateResourceNameSuffix()
			Expect(err).To(BeNil())
			pvc, _, err := runOps.createSourcePodAndPVC(ctx, resourceSuffix)
			Expect(err).To(BeNil())
			_, err = runOps.createSnapshotFromPVC(ctx, testSnapPrefix+resourceSuffix,
				defaultSnapshotClass, pvc.GetName(), resourceSuffix)
			Expect(err).To(BeNil())
		})

		It("Should return error when incorrect pvc is provided", func() {
			_, err := runOps.createSnapshotFromPVC(ctx, testSnapPrefix+testNameSuffix,
				defaultSnapshotClass, invalidPVC, testNameSuffix)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Create restore pod and pvc from volume snapshot", func() {
		It("Should create pod and pvc from volume snapshot", func() {
			resourceSuffix, err := CreateResourceNameSuffix()
			Expect(err).To(BeNil())
			cops := cleanupOps
			cops.UID = resourceSuffix
			defer cops.CleanupPreflightResources(ctx)
			pvc, _, err := runOps.createSourcePodAndPVC(ctx, resourceSuffix)
			Expect(err).To(BeNil())
			volSnap, err := runOps.createSnapshotFromPVC(ctx, testSnapPrefix+resourceSuffix,
				defaultSnapshotClass, pvc.GetName(), resourceSuffix)
			Expect(err).To(BeNil())

			_, err = runOps.createRestorePodFromSnapshot(ctx, volSnap,
				testPVCPrefix+resourceSuffix, testPodPrefix+resourceSuffix, resourceSuffix)
			Expect(err).To(BeNil())
		})
	})

	Context("Generate preflight UID", func() {
		It("Should create uid of length 6", func() {
			uid, err := CreateResourceNameSuffix()
			Expect(err).To(BeNil())
			Expect(len(uid)).To(Equal(6))
		})
	})
})
