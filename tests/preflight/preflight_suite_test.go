package preflighttest

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	client "k8s.io/client-go/kubernetes"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	ctrlRuntime "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	v1APIVersion             = "v1"
	defaultTestStorageClass  = "csi-gce-pd"
	defaultTestNs            = "preflight-test-ns"
	defaultTestSnapshotClass = "default-snapshot-class"
	storageSnapshotGroup     = "snapshot.storage.k8s.io"
	ocpAPIVersion            = "security.openshift.io/v1"

	labelK8sName           = "app.kubernetes.io/name"
	labelK8sNameValue      = "k8s-triliovault"
	labelTrilioKey         = "trilio"
	labelTvkPreflightValue = "tvk-preflight"
	labelPreflightRunKey   = "preflight-run"
	labelK8sPartOf         = "app.kubernetes.io/part-of"
	labelK8sPartOfValue    = "k8s-triliovault"

	gcrRegistryPath       = "gcr.io/kubernetes-e2e-test-images"
	dnsPodNamePrefix      = "test-dns-pod-"
	dnsContainerName      = "test-dnsutils"
	dnsUtilsImage         = "dnsutils:1.3"
	sourcePodNamePrefix   = "source-pod-"
	sourcePVCNamePrefix   = "source-pvc-"
	volSnapshotNamePrefix = "snapshot-source-pvc-"
	busyboxContainerName  = "busybox"
	busyboxImageName      = "busybox"
	volMountName          = "source-data"
	volMountPath          = "/demo/data"
	volSnapPodFilePath    = "/demo/data/sample-file.txt"
	volSnapPodFileData    = "pod preflight data"
	preflightSAName       = "preflight-sa"
	preflightKubeConf     = "preflight_test_config"
	filePermission        = 0644

	letterBytes               = "abcdefghijklmnopqrstuvwxyz"
	deletionGracePeriod int64 = 5

	timeout  = time.Minute * 1
	interval = time.Second * 1
)

var (
	err                  error
	ctx                  = context.Background()
	log                  *logrus.Entry
	storageClassFlag     = "--storage-class"
	snapshotClassFlag    = "--volume-snapshot-class"
	localRegistryFlag    = "--local-registry"
	imagePullSecFlag     = "--image-pull-secret"
	serviceAccountFlag   = "--service-account"
	cleanupOnFailureFlag = "--cleanup-on-failure"
	namespaceFlag        = "--namespace"
	kubeconfigFlag       = "--kubeconfig"
	logLevelFlag         = "--log-level"

	preflightLogFilePrefix    = "preflight-"
	cleanupLogFilePrefix      = "preflight_cleanup-"
	invalidStorageClassName   = "invalid-storage-class"
	invalidSnapshotClassName  = "invalid-snapshot-class"
	invalidLocalRegistryName  = "invalid-local-registry"
	invalidServiceAccountName = "invalid-service-account"
	invalidLogLevel           = "invalidLogLevel"

	kubeConfPath = path.Join(os.Getenv("HOME"), ".kube", "config")

	distDir                 = "dist"
	preflightDir            = "preflight_linux_amd64"
	currentDir, _           = os.Getwd()
	projectRoot             = filepath.Dir(filepath.Dir(currentDir))
	preflightBinaryDir      = filepath.Join(projectRoot, distDir, preflightDir)
	preflightBinaryName     = "preflight"
	preflightBinaryFilePath = filepath.Join(preflightBinaryDir, preflightBinaryName)

	commandSleep3600       = []string{"sleep", "3600"}
	commandBinSh           = []string{"bin/sh", "-c"}
	argsTouchDataFileSleep = []string{
		fmt.Sprintf("echo '%s' > %s && sleep 3000", volSnapPodFileData, volSnapPodFilePath),
	}
	resourceRequirements = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse("64Mi"),
			corev1.ResourceCPU:    resource.MustParse("250m"),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse("128Mi"),
			corev1.ResourceCPU:    resource.MustParse("500m"),
		},
	}

	csiApis = [3]string{
		"volumesnapshotclasses." + storageSnapshotGroup,
		"volumesnapshotcontents." + storageSnapshotGroup,
		"volumesnapshots." + storageSnapshotGroup,
	}

	flagsMap = map[string]string{
		storageClassFlag:     defaultTestStorageClass,
		namespaceFlag:        defaultTestNs,
		cleanupOnFailureFlag: "",
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
	snapshotGVK schema.GroupVersionKind

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
	var (
		kubeconfig string
	)
	fmt.Println("start of before suite")
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	log = logrus.WithFields(logrus.Fields{"namespace": defaultTestNs})

	scheme = runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = clientGoScheme.AddToScheme(scheme)

	kubeconfig, err = internal.NewConfigFromCommandline("")
	Expect(err).To(BeNil())
	kubeAccessor, err = internal.NewEnv(kubeconfig, scheme)
	Expect(err).To(BeNil())
	k8sClient = kubeAccessor.GetClientset()
	runtimeClient = kubeAccessor.GetRuntimeClient()
	discClient = kubeAccessor.GetDiscoveryClient()

	createTestNamespace()

	snapshotGVK = getVolSnapshotGVK()
})

var _ = AfterSuite(func() {
	cleanDirForFiles(preflightLogFilePrefix)
	cleanDirForFiles(cleanupLogFilePrefix)
	deleteTestNamespace()
})

func createTestNamespace() {
	var testNs = &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       internal.NamespaceKind,
			APIVersion: v1APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultTestNs,
		},
	}
	_, err = k8sClient.CoreV1().Namespaces().Create(ctx, testNs, metav1.CreateOptions{})
	Expect(err).To(BeNil())
	log.Infof("Created preflight testing namespace - '%s' successfully\n", defaultTestNs)
}

func deleteTestNamespace() {
	err = k8sClient.CoreV1().Namespaces().Delete(ctx, defaultTestNs, metav1.DeleteOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       internal.NamespaceKind,
			APIVersion: v1APIVersion,
		},
		GracePeriodSeconds: func() *int64 {
			var d = deletionGracePeriod
			return &d
		}(),
	})
	Expect(err).To(BeNil())
	log.Infof("Deleted preflight testing namespace - '%s' successfully\n", defaultTestNs)
}

func getServerPreferredVersionForGroup(grp string) (string, error) {
	var (
		apiResList  *metav1.APIGroupList
		prefVersion string
	)
	apiResList, err = k8sClient.ServerGroups()
	Expect(err).To(BeNil())
	for idx := range apiResList.Groups {
		api := apiResList.Groups[idx]
		if api.Name == grp {
			prefVersion = api.PreferredVersion.Version
			break
		}
	}

	if prefVersion == "" {
		return "", fmt.Errorf("no preferred version for group - %s found on cluster", grp)
	}
	return prefVersion, nil
}

func getVolSnapshotGVK() schema.GroupVersionKind {
	var prefVer string
	prefVer, err = getServerPreferredVersionForGroup(storageSnapshotGroup)
	Expect(err).To(BeNil())
	return schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: prefVer,
		Kind:    internal.VolumeSnapshotKind,
	}
}
