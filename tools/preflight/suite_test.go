package preflight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/ginkgo/reporters"
	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
	testutils "github.com/trilioData/tvk-plugins/tests/test_utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	goclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	//defaultStorageClass      = "csi-gce-pd"
	//defaultSnapshotClass     = "default-snapshot-class"
	defaultPVCStorageRequest = "1Gi"
	defaultPodCPURequest     = "250m"
	defaultPodMemoryRequest  = "64Mi"
	defaultPodCPULimit       = "500m"
	defaultPodMemoryLimit    = "128Mi"
	defaultLogLevel          = "info"

	storageClassGroup = "storage.k8s.io"

	invalidKubectlBinaryName = "invalid_kubectl"
	invalidHelmBinaryName    = "invalid_helm"
	invalidHelmVersion       = "1.0.0"
	invalidK8sVersion        = "99.99.0"
	invalidRBACAPIGroup      = "invalid.rbac.k8s.io"
	invalidService           = "invalid-svc"
	invalidPVC               = "invalid-pvc"
	invalidGroup             = "invalid.group.k8s.io"

	testNameSuffix       = "abcdef"
	testPodPrefix        = "test-pod-"
	testPVCPrefix        = "test-pvc-"
	testSnapPrefix       = "test-snap-"
	testStorageClass     = "unit-test-sc"
	testProvisioner      = "test.csi.k8s.io"
	testSnapshotClass    = "ut-snapshot-class"
	testDriver           = "test.snapshot.driver.io"
	testService          = "unit-test-svc"
	testContainerName    = "test-container"
	reclaimDelete        = "Delete"
	bindingModeImmediate = "Immediate"

	timeout  = time.Minute * 1
	interval = time.Second * 1
)

var (
	ctx              context.Context
	cancel           context.CancelFunc
	testScheme       *runtime.Scheme
	k8sClient        *goclient.Clientset
	contClient       client.Client
	restConfigClient *restclient.Config
	installNs        = testutils.GetInstallNamespace()

	currentDir, _      = os.Getwd()
	projectRoot        = filepath.Dir(filepath.Dir(currentDir))
	testDataDirRelPath = filepath.Join(projectRoot, "tools", "preflight", "test_files")

	nonExistentFile       = "non_exist_file"
	invalidKubeconfigFile = "invalid_kc_file"
	emptyFile             = "empty_file"

	runOps     Run
	cleanupOps Cleanup

	execTestServiceCmd = []string{"nslookup", fmt.Sprintf("%s.%s", testService, installNs)}
)

func TestPreflight(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit-preflight-unit-tests.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"Preflight Suite",
		[]Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	initRunOps()
	err := InitKubeEnv(runOps.Kubeconfig)
	Expect(err).To(BeNil())

	initCleanupOps()

	initServerClients()
})

var _ = AfterSuite(func() {
	cancel()
})

func initRunOps() {
	runOps = Run{
		RunOptions: RunOptions{
			StorageClass:         internal.DefaultTestStorageClass,
			SnapshotClass:        internal.DefaultTestSnapshotClass,
			PerformCleanupOnFail: true,
			PVCStorageRequest:    resource.MustParse(defaultPVCStorageRequest),
			ResourceRequirements: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(defaultPodCPULimit),
					corev1.ResourceMemory: resource.MustParse(defaultPodMemoryLimit),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(defaultPodCPURequest),
					corev1.ResourceMemory: resource.MustParse(defaultPodMemoryRequest),
				},
			},
		},
		CommonOptions: getTestCommonOps(),
	}
}

func initCleanupOps() {
	cleanupOps = Cleanup{
		CleanupOptions: CleanupOptions{
			UID: "",
		},
		CommonOptions: getTestCommonOps(),
	}
}

func getTestCommonOps() CommonOptions {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	return CommonOptions{
		Kubeconfig: os.Getenv(internal.KubeconfigEnv),
		Namespace:  installNs,
		LogLevel:   defaultLogLevel,
		//Logger:     &logrus.Logger{Out: colorable.NewColorableStdout()},
		Logger: logger,
	}
}

func initServerClients() {
	testScheme = runtime.NewScheme()
	kubeconfig := os.Getenv(internal.KubeconfigEnv)

	utilruntime.Must(corev1.AddToScheme(testScheme))
	kubeEnv, err := internal.NewEnv(kubeconfig, testScheme)
	Expect(err).To(BeNil())
	k8sClient = kubeEnv.GetClientset()
	contClient = kubeEnv.GetRuntimeClient()
	restConfigClient = kubeEnv.GetRestConfig()
}
