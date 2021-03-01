package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/trilioData/k8s-triliovault/internal/helpers"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	appsV1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/apis"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
	"github.com/trilioData/k8s-triliovault/tests/integration/common"
	"github.com/trilioData/k8s-triliovault/tests/tools/logprinter"
)

var (
	Cl                            client.Client
	Ctx                           context.Context
	deltOpRestoredResourcesScript = "../deleteOperatorRestoredResources.sh"
	deltCustomLabelResourceScript = "../customResourceCleanup.sh"
	mySQLFillDataScript           = "../../../test-data/mySQLFillData.sh"
	customApp                     = "../../../test-data/CustomResource/customResourceWithPVC.yaml"
	parentDir                     = ".."
	testDataDir                   = "test-data"
	mysqlHelmDir                  = "mysql-helm"
	deleteArg                     = "delete"
	mysqlOpDir                    = "mysql-operator"
	mysqlOperatorScript           = "mysqlOperator.sh"
	mysqlOpCrdFile                = "crd.yaml"
	mysqlOpCrFile                 = "mysqlCluster.yaml"
	mysqlOpCrScrtFile             = "mysqlCluster-secret.yaml"
	helmOpDir                     = "helm-operator"
	helmOperatorScript            = "helmOperator.sh"
	helmOpCrFile                  = "helmOp-hr.yaml"
	helmOpScrtFile                = "helmOp-hr-secret.yaml"
	installArg                    = "install"
	runtimeScheme                 = runtime.NewScheme()
	KubeAccessor                  *kube.Accessor
	AppMap                        = map[string]int32{"mysql": 2, "mongodb": 1, "airflow": 1}

	testDataDirPath       = filepath.Join(parentDir, parentDir, parentDir, testDataDir)
	mysqlOpCrFilePath     = filepath.Join(testDataDirPath, mysqlOpDir, mysqlOpCrFile)
	mysqlOpCrScrtFilePath = filepath.Join(testDataDirPath, mysqlOpDir, mysqlOpCrScrtFile)
	helmOpCrFilePath      = filepath.Join(testDataDirPath, helmOpDir, helmOpCrFile)
	helmOpScrtFilePath    = filepath.Join(testDataDirPath, helmOpDir, helmOpScrtFile)
)

const (
	storageClassPlaceHolder        = "STORAGE_CLASS_NAME"
	BplanForNamespace              = "sample-backupplan-ns"
	BplanForAllComp                = "sample-backupplan-all"
	BplanForCustom                 = "sample-backupplan-custom"
	BplanForCustomWithHook         = "sample-backupplan-custom-with-hook"
	BplanForCustomWithMultiLabel   = "sample-backupplan-custom-multi-label"
	BplanForHelm3                  = "sample-backupplan-helm3"
	BplanForOperator               = "sample-backupplan-operator"
	BplanForOperatorWithMultiLabel = "sample-backupplan-operator-multi-label"
	BplanForHelmBasedOperator      = "sample-backupplan-helm-based-operator"
	BplanForMultiOperator          = "sample-backupplan-multi-operator"
	BackupForNamespace             = "sample-backup-ns"
	BackupForCustom                = "sample-backup-custom"
	BackupForCustomWithHook        = "sample-backup-custom-with-hook"
	BackupForCustomMultiLabels     = "sample-backup-custom-multi-label"
	BackupForAllComp               = "sample-backup-all"
	BackupForHelm3                 = "sample-backup-helm3"
	BackupForOperator              = "sample-backup-operator"
	BackupForOperatorMultiLabels   = "sample-backup-operator-multi-label"
	BackupForMultiOperator         = "sample-backup-multi-operator"
	BackupForHelmBasedOperator     = "sample-backup-helm-based-operator"
	phaseTimeout                   = 860 * time.Second
	tidyTimeout                    = 1560 * time.Second
	smallTimout                    = 150 * time.Second
	tidyInterval                   = 5 * time.Second
)

type HookTestData struct {
	PodName, ContainerName, Namespace string
	Command                           map[string]string
}

func init() {
	var err error
	_ = corev1.AddToScheme(runtimeScheme)
	_ = appsV1.AddToScheme(runtimeScheme)
	_ = rbacv1.AddToScheme(runtimeScheme)
	_ = crd.AddToScheme(runtimeScheme)
	Ctx = context.Background()

	KubeAccessor, err = kube.NewEnv(runtimeScheme)
	if err != nil {
		log.Fatalf("could not create client, err: %s", err.Error())
	}
	Cl = KubeAccessor.GetKubeClient()
}

// TODO: Use this after successful restore for cleanup check
func VerifyRestoreCleanup(restore *crd.Restore) {
	namespace := restore.Spec.RestoreNamespace

	log.Infof("Verifying restore cleanup")
	ginkgo.By("Verifying cleaned up restore jobs")
	restoreJobLabelSelector := fmt.Sprintf("%s=%s", internal.ControllerOwnerUID, string(restore.GetUID()))
	jobs, err := KubeAccessor.GetJobs(namespace, restoreJobLabelSelector)
	if err != nil {
		log.Infof("Error while getting jobs in namespace %s", namespace)
		ginkgo.Fail("Failed to get jobs")
	}
	if len(jobs) != 0 {
		log.Infof("Error while getting jobs in namespace %s", namespace)
		ginkgo.Fail("Failed to get jobs")
	}
}

func VerifyBackupCleanup(backup *crd.Backup) bool {
	namespace := backup.Namespace

	log.Infof("Verifying backup cleanup")
	ginkgo.By("Verifying cleaned up backup jobs")
	jobs, err := KubeAccessor.GetJobs(namespace)
	if err != nil {
		log.Errorf("Error while getting jobs %v", err)
		return false
	}
	datamoverJobs := filterBackupChildJobs(backup, jobs)
	if len(datamoverJobs) != 0 {
		log.Errorf("Mismatch for cleaned-up backup child jobs %d", len(datamoverJobs))
		return false
	}

	ginkgo.By("Verifying cleaned up backup datamover pvc")
	pvcs, err := KubeAccessor.GetPersistentVolumeClaims(namespace)
	if err != nil {
		log.Errorf("Error while getting pvc %v", err)
		return false
	}
	datamoverPVC := filterBackupDatamoverPVC(backup, pvcs)
	if len(datamoverPVC) != 0 {
		log.Infof("Mismatch for cleaned-up backup child PVCs %d", len(datamoverPVC))
		return false
	}

	return true
}

func filterBackupChildJobs(backup *crd.Backup, childJobs []batchv1.Job) []batchv1.Job {
	var filteredChildJobs []batchv1.Job

	for childJobIndex := range childJobs {
		childJob := childJobs[childJobIndex]
		controllerOwner := metav1.GetControllerOf(&childJob)
		if controllerOwner != nil && controllerOwner.Kind == internal.BackupKind &&
			controllerOwner.Name == backup.Name {
			filteredChildJobs = append(filteredChildJobs, childJob)
		}
	}

	return filteredChildJobs
}

func filterBackupDatamoverPVC(backup *crd.Backup, pvcs []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	var filteredPVCs []corev1.PersistentVolumeClaim

	for childPVCIndex := range pvcs {
		pvc := pvcs[childPVCIndex]
		controllerOwner := metav1.GetControllerOf(&pvc)
		if controllerOwner != nil && controllerOwner.Kind == internal.BackupKind &&
			controllerOwner.Name == backup.Name {
			filteredPVCs = append(filteredPVCs, pvc)
		}
	}

	return filteredPVCs
}

