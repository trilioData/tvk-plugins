package preflighttest

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"

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
				var byteData []byte
				runPreflightChecks(flagsMap)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				assertSuccessfulPreflightChecks(flagsMap, outputLogData)
			})

			It("Should fail preflight checks if incorrect storage class flag value is provided", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[storageClassFlag] = invalidStorageClassName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).To(
					ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
						"not found storageclass - %s on cluster", invalidStorageClassName)))
				Expect(outputLogData).To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
				Expect(outputLogData).To(ContainSubstring("Some preflight checks failed"))
			})

			It("Should not preform preflight checks if storage class flag is not provided", func() {
				cmd := fmt.Sprintf("%s run", preflightBinaryFilePath)
				_, err = shell.RunCmd(cmd)
				Expect(err).To(Not(BeNil()))
			})
		})

		Context("Preflight run command snapshot class flag test cases", func() {

			It("Preflight checks should pass even if snapshot class flag value is not provided", func() {
				var byteData []byte
				runPreflightChecks(flagsMap)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				assertSuccessfulPreflightChecks(flagsMap, outputLogData)
				Expect(outputLogData).
					To(MatchRegexp("(Extracted volume snapshot class -)(.*)(found in cluster)"))
				Expect(outputLogData).
					To(MatchRegexp("(Volume snapshot class -)(.*)(driver matches with given StorageClass's provisioner)"))
			})

			It("Preflight checks should pass if snapshot class is present on cluster and provided as a flag value", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = defaultTestSnapshotClass
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				assertSuccessfulPreflightChecks(inputFlags, outputLogData)
				Expect(outputLogData).To(ContainSubstring(
					fmt.Sprintf("Volume snapshot class - %s driver matches with given storage class provisioner",
						defaultTestSnapshotClass)))
			})

			It("Preflight checks should fail if snapshot class is not present on cluster", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = invalidSnapshotClassName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).To(ContainSubstring(
					fmt.Sprintf("volume snapshot class %s not found on cluster :: "+
						"volumesnapshotclasses.snapshot.storage.k8s.io \"%s\" not found",
						invalidSnapshotClassName, invalidSnapshotClassName)))
				Expect(outputLogData).
					To(ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
						"volume snapshot class %s not found", invalidSnapshotClassName)))
				Expect(outputLogData).
					To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
			})
		})

		Context("Preflight run command local registry flag test cases", func() {

			It("Should fail DNS resolution and volume snapshot check if invalid local registry path is provided", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[localRegistryFlag] = invalidLocalRegistryName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).To(
					MatchRegexp("(DNS pod - dnsutils-)([a-z]{6})( hasn't reached into ready state)"))
				Expect(outputLogData).To(
					ContainSubstring("Preflight check for DNS resolution failed :: timed out waiting for the condition"))
				Expect(outputLogData).To(
					MatchRegexp("(Preflight check for volume snapshot and restore failed :: pod source-pod-)" +
						"([a-z]{6})( hasn't reached into ready state)"))

				nonCRUDPreflightCheckAssertion(inputFlags, outputLogData)
			})
		})

		Context("Preflight run command service account flag test cases", func() {

			It("Should pass all preflight check if service account present on cluster is provided", func() {
				var byteData []byte
				createPreflightServiceAccount()
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[serviceAccountFlag] = preflightSAName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)
				deletePreflightServiceAccount()

				assertSuccessfulPreflightChecks(inputFlags, outputLogData)
			})

			It("Should fail DNS resolution and volume snapshot check if invalid service account is provided", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[serviceAccountFlag] = invalidServiceAccountName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).
					To(MatchRegexp(fmt.Sprintf("(Preflight check for DNS resolution failed :: pods \"dnsutils-)([a-z]{6}\")"+
						"( is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found)",
						defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))

				Expect(outputLogData).To(MatchRegexp(
					fmt.Sprintf("(pods \"source-pod-)([a-z]{6})\" is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found",
						defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))

				Expect(outputLogData).To(MatchRegexp(
					fmt.Sprintf("(Preflight check for volume snapshot and restore failed)(.*)"+
						"(error looking up service account %s/%s: serviceaccount \"%s\" not found)",
						defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))

				nonCRUDPreflightCheckAssertion(inputFlags, outputLogData)
			})
		})

		Context("Preflight run command logging level flag test cases", func() {
			It("Should set default logging level as INFO if incorrect logging level is provided", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[logLevelFlag] = invalidLogLevel
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).To(
					ContainSubstring("Failed to parse log-level flag. Setting log level as INFO"))
				assertSuccessfulPreflightChecks(inputFlags, outputLogData)
			})
		})

		Context("preflight run command, namespace flag test cases", func() {
			It("Should perform preflight checks in default namespace if namespace flag is not provided", func() {
				var byteData []byte
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				delete(inputFlags, namespaceFlag)
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).To(ContainSubstring("Using 'default' namespace of the cluster"))
			})
		})

		Context("Preflight run command, kubeconfig flag test cases", func() {
			It("Should perform preflight checks when a kubeconfig file is specified", func() {
				var (
					byteData         []byte
					testKubeConfFile *os.File
					kubeConfFile     *os.File
				)
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				testKubeConfFile, err = os.OpenFile(preflightKubeConf, os.O_CREATE|os.O_WRONLY, filePermission)
				Expect(err).To(BeNil())
				defer testKubeConfFile.Close()
				kubeConfFile, err = os.OpenFile(kubeConfPath, os.O_RDONLY, filePermission)
				Expect(err).To(BeNil())
				defer kubeConfFile.Close()
				copyFile(kubeConfFile, testKubeConfFile)
				inputFlags[kubeconfigFlag] = path.Join(".", testKubeConfFile.Name())
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData := string(byteData)

				Expect(outputLogData).To(ContainSubstring(
					fmt.Sprintf("Using kubeconfig file path - %s", path.Join(".",
						testKubeConfFile.Name()))))

				err = os.Remove(testKubeConfFile.Name())
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Preflight cleanup command test-cases", func() {
		Context("Cleanup all preflight resources on the cluster in a particular namespace", func() {

			It("Should clean all preflight resources in a particular namespace", func() {
				var (
					outputLogData string
					byteData      []byte
				)
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				runCleanupForAllPreflightResources()
				byteData, err = getLogFileData(cleanupLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)

				Expect(outputLogData).To(ContainSubstring("All preflight resources cleaned"))

				By("All preflight pods should be removed from cluster")
				Eventually(func() int {
					podList := unstructured.UnstructuredList{}
					podList.SetGroupVersionKind(podGVK)
					err = runtimeClient.List(ctx, &podList, client.MatchingLabels(preflightResourceLabelMap("")))
					Expect(err).To(BeNil())
					return len(podList.Items)
				}, timeout, interval).Should(Equal(0))

				By("All preflight PVCs should be removed from cluster")
				Eventually(func() int {
					pvcList := unstructured.UnstructuredList{}
					pvcList.SetGroupVersionKind(pvcGVK)
					err = runtimeClient.List(ctx, &pvcList, client.MatchingLabels(preflightResourceLabelMap("")))
					Expect(err).To(BeNil())
					return len(pvcList.Items)
				}, timeout, interval).Should(Equal(0))

				By("All preflight volume snapshots should be removed from the cluster")
				Eventually(func() int {
					snapshotList := unstructured.UnstructuredList{}
					snapshotList.SetGroupVersionKind(snapshotGVK)
					err = runtimeClient.List(ctx, &snapshotList, client.MatchingLabels(preflightResourceLabelMap("")))
					Expect(err).To(BeNil())
					return len(snapshotList.Items)
				}, timeout, interval).Should(Equal(0))
			})
		})

		Context("Cleanup resources according to preflight UID in a particular namespace", func() {

			It("Should clean pods, PVCs and volume snapshots of preflight check for specific uid", func() {
				var (
					outputLogData string
					uid           string
					byteData      []byte
				)
				uid = createPreflightResourcesForCleanup()
				runCleanupWithUID(uid)
				byteData, err = getLogFileData(cleanupLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)

				By(fmt.Sprintf("Should clean source pod with uid=%s", uid))
				srcPodName := strings.Join([]string{sourcePodNamePrefix, uid}, "")
				Expect(outputLogData).To(ContainSubstring("Cleaning Pod - %s", srcPodName))

				By(fmt.Sprintf("Should clean dns pod with uid=%s", uid))
				dnsPodName := strings.Join([]string{dnsPodNamePrefix, uid}, "")
				Expect(outputLogData).To(ContainSubstring("Cleaning Pod - %s", dnsPodName))

				By(fmt.Sprintf("Should clean source pvc with uid=%s", uid))
				srcPvcName := strings.Join([]string{sourcePVCNamePrefix, uid}, "")
				Expect(outputLogData).To(ContainSubstring("Cleaning PersistentVolumeClaim - %s", srcPvcName))

				By(fmt.Sprintf("Should clean source volume snapshot with uid=%s", uid))
				srcVolSnapName := strings.Join([]string{volSnapshotNamePrefix, uid}, "")
				Expect(outputLogData).To(ContainSubstring("Cleaning VolumeSnapshot - %s", srcVolSnapName))

				By(fmt.Sprintf("Should clean all preflight resources for uid=%s", uid))
				Expect(outputLogData).To(ContainSubstring("All preflight resources cleaned"))

				By(fmt.Sprintf("All preflight pods with uid=%s should be removed from cluster", uid))
				Eventually(func() int {
					podList := unstructured.UnstructuredList{}
					podList.SetGroupVersionKind(podGVK)
					err = runtimeClient.List(ctx, &podList, client.MatchingLabels(preflightResourceLabelMap(uid)))
					Expect(err).To(BeNil())
					return len(podList.Items)
				}, timeout, interval).Should(Equal(0))

				By(fmt.Sprintf("All preflight PVCs with uid=%s should be removed from cluster", uid))
				Eventually(func() int {
					pvcList := unstructured.UnstructuredList{}
					pvcList.SetGroupVersionKind(pvcGVK)
					err = runtimeClient.List(ctx, &pvcList, client.MatchingLabels(preflightResourceLabelMap(uid)))
					Expect(err).To(BeNil())
					return len(pvcList.Items)
				}, timeout, interval).Should(Equal(0))

				By(fmt.Sprintf("All preflight volume snapshots with uid=%s should be removed from the cluster", uid))
				Eventually(func() int {
					snapshotList := unstructured.UnstructuredList{}
					snapshotList.SetGroupVersionKind(snapshotGVK)
					err = runtimeClient.List(ctx, &snapshotList, client.MatchingLabels(preflightResourceLabelMap(uid)))
					Expect(err).To(BeNil())
					return len(snapshotList.Items)
				}, timeout, interval).Should(Equal(0))
			})
		})
	})
})

