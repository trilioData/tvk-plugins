package targetbrowsertest

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	guid "github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/cmd/target-browser/cmd"
	"github.com/trilioData/tvk-plugins/internal"
	targetbrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

const (
	backupPlanType            = "backupPlanType"
	name                      = "name"
	successfulBackups         = "successfulBackups"
	backupTimestamp           = "backupTimestamp"
	status                    = "status"
	cmdBackupPlan             = cmd.BackupPlanCmdName
	cmdBackup                 = cmd.BackupCmdName
	cmdMetadata               = cmd.MetadataCmdName
	cmdResourceMetadata       = cmd.ResourceMetadataCmdName
	cmdTrilioResources        = cmd.TrilioResourcesCmdName
	cmdGet                    = "get"
	flagPrefix                = "--"
	hyphen                    = "-"
	flagOrderBy               = flagPrefix + cmd.OrderByFlag
	flagTvkInstanceUIDFlag    = flagPrefix + cmd.TvkInstanceUIDFlag
	flagTvkInstanceNameFlag   = flagPrefix + cmd.TvkInstanceNameFlag
	flagBackupUIDFlag         = flagPrefix + cmd.BackupUIDFlag
	flagBackupStatus          = flagPrefix + cmd.BackupStatusFlag
	flagBackupPlanUIDFlag     = flagPrefix + cmd.BackupPlanUIDFlag
	flagPageSize              = flagPrefix + cmd.PageSizeFlag
	flagTargetNamespace       = flagPrefix + cmd.TargetNamespaceFlag
	flagTargetName            = flagPrefix + cmd.TargetNameFlag
	flagKubeConfig            = flagPrefix + cmd.KubeConfigFlag
	flagCaCert                = flagPrefix + cmd.CertificateAuthorityFlag
	flagInsecureSkip          = flagPrefix + cmd.InsecureSkipTLSFlag
	flagUseHTTPS              = flagPrefix + cmd.UseHTTPS
	flagOutputFormat          = flagPrefix + cmd.OutputFormatFlag
	flagCreationStartTime     = flagPrefix + cmd.CreationStartTimeFlag
	flagCreationEndTime       = flagPrefix + cmd.CreationEndTimeFlag
	flagExpirationStartTime   = flagPrefix + cmd.ExpirationStarTimeFlag
	flagExpirationEndTime     = flagPrefix + cmd.ExpirationEndTimeFlag
	flagOperationScope        = flagPrefix + cmd.OperationScopeFlag
	flagKind                  = flagPrefix + cmd.KindFlag
	flagKinds                 = flagPrefix + cmd.KindsFlag
	flagName                  = flagPrefix + cmd.NameFlag
	flagVersion               = flagPrefix + cmd.VersionFlag
	flagGroup                 = flagPrefix + cmd.GroupFlag
	kubeConf                  = ""
	pageSize                  = 1
	backupCreationStartDate   = "2021-05-16"
	backupCreationEndDate     = "2021-05-18"
	backupExpirationStartDate = "2021-09-13"
	backupExpirationEndDate   = "2021-09-14"
	startTime                 = "00:00:00"
	backupMetadataHelm        = "backup-metadata-helm.json"
)

var (
	backupStatus = []string{"Available", "Failed", "InProgress"}
	commonArgs   = []string{flagTargetName, TargetName, flagTargetNamespace,
		installNs, flagKubeConfig, kubeConf}
)

