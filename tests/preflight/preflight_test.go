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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight"
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
				Consistently(func() int {
					podList := unstructured.UnstructuredList{}
					podList.SetGroupVersionKind(podGVK)
					err = runtimeClient.List(ctx, &podList,
						client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
					Expect(err).To(BeNil())
					return len(podList.Items)
				}, timeout, interval).ShouldNot(Equal(0))

				By("Preflight PVCs should not be removed from cluster")
				Consistently(func() int {
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

			It("Should fail DNS and volume snapshot check if given namespace is not present on cluster", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[namespaceFlag] = invalidNamespace
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				nonCRUDPreflightCheckAssertion(inputFlags, cmdOut.Out)
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					" Preflight check for DNS resolution failed :: namespaces \"%s\" not found", inputFlags[namespaceFlag])))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					"Preflight check for volume snapshot and restore failed :: namespaces \"%s\" not found", inputFlags[namespaceFlag])))
			})

			It("Should fail preflight check if namespace flag is provided with zero value", func() {
				var output []byte
				args := []string{"run", storageClassFlag, defaultTestStorageClass, namespaceFlag, "", cleanupOnFailureFlag}
				cmd := exec.Command(preflightBinaryFilePath, args...)
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
				inputFlags[kubeconfigFlag] = invalidKubeConfFilename
				_, err = runPreflightChecks(inputFlags)
				Expect(err).Should(HaveOccurred())
			})

			It(fmt.Sprintf("Should perform preflight check using kubeconfig file having path of %s "+
				"if kubeconfig flag is provided with zero value", kubeConfPath), func() {
				var output []byte
				args := []string{"run", storageClassFlag, flagsMap[storageClassFlag],
					namespaceFlag, flagsMap[namespaceFlag], kubeconfigFlag, "", cleanupOnFailureFlag}
				cmd := exec.Command(preflightBinaryFilePath, args...)
				output, err = cmd.CombinedOutput()
				Expect(err).To(BeNil())

				assertSuccessfulPreflightChecks(flagsMap, string(output))
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
func runPreflightChecks(flagsMap map[string]string) (cmdOut *shell.CmdOut, err error) {
	var flags string

	for key, val := range flagsMap {
		switch key {
		case storageClassFlag:
			flags = strings.Join([]string{flags, storageClassFlag, val}, spaceSeparator)

		case namespaceFlag:
			flags = strings.Join([]string{flags, namespaceFlag, val}, spaceSeparator)

		case snapshotClassFlag:
			flags = strings.Join([]string{flags, snapshotClassFlag, val}, spaceSeparator)

		case localRegistryFlag:
			flags = strings.Join([]string{flags, localRegistryFlag, val}, spaceSeparator)

		case imagePullSecFlag:
			flags = strings.Join([]string{flags, imagePullSecFlag, val}, spaceSeparator)

		case serviceAccountFlag:
			flags = strings.Join([]string{flags, serviceAccountFlag, val}, spaceSeparator)

		case cleanupOnFailureFlag:
			flags = strings.Join([]string{flags, cleanupOnFailureFlag}, spaceSeparator)

		case logLevelFlag:
			flags = strings.Join([]string{flags, logLevelFlag, val}, spaceSeparator)

		case kubeconfigFlag:
			flags = strings.Join([]string{flags, kubeconfigFlag, val}, spaceSeparator)
		}
	}

	cmd := fmt.Sprintf("%s run %s", preflightBinaryFilePath, flags)
	log.Infof("Preflight check CMD [%s]", cmd)
	cmdOut, err = shell.RunCmd(cmd)
	log.Infof("Preflight binary run execution output: %s", cmdOut.Out)
	return cmdOut, err
}

// Executes cleanup for a particular preflight run
func runCleanupWithUID(uid string) (cmdOut *shell.CmdOut, err error) {
	cmd := fmt.Sprintf("%s cleanup --uid %s -n %s -k %s",
		preflightBinaryFilePath, uid, defaultTestNs, kubeConfPath)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	cmdOut, err = shell.RunCmd(cmd)
	log.Infof("Preflight binary cleanup execution output: %s", cmdOut.Out)
	return cmdOut, err
}

// Executes cleanup for all preflight resources
func runCleanupForAllPreflightResources() (cmdOut *shell.CmdOut, err error) {
	cmd := fmt.Sprintf("%s cleanup -n %s -k %s", preflightBinaryFilePath, defaultTestNs, kubeConfPath)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	cmdOut, err = shell.RunCmd(cmd)
	log.Infof("Preflight binary cleanup all execution output: %s", cmdOut.Out)
	return cmdOut, err
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
