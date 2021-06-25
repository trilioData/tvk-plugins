package targetbrowsertest

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/cmd/target-browser/cmd"
	"github.com/trilioData/tvk-plugins/internal"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient client.Client
	ctx       = context.Background()
	installNs = GetInstallNamespace()

	controlPlaneDeploymentKey = types.NamespacedName{
		Name:      internal.TVKControlPlaneDeployment,
		Namespace: installNs,
	}

	createBackupScript     = "./createBackups.sh"
	cmdBackupPlan          = cmd.BackupPlanCmdName
	cmdBackup              = cmd.BackupCmdName
	flagOrderBy            = "--" + string(cmd.OrderByFlag)
	cmdMetadata            = cmd.MetadataBinaryName
	flagTvkInstanceUIDFlag = "--" + cmd.TvkInstanceUIDFlag
	flagBackupUIDFlag      = "--" + cmd.BackupUIDFlag
	flagBackupStatus       = "--" + cmd.BackupStatusFlag
	flagBackupPlanUIDFlag  = "--" + cmd.BackupPlanUIDFlag
	flagPageSize           = "--" + cmd.PageSizeFlag
	flagTargetNamespace    = "--" + cmd.TargetNamespaceFlag
	flagTargetName         = "--" + cmd.TargetNameFlag
	flagKubeConfig         = "--" + cmd.KubeConfigFlag
	cmdGet                 = cmd.GetFlag
	testDataDirRelPath     = "./test-data"
	targetPath             = "target.yaml"
	nfsIPAddr              string
	nfsServerPath          string
	currentDir, _          = os.Getwd()
)

func TestTargetBrowser(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("target-browser-junit.xml")
	RunSpecsWithCustomReporters(t, "TargetBrowser Suite", []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	Expect(os.Setenv(internal.NFSServerBasePath, internal.TargetBrowserDataPath)).To(BeNil())

	_, err := Mkdir(internal.TargetLocation)
	Expect(err).Should(BeNil())

	scheme := runtime.NewScheme()
	_ = clientGoScheme.AddToScheme(scheme)
	config := config.GetConfigOrDie()

	k8sClient, _ = client.New(config, client.Options{Scheme: scheme})
	Expect(k8sClient).ToNot(BeNil())
	log.Info("Mounting target.")
	MountTarget()
	changeControlPlanePollingPeriod()
	time.Sleep(time.Second * 10)

	makeRandomDirAndMount()
	nfsIPAddr, nfsServerPath = GetNFSCredentials()
	Expect(UpdateYAMLs(
		map[string]string{
			internal.NFSServerIP:       nfsIPAddr,
			internal.NFSServerBasePath: nfsServerPath,
		}, path.Join(testDataDirRelPath, targetPath))).To(BeNil())

}, 60)

var _ = AfterSuite(func() {
	Expect(UpdateYAMLs(
		map[string]string{
			nfsIPAddr:     internal.NFSServerIP,
			nfsServerPath: internal.NFSServerBasePath,
		}, path.Join(testDataDirRelPath, targetPath))).To(BeNil())
	removeRandomDirAndUnmount()
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
		if containers[index].Name == internal.ControlPlaneContainerName {
			container = &containers[index]
			containerIdx = index
			break
		}
	}
	if container != nil {
		for index := range container.Env {
			if container.Env[index].Name == internal.PollingPeriod {
				container.Env[index].Value = pollingPeriod
				deployment.Spec.Template.Spec.Containers[containerIdx].Env = container.Env
				break
			}
		}
	}
	err = k8sClient.Update(ctx, deployment)
	Expect(err).ShouldNot(HaveOccurred())
}
