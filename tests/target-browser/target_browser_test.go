package targetbrowsertest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	guid "github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type backupPlan struct {
	BackupPlanName            string      `json:"BackupPlan Name"`
	BackupPlanUID             string      `json:"BackupPlanUID"`
	BackupPlanType            string      `json:"BackupPlan Type"`
	InstanceID                string      `json:"Instance ID"`
	SuccessfulBackup          int         `json:"Successful Backup"`
	SuccessfulBackupTimestamp metav1.Time `json:"Successful Backup Timestamp"`
}
type backup struct {
	BackupName     string `json:"Backup Name"`
	BackupStatus   string `json:"Backup Status"`
	BackupSize     string `json:"Backup Size"`
	BackupType     string `json:"bBackup Type"`
	BackupUID      string `json:"backupUID"`
	CreationDate   string `json:"Creation Date"`
	TargetLocation string `json:"Target Location"`
}

const (
	BackupPlanType    string = "backupPlanType"
	Name              string = "name"
	SuccessfulBackups string = "successfulBackups"
	BackupTimestamp   string = "backupTimestamp"
)

var (
	kubeConf, _               = internal.NewConfigFromCommandline("")
	targetBrowserBackupStatus = []string{"Available", "Failed", "InProgress"}
	commonArgs                = []string{flagTargetName, internal.TargetName, flagTargetNamespace,
		installNs, flagKubeConfig, kubeConf}
)