func VerifyResourcesNotExists(a *kube.Accessor, ns string, resourceMap map[string][]string) {
	for s, list := range resourceMap {
		gvk := common.GetGVKFromString(s)
		for _, item := range list {
			log.Infof("Checking for gvk: %s, name: %s", gvk, item)
			obj, err := a.GetUnstructuredObject(types.NamespacedName{Namespace: ns, Name: item}, gvk)
			if err == nil {
				log.Errorf("error: %s, objects present", obj.GetName())
				ginkgo.Fail(fmt.Sprintf("Resources exists, GVK: %s, item: %s, namespace: %s", gvk, item, ns))
			}
		}
	}
	log.Infof("Successfully checked all the passed resources")
}

func VerifyCustomRestore(expectedResourceMap map[string][]string, newRes []crd.Resource, restore *crd.Restore) {
	ns := restore.Spec.RestoreNamespace

	for key := range expectedResourceMap {
		resourceSlice := expectedResourceMap[key]
		Gvk := common.GetGVKObject(key)
		obj := unstructured.Unstructured{}
		obj.SetGroupVersionKind(Gvk)
		log.Infof("Checking for GVK: [%s]", key)
		for resourceIndex := range resourceSlice {
			resource := resourceSlice[resourceIndex]
			gomega.Expect(Cl.Get(Ctx, types.NamespacedName{
				Namespace: ns,
				Name:      resource,
			}, &obj)).NotTo(gomega.HaveOccurred())
			log.Infof("Resource Exists: [%s]", resource)
		}
	}

	// Add newResAdded to the expectedResourceMap
	// Can't add this list directly to expectResourceMap because it modifies the original resourceMap
	for i := range newRes {
		res := newRes[i]
		obj := unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind(res.GroupVersionKind))
		log.Infof("Checking for GVK: [%s]", obj.GroupVersionKind().String())
		for j := range res.Objects {
			r := res.Objects[j]
			gomega.Expect(Cl.Get(Ctx, types.NamespacedName{
				Namespace: ns,
				Name:      r,
			}, &obj)).NotTo(gomega.HaveOccurred())
			log.Infof("Resource Exists: [%s]", r)
		}
	}
}

func VerifyHelmRestore(restore *crd.Restore, ns, helmVer string, expectedApps []string) {
	log.Infof("Restore validation started")
	helmCharts := restore.Status.RestoreApplication.HelmCharts
	if len(helmCharts) != len(expectedApps) {
		ginkgo.Fail("helm release count mismatch")
	}
	var helmResourceMap map[string]interface{}
	for i := range helmCharts {
		hs := helmCharts[i].Snapshot
		newName := hs.NewRelease
		log.Infof("verifying Helm meta and data for [%s]", newName)

		if helmVer == string(crd.Helm3) {
			helmResourceMap = GetHelm3ResourceMap(newName, expectedApps[i], AppMap[expectedApps[i]])
		}

		VerifyCustomRestore(helmResourceMap["componentMetadata"].(map[string][]string), nil, restore)
		VerifyHelmMeta(helmResourceMap, hs)

		log.Info("Checking if pods are up")
		switch expectedApps[i] {
		case "mysql":
			CheckIfPodRunning("release", newName, ns)
		case "mongodb":
			CheckIfPodRunning("app.kubernetes.io/instance", newName, ns)
		case "airflow":
			CheckIfPodRunning("app.kubernetes.io/instance", newName, ns)
			CheckIfPodRunning("release", newName, ns)
		}
	}
}

// VerifyHelmMeta verifies helm release meta-data
func VerifyHelmMeta(expectedResourceMap map[string]interface{}, hs *crd.Helm) {

	r := expectedResourceMap["revision"]
	if hs.Revision != r.(int32) {
		ginkgo.Fail("Wrong revision of the helm chart")
	}
	rel := expectedResourceMap["release"]
	if hs.NewRelease != rel.(string) {
		ginkgo.Fail("Invalid release name")
	}
	version := expectedResourceMap["version"]
	if string(hs.Version) != version {
		ginkgo.Fail("Invalid helm version")
	}
	sb := expectedResourceMap["storageBackend"]
	if sb != string(hs.StorageBackend) {
		ginkgo.Fail("Invalid helm storage backend")
	}

}

// VerifyHelmBasedOpRestore verifies helm based operator restore
func VerifyHelmBasedOpRestore(expectedResourceMap map[string]interface{}, restore *crd.Restore) {

	opHelm := restore.Status.RestoreApplication.Operators
	for i := range opHelm {
		hs := opHelm[i].Snapshot.Helm
		newResAdded := opHelm[i].Status.NewResourcesAdded
		VerifyCustomRestore(expectedResourceMap["componentMetadata"].(map[string][]string), newResAdded, restore)
		VerifyHelmMeta(expectedResourceMap, hs)
	}
}

func VerifyOperatorRestore(expectedResourceMap map[string]interface{}, restore *crd.Restore) {
	operatorRestore := restore.Status.RestoreApplication.Operators

	if len(expectedResourceMap) != len(operatorRestore) {
		ginkgo.Fail("Number of operator snapshot mismatch")
	}

	for _, operator := range operatorRestore {
		log.Infof("Verifying OperatorID: %s", operator.Snapshot.OperatorID)
		val, exists := expectedResourceMap[operator.Snapshot.OperatorID]
		if !exists {
			ginkgo.Fail("Expected operatorId not found")
		}
		newResAdded := operator.Status.NewResourcesAdded

		VerifyCustomRestore(val.(map[string][]string), newResAdded, restore)
		log.Infof("Verified OperatorID: %s", operator.Snapshot.OperatorID)

	}

}

func InstallMysqlHelm3(ns, releaseName, storageClass string) {
	log.Info("Installing MySQL helm chart")
	mySQLCmd := fmt.Sprintf("helm install %s %s -n %s "+
		"--set persistence.storageClass=%s --wait --timeout=%ds",
		releaseName, filepath.Join(testDataDirPath, mysqlHelmDir), ns,
		storageClass, common.ResourceDeploymentTimeout)
	out, err := shell.RunCmd(mySQLCmd)
	log.Info(out.Out)
	gomega.Expect(err).To(gomega.BeNil(), "Mysql helm chart installation failed")
	log.Info("Installed MySQL helm chart")
}

func UpgradeMysqlHelm3(ns, releaseName, storageClass string) {
	mySQLCmd := fmt.Sprintf("helm upgrade %s %s -n %s --set persistence.storageClass=%s", releaseName,
		filepath.Join(testDataDirPath, mysqlHelmDir), ns, storageClass)
	out, err := shell.RunCmd(mySQLCmd)
	log.Info(out.Out)
	gomega.Expect(err).To(gomega.BeNil(), "Mysql helm chart upgrading failed")
}

func RollbackMysqlHelm3(releaseName, ns string, revision int) {
	mySQLCmd := fmt.Sprintf("helm rollback %s %d -n %s", releaseName, revision, ns)
	out, err := shell.RunCmd(mySQLCmd)
	log.Info(out.Out)
	gomega.Expect(err).To(gomega.BeNil(), "Mysql helm chart rollback failed")
}