func runPreflightChecks(flagsMap map[string]string) {
	cleanDirForFiles(preflightLogFilePrefix)
	Expect(err).To(BeNil())
	var flags string

	for key, val := range flagsMap {
		switch key {
		case storageClassFlag:
			flags += fmt.Sprintf("%s %s ", storageClassFlag, val)

		case namespaceFlag:
			flags += fmt.Sprintf("%s %s ", namespaceFlag, val)

		case snapshotClassFlag:
			flags += fmt.Sprintf("%s %s ", snapshotClassFlag, val)

		case localRegistryFlag:
			flags += fmt.Sprintf("%s %s ", localRegistryFlag, val)

		case imagePullSecFlag:
			flags += fmt.Sprintf("%s %s ", imagePullSecFlag, val)

		case serviceAccountFlag:
			flags += fmt.Sprintf("%s %s ", serviceAccountFlag, val)

		case cleanupOnFailureFlag:
			flags += fmt.Sprintf("%s ", cleanupOnFailureFlag)

		case logLevelFlag:
			flags += fmt.Sprintf("%s %s ", logLevelFlag, val)

		case kubeconfigFlag:
			flags += fmt.Sprintf("%s %s ", kubeconfigFlag, val)
		}
	}

	cmd := fmt.Sprintf("%s run %s", preflightBinaryFilePath, flags)
	log.Infof("Preflight check CMD [%s]", cmd)
	_, err = shell.RunCmd(cmd)
	Expect(err).To(BeNil())
}

