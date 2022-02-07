package preflighttest

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Preflight Tests", func() {

	Context("Preflight run command test-cases", func() {

		Context("Preflight run command storage class flag test cases", func() {

			It("All preflight checks should pass if correct storage class flag value is provided", func() {
				cmdOut, err = runPreflightChecks(flagsMap)
				Expect(err).To(BeNil())

				assertSuccessfulPreflightChecks(flagsMap, cmdOut.Out)
			})

			It("Should fail preflight checks if incorrect storage class flag value is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[storageClassFlag] = invalidStorageClassName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(
					ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
						"not found storageclass - %s on cluster", invalidStorageClassName)))
				Expect(cmdOut.Out).To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
				Expect(cmdOut.Out).To(ContainSubstring("Some preflight checks failed"))
			})

			It("Should not preform preflight checks if storage class flag is not provided", func() {
				cmd := fmt.Sprintf("%s run", preflightBinaryFilePath)
				_, err = shell.RunCmd(cmd)
				Expect(err).ToNot(BeNil())
			})
		})

		Context("Preflight run command snapshot class flag test cases", func() {

			It("Preflight checks should pass if snapshot class is present on cluster and provided as a flag value", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = defaultTestSnapshotClass
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Preflight checks should fail if snapshot class is not present on cluster", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = invalidSnapshotClassName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("volume snapshot class %s not found on cluster :: "+
						"volumesnapshotclasses.snapshot.storage.k8s.io \"%s\" not found",
						invalidSnapshotClassName, invalidSnapshotClassName)))
				Expect(cmdOut.Out).
					To(ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
						"volume snapshot class %s not found", invalidSnapshotClassName)))
				Expect(cmdOut.Out).
					To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
			})

			It("Preflight checks should fail if volume snapshot class does not match with storage class provisioner", func() {
				createVolumeSnapshotClass()
				defer deleteVolumeSnapshotClass()
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = sampleVolSnapClassName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("Preflight check for SnapshotClass failed :: volume snapshot class - %s "+
						"driver does not match with given StorageClass's provisioner=", sampleVolSnapClassName)))
				Expect(cmdOut.Out).To(ContainSubstring(
					"Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
			})
		})

		Context("Preflight run command local registry flag test cases", func() {

			It("Should fail DNS resolution and volume snapshot check if invalid local registry path is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[localRegistryFlag] = invalidLocalRegistryName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(
					MatchRegexp("(DNS pod - dnsutils-)([a-z]{6})( hasn't reached into ready state)"))
				Expect(cmdOut.Out).To(
					ContainSubstring("Preflight check for DNS resolution failed :: timed out waiting for the condition"))
				Expect(cmdOut.Out).To(
					MatchRegexp("(Preflight check for volume snapshot and restore failed :: pod source-pod-)" +
						"([a-z]{6})( hasn't reached into ready state)"))

				nonCRUDPreflightCheckAssertion(inputFlags, cmdOut.Out)
			})
		})

		Context("Preflight run command service account flag test cases", func() {

			It("Should pass all preflight check if service account present on cluster is provided", func() {
				createPreflightServiceAccount()
				defer deletePreflightServiceAccount()
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[serviceAccountFlag] = preflightSAName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Should fail DNS resolution and volume snapshot check if invalid service account is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[serviceAccountFlag] = invalidServiceAccountName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).
					To(MatchRegexp(fmt.Sprintf("(Preflight check for DNS resolution failed :: pods \"dnsutils-)([a-z]{6}\")"+
						"( is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found)",
						defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))

				Expect(cmdOut.Out).To(MatchRegexp(
					fmt.Sprintf("(pods \"source-pod-)([a-z]{6})\" is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found",
						defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))

				Expect(cmdOut.Out).To(MatchRegexp(
					fmt.Sprintf("(Preflight check for volume snapshot and restore failed)(.*)"+
						"(error looking up service account %s/%s: serviceaccount \"%s\" not found)",
						defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))

				nonCRUDPreflightCheckAssertion(inputFlags, cmdOut.Out)
			})
		})

		Context("Preflight run command logging level flag test cases", func() {
			It("Should set default logging level as INFO if incorrect logging level is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[logLevelFlag] = invalidLogLevel
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(
					ContainSubstring("Failed to parse log-level flag. Setting log level as INFO"))
				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})
		})

		Context("Preflight run command cleanup on failure flag test cases", func() {
			It("Should not clean resources when preflight check fails and cleanup on failure flag is set to false", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				delete(inputFlags, cleanupOnFailureFlag)
				inputFlags[localRegistryFlag] = invalidLocalRegistryName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				By("Preflight pods should not be removed from cluster")
				Eventually(func() int {
					podList := unstructured.UnstructuredList{}
					podList.SetGroupVersionKind(podGVK)
					err = runtimeClient.List(ctx, &podList,
						client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(podList.Items)
				}, timeout, interval).ShouldNot(Equal(0))

				By("Preflight PVCs should not be removed from cluster")
				Eventually(func() int {
					pvcList := unstructured.UnstructuredList{}
					pvcList.SetGroupVersionKind(pvcGVK)
					err = runtimeClient.List(ctx, &pvcList,
						client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(pvcList.Items)
				}, timeout, interval).ShouldNot(Equal(0))
			})
		})

		Context("preflight run command, namespace flag test cases", func() {
			It("Should perform preflight checks in default namespace if namespace flag is not provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				delete(inputFlags, namespaceFlag)
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring("Using 'default' namespace of the cluster"))
				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Should perform preflight checks in namespace provided present on cluster", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[namespaceFlag] = defaultTestNs
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("Using '%s' namespace of the cluster", defaultTestNs)))
				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Should fail preflight check if namespace flag is provided with zero value", func() {
				var output []byte
				args := []string{"run", storageClassFlag, defaultTestStorageClass, namespaceFlag, ""}
				cmd := exec.Command("./tvk-preflight", args...)
				output, err = cmd.CombinedOutput()
				Expect(err).To(BeNil())

				Expect(string(output)).To(ContainSubstring("Preflight check for DNS resolution failed :: " +
					"an empty namespace may not be set during creation"))
				Expect(string(output)).To(ContainSubstring("Preflight check for volume snapshot and restore failed :: " +
					"an empty namespace may not be set during creation"))
			})
		})

		Context("Preflight run command, kubeconfig flag test cases", func() {
			It("Should perform preflight checks when a kubeconfig file is specified", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				_, err = os.Create(preflightKubeConf)
				Expect(err).To(BeNil())
				defer func() {
					err = os.Remove(preflightKubeConf)
					Expect(err).To(BeNil())
				}()
				byteData, err = ioutil.ReadFile(kubeConfPath)
				Expect(err).To(BeNil())
				err = ioutil.WriteFile(preflightKubeConf, byteData, filePermission)
				Expect(err).To(BeNil())

				inputFlags[kubeconfigFlag] = path.Join(".", preflightKubeConf)
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("Using kubeconfig file path - %s", path.Join(".", preflightKubeConf))))
				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Should fail preflight execution if invalid kubeconfig file is provided", func() {
				_, err = os.Create(invalidKubeConfFilename)
				Expect(err).To(BeNil())
				err = ioutil.WriteFile(invalidKubeConfFilename, []byte(invalidKubeConfFileData), filePermission)
				Expect(err).To(BeNil())
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[kubeconfigFlag] = invalidStorageClassName
				_, err = runPreflightChecks(inputFlags)
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Context("Preflight cleanup command test-cases", func() {
		Context("Cleanup all preflight resources on the cluster in a particular namespace", func() {

			It("Should clean all preflight resources in a particular namespace", func() {
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				cmdOut, err = runCleanupForAllPreflightResources()
				if err != nil {
					fmt.Println("All cleanup - \ncmdOut: ", cmdOut.Out, "err: ", err)
				}
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring("All preflight resources cleaned"))

				By("All preflight pods should be removed from cluster")
				Eventually(func() int {
					podList := unstructured.UnstructuredList{}
					podList.SetGroupVersionKind(podGVK)
					err = runtimeClient.List(ctx, &podList,
						client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(podList.Items)
				}, timeout, interval).Should(Equal(0))

				By("All preflight PVCs should be removed from cluster")
				Eventually(func() int {
					pvcList := unstructured.UnstructuredList{}
					pvcList.SetGroupVersionKind(pvcGVK)
					err = runtimeClient.List(ctx, &pvcList,
						client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(pvcList.Items)
				}, timeout, interval).Should(Equal(0))

				By("All preflight volume snapshots should be removed from the cluster")
				Eventually(func() int {
					snapshotList := unstructured.UnstructuredList{}
					snapshotList.SetGroupVersionKind(snapshotGVK)
					err = runtimeClient.List(ctx, &snapshotList,
						client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(snapshotList.Items)
				}, timeout, interval).Should(Equal(0))
			})
		})

		Context("Cleanup resources according to preflight UID in a particular namespace", func() {

			It("Should clean pods, PVCs and volume snapshots of preflight check for specific uid", func() {
				var uid = createPreflightResourcesForCleanup()
				cmdOut, err = runCleanupWithUID(uid)
				Expect(err).To(BeNil())

				By(fmt.Sprintf("Should clean source pod with uid=%s", uid))
				srcPodName := strings.Join([]string{preflight.SourcePodNamePrefix, uid}, "")
				Expect(cmdOut.Out).To(ContainSubstring("Cleaning Pod - %s", srcPodName))

				By(fmt.Sprintf("Should clean dns pod with uid=%s", uid))
				dnsPodName := strings.Join([]string{dnsPodNamePrefix, uid}, "")
				Expect(cmdOut.Out).To(ContainSubstring("Cleaning Pod - %s", dnsPodName))

				By(fmt.Sprintf("Should clean source pvc with uid=%s", uid))
				srcPvcName := strings.Join([]string{preflight.SourcePvcNamePrefix, uid}, "")
				Expect(cmdOut.Out).To(ContainSubstring("Cleaning PersistentVolumeClaim - %s", srcPvcName))

				By(fmt.Sprintf("Should clean source volume snapshot with uid=%s", uid))
				srcVolSnapName := strings.Join([]string{preflight.VolumeSnapSrcNamePrefix, uid}, "")
				Expect(cmdOut.Out).To(ContainSubstring("Cleaning VolumeSnapshot - %s", srcVolSnapName))

				By(fmt.Sprintf("Should clean all preflight resources for uid=%s", uid))
				Expect(cmdOut.Out).To(ContainSubstring("All preflight resources cleaned"))

				By(fmt.Sprintf("All preflight pods with uid=%s should be removed from cluster", uid))
				Eventually(func() int {
					podList := unstructured.UnstructuredList{}
					podList.SetGroupVersionKind(podGVK)
					err = runtimeClient.List(ctx, &podList,
						client.MatchingLabels(getPreflightResourceLabels(uid)), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(podList.Items)
				}, timeout, interval).Should(Equal(0))

				By(fmt.Sprintf("All preflight PVCs with uid=%s should be removed from cluster", uid))
				Eventually(func() int {
					pvcList := unstructured.UnstructuredList{}
					pvcList.SetGroupVersionKind(pvcGVK)
					err = runtimeClient.List(ctx, &pvcList,
						client.MatchingLabels(getPreflightResourceLabels(uid)), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(pvcList.Items)
				}, timeout, interval).Should(Equal(0))

				By(fmt.Sprintf("All preflight volume snapshots with uid=%s should be removed from the cluster", uid))
				Eventually(func() int {
					snapshotList := unstructured.UnstructuredList{}
					snapshotList.SetGroupVersionKind(snapshotGVK)
					err = runtimeClient.List(ctx, &snapshotList,
						client.MatchingLabels(getPreflightResourceLabels(uid)), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(snapshotList.Items)
				}, timeout, interval).Should(Equal(0))
			})
		})
	})
})

// Executes the preflight binary in terminal
func runPreflightChecks(flagsMap map[string]string) (*shell.CmdOut, error) {
	var flags string

	for key, val := range flagsMap {
		switch key {
		case storageClassFlag:
			flags = strings.Join([]string{flags, storageClassFlag, val}, " ")

		case namespaceFlag:
			flags = strings.Join([]string{flags, namespaceFlag, val}, " ")

		case snapshotClassFlag:
			flags = strings.Join([]string{flags, snapshotClassFlag, val}, " ")

		case localRegistryFlag:
			flags = strings.Join([]string{flags, localRegistryFlag, val}, " ")

		case imagePullSecFlag:
			flags = strings.Join([]string{flags, imagePullSecFlag, val}, " ")

		case serviceAccountFlag:
			flags = strings.Join([]string{flags, serviceAccountFlag, val}, " ")

		case cleanupOnFailureFlag:
			flags = strings.Join([]string{flags, cleanupOnFailureFlag}, " ")

		case logLevelFlag:
			flags = strings.Join([]string{flags, logLevelFlag, val}, " ")

		case kubeconfigFlag:
			flags = strings.Join([]string{flags, kubeconfigFlag, val}, " ")
		}
	}

	cmd := fmt.Sprintf("%s run %s", preflightBinaryFilePath, flags)
	log.Infof("Preflight check CMD [%s]", cmd)
	return shell.RunCmd(cmd)
}

// Executes cleanup for a particular preflight run
func runCleanupWithUID(uid string) (*shell.CmdOut, error) {
	cmd := fmt.Sprintf("%s cleanup --uid %s", preflightBinaryFilePath, uid)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	return shell.RunCmd(cmd)
}

// Executes cleanup for all preflight resources
func runCleanupForAllPreflightResources() (*shell.CmdOut, error) {
	cmd := fmt.Sprintf("./%s cleanup -n %s", preflightBinaryFilePath, defaultTestNs)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	return shell.RunCmd(cmd)
}

// Individual assertions of successful preflight checks which do not involve any CRUD operations
func nonCRUDPreflightCheckAssertion(inputFlags map[string]string, outputLog string) {
	storageClass, ok := inputFlags[storageClassFlag]
	Expect(ok).To(BeTrue())
	assertPreflightLogFileCreateSuccess(outputLog)
	assertVolSnapClassCheckSuccess(inputFlags, outputLog)
	assertKubectlBinaryCheckSuccess(outputLog)
	assertK8sClusterRBACCheckSuccess(outputLog)
	assertHelmVersionCheckSuccess(outputLog)
	assertK8sServerVersionCheckSuccess(outputLog)
	assertCsiAPICheckSuccess(outputLog)
	assertClusterAccessCheckSuccess(outputLog)
	assertStorageClassCheckSuccess(storageClass, outputLog)
}

func assertPreflightLogFileCreateSuccess(outputLog string) {
	By("Preflight log file should be created")
	Expect(outputLog).To(ContainSubstring("Created log file with name - preflight-"))
}

func assertSuccessfulPreflightChecks(inputFlags map[string]string, outputLog string) {
	nonCRUDPreflightCheckAssertion(inputFlags, outputLog)
	assertDNSResolutionCheckSuccess(outputLog)
	assertVolumeSnapshotCheckSuccess(outputLog)
}

func assertVolSnapClassCheckSuccess(inputFlags map[string]string, outputLog string) {
	By("Check whether volume snapshot class is present on cluster")
	snapshotClass, ok := inputFlags[snapshotClassFlag]
	if ok {
		Expect(outputLog).To(ContainSubstring(
			fmt.Sprintf("Volume snapshot class - %s driver matches with given storage class provisioner",
				snapshotClass)))
	} else {
		Expect(outputLog).
			To(MatchRegexp("(Extracted volume snapshot class -)(.*)(found in cluster)"))
		Expect(outputLog).
			To(MatchRegexp("(Volume snapshot class -)(.*)(driver matches with given StorageClass's provisioner)"))
	}
}

func assertKubectlBinaryCheckSuccess(outputLog string) {
	By("Find kubectl on client machine")
	Expect(outputLog).To(ContainSubstring("kubectl found at path - "))
	Expect(outputLog).To(ContainSubstring("Preflight check for kubectl utility is successful"))
}

func assertClusterAccessCheckSuccess(outputLog string) {
	By("Check access to cluster")
	Expect(outputLog).To(ContainSubstring("Preflight check for kubectl access is successful"))
}

func assertHelmVersionCheckSuccess(outputLog string) {
	By("Check whether helm is installed or it is an OCP cluster")
	if discClient != nil && internal.CheckIsOpenshift(discClient, internal.OcpAPIVersion) {
		Expect(outputLog).To(ContainSubstring("Running OCP cluster. Helm not needed for OCP clusters"))
	} else {
		Expect(outputLog).To(ContainSubstring("helm found at path - "))
		var helmVersion string
		helmVersion, err = preflight.GetHelmVersion()
		Expect(err).To(BeNil())
		Expect(outputLog).
			To(ContainSubstring(fmt.Sprintf("Helm version %s meets required version", helmVersion)))
	}
}

func assertK8sServerVersionCheckSuccess(outputLog string) {
	By("Check whether K8s server version is satisfied")
	Expect(outputLog).To(ContainSubstring("Preflight check for kubernetes version is successful"))
}

func assertK8sClusterRBACCheckSuccess(outputLog string) {
	By("Check whether RBAC is enabled on cluster")
	Expect(outputLog).To(ContainSubstring("Kubernetes RBAC is enabled"))
	Expect(outputLog).To(ContainSubstring("Preflight check for kubernetes RBAC is successful"))
}

func assertStorageClassCheckSuccess(storageClass, outputLog string) {
	By("Check whether storage class is present on cluster")
	Expect(outputLog).
		To(ContainSubstring(fmt.Sprintf("Storageclass - %s found on cluster", storageClass)))
	Expect(outputLog).To(ContainSubstring("Preflight check for SnapshotClass is successful"))
}

func assertCsiAPICheckSuccess(outputLog string) {
	By("Check whether CSI APIs are installed on the cluster")
	for _, api := range preflight.CsiApis {
		Expect(outputLog).
			To(ContainSubstring(fmt.Sprintf("Found CSI API - %s on cluster", api)))
	}
	Expect(outputLog).To(ContainSubstring("Preflight check for CSI is successful"))
}

func assertDNSResolutionCheckSuccess(outputLog string) {
	By("Check whether DNS can resolved on cluster")
	Expect(outputLog).To(MatchRegexp("(Pod dnsutils-)([a-z]{6})( created in cluster)"))
	Expect(outputLog).To(ContainSubstring("Preflight check for DNS resolution is successful"))
}

func assertVolumeSnapshotCheckSuccess(outputLog string) {
	By("Check whether volume snapshot and restore is possible on cluster")

	Expect(outputLog).
		To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
			"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; then exit 0; else exit 1; fi' " +
			"in container - 'busybox' of pod - 'restored-pod"))

	Expect(outputLog).To(MatchRegexp("(Restored pod - restored-pod-)([a-z]{6})( has expected data)"))

	Expect(outputLog).
		To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
			"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; " +
			"then exit 0; else exit 1; fi' in container - 'busybox' of pod - 'unmounted-restored-pod"))

	Expect(outputLog).To(ContainSubstring("restored pod from volume snapshot of unmounted pv has expected data"))
	Expect(outputLog).To(ContainSubstring("Preflight check for volume snapshot and restore is successful"))
}

