package preflighttest

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tLog "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/cmd/preflight/cmd"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

var _ = Describe("Preflight Tests", func() {

	Context("Preflight run command test-cases", func() {

		Context("Preflight run command storage class flag test cases", func() {

			It("All preflight checks should pass if correct storage class flag value is provided in namespace scope", func() {
				cmdOut, err = runPreflightChecks(flagsMap)
				Expect(err).To(BeNil())

				assertSuccessfulPreflightChecks(flagsMap, cmdOut.Out)
			})

			It("All preflight checks should pass if correct storage class flag value is provided in cluster scope", func() {
				clusterScopeInputs := make(map[string]string)
				copyMap(flagsMap, clusterScopeInputs)
				clusterScopeInputs[scopeFlag] = internal.ClusterScope
				cmdOut, err = runPreflightChecks(clusterScopeInputs)
				Expect(err).To(BeNil())

				assertSuccessfulPreflightChecks(clusterScopeInputs, cmdOut.Out)
			})

			It("Should fail preflight checks if incorrect storage class flag value is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[storageClassFlag] = internal.InvalidStorageClassName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(
					ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
						"not found storageclass - %s on cluster", internal.InvalidStorageClassName)))
				Expect(cmdOut.Out).To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
				Expect(cmdOut.Out).To(ContainSubstring("Some preflight checks failed"))
			})

			It("Should not preform preflight checks if storage class flag is not provided", func() {
				cmd := fmt.Sprintf("%s run", preflightBinaryFilePath)
				_, err = shell.RunCmd(cmd)
				Expect(err).ToNot(BeNil())
			})
		})

		// TODO - Uncomment the tests when suite will be run on prow.
		//Context("Restricted Pod Security Policy is present", func() {
		//	BeforeEach(func() {
		//		// delete privileged PSP
		//		err = k8sClient.PolicyV1beta1().PodSecurityPolicies().Delete(ctx, privilegedPSP.Name, metav1.DeleteOptions{})
		//		Expect(err).To(BeNil())
		//
		//		// create restricted PSP
		//		_, err = k8sClient.PolicyV1beta1().PodSecurityPolicies().Create(ctx, restrictedPSP, metav1.CreateOptions{})
		//		Expect(err).To(BeNil())
		//	})
		//	AfterEach(func() {
		//		// create privileged PSP
		//		_, err = k8sClient.PolicyV1beta1().PodSecurityPolicies().Create(ctx, privilegedPSP, metav1.CreateOptions{})
		//		Expect(err).To(BeNil())
		//
		//		// delete restricted PSP
		//		err = k8sClient.PolicyV1beta1().PodSecurityPolicies().Delete(ctx, restrictedPSP.Name, metav1.DeleteOptions{})
		//		Expect(err).To(BeNil())
		//	})
		//	It("should fail the preflight", func() {
		//		cmdOut, err = runPreflightChecks(flagsMap)
		//		Expect(err).ToNot(BeNil())
		//		Expect(cmdOut.Out).To(ContainSubstring("Failed to create capability validator pod"))
		//		Expect(cmdOut.Out).To(ContainSubstring("Some preflight checks failed"))
		//	})
		//})

		Context("Preflight run command local registry flag test cases", func() {
			// TODO: shiwam, long running test, either add timeout if possible or remove from IT and put in UT
			It("Should fail DNS resolution and volume snapshot check if invalid local registry path is provided", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[localRegistryFlag] = internal.InvalidLocalRegistryName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(
					MatchRegexp("(DNS pod - dnsutils-)([a-z]{6})( hasn't reached into ready state)"))
				Expect(cmdOut.Out).To(
					ContainSubstring("Preflight check for DNS resolution failed :: timed out waiting for the condition"))
				Expect(cmdOut.Out).To(MatchRegexp(
					fmt.Sprintf("Preflight check for %s scope volume snapshot and restore failed :: "+
						"pod: %s/source-pvc-writer-([a-z]{6}), hasn't reached into ready state", inputFlags[scopeFlag], inputFlags[namespaceFlag])))

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
				inputFlags[serviceAccountFlag] = internal.InvalidServiceAccountName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).
					To(MatchRegexp(fmt.Sprintf("(Preflight check for DNS resolution failed :: pods \"dnsutils-)([a-z]{6}\")"+
						"( is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found)",
						defaultTestNs, internal.InvalidServiceAccountName, internal.InvalidServiceAccountName)))

				Expect(cmdOut.Out).To(MatchRegexp(
					fmt.Sprintf("pods \"source-pvc-writer-([a-z]{6})\" is forbidden: "+
						"error looking up service account %s/%s: serviceaccount \"%s\" not found",
						defaultTestNs, internal.InvalidServiceAccountName, internal.InvalidServiceAccountName)))

				Expect(cmdOut.Out).To(MatchRegexp(
					fmt.Sprintf("(Preflight check for %s scope volume snapshot and restore failed)(.*)"+
						"(error looking up service account %s/%s: serviceaccount \"%s\" not found)", inputFlags[scopeFlag],
						defaultTestNs, internal.InvalidServiceAccountName, internal.InvalidServiceAccountName)))

				nonCRUDPreflightCheckAssertion(inputFlags[storageClassFlag], "", cmdOut.Out)
			})
		})

		Context("Preflight run command cleanup on failure flag test cases", func() {
			It("Should not clean resources when preflight check fails and cleanup on failure flag is set to false", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				delete(inputFlags, cleanupOnFailureFlag)
				inputFlags[serviceAccountFlag] = internal.InvalidServiceAccountName
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

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

			It("Should fail DNS and volume snapshot check if given namespace is not present on cluster", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[namespaceFlag] = internal.InvalidNamespace
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				nonCRUDPreflightCheckAssertion(inputFlags[storageClassFlag], "", cmdOut.Out)
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					" Preflight check for DNS resolution failed :: namespaces \"%s\" not found", inputFlags[namespaceFlag])))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf(
					"Preflight check for %s scope volume snapshot and restore failed ::"+
						" namespaces \"%s\" not found", inputFlags[scopeFlag], inputFlags[namespaceFlag])))
			})

			It("Should fail preflight check if namespace flag is provided with zero value", func() {
				var output []byte
				args := []string{"run", storageClassFlag, internal.DefaultTestStorageClass,
					namespaceFlag, "", kubeconfigFlag, kubeConfPath,
					cleanupOnFailureFlag, scopeFlag, internal.NamespaceScope}
				cmd := exec.Command(preflightBinaryFilePath, args...)
				tLog.Infof("Preflight check CMD [%s]", cmd)
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(BeNil())
				tLog.Infof("Preflight binary run execution output: %s", string(output))

				Expect(string(output)).To(ContainSubstring("Preflight check for DNS resolution failed :: " +
					"an empty namespace may not be set during creation"))
				Expect(string(output)).To(ContainSubstring(fmt.Sprintf("Preflight check for %s scope volume snapshot and restore failed :: "+
					"an empty namespace may not be set during creation", internal.NamespaceScope)))
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

			It("Should perform preflight check using kubeconfig file mentioned in KUBECONFIG env, "+
				"if kubeconfig flag is provided with zero value", func() {
				tLog.Infof("KUBECONFIG env file path: %s", kubeConfPath)

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
				Expect(err).To(BeNil())
				Expect(cmdOut.Out).To(And(ContainSubstring("In cluster flag enabled. Skipping check for kubectl"),
					ContainSubstring("In cluster flag enabled. Skipping check for helm")))
				Expect(cmdOut.Out).NotTo(And(ContainSubstring("Checking for kubectl"),
					ContainSubstring("Checking for required Helm version")))
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

				nonCRUDPreflightCheckAssertion(internal.DefaultTestStorageClass, internal.DefaultTestSnapshotClass, cmdOut.Out)
				assertDNSResolutionCheckSuccess(cmdOut.Out)
				assertVolumeSnapshotCheckSuccess(cmdOut.Out, internal.DefaultNs, internal.NamespaceScope)
				assertPVCStorageRequestCheckSuccess(cmdOut.Out, "")
			})

			It("Should perform preflight checks with with file inputs overridden by CLI flag inputs", func() {
				yamlFilePath := filepath.Join(testDataDirRelPath, testFileInputName)
				createNamespace(flagNamespace, nil)
				defer deleteNamespace(flagNamespace)
				inputFlags := make(map[string]string)
				inputFlags[kubeconfigFlag] = kubeConfPath
				inputFlags[configFileFlag] = yamlFilePath
				inputFlags[namespaceFlag] = flagNamespace
				inputFlags[scopeFlag] = internal.NamespaceScope
				inputFlags[pvcStorageRequestFlag] = "2Gi"
				inputFlags[requestsFlag] = strings.Join([]string{resourceCPUToken, internal.CPU400}, "=")
				inputFlags[limitsFlag] = strings.Join([]string{resourceMemoryToken, internal.Memory256}, "=")
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).To(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU REQUEST=\"%s\"", internal.CPU400)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD MEMORY REQUEST=\"%s\"", cmd.DefaultPodRequestMemory)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD CPU LIMIT=\"%s\"", cmd.DefaultPodLimitCPU)))
				Expect(cmdOut.Out).To(ContainSubstring(fmt.Sprintf("POD MEMORY LIMIT=\"%s\"", internal.Memory256)))

				nonCRUDPreflightCheckAssertion(internal.DefaultTestStorageClass, internal.DefaultTestSnapshotClass, cmdOut.Out)
				assertDNSResolutionCheckSuccess(cmdOut.Out)
				assertVolumeSnapshotCheckSuccess(cmdOut.Out, inputFlags[namespaceFlag], inputFlags[scopeFlag])
				assertPVCStorageRequestCheckSuccess(cmdOut.Out, inputFlags[pvcStorageRequestFlag])
			})

		})

		Context("Preflight run command, pvc storage request flag test cases", func() {

			It("Should not perform preflight checks if pvc storage request value is provided in invalid format", func() {
				inputFlags := make(map[string]string)
				copyMap(flagsMap, inputFlags)
				inputFlags[pvcStorageRequestFlag] = internal.InvalidMemoryResourceRequest
				cmdOut, err = runPreflightChecks(inputFlags)
				Expect(err).ToNot(BeNil())

				Expect(cmdOut.Out).To(ContainSubstring(
					fmt.Sprintf("cannot parse '%s': quantities must match the regular expression "+
						"'^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'", internal.InvalidMemoryResourceRequest)))
			})
		})

		Context("Preflight pod scheduling test cases", func() {

			Context("Preflight run command, node selector test cases", Ordered, func() {
				var (
					nodeList     *corev1.NodeList
					testNodeName string
				)
				BeforeAll(func() {
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

				// TODO: shiwam, should we have this in IT, reduce the runtime if kept here otherwise it's not worth it?
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
						fmt.Sprintf("Preflight check for %s scope volume snapshot and restore failed :: pod: %s/source-pvc-writer-([a-z]{6}),"+
							" hasn't reached into ready state", inputFlags[scopeFlag], inputFlags[namespaceFlag])))
				})

				AfterAll(func() {

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
					highAffinityNode, hErr := k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
					Expect(hErr).To(BeNil())
					Expect(highAffinityNode.GetLabels()[preflightNodeAffinityKey]).To(Equal(highAffinity))

					if len(nodeList.Items) > 1 {
						node = &nodeList.Items[1]
						lowAffineTestNode = node.GetName()
						nodeLabels = node.GetLabels()
						nodeLabels[preflightNodeAffinityKey] = lowAffinity
						node.SetLabels(nodeLabels)
						lowAffinityNode, lErr := k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
						Expect(lErr).To(BeNil())
						Expect(lowAffinityNode.GetLabels()[preflightNodeAffinityKey]).To(Equal(lowAffinity))
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

					nonCRUDPreflightCheckAssertion(internal.DefaultTestStorageClass, "", cmdOut.Out)
					assertDNSResolutionCheckSuccess(cmdOut.Out)
					assertVolumeSnapshotCheckSuccess(cmdOut.Out, defaultTestNs, internal.NamespaceScope)
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
				// TODO: shiwam, long running test, either add timeout if possible or remove from IT and put in UT
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
						fmt.Sprintf("Preflight check for %s scope volume snapshot and restore failed :: "+
							"pod: %s/source-pvc-writer-([a-z]{6}), hasn't reached into ready state", inputFlags[scopeFlag], inputFlags[namespaceFlag])))
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
					nonCRUDPreflightCheckAssertion(internal.DefaultTestStorageClass, "", cmdOut.Out)
					assertDNSResolutionCheckSuccess(cmdOut.Out)
					assertVolumeSnapshotCheckSuccess(cmdOut.Out, inputFlags[namespaceFlag], internal.NamespaceScope)
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

				// TODO: shiwam, long running test, either add timeout if possible or remove from IT and put in UT
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
						fmt.Sprintf("Preflight check for %s scope volume snapshot and restore failed :: "+
							"pod: %s/source-pvc-writer-([a-z]{6}), hasn't reached into ready state", inputFlags[scopeFlag], inputFlags[namespaceFlag])))
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

		Context("cleanup all preflight resources in a particular namespace"+
			" along with other namespaces created as a part of cluster scope checks", func() {

			It("Should clean all preflight resources with the same uid", func() {
				uid := createPreflightResourcesForCleanup()
				createNamespace(preflight.BackupNamespacePrefix+uid, getPreflightResourceLabels(uid))

				cmdOut, err = runCleanupWithUID(uid)
				Expect(err).To(BeNil())

				assertSuccessCleanupAll(cmdOut.Out)
			})

			It("Should clean all preflight resources with the different uids spanning across different runs", func() {
				uid := createPreflightResourcesForCleanup()
				createNamespace(preflight.BackupNamespacePrefix+uid, getPreflightResourceLabels(uid))

				uid2, uid2Err := preflight.CreateResourceNameSuffix()
				Expect(uid2Err).To(BeNil())

				createNamespace(preflight.BackupNamespacePrefix+uid2, getPreflightResourceLabels(uid2))
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
