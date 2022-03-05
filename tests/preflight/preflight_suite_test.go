package preflighttest

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/cmd/preflight/cmd"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	client "k8s.io/client-go/kubernetes"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	ctrlRuntime "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	testutils "github.com/trilioData/tvk-plugins/tests/test_utils"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

const (
	defaultTestStorageClass  = "csi-gce-pd"
	defaultTestSnapshotClass = "default-snapshot-class"
	memory256                = "256Mi"
	cpu300                   = "300m"
	cpu400                   = "400m"
	cpu600                   = "600m"
	resourceCPUToken         = "cpu"
	resourceMemoryToken      = "memory"

	dnsPodNamePrefix = "test-dns-pod-"
	dnsContainerName = "test-dnsutils"

	sampleVolSnapClassName = "sample-snap-class"
	invalidVolSnapDriver   = "invalid.csi.k8s.io"
	preflightSAName        = "preflight-sa"
	preflightKubeConf      = "preflight_test_config"
	flagNamespace          = "preflight-flag-ns"
	kubeconfigEnv          = "KUBECONFIG"
	filePermission         = 0644

	timeout        = time.Minute * 1
	interval       = time.Second * 1
	spaceSeparator = " "
)

var (
	err                   error
	cmdOut                *shell.CmdOut
	kubeconfig            string
	ctx                   = context.Background()
	log                   *logrus.Entry
	flagPrefix            = "--"
	storageClassFlag      = flagPrefix + cmd.StorageClassFlag
	snapshotClassFlag     = flagPrefix + cmd.SnapshotClassFlag
	localRegistryFlag     = flagPrefix + cmd.LocalRegistryFlag
	serviceAccountFlag    = flagPrefix + cmd.ServiceAccountFlag
	cleanupOnFailureFlag  = flagPrefix + cmd.CleanupOnFailureFlag
	namespaceFlag         = flagPrefix + cmd.NamespaceFlag
	kubeconfigFlag        = flagPrefix + internal.KubeconfigFlag
	logLevelFlag          = flagPrefix + internal.LogLevelFlag
	configFileFlag        = flagPrefix + cmd.ConfigFileFlag
	pvcStorageRequestFlag = flagPrefix + cmd.PVCStorageRequestFlag
	limitsFlag            = flagPrefix + cmd.PodLimitFlag
	requestsFlag          = flagPrefix + cmd.PodRequestFlag

	preflightLogFilePrefix    = "preflight-"
	cleanupLogFilePrefix      = "preflight_cleanup-"
	invalidStorageClassName   = "invalid-storage-class"
	invalidSnapshotClassName  = "invalid-snapshot-class"
	invalidLocalRegistryName  = "invalid-local-registry"
	invalidServiceAccountName = "invalid-service-account"
	invalidLogLevel           = "invalidLogLevel"
	invalidNamespace          = "invalid-ns"
	invalidKubeConfFilename   = path.Join([]string{".", "invalid_kc_file"}...)
	invalidKubeConfFileData   = "invalid data"
	invalidYamlFilePath       = "invalid_path.yaml"
	invalidKeyYamlFileName    = "invalid_key_file.yaml"
	invalidPVCStorageRequest  = "2Ga"
	defaultTestNs             = testutils.GetInstallNamespace()
	permYamlFile              = "file_permission.yaml"
	cleanupUIDInputYamlFile   = "cleanup_uid_input.yaml"
	cleanupFileInputData      = strings.Join([]string{"cleanup:",
		fmt.Sprintf("  namespace: %s", defaultTestNs), "  logLevel: info"}, "\n")
	cleanupAllInputYamlFile = "cleanup_all_input.yaml"
	kubeConfPath            = os.Getenv(kubeconfigEnv)

	distDir                 = "dist"
	preflightDir            = "preflight_linux_amd64"
	currentDir, _           = os.Getwd()
	projectRoot             = filepath.Dir(filepath.Dir(currentDir))
	preflightBinaryDir      = filepath.Join(projectRoot, distDir, preflightDir)
	preflightBinaryName     = "preflight"
	preflightBinaryFilePath = filepath.Join(preflightBinaryDir, preflightBinaryName)
	testDataDirRelPath      = filepath.Join(projectRoot, "tests", "preflight", "test-data")
	testFileInputName       = "preflight_file_input.yaml"

	resourceReqs = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse(cmd.DefaultPodRequestMemory),
			corev1.ResourceCPU:    resource.MustParse(cmd.DefaultPodRequestCPU),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse(cmd.DefaultPodLimitMemory),
			corev1.ResourceCPU:    resource.MustParse(cmd.DefaultPodLimitCPU),
		},
	}

	flagsMap = map[string]string{
		storageClassFlag:     defaultTestStorageClass,
		namespaceFlag:        defaultTestNs,
		cleanupOnFailureFlag: "",
		kubeconfigFlag:       kubeConfPath,
	}

	podGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.PodKind,
	}
	pvcGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.PersistentVolumeClaimKind,
	}
	snapshotGVK      schema.GroupVersionKind
	snapshotClassGVK schema.GroupVersionKind

	scheme        *runtime.Scheme
	kubeAccessor  *internal.Accessor
	k8sClient     *client.Clientset
	runtimeClient ctrlRuntime.Client
	discClient    *discovery.DiscoveryClient
)

