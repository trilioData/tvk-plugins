package cmd

import (
	"fmt"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Preflight cmd helper unit tests", func() {
	Context("Create logger and logging file", func() {
		It("Should create logging file and initialize logger", func() {
			terr := setupLogger(testLogFilePrefix, internal.DefaultLogLevel)
			Expect(terr).To(BeNil())
		})

		It("Should return err when invalid characters are provided as file prefix ", func() {
			terr := setupLogger(invalidPrefix, internal.DefaultLogLevel)
			Expect(terr).ToNot(BeNil())
		})

		It("Should not return error if invalid log-level is provided", func() {
			terr := setupLogger(testLogFilePrefix, internal.InvalidLogLevel)
			Expect(terr).To(BeNil())
		})
	})

	Context("Read input files", func() {
		It("Should read input file with run options", func() {
			terr := readFileInputOptions(filepath.Join(testDataDir, testInputFile))
			Expect(terr).To(BeNil())
		})

		It("Should return error when input format does not match with struct variable fields "+
			"or contains incorrect hierarchy of field values", func() {
			terr := readFileInputOptions(filepath.Join(testDataDir, invalidTestInputFile))
			Expect(terr).ToNot(BeNil())
		})
	})

	Context("Update resource requirements", func() {
		var once sync.Once
		BeforeEach(func() {
			once.Do(func() {
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
		})

		It("Should update resource requirements when provided in correct format", func() {
			podRequests = "cpu=300m,memory=96Mi"
			podLimits = "cpu=400m,memory=128Mi"
			terr := updateResReqFromCLI()
			Expect(terr).To(BeNil())
		})

		It("Should return error when resource requirements are provided in incorrect format", func() {
			podRequests = "cpu=300m,memory-64Mi"
			podLimits = "cpu,memory="
			terr := updateResReqFromCLI()
			Expect(terr).ToNot(BeNil())
		})
	})

	Context("Create resource list from comma separated <key>=<value> strings", func() {
		It("Should return list of resources with values when correct comma separated string is provided", func() {
			resourceStr := "network=5Mi,cpu=300m,disk=10Gi,memory=128Mi"
			resList, terr := populateResourceList(resourceStr)
			Expect(terr).To(BeNil())
			Expect(len(resList)).To(Equal(4))
		})

		It("Should return error when resource string is speficied in incorrect format", func() {
			resourceStr := "network=5Mi,cpu=,disk,memory=128Mi"
			_, terr := populateResourceList(resourceStr)
			Expect(terr).ToNot(BeNil())
		})

		It("Should return err when resource value does not satisfy parsing rules", func() {
			resourceStr := "cpu=40Hz,memory=64Ma"
			_, terr := populateResourceList(resourceStr)
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("quantities must match the regular expression" +
				" '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'"))
		})
	})

	Context("Parse node selector labels", func() {
		It("Should parse node selector labels string when provided in correct format", func() {
			nodeSelStr := "class=gold,disk=ssd"
			labels, terr := parseNodeSelectorLabels(nodeSelStr)
			Expect(terr).To(BeNil())
			Expect(len(labels)).To(Equal(2))
		})

		It("Should return error when node selector string is provided in an incorrect format", func() {
			nodeSelStr := "class=,disk"
			_, terr := parseNodeSelectorLabels(nodeSelStr)
			Expect(terr).ToNot(BeNil())
		})
	})

	Context("Validate run options struct variable", func() {
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
			cmdOps.Run.ImagePullSecret = "abcdef"
			cmdOps.Run.LocalRegistry = ""
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("cannot give image pull secret if local registry is not provided." +
				"\nUse --local-registry flag to provide local registry"))
		})
		It("Should return error when request memory is greater than limit memory", func() {
			cmdOps.Run.Requests["memory"] = resource.MustParse("256Mi")
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("request memory cannot be greater than limit memory"))
		})
		It("Should return error when request cpu is greater than limit cpu", func() {
			cmdOps.Run.Requests["cpu"] = resource.MustParse("700m")
			terr := validateRunOptions()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("request CPU cannot be greater than limit CPU"))
		})
	})

	Context("Validate cleanup options struct variable", func() {
		BeforeEach(func() {
			cmdOps.Cleanup = preflight.Cleanup{
				CleanupOptions: preflight.CleanupOptions{
					UID: "abcdef",
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
			cmdOps.Cleanup.CleanupOptions.UID = "abcd"
			terr := validateCleanupFields()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("valid 6-length preflight UID must be specified"))
		})
		It(fmt.Sprintf("Should return error when preflight uid length is greater than default uid length(%d)",
			preflightUIDLength), func() {
			cmdOps.Cleanup.CleanupOptions.UID = "abcdefgh"
			terr := validateCleanupFields()
			Expect(terr).ToNot(BeNil())
			Expect(terr.Error()).To(ContainSubstring("valid 6-length preflight UID must be specified"))
		})
	})
})
