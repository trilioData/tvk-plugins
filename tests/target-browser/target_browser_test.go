package targetbrowsertest

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"sync"
	"time"

	guid "github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"github.com/trilioData/tvk-plugins/cmd/target-browser/cmd"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
)

const (
	backupPlanType         = "backupPlanType"
	name                   = "name"
	successfulBackups      = "successfulBackups"
	backupTimestamp        = "backupTimestamp"
	status                 = "status"
	cmdBackupPlan          = cmd.BackupPlanCmdName
	cmdBackup              = cmd.BackupCmdName
	cmdMetadata            = cmd.MetadataCmdName
	cmdGet                 = "get"
	flagPrefix             = "--"
	hyphen                 = "-"
	flagOrderBy            = flagPrefix + cmd.OrderByFlag
	flagTvkInstanceUIDFlag = flagPrefix + cmd.TvkInstanceUIDFlag
	flagBackupUIDFlag      = flagPrefix + cmd.BackupUIDFlag
	flagBackupStatus       = flagPrefix + cmd.BackupStatusFlag
	flagBackupPlanUIDFlag  = flagPrefix + cmd.BackupPlanUIDFlag
	flagPageSize           = flagPrefix + cmd.PageSizeFlag
	flagTargetNamespace    = flagPrefix + cmd.TargetNamespaceFlag
	flagTargetName         = flagPrefix + cmd.TargetNameFlag
	flagKubeConfig         = flagPrefix + cmd.KubeConfigFlag
	flagCaCert             = flagPrefix + cmd.CertificateAuthorityFlag
	flagInsecureSkip       = flagPrefix + cmd.InsecureSkipTLSFlag
	flagUseHTTPS           = flagPrefix + cmd.UseHTTPS
	flagOutputFormat       = flagPrefix + cmd.OutputFormatFlag
	kubeConf               = ""
	pageSize               = 1
)