func runCleanupWithUID(uid string) {
	cleanDirForFiles(cleanupLogFilePrefix)
	Expect(err).To(BeNil())
	cmd := fmt.Sprintf("%s cleanup --uid %s", preflightBinaryFilePath, uid)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	_, err = shell.RunCmd(cmd)
	Expect(err).To(BeNil())
}

func runCleanupForAllPreflightResources() {
	cleanDirForFiles(cleanupLogFilePrefix)
	Expect(err).To(BeNil())
	cmd := fmt.Sprintf("%s cleanup -n %s", preflightBinaryFilePath, defaultTestNs)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	_, err = shell.RunCmd(cmd)
	Expect(err).To(BeNil())
}

func cleanDirForFiles(filePrefix string) {
	var names []fs.FileInfo
	names, err = ioutil.ReadDir(preflightBinaryDir)
	Expect(err).To(BeNil())
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			err = os.RemoveAll(path.Join([]string{preflightBinaryDir, entry.Name()}...))
			Expect(err).To(BeNil())
		}
	}
}

func getLogFileData(filePrefix string) ([]byte, error) {
	var (
		foundFile   = false
		logFilename string
		names       []fs.FileInfo
	)
	names, err = ioutil.ReadDir(preflightBinaryDir)
	Expect(err).To(BeNil())
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			logFilename = entry.Name()
			foundFile = true
			break
		}
	}

	if !foundFile {
		return nil, fmt.Errorf("preflight log file not found")
	}
	return ioutil.ReadFile(logFilename)
}