func createPreflightServiceAccount() {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      preflightSAName,
			Namespace: defaultTestNs,
		},
	}
	err = runtimeClient.Create(ctx, sa)
	Expect(err).To(BeNil())
}

func deletePreflightServiceAccount() {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      preflightSAName,
			Namespace: defaultTestNs,
		},
	}
	err = runtimeClient.Delete(ctx, sa)
	Expect(err).To(BeNil())
}

func createPreflightResourcesForCleanup() string {
	var uid string
	uid, err = preflight.CreateResourceNameSuffix()
	Expect(err).To(BeNil())
	createPreflightPVC(uid)
	srcPvcName := strings.Join([]string{preflight.SourcePvcNamePrefix, uid}, "")
	createPreflightVolumeSnapshot(srcPvcName, uid)
	createPreflightPods(srcPvcName, uid)

	return uid
}

func createPreflightPods(pvcName, preflightUID string) {
	createDNSPod(preflightUID)
	createSourcePod(pvcName, preflightUID)
}

func createDNSPod(preflightUID string) {
	dnsPod := createDNSPodSpec(preflightUID)
	err = runtimeClient.Create(ctx, dnsPod)
	Expect(err).To(BeNil())
}

func createDNSPodSpec(preflightUID string) *corev1.Pod {
	pod := getPodTemplate(dnsPodNamePrefix, preflightUID)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            dnsContainerName,
			Image:           strings.Join([]string{preflight.GcrRegistryPath, preflight.DNSUtilsImage}, "/"),
			Command:         preflight.CommandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       preflight.ResourceRequirements,
		},
	}

	return pod
}