func DeleteMysqlHelm3(releaseName, ns string) {
	mySQLCmd := fmt.Sprintf("helm delete %s -n %s", releaseName, ns)
	out, err := shell.RunCmd(mySQLCmd)
	log.Info(out.Out)
	if err != nil {
		log.Errorf("Mysql helm chart deletion failed - %s", err)
		return
	}
	log.Infof("Mysql helm chart deleted")
}

func CheckIfDataPersists(ns, releaseName string) {
	log.Infof("Checking data in pod of release: [%s]", releaseName)
	checkData := fmt.Sprintf("%s check %s %s", mySQLFillDataScript, ns, releaseName)
	cmdOut, err := shell.RunCmd(checkData)
	log.Infof("Table entries after restore %s\n", cmdOut.Out)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(cmdOut.Out).To(gomega.ContainSubstring("4"))

}

func DeleteHelm3App(releaseName, ns string) {
	helmCmd := fmt.Sprintf("helm delete %s -n %s", releaseName, ns)
	out, _ := shell.RunCmd(helmCmd)
	log.Info(out.Out)
}

func InstallMysqlOperator(ns, releaseName string) {
	log.Infof("Installing mysql operator: [%s]", releaseName)
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(parentDir, parentDir, parentDir,
		testDataDir, mysqlOpDir, mysqlOperatorScript), installArg, releaseName, ns}, internal.Space))
	log.Info(out.Out)
	gomega.Expect(err).To(gomega.BeNil())

	// install MysqlCluster crd, it will be required
	gomega.Expect(KubeAccessor.Apply(ns, filepath.Join(testDataDirPath, mysqlOpDir,
		mysqlOpCrdFile))).NotTo(gomega.HaveOccurred())
	gomega.Expect(KubeAccessor.Apply(ns, mysqlOpCrFilePath)).To(gomega.BeNil())
	gomega.Expect(KubeAccessor.Apply(ns, mysqlOpCrScrtFilePath)).To(gomega.BeNil())

	time.Sleep(30 * time.Second) // -> giving it 30 sec to start all pods

	gomega.Eventually(func() error {
		_, err = KubeAccessor.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = KubeAccessor.GetPods(ns, "app.kubernetes.io/managed-by=mysql.presslabs.org", "app.kubernetes.io/name=mysql")
			otherPods, _ := KubeAccessor.GetPods(ns, "app=mysql-operator", fmt.Sprintf("release=%s", releaseName))
			pods = append(pods, otherPods...)
			return pods, lErr
		})
		return err
	}, common.ResourceDeploymentTimeout, common.ResourceDeploymentInterval).Should(gomega.BeNil())
	log.Info("Installed mysql operator")

}

func DeleteMysqlOperator(ns, operatorName string) {
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(parentDir, parentDir, parentDir,
		testDataDir, mysqlOpDir, mysqlOperatorScript), deleteArg, operatorName, ns}, internal.Space))
	log.Info(out.Out)

	if err != nil {
		log.Errorf("Mysql Opearator deletion failed - %s", err)
	}
	err = KubeAccessor.Delete(ns, filepath.Join(testDataDirPath, mysqlOpDir, mysqlOpCrScrtFile))
	if err != nil {
		log.Info(err)
	}
}

func InstallHelmOperator(ns, releaseName string) {
	log.Info("Installing helm operator")
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(testDataDirPath, helmOpDir, helmOperatorScript),
		installArg, releaseName, ns}, internal.Space))
	log.Info(out.Out)
	gomega.Expect(err).To(gomega.BeNil())

	// install HelmRelease CR and secret
	gomega.Expect(KubeAccessor.Apply(ns, helmOpScrtFilePath)).To(gomega.BeNil())
	gomega.Expect(KubeAccessor.Apply(ns, helmOpCrFilePath)).To(gomega.BeNil())
}

func DeleteHelmOperator(ns, operatorName string) {
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(testDataDirPath, helmOpDir, helmOperatorScript),
		deleteArg, operatorName, ns}, internal.Space))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	log.Info(out.Out)
	if err != nil {
		log.Error(err)
	}

	err = KubeAccessor.Delete(ns, filepath.Join(testDataDirPath, helmOpDir, helmOpScrtFile))
	if err != nil {
		log.Error(err)
	}
}

func DeleteOperatorRestoredResources(ns, uniqueID string) {
	deleteCmd := fmt.Sprintf("%s %s %s", deltOpRestoredResourcesScript, ns, uniqueID)
	out, err := shell.RunCmd(deleteCmd)
	if err != nil {
		log.Error(err.Error())
	}
	log.Info(out.Out)
}

func DeleteCustomLabelResource(ns, uniqueID, resourceFile string) {
	log.Info("Cleaning custom label resources")
	deleteCmd := fmt.Sprintf("%s %s %s %s", deltCustomLabelResourceScript, ns, uniqueID, resourceFile)
	cmd, err := shell.RunCmd(deleteCmd)
	if err != nil {
		log.Error(err.Error())
	}
	log.Info(cmd.Out)
}

func DeployCustomResources(namespace, yamlPath, uniqueID string, a *kube.Accessor) {
	_ = a.Apply(namespace, yamlPath)
	// Wait till all the custom components comes in Running state
	gomega.Eventually(func() error {
		_, err := a.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = a.GetPods(namespace, "app=nginx-deployment")
			stsPods, _ := a.GetPods(namespace, "app=nginx-sts")
			rcPods, _ := a.GetPods(namespace, "app=nginx-rc")
			rsPods, _ := a.GetPods(namespace, "app=nginx-rs")
			otherPods, _ := a.GetPods(namespace, fmt.Sprintf("triliobackupall=%s", uniqueID))
			pods = append(append(append(append(pods, stsPods...), rcPods...), rsPods...), otherPods...)
			return pods, lErr
		})
		return err
	}, common.ResourceDeploymentTimeout, common.ResourceDeploymentInterval).Should(gomega.BeNil())
}

