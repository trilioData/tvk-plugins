/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logcollectortest

import (
	"context"
	"fmt"

	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	utilretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/tests/integration/common"
	com "github.com/trilioData/tvk-plugins/internal/common"
	"github.com/trilioData/tvk-plugins/internal/logprinter"
	"github.com/trilioData/tvk-plugins/internal/shell"
	"github.com/trilioData/tvk-plugins/tests/helper/kube"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	log          *logrus.Entry
	KubeAccessor *kube.Accessor
	k8sClient    client.Client
	scheme       *runtime.Scheme
	ctx          = context.Background()

	namespace        = os.Getenv("INSTALL_NAMESPACE")
	backupNamespace  = os.Getenv("BACKUP_NAMESPACE")
	restoreNamespace = os.Getenv("RESTORE_NAMESPACE")

	skipCleanup = os.Getenv(common.SkipCleanup)

	pvcName = "pod-raw-pvc"
	podName = "pod-raw"

	sampleTargetName  = "sample-target"
	samplePolicyName  = "sample-policy"
	sampleBackupName  = "sample-backup"
	sampleRestoreName = "sample-restore"

	targetBrowserLabel   = "sample-target-browser"
	targetValidatorLabel = "sample-target-validator"
	trilioWebLabel       = "k8s-triliovault-web"
	trilioBackendLabel   = "k8s-triliovault-backend"
	trilioWebSvcLabel    = "k8s-triliovault-web-svc"
	trilioBackedSvcLabel = "k8s-triliovault-backend-svc"

	uniqueID = "log-collector"

	currentDir, _ = os.Getwd()
	projectRoot   = filepath.Dir(filepath.Dir(currentDir))

	testYamls = "tests/test-data/test-yamls"

	mySQLOperatorCRFile     = filepath.Join(projectRoot, "tests/test-data/mysql-operator/mysqlCluster.yaml")
	mySQLOperatorSecretFile = filepath.Join(projectRoot, "tests/test-data/mysql-operator/mysqlCluster-secret.yaml")

	mySQLOperatorScript = filepath.Join(projectRoot, "tests/test-data/mysql-operator/mysqlOperator.sh")
	mySQLCRDFile        = filepath.Join(projectRoot, "tests/test-data/mysql-operator/crd.yaml")
	mySQLCRFile         = filepath.Join(projectRoot, "tests/test-data/mysql-operator/mysqlCluster.yaml")
	mySQLCRSecFile      = filepath.Join(projectRoot, "tests/test-data/mysql-operator/mysqlCluster-secret.yaml")

	customAppFile         = "bplan_with_custom.yaml"
	customOperatorAppFile = "bplan_custom_and_operator.yaml"

	controlPlaneDeploymentKey types.NamespacedName

	logCollectorPrefix = "triliovault-"

	logCollectorFilePath = "tools/log-collector/log_collector.go"

	testDataDir      = "tests"
	allCleanupScript = "cleanup.sh"

	uniqueMySQLOperator = uniqueID + "-" + "mysql-operator"

	customBPlan         = "sample-application-custom"
	customOperatorBPlan = "sample-application-custom-operator"

	customAvailableBackup         = "sample-backup-custom"
	customOperatorAvailableBackup = "sample-backup-custom-operator"

	customOperatorFailedBackup = "sample-backup-custom-op-failed"

	timeout    = time.Second * 130
	interval   = time.Second * 1
	installArg = "install"
	deleteArg  = "delete"
	Space      = " "
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"Log Collector Suite",
		[]Reporter{printer.NewlineReporter{}, junitReporter})
}

