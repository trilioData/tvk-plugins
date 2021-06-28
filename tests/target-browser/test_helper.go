package targetbrowsertest

import (
	"context"
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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
)

const (
	timeout         = time.Second * 300
	interval        = time.Second * 1
	apiRetryTimeout = time.Second * 5
)

var (
	NfsIPAddress    = getNFSIPAddr()
	randomDirectory string
)

func getInstallNamespace() string {
	namespace, present := os.LookupEnv(installNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
}
func getNFSIPAddr() string {
	nfsIPAddr, isNFSIPAddrPresent := os.LookupEnv(nfsServerIP)
	if !isNFSIPAddrPresent {
		panic("NFS IP address not found in environment.")
	}
	return nfsIPAddr
}
func generateRandomString(n int, isOnlyAlphabetic bool) string {
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

func unMountTarget() {
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
	randomDirectory = generateRandomString(5, false)

	// making directory of random name
	err = os.MkdirAll(filepath.Join(targetLocation, randomDirectory), 0777)
	gomega.Expect(err).To(gomega.BeNil())
	_, err = shell.ChmodR(filepath.Join(targetLocation, randomDirectory), "777")
	gomega.Expect(err).To(gomega.BeNil())
	log.Info("directory created:", randomDirectory)

	// unmounting from default target path
	unMountTarget()
	log.Info("unmounted from default path")

	time.Sleep(time.Second * 10)

	// setting "NFS_SERVER_BASE_PATH" to newly formed directory and mounting to it
	gomega.Expect(os.Setenv(nfsServerBasePath, targetBrowserDataPath+"/"+randomDirectory)).To(gomega.BeNil())
	mountTarget()
	log.Info("mounted to new path")
	gomega.Expect(err).To(gomega.BeNil())

	time.Sleep(time.Second * 20)
}

func mountTarget() {
	targetBrowserPath := os.Getenv(nfsServerBasePath)
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
	unMountTarget()
	log.Info("unmounted from random named directory", randomDirectory)

	time.Sleep(time.Second * 10)

	// setting "NFS_SERVER_BASE_PATH" to default target path and mounting to it
	gomega.Expect(os.Setenv(nfsServerBasePath, targetBrowserDataPath)).To(gomega.BeNil())
	mountTarget()
	log.Info("mounted to default path")

	time.Sleep(time.Second * 10)

	// removing random named directory
	_, err := shell.RmRf(filepath.Join(targetLocation, randomDirectory))
	gomega.Expect(err).To(gomega.BeNil())
	log.Info("removed directory")
}

func verifyBackupPlansAndBackupsOnNFS(backupPlans, backups int) (backupPlanUIDs []string) {

	var (
		err                        error
		tempBackupUIDs, backupUIDs []string
	)

	gomega.Eventually(func() []string {
		return gomega.InterceptGomegaFailures(func() {
			backupPlanUIDs, err = shell.ReadChildDir(targetLocation)
			gomega.Expect(err).To(gomega.BeNil())
			log.Info(len(backupPlanUIDs), " backupplans present on target location")
			gomega.Expect(len(backupPlanUIDs)).To(gomega.Equal(backupPlans))
		})
	}, timeout, interval).Should(gomega.BeEmpty())

	gomega.Eventually(func() []string {
		return gomega.InterceptGomegaFailures(func() {
			for i := range backupPlanUIDs {
				tempBackupUIDs, err = shell.ReadChildDir(targetLocation + "/" + backupPlanUIDs[i])
				gomega.Expect(err).To(gomega.BeNil())
				backupUIDs = append(backupUIDs, tempBackupUIDs...)
			}
			log.Info(len(backupUIDs), " backups present on target location")
			gomega.Expect(len(backupUIDs)).To(gomega.Equal(backups))
		})
	}, timeout, interval).Should(gomega.BeEmpty())

	return backupPlanUIDs

}

func verifyTargetStatus(ctx context.Context, installNs string, cl client.Client) {
	// get target
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   internal.TriliovaultGroup,
		Version: internal.V1Version,
		Kind:    internal.TargetKind,
	})
	gomega.Eventually(func() bool {
		err := cl.Get(ctx, types.NamespacedName{Namespace: installNs, Name: targetName},
			target)
		gomega.Expect(err).To(gomega.BeNil())
		targetStatus, _, err := unstructured.NestedString(target.Object, "status", "status")
		gomega.Expect(err).To(gomega.BeNil())
		return targetStatus == "Available"

	}, timeout, interval).Should(gomega.BeTrue())
}

func getNFSCredentials() (nfsIPAddr, nfsServerPath string) {
	nfsIPAddr, isNFSIPAddrPresent := os.LookupEnv(nfsServerIP)
	nfsServerPath, isNFSServerPathPresent := os.LookupEnv(nfsServerBasePath)
	if !isNFSIPAddrPresent || !isNFSServerPathPresent {
		panic("NFS Credentials not present in env")
	}

	return nfsIPAddr, nfsServerPath
}

// updateYAMLs Update old YAML values with new values
// kv is map of old value to new value
func updateYAMLs(kv map[string]string, yamlPath string) error {
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