func nonCRUDPreflightCheckAssertion(inputFlags map[string]string, outputLog string) {
	storageClass, ok := inputFlags[storageClassFlag]
	Expect(ok).To(BeTrue())
	assertKubectlBinaryCheckSuccess(outputLog)
	assertK8sClusterRBACCheckSuccess(outputLog)
	assertHelmVersionCheckSuccess(outputLog)
	assertK8sServerVersionCheckSuccess(outputLog)
	assertCsiAPICheckSuccess(outputLog)
	assertClusterAccessCheckSuccess(outputLog)
	assertStorageClassCheckSuccess(storageClass, outputLog)
}

func assertSuccessfulPreflightChecks(inputFlags map[string]string, outputLog string) {
	nonCRUDPreflightCheckAssertion(inputFlags, outputLog)
	assertDNSResolutionCheckSuccess(outputLog)
	assertVolumeSnapshotCheckSuccess(outputLog)
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
	if discClient != nil && internal.CheckIsOpenshift(discClient, ocpAPIVersion) {
		Expect(outputLog).To(ContainSubstring("Running OCP cluster. Helm not needed for OCP clusters"))
	} else {
		Expect(outputLog).To(ContainSubstring("helm found at path - "))
		var helmVersion = getHelmVersion()
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
	for _, api := range csiApis {
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
	Expect(outputLog).To(MatchRegexp("(Created source pvc - source-pvc-)([a-z]{6})"))
	Expect(outputLog).To(MatchRegexp("(Created source pod - source-pod-)([a-z]{6})"))
	Expect(outputLog).To(MatchRegexp("(Created volume snapshot - snapshot-source-pvc-)([a-z]{6})( from source pvc)"))
	Expect(outputLog).To(MatchRegexp("(Created restore pvc - restored-pvc-)([a-z]{6})" +
		"( from volume snapshot - snapshot-source-pvc-)([a-z]{6})"))

	Expect(outputLog).To(MatchRegexp("(Created restore pod - restored-pod-)([a-z]{6})"))
	Expect(outputLog).
		To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
			"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; then exit 0; else exit 1; fi' " +
			"in container - 'busybox' of pod - 'restored-pod"))

	Expect(outputLog).To(MatchRegexp("(Restored pod - restored-pod-)([a-z]{6})( has expected data)"))
	Expect(outputLog).To(MatchRegexp("(Created volume snapshot - unmounted-source-pvc-)([a-z]{6})"))
	Expect(outputLog).To(MatchRegexp("(Created restore pod - unmounted-restored-pod-)([a-z]{6})( from volume snapshot of unmounted pv)"))

	Expect(outputLog).
		To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
			"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; " +
			"then exit 0; else exit 1; fi' in container - 'busybox' of pod - 'unmounted-restored-pod"))

	Expect(outputLog).To(ContainSubstring("restored pod from volume snapshot of unmounted pv has expected data"))
}