func CheckIfPodRunning(key, value, ns string) {
	log.Infof("Checking pods with label %s:%s", key, value)
	checkRunningPod := fmt.Sprintf("kubectl wait --for=condition=Ready pod -l %s=%s"+
		" --timeout=300s --all -n %s", key, value, ns)
	cmdout, err := shell.RunCmd(checkRunningPod)
	log.Info(cmdout.Out)
	gomega.Expect(err).Should(gomega.BeNil())

}
func VerifyDataSnapshotContent(backupPath, backupName, namespace string,
	targetSnapshot *apis.FullSnapshot) {

	var (
		pvcCustomDataBackup = make(map[string]string)
		pvcCustomDataTarget = make(map[string]string)
		pvcOpDataTarget     = make(map[string]string)
		pvcOpHelmDataTarget = make(map[string]string)
		pvcOpDataBackup     = make(map[string]string)
		pvcOpHelmDataBackup = make(map[string]string)
		pvcHelmDataTarget   = make(map[string]string)
		pvcHelmDataBackup   = make(map[string]string)
		backup              crd.Backup
		err                 error
		index               int
		qCow2Size           string
	)

	err = Cl.Get(Ctx,
		types.NamespacedName{Name: backupName,
			Namespace: namespace}, &backup)
	gomega.Expect(err).To(gomega.BeNil())

	// Process Custom application
	if backup.Status.Snapshot.Custom != nil {
		log.Info("Verifying CustomData")
		// Create map of the PVC name and size from backup CR
		for dsCustomIndex := range backup.Status.Snapshot.Custom.DataSnapshots {
			dsCustom := backup.Status.Snapshot.Custom.DataSnapshots[dsCustomIndex]
			pvcCustom := unstructured.Unstructured{}
			pvcCustom.SetGroupVersionKind(schema.GroupVersionKind{
				Version: "v1",
				Kind:    "PersistentVolumeClaim",
			})
			err = json.Unmarshal([]byte(dsCustom.PersistentVolumeClaimMetadata), &pvcCustom)
			gomega.Expect(err).To(gomega.BeNil())
			pvcCustomDataBackup[pvcCustom.GetName()] = dsCustom.Size.String()
		}
		// Create map of the PVC name and size from the target
		for index = range targetSnapshot.CustomSnapshot.DataSnapshots {
			dsCustomOnTarget := targetSnapshot.CustomSnapshot.DataSnapshots[index]
			pvcCustom := unstructured.Unstructured{}
			pvcCustom.SetGroupVersionKind(schema.GroupVersionKind{
				Version: "v1",
				Kind:    "PersistentVolumeClaim",
			})
			err = json.Unmarshal([]byte(dsCustomOnTarget.PersistentVolumeClaimMetadata), &pvcCustom)
			gomega.Expect(err).To(gomega.BeNil())
			qCow2Path := path.Join(backupPath, internal.CustomBackupDir, internal.DataSnapshotDir,
				pvcCustom.GetName(), internal.Qcow2PV)
			qCow2Size, err = getQcow2SizeFromTarget(qCow2Path)
			gomega.Expect(err).To(gomega.BeNil())
			pvcCustomDataTarget[pvcCustom.GetName()] = qCow2Size
		}
		// Validate the PVC name and size from the backup and target
		validatePvcAndSizes(pvcCustomDataBackup, pvcCustomDataTarget)
		log.Info("CustomData verification passed")

	}

	// Process Operator based application
	// Create map of the PVC name and size from backup CR
	if len(backup.Status.Snapshot.Operators) != 0 {
		log.Info("Verifying OperatorData")

		for oSnapIndex := range backup.Status.Snapshot.Operators {
			oSnap := backup.Status.Snapshot.Operators[oSnapIndex]
			if len(oSnap.DataSnapshots) != 0 {
				getPVCListWithSizeFromBackup(oSnap.DataSnapshots, pvcOpDataBackup)
			}

			opHelmSnap := backup.Status.Snapshot.Operators[oSnapIndex].Helm
			if opHelmSnap != nil {
				getPVCListWithSizeFromBackup(opHelmSnap.DataSnapshots, pvcOpHelmDataBackup)
			}

		}
		// Create map of the PVC name and size from the target
		for index = range targetSnapshot.OperatorSnapshots {
			oSnapTarget := targetSnapshot.OperatorSnapshots[index]
			if len(oSnapTarget.DataSnapshots) != 0 {
				getPVCListWithSizeFromTarget(oSnapTarget.DataSnapshots, backupPath, internal.OperatorKind,
					oSnapTarget.OperatorID, pvcOpDataTarget)
			}

			oHelmSnapTarget := targetSnapshot.OperatorSnapshots[index].Helm
			if oHelmSnapTarget != nil {
				opHelmBackupPath := path.Join(backupPath, internal.OperatorBackupDir, oSnapTarget.OperatorID)
				getPVCListWithSizeFromTarget(oHelmSnapTarget.DataSnapshots, opHelmBackupPath, internal.HelmKind,
					oHelmSnapTarget.Release, pvcOpHelmDataTarget)
			}
		}

		// Validate the PVC name and size from the backup and target
		validatePvcAndSizes(pvcOpDataBackup, pvcOpDataTarget)
		validatePvcAndSizes(pvcOpHelmDataBackup, pvcOpHelmDataTarget)
		log.Info("OperatorData verification passed")
	}

	// Process helm based application
	// Create map of the PVC name and size from backup CR
	if len(backup.Status.Snapshot.HelmCharts) != 0 {
		log.Info("Verifying HelmData")

		for helmSnapIndex := range backup.Status.Snapshot.HelmCharts {
			helmSnap := backup.Status.Snapshot.HelmCharts[helmSnapIndex]
			getPVCListWithSizeFromBackup(helmSnap.DataSnapshots, pvcHelmDataBackup)
		}
		// Create map of the PVC name and size from the target
		for index = range targetSnapshot.HelmSnapshots {
			helmSnapTarget := targetSnapshot.HelmSnapshots[index]

			getPVCListWithSizeFromTarget(helmSnapTarget.DataSnapshots, backupPath, internal.HelmKind,
				helmSnapTarget.Release, pvcHelmDataTarget)
		}
		// Validate the PVC name and size from the backup and target
		validatePvcAndSizes(pvcHelmDataBackup, pvcHelmDataTarget)
		log.Info("HelmData verification passed")
	}
}
func getPVCListWithSizeFromBackup(dataSnapshotList []crd.DataSnapshot, pvcMap map[string]string) {
	for dsIndex := range dataSnapshotList {
		ds := dataSnapshotList[dsIndex]
		pvc := unstructured.Unstructured{}
		pvc.SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "PersistentVolumeClaim",
		})
		err := json.Unmarshal([]byte(ds.PersistentVolumeClaimMetadata), &pvc)
		gomega.Expect(err).To(gomega.BeNil())
		pvcMap[pvc.GetName()] = ds.Size.String()
	}
}

func getPVCListWithSizeFromTarget(dataSnapshotList []crd.DataSnapshot, backupPath, kind, kindName string,
	pvcMap map[string]string) {
	var qCow2Path string
	for dsIndex := range dataSnapshotList {
		dsOpTarget := dataSnapshotList[dsIndex]
		pvc := unstructured.Unstructured{}
		pvc.SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "PersistentVolumeClaim",
		})
		err := json.Unmarshal([]byte(dsOpTarget.PersistentVolumeClaimMetadata), &pvc)
		gomega.Expect(err).To(gomega.BeNil())

		if kind == internal.OperatorKind {
			qCow2Path = path.Join(backupPath, internal.OperatorBackupDir, kindName,
				internal.DataSnapshotDir, pvc.GetName(), internal.Qcow2PV)
		} else if kind == internal.HelmKind {
			qCow2Path = path.Join(backupPath, internal.HelmBackupDir, kindName, internal.DataSnapshotDir,
				pvc.GetName(), internal.Qcow2PV)
		}
		qCow2Size, err := getQcow2SizeFromTarget(qCow2Path)
		gomega.Expect(err).To(gomega.BeNil())
		pvcMap[pvc.GetName()] = qCow2Size
	}
}

func validatePvcAndSizes(pvcMetaFromBackup, pvcMetaFromTarget map[string]string) {
	log.Info("pvcMetaFromBackup:", pvcMetaFromBackup)
	log.Info("pvcMetaFromTarget:", pvcMetaFromTarget)
	for pvcNameFromBackup, pvcSizeFromBackup := range pvcMetaFromBackup {
		var isPresent bool
		for pvcNameFromTarget, pvcSizeFromTarget := range pvcMetaFromTarget {
			if pvcNameFromBackup == pvcNameFromTarget {
				if pvcSizeFromBackup == pvcSizeFromTarget {
					isPresent = true
					break
				} else {
					ginkgo.Fail(fmt.Sprintf("PVC %s from backup CR and PVC %s "+
						"from target are having data size mismatch. PVC "+
						"from backup CR is having size %s and PVC from "+
						"target is having size %s", pvcNameFromBackup, pvcNameFromTarget,
						pvcSizeFromBackup, pvcSizeFromTarget))
				}
			}
		}
		gomega.Expect(isPresent).To(gomega.BeTrue())
	}
}

