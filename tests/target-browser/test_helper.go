package targetbrowsertest

import (
	cryptorand "crypto/rand"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
)

const (
	targetLocation = internal.TargetLocation
	timeout        = time.Second * 300
	interval       = time.Second * 1
)

var (
	NfsIPAddress    = GetNFSIPAddr()
	randomDirectory string
)

func GetInstallNamespace() string {
	namespace, present := os.LookupEnv(internal.InstallNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
}
func GetNFSIPAddr() string {
	nfsIPAddr, isNFSIPAddrPresent := os.LookupEnv(internal.NFSServerIPAddress)
	if !isNFSIPAddrPresent {
		panic("NFS IP address not found in environment.")
	}
	return nfsIPAddr
}
func GenerateRandomString(n int, isOnlyAlphabetic bool) string {
	letters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "1234567890"
	rand.Seed(time.Now().UnixNano())
	letterRunes := []rune(letters)
	if !isOnlyAlphabetic {
		letterRunes = []rune(letters + numbers)
	}
	b := make([]rune, n)
	for i := range b {
		randNum, _ := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(letterRunes))))
		b[i] = letterRunes[randNum.Int64()]
	}
	return string(b)
}

func UnMountTarget() {
	c := fmt.Sprintf("sudo umount %s", targetLocation)
	command := exec.Command("bash", "-c", c)
	_, err := command.CombinedOutput()
	if err != nil {
		log.Errorf("error %s", err.Error())
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func makeRandomDirAndMount() {

	var err error
	randomDirectory = GenerateRandomString(5, false)

	// making directory of random name
	err = os.MkdirAll(filepath.Join(targetLocation, randomDirectory), 0777)
	gomega.Expect(err).To(gomega.BeNil())
	_, err = ChmodR(filepath.Join(targetLocation, randomDirectory), "777")
	gomega.Expect(err).To(gomega.BeNil())
	log.Info("directory created:", randomDirectory)

	// unmounting from default target path
	UnMountTarget()
	log.Info("unmounted from default path")

	time.Sleep(time.Second * 10)

	// setting "NFS_SERVER_BASE_PATH" to newly formed directory and mounting to it
	gomega.Expect(os.Setenv(internal.NFSServerBasePath, internal.TargetBrowserDataPath+"/"+randomDirectory)).To(gomega.BeNil())
	MountTarget()
	log.Info("mounted to new path")
	gomega.Expect(err).To(gomega.BeNil())

	time.Sleep(time.Second * 20)
}

func MountTarget() {
	targetBrowserPath := os.Getenv(internal.NFSServerBasePath)
	c := fmt.Sprintf("sudo mount -t nfs -o  nfsvers=4 %s:%s %s", NfsIPAddress, targetBrowserPath, targetLocation)
	fmt.Printf("Mount command %s\n", c)
	command := exec.Command("bash", "-c", c)
	_, err := command.CombinedOutput()
	if err != nil {
		log.Errorf("error %s", err.Error())
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

}

func removeRandomDirAndUnmount() {

	// unmounting from random named directory
	UnMountTarget()
	log.Info("unmounted from random named directory", randomDirectory)

	time.Sleep(time.Second * 10)

	// setting "NFS_SERVER_BASE_PATH" to default target path and mounting to it
	gomega.Expect(os.Setenv(internal.NFSServerBasePath, internal.TargetBrowserDataPath)).To(gomega.BeNil())
	MountTarget()
	log.Info("mounted to default path")

	time.Sleep(time.Second * 10)

	// removing random named directory
	_, _ = RmRf(filepath.Join(targetLocation, randomDirectory))

	log.Info("removed directory")
}

func verifyBackupPlansAndBackupsOnNFS(backupPlans, backups int) (backupPlanUIDs []string) {

	var (
		err                        error
		tempBackupUIDs, backupUIDs []string
	)

	gomega.Eventually(func() []string {
		return gomega.InterceptGomegaFailures(func() {
			backupPlanUIDs, err = ReadChildDir(targetLocation)
			gomega.Expect(err).To(gomega.BeNil())
			log.Info(len(backupPlanUIDs), " backupplans present on target location")
			gomega.Expect(len(backupPlanUIDs)).To(gomega.Equal(backupPlans))
		})
	}, timeout, interval).Should(gomega.BeEmpty())

	gomega.Eventually(func() []string {
		return gomega.InterceptGomegaFailures(func() {
			for i := range backupPlanUIDs {
				tempBackupUIDs, err = ReadChildDir(targetLocation + "/" + backupPlanUIDs[i])
				gomega.Expect(err).To(gomega.BeNil())
				backupUIDs = append(backupUIDs, tempBackupUIDs...)
			}
			log.Info(len(backupUIDs), " backups present on target location")
			gomega.Expect(len(backupUIDs)).To(gomega.Equal(backups))
		})
	}, timeout, interval).Should(gomega.BeEmpty())

	return backupPlanUIDs

}

func VerifyTargetStatus(installNs string) {
	gomega.Eventually(func() bool {
		getTarget := fmt.Sprintf("kubectl get target %s  --namespace %s -o=jsonpath=\"{.items[*]}{.status.status}\"",
			internal.TargetName, installNs)
		cmd := exec.Command("bash", "-c", getTarget)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Errorf("Error to execute command %s", err.Error())
		}
		log.Info("Target status is ", string(output))
		return string(output) == "Available"
	}, timeout, interval).Should(gomega.BeTrue())
}

func GetNFSCredentials() (nfsIPAddr, nfsServerPath string) {
	nfsIPAddr, isNFSIPAddrPresent := os.LookupEnv(internal.NFSServerIPAddress)
	nfsServerPath, isNFSServerPathPresent := os.LookupEnv(internal.NFSServerBasePath)
	if !isNFSIPAddrPresent || !isNFSServerPathPresent {
		panic("NFS Credentials not present in env")
	}

	return nfsIPAddr, nfsServerPath
}

// UpdateYAMLs Update old YAML values with new values
// kv is map of old value to new value
func UpdateYAMLs(kv map[string]string, yamlPath string) error {
	read, readErr := ioutil.ReadFile(yamlPath)
	if readErr != nil {
		return readErr
	}
	updatedFile := string(read)
	for placeholder, value := range kv {
		if strings.Contains(updatedFile, placeholder) && placeholder != "" {
			updatedFile = strings.ReplaceAll(updatedFile, placeholder, value)
			log.Infof("Updated the old value: [%s] with new value: [%s] in file [%s]",
				placeholder, value, yamlPath)
		}
	}

	if writeErr := ioutil.WriteFile(yamlPath, []byte(updatedFile), 0); writeErr != nil {
		return writeErr
	}

	return nil
}