var _ = Describe("Target Browser Tests", func() {

	Context("Global Flag's test-cases", func() {

		Context("test-cases without target creation", func() {

			It(fmt.Sprintf("Should consider default value if flag %s not given", flagKubeConfig), func() {
				args := []string{cmdGet, cmdBackupPlan}
				testArgs := []string{flagTargetName, TargetName, flagTargetNamespace, installNs}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(".kube/config: no such file or directory"))
			})

			It(fmt.Sprintf("Should consider KUBECONFIG env variable's value if flag %s given with zero value", flagKubeConfig), func() {
				args := []string{cmdGet, cmdBackupPlan}
				testArgs := []string{flagKubeConfig, "", flagTargetName, TargetName, flagTargetNamespace, installNs}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("targets.triliovault.trilio.io \"%s\" not found", TargetName)))
			})

			It(fmt.Sprintf("Should fail if flag %s not given", flagTargetName), func() {
				args := []string{cmdGet, cmdBackupPlan}
				testArgs := []string{flagTargetNamespace, installNs, flagKubeConfig, kubeConf}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.TargetNameFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given with zero value", flagTargetName), func() {
				args := []string{cmdGet, cmdBackupPlan}
				testArgs := []string{flagTargetName, "", flagTargetNamespace, installNs, flagKubeConfig, kubeConf}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.TargetNameFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s & %s both are given", flagCaCert, flagInsecureSkip), func() {
				args := []string{cmdGet, cmdBackupPlan, flagInsecureSkip, strconv.FormatBool(true), flagCaCert, "ca-cert-file-path"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag cannot be provided if [%s] is provided",
					cmd.InsecureSkipTLSFlag, cmd.CertificateAuthorityFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given with zero value", flagCaCert), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCaCert, ""}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.CertificateAuthorityFlag)))
			})
		})

		Context("test-cases with target 'browsingEnabled=false'", Ordered, func() {

			BeforeAll(func() {
				createTarget(false)

			})

			AfterAll(func() {
				deleteTarget(true)
			})

			It(fmt.Sprintf("Should fail if flag %s is given with incorrect value", flagTargetName), func() {
				incorrectTarget := "incorrect-target-name"
				args := []string{cmdGet, cmdBackupPlan, flagKubeConfig, kubeConf}
				testArgs := []string{flagTargetName, incorrectTarget, flagTargetNamespace, installNs}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("targets.triliovault.trilio.io \"%s\" not found", incorrectTarget)))
			})

			It("Should fail if target CR status does not have 'browsingEnabled=true'", func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				cmd := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := cmd.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("browsing is not enabled for "+
					"given target %s namespace %s", TargetName, installNs)))
			})

			It(fmt.Sprintf("Should consider default value if flag %s is not given", flagTargetNamespace), func() {
				args := []string{cmdGet, cmdBackupPlan, flagKubeConfig, kubeConf}
				testArgs := []string{flagTargetName, TargetName}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				_, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				//Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("targets.triliovault.trilio.io \"%s\" not found", TargetName)))
			})
		})

		Context("test-cases with target 'browsingEnabled=true'", func() {
			var ing *v1beta1.Ingress
			BeforeEach(func() {
				createTarget(true)
				// update target browser's ingress OwnerReferences to nil so that it won't be identified as target browser's ingress
				ing = getTargetBrowserIngress()
				Expect(ing).ToNot(BeNil())
				ing.SetOwnerReferences([]metav1.OwnerReference{})
				UpdateIngress(ctx, k8sClient, ing)
			})

			AfterEach(func() {
				deleteTarget(false)
				if ing != nil {
					Expect(k8sClient.Delete(ctx, ing, &client.DeleteOptions{})).To(BeNil())
					log.Infof("deleted ingress %s namespace %s", ing.Name, installNs)
				}
			})

			It("Should fail if target browser's ingress resource not found", func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("targetBrowserPath could not"+
					" retrieved for target %s namespace %s", TargetName, installNs)))
			})

		})

		Context("test-cases with target 'browsingEnabled=true' && TVK host is serving HTTPS", Ordered, func() {
			BeforeAll(func() {
				switchTvkHostFromHTTPToHTTPS()
				time.Sleep(time.Second * 10)
				createTarget(true)
			})

			AfterAll(func() {
				deleteTarget(false)
				switchTvkHostFromHTTPSToHTTP()
				time.Sleep(time.Second * 10)
			})

			It(fmt.Sprintf("Should pass if flag %s & %s both are provided", cmd.UseHTTPS, cmd.CertificateAuthorityFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagUseHTTPS, flagCaCert, filepath.Join(testDataDirRelPath, tlsCertFile)}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				_, err := command.CombinedOutput()
				Expect(err).ShouldNot(HaveOccurred())
			})

			It(fmt.Sprintf("Should fail if flag %s is not provided", cmd.UseHTTPS), func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				_, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
			})

			It(fmt.Sprintf("Should fail if flag %s is provided but %s is not provided", cmd.UseHTTPS, cmd.CertificateAuthorityFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagUseHTTPS}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("certificate signed by unknown authority"))
			})

			It(fmt.Sprintf("Should fail if flag %s is provided but %s is not provided", cmd.CertificateAuthorityFlag, cmd.UseHTTPS), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCaCert, filepath.Join(testDataDirRelPath, tlsCertFile)}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				_, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
			})
		})

		Context(fmt.Sprintf("Backup & BackupPlan test-cases for %s flag", cmd.OutputFormatFlag), Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 2
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID, tvkUID                string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				createTarget(true)
				backupUID = guid.New().String()
				tvkUID = guid.New().String()
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "mutate-tvk-id", tvkUID, "tvk-instance")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {

				deleteTarget(false)
				removeBackupPlanDir()

			})
			It(fmt.Sprintf("Should fail cmd BackupPlan if flag %s is given without value", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagTargetName, TargetName, flagTargetNamespace,
					installNs, flagKubeConfig, kubeConf, flagOutputFormat}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagOutputFormat)))
			})

			It(fmt.Sprintf("Should fail cmd backupPlan if flag %s is given and value is invalid", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagTargetName, TargetName, flagTargetNamespace,
					installNs, flagOutputFormat, "invalid", flagKubeConfig, kubeConf}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.OutputFormatFlag)))
			})

			It(fmt.Sprintf("Should succeed cmd backupPlan if flag %s is given and value is %s", cmd.OutputFormatFlag, internal.FormatJSON), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfBackupPlansToCreate))
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Kind).To(Equal(internal.BackupPlanKind))
					Expect(backupPlanData[index].TvkInstanceID).To(Equal(tvkUID))
				}
			})

			It(fmt.Sprintf("Should get one backupPlan for specific backupPlan UID if flag %s is given and value is %s",
				cmd.OutputFormatFlag, internal.FormatJSON), func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[1], flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(1))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[1]))
			})

			It(fmt.Sprintf("Should succeed cmd backupPlan if flag %s is given and value is %s", cmd.OutputFormatFlag, internal.FormatYAML), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatYAML}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfBackupPlansToCreate))
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Kind).To(Equal(internal.BackupPlanKind))
					Expect(backupPlanData[index].TvkInstanceID).To(Equal(tvkUID))
				}
			})

			It(fmt.Sprintf("Should get one backupPlan for specific backupPlan UID if flag %s is given and value is %s",
				cmd.OutputFormatFlag, internal.FormatYAML), func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[1], flagOutputFormat, internal.FormatYAML}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(1))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[1]))
			})

			It(fmt.Sprintf("Should succeed cmd backupPlan if flag %s is not provided", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackupPlan}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfBackupPlansToCreate))
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Kind).To(Equal(internal.BackupPlanKind))
					Expect(backupPlanData[index].TvkInstanceID).To(Equal(tvkUID))
				}
			})

			It(fmt.Sprintf("Should get one backupPlan for specific backupPlan UID if flag %s is not provided",
				cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[0]}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(1))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[0]))
			})

			It(fmt.Sprintf("Should succeed cmd backupPlan if flag %s is given and value is %s", cmd.OutputFormatFlag, internal.FormatWIDE), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatWIDE}
				outputWide := exeCommand(args, cmdBackupPlan)
				args = []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
				outputJSON := exeCommand(args, cmdBackupPlan)
				Expect(reflect.DeepEqual(outputWide, outputJSON))
			})

			It(fmt.Sprintf("Should get one backupPlan for specific backupPlan UID if flag %s is given and value is %s",
				cmd.OutputFormatFlag, internal.FormatWIDE), func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[0], flagOutputFormat, internal.FormatWIDE}
				outputWide := exeCommand(args, cmdBackupPlan)
				args = []string{cmdGet, cmdBackupPlan, backupPlanUIDs[0], flagOutputFormat, internal.FormatJSON}
				outputJSON := exeCommand(args, cmdBackupPlan)
				Expect(reflect.DeepEqual(outputWide, outputJSON))
			})

			It(fmt.Sprintf("Should fail cmd Backup if flag %s is given without value", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackup, flagTargetName, TargetName, flagTargetNamespace,
					installNs, flagKubeConfig, kubeConf, flagOutputFormat}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagOutputFormat)))
			})

			It(fmt.Sprintf("Should fail cmd backup if flag %s is given and value is invalid", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackup, flagTargetName, TargetName, flagTargetNamespace,
					installNs, flagOutputFormat, "invalid", flagKubeConfig, kubeConf}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.OutputFormatFlag)))
			})

			It(fmt.Sprintf("Should succeed cmd backup if flag %s is given and value is %s", cmd.OutputFormatFlag, internal.FormatJSON), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(noOfBackupPlansToCreate * noOfBackupsToCreatePerBackupPlan))
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Kind).To(Equal(internal.BackupKind))
					Expect(backupData[index].UID).To(Equal(backupUID))
				}
			})

			It(fmt.Sprintf("Should get one backup for specific backup UID if flag %s is given and value is %s",
				cmd.OutputFormatFlag, internal.FormatJSON), func() {
				args := []string{cmdGet, cmdBackup, backupUID, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should succeed cmd backup if flag %s is given and value is %s", cmd.OutputFormatFlag, internal.FormatYAML), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatYAML}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(noOfBackupPlansToCreate * noOfBackupsToCreatePerBackupPlan))
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Kind).To(Equal(internal.BackupKind))
					Expect(backupData[index].UID).To(Equal(backupUID))
				}
			})
			It(fmt.Sprintf("Should get one backup for specific backup UID if flag %s is given and value is %s",
				cmd.OutputFormatFlag, internal.FormatYAML), func() {
				args := []string{cmdGet, cmdBackup, backupUID, flagOutputFormat, internal.FormatYAML}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should get one backup for specific backup UID if flag %s is not privded",
				cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackup, backupUID}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should succeed cmd backup if flag %s is not provided", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdBackup}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(noOfBackupPlansToCreate * noOfBackupsToCreatePerBackupPlan))
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Kind).To(Equal(internal.BackupKind))
					Expect(backupData[index].UID).To(Equal(backupUID))
				}
			})

			It(fmt.Sprintf("Should get one backup for specific backup UID if flag %s is given and value is %s",
				cmd.OutputFormatFlag, internal.FormatWIDE), func() {
				args := []string{cmdGet, cmdBackup, backupUID, flagOutputFormat, internal.FormatWIDE}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
				Expect(backupData[0].TvkInstanceID).To(Equal(tvkUID))
			})

			It(fmt.Sprintf("Should succeed cmd backup if flag %s is given and value is %s", cmd.OutputFormatFlag, internal.FormatWIDE), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatWIDE}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(noOfBackupPlansToCreate * noOfBackupsToCreatePerBackupPlan))
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Kind).To(Equal(internal.BackupKind))
					Expect(backupData[index].UID).To(Equal(backupUID))
					Expect(backupData[0].TvkInstanceID).To(Equal(tvkUID))
				}
			})
		})
	})

	Context("BackupPlan command test-cases", func() {
		Context(fmt.Sprintf("test-cases for sorting on columns %s, %s, %s, %s",
			backupPlanType, name, successfulBackups, backupTimestamp), Ordered, func() {
			var (
				noOfBackupPlansToCreate          = 4
				noOfBackupsToCreatePerBackupPlan = 1

				backupUID string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup")
				_ = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {
				deleteTarget(false)
				removeBackupPlanDir()
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", backupPlanType), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, backupPlanType, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Type <= backupPlanData[index+1].Type).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", backupPlanType), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + backupPlanType, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Type >= backupPlanData[index+1].Type).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, name, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + name, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name >= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", successfulBackups), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, successfulBackups, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].SuccessfulBackup <= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", successfulBackups), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + successfulBackups, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].SuccessfulBackup >= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", backupTimestamp), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, backupTimestamp, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					if backupPlanData[index].SuccessfulBackup != 0 && backupPlanData[index+1].SuccessfulBackup != 0 {
						bkpTS1, _ := time.Parse(time.RFC3339, backupPlanData[index].SuccessfulBackupTimestamp)
						bkpTS2, _ := time.Parse(time.RFC3339, backupPlanData[index+1].SuccessfulBackupTimestamp)
						if !bkpTS1.Equal(bkpTS2) {
							Expect(bkpTS1.Before(bkpTS2)).Should(BeTrue())
						}
					}
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", backupTimestamp), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + backupTimestamp, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					if backupPlanData[index].SuccessfulBackup != 0 && backupPlanData[index+1].SuccessfulBackup != 0 {
						bkpTS1, _ := time.Parse(time.RFC3339, backupPlanData[index].SuccessfulBackupTimestamp)
						bkpTS2, _ := time.Parse(time.RFC3339, backupPlanData[index+1].SuccessfulBackupTimestamp)
						if !bkpTS1.Equal(bkpTS2) {
							Expect(bkpTS1.After(bkpTS2)).Should(BeTrue())
						}
					}
				}
			})
		})

		Context(fmt.Sprintf("test-cases for flag %s, %s, %s, %s with path value, zero, valid  and invalid value",
			cmd.OrderByFlag, cmd.PageSizeFlag, cmd.CreationStartTimeFlag, cmd.CreationEndTimeFlag), Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 11
				noOfBackupsToCreatePerBackupPlan = 2

				backupUID string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {

				deleteTarget(false)
				removeBackupPlanDir()

			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				args = append(args, flagOrderBy)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagOrderBy)))
			})

			It(fmt.Sprintf("Should sort in ascending order if flag %s is not provided", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should succeed if flag %s is given zero value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				_, err := runCommand(args, cmdBackupPlan)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "invalid"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackupPlan)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})

			It(fmt.Sprintf("Should get one page backupPlan using flag %s", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, strconv.Itoa(pageSize), flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(pageSize))
			})

			It(fmt.Sprintf("Should get one page backupPlan if flag %s is not provided", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should get one page backupPlan if flag %s is given zero value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, "0", flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, "invalid", flagOutputFormat, internal.FormatJSON}
				output, err := runCommand(args, cmdBackupPlan)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf(" invalid argument \"invalid\" for \"%s\"", flagPageSize)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given negative value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, "-1", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackupPlan)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})

			It("Should get one backupPlan for specific backupPlan UID", func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[1], flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(1))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[1]))
			})

			It("Should get two backupPlan for backupPlan UIDs", func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[0], backupPlanUIDs[1], flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(2))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[0]))
				Expect(backupPlanData[1].UID).To(Equal(backupPlanUIDs[1]))
			})

			It("Should fail using backupPlanUID 'invalidUID'", func() {
				args := []string{cmdGet, cmdBackupPlan, "invalidUID"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackupPlan)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("404 Not Found"))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				args = append(args, flagCreationStartTime)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagCreationStartTime)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given zero value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, ""}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.CreationStartTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.CreationStartTimeFlag)))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given valid format timestamp value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, fmt.Sprintf("%sT%sZ", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given valid format date value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, backupCreationEndDate, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'T' value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, fmt.Sprintf("%sT%s", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'Z' value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, fmt.Sprintf("%s %sZ", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'T' & 'Z' value",
				flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, fmt.Sprintf("%s %s", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				args = append(args, flagCreationEndTime)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagCreationEndTime)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationEndTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.CreationStartTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is given 'invalid' value",
				flagCreationStartTime, flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, backupCreationStartDate, flagCreationEndTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.CreationEndTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is given 'invalid' value",
				flagCreationEndTime, flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationEndTime, backupCreationStartDate, flagCreationStartTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.CreationStartTimeFlag)))
			})
			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is not provided",
				flagCreationEndTime, flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationEndTime, backupCreationStartDate}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.CreationStartTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s &  %s are given valid format and equal Timestamp",
				flagCreationStartTime, flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, backupCreationStartDate,
					flagCreationEndTime, backupCreationStartDate}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] and [%s] flag values %sT00:00:00Z and "+
					"%sT00:00:00Z can't be same", cmd.CreationStartTimeFlag, cmd.CreationEndTimeFlag, backupCreationStartDate, backupCreationStartDate)))
			})
			It(fmt.Sprintf("Should fail if flag %s &  %s are given valid format and flagCreationStartTime > CreationEndTimestamp",
				flagCreationStartTime, flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackupPlan, flagCreationStartTime, backupCreationEndDate,
					flagCreationEndTime, backupCreationStartDate}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackupPlan)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})
		})

		Context(fmt.Sprintf("test-cases for flag %s with zero, valid  and invalid value", cmd.OperationScopeFlag), Ordered, func() {
			var (
				backupPlanUIDs, clusterBackupPlanUIDs          []string
				noOfBackupPlansToCreate                        = 3
				noOfBackupsToCreatePerBackupPlan               = 1
				noOfClusterBackupPlansToCreate                 = 3
				noOfClusterBackupsToCreatePerClusterBackupPlan = 1
				backupUID, clusterBackupUID                    string
				clusterBackupPlanUIDSet, backupPlanUIDSet      sets.String
			)
			BeforeAll(func() {

				removeBackupPlanDir()
				backupUID = guid.New().String()
				clusterBackupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfClusterBackupPlansToCreate, noOfClusterBackupsToCreatePerClusterBackupPlan, clusterBackupUID, "cluster_backup")
				clusterBackupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfClusterBackupPlansToCreate,
					noOfClusterBackupPlansToCreate*noOfClusterBackupsToCreatePerClusterBackupPlan)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate+noOfClusterBackupPlansToCreate,
					noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan+
						noOfClusterBackupsToCreatePerClusterBackupPlan*noOfClusterBackupPlansToCreate)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate + noOfClusterBackupPlansToCreate)).Should(BeTrue())
				clusterBackupPlanUIDSet = sets.NewString(clusterBackupPlanUIDs...)
				backupPlanUIDSet = sets.NewString(backupPlanUIDs...).Difference(clusterBackupPlanUIDSet)

			})

			AfterAll(func() {

				deleteTarget(false)
				removeBackupPlanDir()

			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagOperationScope), func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				args = append(args, flagOperationScope)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagOperationScope)))
			})

			It(fmt.Sprintf("Should get both OperationScoped backupPlan and clusterBackupPlans if flag %s is not provided",
				flagOperationScope), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfBackupPlansToCreate + noOfClusterBackupPlansToCreate))
				validateBackupPlanKind(backupPlanData, backupPlanUIDSet, clusterBackupPlanUIDSet)
			})

			It(fmt.Sprintf("Should get both backupPlans and clusterbackupPlans if flag %s is given zero value", flagOperationScope), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOperationScope, "", flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfBackupPlansToCreate + noOfClusterBackupPlansToCreate))
				validateBackupPlanKind(backupPlanData, backupPlanUIDSet, clusterBackupPlanUIDSet)
			})

			It(fmt.Sprintf("Should get %d backupPlan if flag %s is given %s value", noOfClusterBackupPlansToCreate,
				flagOperationScope, internal.MultiNamespace), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOperationScope, internal.MultiNamespace, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfClusterBackupPlansToCreate))
				for idx := range backupPlanData {
					Expect(backupPlanData[idx].Kind).To(Equal(internal.ClusterBackupPlanKind))
				}
			})

			It(fmt.Sprintf("Should get %d backupPlan if flag %s is given %s value", noOfBackupPlansToCreate,
				flagOperationScope, internal.SingleNamespace), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOperationScope, internal.SingleNamespace, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(noOfBackupPlansToCreate))
				for idx := range backupPlanData {
					Expect(backupPlanData[idx].Kind).To(Equal(internal.BackupPlanKind))
				}
			})
		})

		Context("Filtering BackupPlans based on TVK Instance ID and Name", Ordered, func() {
			var (
				backupUID           string
				tvkInstanceIDValues map[string]string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				createTarget(true)
				// Generating backupPlans and backups with different TVK instance UID
				tvkInstanceIDValues = map[string]string{
					guid.New().String(): "tvk-instance-name1",
					guid.New().String(): "tvk-instance-name2",
				}
				backupUID = guid.New().String()

				for tvkUID, tvkName := range tvkInstanceIDValues {
					createBackups(1, 1, backupUID, "mutate-tvk-id", tvkUID, tvkName)
				}
				_ = verifyBackupPlansAndBackupsOnNFS(2, 2)
				Expect(verifyBrowserCacheBPlan(2)).Should(BeTrue())

			})
			AfterAll(func() {

				deleteTarget(false)
				removeBackupPlanDir()

			})

			It(fmt.Sprintf("Should filter backupPlans on flag %s", cmd.TvkInstanceUIDFlag), func() {
				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, tvkUID, flagOutputFormat, internal.FormatJSON}
					backupPlanData := runCmdBackupPlan(args)
					Expect(len(backupPlanData)).To(Equal(1))
					Expect(backupPlanData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupPlanData[0].TvkInstanceName).Should(Equal(tvkName))
				}
			})

			It(fmt.Sprintf("Should get zero backupPlans using flag %s with 'invalid' value", cmd.TvkInstanceUIDFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, "invalid", flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(0))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagTvkInstanceUIDFlag), func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				args = append(args, flagTvkInstanceUIDFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagTvkInstanceUIDFlag)))
			})

			It(fmt.Sprintf("Should filter backupPlans on flag %s and order by backupPlan name ascending order", cmd.TvkInstanceUIDFlag), func() {
				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, tvkUID, flagOrderBy, name, flagOutputFormat, internal.FormatJSON}
					backupPlanData := runCmdBackupPlan(args)
					Expect(len(backupPlanData)).To(Equal(1))
					Expect(backupPlanData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupPlanData[0].TvkInstanceName).Should(Equal(tvkName))
					for index := 0; index < len(backupPlanData)-1; index++ {
						Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
					}
				}
			})
			It(fmt.Sprintf("Should filter backupPlans on flag %s", cmd.TvkInstanceNameFlag), func() {
				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceNameFlag, tvkName, flagOutputFormat, internal.FormatJSON}
					backupPlanData := runCmdBackupPlan(args)
					Expect(len(backupPlanData)).To(Equal(1))
					Expect(backupPlanData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupPlanData[0].TvkInstanceName).Should(Equal(tvkName))
				}
			})

			It(fmt.Sprintf("Should get zero backupPlans using flag %s with 'invalid' value", cmd.TvkInstanceNameFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceNameFlag, "invalid", flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(0))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagTvkInstanceNameFlag), func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				args = append(args, flagTvkInstanceNameFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagTvkInstanceNameFlag)))
			})

			It(fmt.Sprintf("Should filter backupPlans on flag %s and order by backupPlan name ascending order", cmd.TvkInstanceNameFlag), func() {

				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceNameFlag, tvkName, flagOrderBy, name, flagOutputFormat, internal.FormatJSON}
					backupPlanData := runCmdBackupPlan(args)
					Expect(len(backupPlanData)).To(Equal(1))
					Expect(backupPlanData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupPlanData[0].TvkInstanceName).Should(Equal(tvkName))
					for index := 0; index < len(backupPlanData)-1; index++ {
						Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
					}
				}
			})
		})
	})

	Context("Backup command test-cases", func() {
		Context(fmt.Sprintf("test-cases for filtering and sorting operations on %s, %s columns of Backup", status, name), Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 4
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID                        string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {
				deleteTarget(false)
				removeBackupPlanDir()

			})
			for _, bkpStatus := range backupStatus {
				bkpStatus := bkpStatus
				It(fmt.Sprintf("Should filter backup with backup status '%s'", bkpStatus), func() {
					args := []string{cmdGet, cmdBackup, flagBackupStatus, bkpStatus, flagOutputFormat, internal.FormatJSON}
					backupData := runCmdBackup(args)
					// compare backups status with status value passed as arg
					for index := 0; index < len(backupData); index++ {
						Expect(backupData[index].Status).Should(Equal(bkpStatus))
					}
				})
			}

			It(fmt.Sprintf("Should sort in ascending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackup, flagOrderBy, name, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, hyphen + name,
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name >= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", status), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, status,
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Status <= backupData[index+1].Status).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", status), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, hyphen + status,
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Status <= backupData[index+1].Status).Should(BeTrue())
				}
			})
		})

		Context(fmt.Sprintf("test-cases for flag %s, %s, %s, %s, %s, %s, %s, %s with path value, zero, valid  and invalid value",
			cmd.OrderByFlag, cmd.PageSizeFlag, cmd.BackupPlanUIDFlag, cmd.BackupStatusFlag, cmd.CreationStartTimeFlag,
			cmd.CreationEndTimeFlag, cmd.ExpirationStarTimeFlag, cmd.ExpirationEndTimeFlag), Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 11
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID                        string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {
				deleteTarget(false)
				removeBackupPlanDir()
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.BackupStatusFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[1], flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				args = append(args, flagBackupStatus)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagBackupStatus)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", cmd.BackupStatusFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupStatus, "invalid"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given zero value", cmd.BackupStatusFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupStatus, "", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				_, err := runCommand(args, cmdBackup)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It(fmt.Sprintf("Should get one backup for specific backup UID %s", backupUID), func() {
				args := []string{cmdGet, cmdBackup, backupUID, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should get one backup for 2 duplicate backup UID %s", backupUID), func() {
				args := []string{cmdGet, cmdBackup, backupUID, backupUID, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It("Should get zero backup using backupPlanUID with 'invalid' value", func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, "invalid", flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).Should(Equal(0))
			})

			It(fmt.Sprintf("Should get %d backup using flag %s with zero value", cmd.PageSizeDefault, cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, "", flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).Should(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should get one page backup using flag %s with value %d with flag %s", cmd.PageSizeFlag,
				pageSize, cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[1], flagPageSize, strconv.Itoa(pageSize),
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(pageSize))
			})
			It(fmt.Sprintf("Should get one page backup if flag %s is not provided", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given zero value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup, flagPageSize, "0", flagOutputFormat, internal.FormatJSON}
				_, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
			})

			It(fmt.Sprintf("Should fail if flag %s is given negative value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup, flagPageSize, "-1", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup, flagPageSize, "invalid", flagOutputFormat, internal.FormatJSON}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf(" invalid argument \"invalid\" for \"%s\"", flagPageSize)))
			})

			It(fmt.Sprintf("Should get one page backup using flag %s value %d without flag %s", cmd.PageSizeFlag,
				pageSize, cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, backupUID, flagPageSize, strconv.Itoa(pageSize), flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(pageSize))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should sort in ascending order name if flag %s is not provided", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should succeed if flag %s is given zero value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackup, flagOrderBy, "", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				_, err := runCommand(args, cmdBackup)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackup, flagOrderBy, "invalid"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})

			It("Should fail if path param is given 'invalidUID' value", func() {
				args := []string{cmdGet, cmdBackup, "invalidUID"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("404 Not Found"))
			})
			It(fmt.Sprintf("Should fail if flag %s is given without value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup}
				args = append(args, commonArgs...)
				args = append(args, flagCreationStartTime)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagCreationStartTime)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given 'invalid' value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.CreationStartTimeFlag)))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given valid format timestamp value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, fmt.Sprintf("%sT%sZ", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given valid format date value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, backupCreationEndDate, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'T' value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, fmt.Sprintf("%s %sZ", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'Z' value", flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, fmt.Sprintf("%sT%s", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'T' & 'Z' value",
				flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, fmt.Sprintf("%s %s", backupCreationEndDate, startTime),
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				args = append(args, flagCreationEndTime)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagCreationEndTime)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is given 'invalid' value",
				flagCreationStartTime, flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, backupCreationStartDate, flagCreationEndTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.CreationEndTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is given 'invalid' value",
				flagCreationEndTime, flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationEndTime, backupCreationStartDate, flagCreationStartTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.CreationStartTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s &  %s are given valid format and flagCreationStartTime > CreationEndTimestamp",
				flagCreationStartTime, flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, backupCreationEndDate,
					flagCreationEndTime, backupCreationStartDate}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})
			It(fmt.Sprintf("Should fail if flag %s &  %s are given valid format and equal Timestamp",
				flagCreationStartTime, flagCreationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationStartTime, backupCreationStartDate,
					flagCreationEndTime, backupCreationStartDate}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] and [%s] flag values %sT00:00:00Z and "+
					"%sT00:00:00Z can't be same", cmd.CreationStartTimeFlag, cmd.CreationEndTimeFlag, backupCreationStartDate, backupCreationStartDate)))
			})
			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is not provided",
				flagCreationEndTime, flagCreationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagCreationEndTime, backupCreationStartDate}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.CreationStartTimeFlag)))
			})

			//test case for flag expiration
			It(fmt.Sprintf("Should fail if flag %s is given without value", flagExpirationStartTime), func() {
				args := []string{cmdGet, cmdBackup}
				args = append(args, commonArgs...)
				args = append(args, flagExpirationStartTime)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagExpirationStartTime)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup}
				args = append(args, commonArgs...)
				args = append(args, flagExpirationEndTime)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagExpirationEndTime)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given zero value", flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationEndTime, ""}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.ExpirationEndTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is given 'invalid' value",
				flagExpirationStartTime, flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupCreationStartDate, flagExpirationEndTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.ExpirationEndTimeFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given valid & flag %s is given 'invalid' value",
				flagExpirationEndTime, flagExpirationStartTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationEndTime, backupCreationStartDate, flagExpirationStartTime, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.ExpirationStarTimeFlag)))
			})

			It(fmt.Sprintf("Should succeed if flag %s &  %s are given valid format date value",
				flagExpirationStartTime, flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationStartDate,
					flagExpirationEndTime, backupExpirationEndDate, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given valid format timestamp value", flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationStartDate, flagExpirationEndTime,
					fmt.Sprintf("%sT%sZ", backupExpirationEndDate, startTime), flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given valid format date value", flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationStartDate, flagExpirationEndTime,
					backupExpirationEndDate, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'T' value", flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationStartDate, flagExpirationEndTime,
					fmt.Sprintf("%s %sZ", backupExpirationEndDate, startTime), flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'Z' value", flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationStartDate, flagExpirationEndTime,
					fmt.Sprintf("%sT%s", backupExpirationEndDate, startTime), flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should succeed if flag %s is given invalid format timestamp without 'T' & 'Z' value",
				flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationStartDate, flagExpirationEndTime,
					fmt.Sprintf("%s %s", backupExpirationEndDate, startTime), flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})
			It(fmt.Sprintf("Should fail if flag %s &  %s are given valid format and equal Timestamp",
				flagExpirationStartTime, flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationEndDate, flagExpirationEndTime, backupExpirationEndDate}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] and [%s] flag values %sT00:00:00Z and "+
					"%sT00:00:00Z can't be same", cmd.ExpirationStarTimeFlag, cmd.ExpirationEndTimeFlag,
					backupExpirationEndDate, backupExpirationEndDate)))
			})
			It(fmt.Sprintf("Should fail if flag %s is given valid value & flag %s is not provided",
				flagExpirationStartTime, flagExpirationEndTime), func() {
				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, backupExpirationEndDate}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag value cannot be empty", cmd.ExpirationEndTimeFlag)))
			})
			It(fmt.Sprintf("Should fail if flag %s &  %s are given valid format and ExpirationStartTimestamp > CreationEndTimestamp",
				flagExpirationStartTime, flagExpirationEndTime), func() {

				args := []string{cmdGet, cmdBackup, flagExpirationStartTime, "2021-05-19", flagExpirationEndTime, backupCreationEndDate}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdBackup)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("400 Bad Request"))
			})
		})

		Context("Filtering Backups based on TVK Instance ID and Name", Ordered, func() {
			var (
				backupUID           string
				tvkInstanceIDValues map[string]string
			)
			BeforeAll(func() {

				removeBackupPlanDir()
				createTarget(true)
				// Generating backupPlans and backups with different TVK instance UID
				tvkInstanceIDValues = map[string]string{
					guid.New().String(): "tvk-instance-name1",
					guid.New().String(): "tvk-instance-name2",
				}
				backupUID = guid.New().String()

				for tvkUID, tvkName := range tvkInstanceIDValues {
					createBackups(1, 1, backupUID, "mutate-tvk-id", tvkUID, tvkName)
				}
				_ = verifyBackupPlansAndBackupsOnNFS(2, 2)
				Expect(verifyBrowserCacheBPlan(2)).Should(BeTrue())

			})
			AfterAll(func() {

				deleteTarget(false)
				removeBackupPlanDir()

			})

			It(fmt.Sprintf("Should filter backups if flag %s is provided", cmd.TvkInstanceUIDFlag), func() {
				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackup, flagTvkInstanceUIDFlag, tvkUID, flagOutputFormat, internal.FormatJSON}
					backupData := runCmdBackup(args)
					Expect(len(backupData)).To(Equal(1))
					Expect(backupData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupData[0].TvkInstanceName).Should(Equal(tvkName))
				}
			})

			It(fmt.Sprintf("Should get zero backups if flag %s is given 'invalid' value", cmd.TvkInstanceUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, flagTvkInstanceUIDFlag, "invalid", flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(0))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagTvkInstanceUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				args = append(args, flagTvkInstanceUIDFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagTvkInstanceUIDFlag)))
			})

			It(fmt.Sprintf("Should filter backups on flag %s and order by backup name ascending order", cmd.TvkInstanceUIDFlag), func() {
				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackup, flagTvkInstanceUIDFlag, tvkUID, flagOrderBy, name, flagOutputFormat, internal.FormatJSON}
					backupData := runCmdBackup(args)
					Expect(backupData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupData[0].TvkInstanceName).Should(Equal(tvkName))
					for index := 0; index < len(backupData)-1; index++ {
						Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
					}
				}
			})
			It(fmt.Sprintf("Should filter backups if flag %s is provided", cmd.TvkInstanceNameFlag), func() {
				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackup, flagTvkInstanceNameFlag, tvkName, flagOutputFormat, internal.FormatJSON}
					backupData := runCmdBackup(args)
					Expect(len(backupData)).To(Equal(1))
					Expect(backupData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupData[0].TvkInstanceName).Should(Equal(tvkName))
				}
			})
			It(fmt.Sprintf("Should get zero backups if flag %s is given 'invalid' value", cmd.TvkInstanceNameFlag), func() {
				args := []string{cmdGet, cmdBackup, flagTvkInstanceNameFlag, "invalid", flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(0))
			})
			It(fmt.Sprintf("Should fail if flag %s is given without value", flagTvkInstanceNameFlag), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				args = append(args, flagTvkInstanceNameFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagTvkInstanceNameFlag)))
			})
			It(fmt.Sprintf("Should filter backups on flag %s and order by backup name ascending order", cmd.TvkInstanceNameFlag), func() {

				for tvkUID, tvkName := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackup, flagTvkInstanceNameFlag, tvkName, flagOrderBy, name, flagOutputFormat, internal.FormatJSON}
					backupData := runCmdBackup(args)
					Expect(backupData[0].TvkInstanceID).Should(Equal(tvkUID))
					Expect(backupData[0].TvkInstanceName).Should(Equal(tvkName))
					for index := 0; index < len(backupData)-1; index++ {
						Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
					}
				}
			})
		})

		Context(fmt.Sprintf("test-cases for flag %s with zero, valid  and invalid value", cmd.OperationScopeFlag), Ordered, func() {
			var (
				noOfBackupPlansToCreate                        = 3
				noOfBackupsToCreatePerBackupPlan               = 1
				noOfClusterBackupPlansToCreate                 = 3
				noOfClusterBackupsToCreatePerClusterBackupPlan = 1

				backupUID, clusterBackupUID string
			)
			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				clusterBackupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfClusterBackupPlansToCreate, noOfClusterBackupsToCreatePerClusterBackupPlan, clusterBackupUID, "cluster_backup")
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup")
				_ = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate+noOfClusterBackupPlansToCreate,
					noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan+
						noOfClusterBackupsToCreatePerClusterBackupPlan*noOfClusterBackupPlansToCreate)
				//wait to sync data on target browser
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate + noOfClusterBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {
				deleteTarget(false)
				removeBackupPlanDir()
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", flagOperationScope), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				args = append(args, flagOperationScope)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagOperationScope)))
			})

			It(fmt.Sprintf("Should get both OperationScoped backup and clusterbackups if flag %s is not provided", flagOperationScope), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(
					noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan +
						noOfClusterBackupsToCreatePerClusterBackupPlan*noOfClusterBackupPlansToCreate))
				for idx := range backupData {
					if backupData[idx].UID == clusterBackupUID {
						Expect(backupData[idx].Kind).To(Equal(internal.ClusterBackupKind))
					} else {
						Expect(backupData[idx].Kind).To(Equal(internal.BackupKind))
					}
				}
			})

			It(fmt.Sprintf("Should get both backups and clusterbackups if flag %s is given zero value", flagOperationScope), func() {
				args := []string{cmdGet, cmdBackup, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(
					noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan +
						noOfClusterBackupsToCreatePerClusterBackupPlan*noOfClusterBackupPlansToCreate))
				for idx := range backupData {
					if backupData[idx].UID == clusterBackupUID {
						Expect(backupData[idx].Kind).To(Equal(internal.ClusterBackupKind))
					} else {
						Expect(backupData[idx].Kind).To(Equal(internal.BackupKind))
					}
				}

			})

			It(fmt.Sprintf("Should get %d backup if flag %s is given %s value",
				noOfClusterBackupPlansToCreate*noOfClusterBackupsToCreatePerClusterBackupPlan,
				flagOperationScope, internal.MultiNamespace), func() {
				args := []string{cmdGet, cmdBackup, flagOperationScope, internal.MultiNamespace, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(noOfClusterBackupPlansToCreate * noOfClusterBackupsToCreatePerClusterBackupPlan))
				for idx := range backupData {
					Expect(backupData[idx].Kind).To(Equal(internal.ClusterBackupKind))
				}
			})

			It(fmt.Sprintf("Should get %d backup if flag %s is given %s value", noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan,
				flagOperationScope, internal.SingleNamespace), func() {
				args := []string{cmdGet, cmdBackup, flagOperationScope, internal.SingleNamespace, flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(noOfBackupPlansToCreate * noOfBackupsToCreatePerBackupPlan))
				for idx := range backupData {
					Expect(backupData[idx].Kind).To(Equal(internal.BackupKind))
				}
			})

			It(fmt.Sprintf("Should get one backup for specific cluster backup UID %s if flag %s is given %s value",
				clusterBackupUID, cmd.OperationScopeFlag, internal.MultiNamespace), func() {

				args := []string{cmdGet, cmdBackup, clusterBackupUID, flagOperationScope, internal.MultiNamespace,
					flagOutputFormat, internal.FormatJSON}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(clusterBackupUID))
				Expect(backupData[0].Kind).To(Equal(internal.ClusterBackupKind))
			})
		})

	})

	Context("Metadata command test-cases", func() {

		Context("Filter Operations on different fields of backupPlan & backup", Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 1
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID                        string
			)

			BeforeAll(func() {

				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "all_type_backup")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {

				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget(false)
				removeBackupPlanDir()

			})

			It(fmt.Sprintf("Should fail if flag %s not given", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0]}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("required flag(s) \"%s\" not set",
					cmd.BackupUIDFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata}
				args = append(args, commonArgs...)
				args = append(args, flagBackupPlanUIDFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagBackupPlanUIDFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata}
				args = append(args, commonArgs...)
				args = append(args, flagBackupUIDFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagBackupUIDFlag)))
			})

			It(fmt.Sprintf("Should fail  metadata if flag %s is given zero value", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, ""}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("404 Not Found"))
			})

			It(fmt.Sprintf("Should fail  metadata if flag %s is given invalid value", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, "invalid"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("404 Not Found"))
			})

			It(fmt.Sprintf("Should filter metadata if flag %s is given zero value", cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, "", flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				validateMetadata(output, "backup-metadata-all.json")
			})

			It(fmt.Sprintf("Should filter metadata if flag %s is given valid value", cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				validateMetadata(output, "backup-metadata-all.json")
			})

			It("Should filter metadata on BackupPlan and backup", func() {

				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				validateMetadata(output, "backup-metadata-all.json")
			})
		})

		Context(fmt.Sprintf("Filter Operations on metadata with flag %s", cmd.OutputFormatFlag), Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 1
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID                        string
			)

			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup", "helm")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())
			})

			AfterAll(func() {

				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget(false)
				removeBackupPlanDir()

			})
			It(fmt.Sprintf("Should fail cmd metadata if flag %s is given without value", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				args = append(args, flagOutputFormat)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagOutputFormat)))
			})

			It(fmt.Sprintf("Should fail cmd metadata if flag %s is given invalid value", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupUIDFlag, backupUID, flagOutputFormat, "invalid"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("[%s] flag invalid value", cmd.OutputFormatFlag)))
			})

			It(fmt.Sprintf("Should filter metadata on BackupPlan and backup if flag %s is not provided", cmd.OutputFormatFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Metadata is %s", output)
				validateMetadata(output, backupMetadataHelm)
			})

			It("Should filter metadata on BackupPlan and backup", func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID,
					flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Metadata is %s", output)
				validateMetadata(output, backupMetadataHelm)
			})

			It(fmt.Sprintf("Should filter metadata on BackupPlan and backup using flag %s and value is %s",
				cmd.OutputFormatFlag, internal.FormatYAML), func() {

				args := []string{cmdGet, cmdMetadata, flagBackupUIDFlag, backupUID, flagOutputFormat, internal.FormatYAML}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Metadata is %s", output)
				validateMetadata(output, backupMetadataHelm, internal.FormatYAML)
			})

		})
		Context("Filter Operations on of backupPlan & backup for custom type backup", Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 1
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID                        string
			)

			BeforeAll(func() {

				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup", "custom")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())
			})

			AfterAll(func() {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget(false)
				removeBackupPlanDir()
			})

			It("Should filter metadata on BackupPlan and backup", func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID,
					flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdResourceMetadata)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Metadata is %s", output)
				validateMetadata(output, "backup-metadata-custom.json")
			})
		})
	})

	Context("Resource Metadata command test-cases", func() {

		Context("Filter Operations on multiple flags & their fields", Ordered, func() {

			BeforeAll(func() {

				createTarget(true)

			})

			AfterAll(func() {

				deleteTarget(false)

			})

			It("Should fail if multiple flags are not given", func() {
				args := []string{cmdGet, cmdResourceMetadata}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("required flag(s) \"%s\", \"%s\", \"%s\", \"%s\" not set",
					cmd.BackupUIDFlag, cmd.KindFlag, cmd.NameFlag, cmd.VersionFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s, %s & %s not given", cmd.KindFlag, cmd.NameFlag, cmd.VersionFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagBackupUIDFlag, "", flagBackupPlanUIDFlag, ""}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("required flag(s) \"%s\", \"%s\", \"%s\" not set",
					cmd.KindFlag, cmd.NameFlag, cmd.VersionFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s not given", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagKind, "", flagName, "", flagVersion, ""}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("required flag(s) \"%s\" not set",
					cmd.BackupUIDFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagKind, "", flagName, "", flagVersion, "", flagBackupPlanUIDFlag, ""}
				args = append(args, commonArgs...)
				args = append(args, flagBackupUIDFlag)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagBackupUIDFlag)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.KindFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagName, "", flagVersion, "", flagBackupUIDFlag, "", flagBackupPlanUIDFlag, ""}
				args = append(args, commonArgs...)
				args = append(args, flagKind)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagKind)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.VersionFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagKind, "", flagName, "", flagBackupUIDFlag, "", flagBackupPlanUIDFlag, ""}
				args = append(args, commonArgs...)
				args = append(args, flagVersion)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagVersion)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.NameFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagKind, "", flagVersion, "", flagBackupUIDFlag, "", flagBackupPlanUIDFlag, ""}
				args = append(args, commonArgs...)
				args = append(args, flagName)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagName)))
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.GroupFlag), func() {
				args := []string{cmdGet, cmdResourceMetadata, flagKind, "", flagName, "", flagVersion, "", flagBackupUIDFlag, "",
					flagBackupPlanUIDFlag, ""}
				args = append(args, commonArgs...)
				args = append(args, flagGroup)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagGroup)))
			})

			It("Should fail if flags are given wrong values", func() {

				args := []string{cmdGet, cmdResourceMetadata, flagGroup, "app", flagKind, "Deplo", flagVersion, "v",
					flagName, "mysq", flagBackupUIDFlag, "random", flagBackupPlanUIDFlag, "random"}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdResourceMetadata)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("did not successfully completed - 404 Not Found"))
			})
		})

		Context("Filter Operations on absolute values for flags used", Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 1
				noOfBackupsToCreatePerBackupPlan = 1
				backupUID                        string
			)

			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
					strconv.Itoa(noOfBackupsToCreatePerBackupPlan), "all_type_backup", backupUID, "custom", "resource-metadata").Output()

				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget(false)
				removeBackupPlanDir()

			})

			It("Should filter resource metadata on specific values of flags", func() {

				args := []string{cmdGet, cmdResourceMetadata, flagGroup, "storage.k8s.io", flagKind, "StorageClass",
					flagVersion, "v1", flagName, "csi-gce-pd", flagBackupUIDFlag, backupUID, flagBackupPlanUIDFlag,
					backupPlanUIDs[0], flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdResourceMetadata)
				log.Debugf("Resource Metadata is %s", output)
				Expect(err).ShouldNot(HaveOccurred())
				validateMetadata(output, "resource-metadata.json")
			})
		})
	})

	Context("Trilio Resources subcommand test-cases", func() {

		Context("Filter Operations on UID", Ordered, func() {

			BeforeAll(func() {
				createTarget(true)

			})

			AfterAll(func() {
				deleteTarget(false)

			})

			It("Should fail if backup uid not given", func() {
				args := []string{cmdGet, cmdBackup, cmdTrilioResources}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdTrilioResources)
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("at-least 1 backupUID is needed"))
			})

		})

		Context("Filter Operations on absolute values for flags used", Ordered, func() {
			var (
				backupPlanUIDs                   []string
				noOfBackupPlansToCreate          = 1
				noOfBackupsToCreatePerBackupPlan = 1

				backupUID string
			)

			BeforeAll(func() {
				removeBackupPlanDir()
				backupUID = guid.New().String()
				createTarget(true)
				createBackups(noOfBackupPlansToCreate, noOfBackupsToCreatePerBackupPlan, backupUID, "backup", "custom")
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreatePerBackupPlan)
				Expect(verifyBrowserCacheBPlan(noOfBackupPlansToCreate)).Should(BeTrue())

			})

			AfterAll(func() {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget(false)
				removeBackupPlanDir()

			})

			It("Should filter trilio resources when backupPlan not given and only 1 kind given", func() {
				args := []string{cmdGet, cmdBackup, cmdTrilioResources, backupUID, flagKinds, "Backup", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdTrilioResources)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Trilio Resources are %s", output)
				validateMetadata(output, "trilio-resources-backup.json")
			})

			It("Should filter trilio resources when no kinds given", func() {
				args := []string{cmdGet, cmdBackup, cmdTrilioResources, backupUID, flagBackupPlanUIDFlag, backupPlanUIDs[0],
					flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdTrilioResources)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Trilio Resources are %s", output)
				validateMetadata(output, "trilio-resources-backup-backupplan.json")
			})

			It("Should filter trilio resources when multiple kinds given", func() {

				args := []string{cmdGet, cmdBackup, cmdTrilioResources, backupUID, flagBackupPlanUIDFlag, backupPlanUIDs[0],
					flagKinds, "Backup,BackupPlan", flagOutputFormat, internal.FormatJSON}
				args = append(args, commonArgs...)
				output, err := runCommand(args, cmdTrilioResources)
				Expect(err).ShouldNot(HaveOccurred())
				log.Debugf("Trilio Resources are %s", output)
				validateMetadata(output, "trilio-resources-backup-backupplan.json")
			})
		})
	})
})

