package targetbrowsertest

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

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

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
)

var (
	k8sClient client.Client
	ctx       = context.Background()
	installNs = getInstallNamespace()

	controlPlaneDeploymentKey = types.NamespacedName{
		Name:      internal.TVKControlPlaneDeployment,
		Namespace: installNs,
	}

	createBackupScript = "./createBackups.sh"

	testDataDirRelPath          = "./test-data"
	targetPath                  = "target.yaml"
	nfsIPAddr                   string
	nfsServerPath               string
	currentDir, _               = os.Getwd()
	projectRoot                 = filepath.Dir(filepath.Dir(currentDir))
	targetBrowserBinaryLocation = filepath.Join(projectRoot, distDir, targetBrowserDir)
)

func TestTargetBrowser(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("target-browser-junit.xml")
	RunSpecsWithCustomReporters(t, "TargetBrowser Suite", []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")

	scheme := runtime.NewScheme()
	_ = clientGoScheme.AddToScheme(scheme)
	config := config.GetConfigOrDie()
	var err error
	k8sClient, err = client.New(config, client.Options{Scheme: scheme})
	Expect(err).Should(BeNil())
	Expect(k8sClient).ToNot(BeNil())

	Expect(os.Setenv(nfsServerBasePath, targetBrowserDataPath)).To(BeNil())

	_, err = shell.Mkdir(targetLocation)
	Expect(err).Should(BeNil())

	log.Info("Mounting target.")
	mountTarget()
	changeControlPlanePollingPeriod()
	time.Sleep(time.Second * 10)

	makeRandomDirAndMount()
	nfsIPAddr, nfsServerPath = getNFSCredentials()
	Expect(updateYAMLs(
		map[string]string{
			nfsServerIP:       nfsIPAddr,
			nfsServerBasePath: nfsServerPath,
		}, path.Join(testDataDirRelPath, targetPath))).To(BeNil())

}, 60)

var _ = AfterSuite(func() {
	Expect(updateYAMLs(
		map[string]string{
			nfsIPAddr:     nfsServerIP,
			nfsServerPath: nfsServerBasePath,
		}, path.Join(testDataDirRelPath, targetPath))).To(BeNil())
	removeRandomDirAndUnmount()
})

func changeControlPlanePollingPeriod() {

	var (
		container    *corev1.Container
		containerIdx int
		// setting polling period to update browser cache to 10 seconds
		pollingPeriodTime = "10s"
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
			if container.Env[index].Name == pollingPeriod {
				container.Env[index].Value = pollingPeriodTime
				deployment.Spec.Template.Spec.Containers[containerIdx].Env = container.Env
				break
			}
		}
	}

	Eventually(func() error {
		err = k8sClient.Update(ctx, deployment)
		return err
	}, timeout, interval).Should(BeNil())

	Expect(err).ShouldNot(HaveOccurred())
}
