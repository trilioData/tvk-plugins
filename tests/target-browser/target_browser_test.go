package targetbrowser

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
	"sync"

	guid "github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
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
	targetBrowserBackupStatus = []string{"Available", "Failed", "InProgress"}
)

var _ = Describe("Target Browser Tests", func() {

	Context("Sorting Operations on multiple columns of backupplan & backup", func() {
		var (
			backupPlanUIDs          []string
			noOfBackupplansToCreate = 2
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
				//createTarget()
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupplansToCreate),
					strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs, _ = verifyBackupPlansAndBackupsOnNFS(noOfBackupplansToCreate, noOfBackupplansToCreate*noOfBackupsToCreate)
			})

		})

		AfterEach(func() {
			if isLast {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				//deleteTarget()
				for _, backupPlans := range backupPlanUIDs {
					_, err := RmRf(targetLocation + "/" + backupPlans)
					fmt.Println(err)
					Expect(err).To(BeNil())
				}
			}
		})

		It("Should sort backupplans on BackupPlan Application Type in ascending order", func() {
			//isLast=true
			args := []string{cmdBackupPlan, flagOrderBy, BackupPlanType}
			backupPlanData := runCmdBackupPlan(args)
			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanType <= backupPlanData[index+1].BackupPlanType).Should(BeTrue())
			}
		})

		It("Should sort backupplans on BackupPlan Application Type in descending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, "-" + BackupPlanType}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanType >= backupPlanData[index+1].BackupPlanType).Should(BeTrue())
			}
		})
		It("Should sort backupplans on BackupPlan Name in ascending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, Name}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanName <= backupPlanData[index+1].BackupPlanName).Should(BeTrue())
			}
		})

		It("Should sort backupplans on BackupPlan Name in ascending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, "-" + Name}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].BackupPlanName >= backupPlanData[index+1].BackupPlanName).Should(BeTrue())
			}
		})
		It("Should sort backupplans on Successful Backup Count in ascending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, SuccessfulBackups}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].SuccessfulBackup <= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
			}
		})
		It("Should sort backupplans on Successful Backup Count in descending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, "-" + SuccessfulBackups}
			backupPlanData := runCmdBackupPlan(args)
			for index := 0; index < len(backupPlanData)-1; index++ {
				Expect(backupPlanData[index].SuccessfulBackup >= backupPlanData[index+1].SuccessfulBackup).Should(BeTrue())
			}
		})
		It("Should sort backupplans on LastSuccessfulBackupTimestamp in ascending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, BackupTimestamp}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				if backupPlanData[index].SuccessfulBackup != 0 && backupPlanData[index+1].SuccessfulBackup != 0 {
					fmt.Println(backupPlanData[index].SuccessfulBackupTimestamp.Time, backupPlanData[index+1].
						SuccessfulBackupTimestamp.Time)
					Expect(backupPlanData[index].SuccessfulBackupTimestamp.Time.Before(backupPlanData[index+1].
						SuccessfulBackupTimestamp.Time)).Should(BeTrue())
				}
			}
		})
		It("Should sort backupplans on LastSuccessfulBackupTimestamp in descending order", func() {
			args := []string{cmdBackupPlan, flagOrderBy, "-" + BackupTimestamp}
			backupPlanData := runCmdBackupPlan(args)

			for index := 0; index < len(backupPlanData)-1; index++ {
				if backupPlanData[index].SuccessfulBackup != 0 && backupPlanData[index+1].SuccessfulBackup != 0 {
					Expect(backupPlanData[index].SuccessfulBackupTimestamp.Time.After(backupPlanData[index+1].
						SuccessfulBackupTimestamp.Time)).Should(BeTrue())
				}
			}
		})
		It("Should sort backups on name in ascending order", func() {
			args := []string{cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, Name}
			backupData := runCmdBackup(args)

			for index := 0; index < len(backupData)-1; index++ {
				Expect(backupData[index].BackupName <= backupData[index+1].BackupName).Should(BeTrue())
			}
		})
		It("Should sort backups on name in descending order", func() {
			// set isLast true here so that cleanup logic placed in AfterEach can run
			isLast = true
			args := []string{cmdBackup, flagBackupPlanUIDFlag, backupPlanUIDs[0], flagOrderBy, "-" + Name}
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
				//createTarget()
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupplansToCreate),
					strconv.Itoa(noOfBackupsToCreate), "true", backupUID).Output()
				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs, _ = verifyBackupPlansAndBackupsOnNFS(noOfBackupplansToCreate, noOfBackupplansToCreate*noOfBackupsToCreate)
			})
		})

		AfterEach(func() {
			if isLast {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				//deleteTarget()
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
			args := []string{cmdBackup, flagBackupPlanUIDFlag, backupPlanUID, flagBackupStatus, statusFilterValue}
			backupData := runCmdBackup(args)
			// compare backups status with status value passed as arg
			for index := 0; index < len(backupData); index++ {
				Expect(backupData[index].BackupStatus).Should(Equal(statusFilterValue))
			}
		})

		It("Should get one backupplan", func() {
			args := []string{cmdBackupPlan, flagPageSize, "1"}
			backupPlanData := runCmdBackupPlan(args)
			Expect(len(backupPlanData)).To(Equal(1))
		})

		It("Should get one backup", func() {
			isLast = true
			backupPlanUID := backupPlanUIDs[0]
			args := []string{cmdBackup, flagBackupPlanUIDFlag, backupPlanUID, flagPageSize, "1"}
			backupData := runCmdBackup(args)
			Expect(len(backupData)).To(Equal(1))
		})

	})
	Context("Filtering BackupPlans based on TVK Instance ID", func() {

		var (
			backupPlanUIDs []string
		)

		AfterEach(func() {
			//deleteTarget()

			for _, backupPlans := range backupPlanUIDs {
				_, err := RmRf(targetLocation + "/" + backupPlans)
				log.Info(err)
			}
		})

		It("Should filter backupplans on TVK Instance UID", func() {

			tvkInstanceIDValues := []string{
				guid.New().String(),
				guid.New().String(),
			}

			// Generating backupplans and backups with different TVK instance UID
			for _, value := range tvkInstanceIDValues {
				_, err := exec.Command(createBackupScript, "1", "1", "mutate-tvk-id", value).Output()
				Expect(err).To(BeNil())
			}

			//createTarget()
			backupPlanUIDs, _ = verifyBackupPlansAndBackupsOnNFS(1, 1)

			for _, value := range tvkInstanceIDValues {
				args := []string{cmdBackupPlan, flagTvkInstanceUIDFlag, value}
				backupPlanData := runCmdBackupPlan(args)
				Expect(len(backupPlanData)).To(Equal(1))
				Expect(backupPlanData[0].InstanceID).Should(Equal(value))
			}
		})

	})

	FContext("Metadata filtering Operations on different fields of backupplan & backup", func() {
		var (
			backupPlanUIDs          []string
			noOfBackupplansToCreate = 1
			noOfBackupsToCreate     = 1
			once                    sync.Once
			isLast                  bool
		)

		BeforeEach(func() {
			backupIDValue := guid.New().String()
			// once.Do run once for this Context
			once.Do(func() {
				// create target with browsing enabled & create all files & directories required for this Context in NFS server
				// being used by target - only once Before all It in this context
				//createTarget()
				output, _ := exec.Command(createBackupScript, strconv.Itoa(noOfBackupplansToCreate),
					strconv.Itoa(noOfBackupsToCreate), "true", backupIDValue, "helm_backup_type").Output()

				log.Info("Shell Script Output: ", string(output))
				backupPlanUIDs, _ = verifyBackupPlansAndBackupsOnNFS(noOfBackupplansToCreate, noOfBackupplansToCreate*noOfBackupsToCreate)
			})
		})
		AfterEach(func() {
			if isLast {
				// delete target & remove all files & directories created for this Context - only once After all It in this context
				//deleteTarget()
				for _, backupPlans := range backupPlanUIDs {
					_, err := RmRf(targetLocation + "/" + backupPlans)
					Expect(err).To(BeNil())
				}
			}
		})
		It("Should filter metadata on BackupPlan and backup", func() {

			isLast = true
			backupPlanUID := backupPlanUIDs[0]
			args := []string{cmdBackup, flagBackupPlanUIDFlag, backupPlanUID, flagPageSize, "1"}
			backupData := runCmdBackup(args)
			args = []string{cmdMetadata, flagBackupPlanUIDFlag, backupPlanUID, flagBackupUIDFlag, backupData[0].BackupUID}
			cmd := exec.Command("./"+path.Join(cmdPath, binaryName), args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Debugf("Error to execute command %s", err.Error())
			}
			re := regexp.MustCompile("(?m)[\r\n]+^.*location.*$")
			metadata := re.ReplaceAllString(string(output), "")
			jsonFile, err := os.Open("./test-data/metadata-helm.json")
			defer jsonFile.Close()

			Expect(err).To(BeNil())
			expectedMetadata, _ := ioutil.ReadAll(jsonFile)
			Expect(reflect.DeepEqual(expectedMetadata, metadata))
		})
	})

})