func getQcow2SizeFromTarget(qCow2Path string) (string, error) {
	pvcImageInfo, _, err := helpers.GetImageInfo(qCow2Path)
	if err != nil {
		return "", err
	}
	return pvcImageInfo.Size, nil
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func WaitForBackupPlanAvailable
func WaitForBackupAvailable(backupName, ns string) {

	gomega.Eventually(func() (crd.Status, error) {
		s, err := KubeAccessor.GetBackupStatus(backupName, ns)
		if err != nil {
			return s, err
		}
		if s == crd.Failed {
			logprinter.PrintCR(internal.BackupKind, ns)
			ginkgo.Fail("Backup is in Failed state")
		}
		return s, nil
	}, tidyTimeout, tidyInterval).Should(gomega.Or(gomega.Equal(crd.InProgress),
		gomega.Equal(crd.Completed), gomega.Equal(crd.Available)))
	log.Infof("Backup: [%s] is in InProgress state", backupName)

	t1 := time.Now()
	gomega.Eventually(func() (crd.Status, error) {
		s, err := KubeAccessor.GetBackupStatus(backupName, ns)
		if err != nil {
			return s, err
		}
		if s == crd.Failed {
			logprinter.PrintCR(internal.BackupKind, ns)
			ginkgo.Fail(fmt.Sprintf("Backup: [%s] is in Failed state", backupName))
		}

		t2 := time.Now()
		if diff := t2.Sub(t1); diff.Seconds() > float64(tidyTimeout-60) && s == crd.InProgress {
			if err := KubeAccessor.UpdateBackupStatus(backupName, ns, crd.Failed); err != nil {
				return s, err
			}
			ginkgo.Fail(fmt.Sprintf("Backup: [%s] stuck in InProgress state", backupName))
		}
		return s, nil
	}, tidyTimeout, tidyInterval).Should(gomega.Or(gomega.Equal(crd.Completed),
		gomega.Equal(crd.Available)))

	log.Infof("Backup: [%s] is in Completed state", backupName)

	// check if all backups are in available state
	gomega.Eventually(func() (crd.Status, error) {
		return KubeAccessor.GetBackupStatus(backupName, ns)
	}, tidyTimeout, tidyInterval).Should(gomega.Equal(crd.Available))

	log.Infof("Backup: [%s] is in Available state", backupName)
}

//nolint:dupl // added to get rid of lint errors of duplicate code
func VerifyBackupHookExecution(backupName, ns string, hookTestData []HookTestData) {

	t1 := time.Now()
	gomega.Eventually(func() (crd.OperationType, error) {
		s, err := KubeAccessor.GetBackup(backupName, ns)
		if err != nil {
			return "", err
		}

		if s.Status.Status == crd.Failed {
			logprinter.PrintCR(internal.BackupKind, ns)
			ginkgo.Fail("Backup is in Failed state")
		}

		t2 := time.Now()
		if diff := t2.Sub(t1); diff.Seconds() > float64(phaseTimeout-60) && s.Status.Status == crd.InProgress {
			if err := KubeAccessor.UpdateBackupStatus(backupName, ns, crd.Failed); err != nil {
				return "", err
			}
			ginkgo.Fail(fmt.Sprintf("Backup: [%s] stuck in InProgress state", backupName))
		}
		return s.Status.Phase, nil
	}, phaseTimeout, tidyInterval).Should(gomega.Equal(crd.DataUploadUnquiesceOperation))
	log.Infof("Backup: [%s] is in InProgress state and Quiesce phase is completed", backupName)

	// check whether quiescing was successful
	for i := range hookTestData {
		hook := hookTestData[i]
		_, execErr := KubeAccessor.Exec(hook.Namespace, hook.PodName, hook.ContainerName,
			hook.Command["pre"])
		gomega.Expect(execErr).To(gomega.HaveOccurred())
		log.Infof("Quiescing successful for pod/container=%s/%s", hook.PodName, hook.ContainerName)
	}

	t1 = time.Now()
	gomega.Eventually(func() (crd.OperationType, error) {
		s, err := KubeAccessor.GetBackup(backupName, ns)
		if err != nil {
			return "", err
		}
		if s.Status.Status == crd.Failed {
			logprinter.PrintCR(internal.BackupKind, ns)
			ginkgo.Fail(fmt.Sprintf("Backup: [%s] is in Failed state", backupName))
		}

		t2 := time.Now()
		if diff := t2.Sub(t1); diff.Seconds() > float64(phaseTimeout-60) && s.Status.Status == crd.InProgress {
			if err := KubeAccessor.UpdateBackupStatus(backupName, ns, crd.Failed); err != nil {
				return "", err
			}
			ginkgo.Fail(fmt.Sprintf("Backup: [%s] stuck in InProgress state", backupName))
		}
		return s.Status.Phase, nil
	}, phaseTimeout, tidyInterval).Should(gomega.Or(gomega.Equal(crd.MetadataUploadOperation), gomega.Equal(crd.RetentionOperation)))
	log.Infof("Backup: [%s] is in InProgress state and Unquiesce phase is completed", backupName)

	// check whether unquiescing was successful
	for i := range hookTestData {
		hook := hookTestData[i]
		_, execErr := KubeAccessor.Exec(hook.Namespace, hook.PodName, hook.ContainerName,
			hook.Command["post"])
		gomega.Expect(execErr).To(gomega.HaveOccurred())
		log.Infof("UnQuiescing successful for pod/container=%s/%s", hook.PodName, hook.ContainerName)
	}
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func WaitForBackupAvailable
func WaitForBackupPlanAvailable(bpName, ns string) {
	t1 := time.Now()
	gomega.Eventually(func() (crd.Status, error) {
		s, err := KubeAccessor.GetBackupPlanStatus(bpName, ns)
		if err != nil {
			return s, err
		}

		if s == crd.Failed {
			logprinter.PrintCR(internal.BackupplanKind, ns)
			ginkgo.Fail("backupPlan in error status")
		}

		t2 := time.Now()
		if diff := t2.Sub(t1); diff.Seconds() > float64(tidyTimeout-60) && s == crd.InProgress {
			if err := KubeAccessor.UpdateBackupPlanStatus(bpName, ns, crd.Failed); err != nil {
				return s, err
			}
			ginkgo.Fail(fmt.Sprintf("BackupPlan: [%s] stuck in InProgress state", bpName))
		}

		return s, nil
	}, smallTimout, tidyInterval).Should(gomega.Equal(crd.Available))
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func WaitForBackupAvailable
func WaitForRestoreCompletion(restoreName, ns string) {

	t1 := time.Now()
	gomega.Eventually(func() (crd.Status, error) {
		s, err := KubeAccessor.GetRestoreStatus(restoreName, ns)
		if err != nil {
			return s, err
		}

		if s == crd.Failed {
			logprinter.PrintCR(internal.RestoreKind, ns)
			ginkgo.Fail(fmt.Sprintf("Restore: [%s] failed", restoreName))
		}

		t2 := time.Now()
		if diff := t2.Sub(t1); diff.Seconds() > float64(tidyTimeout-60) && s == crd.InProgress {
			if err := KubeAccessor.UpdateRestoreStatus(restoreName, ns, crd.Failed); err != nil {
				return s, err
			}
			ginkgo.Fail(fmt.Sprintf("Restore: [%s] stuck in InProgress state", restoreName))
		}

		return s, nil
	}, tidyTimeout, tidyInterval).Should(gomega.Equal(crd.Completed))

}

// CheckWarningsFromBackup checks for pod not Running/Succeeded state from backup status
// fails the spec if CheckPodWarnings() returns true
func CheckWarningsFromBackup(backupName, ns string) {

	bkpObj, err := KubeAccessor.GetBackup(backupName, ns)
	gomega.Expect(err).To(gomega.BeNil())

	bkpJSON, err := json.MarshalIndent(bkpObj, "", "  ")
	gomega.Expect(err).To(gomega.BeNil())

	if bkpObj.Status.Snapshot.Custom != nil {
		if CheckPodWarnings(bkpObj.Status.Snapshot.Custom.Warnings) {
			log.Infof("backup object: %s", string(bkpJSON))
			ginkgo.Fail("found warnings related to pods not running/succeeded in backup")
		}
	}

	for i := range bkpObj.Status.Snapshot.Operators {
		if CheckPodWarnings(bkpObj.Status.Snapshot.Operators[i].Warnings) {
			log.Infof("backup object: %s", string(bkpJSON))
			ginkgo.Fail("found warnings related to pods not running/succeeded in backup")
		}
	}

	for i := range bkpObj.Status.Snapshot.HelmCharts {
		if CheckPodWarnings(bkpObj.Status.Snapshot.HelmCharts[i].Warnings) {
			log.Infof("backup object: %s", string(bkpJSON))
			ginkgo.Fail("found warnings related to pods not running/succeeded in backup")
		}
	}

}

// CheckPodWarnings checks for pod not Running/Succeeded state warnings,
// returns true if found, else false
func CheckPodWarnings(warnings []string) bool {
	warningString := "Pods are not in Running/Succeeded state"

	for i := range warnings {
		if strings.Contains(warnings[i], warningString) {
			return true
		}
	}

	return false
}

func UpdateMysqlClusterSecret(name, ns string) {
	var secret corev1.Secret

	gomega.Expect(KubeAccessor.Apply(ns, filepath.Join(testDataDirPath, mysqlOpDir,
		mysqlOpCrScrtFile))).To(gomega.BeNil())

	gomega.Expect(Cl.Get(Ctx, types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, &secret)).NotTo(gomega.HaveOccurred())

	secret.Data["ROOT_PASSWORD"] = []byte("pwd")
	gomega.Expect(Cl.Update(Ctx, &secret)).NotTo(gomega.HaveOccurred())
}

func CheckMysqlClusterSecretPatchOperation(name, ns string) {
	var secret corev1.Secret
	gomega.Expect(Cl.Get(Ctx, types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, &secret)).NotTo(gomega.HaveOccurred())

	log.Info("Checking for mysqlcluster-secret patch operation")
	gomega.Expect(secret.Data["ROOT_PASSWORD"]).To(gomega.Equal([]uint8("mypass")))
}

func UpdateClusterRole(name string) {
	var clusterRole rbacv1.ClusterRole
	gomega.Expect(Cl.Get(Ctx, types.NamespacedName{
		Namespace: "",
		Name:      name,
	}, &clusterRole)).NotTo(gomega.HaveOccurred())

	// Remove last PolicyRule in clusterRole rules to check patch operation
	clusterRole.Rules = append(clusterRole.Rules[:len(clusterRole.Rules)-1], clusterRole.Rules[:0]...)
	gomega.Expect(Cl.Update(Ctx, &clusterRole)).NotTo(gomega.HaveOccurred())
}

func CheckClusterRolePatchOperation(name string, expectedRules int) {
	var clusterRole rbacv1.ClusterRole
	gomega.Expect(Cl.Get(Ctx, types.NamespacedName{
		Namespace: "",
		Name:      name,
	}, &clusterRole)).NotTo(gomega.HaveOccurred())

	gomega.Expect(len(clusterRole.Rules)).To(gomega.Equal(expectedRules))
}

func GetHelm3ResourceMap(releaseName, appName string, revision int32) map[string]interface{} {

	resourceMap := map[string]interface{}{
		"mysql": map[string]interface{}{
			"release": releaseName, "revision": revision, "version": "v3", "storageBackend": "Secret",
			"componentMetadata": map[string][]string{
				"v1,kind=Secret": {"sh.helm.release.v1." + releaseName + ".v1",
					"sh.helm.release.v1." + releaseName + ".v2", releaseName + "-mysql"},
				"v1,kind=Service":               {"helm-mysql-" + releaseName},
				"v1,kind=ConfigMap":             {releaseName + "-mysql" + "-test"},
				"v1,kind=PersistentVolumeClaim": {releaseName + "-mysql"},
				"apps/v1,kind=Deployment":       {releaseName + "-mysql"},
			},
		},
		"mongodb": map[string]interface{}{
			"release": releaseName, "revision": revision, "version": "v3", "storageBackend": "Secret",
			"componentMetadata": map[string][]string{
				"v1,kind=Secret":                {"sh.helm.release.v1." + releaseName + ".v1", releaseName + "-mongodb"},
				"v1,kind=ServiceAccount":        {releaseName + "-mongodb"},
				"v1,kind=Service":               {releaseName + "-mongodb"},
				"v1,kind=PersistentVolumeClaim": {releaseName + "-mongodb"},
				"apps/v1,kind=Deployment":       {releaseName + "-mongodb"},
			},
		},
		"airflow": map[string]interface{}{
			"release": releaseName, "revision": revision, "version": "v3", "storageBackend": "Secret",
			"componentMetadata": map[string][]string{
				"v1,kind=Secret": {"sh.helm.release.v1." + releaseName + ".v1", releaseName + "-postgresql",
					releaseName + "-redis", releaseName + "-airflow"},
				"v1,kind=Service": {releaseName + "-postgresql-headless", releaseName + "-postgresql",
					releaseName + "-redis-headless", releaseName + "-redis-master", releaseName + "-redis-slave",
					releaseName + "-airflow-headless", releaseName + "-airflow"},
				"v1,kind=ConfigMap": {releaseName + "-redis", releaseName + "-redis-health"},
				"apps/v1,kind=StatefulSet": {releaseName + "-postgresql", releaseName + "-redis-master",
					releaseName + "-redis-slave", releaseName + "-airflow-worker"},
				"apps/v1,kind=Deployment": {releaseName + "-airflow-scheduler", releaseName + "-airflow-web"},
			},
		},
	}

	if res, exists := resourceMap[appName]; exists {
		return res.(map[string]interface{})
	}
	return map[string]interface{}{}
}

func GetMysqlOpHelmResourceMap(uniqueID, releaseName string, revision int32) map[string]interface{} {

	return map[string]interface{}{
		"release":        releaseName,
		"revision":       revision,
		"version":        "v3",
		"storageBackend": "Secret",
		"componentMetadata": map[string][]string{
			"v1,kind=Secret": {"sh.helm.release.v1." + releaseName + ".v1",
				uniqueID + "-sample-mysqlcluster-mysql-operated", releaseName + "-mysql-operator" + "-orc",
				uniqueID + "-sample-mysql-cluster-secret"},
			"v1,kind=ConfigMap":        {releaseName + "-mysql-operator" + "-orc"},
			"v1,kind=ServiceAccount":   {releaseName + "-mysql-operator"},
			"v1,kind=Service":          {releaseName + "-mysql-operator", releaseName + "-mysql-operator" + "-0-svc"},
			"apps/v1,kind=StatefulSet": {releaseName + "-mysql-operator"},
			"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {releaseName + "-mysql-operator"},
			"rbac.authorization.k8s.io/v1,kind=ClusterRole":        {releaseName + "-mysql-operator"},
			"apiextensions.k8s.io/v1,kind=CustomResourceDefinition": {
				"mysqlclusters.mysql.presslabs.org", "mysqlbackups.mysql.presslabs.org"},
			"mysql.presslabs.org/v1alpha1,kind=MysqlCluster": {uniqueID + "-sample-mysqlcluster"},
		},
	}
}

func CleanupResourceInNamespace(scriptPath, uniqueID, ns string) {
	log.Infof("Cleaning up everything before setting up suite from %s namespace", ns)
	_, err := shell.RunCmd(fmt.Sprintf("%s %s %s", scriptPath, uniqueID, ns))
	gomega.Expect(err).To(gomega.BeNil())
}

//nolint
func AssignPlaceholderValues(uniqueID, testSc, backupNamespace, uniqueMySQLOperator, uniqueHelmOperator string) {
	// set Storage class and uniqueID
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			storageClassPlaceHolder: testSc,
			common.UniqueID:         uniqueID,
		}, customApp)).To(gomega.BeNil())

	// set MySqlOperator CR name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			common.UniqueID: uniqueID,
		}, mysqlOpCrFilePath)).To(gomega.BeNil())

	// set MySqlOperator secret name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			common.UniqueID:    uniqueID,
			common.ReleaseName: uniqueMySQLOperator,
		}, mysqlOpCrScrtFilePath)).To(gomega.BeNil())

	// set HelmOperator CR name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			common.UniqueID: uniqueID,
		}, helmOpCrFilePath)).To(gomega.BeNil())

	// set HelmOperator secret name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			common.UniqueID:    uniqueID,
			common.ReleaseName: uniqueHelmOperator,
		}, helmOpScrtFilePath)).To(gomega.BeNil())
}

