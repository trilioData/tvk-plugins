package targetbrowser

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/types"
	client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"math/big"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	ctrlRuntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
	"time"
)

const (
	TrilioSecName         = "trilio-secret"
	NFSServerIPAddress    = "NFS_SERVER_IP_ADDR"
	NFSServerBasePath     = "NFS_SERVER_BASE_PATH"
	NFSServerOptions      = "NFS_SERVER_OPTIONS"
	targetBrowserDataPath = "/src/nfs/ajay" // "/src/nfs/targetbrowsertesting"
	Py3Path               = "/usr/bin/python3"
	InstallNamespace      = "INSTALL_NAMESPACE"
	TargetName            = "target-sample"

	TVKControlPlaneDeployment = "k8s-triliovault-control-plane"
	targetLocation            = "/triliodata"

	PollingPeriod               = "POLLING_PERIOD"
	DataStoreAttacherPath       = "/triliodata"
	TempDataStoreBasePath       = "/triliodata-temp"
	DataStoreAttacherSecretPath = "/etc/secret"
	timeout                     = time.Second * 300
	interval                    = time.Second * 1
)

var (
	randomDirectory string
	targetKey       = types.NamespacedName{
		Name:      TargetName,
		Namespace: "installNs",
	}
)

func GetInstallNamespace() string {
	namespace, present := os.LookupEnv(InstallNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
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

func GetUniqueID(suiteName string) string {
	return suiteName + "-" + GenerateRandomString(4, true)
}
func UnMountTarget(mountpoint, locationToDataAttacher string) {
	if locationToDataAttacher == "" {
		locationToDataAttacher = "../../../../../"
	}

	dataAttacherCommand := fmt.Sprintf("%s "+locationToDataAttacher+"datastore-attacher/mount_utility/unmount_datastore/unmount_datastore.py "+
		"--mountpoint=%s", Py3Path, mountpoint)
	log.Infof("Running command: %s", dataAttacherCommand)
	out, cmdErr := RunCmd(dataAttacherCommand)
	log.Info(out.Out)
	Expect(cmdErr).Should(BeNil())
	log.Info("Target Unmounted")
}

func makeRandomDirAndMount() {

	var err error
	randomDirectory = GenerateRandomString(5, false)

	// making directory of random name
	err = os.MkdirAll(filepath.Join(targetLocation, randomDirectory), 0777)
	Expect(err).To(BeNil())
	_, err = ChmodR(filepath.Join(targetLocation, randomDirectory), "777")
	Expect(err).To(BeNil())
	log.Info("directory created:", randomDirectory)

	// unmounting from default target path
	UnMountTarget(targetLocation, "../../../")
	log.Info("unmounted from default path")

	time.Sleep(time.Second * 10)

	// setting "NFS_SERVER_BASE_PATH" to newly formed directory and mounting to it
	Expect(os.Setenv(NFSServerBasePath, targetBrowserDataPath+"/"+randomDirectory)).To(BeNil())
	MountTarget(targetKey.Name, "../../../")
	log.Info("mounted to new path")
	Expect(err).To(BeNil())

	time.Sleep(time.Second * 20)
}
func MarshalStruct(v interface{}, isYaml bool) string {

	var fstring []byte
	var err error
	if isYaml {
		fstring, err = yaml.Marshal(v)
	} else {
		fstring, err = json.Marshal(v)
	}

	if err != nil {
		panic("Error while marshaling")
	}
	return string(fstring)
}

func GetNFSCredentials() (nfsIPAddr, nfsServerPath, nfsOptions string) {
	nfsIPAddr, isNFSIPAddrPresent := os.LookupEnv(NFSServerIPAddress)
	nfsServerPath, isNFSServerPathPresent := os.LookupEnv(NFSServerBasePath)
	nfsOptions, isNFSOptionsPresent := os.LookupEnv(NFSServerOptions)
	if !isNFSIPAddrPresent || !isNFSServerPathPresent || !isNFSOptionsPresent {
		panic("NFS Credentials not present in env")
	}

	return nfsIPAddr, nfsServerPath, nfsOptions
}

// TODO: Make this as generic solution for multiple targets and of different types
func CreateTargetSecret(targetName string) string {

	nfsIPAddr, nfsServerPath, nfsOptions := GetNFSCredentials()

	nfsMetadata := map[string]interface{}{
		"mountOptions": nfsOptions,
		"server":       nfsIPAddr,
		"share":        nfsServerPath,
	}

	nfsdatastore := map[string]interface{}{
		"name":             targetName,
		"storageType":      "nfs",
		"defaultDatastore": "yes",
		"metaData":         nfsMetadata,
	}

	datastore := []map[string]interface{}{nfsdatastore}

	return MarshalStruct(map[string]interface{}{"datastore": datastore}, true)
}

func MountTarget(targetName, locationToDataAttacher string) {

	if locationToDataAttacher == "" {
		locationToDataAttacher = "../../../../../"
	}

	log.Info("Creating Secret for data attacher")
	secret := CreateTargetSecret(targetName)

	out, err := Mkdir(DataStoreAttacherSecretPath)
	log.Info(out)
	Expect(err).Should(BeNil())

	secretFilePath := path.Join(DataStoreAttacherSecretPath, TrilioSecName)
	err = WriteToFile(secretFilePath, secret)
	Expect(err).Should(BeNil())

	log.Info("Mounting target")
	log.Infof("Creating data store base directory")
	_, err = Mkdir(DataStoreAttacherPath)
	Expect(err).Should(BeNil())
	log.Infof("Creating temp datastore directory for objectstore")
	_, err = Mkdir(TempDataStoreBasePath)
	Expect(err).Should(BeNil())
	//var cmdErr error
	go func() {
		defer ginkgo.GinkgoRecover()
		dataAttacherCommand := fmt.Sprintf("%s "+locationToDataAttacher+"datastore-attacher/mount_utility/mount_by_secret/mount_datastores.py "+
			"--target-name=%s", Py3Path, targetName)
		log.Infof("Running command: %s", dataAttacherCommand)

		output, cmdErr := RunCmd(dataAttacherCommand)
		log.Info(output)
		Expect(cmdErr).Should(BeNil())
		log.Info("Target mounted")
	}()
}

func removeRandomDirAndUnmount() {

	// unmounting from random named directory
	UnMountTarget(targetLocation, "../../../")
	log.Info("unmounted from random named directory")

	time.Sleep(time.Second * 10)

	// setting "NFS_SERVER_BASE_PATH" to default target path and mounting to it
	Expect(os.Setenv(NFSServerBasePath, targetBrowserDataPath)).To(BeNil())
	MountTarget(targetKey.Name, "../../../")
	log.Info("mounted to default path")

	time.Sleep(time.Second * 10)

	// removing random named directory
	RmRf(filepath.Join(targetLocation, randomDirectory))
	log.Info("removed directory")
}

// Accessor is a helper for accessing Kubernetes programmatically. It bundles some of the high-level
// operations that is frequently used by the test framework.
type Accessor struct {
	restConfig *rest.Config
	ctl        *kubectl
	set        *client.Clientset
	client     ctrlRuntime.Client
	context    context.Context
}

func verifyBackupPlansAndBackupsOnNFS(backupPlans, backups int) (backupPlanUIDs, backupUIDs []string) {

	var (
		err            error
		tempBackupUIDs []string
	)

	Eventually(func() []string {
		return InterceptGomegaFailures(func() {
			backupPlanUIDs, err = ReadChildDir(targetLocation)
			Expect(err).To(BeNil())
			fmt.Println(backupPlanUIDs)
			log.Info(len(backupPlanUIDs), " backupplans present on target location")
			Expect(len(backupPlanUIDs)).To(Equal(backupPlans))
		})
	}, timeout, interval).Should(BeEmpty())

	Eventually(func() []string {
		return InterceptGomegaFailures(func() {
			for i := range backupPlanUIDs {
				tempBackupUIDs, err = ReadChildDir(targetLocation + "/" + backupPlanUIDs[i])
				Expect(err).To(BeNil())
				backupUIDs = append(backupUIDs, tempBackupUIDs...)
			}
			log.Info(len(backupUIDs), " backups present on target location")
			Expect(len(backupUIDs)).To(Equal(backups))
		})
	}, timeout, interval).Should(BeEmpty())

	return backupPlanUIDs, backupUIDs

}

func ReadChildDir(dirPath string) (dirNames []string, err error) {
	// ReadChildDir reads the dir name from the given path
	// Input:
	//		dirPath: Directory path
	// Output:
	//		dirNames: Directory name list
	//		err: Error

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return dirNames, err
	}
	for _, file := range files {
		if file.IsDir() {
			dirNames = append(dirNames, file.Name())
		}
	}

	return dirNames, nil
}
