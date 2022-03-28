package preflight

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	vsnapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	vsnapv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1beta1"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	goclient "k8s.io/client-go/kubernetes"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	"github.com/trilioData/tvk-plugins/internal"
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
	defaultCleanupGVKListLen = 2

	storageClassGroup = "storage.k8s.io"

	invalidKubectlBinaryName = "invalid_kubectl"
	invalidHelmBinaryName    = "invalid_helm"
	invalidHelmVersion       = "1.0.0"
	invalidK8sVersion        = "99.99.0"
	invalidRBACAPIGroup      = "invalid.rbac.k8s.io"
	invalidSnapshotClass     = "invalid-vssc"
	invalidGroup             = "invalid.group.k8s.io"

	testNameSuffix    = "abcdef"
	testPodPrefix     = "test-pod-"
	testSnapshotClass = "ut-snapshot-class"
	testDriver        = "test.snapshot.driver.io"
	testMinK8sVersion = "1.10.0"
	testContainerName = "test-container"

	installNs = "default"
)

var (
	ctx           = context.Background()
	k8sClient     *goclient.Clientset
	k8sDiscClient *discovery.DiscoveryClient
	k8sManager    ctrl.Manager
	testEnv       = &envtest.Environment{}
	envTestScheme *runtime.Scheme
	logger        *logrus.Logger

	contClient client.Client

	currentDir, _      = os.Getwd()
	projectRoot        = filepath.Dir(filepath.Dir(currentDir))
	testDataDirRelPath = filepath.Join(projectRoot, "tools", "preflight", "test_files")

	nonExistentFile       = "non_exist_file"
	invalidKubeconfigFile = "invalid_kc_file"
	emptyFile             = "empty_file"

	runOps     Run
	cleanupOps Cleanup
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

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: envTestScheme,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient, err = goclient.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())
	k8sDiscClient = k8sClient.DiscoveryClient
	Expect(k8sDiscClient).ToNot(BeNil())
	contClient = k8sManager.GetClient()
	Expect(contClient).ToNot(BeNil())

	initRunOps()
	initCleanupOps()

	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
})

var _ = AfterSuite(func() {
	err := testEnv.Stop()
	Expect(err).To(BeNil())
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
		Logger:     logger,
	}
}
