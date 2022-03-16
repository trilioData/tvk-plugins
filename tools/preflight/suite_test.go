package preflight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	vsnapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	vsnapv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1beta1"
	"github.com/onsi/ginkgo/reporters"
	"github.com/sirupsen/logrus"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal"
	testutils "github.com/trilioData/tvk-plugins/tests/test_utils"
)

const (
	defaultStorageClass      = "csi-gce-pd"
	defaultSnapshotClass     = "default-snapshot-class"
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
	invalidStorageClass      = "invalid-sc"
	invalidSnapshotClass     = "invalid-vssc"
	invalidService           = "invalid-svc"
	invalidNamespace         = "invalid-ns"
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
)

var (
	ctx           = context.Background()
	k8sClient     client.Client
	testEnv       = &envtest.Environment{}
	envTestScheme *runtime.Scheme
	logger        *logrus.Logger
	run           *Run

	cancel     context.CancelFunc
	testScheme *runtime.Scheme
	contClient client.Client
	installNs  = testutils.GetInstallNamespace()

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
	junitReporter := reporters.NewJUnitReporter("junit-preflight-unit-test.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"preflight unit tests",
		[]Reporter{printer.NewlineReporter{}, junitReporter})
}

var _ = BeforeSuite(func() {
	By("Bootstrapping test environment")
	envTestScheme = runtime.NewScheme()
	Expect(apiextensionsv1.AddToScheme(envTestScheme)).To(BeNil())
	Expect(vsnapv1.AddToScheme(envTestScheme)).To(BeNil())
	Expect(vsnapv1beta1.AddToScheme(envTestScheme)).To(BeNil())
	Expect(clientGoScheme.AddToScheme(envTestScheme)).To(BeNil())

	initRunOps()
	err := InitKubeEnv(runOps.Kubeconfig)
	Expect(err).To(BeNil())

	initCleanupOps()

	initServerClients()

	// starting the env cluster
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: envTestScheme,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())
	runtimeClient = k8sClient

	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
})

var _ = AfterSuite(func() {
	cancel()
})

func initRunOps() {
	runOps = Run{
		RunOptions: RunOptions{
			StorageClass:         defaultStorageClass,
			SnapshotClass:        defaultSnapshotClass,
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
}