func getHelmVersion() string {
	var cmdOut *shell.CmdOut
	cmdOut, err = shell.RunCmd("helm version --template '{{.Version}}'")
	Expect(err).To(BeNil())
	helmVersion := cmdOut.Out[2 : len(cmdOut.Out)-1]
	return helmVersion
}

func generatePreflightUID() string {
	var randNum *big.Int
	uid := make([]byte, 6)
	randRange := big.NewInt(int64(len(letterBytes)))
	for i := range uid {
		randNum, err = rand.Int(rand.Reader, randRange)
		Expect(err).To(BeNil())
		idx := randNum.Int64()
		uid[i] = letterBytes[idx]
	}

	return string(uid)
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
	var uid = generatePreflightUID()
	Expect(err).To(BeNil())
	createPreflightPVC(uid)
	srcPvcName := strings.Join([]string{sourcePVCNamePrefix, uid}, "")
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
			Image:           strings.Join([]string{gcrRegistryPath, dnsUtilsImage}, "/"),
			Command:         commandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       resourceRequirements,
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
	pod := getPodTemplate(sourcePodNamePrefix, preflightUID)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:      busyboxContainerName,
			Image:     busyboxImageName,
			Command:   commandBinSh,
			Args:      argsTouchDataFileSleep,
			Resources: resourceRequirements,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volMountName,
					MountPath: volMountPath,
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volMountName,
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
			Name:      strings.Join([]string{sourcePVCNamePrefix, preflightUID}, ""),
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
	snapshotVersion, err = getServerPreferredVersionForGroup(storageSnapshotGroup)
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
	volSnap.SetName(strings.Join([]string{volSnapshotNamePrefix, preflightUID}, ""))
	volSnap.SetNamespace(defaultTestNs)
	volSnap.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: snapshotVersion,
		Kind:    internal.VolumeSnapshotKind,
	})
	volSnap.SetLabels(getPreflightResourceLabels(preflightUID))

	return volSnap
}

func getPodTemplate(name, preflightUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, preflightUID}, ""),
			Namespace: defaultTestNs,
			Labels:    getPreflightResourceLabels(preflightUID),
		},
	}
}

func getPreflightResourceLabels(preflightUID string) map[string]string {
	return map[string]string{
		labelK8sName:         labelK8sNameValue,
		labelTrilioKey:       labelTvkPreflightValue,
		labelPreflightRunKey: preflightUID,
		labelK8sPartOf:       labelK8sPartOfValue,
	}
}

func preflightResourceLabelMap(uid string) map[string]string {
	labels := map[string]string{
		labelK8sName:   labelK8sNameValue,
		labelTrilioKey: labelTvkPreflightValue,
		labelK8sPartOf: labelK8sPartOfValue,
	}
	if uid != "" {
		labels[labelPreflightRunKey] = uid
	}

	return labels
}

func copyFile(src, dest *os.File) {
	_, err = io.Copy(dest, src)
	Expect(err).To(BeNil())
}

func copyMap(from, to map[string]string) {
	for key, value := range from {
		to[key] = value
	}
}