//nolint
func RevertPlaceholderValues(uniqueID, testSc, backupNamespace, uniqueMySQLOperator, uniqueHelmOperator string) {
	// reset Storage class and uniqueID
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			testSc:   storageClassPlaceHolder,
			uniqueID: common.UniqueID,
		}, customApp)).To(gomega.BeNil())

	// reset MySqlOperator CR name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			uniqueID: common.UniqueID,
		}, mysqlOpCrFilePath)).To(gomega.BeNil())

	// reset MySqlOperator secret name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			uniqueMySQLOperator: common.ReleaseName,
		}, mysqlOpCrScrtFilePath)).To(gomega.BeNil())

	// reset MySqlOperator secret name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			uniqueID: common.UniqueID,
		}, mysqlOpCrScrtFilePath)).To(gomega.BeNil())

	// reset HelmOperator CR name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			uniqueID: common.UniqueID,
		}, helmOpCrFilePath)).To(gomega.BeNil())

	// reset HelmOperator secret name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			uniqueHelmOperator: common.ReleaseName,
		}, helmOpScrtFilePath)).To(gomega.BeNil())

	// reset HelmOperator secret name
	gomega.Expect(common.UpdateYAMLs(
		map[string]string{
			uniqueID: common.UniqueID,
		}, helmOpScrtFilePath)).To(gomega.BeNil())
}