func createSourcePod(pvcName, preflightUID string) {
	srcPod := createSourcePodSpec(pvcName, preflightUID)
	err = runtimeClient.Create(ctx, srcPod)
	Expect(err).To(BeNil())
}

func createSourcePodSpec(pvcName, preflightUID string) *corev1.Pod {
	pod := getPodTemplate(preflight.SourcePodNamePrefix, preflightUID)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:      preflight.BusyboxContainerName,
			Image:     preflight.BusyboxImageName,
			Command:   preflight.CommandBinSh,
			Args:      preflight.ArgsTouchDataFileSleep,
			Resources: preflight.ResourceRequirements,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      preflight.VolMountName,
					MountPath: preflight.VolMountPath,
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: preflight.VolMountName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
					ReadOnly:  false,
				},
			},
		},
	}

	return pod
}

func createPreflightPVC(preflightUID string) {
	pvc := createPreflightPVCSpec(preflightUID)
	err = runtimeClient.Create(ctx, pvc)
	Expect(err).To(BeNil())
}

func createPreflightPVCSpec(preflightUID string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{preflight.SourcePvcNamePrefix, preflightUID}, ""),
			Namespace: defaultTestNs,
			Labels:    getPreflightResourceLabels(preflightUID),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: func() *string { var storageClass = defaultTestStorageClass; return &storageClass }(),
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}

