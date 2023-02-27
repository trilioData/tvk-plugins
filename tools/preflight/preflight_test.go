package preflight

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
)

const (
	v1beta1K8sVersion        = "v1.19.0"
	v1K8sVersion             = "v1.20.0"
	timeout                  = time.Second * 60
	interval                 = time.Second * 1
	vsClassCRD               = "volumesnapshotclasses." + StorageSnapshotGroup
	vsContentCRD             = "volumesnapshotcontents." + StorageSnapshotGroup
	vsCRD                    = "volumesnapshots." + StorageSnapshotGroup
	dummyProvisioner         = "dummy-provisioner"
	dummyVolumeSnapshotClass = "dummy-vsc"
)

func preflightCSITestcases(serverVersion string) {

	Describe("Preflight run command volume snapshot CRD test cases", func() {

		AfterEach(func() {
			deleteAllVolumeSnapshotCRD()
		})

		Context("When preflight run command executed with/without volume snapshot CRD on cluster", func() {

			It("Should skip installation if all volume snapshot CRDs are present", func() {
				vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
				installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
				Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotCRDExists()
			})

			for i, crd := range VolumeSnapshotCRDs {
				vsCRDsMap := map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
				It(fmt.Sprintf("Should install missing volume snapshot CRD %s when it is not present", crd), func() {
					vsCRDsMap[VolumeSnapshotCRDs[i]] = false
					installVolumeSnapshotCRD(serverVersion, vsCRDsMap)
					Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, testClient.RuntimeClient)).To(BeNil())
					checkVolumeSnapshotCRDExists()
				})
			}

			It("Should install all volume snapshot CRDs when none of them are present", func() {
				deleteAllVolumeSnapshotCRD()
				Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, testClient.RuntimeClient)).To(BeNil())
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
			deleteAllVolumeSnapshotCRD()
		})

		Context("When preflight run command executed without volume snapshot class flag", func() {

			It("Should skip installation if volume snapshot class is present", func() {
				installVolumeSnapshotClass(crVersion, dummyProvisioner, dummyVolumeSnapshotClass)
				Expect(runOps.validateStorageSnapshotClass(ctx, dummyProvisioner, crVersion,
					testClient.ClientSet, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotClassExists(dummyVolumeSnapshotClass, crVersion, 1)
				deleteAllVolumeSnapshotClass(crVersion, 1)
			})

			It("Should install volume snapshot class with default name when volume snapshot class doesn't exists", func() {
				Expect(runOps.validateStorageSnapshotClass(ctx, dummyProvisioner, crVersion,
					testClient.ClientSet, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotClassExists("", crVersion, 1)
				deleteAllVolumeSnapshotClass(crVersion, 1)
			})

			It("Should install volume snapshot class with default name when volume snapshot class exists but with"+
				" a different driver", func() {
				installVolumeSnapshotClass(crVersion, "dummy-provisioner-2", "another-snapshot-class")
				Expect(runOps.validateStorageSnapshotClass(ctx, dummyProvisioner, crVersion,
					testClient.ClientSet, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotClassExists("", crVersion, 2)
				deleteAllVolumeSnapshotClass(crVersion, 2)
			})

		})

		Context("When preflight run command executed with volume snapshot class name on cluster", func() {

			It("Should skip installation if volume snapshot class with provided name is present", func() {
				runOps.SnapshotClass = dummyVolumeSnapshotClass
				installVolumeSnapshotClass(crVersion, dummyProvisioner, dummyVolumeSnapshotClass)
				Expect(runOps.validateStorageSnapshotClass(ctx, dummyProvisioner, crVersion,
					testClient.ClientSet, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotClassExists(dummyVolumeSnapshotClass, crVersion, 1)
				deleteAllVolumeSnapshotClass(crVersion, 1)
			})

			It("Should fail when volume snapshot class with provided name doesn't exist", func() {
				runOps.SnapshotClass = "abc"
				err = runOps.validateStorageSnapshotClass(ctx, dummyProvisioner, crVersion,
					testClient.ClientSet, testClient.RuntimeClient)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("Should create volume snapshot class when volume snapshot CRDs doesn't exist and override provided name", func() {
				runOps.SnapshotClass = dummyVolumeSnapshotClass
				deleteAllVolumeSnapshotCRD()
				Expect(runOps.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotCRDExists()
				Expect(runOps.SnapshotClass).To(Equal(""))
				Expect(runOps.validateStorageSnapshotClass(ctx, dummyProvisioner, crVersion,
					testClient.ClientSet, testClient.RuntimeClient)).To(BeNil())
				checkVolumeSnapshotClassExists("", crVersion, 1)
				deleteAllVolumeSnapshotClass(crVersion, 1)
			})

		})

	})

}

func preflightFuncsTestcases() {
	Describe("Preflight kubectl binary check test cases", func() {

		Context("Check whether kubectl binary is present on the system", func() {

			It("Should be able to find kubectl binary when correct binary name is provided", func() {
				err := runOps.validateKubectl(kubectlBinaryName)
				Expect(err).To(BeNil())
			})

			It("Should return error when invalid kubectl binary name is provided", func() {
				err := runOps.validateKubectl(invalidKubectlBinaryName)
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
						"helm does not meet minimum version requirement.\nUpgrade helm to minimum version - %s", minHelmVersion)))
				})

			})

			It("Should pass helm check if correct binary name is provided", func() {
				err := runOps.validateSystemHelmVersion(HelmBinaryName, testClient.DiscClient)
				Expect(err).To(BeNil())
			})

			It("Should fail helm binary check if invalid binary name is provided", func() {
				err := runOps.validateSystemHelmVersion(invalidHelmBinaryName, testClient.DiscClient)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
					"error finding '%s' binary in $PATH of the system ::", invalidHelmBinaryName)))
			})
		})
	})

	Describe("Preflight kubernetes server version check test cases", func() {

		Context("When kubernetes server version satisfy/not satisfy minimum version requirement", func() {

			It("Should pass kubernetes server version check if minimum version provided is >= threshold minimum version", func() {
				err := runOps.validateKubernetesVersion(testMinK8sVersion, testClient.ClientSet)
				Expect(err).To(BeNil())
			})

			It("Should return error when kubernetes server version is less than the minimum required version", func() {
				err := runOps.validateKubernetesVersion(invalidK8sVersion, testClient.ClientSet)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("kubernetes server version does not meet minimum requirements"))
			})
		})
	})

	Describe("Preflight kubernetes server default namespace access check", func() {

		Context("When namespace exists on cluster", func() {

			It("Should be able to access default namespace of the cluster", func() {
				err := runOps.validateClusterAccess(ctx, internal.DefaultNs, testClient.ClientSet)
				Expect(err).To(BeNil())
			})
		})

		Context("When namespace does not exist on cluster", func() {
			It("Should not able access non-existent namesapce and return error", func() {
				err := runOps.validateClusterAccess(ctx, invalidNamespace, testClient.ClientSet)
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("Preflight volume snapshot resources test scenarios", func() {

		Context("When creating source pod from pvc", func() {
			var (
				nameSuffix string
				err        error
				pvc        = &corev1.PersistentVolumeClaim{}
				pod        = &corev1.Pod{}
				pvcKey     types.NamespacedName
				podKey     types.NamespacedName
				resultChan = make(chan error, 1)
				once       sync.Once
			)

			BeforeEach(func() {
				once.Do(func() {
					nameSuffix, err = CreateResourceNameSuffix()
					pvcKey = types.NamespacedName{
						Name:      testPVC + "-" + internal.GenerateRandomString(6, false),
						Namespace: installNs,
					}
					podKey = types.NamespacedName{
						Name:      SourcePodNamePrefix + nameSuffix,
						Namespace: installNs,
					}
				})
			})

			It("Should create source pod with appropriate spec fields", func() {
				structPod := createVolumeSnapshotPodSpec(pvcKey.Name, &runOps, nameSuffix)

				// check volume fields
				val := structPod.Spec.Volumes[0].Name
				Expect(val).To(Equal(VolMountName))
				val = structPod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName
				Expect(val).To(Equal(pvcKey.Name))

				verifyTVKResourceLabels(structPod, nameSuffix)
			})

			It("Should create source pod from pvc in bound state", func() {
				// create pvc in bound state
				err = createTestPVC(pvcKey)
				Expect(err).To(BeNil())
				Eventually(func() error {
					return testClient.RuntimeClient.Get(ctx, pvcKey, pvc)
				}, timeout, interval).Should(BeNil())

				// Bind the PVC
				pvc.Status.Phase = corev1.ClaimBound
				Expect(testClient.RuntimeClient.Status().Update(ctx, pvc)).To(BeNil())
				Eventually(func() bool {
					Expect(testClient.RuntimeClient.Get(ctx, pvcKey, pvc)).To(BeNil())
					return pvc.Status.Phase == corev1.ClaimBound
				})

				// create pod from pvc
				go func(pvcName string) {
					var testErr error
					pod, testErr = runOps.createSourcePodFromPVC(ctx, nameSuffix, pvcName, testClient.ClientSet)
					resultChan <- testErr
				}(pvc.GetName())

				// Get the source pod from cache
				Eventually(func() error {
					return testClient.RuntimeClient.Get(ctx, podKey, pod)
				}, timeout, interval).Should(BeNil())

				// Update the status of pod to ready
				Expect(testClient.RuntimeClient.Get(ctx, podKey, pod)).To(BeNil())
				pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: time.Now()},
				})
				Expect(testClient.RuntimeClient.Status().Update(ctx, pod)).To(BeNil())

				Expect(<-resultChan).To(BeNil())

				// delete source pod
				deleteTestPod(podKey)
			})
		})

		Context("When creating volume snapshot from pvc", Ordered, func() {

			var (
				volSnapKey types.NamespacedName
				volSnap    = &unstructured.Unstructured{}
				vsCRDsMap  = map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
				resultChan = make(chan error, 1)
			)

			BeforeAll(func() {
				// Install the volume snapshot CRDs
				installVolumeSnapshotCRD(v1K8sVersion, vsCRDsMap)
				checkVolumeSnapshotCRDExists()

				// create volume-snapshot on cluster
				volSnapKey = types.NamespacedName{
					Name:      testVolumeSnapshot,
					Namespace: runOps.Namespace,
				}

				volSnap.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   StorageSnapshotGroup,
					Version: internal.V1Version,
					Kind:    internal.VolumeSnapshotKind,
				})
			})

			AfterAll(func() {
				deleteAllVolumeSnapshotCRD()
			})

			It("Should have correct spec fields and metadata labels for volume-snapshot created from PVC", func() {
				volSnap = createVolumeSnapsotSpec(volSnapKey.Name, testSnapshotClass, runOps.Namespace, internal.V1Version, testPVC, testNameSuffix)

				// check namespace
				Expect(volSnap.GetNamespace()).To(Equal(runOps.Namespace))

				// correct pvc name
				pvcMap, found, err := unstructured.NestedStringMap(volSnap.Object, "spec", "source")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				val, ok := pvcMap["persistentVolumeClaimName"]
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal(testPVC))

				//snapshot-class name check
				vscName, found, err := unstructured.NestedFieldNoCopy(volSnap.Object, "spec", "volumeSnapshotClassName")
				Expect(found).Should(BeTrue())
				Expect(err).To(BeNil())
				Expect(vscName).To(Equal(testSnapshotClass))

				// label verification
				verifyTVKResourceLabels(volSnap, testNameSuffix)
			})

			It("Should not return error when volume-snapshot becomes readyToUse", func() {
				Skip("TODO - working as expected in local. Will be handled in suite refactoring.")
				go func() {
					_, testErr := runOps.createSnapshotFromPVC(ctx, volSnapKey.Name, testSnapshotClass,
						internal.V1Version, testPVC, testNameSuffix, testClient)
					resultChan <- testErr
				}()
				Eventually(func() error {
					return testClient.RuntimeClient.Get(ctx, volSnapKey, volSnap)
				}, timeout, interval).Should(BeNil())

				// update the readyToUse field
				Eventually(func() error {
					return testClient.RuntimeClient.Get(ctx, volSnapKey, volSnap)
				}, timeout, interval).ShouldNot(HaveOccurred())
				Expect(unstructured.SetNestedField(volSnap.Object, true, "status", "readyToUse")).To(BeNil())
				Expect(testClient.RuntimeClient.Status().Update(ctx, volSnap)).To(BeNil())

				// volume-snapshot should be ready-to-use
				Eventually(func() bool {
					Expect(testClient.RuntimeClient.Get(ctx, volSnapKey, volSnap)).To(BeNil())
					ready, found, err := unstructured.NestedBool(volSnap.Object, "status", "readyToUse")
					Expect(found).To(BeTrue())
					Expect(err).To(BeNil())
					return ready
				}, timeout, interval).Should(BeTrue())

				// successfully complete execution of func
				Expect(<-resultChan).To(BeNil())
			})
		})

		Context("When creating restore pod from volume snapshot", Ordered, func() {
			var (
				volSnapKey    types.NamespacedName
				restorePVCKey types.NamespacedName
				podKey        types.NamespacedName
				volSnap       = &unstructured.Unstructured{}
				pvc           = &corev1.PersistentVolumeClaim{}
				vsCRDsMap     = map[string]bool{vsClassCRD: true, vsContentCRD: true, vsCRD: true}
			)

			BeforeAll(func() {
				installVolumeSnapshotCRD(v1K8sVersion, vsCRDsMap)
				checkVolumeSnapshotCRDExists()

				volSnap.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   StorageSnapshotGroup,
					Version: internal.V1Version,
					Kind:    internal.VolumeSnapshotKind,
				})
				volSnapKey = types.NamespacedName{
					Name:      testVolumeSnapshot,
					Namespace: installNs,
				}
			})

			BeforeEach(func() {
				restorePVCKey = types.NamespacedName{
					Name:      testPVC + "-" + internal.GenerateRandomString(6, false),
					Namespace: installNs,
				}
				podKey = types.NamespacedName{
					Name:      testPodName + "-" + internal.GenerateRandomString(6, false),
					Namespace: installNs,
				}
			})

			AfterAll(func() {
				deleteAllVolumeSnapshotCRD()
			})

			It("Should create pvc for restore pod with volume snapshot as its data-source", func() {
				pvc = createRestorePVCSpec(restorePVCKey.Name, volSnapKey.Name, testNameSuffix, &runOps)

				Expect(pvc.Namespace).Should(Equal(runOps.Namespace))
				Expect(*pvc.Spec.StorageClassName).Should(Equal(runOps.StorageClass))

				verifyTVKResourceLabels(pvc, testNameSuffix)
			})

			It("Should create pod using restore PVC", func() {
				pod := createRestorePodSpec(podKey.Name, restorePVCKey.Name, testNameSuffix, &runOps)

				Expect(pod.Spec.Volumes).ShouldNot(BeNil())
				Expect(len(pod.Spec.Volumes)).ShouldNot(BeZero())

				Expect(pod.Spec.Volumes[0].Name).Should(Equal(VolMountName))
				Expect(pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).Should(Equal(restorePVCKey.Name))

				verifyTVKResourceLabels(pod, testNameSuffix)
			})
		})
	})

	Describe("Preflight DNS Pod test scenarios", Ordered, func() {
		var (
			podKey     types.NamespacedName
			resultChan = make(chan error)
		)

		BeforeAll(func() {
			podKey = types.NamespacedName{
				Name:      dnsUtils + testNameSuffix,
				Namespace: installNs,
			}
		})

		It("Should create DNS pod with appropriate spec values", func() {
			pod := createDNSPodSpec(&runOps, testNameSuffix)

			Expect(len(pod.Spec.Containers)).ShouldNot(BeZero())
			container := pod.Spec.Containers[0]
			Expect(container.Image).Should(Equal(strings.Join([]string{GcrRegistryPath, DNSUtilsImage}, "/")))
			Expect(container.Name).Should(Equal(dnsContainerName))

			resreq := container.Resources
			Expect(resreq).ShouldNot(BeNil())
			Expect(func() bool {
				limits := resreq.Limits
				requests := resreq.Requests
				return runOps.Limits.Cpu().String() == limits.Cpu().String() &&
					runOps.Limits.Memory().String() == limits.Memory().String() &&
					runOps.Requests.Cpu().String() == requests.Cpu().String() &&
					runOps.Requests.Memory().String() == requests.Memory().String()
			}()).Should(BeTrue())

			verifyTVKResourceLabels(pod, testNameSuffix)
		})

		It("DNS pod should be created without any error after reaching into ready state", func() {
			go func() {
				_, testErr := runOps.createDNSPodOnCluster(ctx, testNameSuffix, testClient.ClientSet)
				resultChan <- testErr
			}()
			structPod := &corev1.Pod{}
			Eventually(func() error {
				return testClient.RuntimeClient.Get(ctx, podKey, structPod)
			}, timeout, interval).Should(BeNil())

			// udpate pod to ready condition
			structPod.Status.Conditions = append(structPod.Status.Conditions, corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			Expect(testClient.RuntimeClient.Status().Update(ctx, structPod)).To(BeNil())

			Eventually(func() bool {
				Expect(testClient.RuntimeClient.Get(ctx, podKey, structPod)).To(BeNil())
				for _, cond := range structPod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			Expect(<-resultChan).To(BeNil())

			// delete dns pod
			deleteTestPod(podKey)
		})
	})

	Describe("Preflight pod capability test scenarios", func() {
		var (
			podKey         types.NamespacedName
			resultChan     = make(chan error)
			testCapability capability
		)

		BeforeEach(func() {
			podKey = types.NamespacedName{
				Name:      podCapability + testNameSuffix,
				Namespace: installNs,
			}
			testCapability = capability{
				userID:                   1001,
				allowPrivilegeEscalation: false,
				privileged:               false,
			}
		})

		It("Should create capability validator pod with appropriate spec values", func() {
			pod := createPodSpecWithCapability(&runOps, testNameSuffix, testCapability)

			Expect(len(pod.Spec.Containers)).ShouldNot(BeZero())
			containerSecurityContext := pod.Spec.Containers[0].SecurityContext

			podSecurityContext := pod.Spec.SecurityContext
			Expect(func() bool {
				return *podSecurityContext.RunAsUser == testCapability.userID &&
					*podSecurityContext.RunAsNonRoot == (testCapability.userID != 0)
			}()).Should(BeTrue())
			Expect(func() bool {
				return testCapability.allowPrivilegeEscalation == *containerSecurityContext.AllowPrivilegeEscalation &&
					testCapability.privileged == *containerSecurityContext.Privileged &&
					*containerSecurityContext.ReadOnlyRootFilesystem == false &&
					len(containerSecurityContext.Capabilities.Add) != 0
			}()).Should(BeTrue())

			verifyTVKResourceLabels(pod, testNameSuffix)
		})

		It("capability validator pod should be created without any error and should be in ready state", func() {
			go func() {
				testErr := runOps.validatePodCapability(ctx, testNameSuffix, testClient, testCapability)
				resultChan <- testErr
			}()
			structPod := &corev1.Pod{}
			Eventually(func() error {
				return testClient.RuntimeClient.Get(ctx, podKey, structPod)
			}, timeout, interval).Should(BeNil())

			// udpate pod to ready condition
			structPod.Status.Conditions = append(structPod.Status.Conditions, corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			Expect(testClient.RuntimeClient.Status().Update(ctx, structPod)).To(BeNil())

			Eventually(func() bool {
				Expect(testClient.RuntimeClient.Get(ctx, podKey, structPod)).To(BeNil())
				for _, cond := range structPod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			Expect(<-resultChan).To(BeNil())

		})

	})

	Context("Check rbac-API group and version on the cluster", func() {

		It("Should pass RBAC check when correct rbac-API group and version is provided", func() {
			err := runOps.validateKubernetesRBAC(RBACAPIGroup, RBACAPIVersion, testClient.DiscClient)
			Expect(err).To(BeNil())
		})

		It("Should return error when rbac-API group and version is not present on server", func() {
			err := runOps.validateKubernetesRBAC(invalidRBACAPIGroup, RBACAPIVersion, testClient.DiscClient)
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
}

var _ = Context("Preflight Unit Tests", func() {
	preflightCSITestcases(v1beta1K8sVersion)
	preflightCSITestcases(v1K8sVersion)

	preflightFuncsTestcases()
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

			Expect(testClient.RuntimeClient.Create(ctx, crdObj)).To(BeNil())
			Eventually(func() error {
				volSnapCRDObj := &apiextensions.CustomResourceDefinition{}
				return testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: VolumeSnapshotCRDs[i]}, volSnapCRDObj)
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
	Eventually(func() error {
		return testClient.RuntimeClient.Create(ctx, vscUnstrObj)
	}, timeout, interval).ShouldNot(HaveOccurred())
	Eventually(func() error {
		vscObj := &unstructured.Unstructured{}
		vscObj.SetGroupVersionKind(vscGVK)
		return testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: vscName}, vscObj)
	}, timeout, interval).ShouldNot(HaveOccurred())
	vscObj := &unstructured.Unstructured{}
	vscObj.SetGroupVersionKind(vscGVK)
	Expect(testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: vscName}, vscObj)).To(BeNil())
}

func checkVolumeSnapshotCRDExists() {
	for _, crd := range VolumeSnapshotCRDs {
		crdObj := &apiextensions.CustomResourceDefinition{}
		crdObj.SetName(crd)
		Eventually(func() error {
			return testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
		}, timeout, interval).ShouldNot(HaveOccurred())
	}
}

func checkVolumeSnapshotClassExists(vscName, version string, expectedVscCount int) {
	vscUnstrObjList := &unstructured.UnstructuredList{}
	vscUnstrObjList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: version,
		Kind:    internal.VolumeSnapshotClassKind + "List",
	})
	Eventually(func() bool {
		if err := testClient.RuntimeClient.List(ctx, vscUnstrObjList); err != nil {
			return false
		}
		return len(vscUnstrObjList.Items) > 0
	}, timeout, interval).Should(BeTrue())
	Expect(len(vscUnstrObjList.Items)).To(Equal(expectedVscCount))

	if vscName == "" {
		vscName = defaultVSCNamePrefix
	}
	var found bool
	for _, vsc := range vscUnstrObjList.Items {
		if strings.Contains(vsc.GetName(), vscName) {
			Eventually(func() error {
				return testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: vsc.GetName()}, &vsc)
			}, timeout, interval).ShouldNot(HaveOccurred())
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}

func deleteAllVolumeSnapshotCRD() {
	for _, crd := range VolumeSnapshotCRDs {
		crdObj := &apiextensions.CustomResourceDefinition{}
		crdObj.SetName(crd)
		Eventually(func() bool {
			err := testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(testClient.RuntimeClient.Delete(ctx, crdObj)).To(BeNil())
			return false
		}, timeout, interval).Should(BeTrue())
	}
}

func deleteAllVolumeSnapshotClass(version string, vscCountToDelete int) {
	vscUnstrObjList := &unstructured.UnstructuredList{}
	vscUnstrObjList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: version,
		Kind:    internal.VolumeSnapshotClassKind + "List",
	})
	Eventually(func() bool {
		if err := testClient.RuntimeClient.List(ctx, vscUnstrObjList); err != nil {
			return false
		}
		return len(vscUnstrObjList.Items) == vscCountToDelete
	}, timeout, interval).Should(BeTrue())

	for _, vsc := range vscUnstrObjList.Items {
		Eventually(func() bool {
			err := testClient.RuntimeClient.Get(ctx, types.NamespacedName{Name: vsc.GetName()}, &vsc)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(testClient.RuntimeClient.Delete(ctx, &vsc)).To(BeNil())
			return false
		}, timeout, interval).Should(BeTrue())
	}
}