func DeleteResources(resources []crd.Resource, ns string) {
	for index := range resources {
		res := resources[index]
		gvk := res.GroupVersionKind
		for _, item := range res.Objects {
			log.Infof("deleting gvk: %s, name: %s", gvk, item)
			err := KubeAccessor.DeleteUnstructuredObject(types.NamespacedName{Namespace: ns, Name: item},
				schema.GroupVersionKind(gvk), client.GracePeriodSeconds(0))
			if err != nil && !(apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind")) {
				log.Errorf("error while deleting the object: %s", err.Error())
				ginkgo.Fail(fmt.Sprintf("Failed to delete, GVK: %s, item: %s, namespace: %s", gvk, item, ns))
			}
		}
	}
}

func GetNamespaceResourceMap(uniqueID, testSc string, installedApps map[string]string, secretNames []string) map[string][]string {

	uniqueHelm3MySQL := installedApps["Helm3MySQL"]
	uniqueHelm3MongoDB := installedApps["Helm3MongoDB"]
	uniqueHelm3Airflow := installedApps["Helm3Airflow"]
	uniqueMySQLOperator := installedApps["MySQLOperator"]
	uniqueHelmOperator := installedApps["HelmOperator"]

	return map[string][]string{
		"apps/v1,kind=Deployment": {uniqueID + "-nginx-deployment", uniqueHelm3MySQL, uniqueHelmOperator,
			uniqueHelm3MongoDB, uniqueHelm3Airflow + "-scheduler", uniqueHelm3Airflow + "-web"},

		"v1,kind=Service": {uniqueID + "-nginx-deployment-svc", uniqueID + "-nginx-rc-svc",
			uniqueID + "-nginx-pod-svc", uniqueID + "-nginx-rs-svc", uniqueID + "-nginx-sts-svc",
			uniqueMySQLOperator, uniqueMySQLOperator + "-0-svc", uniqueHelmOperator, "helm-mysql-" + uniqueHelm3MySQL,
			"mysql", uniqueID + "-redis-headless", uniqueID + "-redis-master", uniqueID + "-redis-metrics",
			uniqueHelm3MongoDB, uniqueHelm3Airflow, uniqueHelm3Airflow + "-headless",
			uniqueHelm3Airflow + "-postgresql-headless", uniqueHelm3Airflow + "-postgresql",
			uniqueHelm3Airflow + "-redis-headless", uniqueHelm3Airflow + "-redis-master", uniqueHelm3Airflow + "-redis-slave"},

		"v1,kind=Pod": {uniqueID + "-nginx-pod", uniqueID + "-pod-raw"},
		"apps/v1,kind=StatefulSet": {uniqueID + "-nginx-sts", uniqueMySQLOperator,
			uniqueHelm3Airflow + "-postgresql", uniqueHelm3Airflow + "-redis-master",
			uniqueHelm3Airflow + "-redis-slave", uniqueHelm3Airflow + "-worker", uniqueID + "-redis-master"},

		"v1,kind=ReplicationController": {uniqueID + "-nginx-rc"},
		"apps/v1,kind=ReplicaSet":       {uniqueID + "-nginx-rs"},

		"storage.k8s.io/v1,kind=StorageClass": {testSc},

		"rbac.authorization.k8s.io/v1,kind=ClusterRole": {uniqueMySQLOperator, uniqueHelmOperator},

		"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {uniqueMySQLOperator, uniqueHelmOperator},

		"v1,kind=ServiceAccount": {uniqueID + "-sa-test", uniqueMySQLOperator, uniqueHelmOperator, "default", uniqueHelm3MongoDB},

		"v1,kind=ConfigMap": {uniqueHelm3Airflow + "-redis", uniqueHelm3Airflow + "-redis-health",
			uniqueID + "-configmap-test", uniqueHelmOperator + "-kube-config", uniqueMySQLOperator + "-orc",
			uniqueHelm3MySQL + "-test", uniqueID + "-redis", uniqueID + "-redis-health", uniqueID + "-redis-scripts",
			uniqueID + "-sample-mysqlcluster-mysql",
			"mysql-operator-leader-election"},

		"v1,kind=Secret":                              secretNames,
		"apps/v1,kind=DaemonSet":                      {uniqueID + "-fluentd-elasticsearch"},
		"networking.k8s.io/v1,kind=NetworkPolicy":     {uniqueID + "-network-policy"},
		"batch/v1,kind=Job":                           {uniqueID + "-ubuntu-job"},
		"batch/v1beta1,kind=CronJob":                  {uniqueID + "-ubuntu-cronjob"},
		"networking.k8s.io/v1beta1,kind=Ingress":      {uniqueID + "-test-ingress"},
		"autoscaling/v1,kind=HorizontalPodAutoscaler": {uniqueID + "-demo-hpa"},
		"apiextensions.k8s.io/v1,kind=CustomResourceDefinition": {"mysqlclusters.mysql.presslabs.org",
			"helmreleases.helm.fluxcd.io"},
		"mysql.presslabs.org/v1alpha1,kind=MysqlCluster": {uniqueID + "-sample-mysqlcluster"},
		"helm.fluxcd.io/v1,kind=HelmRelease":             {uniqueID + "-redis"},
	}
}

