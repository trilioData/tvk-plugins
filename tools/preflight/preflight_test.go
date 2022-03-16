package preflight

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
)

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
		It("Should return err when storage class is not present on cluster", func() {
			ops := runOps
			ops.StorageClass = internal.InvalidStorageClassName
			err := ops.checkStorageSnapshotClass(ctx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("not found storageclass - %s on cluster", internal.InvalidStorageClassName)))
		})

		It("Should return error when storage class provisioner has no matching snapshot class driver", func() {
			createStorageClass(testStorageClass, testProvisioner)
			defer deleteStorageClass(testStorageClass)
			ops := runOps
			ops.StorageClass = testStorageClass
			err := ops.checkStorageSnapshotClass(ctx)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when given snapshot class driver does not match with storage class provisioner", func() {
			createStorageClass(testStorageClass, testProvisioner)
			defer deleteStorageClass(testStorageClass)
			createSnapshotClass(testDriver)
			defer deleteSnapshotClass()
			ops := runOps
			ops.StorageClass = testStorageClass
			ops.SnapshotClass = testSnapshotClass
			err := ops.checkStorageSnapshotClass(ctx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"volume snapshot class - %s driver does not match with given StorageClass's provisioner=%s",
				testSnapshotClass, testProvisioner)))
		})

		It("Should pass storage-snapshot check when given storage class provisioner and snapshot class driver match", func() {
			createStorageClass(testStorageClass, testProvisioner)
			defer deleteStorageClass(testStorageClass)
			createSnapshotClass(testProvisioner)
			defer deleteSnapshotClass()
			ops := runOps
			ops.StorageClass = testStorageClass
			ops.SnapshotClass = testSnapshotClass
			err := ops.checkStorageSnapshotClass(ctx)
			Expect(err).To(BeNil())
		})

		It("Should pass storage-snapshot checks when storage class provisioner matches with snapshot class driver on cluster", func() {
			err := runOps.checkStorageSnapshotClass(ctx)
			Expect(err).To(BeNil())
		})
	})

	Context("Check snapshot class against a provisioner", func() {
		It("Should find snapshot class with the given provisioner", func() {
			createSnapshotClass(testDriver)
			defer deleteSnapshotClass()
			ops := runOps
			ops.SnapshotClass = testSnapshotClass
			sscName, err := ops.checkSnapshotclassForProvisioner(ctx, testDriver)
			Expect(err).To(BeNil())
			Expect(sscName).To(Equal(testSnapshotClass))
		})

		It("Should return error when no snapshot class is found against a provisioner", func() {
			createSnapshotClass(testDriver)
			defer deleteSnapshotClass()
			ops := runOps
			ops.SnapshotClass = testSnapshotClass
			_, err := ops.checkSnapshotclassForProvisioner(ctx, testProvisioner)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"no matching volume snapshot class having driver same as provisioner - %s found on cluster", testProvisioner)))
		})
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
			ops.Namespace = internal.InvalidNamespace
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
			ops.Namespace = internal.InvalidNamespace
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
			ops.Namespace = internal.InvalidNamespace
			_, _, err = ops.createSourcePodAndPVC(ctx, resourceSuffix)
			Expect(err).ToNot(BeNil())
		})

		It("Should return error when storage class does not exist on cluster", func() {
			ops := runOps
			ops.StorageClass = internal.InvalidStorageClassName
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
				internal.DefaultTestSnapshotClass, pvc.GetName(), resourceSuffix)
			Expect(err).To(BeNil())
		})

		It("Should return error when incorrect pvc is provided", func() {
			_, err := runOps.createSnapshotFromPVC(ctx, testSnapPrefix+testNameSuffix,
				internal.DefaultTestSnapshotClass, invalidPVC, testNameSuffix)
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
				internal.DefaultTestSnapshotClass, pvc.GetName(), resourceSuffix)
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