func createPreflightVolumeSnapshot(pvcName, preflightUID string) {
	volSnap := createPreflightVolumeSnapshotSpec(pvcName, preflightUID)
	err = runtimeClient.Create(ctx, volSnap)
	Expect(err).To(BeNil())
}

func createPreflightVolumeSnapshotSpec(pvcName, preflightUID string) *unstructured.Unstructured {
	var snapshotVersion string
	snapshotVersion, err = preflight.GetServerPreferredVersionForGroup(preflight.StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	volSnap := &unstructured.Unstructured{}
	volSnap.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": defaultTestSnapshotClass,
			"source": map[string]string{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}
	volSnap.SetName(strings.Join([]string{preflight.VolumeSnapSrcNamePrefix, preflightUID}, ""))
	volSnap.SetNamespace(defaultTestNs)
	volSnap.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   preflight.StorageSnapshotGroup,
		Version: snapshotVersion,
		Kind:    internal.VolumeSnapshotKind,
	})
	volSnap.SetLabels(getPreflightResourceLabels(preflightUID))

	return volSnap
}

func createVolumeSnapshotClass() {
	vsc := &unstructured.Unstructured{}
	vsc.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": sampleVolSnapClassName,
		},
		"driver":         invalidVolSnapDriver,
		"deletionPolicy": "Delete",
	}
	vsc.SetGroupVersionKind(snapshotClassGVK)

	err = runtimeClient.Create(ctx, vsc)
	Expect(err).To(BeNil())
}

func deleteVolumeSnapshotClass() {
	vsc := &unstructured.Unstructured{}
	vsc.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": sampleVolSnapClassName,
		},
	}
	vsc.SetGroupVersionKind(snapshotClassGVK)

	err = runtimeClient.Delete(ctx, vsc)
	Expect(err).To(BeNil())
}

// A basic pod template for any pod to be created for testing purpose
func getPodTemplate(name, preflightUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, preflightUID}, ""),
			Namespace: defaultTestNs,
			Labels:    getPreflightResourceLabels(preflightUID),
		},
	}
}

// Labels of any preflight resource will have the below labels
func getPreflightResourceLabels(uid string) map[string]string {
	labels := map[string]string{
		preflight.LabelK8sName:   preflight.LabelK8sNameValue,
		preflight.LabelTrilioKey: preflight.LabelTvkPreflightValue,
		preflight.LabelK8sPartOf: preflight.LabelK8sPartOfValue,
	}
	if uid != "" {
		labels[preflight.LabelPreflightRunKey] = uid
	}

	return labels
}

// copy map 'from' to 'to' key-by-key and value-by-value
func copyMap(from, to map[string]string) {
	for key, value := range from {
		to[key] = value
	}
}