var _ = Describe("Target Browser Tests", func() {

	Context("Sorting Operations on multiple columns of backupplan & backup", func() {
		var (
			backupPlanUIDs          []string
			noOfBackupplansToCreate = 8
			noOfBackupsToCreate     = 2
			once                    sync.Once
			isLast                  bool
		)
		BeforeEach(func() {
			backupUID := guid.New().String()
			// once.Do run once for this Context
			once.Do(func() {

				// create target with browsing enabled & create all files & directories required for this Context in NFS server
				// being used by target - only once Before all It in this context
				createTarget()
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupplansToCreate),
					strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupplansToCreate, noOfBackupplansToCreate*noOfBackupsToCreate)
				time.Sleep(2 * time.Minute) //wait to sync data on target browser
			})

		})

		AfterEach(func() {
			if isLast {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget()
				for _, backupPlans := range backupPlanUIDs {
					_, err := RmRf(targetLocation + "/" + backupPlans)
					Expect(err).To(BeNil())
				}
			}
		})

		It("Should sort backupplans on BackupPlan Application Type in ascending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, BackupPlanType}
			backupPlanData := runCmdBackupPlan(args)
			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanType <= backupPlanData[index+1].BackupPlanType).Should(BeTrue())
			}
		})

		It("Should sort backupplans on BackupPlan Application Type in descending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "-" + BackupPlanType}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanType >= backupPlanData[index+1].BackupPlanType).Should(BeTrue())
			}
		})
		It("Should sort backupplans on BackupPlan Name in ascending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, Name}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanName <= backupPlanData[index+1].BackupPlanName).Should(BeTrue())
			}
		})

		It("Should sort backupplans on BackupPlan Name in ascending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "-" + Name}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanName >= backupPlanData[index+1].BackupPlanName).Should(BeTrue())
			}
		})
		It("Should sort backupplans on Successful Backup Count in ascending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, SuccessfulBackups}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].SuccessfulBackup <= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
			}
		})
		It("Should sort backupplans on Successful Backup Count in descending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "-" + SuccessfulBackups}
			backupPlanData := runCmdBackupPlan(args)
			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].SuccessfulBackup >= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
			}
		})
		It("Should sort backupplans on LastSuccessfulBackupTimestamp in ascending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, BackupTimestamp}
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
		It("Should sort backupplans on LastSuccessfulBackupTimestamp in descending order", func() {
			args := []string{cmdGet, cmdBackupPlan, flagOrderBy, "-" + BackupTimestamp}
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
		It("Should sort backups on name in ascending order", func() {
			args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, Name}
			backupData := runCmdBackup(args)

			for index := 0; index < len(backupData)-1; index++ {
				Expect(backupData[index].BackupName <= backupData[index+1].BackupName).Should(BeTrue())
			}
		})
		It("Should sort backups on name in descending order", func() {
			// set isLast true here so that cleanup logic placed in AfterEach can run
			isLast = true
			args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, "-" + Name}
			backupData := runCmdBackup(args)

			for index := 0; index < len(backupData)-1; index++ {
				Expect(backupData[index].BackupName >= backupData[index+1].BackupName).Should(BeTrue())
			}
		})
	})

	Context("Filtering Operations on different fields of backupplan & backup", func() {
		var (
			backupPlanUIDs          []string
			noOfBackupplansToCreate = 1
			noOfBackupsToCreate     = 1
			once                    sync.Once
			isLast                  bool
		)

		BeforeEach(func() {
			backupUID := guid.New().String()
			// once.Do run once for this Context
			once.Do(func() {
				//create target with browsing enabled & create all files & directories required for this Context in NFS server
				//being used by target - only once Before all It in this context
				createTarget()
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupplansToCreate),
					strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupplansToCreate, noOfBackupplansToCreate*noOfBackupsToCreate)
				time.Sleep(2 * time.Minute) //wait to sync data on target browser
			})

		})

		AfterEach(func() {
			if isLast {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget()
				for _, backupPlans := range backupPlanUIDs {
					_, err := RmRf(targetLocation + "/" + backupPlans)
					Expect(err).To(BeNil())
				}
			}
		})
		It("Should filter backupplans on BackupPlan Application Type", func() {
			// select random backup status for status filter
			statusFilterValue := targetBrowserBackupStatus[rand.Intn(len(targetBrowserBackupStatus))]
			backupPlanUID := backupPlanUIDs[0]
			args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUID, flagBackupStatus, statusFilterValue}
			backupData := runCmdBackup(args)
			// compare backups status with status value passed as arg
			for index := 0; index < len(backupData); index++ {
				Expect(backupData[index].BackupStatus).Should(Equal(statusFilterValue))
			}
		})

		It("Should get one backupplan", func() {
			args := []string{cmdGet, cmdBackupPlan, flagPageSize, "1"}
			backupPlanData := runCmdBackupPlan(args)
			Expect(len(backupPlanData)).To(Equal(1))
		})

		It("Should get one backup", func() {
			isLast = true
			backupPlanUID := backupPlanUIDs[0]
			args := []string{cmdGet, cmdBackup, flagBackupPlanUIDFlag, backupPlanUID, flagPageSize, "1"}
			backupData := runCmdBackup(args)
			Expect(len(backupData)).To(Equal(1))
		})

	})
	Context("Filtering BackupPlans based on TVK Instance ID", func() {

		var (
			backupPlanUIDs []string
		)
		BeforeEach(func() {
			createTarget()
		})
		AfterEach(func() {
			deleteTarget()
			for _, backupPlans := range backupPlanUIDs {
				_, err := RmRf(targetLocation + "/" + backupPlans)
				Expect(err).To(BeNil())
			}
		})

		It("Should filter backupplans on TVK Instance UID", func() {

			// Generating backupplans and backups with different TVK instance UID
			tvkInstanceIDValue := guid.New().String()
			_, err := exec.Command(createBackupScript, "1", "1", "mutate-tvk-id", tvkInstanceIDValue).Output()
			Expect(err).To(BeNil())
			time.Sleep(2 * time.Minute) //wait to sync data on target browser
			backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(1, 1)
			args := []string{cmdGet, cmdBackupPlan, flagTvkInstanceUIDFlag, tvkInstanceIDValue}
			backupPlanData := runCmdBackupPlan(args)
			Expect(len(backupPlanData)).To(Equal(1))
			Expect(backupPlanData[0].InstanceID).Should(Equal(tvkInstanceIDValue))
		})

	})

	Context("Metadata filtering Operations on different fields of backupplan & backup", func() {
		var (
			backupPlanUIDs          []string
			noOfBackupplansToCreate = 1
			noOfBackupsToCreate     = 1
			once                    sync.Once
			isLast                  bool
			backupIDValue           string
		)

		BeforeEach(func() {
			backupIDValue = guid.New().String()
			// once.Do run once for this Context
			once.Do(func() {
				// create target with browsing enabled & create all files & directories required for this Context in NFS server
				// being used by target - only once Before all It in this context
				createTarget()
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupplansToCreate),
					strconv.Itoa(noOfBackupsToCreate), "true", backupIDValue, "helm_backup_type").Output()

				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs = verifyBackupPlansAndBackupsOnNFS(noOfBackupplansToCreate, noOfBackupplansToCreate*noOfBackupsToCreate)
				time.Sleep(2 * time.Minute) //wait to sync data on target browser
			})
		})
		AfterEach(func() {
			if isLast {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				deleteTarget()
				for _, backupPlans := range backupPlanUIDs {
					_, err := RmRf(targetLocation + "/" + backupPlans)
					Expect(err).To(BeNil())
				}
			}
		})
		It("Should filter metadata on BackupPlan and backup", func() {
			isLast = true
			backupPlanUID := backupPlanUIDs[0]
			args := []string{cmdGet, cmdMetadata, flagBackupPlanUIDFlag, backupPlanUID, flagBackupUIDFlag, backupIDValue}
			args = append(args, commonArgs...)
			cmd := exec.Command("./"+path.Join(internal.TargetBrowserBinaryLocation, internal.TargetBrowserBinaryName), args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				Fail(fmt.Sprintf("Error to execute command %s", err.Error()))
			}
			log.Infof("Metadata is %s", output)
			re := regexp.MustCompile("(?m)[\r\n]+^.*location.*$")
			metadata := re.ReplaceAllString(string(output), "")
			jsonFile, err := os.Open(path.Join(path.Join(currentDir, testDataDirRelPath, "metadata-helm.json")))
			Expect(err).To(BeNil())

			defer jsonFile.Close()

			expectedMetadata, _ := ioutil.ReadAll(jsonFile)
			Expect(reflect.DeepEqual(expectedMetadata, metadata))
		})
	})

})

