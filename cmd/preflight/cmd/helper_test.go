package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

var _ = Describe("Preflight cmd helper unit tests", func() {

	Context("setupLogger func test-cases", func() {

		It("Should create logging file and initialize logger", func() {
			var fi os.FileInfo
			logFilename, terr := setupLogger(testLogFilePrefix, internal.DefaultLogLevel)
			Expect(terr).To(BeNil())

			// Check if file is created
			fi, terr = os.Stat(logFilename)
			Expect(terr).To(BeNil())
			Expect(fi.IsDir()).To(Equal(false))
			Expect(fi.Name()).To(Equal(logFilename))
		})

		It("Should return error when invalid characters are provided as file prefix ", func() {
			_, terr := setupLogger(invalidPrefix, internal.DefaultLogLevel)
			Expect(terr).ToNot(BeNil())
		})

		It("Should not return error if invalid log-level is provided and log level should be set to INFO", func() {
			_, terr := setupLogger(testLogFilePrefix, internal.InvalidLogLevel)
			Expect(terr).To(BeNil())
			Expect(logger.Level).To(Equal(log.InfoLevel))
		})
	})

	Context("readFileInputOptions func test-cases", func() {

		It("Should read inputs from file if data is present in correct yaml format", func() {
			terr := readFileInputOptions(filepath.Join(testDataDir, testInputFile))
			Expect(terr).To(BeNil())

			// run values
			Expect(cmdOps.Run.StorageClass).To(Equal("default"))
			Expect(cmdOps.Run.Namespace).To(Equal(internal.DefaultNs))
			Expect(cmdOps.Run.PerformCleanupOnFail).To(BeTrue())
			Expect(cmdOps.Run.PVCStorageRequest.String()).To(Equal("1Gi"))
			Expect(cmdOps.Run.Requests.Memory().String()).To(Equal("64Mi"))
			Expect(cmdOps.Run.Requests.Cpu().String()).To(Equal("250m"))
			Expect(cmdOps.Run.Limits.Memory().String()).To(Equal("128Mi"))
			Expect(cmdOps.Run.Limits.Cpu().String()).To(Equal("500m"))

			//cleanup values
			Expect(cmdOps.Cleanup.Namespace).To(Equal(internal.DefaultNs))
			Expect(cmdOps.Cleanup.UID).To(Equal("abcdef"))
			Expect(cmdOps.Cleanup.LogLevel).To(Equal("debug"))
		})

		It("Should return error when input data format does not match with struct variable fields "+
			"or contains incorrect hierarchy of field values", func() {
			terr := readFileInputOptions(filepath.Join(testDataDir, invalidTestInputFile))
			Expect(terr).ToNot(BeNil())
		})

		It("Should return error if file does not exist on given path", func() {
			var nonExistentFile = filepath.Join(testDataDir, "nonExistentFile.yaml")

			terr := readFileInputOptions(nonExistentFile)
			Expect(terr).ShouldNot(BeNil())
			Expect(terr.Error()).Should(Equal(fmt.Sprintf("open %s: no such file or directory", nonExistentFile)))
		})

		It("Should return error if file is not read permissible", func() {
			var (
				file             *os.File
				permYamlFilePath = filepath.Join(testDataDir, "file_permission.yaml")
			)

			By("Create non-read permission file")
			file, err = os.OpenFile(permYamlFilePath, os.O_CREATE, 0000)
			Expect(err).Should(BeNil())
			Expect(file.Close()).Should(BeNil())

			terr := readFileInputOptions(permYamlFilePath)
			Expect(terr).ShouldNot(BeNil())
			Expect(terr.Error()).Should(Equal(fmt.Sprintf("open %s: permission denied", permYamlFilePath)))

			By("Delete non-read permission file")
			Expect(os.Remove(permYamlFilePath)).Should(BeNil())
		})
	})

	Context("updateResReqFromCLI func test-cases", Ordered, func() {

		BeforeAll(func() {
			cmdOps = &preflightCmdOps{
				Run: preflight.Run{
					RunOptions: preflight.RunOptions{
						ResourceRequirements: corev1.ResourceRequirements{
							Limits:   map[corev1.ResourceName]resource.Quantity{},
							Requests: map[corev1.ResourceName]resource.Quantity{},
						},
					},
				},
			}
		})

		AfterAll(func() {
			podRequests = ""
			podLimits = ""
		})

		It("Should update resource requirements when provided in correct format", func() {
			podRequests = strings.Join([]string{
				fmt.Sprintf("%s=%s", corev1.ResourceCPU, internal.CPU300),
				fmt.Sprintf("%s=%s", corev1.ResourceMemory, internal.Memory128),
			}, ",")
			podLimits = strings.Join([]string{
				fmt.Sprintf("%s=%s", corev1.ResourceCPU, internal.CPU400),
				fmt.Sprintf("%s=%s", corev1.ResourceMemory, internal.Memory256),
			}, ",")
			terr := updateResReqFromCLI()
			Expect(terr).To(BeNil())

			Expect(cmdOps.Run.Requests.Cpu().String()).To(Equal(internal.CPU300))
			Expect(cmdOps.Run.Requests.Memory().String()).To(Equal(internal.Memory128))

			Expect(cmdOps.Run.Limits.Cpu().String()).To(Equal(internal.CPU400))
			Expect(cmdOps.Run.Limits.Memory().String()).To(Equal(internal.Memory256))
		})

		It("Should return error when resource requirements are provided in incorrect format", func() {
			podRequests = strings.Join([]string{
				fmt.Sprintf("%s=%s", corev1.ResourceCPU, internal.CPU300),
				fmt.Sprintf("%s-%s", corev1.ResourceMemory, internal.Memory256),
			}, ",")
			podRequests = strings.Join([]string{
				fmt.Sprintf("%s=%s", corev1.ResourceCPU, internal.CPU400),
				fmt.Sprintf("%s-%s", corev1.ResourceMemory, internal.Memory256),
			}, ",")
			terr := updateResReqFromCLI()
			Expect(terr).ToNot(BeNil())
		})
	})

	Context("populateResourceList func test-cases", func() {

		It("Should return list of resources with values when correct comma separated string is provided", func() {
			var (
				val resource.Quantity
				ok  bool
			)
			resourceStr := strings.Join([]string{
				fmt.Sprintf("%s=%s", corev1.ResourceCPU, internal.CPU300),
				fmt.Sprintf("%s=%s", nodeSelKeyDisk, internal.Memory256),
				fmt.Sprintf("%s=%s", corev1.ResourceMemory, internal.Memory256),
			}, ",")
			resList, terr := populateResourceList(resourceStr)
			Expect(terr).To(BeNil())
			Expect(len(resList)).To(Equal(3))

			val, ok = resList[corev1.ResourceCPU]
			Expect(ok).To(Equal(true))
			Expect(val.String()).To(Equal(internal.CPU300))

			val, ok = resList[nodeSelKeyDisk]
			Expect(ok).To(BeTrue())
			Expect(val.String()).To(Equal(internal.Memory256))

			val, ok = resList[corev1.ResourceMemory]
			Expect(ok).To(BeTrue())
			Expect(val.String()).To(Equal(internal.Memory256))
		})

		It("Should return error when resource string is specified in incorrect format", func() {
			resourceStr := strings.Join([]string{
				fmt.Sprintf("%s=", corev1.ResourceCPU),
				nodeSelKeyDisk,
				fmt.Sprintf("%s=%s", corev1.ResourceMemory, internal.Memory256),
			}, ",")
			_, terr := populateResourceList(resourceStr)
			Expect(terr).ToNot(BeNil())
		})

		It("Should return err when resource value does not satisfy parsing rules", func() {
			resourceStr := strings.Join([]string{fmt.Sprintf("%s=40Hz", corev1.ResourceCPU),
				fmt.Sprintf("%s=64Ma", corev1.ResourceMemory)},
				",")
			_, terr := populateResourceList(resourceStr)
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("quantities must match the regular expression" +
				" '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'"))
		})
	})

	Context("parseNodeSelectorLabels func test-cases", func() {

		It("Should parse node selector labels string when provided in correct format", func() {
			var (
				val string
				ok  bool
			)

			nodeSelStr := strings.Join([]string{fmt.Sprintf("%s=%s", nodeSelKeyClass, nodeSelValueGold),
				fmt.Sprintf("%s=%s", nodeSelKeyDisk, nodeSelValueSSD)},
				",")
			labels, terr := parseNodeSelectorLabels(nodeSelStr)
			Expect(terr).To(BeNil())
			Expect(len(labels)).To(Equal(2))

			val, ok = labels[nodeSelKeyClass]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal(nodeSelValueGold))

			val, ok = labels[nodeSelKeyDisk]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal(nodeSelValueSSD))
		})

		It("Should return error when node selector string is provided in an incorrect format", func() {
			nodeSelStr := strings.Join([]string{fmt.Sprintf("%s=", nodeSelKeyClass), nodeSelKeyDisk}, ",")
			_, terr := parseNodeSelectorLabels(nodeSelStr)
			Expect(terr).ToNot(BeNil())
		})
	})

	Context("managePreflightInputs func test-cases", func() {

		It("Should set namespace as default when namespace is not explicitly provided", func() {
			inputFileName = ""
			cmdOps.Run.Namespace = ""
			Expect(managePreflightInputs(&cobra.Command{})).Should(BeNil())
			Expect(cmdOps.Run.Namespace).To(Equal(internal.DefaultNs))
		})

		It("Should set default PVC size when pvc-storage request is not explicitly provided", func() {
			cmdOps.Run.PVCStorageRequest = resource.Quantity{}
			Expect(managePreflightInputs(&cobra.Command{})).Should(BeNil())
			Expect(cmdOps.Run.PVCStorageRequest).To(Equal(resource.MustParse(DefaultPVCStorage)))
		})

		It("Should set default pod resources requests and limits when explicitly not provided", func() {
			cmdOps.Run.Requests = map[corev1.ResourceName]resource.Quantity{}
			cmdOps.Run.Limits = map[corev1.ResourceName]resource.Quantity{}
			Expect(managePreflightInputs(&cobra.Command{})).Should(BeNil())

			Expect(cmdOps.Run.Requests.Cpu().String()).To(Equal(DefaultPodRequestCPU))
			Expect(cmdOps.Run.Requests.Memory().String()).To(Equal(DefaultPodRequestMemory))
			Expect(cmdOps.Run.Limits.Cpu().String()).To(Equal(DefaultPodLimitCPU))
			Expect(cmdOps.Run.Limits.Memory().String()).To(Equal(DefaultPodLimitMemory))
		})

		It("Should set logLevel as INFO when log level is not explicitly provided", func() {
			cmdOps.Run.LogLevel = ""
			Expect(managePreflightInputs(&cobra.Command{})).Should(BeNil())
			Expect(cmdOps.Run.LogLevel).Should(Equal(internal.DefaultLogLevel))
		})

		It("Should perform preflight check using kubeconfig file mentioned in KUBECONFIG env when no explicit"+
			" kubeconfig file is provided", func() {
			testConfigPath := filepath.Join(projectRoot, "manual", "path", "to", "kubeconfig")
			cmdOps.Run.Kubeconfig = ""
			kubeconfig = testConfigPath
			Expect(managePreflightInputs(&cobra.Command{})).Should(BeNil())
			Expect(cmdOps.Run.Kubeconfig).Should(Equal(testConfigPath))
		})

	})

	Context("validateRunOptions func test-cases", func() {
		BeforeEach(func() {
			cmdOps.Run = preflight.Run{
				RunOptions: preflight.RunOptions{
					StorageClass:      internal.DefaultTestStorageClass,
					SnapshotClass:     internal.DefaultTestSnapshotClass,
					PVCStorageRequest: resource.Quantity{},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			}
		})

		It("Should validate run options and return no error when run options are correct", func() {
			terr := validateRunOptions()
			Expect(terr).To(BeNil())
		})

		It("Should return error when storage class is empty", func() {
			cmdOps.Run.StorageClass = ""
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("storage-class is required, cannot be empty"))
		})

		It("Should return error when image pull secret is provided and local registry path is empty", func() {
			cmdOps.Run.ImagePullSecret = imagePullSecretStr
			cmdOps.Run.LocalRegistry = ""
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("cannot give image pull secret if local registry is not provided." +
				"\nUse --local-registry flag to provide local registry"))
		})

		It("Should return error when request memory is greater than limit memory", func() {
			cmdOps.Run.Requests["memory"] = resource.MustParse(internal.Memory256)
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("request memory cannot be greater than limit memory"))
		})

		It("Should return error when request cpu is greater than limit cpu", func() {
			cmdOps.Run.Requests[corev1.ResourceCPU] = resource.MustParse(internal.CPU600)
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("request CPU cannot be greater than limit CPU"))
		})
	})

	Context("validateCleanupFields func test-cases", func() {
		BeforeEach(func() {
			cmdOps.Cleanup = preflight.Cleanup{
				CleanupOptions: preflight.CleanupOptions{
					UID: validPreflightUID,
				},
			}
		})

		It("Should validate cleanup options and return no error when cleanup options are correct", func() {
			terr := validateCleanupFields()
			Expect(terr).To(BeNil())
		})

		It("Should not return error when preflight uid is empty", func() {
			cmdOps.Cleanup.CleanupOptions.UID = ""
			terr := validateCleanupFields()
			Expect(terr).To(BeNil())
		})

		It(fmt.Sprintf("Should return error when preflight uid length is less than default uid length(%d)",
			preflightUIDLength), func() {
			cmdOps.Cleanup.CleanupOptions.UID = invalidShortPreflightUID
			terr := validateCleanupFields()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("valid 6-length preflight UID must be specified"))
		})

		It(fmt.Sprintf("Should return error when preflight uid length is greater than default uid length(%d)",
			preflightUIDLength), func() {
			cmdOps.Cleanup.CleanupOptions.UID = invalidLongPreflightUID
			terr := validateCleanupFields()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("valid 6-length preflight UID must be specified"))
		})
	})

	Context("manageCleanupInputs func test-cases", func() {

		It("Should set namespace as default when namespace is not explicitly provided", func() {
			inputFileName = ""
			cmdOps.Cleanup.Namespace = ""
			Expect(manageCleanupInputs(&cobra.Command{})).Should(BeNil())
			Expect(cmdOps.Cleanup.Namespace).To(Equal(internal.DefaultNs))
		})
	})

})