//func createTarget() {
//	By("Creating target and marking it available")
//	cmd := fmt.Sprintf("kubect apply -f %s --namespace %s", testDataPath+targetPath, installNs)
//	output, err := RunCmd(cmd)
//	if err != nil {
//		log.Fatalf("target creation failed %s.", err.Error())
//	}
//	fmt.Println(output)
//}
//
//func deleteTarget() {
//	cmd := fmt.Sprintf("kubect delete -f %s --namespace %s", testDataPath+targetPath, installNs)
//	_, err := RunCmd(cmd)
//	if err != nil {
//		log.Fatalf("target deletion failed. %s", err.Error())
//	}
//}

func runCmdBackupPlan(args []string) []backupPlan {
	cmd := exec.Command("./"+path.Join(cmdPath, binaryName), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Debugf("Error to execute command %s", err.Error())
	}
	fmt.Println(string(output))
	var backupPlanData []backupPlan
	err = json.Unmarshal(output, &backupPlanData)
	if err != nil {
		fmt.Println(err.Error())
	}
	return backupPlanData
}

func runCmdBackup(args []string) []backup {
	cmd := exec.Command("./"+path.Join(cmdPath, binaryName), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Debugf("Error to execute command %s", err.Error())
	}
	fmt.Println(string(output))
	var backupData []backup
	err = json.Unmarshal(output, &backupData)
	if err != nil {
		fmt.Println(err.Error())
	}
	return backupData
}
