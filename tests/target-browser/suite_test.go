package targetbrowser

import (
	"context"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/cmd/target-browser/cmd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
	"time"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient client.Client
	ctx       = context.Background()
	installNs = "pankaj-tb" //GetInstallNamespace()
	apiURL    string

	controlPlaneDeploymentKey = types.NamespacedName{
		Name:      TVKControlPlaneDeployment,
		Namespace: installNs,
	}
	controlPlaneContainerName = "triliovault-control-plane"
	createBackupScript        = "./createBackups.sh"
	binaryName                = "main"
	cmdBackupPlan             = cmd.BackupPlanBinaryName
	cmdPath                   = "../../cmd/target-browser"
	cmdBackup                 = cmd.BackupBinaryName
	flagOrderBy               = "--" + string(cmd.OrderingFlag)
	cmdMetadata               = cmd.MetadataBinaryName
	flagTvkInstanceUIDFlag    = "--" + cmd.TvkInstanceUIDFlag
	flagBackupUIDFlag         = "--" + cmd.BackupUIDFlag
	flagBackupStatus          = "--" + cmd.BackupStatusFlag
	flagBackupPlanUIDFlag     = "--" + cmd.BackupPlanUIDFlag
	flagPageSize              = "--" + cmd.PageSizeFlag
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("target-browser.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"Target Browser Suite",
		[]Reporter{printer.NewlineReporter{}, junitReporter})
}

var _ = BeforeSuite(func() {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	//Expect(os.Setenv(NFSServerBasePath, targetBrowserDataPath)).To(BeNil())

	By("bootstrapping test environment")
	scheme := runtime.NewScheme()
	_ = clientGoScheme.AddToScheme(scheme)
	config := config.GetConfigOrDie()

	k8sClient, _ = client.New(config, client.Options{Scheme: scheme})
	Expect(k8sClient).ToNot(BeNil())

	apiURL = ""
	log.Infof("API URL: %s", apiURL)

	changeControlPlanePollingPeriod()

	//valcommon.MountTarget(targetKey.Name, "../../../")

	time.Sleep(time.Second * 10)
	//makeRandomDirAndMount()
}, 60)

var _ = AfterSuite(func() {
	//removeRandomDirAndUnmount()
})

func changeControlPlanePollingPeriod() {

	var (
		container    *corev1.Container
		containerIdx int
		// setting polling period to update browser cache to 10 seconds
		pollingPeriod = "10s"
	)

	By("Getting Control Plane Deployment")
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, controlPlaneDeploymentKey, deployment)
	Expect(err).To(BeNil())
	containers := deployment.Spec.Template.Spec.Containers
	for index := range containers {
		if containers[index].Name == controlPlaneContainerName {
			container = &containers[index]
			containerIdx = index
			break
		}
	}
	if container != nil {
		for index := range container.Env {
			if container.Env[index].Name == PollingPeriod {
				container.Env[index].Value = pollingPeriod
				deployment.Spec.Template.Spec.Containers[containerIdx].Env = container.Env
				break
			}
		}
	}
	err = k8sClient.Update(ctx, deployment)
	Expect(err).ShouldNot(HaveOccurred())
}
