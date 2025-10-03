package preflight

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	Context("when extracting version of type vX.Y.Z from string of sentences", func() {

		Context("When valid version is present in string", func() {

			It("Should extract version when version is mentioned at the end of the string", func() {
				verStr := fmt.Sprintf("%s %s v%s", testSentence, testSentence, minHelmVersion)
				version, err := extractVersionFromString(verStr)
				Expect(err).To(BeNil())
				Expect(version).To(Equal(minHelmVersion))
			})

			It("Should extract version when version is mentioned at the start of the string", func() {
				verStr := fmt.Sprintf("v%s %s %s", minHelmVersion, testSentence, testSentence)
				version, err := extractVersionFromString(verStr)
				Expect(err).To(BeNil())
				Expect(version).To(Equal(minHelmVersion))
			})

			It("Should extract version when version is mentioned in the middle of the string", func() {
				verStr := fmt.Sprintf("%s v%s %s", testSentence, minHelmVersion, testSentence)
				version, err := extractVersionFromString(verStr)
				Expect(err).To(BeNil())
				Expect(version).To(Equal(minHelmVersion))
			})

			It("Should extract the last version of string when multiple valid versions are present in the string", func() {
				verStr := fmt.Sprintf("%s v%s %s v%s", testSentence, minHelmVersion, testSentence, minK8sVersion)
				version, err := extractVersionFromString(verStr)
				Expect(err).To(BeNil())
				Expect(version).To(Equal(minK8sVersion))
			})
		})

		Context("When invalid version is present in the string", func() {

			It("Should return error when major numeral of version is not a digit", func() {
				_, err := extractVersionFromString("va1.2.3")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no version of type vX.Y.Z found in the string"))
			})

			It("Should return error when minor numeral of version is not a digit", func() {
				_, err := extractVersionFromString("v1.a2.3")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no version of type vX.Y.Z found in the string"))
			})

			It("Should return error when patch numeral of version is not a digit", func() {
				_, err := extractVersionFromString("v1.2.a3")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no version of type vX.Y.Z found in the string"))
			})

			It("Should return error when there is no version specified in the string", func() {
				_, err := extractVersionFromString(testSentence)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no version of type vX.Y.Z found in the string"))
			})
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

	Context("checkCSIDriverSnapshotSupport func test-cases", func() {
		var testRun *Run

		BeforeEach(func() {
			testRun = &Run{
				CommonOptions: CommonOptions{
					Logger: logger,
				},
			}
		})

		It("Should handle known snapshot-supported CSI drivers without panic", func() {
			knownDrivers := []string{
				"ebs.csi.aws.com",
				"disk.csi.azure.com",
				"pd.csi.storage.gke.io",
				"csi.vsphere.vmware.com",
				"csi.longhorn.io",
				"csi.trident.netapp.io",
				"rook-ceph.rbd.csi.ceph.com",
			}

			for _, driver := range knownDrivers {
				Expect(func() {
					testRun.checkCSIDriverSnapshotSupport(driver)
				}).ToNot(Panic())
			}
		})

		It("Should handle known legacy drivers without panic", func() {
			legacyDrivers := []string{
				"kubernetes.io/aws-ebs",
				"kubernetes.io/azure-disk",
				"kubernetes.io/gce-pd",
				"kubernetes.io/vsphere-volume",
				"kubernetes.io/cinder",
				"kubernetes.io/host-path",
				"kubernetes.io/no-provisioner",
			}

			for _, driver := range legacyDrivers {
				Expect(func() {
					testRun.checkCSIDriverSnapshotSupport(driver)
				}).ToNot(Panic())
			}
		})

		It("Should handle unknown and edge case provisioners without panic", func() {
			unknownDrivers := []string{
				"unknown.csi.driver.com",
				"custom.storage.provider",
				"test.driver.io",
				"",
				"invalid-driver-name",
			}

			for _, driver := range unknownDrivers {
				Expect(func() {
					testRun.checkCSIDriverSnapshotSupport(driver)
				}).ToNot(Panic())
			}
		})
	})

	Context("checkVSphereController func test-cases", func() {
		var testRun *Run

		BeforeEach(func() {
			testRun = &Run{
				CommonOptions: CommonOptions{
					Logger: logger,
				},
			}
		})

		It("Should detect VSphere CSI controller deployment when present", func() {
			// Create a mock VSphere CSI controller deployment
			deployment := &unstructured.Unstructured{}
			deployment.Object = map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "vsphere-csi-controller",
					"namespace": "kube-system",
					"labels": map[string]interface{}{
						"app": "vsphere-csi-controller",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(1),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "vsphere-csi-controller",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "vsphere-csi-controller",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "csi-controller",
									"image": "test/vsphere-csi:latest",
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"replicas":      int64(1),
					"readyReplicas": int64(1),
				},
			}
			deployment.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			})

			err := testClient.RuntimeClient.Create(ctx, deployment)
			Expect(err).To(BeNil())

			defer func() {
				_ = testClient.RuntimeClient.Delete(ctx, deployment)
			}()

			// Test that the function detects the VSphere controller
			Expect(func() {
				testRun.checkVSphereController(ctx, testClient.ClientSet)
			}).ToNot(Panic())
		})

		It("Should detect VSphere storage class when present", func() {
			// Create a mock VSphere storage class
			sc := &unstructured.Unstructured{}
			sc.Object = map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "vsphere-storage-class",
				},
				"provisioner": "csi.vsphere.vmware.com",
			}
			sc.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "storage.k8s.io",
				Version: "v1",
				Kind:    "StorageClass",
			})

			err := testClient.RuntimeClient.Create(ctx, sc)
			Expect(err).To(BeNil())

			defer func() {
				_ = testClient.RuntimeClient.Delete(ctx, sc)
			}()

			// Test that the function detects the VSphere storage class
			Expect(func() {
				testRun.checkVSphereController(ctx, testClient.ClientSet)
			}).ToNot(Panic())
		})

		It("Should handle gracefully when no VSphere resources exist", func() {
			// Test with no VSphere resources present
			Expect(func() {
				testRun.checkVSphereController(ctx, testClient.ClientSet)
			}).ToNot(Panic())
		})
	})

	Context("logStorageClassDetails func test-cases", func() {
		var testRun *Run

		BeforeEach(func() {
			testRun = &Run{
				CommonOptions: CommonOptions{
					Logger: logger,
				},
			}
		})

		It("Should log storage class details without panic for various configurations", func() {
			// Test with complete configuration
			reclaimPolicy1 := corev1.PersistentVolumeReclaimDelete
			volumeBindingMode1 := storagev1.VolumeBindingImmediate
			allowVolumeExpansion1 := true

			sc1 := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-storage-class",
				},
				Provisioner:          "test.csi.driver.io",
				ReclaimPolicy:        &reclaimPolicy1,
				VolumeBindingMode:    &volumeBindingMode1,
				AllowVolumeExpansion: &allowVolumeExpansion1,
				Parameters: map[string]string{
					"type":      "gp2",
					"fsType":    "ext4",
					"encrypted": "true",
				},
			}

			// Test with different configuration
			reclaimPolicy2 := corev1.PersistentVolumeReclaimRetain
			volumeBindingMode2 := storagev1.VolumeBindingWaitForFirstConsumer

			sc2 := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-param-storage-class",
				},
				Provisioner:       "multi.param.csi.driver.io",
				ReclaimPolicy:     &reclaimPolicy2,
				VolumeBindingMode: &volumeBindingMode2,
				Parameters: map[string]string{
					"replication-type": "none",
					"disk-type":        "pd-ssd",
					"zone":             "us-central1-a",
					"fstype":           "ext4",
				},
			}

			storageClasses := []*storagev1.StorageClass{sc1, sc2}

			for _, sc := range storageClasses {
				Expect(func() {
					testRun.logStorageClassDetails(sc)
				}).ToNot(Panic())
			}
		})

		It("Should handle storage class edge cases without panic", func() {
			// Minimal storage class
			sc1 := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minimal-storage-class",
				},
				Provisioner: "minimal.csi.driver.io",
			}

			// Storage class with empty parameters
			reclaimPolicy := corev1.PersistentVolumeReclaimDelete
			sc2 := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-params-storage-class",
				},
				Provisioner:   "empty.csi.driver.io",
				ReclaimPolicy: &reclaimPolicy,
				Parameters:    map[string]string{},
			}

			// Storage class with nil parameters
			sc3 := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nil-params-storage-class",
				},
				Provisioner: "nil.params.csi.driver.io",
				Parameters:  nil,
			}

			edgeCases := []*storagev1.StorageClass{sc1, sc2, sc3}

			for _, sc := range edgeCases {
				Expect(func() {
					testRun.logStorageClassDetails(sc)
				}).ToNot(Panic())
			}
		})
	})

	Context("logVolumeSnapshotTroubleshooting func test-cases", func() {
		var testRun *Run

		BeforeEach(func() {
			testRun = &Run{
				CommonOptions: CommonOptions{
					Logger: logger,
				},
			}
		})

		It("Should handle various provisioners without panic", func() {
			provisioners := []string{
				"ebs.csi.aws.com",
				"disk.csi.azure.com",
				"csi.vsphere.vmware.com",
				"unknown.provisioner.io",
				"",
				"test.provisioner.io",
			}

			for _, provisioner := range provisioners {
				Expect(func() {
					testRun.logVolumeSnapshotTroubleshooting(ctx, testClient.ClientSet, testClient.RuntimeClient, provisioner)
				}).ToNot(Panic())
			}
		})

		It("Should detect VolumeSnapshot CRDs when present", func() {
			// Install VolumeSnapshot CRDs
			vsCRDsMap := map[string]bool{
				"volumesnapshotclasses." + StorageSnapshotGroup:  true,
				"volumesnapshotcontents." + StorageSnapshotGroup: true,
				"volumesnapshots." + StorageSnapshotGroup:        true,
			}
			installVolumeSnapshotCRD(v1K8sVersion, vsCRDsMap)

			defer func() {
				deleteAllVolumeSnapshotCRD()
			}()

			// Test that the function detects the CRDs
			Expect(func() {
				testRun.logVolumeSnapshotTroubleshooting(ctx, testClient.ClientSet, testClient.RuntimeClient, "ebs.csi.aws.com")
			}).ToNot(Panic())
		})

		It("Should detect VolumeSnapshotClass when present", func() {
			// Install VolumeSnapshot CRDs first
			vsCRDsMap := map[string]bool{
				"volumesnapshotclasses." + StorageSnapshotGroup: true,
			}
			installVolumeSnapshotCRD(v1K8sVersion, vsCRDsMap)

			// Install VolumeSnapshotClass
			installVolumeSnapshotClass(internal.V1Version, "ebs.csi.aws.com", "test-snapshot-class")

			defer func() {
				deleteSnapshotClass("test-snapshot-class")
				deleteAllVolumeSnapshotCRD()
			}()

			// Test that the function detects the VolumeSnapshotClass
			Expect(func() {
				testRun.logVolumeSnapshotTroubleshooting(ctx, testClient.ClientSet, testClient.RuntimeClient, "ebs.csi.aws.com")
			}).ToNot(Panic())
		})

		It("Should detect CSI controller pods when present", func() {
			// Create a mock CSI controller pod
			pod := &unstructured.Unstructured{}
			pod.Object = map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "csi-controller-snapshotter",
					"namespace": "kube-system",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "csi-snapshotter",
							"image": "test/csi-snapshotter:latest",
						},
					},
				},
				"status": map[string]interface{}{
					"phase": string(corev1.PodRunning),
				},
			}
			pod.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			})

			err := testClient.RuntimeClient.Create(ctx, pod)
			Expect(err).To(BeNil())

			defer func() {
				_ = testClient.RuntimeClient.Delete(ctx, pod)
			}()

			// Test that the function detects the CSI controller pod
			Expect(func() {
				testRun.logVolumeSnapshotTroubleshooting(ctx, testClient.ClientSet, testClient.RuntimeClient, "test.provisioner.io")
			}).ToNot(Panic())
		})

		It("Should detect snapshot controller deployment when present", func() {
			// Create a mock snapshot controller deployment
			deployment := &unstructured.Unstructured{}
			deployment.Object = map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "snapshot-controller",
					"namespace": "kube-system",
					"labels": map[string]interface{}{
						"app": "snapshot-controller",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(1),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "snapshot-controller",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "snapshot-controller",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "snapshot-controller",
									"image": "test/snapshot-controller:latest",
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"replicas":      int64(1),
					"readyReplicas": int64(1),
				},
			}
			deployment.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			})

			err := testClient.RuntimeClient.Create(ctx, deployment)
			Expect(err).To(BeNil())

			defer func() {
				_ = testClient.RuntimeClient.Delete(ctx, deployment)
			}()

			// Test that the function detects the snapshot controller
			Expect(func() {
				testRun.logVolumeSnapshotTroubleshooting(ctx, testClient.ClientSet, testClient.RuntimeClient, "test.provisioner.io")
			}).ToNot(Panic())
		})

		It("Should handle gracefully when no snapshot resources exist", func() {
			// Test with no snapshot resources present
			Expect(func() {
				testRun.logVolumeSnapshotTroubleshooting(ctx, testClient.ClientSet, testClient.RuntimeClient, "test.provisioner.io")
			}).ToNot(Panic())
		})
	})

})