var _ = BeforeSuite(func() {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	log = logrus.WithFields(logrus.Fields{"namespace": namespace})

	By("bootstrapping test environment")
	scheme = runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = clientGoScheme.AddToScheme(scheme)

	KubeAccessor, _ = kube.NewEnv(scheme)
	restConf := ctrl.GetConfigOrDie()
	var err error
	k8sClient, err = client.New(restConf, client.Options{Scheme: scheme})
	Expect(err).To(BeNil())

	controlPlaneDeploymentKey = types.NamespacedName{
		Name:      common.TVControlPlaneDeployment,
		Namespace: namespace,
	}

	stopControlPlane()
	//cleanup()

	// set MySqlOperator CR name
	Expect(common.UpdateYAMLs(map[string]string{common.UniqueID: uniqueID}, mySQLOperatorCRFile)).To(BeNil())
	// set MySqlOperator secret name
	Expect(common.UpdateYAMLs(map[string]string{common.UniqueID: uniqueID}, mySQLOperatorSecretFile)).To(BeNil())

	deployCustomApp()
	InstallMysqlOperator(backupNamespace, uniqueMySQLOperator)

	startControlPlane()
	setupForApplication()

	license, err := com.SetupLicense(ctx, k8sClient, namespace, projectRoot)
	Expect(err).ShouldNot(HaveOccurred())
	common.WaitForLicenseToState(ctx, k8sClient, types.NamespacedName{Name: license.Name, Namespace: license.Namespace},
		v1.LicenseActive)

	log.Info("Creating application")
	createApplication(customAppFile, customBPlan)
	createApplication(customOperatorAppFile, customOperatorBPlan)

	log.Info("Creating custom operator backup")
	createBackupWithApp(customOperatorBPlan, customOperatorAvailableBackup, false)
	waitForBackup(customOperatorAvailableBackup, namespace, v1.Available)

	log.Info("Creating custom backup")
	createBackupWithApp(customBPlan, customAvailableBackup, false)
	waitForBackup(customAvailableBackup, namespace, v1.Available)

}, 60)

var _ = AfterSuite(func() {
	if CurrentGinkgoTestDescription().Failed {
		logprinter.PrintDebugLogs()
	}

	log.Info("Deleting installed application")
	deleteCustomApp()
	DeleteMysqlOperator(backupNamespace, uniqueMySQLOperator)
	//
	//// reset MySqlOperator CR name
	Expect(common.UpdateYAMLs(map[string]string{uniqueID: common.UniqueID}, mySQLOperatorCRFile)).To(BeNil())
	//// reset MySqlOperator secret name
	Expect(common.UpdateYAMLs(map[string]string{uniqueID: common.UniqueID}, mySQLOperatorSecretFile)).To(BeNil())
	//
	log.Info("Deleting backup")
	deleteBackup(customOperatorAvailableBackup)
	deleteBackup(customAvailableBackup)

	log.Info("Deleting application")
	deleteApplication(customOperatorBPlan)
	deleteApplication(customBPlan)

	teardownForApplication()
	Expect(common.TearDownLicense(ctx, k8sClient, namespace)).ShouldNot(HaveOccurred())
	cleanup()
	startControlPlane()
})

func setupForApplication() {
	createTarget(sampleTargetName)
	createPolicy(samplePolicyName)
}

func teardownForApplication() {
	deleteTarget(sampleTargetName)
	deletePolicy(samplePolicyName)
}

func startControlPlane() {
	By("Start Control Plane deployment")
	log.Infof("Starting control plane deployment")
	err := common.ScaleDeployment(k8sClient, controlPlaneDeploymentKey, 1)
	Expect(err).To(BeNil())
}

func stopControlPlane() {
	By("Stop Control Plane deployment")
	log.Infof("Stopping control plane deployment")
	err := common.ScaleDeployment(k8sClient, controlPlaneDeploymentKey, 0)
	Expect(err).To(BeNil())
}

func cleanup() {
	By("tearing down the test environment")
	if skipCleanup != common.True {
		log.Infof("Cleaning up everything before tearing down suite from %s namespace", backupNamespace)
		k := fmt.Sprintf("%s %s", filepath.Join(projectRoot, testDataDir, allCleanupScript), backupNamespace)
		log.Info(k)
		_, err := shell.RunCmd(fmt.Sprintf("%s %s", filepath.Join(projectRoot, testDataDir, allCleanupScript),
			backupNamespace))
		Expect(err).To(BeNil())
		log.Infof("Cleaning up everything before tearing down suite from %s namespace", restoreNamespace)
		_, err = shell.RunCmd(fmt.Sprintf("%s %s", filepath.Join(projectRoot, testDataDir, allCleanupScript),
			restoreNamespace))
		Expect(err).To(BeNil())
	}
}

func deployCustomApp() {
	pvc := common.CreatePVC(backupNamespace, false, 100, nil, true)
	pvc.SetName(pvcName)
	err := KubeAccessor.CreatePersistentVolumeClaim(backupNamespace, pvc)
	Expect(err).Should(BeNil())

	container := common.CreateDataInjectionContainer(pvc, false)
	injectorPod := common.CreatePod(backupNamespace, com.VolumeDeviceName, container, corev1.RestartPolicyOnFailure, pvc)
	injectorPod.SetName(podName)
	injectorPod.Spec.ServiceAccountName = ""
	injectorPod.SetLabels(map[string]string{"triliobackupall": "all"})
	err = KubeAccessor.CreatePod(backupNamespace, injectorPod)
	Expect(err).Should(BeNil())
}