var (
	backupStatus = []string{"Available", "Failed", "InProgress"}
	commonArgs   = []string{flagTargetName, TargetName, flagTargetNamespace,
		installNs, flagOutputFormat, internal.FormatJSON, flagKubeConfig, kubeConf}
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

		Context("test-cases with target 'browsingEnabled=false'", func() {
			var (
				once   sync.Once
				isLast bool
			)

			BeforeEach(func() {
				once.Do(func() {
					createTarget(false)
				})
			})

			AfterEach(func() {
				if isLast {
					deleteTarget(true)
				}
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
				isLast = true
				args := []string{cmdGet, cmdBackupPlan, flagKubeConfig, kubeConf}
				testArgs := []string{flagTargetName, TargetName}
				args = append(args, testArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("targets.triliovault.trilio.io \"%s\" not found", TargetName)))
			})
		})

		Context("test-cases with target 'browsingEnabled=true'", func() {
			BeforeEach(func() {
				createTarget(true)
				deleteTargetBrowserIngress()
			})

			AfterEach(func() {
				deleteTarget(false)
			})

			It("Should fail if target browser's ingress resource not found", func() {
				args := []string{cmdGet, cmdBackupPlan}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("either tvkHost or targetBrowserPath could not"+
					" retrieved for target %s namespace %s", TargetName, installNs)))
			})

		})

		Context("test-cases with target 'browsingEnabled=true' && TVK host is serving HTTPS", func() {
			var (
				once   sync.Once
				isLast bool
			)

			BeforeEach(func() {
				once.Do(func() {
					switchTvkHostFromHTTPToHTTPS()
					time.Sleep(time.Second * 10)
					createTarget(true)
				})
			})

			AfterEach(func() {
				if isLast {
					deleteTarget(false)
					switchTvkHostFromHTTPSToHTTP()
					time.Sleep(time.Second * 10)
				}
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
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("Client.Timeout exceeded while awaiting headers"))
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
				isLast = true
				args := []string{cmdGet, cmdBackupPlan, flagCaCert, filepath.Join(testDataDirRelPath, tlsCertFile)}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("Client.Timeout exceeded while awaiting headers"))
			})
		})
	})

	Context("BackupPlan command's Flag's test-cases", func() {
		Context(fmt.Sprintf("test-cases for sorting on columns %s, %s, %s, %s",
			backupPlanType, name, successfulBackups, backupTimestamp), func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 4
				noOfBackupsToCreate     = 1
				once                    sync.Once
				isLast                  bool
				backupUID               string
			)
			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(2 * time.Minute) // wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", backupPlanType), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, backupPlanType}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Type <= backupPlanData[index+1].Type).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", backupPlanType), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + backupPlanType}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Type >= backupPlanData[index+1].Type).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, name}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + name}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name >= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", successfulBackups), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, successfulBackups}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].SuccessfulBackup <= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", successfulBackups), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + successfulBackups}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].SuccessfulBackup >= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", backupTimestamp), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, backupTimestamp}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					if backupPlanData[index].SuccessfulBackup != 0 && backupPlanData[index+1].SuccessfulBackup != 0 &&
						!backupPlanData[index].SuccessfulBackupTimestamp.Time.Equal(backupPlanData[index+1].
							SuccessfulBackupTimestamp.Time) {
						Expect(backupPlanData[index].SuccessfulBackupTimestamp.Time.Before(backupPlanData[index+1].
							SuccessfulBackupTimestamp.Time)).Should(BeTrue())
					}
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", backupTimestamp), func() {
				isLast = true
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, hyphen + backupTimestamp}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					if backupPlanData[index].SuccessfulBackup != 0 && backupPlanData[index+1].SuccessfulBackup != 0 &&
						!backupPlanData[index].SuccessfulBackupTimestamp.Time.Equal(backupPlanData[index+1].
							SuccessfulBackupTimestamp.Time) {
						Expect(backupPlanData[index].SuccessfulBackupTimestamp.Time.After(backupPlanData[index+1].
							SuccessfulBackupTimestamp.Time)).Should(BeTrue())
					}
				}
			})
		})

		Context(fmt.Sprintf("test-cases for flag %s, %s with path value, zero, valid  and invalid value",
			cmd.OrderByFlag, cmd.PageSizeFlag), func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 11
				noOfBackupsToCreate     = 2
				once                    sync.Once
				isLast                  bool
				backupUID               string
			)
			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(2 * time.Minute) //wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
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

			It(fmt.Sprintf("Should sort in ascending order using flag %s with default value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order name using default flag %s with zero value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, ""}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order name using default flag %s with 'invalid' value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "invalid"}
				backupPlanData := runCmdBackupPlan(args)
				for index := 0; index < len(backupPlanData)-1; index++ {
					Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should get one page backupPlan using flag %s", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, strconv.Itoa(pageSize)}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(pageSize))
			})

			It(fmt.Sprintf("Should get one page backupPlan using flag %s with default value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should get one page backupPlan using flag %s with zero value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, "0"}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should fail using flag %s with invalid value 'invalid'", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagPageSize, "invalid"}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf(" invalid argument \"invalid\" for \"%s\"", flagPageSize)))
			})

			It("Should get one backupPlan for specific backupPlan UID", func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[1]}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(1))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[1]))
			})

			It("Should get two backupPlan for backupPlan UIDs", func() {
				args := []string{cmdGet, cmdBackupPlan, backupPlanUIDs[0], backupPlanUIDs[1]}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(2))
				Expect(backupPlanData[0].UID).To(Equal(backupPlanUIDs[0]))
				Expect(backupPlanData[1].UID).To(Equal(backupPlanUIDs[1]))
			})

			It("Should fail using backupPlanUID 'invalidUID'", func() {
				isLast = true
				args := []string{cmdGet, cmdBackupPlan, "invalidUID"}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("404 Not Found"))
			})
		})

		Context("Filtering BackupPlans based on TVK Instance ID", func() {

			var (
				backupPlanUIDs      []string
				tvkInstanceIDValues []string
				once                sync.Once
				isLast              bool
			)
			BeforeEach(func() {
				once.Do(func() {
					createTarget(true)
					// Generating backupPlans and backups with different TVK instance UID
					tvkInstanceIDValues = []string{
						guid.New().String(),
						guid.New().String(),
					}
					for _, value := range tvkInstanceIDValues {
						_, err := exec.Command(createBackupScript, "1", "1", "mutate-tvk-id", value).Output()
						Expect(err).To(BeNil())
					}
					time.Sleep(2 * time.Minute) // wait to sync data on target browser
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(2, 2)
				})
			})
			AfterEach(func() {
				if isLast {
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})

			It(fmt.Sprintf("Should filter backupPlans on flag %s", cmd.TvkInstanceUIDFlag), func() {
				for _, value := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, value}
					backupPlanData := runCmdBackupPlan(args)
					Expect(len(backupPlanData)).To(Equal(1))
					Expect(backupPlanData[0].TvkInstanceID).Should(Equal(value))
				}
			})

			It(fmt.Sprintf("Should get zero backupPlans using flag %s with 'invalid' value", cmd.TvkInstanceUIDFlag), func() {
				args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, "invalid"}
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
				isLast = true
				for _, value := range tvkInstanceIDValues {
					args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, value, flagOrderBy, name}
					backupPlanData := runCmdBackupPlan(args)
					Expect(len(backupPlanData)).To(Equal(1))
					Expect(backupPlanData[0].TvkInstanceID).Should(Equal(value))
					for index := 0; index < len(backupPlanData)-1; index++ {
						Expect(backupPlanData[index].Name <= backupPlanData[index+1].Name).Should(BeTrue())
					}
				}
			})
		})
	})

	Context("Backup command's Flag's test-cases", func() {
		Context(fmt.Sprintf("test-cases for filtering and sorting operations on %s, %s columns of Backup", status, name), func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 11
				noOfBackupsToCreate     = 1
				once                    sync.Once
				isLast                  bool
				backupUID               string
			)
			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(2 * time.Minute) //wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					//delete target & remove all files & directories created for this Context - only once After all It in this context
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})
			for _, bkpStatus := range backupStatus {
				bkpStatus := bkpStatus
				It(fmt.Sprintf("Should filter backup with backup status '%s'", bkpStatus), func() {
					args := []string{cmdGet, cmdBackup, flagBackupStatus, bkpStatus}
					backupData := runCmdBackup(args)
					// compare backups status with status value passed as arg
					for index := 0; index < len(backupData); index++ {
						Expect(backupData[index].Status).Should(Equal(bkpStatus))
					}
				})
			}

			It(fmt.Sprintf("Should sort in ascending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackup, flagOrderBy, name}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", name), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, hyphen + name}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name >= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order '%s'", status), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, status}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Status <= backupData[index+1].Status).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in descending order '%s'", status), func() {
				isLast = true
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, hyphen + status}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Status <= backupData[index+1].Status).Should(BeTrue())
				}
			})
		})

		Context(fmt.Sprintf("test-cases for flag %s, %s, %s, %s with path value, zero, valid  and invalid value",
			cmd.OrderByFlag, cmd.PageSizeFlag, cmd.BackupPlanUIDFlag, cmd.BackupStatusFlag), func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 11
				noOfBackupsToCreate     = 1
				once                    sync.Once
				isLast                  bool
				backupUID               string
			)
			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(2 * time.Minute) // wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})

			It(fmt.Sprintf("Should fail if flag %s is given without value", cmd.BackupStatusFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[1]}
				args = append(args, commonArgs...)
				args = append(args, flagBackupStatus)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("flag needs an argument: %s", flagBackupStatus)))
			})
			It(fmt.Sprintf("Should get zero backup using flag %s with 'invalid' value", cmd.BackupStatusFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupStatus, "invalid"}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(0))
			})

			It(fmt.Sprintf("Should get one page backup using flag %s with zero value", cmd.BackupStatusFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupStatus, ""}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should get one backup for specific backup UID %s", backupUID), func() {
				args := []string{cmdGet, cmdBackup, backupUID}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(1))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should get two backup for backup UID %s", backupUID), func() {
				args := []string{cmdGet, cmdBackup, backupUID, backupUID}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(2))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It("Should get zero backup using backupPlanUID with 'invalid' value", func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, "invalid"}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).Should(Equal(0))
			})

			It(fmt.Sprintf("Should get %d backup using flag %s with zero value", cmd.PageSizeDefault, cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, ""}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).Should(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should get one page backup using flag %s with value %d with flag %s", cmd.PageSizeFlag,
				pageSize, cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[1], flagPageSize, strconv.Itoa(pageSize)}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(pageSize))
			})
			It(fmt.Sprintf("Should get one page backup using flag %s with default value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should get one page backup using flag %s with zero value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup, flagPageSize, "0"}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(cmd.PageSizeDefault))
			})

			It(fmt.Sprintf("Should fail using flag %s with 'invalid' value", cmd.PageSizeFlag), func() {
				args := []string{cmdGet, cmdBackup, flagPageSize, "invalid"}
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf(" invalid argument \"invalid\" for \"%s\"", flagPageSize)))
			})

			It(fmt.Sprintf("Should get one page backup using flag %s value %d without flag %s", cmd.PageSizeFlag,
				pageSize, cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdBackup, backupUID, flagPageSize, strconv.Itoa(pageSize)}
				backupData := runCmdBackup(args)
				Expect(len(backupData)).To(Equal(pageSize))
				Expect(backupData[0].UID).To(Equal(backupUID))
			})

			It(fmt.Sprintf("Should sort in ascending order name using default flag %s", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackup}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order name using default flag %s with zero value", flagOrderBy), func() {
				args := []string{cmdGet, cmdBackup, flagOrderBy, ""}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
				}
			})

			It(fmt.Sprintf("Should sort in ascending order name using default flag %s with 'invalid' value", flagOrderBy), func() {
				isLast = true
				args := []string{cmdGet, cmdBackup, flagOrderBy, "invalid"}
				backupData := runCmdBackup(args)
				for index := 0; index < len(backupData)-1; index++ {
					Expect(backupData[index].Name <= backupData[index+1].Name).Should(BeTrue())
				}
			})
		})
	})
	Context("Metadata command's Flag's test-cases", func() {
		Context("Filter Operations on different fields of backupPlan & backup", func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 1
				noOfBackupsToCreate     = 1
				once                    sync.Once
				isLast                  bool
				backupUID               string
				expectedEmptyMetadata   = `{"custom": {}}`
			)

			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "all_type_backup", backupUID).Output()

					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(2 * time.Minute) //wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					// delete target & remove all files & directories created for this Context - only once After all It in this context
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})
			It(fmt.Sprintf("Should fail if flag %s and %s not given", cmd.BackupPlanUIDFlag, cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("required flag(s) \"%s\", \"%s\" not set",
					cmd.BackupPlanUIDFlag, cmd.BackupUIDFlag)))
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

			It(fmt.Sprintf("Should fail if flag %s not given", cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).Should(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring(fmt.Sprintf("required flag(s) \"%s\" not set",
					cmd.BackupPlanUIDFlag)))
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

			It(fmt.Sprintf("Should get empty  metadata if flag %s is given zero value", cmd.BackupUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, ""}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(reflect.DeepEqual(expectedEmptyMetadata, output))
			})

			It(fmt.Sprintf("Should get empty  metadata if flag %s is given zero value", cmd.BackupPlanUIDFlag), func() {
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, "", flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(reflect.DeepEqual(expectedEmptyMetadata, output))
			})

			It("Should filter metadata on BackupPlan and backup", func() {
				isLast = true
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).ShouldNot(HaveOccurred())
				re := regexp.MustCompile("(?m)[\r\n]+^.*location.*$")
				metadata := re.ReplaceAllString(string(output), "")
				jsonFile, err := os.Open(filepath.Join(testDataDirRelPath, "backup-metadata-all.json"))
				Expect(err).To(BeNil())
				defer jsonFile.Close()
				expectedMetadata, _ := ioutil.ReadAll(jsonFile)
				Expect(reflect.DeepEqual(expectedMetadata, metadata))
			})
		})
		Context("Filter Operations on of backupPlan & backup for helm type backup", func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 1
				noOfBackupsToCreate     = 1
				once                    sync.Once
				isLast                  bool
				backupUID               string
			)

			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "true", backupUID, "helm").Output()

					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(1 * time.Minute) //wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					// delete target & remove all files & directories created for this Context - only once After all It in this context
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})

			It("Should filter metadata on BackupPlan and backup", func() {
				isLast = true
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).ShouldNot(HaveOccurred())
				log.Infof("Metadata is %s", output)
				re := regexp.MustCompile("(?m)[\r\n]+^.*location.*$")
				metadata := re.ReplaceAllString(string(output), "")
				jsonFile, err := os.Open(filepath.Join(testDataDirRelPath, "backup-metadata-helm.json"))
				Expect(err).To(BeNil())
				defer jsonFile.Close()
				expectedMetadata, _ := ioutil.ReadAll(jsonFile)
				Expect(reflect.DeepEqual(expectedMetadata, metadata))
			})
		})
		Context("Filter Operations on of backupPlan & backup for custom type backup", func() {
			var (
				backupPlanUIDs          []string
				noOfBackupPlansToCreate = 1
				noOfBackupsToCreate     = 1
				once                    sync.Once
				isLast                  bool
				backupUID               string
			)

			BeforeEach(func() {
				once.Do(func() {
					backupUID = guid.New().String()
					createTarget(true)
					output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
						strconv.Itoa(noOfBackupsToCreate), "true", backupUID, "custom").Output()

					log.Info("Shell Script Output: ", string(output))
					backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupPlansToCreate, noOfBackupPlansToCreate*noOfBackupsToCreate)
					time.Sleep(1 * time.Minute) //wait to sync data on target browser
				})
			})

			AfterEach(func() {
				if isLast {
					// delete target & remove all files & directories created for this Context - only once After all It in this context
					deleteTarget(false)
					for _, backupPlans := range backupPlanUIDs {
						_, err := shell.RmRf(filepath.Join(TargetLocation, backupPlans))
						Expect(err).To(BeNil())
					}
				}
			})

			It("Should filter metadata on BackupPlan and backup", func() {
				isLast = true
				args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagBackupUIDFlag, backupUID}
				args = append(args, commonArgs...)
				command := exec.Command(targetBrowserBinaryFilePath, args...)
				output, err := command.CombinedOutput()
				Expect(err).ShouldNot(HaveOccurred())
				log.Infof("Metadata is %s", output)
				re := regexp.MustCompile("(?m)[\r\n]+^.*location.*$")
				metadata := re.ReplaceAllString(string(output), "")
				jsonFile, err := os.Open(filepath.Join(testDataDirRelPath, "backup-metadata-custom.json"))
				Expect(err).To(BeNil())
				defer jsonFile.Close()
				expectedMetadata, _ := ioutil.ReadAll(jsonFile)
				Expect(reflect.DeepEqual(expectedMetadata, metadata))
			})
		})
	})
})