func GetMysqlAndHelmOpBackupResourceMap(uniqueID, uniqueMySQLOperator, uniqueHelmOperator, testSc string) map[string]interface{} {

	return map[string]interface{}{
		uniqueMySQLOperator: map[string]interface{}{
			"customResource": map[string][]string{
				"mysql.presslabs.org/v1alpha1,kind=MysqlCluster": {uniqueID + "-sample-mysqlcluster"},
			},
			"crd": map[string][]string{extensionsv1beta1.SchemeGroupVersion.WithKind(internal.CRDKind).String(): {
				"mysqlclusters.mysql.presslabs.org", "mysqlbackups.mysql.presslabs.org"}},
			"componentMetadata": map[string][]string{
				"v1,kind=ConfigMap":                                    {uniqueMySQLOperator + "-orc"},
				"v1,kind=Secret":                                       {uniqueID + "-sample-mysql-cluster-secret", uniqueMySQLOperator + "-orc"},
				"v1,kind=ServiceAccount":                               {uniqueMySQLOperator},
				"v1,kind=Service":                                      {uniqueMySQLOperator, uniqueMySQLOperator + "-0-svc"},
				"storage.k8s.io/v1,kind=StorageClass":                  {testSc},
				"apps/v1,kind=StatefulSet":                             {uniqueMySQLOperator},
				"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {uniqueMySQLOperator},
				"rbac.authorization.k8s.io/v1,kind=ClusterRole":        {uniqueMySQLOperator},
			},
		},

		uniqueHelmOperator: map[string]interface{}{
			"customResource": map[string][]string{
				"helm.fluxcd.io/v1,kind=HelmRelease": {uniqueID + "-redis"},
			},
			"crd": map[string][]string{extensionsv1beta1.SchemeGroupVersion.WithKind(internal.CRDKind).String(): {
				"helmreleases.helm.fluxcd.io"}},
			"componentMetadata": map[string][]string{
				"v1,kind=ConfigMap":       {uniqueHelmOperator + "-kube-config"},
				"v1,kind=Service":         {uniqueHelmOperator},
				"v1,kind=ServiceAccount":  {uniqueHelmOperator},
				"v1,kind=Secret":          {uniqueHelmOperator + "-git-deploy", "redis-auth"},
				"apps/v1,kind=Deployment": {uniqueHelmOperator},
				"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {uniqueHelmOperator},
				"rbac.authorization.k8s.io/v1,kind=ClusterRole":        {uniqueHelmOperator},
			},
		},
	}

}

func GetMysqlHelmBasedOpResourceMap(uniqueID, uniqueMySQLOperator, testSc string) map[string]interface{} {
	return map[string]interface{}{
		uniqueMySQLOperator: map[string]interface{}{
			"customResource": map[string][]string{
				"mysql.presslabs.org/v1alpha1,kind=MysqlCluster": {uniqueID + "-sample-mysqlcluster"},
			},
			"crd": map[string][]string{extensionsv1beta1.SchemeGroupVersion.WithKind(internal.CRDKind).String(): {
				"mysqlclusters.mysql.presslabs.org", "mysqlbackups.mysql.presslabs.org"}},
			"helmSnapshot": map[string]interface{}{
				uniqueMySQLOperator: map[string]interface{}{
					"release":           uniqueMySQLOperator,
					"revision":          int32(1),
					"version":           "v3",
					"storageBackend":    "Secret",
					"componentMetadata": map[string][]string{"v1,kind=Secret": {"sh.helm.release.v1." + uniqueMySQLOperator + ".v1"}},
				},
			},
			"componentMetadata": map[string][]string{
				"v1,kind=Secret":                      {uniqueID + "-sample-mysql-cluster-secret"},
				"storage.k8s.io/v1,kind=StorageClass": {testSc},
			},
		},
	}
}

func GetMysqlAndHelmOpRestoredResourceMap(uniqueID, uniqueMySQLOperator, uniqueHelmOperator, testSc string) map[string]interface{} {

	return map[string]interface{}{
		uniqueMySQLOperator: map[string][]string{
			"v1,kind=ConfigMap":                             {uniqueMySQLOperator + "-orc"},
			"v1,kind=Secret":                                {uniqueID + "-sample-mysql-cluster-secret", uniqueMySQLOperator + "-orc"},
			"v1,kind=ServiceAccount":                        {uniqueMySQLOperator},
			"v1,kind=Service":                               {uniqueMySQLOperator, uniqueMySQLOperator + "-0-svc"},
			"apps/v1,kind=StatefulSet":                      {uniqueMySQLOperator},
			"storage.k8s.io/v1,kind=StorageClass":           {testSc},
			"rbac.authorization.k8s.io/v1,kind=ClusterRole": {uniqueMySQLOperator},
			"apiextensions.k8s.io/v1,kind=CustomResourceDefinition": {"mysqlclusters.mysql.presslabs.org",
				"mysqlbackups.mysql.presslabs.org"},
			"mysql.presslabs.org/v1alpha1,kind=MysqlCluster": {uniqueID + "-sample-mysqlcluster"},
		},
		uniqueHelmOperator: map[string][]string{
			"v1,kind=ConfigMap":                                     {uniqueHelmOperator + "-kube-config"},
			"v1,kind=Service":                                       {uniqueHelmOperator},
			"v1,kind=ServiceAccount":                                {uniqueHelmOperator},
			"v1,kind=Secret":                                        {uniqueHelmOperator + "-git-deploy", "redis-auth"},
			"apps/v1,kind=Deployment":                               {uniqueHelmOperator},
			"rbac.authorization.k8s.io/v1,kind=ClusterRole":         {uniqueHelmOperator},
			"helm.fluxcd.io/v1,kind=HelmRelease":                    {uniqueID + "-redis"},
			"apiextensions.k8s.io/v1,kind=CustomResourceDefinition": {"helmreleases.helm.fluxcd.io"},
		},
	}
}