func deleteCustomApp() {
	err := KubeAccessor.DeletePod(namespace, podName)
	Expect(err).ShouldNot(HaveOccurred())

	err = KubeAccessor.DeletePersistentVolumeClaim(namespace, pvcName)
	Expect(err).ShouldNot(HaveOccurred())
}

func GetRestoreJobLabels(restore *v1.Restore) map[string]string {
	labels := map[string]string{com.ControllerOwnerUID: string(restore.GetUID())}

	return labels
}

func WaitForRestoreToDelete(acc *kube.Accessor, restoreName, ns string) {
	Eventually(func() bool {
		_, err := acc.GetRestore(restoreName, ns)
		if err != nil && apierrors.IsNotFound(err) {
			return true
		}
		if err == nil {
			_ = acc.DeleteRestore(types.NamespacedName{Name: restoreName, Namespace: ns})
		}
		return false
	}, "120s", "2s").Should(BeTrue())
}

func SetBackupPlanStatus(KubeAccessor *kube.Accessor, appName, namespace string, reqStatus v1.Status) error {
	var appCr *v1.BackupPlan
	var err error
	log.Infof("Updating %s status to %s", appName, reqStatus)
	Eventually(func() error {
		retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {

			appCr, err = KubeAccessor.GetBackupPlan(appName, namespace)
			if err != nil {
				log.Errorf(err.Error())
				return err
			}
			log.Infof("requested backupPlan status %v, actual status: %v",
				reqStatus, appCr.Status.Status)
			appCr.Status.Status = reqStatus
			err = KubeAccessor.StatusUpdate(appCr)
			if err != nil {
				log.Errorf("Failed to update application status:%+v", err)
				return err
			}

			return nil
		})
		appCr, err = KubeAccessor.GetBackupPlan(appName, namespace)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
		if appCr.Status.Status != reqStatus {
			log.Errorf("failed to update backupplan status reqStatus: %v, "+
				"actualStatus: %v", reqStatus, appCr.Status.Status)
			return fmt.Errorf("failed to update backupplan status")
		}
		log.Infof("Updated %s  requestedStatus %v to %s", appName,
			reqStatus, appCr.Status.Status)
		return retErr
	}, timeout, interval).ShouldNot(HaveOccurred())

	return nil
}

func InstallMysqlOperator(ns, releaseName string) {
	log.Infof("Installing mysql operator: [%s]", releaseName)
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(mySQLOperatorScript), installArg, releaseName, ns}, Space))
	log.Info(out.Out)
	Expect(err).To(BeNil())

	// install MysqlCluster crd, it will be required
	Expect(KubeAccessor.Apply(ns, filepath.Join(mySQLCRDFile))).NotTo(HaveOccurred())
	Expect(KubeAccessor.Apply(ns, mySQLCRFile)).To(BeNil())
	Expect(KubeAccessor.Apply(ns, mySQLCRSecFile)).To(BeNil())

	time.Sleep(30 * time.Second) // -> giving it 30 sec to start all pods

	Eventually(func() error {
		_, err = KubeAccessor.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = KubeAccessor.GetPods(ns, "app.kubernetes.io/managed-by=mysql.presslabs.org", "app.kubernetes.io/name=mysql")
			otherPods, _ := KubeAccessor.GetPods(ns, "app=mysql-operator", fmt.Sprintf("release=%s", releaseName))
			pods = append(pods, otherPods...)
			return pods, lErr
		})
		return err
	}, common.ResourceDeploymentTimeout, common.ResourceDeploymentInterval).Should(BeNil())
	log.Info("Installed mysql operator")

}

func DeleteMysqlOperator(ns, operatorName string) {
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(mySQLOperatorScript), deleteArg, operatorName, ns}, Space))
	log.Info(out.Out)

	if err != nil {
		log.Errorf("Mysql Opearator deletion failed - %s", err)
	}
	err = KubeAccessor.Delete(ns, filepath.Join(mySQLCRSecFile))
	if err != nil {
		log.Info(err)
	}
}