func validateBackupPlanKind(backupPlanData []targetbrowser.BackupPlan, backupPlanUIDSet, clusterBackupPlanUIDSet sets.String) {
	for idx := range backupPlanData {
		if clusterBackupPlanUIDSet.Has(backupPlanData[idx].UID) {
			Expect(backupPlanData[idx].Kind).To(Equal(internal.ClusterBackupPlanKind))
		} else if backupPlanUIDSet.Has(backupPlanData[idx].UID) {
			Expect(backupPlanData[idx].Kind).To(Equal(internal.BackupPlanKind))
		}
	}
}

func removeBackupPlanDir() {
	dir, err := ioutil.ReadDir(TargetLocation)
	Expect(err).Should(BeNil())
	for _, d := range dir {
		err = os.RemoveAll(path.Join([]string{TargetLocation, d.Name()}...))
		Expect(err).Should(BeNil())
	}
}

func validateMetadata(data []byte, metadataFileName string, dataType ...string) {
	re := regexp.MustCompile("(?m)[\r\n]+^.*location.*$")
	metadata := re.ReplaceAllString(string(data), "")
	re = regexp.MustCompile("(?m)[\r\n]+^.*uid.*$")
	metadata = re.ReplaceAllString(metadata, "")

	jsonFile, err := os.Open(filepath.Join(testDataDirRelPath, metadataFileName))
	Expect(err).To(BeNil())
	defer jsonFile.Close()
	expectedMetadata, _ := ioutil.ReadAll(jsonFile)
	if strings.Join(dataType, "") == internal.FormatYAML {
		expectedMetadata, err = yaml.JSONToYAML(expectedMetadata)
		Expect(err).To(BeNil())
	}
	Expect(reflect.DeepEqual(expectedMetadata, metadata))
}
