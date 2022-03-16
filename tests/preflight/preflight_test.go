package preflighttest

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tLog "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/cmd/preflight/cmd"
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
				Expect(err).ToNot(BeNil())

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

		Context("Preflight run command local registry flag test cases", func() {

			It("Should fail DNS resolution and volume snapshot check if invalid local registry path is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[localRegistryFlag] = invalidLocalRegistryName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(
					MatchRegexp("(DNS pod - dnsutils-)([a-z]{6})( hasn't reached into ready state)"))
				Expect(cmdOut.Out).To(
					ContainSubstring("Preflight check for DNS resolution failed :: timed out waiting for the condition"))
				Expect(cmdOut.Out).To(
					MatchRegexp("(Preflight check for volume snapshot and restore failed :: pod source-pod-)" +
						"([a-z]{6})( hasn't reached into ready state)"))

				nonCRUDPreflightCheckAssertion(inputFlags[storageClassFlag], "", cmdOut.Out)
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
				Expect(err).ToNot(BeNil())

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

				nonCRUDPreflightCheckAssertion(inputFlags[storageClassFlag], "", cmdOut.Out)
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
				Expect(err).ToNot(BeNil())

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

				Expect(cmdOut.Out).To(ContainSubstring("NAMESPACE=\"default\""))
				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Should fail DNS and volume snapshot check if given namespace is not present on cluster", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[namespaceFlag] = invalidNamespace
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				nonCRUDPreflightCheckAssertion(inputFlags[storageClassFlag], "", cmdOut.Out)
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					" Preflight check for DNS resolution failed :: namespaces \"%s\" not found", inputFlags[namespaceFlag])))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					"Preflight check for volume snapshot and restore failed :: namespaces \"%s\" not found", inputFlags[namespaceFlag])))
			})

			It("Should fail preflight check if namespace flag is provided with zero value", func() {
				var output []byte
				args := []string{"run", storageClassFlag, defaultTestStorageClass,
					namespaceFlag, "", kubeconfigFlag, kubeConfPath,
					cleanupOnFailureFlag}
				cmd := exec.Command(preflightBinaryFilePath, args...)
				tLog.Infof("Preflight check CMD [%s]", cmd)
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(BeNil())
				tLog.Infof("Preflight binary run execution output: %s", string(output))

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

				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("KUBECONFIG-PATH=\"%s\"", preflightKubeConf)))

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

		Context("Preflight run command, inCluster flag test cases", func() {

			It("Should skip kubectl and helm checks when inCluster flag is set to true", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[inClusterFlag] = ""
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())
				Expect(cmdOut.Out).To(And(ContainSubstring("In cluster flag enabled. Skipping check for kubectl"),
					ContainSubstring("In cluster flag enabled. Skipping check for helm")))
				Expect(cmdOut.Out).NotTo(And(ContainSubstring("Checking for kubectl"),
					ContainSubstring("Checking for required Helm version")))
			})

		})

		Context("Preflight run command, volume snapshot pod resource requests and limits flag testcase", func() {
			It("Pods for volume snapshot check should use CPU and memory resources according to the given flag values", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[requestsFlag] = fmt.Sprintf("cpu=%s,memory=%s", cpu300, cmd.DefaultPodLimitMemory)
				inputFlags[limitsFlag] = fmt.Sprintf("cpu=%s,memory=%s", cpu600, memory256)
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU REQUEST=\"%s\"", cpu300)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU REQUEST=\"%s\"", cpu300)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD MEMORY REQUEST=\"%s\"", cmd.DefaultPodLimitMemory)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU LIMIT=\"%s\"", cpu600)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD MEMORY LIMIT=\"%s\"", memory256)))

				assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
			})

			It("Should not perform preflight checks if volume snapshot pod request memory is greater than limit memory", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[requestsFlag] = strings.Join([]string{resourceMemoryToken, memory256}, "=")
				inputFlags[limitsFlag] = strings.Join([]string{resourceMemoryToken, cmd.DefaultPodLimitMemory}, "=")
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring("request memory cannot be greater than limit memory"))
			})

			It("Should not perform preflight checks if volume snapshot pod request cpu is greater than limit cpu", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[requestsFlag] = strings.Join([]string{resourceCPUToken, cpu600}, "=")
				inputFlags[limitsFlag] = strings.Join([]string{resourceCPUToken, cpu300}, "=")
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring("request CPU cannot be greater than limit CPU"))
			})
		})

		Context("Preflight run command, config file flag test cases", func() {
			It("Should perform preflight checks when inputs are provided from a yaml file", func() {
				yamlFilePath := filepath.Join(testDataDirRelPath, testFileInputName)
				inputFlags := make(map[string]string)
				inputFlags[kubeconfigFlag] = kubeConfPath
				inputFlags[configFileFlag] = yamlFilePath
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				nonCRUDPreflightCheckAssertion(defaultTestStorageClass, defaultTestSnapshotClass, cmdOut.Out)
				assertDNSResolutionCheckSuccess(cmdOut.Out)
				assertVolumeSnapshotCheckSuccess(cmdOut.Out)
				assertPVCStorageRequestCheckSuccess(cmdOut.Out, "")
			})

			It("Should perform preflight checks with with file inputs overridden by CLI flag inputs", func() {
				yamlFilePath := filepath.Join(testDataDirRelPath, testFileInputName)
				createNamespace(flagNamespace)
				defer deleteNamespace(flagNamespace)
				inputFlags := make(map[string]string)
				inputFlags[kubeconfigFlag] = kubeConfPath
				inputFlags[configFileFlag] = yamlFilePath
				inputFlags[namespaceFlag] = flagNamespace
				inputFlags[pvcStorageRequestFlag] = "2Gi"
				inputFlags[requestsFlag] = strings.Join([]string{resourceCPUToken, cpu400}, "=")
				inputFlags[limitsFlag] = strings.Join([]string{resourceMemoryToken, memory256}, "=")
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU REQUEST=\"%s\"", cpu400)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD MEMORY REQUEST=\"%s\"", cmd.DefaultPodRequestMemory)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU LIMIT=\"%s\"", cmd.DefaultPodLimitCPU)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD MEMORY LIMIT=\"%s\"", memory256)))

				nonCRUDPreflightCheckAssertion(defaultTestStorageClass, defaultTestSnapshotClass, cmdOut.Out)
				assertDNSResolutionCheckSuccess(cmdOut.Out)
				assertVolumeSnapshotCheckSuccess(cmdOut.Out)
				assertPVCStorageRequestCheckSuccess(cmdOut.Out, inputFlags[pvcStorageRequestFlag])
			})

			It("Should not perform preflight checks if file does not exist at the given path", func() {
				inputFlags := make(map[string]string)
				inputFlags[configFileFlag] = invalidYamlFilePath
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					"failed to read preflight input from file :: open %s: no such file or directory", invalidYamlFilePath)))
			})

			It("Should not be able to perform preflight checks if file read permission is not present", func() {
				var file *os.File
				file, err = os.OpenFile(permYamlFile, os.O_CREATE, 0000)
				Expect(err).To(BeNil())
				file.Close()
				inputFlags := make(map[string]string)
				inputFlags[configFileFlag] = permYamlFile
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					"failed to read preflight input from file :: open %s: permission denied", permYamlFile)))

				err = os.Remove(permYamlFile)
				Expect(err).To(BeNil())
			})

			It("Should not perform preflight checks if file contains invalid keys or invalid key hierarchy", func() {
				yamlFilePath := filepath.Join([]string{testDataDirRelPath, invalidKeyYamlFileName}...)
				inputFlags := make(map[string]string)
				inputFlags[configFileFlag] = yamlFilePath
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring("failed to read preflight input from file :: " +
					"error unmarshaling JSON: while decoding JSON: json: unknown field"))
			})
		})

		Context("Preflight run command, pvc storage request flag test cases", func() {
			It("Should not perform preflight checks if pvc storage request value is provided in invalid format", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[pvcStorageRequestFlag] = invalidPVCStorageRequest
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("cannot parse '%s': quantities must match the regular expression "+
						"'^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'", invalidPVCStorageRequest)))
			})
		})

		Context("Preflight pod scheduling test cases", func() {

			Context("Preflight run command, node selector test cases", func() {
				var (
					beforeOnce   sync.Once
					afterOnce    sync.Once
					nodeList     *corev1.NodeList
					testNodeName string
				)
				BeforeEach(func() {
					beforeOnce.Do(func() {
						nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
						Expect(err).To(BeNil())
						Expect(len(nodeList.Items)).ToNot(Equal(0))
						node := nodeList.Items[0]
						testNodeName = node.GetName()
						nodeLabels := node.GetLabels()
						nodeLabels[preflightNodeLabelKey] = preflightNodeLabelValue
						node.SetLabels(nodeLabels)
						_, err = k8sClient.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{})
						Expect(err).To(BeNil())
					})
				})

				It(fmt.Sprintf("Should schedule preflight pods on node - %s and perform preflight checks", testNodeName), func() {
					inputFlags := make(map[string]string)
					copyMap(flagsMap, inputFlags)
					inputFlags[nodeSelectorFlag] = strings.Join(
						[]string{preflightNodeLabelKey, preflightNodeLabelValue}, "=")
					inputFlags[logLevelFlag] = debugLog
					cmdOut, err = runPreflightChecks(inputFlags)
					Expect(err).To(BeNil())

					assertPodScheduleSuccess(cmdOut.Out, testNodeName)
					assertSuccessfulPreflightChecks(inputFlags, cmdOut.Out)
				})

				It("Should not be able to schedule DNS and source pod when node selector does not match any node on the cluster", func() {
					inputFlags := make(map[string]string)
					copyMap(flagsMap, inputFlags)
					inputFlags[nodeSelectorFlag] = strings.Join([]string{invalidNodeSelectorKey, invalidNodeSelectorValue}, "=")
					cmdOut, err = runPreflightChecks(inputFlags)
					Expect(err).ToNot(BeNil())

					Expect(cmdOut.Out).To(MatchRegexp("DNS pod - dnsutils-[a-z]{6} hasn't reached into ready state"))
					Expect(cmdOut.Out).To(ContainSubstring(
						"Preflight check for DNS resolution failed :: timed out waiting for the condition"))

					Expect(cmdOut.Out).To(MatchRegexp(
						"Preflight check for volume snapshot and restore failed :: pod source-pod-[a-z]{6} hasn't reached into ready state"))
				})

				AfterEach(func() {
					afterOnce.Do(func() {
						var testNode *corev1.Node
						testNode, err = k8sClient.CoreV1().Nodes().Get(ctx, testNodeName, metav1.GetOptions{})
						Expect(err).To(BeNil())
						nodeLabels := testNode.GetLabels()
						delete(nodeLabels, preflightNodeLabelKey)
						testNode.SetLabels(nodeLabels)
						_, err = k8sClient.CoreV1().Nodes().Update(ctx, testNode, metav1.UpdateOptions{})
						Expect(err).To(BeNil())
					})
				})
			})

			Context("Preflight run command, node affinity test cases with pods scheduled on nodes of cluster", func() {
				var (
					nodeList           *corev1.NodeList
					highAffineTestNode string
					lowAffineTestNode  string
					node               *corev1.Node
				)

				BeforeEach(func() {
					nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					Expect(err).To(BeNil())
					Expect(len(nodeList.Items)).ToNot(Equal(0))
					node = &nodeList.Items[0]
					highAffineTestNode = node.GetName()
					nodeLabels := node.GetLabels()
					nodeLabels[preflightNodeAffinityKey] = highAffinity
					node.SetLabels(nodeLabels)
					_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(err).To(BeNil())

					if len(nodeList.Items) > 1 {
						node = &nodeList.Items[1]
						lowAffineTestNode = node.GetName()
						nodeLabels = node.GetLabels()
						nodeLabels[preflightNodeAffinityKey] = lowAffinity
						node.SetLabels(nodeLabels)
						_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
						Expect(err).To(BeNil())
					}
				})

				It(fmt.Sprintf("Should read node affinity labels from yaml file and able to schedule pods on node - %s", highAffineTestNode), func() {
					yamlFilePath := filepath.Join(testDataDirRelPath, nodeAffinityInputFile)
					inputFlags := make(map[string]string)
					copyMap(flagsMap, inputFlags)
					inputFlags[configFileFlag] = yamlFilePath
					cmdOut, err = runPreflightChecks(inputFlags)
					Expect(err).To(BeNil())

					assertPodScheduleSuccess(cmdOut.Out, highAffineTestNode)

					nonCRUDPreflightCheckAssertion(defaultTestStorageClass, "", cmdOut.Out)
					assertDNSResolutionCheckSuccess(cmdOut.Out)
					assertVolumeSnapshotCheckSuccess(cmdOut.Out)
				})

				AfterEach(func() {
					node, err = k8sClient.CoreV1().Nodes().Get(ctx, highAffineTestNode, metav1.GetOptions{})
					Expect(err).To(BeNil())
					nodeLabels := node.GetLabels()
					delete(nodeLabels, preflightNodeAffinityKey)
					node.SetLabels(nodeLabels)
					_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(err).To(BeNil())

					if len(nodeList.Items) > 1 {
						node, err = k8sClient.CoreV1().Nodes().Get(ctx, lowAffineTestNode, metav1.GetOptions{})
						Expect(err).To(BeNil())
						nodeLabels = node.GetLabels()
						delete(nodeLabels, preflightNodeAffinityKey)
						node.SetLabels(nodeLabels)
						_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
						Expect(err).To(BeNil())
					}
				})
			})

			Context("Preflight run command, node affinity test cases and pods not able to scheduled on cluster", func() {
				var nodeList *corev1.NodeList

				BeforeEach(func() {
					nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					Expect(err).To(BeNil())
					for _, node := range nodeList.Items {
						nodeLabels := node.GetLabels()
						nodeLabels[preflightNodeAffinityKey] = lowAffinity
						node.SetLabels(nodeLabels)
						_, err = k8sClient.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{})
						Expect(err).To(BeNil())
					}
				})

				It("Should not be able to schedule DNS and source pod on cluster when pod affinity required rules do not satisfy", func() {
					yamlFilePath := filepath.Join(testDataDirRelPath, nodeAffinityInputFile)
					inputFlags := make(map[string]string)
					copyMap(flagsMap, inputFlags)
					inputFlags[configFileFlag] = yamlFilePath
					cmdOut, err = runPreflightChecks(inputFlags)
					Expect(err).ToNot(BeNil())

					Expect(cmdOut.Out).To(MatchRegexp("DNS pod - dnsutils-[a-z]{6} hasn't reached into ready state"))
					Expect(cmdOut.Out).To(ContainSubstring(
						"Preflight check for DNS resolution failed :: timed out waiting for the condition"))

					Expect(cmdOut.Out).To(MatchRegexp(
						"Preflight check for volume snapshot and restore failed :: pod source-pod-[a-z]{6} hasn't reached into ready state"))
				})

				AfterEach(func() {
					nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					Expect(err).To(BeNil())
					for _, node := range nodeList.Items {
						nodeLabels := node.GetLabels()
						delete(nodeLabels, preflightNodeAffinityKey)
						node.SetLabels(nodeLabels)
						_, err = k8sClient.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{})
						Expect(err).To(BeNil())
					}
				})
			})

			Context("Preflight run command, pod affinity test cases with pods able to schedule on node of a cluster", func() {
				var (
					nodeList     *corev1.NodeList
					testNodeName string
					node         *corev1.Node
				)

				BeforeEach(func() {
					nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					Expect(err).To(BeNil())
					Expect(len(nodeList.Items)).ToNot(Equal(0))
					node = &nodeList.Items[0]
					testNodeName = node.GetName()
					nodeLabels := node.GetLabels()
					nodeLabels[preflightNodeLabelKey] = preflightNodeLabelValue
					node.SetLabels(nodeLabels)
					node, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(err).To(BeNil())
					createAffineBusyboxPod(preflightBusyboxPod, mediumAffinity, defaultTestNs)
				})

				It(fmt.Sprintf("Should schedule preflight pods on node where pod '%s' is scheduled", preflightBusyboxPod), func() {
					yamlFilePath := filepath.Join(testDataDirRelPath, podAffinityInputFile)
					inputFlags := make(map[string]string)
					copyMap(flagsMap, inputFlags)
					inputFlags[configFileFlag] = yamlFilePath
					inputFlags[namespaceFlag] = defaultTestNs
					cmdOut, err = runPreflightChecks(inputFlags)
					Expect(err).To(BeNil())

					assertPodScheduleSuccess(cmdOut.Out, testNodeName)
					nonCRUDPreflightCheckAssertion(defaultTestStorageClass, "", cmdOut.Out)
					assertDNSResolutionCheckSuccess(cmdOut.Out)
					assertVolumeSnapshotCheckSuccess(cmdOut.Out)
				})

				AfterEach(func() {
					node, err = k8sClient.CoreV1().Nodes().Get(ctx, testNodeName, metav1.GetOptions{})
					nodeLabels := node.GetLabels()
					delete(nodeLabels, preflightNodeLabelKey)
					node.SetLabels(nodeLabels)
					node, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(err).To(BeNil())
					deleteAffineBusyboxPod(preflightBusyboxPod, defaultTestNs)
				})
			})

			Context("Preflight run command, pod affinity test cases with pods not able to schedule on node of a cluster", func() {
				var (
					nodeList     *corev1.NodeList
					testNodeName string
					node         *corev1.Node
				)

				BeforeEach(func() {
					nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					Expect(err).To(BeNil())
					Expect(len(nodeList.Items)).ToNot(Equal(0))
					node = &nodeList.Items[0]
					testNodeName = node.GetName()
					nodeLabels := node.GetLabels()
					nodeLabels[preflightNodeLabelKey] = preflightNodeLabelValue
					node.SetLabels(nodeLabels)
					node, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(err).To(BeNil())
					createAffineBusyboxPod(preflightBusyboxPod, highAffinity, defaultTestNs)
				})

				It("Should not be able to schedule DNS and source pod on any node of the cluster", func() {
					yamlFilePath := filepath.Join(testDataDirRelPath, podAffinityInputFile)
					inputFlags := make(map[string]string)
					copyMap(flagsMap, inputFlags)
					inputFlags[configFileFlag] = yamlFilePath
					cmdOut, err = runPreflightChecks(inputFlags)
					Expect(err).ToNot(BeNil())

					Expect(cmdOut.Out).To(MatchRegexp("DNS pod - dnsutils-[a-z]{6} hasn't reached into ready state"))
					Expect(cmdOut.Out).To(ContainSubstring(
						"Preflight check for DNS resolution failed :: timed out waiting for the condition"))

					Expect(cmdOut.Out).To(MatchRegexp(
						"Preflight check for volume snapshot and restore failed :: pod source-pod-[a-z]{6} hasn't reached into ready state"))
				})

				AfterEach(func() {
					node, err = k8sClient.CoreV1().Nodes().Get(ctx, testNodeName, metav1.GetOptions{})
					nodeLabels := node.GetLabels()
					delete(nodeLabels, preflightNodeLabelKey)
					node.SetLabels(nodeLabels)
					node, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(err).To(BeNil())
					deleteAffineBusyboxPod(preflightBusyboxPod, defaultTestNs)
				})
			})

			//Context("Preflight run command, taints and tolerations test cases with pods able to schedule on node of a cluster", func() {
			//	var (
			//		nodeList      *corev1.NodeList
			//		taintNodeName string
			//		node          *corev1.Node
			//	)
			//
			//	BeforeEach(func() {
			//		nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			//		Expect(err).To(BeNil())
			//		Expect(len(nodeList.Items)).ToNot(Equal(0))
			//		node = &nodeList.Items[0]
			//		taintNodeName = node.GetName()
			//		nodeLabels := node.GetLabels()
			//		nodeLabels[preflightNodeLabelKey] = preflightNodeLabelValue
			//		node.SetLabels(nodeLabels)
			//		nodeTaints := node.Spec.Taints
			//		nodeTaints = append(nodeTaints, corev1.Taint{
			//			Key:    preflightTaintKey,
			//			Value:  preflightTaintValue,
			//			Effect: corev1.TaintEffectNoSchedule,
			//		})
			//		node.Spec.Taints = nodeTaints
			//		_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			//	})
			//
			//	It("Should schedule preflight pods on a node of a cluster and perform preflight checks with tolerations applied on pods", func() {
			//		inputFlags := make(map[string]string)
			//		yamlFilePath := filepath.Join(testDataDirRelPath, taintsFileInputFile)
			//		copyMap(flagsMap, inputFlags)
			//		inputFlags[configFileFlag] = yamlFilePath
			//		inputFlags[logLevelFlag] = debugLog
			//		cmdOut, err = runPreflightChecks(inputFlags)
			//		Expect(err).To(BeNil())
			//
			//		assertPodScheduleSuccess(cmdOut.Out, taintNodeName)
			//		nonCRUDPreflightCheckAssertion(defaultTestStorageClass, "", cmdOut.Out)
			//		assertDNSResolutionCheckSuccess(cmdOut.Out)
			//		assertVolumeSnapshotCheckSuccess(cmdOut.Out)
			//	})
			//
			//	AfterEach(func() {
			//		taintPos := -1
			//		node, err = k8sClient.CoreV1().Nodes().Get(ctx, taintNodeName, metav1.GetOptions{})
			//		nodeLabels := node.GetLabels()
			//		delete(nodeLabels, preflightNodeLabelKey)
			//		node.SetLabels(nodeLabels)
			//		nodeTaints := node.Spec.Taints
			//		for i := 0; i < len(nodeTaints); i++ {
			//			if nodeTaints[i].Key == preflightTaintKey {
			//				taintPos = i
			//				break
			//			}
			//		}
			//		Expect(taintPos).ToNot(Equal(-1))
			//		node.Spec.Taints = append(nodeTaints[:taintPos], nodeTaints[taintPos+1:]...)
			//		_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			//	})
			//})
			//
			//Context("Preflight run command, taints and tolerations test cases with pods not able to schedule on node of a cluster", func() {
			//	var (
			//		nodeList      *corev1.NodeList
			//		taintNodeName string
			//		node          *corev1.Node
			//	)
			//
			//	BeforeEach(func() {
			//		nodeList, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			//		Expect(err).To(BeNil())
			//		Expect(len(nodeList.Items)).ToNot(Equal(0))
			//		node = &nodeList.Items[0]
			//		taintNodeName = node.GetName()
			//		nodeLabels := node.GetLabels()
			//		nodeLabels[preflightNodeLabelKey] = preflightNodeLabelValue
			//		node.SetLabels(nodeLabels)
			//		nodeTaints := node.Spec.Taints
			//		nodeTaints = append(nodeTaints, corev1.Taint{
			//			Key:    preflightTaintKey,
			//			Value:  preflightTaintInvValue,
			//			Effect: corev1.TaintEffectNoSchedule,
			//		})
			//		node.Spec.Taints = nodeTaints
			//		_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			//	})
			//
			//	It("Should not be able to schedule DNS and source pod on any nodes of cluster with incorrect toleration applied", func() {
			//		inputFlags := make(map[string]string)
			//		yamlFilePath := filepath.Join(testDataDirRelPath, taintsFileInputFile)
			//		inputFlags[configFileFlag] = yamlFilePath
			//		inputFlags[logLevelFlag] = debugLog
			//		inputFlags[kubeconfigFlag] = kubeConfPath
			//		cmdOut, err = runPreflightChecks(inputFlags)
			//		Expect(err).ToNot(BeNil())
			//
			//		Expect(cmdOut.Out).To(MatchRegexp("DNS pod - dnsutils-[a-z]{6} hasn't reached into ready state"))
			//		Expect(cmdOut.Out).To(ContainSubstring(
			//			"Preflight check for DNS resolution failed :: timed out waiting for the condition"))
			//
			//		Expect(cmdOut.Out).To(MatchRegexp(
			//			"Preflight check for volume snapshot and restore failed :: pod source-pod-[a-z]{6} hasn't reached into ready state"))
			//	})
			//
			//	AfterEach(func() {
			//		taintPos := -1
			//		node, err = k8sClient.CoreV1().Nodes().Get(ctx, taintNodeName, metav1.GetOptions{})
			//		nodeLabels := node.GetLabels()
			//		delete(nodeLabels, preflightNodeLabelKey)
			//		node.SetLabels(nodeLabels)
			//		nodeTaints := node.Spec.Taints
			//		for i := 0; i < len(nodeTaints); i++ {
			//			if nodeTaints[i].Key == preflightTaintKey {
			//				taintPos = i
			//				break
			//			}
			//		}
			//		Expect(taintPos).ToNot(Equal(-1))
			//		node.Spec.Taints = append(nodeTaints[:taintPos], nodeTaints[taintPos+1:]...)
			//		_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			//	})
			//})
		})
	})

	Context("Preflight cleanup command test-cases", func() {

		Context("cleanup all preflight resources on the cluster in a particular namespace", func() {

			It("Should clean all preflight resources in a particular namespace", func() {
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				cmdOut, err = runCleanupForAllPreflightResources()
				Expect(err).To(BeNil())

				assertSuccessCleanupAll(cmdOut.Out)
			})
		})

		Context("cleanup resources according to preflight UID in a particular namespace", func() {

			It("Should clean pods, PVCs and volume snapshots of preflight check for specific uid", func() {
				var uid = createPreflightResourcesForCleanup()
				cmdOut, err = runCleanupWithUID(uid)
				Expect(err).To(BeNil())

				assertSuccessCleanupUID(uid, cmdOut.Out)
			})
		})

		Context("cleanup resources according to the input given in yaml file", func() {
			It("Should cleanup resources with a particular UID when cleanup inputs are given through a file "+
				"and 'uid' field is specified in the file", func() {

				var file *os.File
				uid := createPreflightResourcesForCleanup()
				yamlFilePath := filepath.Join(testDataDirRelPath, cleanupUIDInputYamlFile)
				file, err = os.OpenFile(yamlFilePath, os.O_CREATE|os.O_WRONLY, filePermission)
				defer func() {
					err = file.Close()
					Expect(err).To(BeNil())
				}()
				uidCleanupFileData := cleanupFileInputData
				uidCleanupFileData += strings.Join([]string{"\n", fmt.Sprintf("  uid: %s", uid)}, "")
				_, err = file.Write([]byte(uidCleanupFileData))
				Expect(err).To(BeNil())

				cmd := fmt.Sprintf("%s cleanup -f %s -k %s", preflightBinaryFilePath, yamlFilePath, kubeConfPath)
				tLog.Infof("Preflight cleanup CMD [%s]", cmd)
				cmdOut, err = shell.RunCmd(cmd)
				tLog.Infof("Preflight binary cleanup execution output: %s", cmdOut.Out)

				assertSuccessCleanupUID(uid, cmdOut.Out)

				err = os.Remove(yamlFilePath)
				Expect(err).To(BeNil())
			})

			It("Should cleanup all preflight resources when cleanup inputs are given through  file "+
				"and no value for uid field is specified", func() {
				var file *os.File
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				yamlFilePath := filepath.Join(testDataDirRelPath, cleanupAllInputYamlFile)
				file, err = os.OpenFile(yamlFilePath, os.O_CREATE|os.O_WRONLY, filePermission)
				defer func() {
					file.Close()
					Expect(err).To(BeNil())
				}()
				_, err = file.Write([]byte(cleanupFileInputData))
				cmd := fmt.Sprintf("%s cleanup -f %s -k %s", preflightBinaryFilePath, yamlFilePath, kubeConfPath)
				tLog.Infof("Preflight cleanup CMD [%s]", cmd)
				cmdOut, err = shell.RunCmd(cmd)
				tLog.Infof("Preflight binary cleanup execution output: %s", cmdOut.Out)

				assertSuccessCleanupAll(cmdOut.Out)

				err = os.Remove(yamlFilePath)
				Expect(err).To(BeNil())
			})

			It("Should not perform cleanup if invalid input file path is given", func() {
				cmd := fmt.Sprintf("%s cleanup -f %s", preflightBinaryFilePath, invalidYamlFilePath)
				tLog.Infof("Preflight cleanup CMD [%s]", cmd)
				cmdOut, err = shell.RunCmd(cmd)
				tLog.Infof("Preflight binary cleanup execution output: %s", cmdOut.Out)

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("preflight command execution failed - open %s: no such file or directory",
						invalidYamlFilePath)))
			})
		})

		Context("cleanup resources according to preflight UID in a particular namespace", func() {

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