func TestPreflight(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit-preflight.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"Preflight Test Suite",
		[]Reporter{junitReporter},
	)
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	log = logrus.WithFields(logrus.Fields{"namespace": defaultTestNs})

	scheme = runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).ShouldNot(HaveOccurred())
	Expect(appsv1.AddToScheme(scheme)).ShouldNot(HaveOccurred())
	Expect(clientGoScheme.AddToScheme(scheme)).ShouldNot(HaveOccurred())

	kubeconfig, err = internal.NewConfigFromCommandline("")
	Expect(err).To(BeNil())
	kubeAccessor, err = internal.NewAccessor(kubeconfig, scheme)
	Expect(err).To(BeNil())
	k8sClient = kubeAccessor.GetClientset()
	runtimeClient = kubeAccessor.GetRuntimeClient()
	discClient = kubeAccessor.GetDiscoveryClient()

	snapshotGVK = getVolSnapshotGVK()
	snapshotClassGVK = getVolSnapClassGVK()
})

var _ = AfterSuite(func() {
	cmdOut, err = runCleanupForAllPreflightResources()
	log.Infof("Resource cleanup at the end of suitte: %s", cmdOut.Out)
	Expect(err).To(BeNil())
	cleanDirForFiles(preflightLogFilePrefix)
	cleanDirForFiles(cleanupLogFilePrefix)
})

// Deletes all the log files generated at the end of suite
func cleanDirForFiles(filePrefix string) {
	var names []fs.FileInfo
	names, err = ioutil.ReadDir(preflightBinaryDir)
	Expect(err).To(BeNil())
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			err = os.RemoveAll(path.Join([]string{preflightBinaryDir, entry.Name()}...))
			Expect(err).To(BeNil())
		}
	}
}

func getVolSnapshotGVK() schema.GroupVersionKind {
	var prefVer string
	prefVer, err = preflight.GetServerPreferredVersionForGroup(preflight.StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	return schema.GroupVersionKind{
		Group:   preflight.StorageSnapshotGroup,
		Version: prefVer,
		Kind:    internal.VolumeSnapshotKind,
	}
}

func getVolSnapClassGVK() schema.GroupVersionKind {
	var prefVer string
	prefVer, err = preflight.GetServerPreferredVersionForGroup(preflight.StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	return schema.GroupVersionKind{
		Group:   preflight.StorageSnapshotGroup,
		Version: prefVer,
		Kind:    internal.VolumeSnapshotClassKind,
	}
}