func createTarget() {
	By("Creating target and marking it available")
	target := fmt.Sprintf("kubectl apply -f %s --namespace %s", path.Join(currentDir, testDataDirRelPath, targetPath), installNs)
	cmd := exec.Command("bash", "-c", target)

	_, err := cmd.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("target creation failed %s.", err.Error()))
	}
	VerifyTargetStatus(installNs)
}

func deleteTarget() {
	target := fmt.Sprintf("kubectl delete -f %s --namespace %s", path.Join(currentDir, testDataDirRelPath, targetPath), installNs)
	cmd := exec.Command("bash", "-c", target)
	_, err := cmd.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("target creation failed %s.", err.Error()))
	}
}

func runCmdBackupPlan(args []string) []backupPlan {
	args = append(args, commonArgs...)
	var output []byte
	var err error
	Eventually(func() bool {
		cmd := exec.Command(path.Join(internal.TargetBrowserBinaryLocation, internal.TargetBrowserBinaryName), args...)
		log.Info("BackupPlan command is: ", cmd)
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Errorf(fmt.Sprintf("Error to execute command %s", err.Error()))
		}
		log.Infof("BackupPlan data is %s", output)
		return strings.Contains(string(output), "502 Bad Gateway")
	}, timeout, interval).Should(BeFalse())

	var backupPlanData []backupPlan
	err = json.Unmarshal(output, &backupPlanData)
	if err != nil {
		log.Errorf(fmt.Sprintf("Failed to get backupplan data from target browser %s.", err.Error()))
	}
	return backupPlanData
}

func runCmdBackup(args []string) []backup {
	var output []byte
	var err error
	args = append(args, commonArgs...)
	Eventually(func() bool {
		cmd := exec.Command(path.Join(internal.TargetBrowserBinaryLocation, internal.TargetBrowserBinaryName), args...)
		log.Info("Backup command is: ", cmd)
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Infof(fmt.Sprintf("Error to execute command %s", err.Error()))
		}
		log.Infof("Backup data is %s", output)
		return strings.Contains(string(output), "502 Bad Gateway")
	}, timeout, interval).Should(BeFalse())

	var backupData []backup
	err = json.Unmarshal(output, &backupData)
	if err != nil {
		Fail(fmt.Sprintf("Failed to get backup data from target browser %s.", err.Error()))
	}
	return backupData
}
