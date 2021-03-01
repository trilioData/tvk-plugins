package logcollectortest

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	controllerHelpers "github.com/trilioData/k8s-triliovault/controllers/helpers"
	"github.com/trilioData/k8s-triliovault/tests/integration/common"
	"github.com/trilioData/k8s-triliovault/tests/tools/logprinter"
	com "github.com/trilioData/tvk-plugins/tests/common"
)

var _ = Describe("Log Collector Tests", func() {

	Context("When a backup of custom application is in available", func() {

		It("Should collect backup and related CR yaml with logs", func() {
			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyBackupLogs(customAvailableBackup, fileName, cRes, nRes)
		})

	})

	Context("When a backup of custom, operator and helm application is in available", func() {

		It("Should collect backup and related CR yaml with logs", func() {
			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyBackupLogs(customOperatorAvailableBackup, fileName, cRes, nRes)
		})

	})

	Context("When a backup of custom application is in failed", func() {

		JustBeforeEach(func() {
			createBackupWithApp(customBPlan, sampleBackupName, true)
			waitForBackup(sampleBackupName, namespace, v1.Failed)
		})

		JustAfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				logprinter.PrintDebugLogs()
			}
			deleteBackup(sampleBackupName)
		})

		It("Should collect backup and related CR yaml with logs", func() {

			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyBackupLogs(sampleBackupName, fileName, cRes, nRes)

		})

	})

	Context("When a backup of custom, operator and helm application is in failed", func() {

		JustBeforeEach(func() {
			createBackupWithApp(customOperatorBPlan, customOperatorFailedBackup, true)
			waitForBackup(customOperatorFailedBackup, namespace, v1.Failed)
		})

		JustAfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				logprinter.PrintDebugLogs()
			}
			deleteBackup(customOperatorFailedBackup)
		})

		It("Should collect backup and related CR yaml with logs", func() {
			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyBackupLogs(customOperatorFailedBackup, fileName, cRes, nRes)
		})

	})

	Context("When a restore of backed up application is in available", func() {

		JustBeforeEach(func() {
			createRestoreForBackup(sampleRestoreName, customAvailableBackup, restoreNamespace, false)
			waitForRestore(sampleRestoreName, namespace, v1.Completed)
		})

		JustAfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				logprinter.PrintDebugLogs()
			}
			deleteRestore(sampleRestoreName)
			com.WaitForRestoreToDelete(KubeAccessor, sampleRestoreName, namespace)
		})

		It("Should collect restore and related CR yaml with logs", func() {
			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyRestoreLogs(sampleRestoreName, fileName, cRes, nRes)
		})

	})

	Context("When a restore of backed up application is in failed", func() {

		JustBeforeEach(func() {
			createRestoreForBackup(sampleRestoreName, customAvailableBackup, restoreNamespace, true)
			waitForRestore(sampleRestoreName, namespace, v1.Failed)
		})

		JustAfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				logprinter.PrintDebugLogs()
			}
			deleteRestore(sampleRestoreName)
		})

		It("Should collect restore and related CR yaml with logs", func() {
			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyRestoreLogs(sampleRestoreName, fileName, cRes, nRes)
		})
	})

	Context("When none of applications are installed", func() {

		It("Should collect installed CRDs, Trilio Application Pod yaml-logs, "+
			"Storage Class and VolumeSnapshot Class yaml", func() {

			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyResources(cRes, []string{CRD, StorageClass, VolumeSnapshotClass})

			verifyTrilioResources(cRes, nRes, fileName)

		})

		It("Should collect target browser, web, and backend resources yaml", func() {

			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyTargetWebAndBackendResources(cRes, nRes, fileName)

		})
	})

	Context("When Trilio Application is in failed state", func() {

		var dataStoreAttacherImage string

		JustBeforeEach(func() {

			controlPlane, err := KubeAccessor.GetDeployment(controlPlaneDeploymentKey.Namespace, controlPlaneDeploymentKey.Name)
			Expect(err).ShouldNot(HaveOccurred())

			envs := controlPlane.Spec.Template.Spec.Containers[0].Env
			var updatedEnvs []corev1.EnvVar
			for envIndex := range envs {
				env := envs[envIndex]
				if env.Name == com.DataStoreAttacherImage {
					dataStoreAttacherImage = env.Value
					updatedEnvs = append(envs[:envIndex], envs[envIndex+1:]...)
					break
				}
			}

			controlPlane.Spec.Template.Spec.Containers[0].Env = updatedEnvs
			err = KubeAccessor.UpdateDeployment(controlPlaneDeploymentKey.Namespace, controlPlane)
			Expect(err).ShouldNot(HaveOccurred())
		})

		JustAfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				logprinter.PrintDebugLogs()
			}

			controlPlane, err := KubeAccessor.GetDeployment(controlPlaneDeploymentKey.Namespace, controlPlaneDeploymentKey.Name)
			Expect(err).ShouldNot(HaveOccurred())

			envs := controlPlane.Spec.Template.Spec.Containers[0].Env
			controlPlane.Spec.Template.Spec.Containers[0].Env = append(envs,
				corev1.EnvVar{Name: com.DataStoreAttacherImage, Value: dataStoreAttacherImage})
			err = KubeAccessor.UpdateDeployment(controlPlaneDeploymentKey.Namespace, controlPlane)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Should collect Trilio Application Pod yaml-logs", func() {

			targetNS := types.NamespacedName{Name: sampleTargetName + com.GenerateRandomString(6, true), Namespace: namespace}
			target := common.CreateTarget(targetNS, false, "")
			Expect(k8sClient.Create(ctx, target)).ShouldNot(HaveOccurred())

			Eventually(func() v1.Status {
				log.Info("Checking if the target status has changed to [InProgress]")
				targetInstance := v1.Target{}
				Expect(k8sClient.Get(ctx, targetNS, &targetInstance)).To(BeNil())
				return targetInstance.Status.Status
			}, common.Timeout, common.Interval).Should(Equal(v1.InProgress))

			fileName, err := runLogCollector(true)
			Expect(err).Should(BeNil())

			cRes, nRes, err := readZipResources(fileName)
			Expect(err).Should(BeNil())

			verifyTrilioResources(cRes, nRes, fileName)

		})

	})

	Context("When incorrect arguments passed to log collector command", func() {

		It("Should fail if invalid namespace passed to log collector", func() {

			filePath := filepath.Join(projectRoot, logCollectorFilePath)
			flags := "--namespaces " + com.GenerateRandomString(6, true)
			cmd := fmt.Sprintf("go run %s %s", filePath, flags)
			log.Infof("Log Collector CMD [%s]", cmd)
			cmdOut, err := com.RunCmd(cmd)
			Expect(err).To(BeNil())
			Expect(cmdOut.Out).To(ContainSubstring("namespaces doesn't exists"))

		})

	})

})

func waitForRestore(restoreName, namespace string, status v1.Status) {
	log.Infof("Waiting for restore [%v] to [%v]", restoreName, status)
	Eventually(func() (v1.Status, error) {
		restore, err := KubeAccessor.GetRestore(restoreName, namespace)
		return restore.Status.Status, err
	}, "1200s", "5s").Should(Equal(status))
}

func waitForBackup(backupName, namespace string, status v1.Status) {
	log.Infof("Waiting for backup [%v] to [%v]", backupName, status)
	Eventually(func() (v1.Status, error) {
		backup, err := KubeAccessor.GetBackup(backupName, namespace)
		return backup.Status.Status, err
	}, "1200s", "5s").Should(Equal(status))
}

func getJobYaml(fileName, namespace, job string) *unstructured.Unstructured {
	jobYaml, err := ioutil.ReadFile(filepath.Join(getUnzipDir(fileName), Jobs, namespace, job))
	Expect(err).Should(BeNil())

	var j map[string]interface{}

	err = yaml.Unmarshal(jobYaml, &j)
	Expect(err).Should(BeNil())
	j["Kind"] = "Job"

	jobStruct := &unstructured.Unstructured{}
	jobStruct.SetGroupVersionKind(batchv1.SchemeGroupVersion.WithKind(com.JobKind))

	jobStruct.Object = j

	return jobStruct
}

func isRestoreResource(job *unstructured.Unstructured, restore *v1.Restore) bool {
	labels := job.GetLabels()
	annotations := job.GetAnnotations()
	if labels[com.ControllerOwnerUID] == string(restore.UID) &&
		annotations[com.ControllerOwnerName] == restore.Name &&
		annotations[com.ControllerOwnerNamespace] == restore.Namespace {
		return true
	}
	return false
}

func getRestoreJobYamls(fileName string, restore *v1.Restore, nRes map[string][]string) (validation string,
	dataRestoreMap map[string]string, metadataRestore string) {

	restoreNs := restore.Spec.RestoreNamespace

	dataRestoreMap = make(map[string]string)
	nsJobs := nRes[restoreNs]
	for _, jobName := range nsJobs {
		job := getJobYaml(fileName, restoreNs, jobName)
		if isRestoreResource(job, restore) {
			annotations := job.GetAnnotations()
			if annotations[com.Operation] == com.MetadataRestoreValidationOperation {
				validation = jobName
			} else if annotations[com.Operation] == com.MetadataRestoreOperation {
				metadataRestore = jobName
			} else if annotations[com.Operation] == com.DataRestoreOperation {
				dataRestoreMap[annotations[com.RestorePVCName]] = jobName
			}
		}
	}

	return validation, dataRestoreMap, metadataRestore
}

func getBackupJobYamls(fileName, backupNamespace string, backup *v1.Backup, nRes map[string][]string) (snapshotter string,
	datauploadMap map[string]string, retention, metadataUpload string) {

	datauploadMap = make(map[string]string)
	nsJobs := nRes[backupNamespace]
	for _, jobName := range nsJobs {
		job := getJobYaml(fileName, backupNamespace, jobName)
		if metav1.IsControlledBy(job, backup) {
			annotations := job.GetAnnotations()
			if annotations[com.Operation] == com.SnapshotterOperation {
				snapshotter = jobName
			} else if annotations[com.Operation] == com.RetentionOperation {
				retention = jobName
			} else if annotations[com.Operation] == com.MetadataUploadOperation {
				metadataUpload = jobName
			} else if annotations[com.Operation] == com.DataUploadOperation {
				datauploadMap[annotations[com.UploadPVCName]] = jobName
			}
		}
	}

	return snapshotter, datauploadMap, retention, metadataUpload
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func verifyBackupVolumeSnapshot(dataSnapshots []com.ApplicationDataSnapshot, volumeSnapshot []string) {

	for index := range dataSnapshots {
		dataSnapshot := dataSnapshots[index].DataComponent
		if dataSnapshot.VolumeSnapshot != nil &&
			dataSnapshot.VolumeSnapshot.VolumeSnapshot != nil {
			volumeSnapshotName := dataSnapshot.VolumeSnapshot.VolumeSnapshot.Name
			if !stringInSlice(volumeSnapshotName+".yaml", volumeSnapshot) {
				log.Infof("Volume Snapshot Yaml [%s] not found", volumeSnapshotName)
				Fail("Volume Snapshot Yaml not found")
			}
		}
	}
}

func verifyRestoreLogs(restoreName, fileName string, cRes map[string][]string, nRes map[string]map[string][]string) {

	log.Infof("Verifying restore [%v] with [%v]", restoreName, fileName)

	restore, err := KubeAccessor.GetRestore(restoreName, namespace)
	Expect(err).Should(BeNil())
	verifyResourceYaml(cRes, nRes, fileName, com.RestoreKind, restore)

	if restore.Spec.Source.Backup != nil {
		backupRef := restore.Spec.Source.Backup
		backup, err := KubeAccessor.GetBackup(backupRef.Name, backupRef.Namespace)
		Expect(err).Should(BeNil())
		verifyResourceYaml(cRes, nRes, fileName, com.BackupKind, backup)
	}

	validation, dataRestoreMap, metadataRestore := getRestoreJobYamls(fileName, restore, nRes[Jobs])

	dataComponents := controllerHelpers.GetRestoreApplicationDataComponents(restore.Status.RestoreApplication)

	if restore.Status.Status == v1.Failed {
		// Verify Restore Jobs
		if restore.Status.Phase == v1.RestoreValidation {
			Expect(validation).ShouldNot(BeEmpty())
		} else if restore.Status.Phase == v1.DataRestore {
			Expect(validation).ShouldNot(BeEmpty())
			Expect(len(dataRestoreMap)).ShouldNot(BeZero())
		} else if restore.Status.Phase == v1.MetadataRestore {
			Expect(validation).ShouldNot(BeEmpty())
			Expect(len(dataRestoreMap)).Should(Equal(len(dataComponents)))
			Expect(metadataRestore).ShouldNot(BeEmpty())
		}
	}
}

func getYamlName(name string) string {
	return name + ".yaml"
}

func getPodObject(object runtime.Object) *corev1.Pod {
	pod := &corev1.Pod{}
	unstructObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	Expect(err).ShouldNot(HaveOccurred())
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructObj, pod)
	Expect(err).ShouldNot(HaveOccurred())

	return pod
}

func verifyResourceYaml(cRes map[string][]string, nRes map[string]map[string][]string, fileName string, kind string,
	objects ...runtime.Object) {

	for index := range objects {
		object := objects[index]
		metaObj, err := meta.Accessor(object)
		Expect(err).ShouldNot(HaveOccurred())

		resourceSlice := cRes[kind]
		if metaObj.GetNamespace() != "" {
			resourceSlice = nRes[kind][metaObj.GetNamespace()]
		}
		if !stringInSlice(getYamlName(metaObj.GetName()), resourceSlice) {
			log.Infof("Yaml [%s] not found for Kind [%s]", metaObj.GetName(), kind)
			Fail("Yaml not found")
		}

		if kind == com.PodKind {
			pod := getPodObject(object)
			Expect(pod).ShouldNot(BeNil())

			for containerIndex := range pod.Status.ContainerStatuses {
				containerStatus := pod.Status.ContainerStatuses[containerIndex]
				if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
					logFileName := strings.Join([]string{metaObj.GetName(), containerStatus.Name, "curr", "log"}, ".")
					filePath := filepath.Join(getUnzipDir(fileName), Pod, metaObj.GetNamespace(), logFileName)
					_, err := os.Stat(filePath)
					if os.IsNotExist(err) {
						log.Infof("Log for pod [%s/%s] not found", metaObj.GetName(), metaObj.GetNamespace())
						Fail("Logs not found")
					}
				}
			}
		}
	}
}

func verifyBackupLogs(backupName, fileName string, cRes map[string][]string, nRes map[string]map[string][]string) {

	log.Infof("Verifying backup [%v] with [%v]", backupName, fileName)

	backup, err := KubeAccessor.GetBackup(backupName, namespace)
	Expect(err).Should(BeNil())
	verifyResourceYaml(cRes, nRes, fileName, com.BackupKind, backup)

	backupPlan, _ := KubeAccessor.GetBackupPlan(backup.Spec.BackupPlan.Name, backup.Namespace)
	verifyResourceYaml(cRes, nRes, fileName, com.BackupplanKind, backupPlan)

	backupNamespace := backupPlan.Namespace
	snapshotter, datauploadMap, retention, metadataUpload := getBackupJobYamls(fileName, backupNamespace,
		backup, nRes[Jobs])

	dataComponents, totalDataComponents := com.GetBackupDataComponents(backup.Status.Snapshot, false)

	if backup.Status.Status == v1.Failed {

		// Verify Volume Snapshot
		verifyBackupVolumeSnapshot(dataComponents, nRes[VolumeSnapshot][backupNamespace])

		// Verify Backup Jobs
		Expect(snapshotter).ShouldNot(BeEmpty())
		Expect(metadataUpload).ShouldNot(BeEmpty())
		if backup.Status.Phase == v1.DataUploadOperation {
			Expect(len(datauploadMap)).ShouldNot(BeZero())
		} else if backup.Status.Phase == v1.RetentionOperation {
			Expect(len(datauploadMap)).Should(Equal(totalDataComponents))
			Expect(retention).ShouldNot(BeEmpty())
		}
	}

}

func createBackupWithApp(applicationName, backupName string, isFailed bool) {
	application, _ := KubeAccessor.GetBackupPlan(applicationName, namespace)
	backup := createBackup(backupName, application)
	if isFailed {
		retentionJob := getBackupJobTemplate(backup, getRetentionJobAnnotations(backup),
			false, false)
		createJob(retentionJob)
	}
}

func createRestoreForBackup(restoreName, backupName, restoreNamespace string, isFailed bool) {
	targetNsNm := types.NamespacedName{Name: "sample-target", Namespace: namespace}
	restoreNsNm := types.NamespacedName{Name: restoreName, Namespace: namespace}
	backupNsNm := &types.NamespacedName{Name: backupName, Namespace: namespace}

	restore := common.CreateRestore(restoreNsNm, &targetNsNm, backupNsNm, nil, restoreNamespace, nil)
	Expect(k8sClient.Create(ctx, restore)).ShouldNot(HaveOccurred())
	if isFailed {
		metadataRestoreJob := getRestoreJobTemplate(restore, getMetadataJobAnnotations(restore),
			false, false)
		createJob(metadataRestoreJob)
	}
}

func createBackup(backupName string, application *v1.BackupPlan) *v1.Backup {
	log.Infof("Creating backup: %s", backupName)
	backup := common.CreateBackup(types.NamespacedName{Name: backupName, Namespace: namespace}, application, false, false)
	Expect(k8sClient.Create(ctx, backup)).ShouldNot(HaveOccurred())

	return backup
}

func deleteRestore(restoreName string) {
	log.Infof("Deleting restore [%v]", restoreName)
	Eventually(func() error {
		restore := &v1.Restore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      restoreName,
				Namespace: namespace,
			},
		}
		return k8sClient.Delete(ctx, restore)
	}, common.Timeout, common.Interval).Should(Succeed())
}

func deleteBackup(backupName string) {
	log.Infof("Deleting backup [%v]", backupName)
	Eventually(func() error {
		backup := &v1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupName,
				Namespace: namespace,
			},
		}
		return k8sClient.Delete(ctx, backup)
	}, common.Timeout, common.Interval).Should(Succeed())
}

func createJob(job *batchv1.Job) {
	By("Creating job")
	log.Infof("Creating job [%v]", job.Name)
	Eventually(func() error {
		return k8sClient.Create(ctx, job)
	}, common.Timeout, common.Interval).Should(Succeed())
}

func createPolicy(policyName string) {
	policy := common.CreateRetentionPolicy(types.NamespacedName{Name: policyName, Namespace: namespace}, 1)
	Expect(k8sClient.Create(ctx, policy)).ShouldNot(HaveOccurred())
}

func deletePolicy(policyName string) {
	Eventually(func() error {
		policy := &v1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: namespace,
			},
		}
		return k8sClient.Delete(ctx, policy)
	}, common.Timeout, common.Interval).Should(Succeed())
}

func createTarget(targetName string) {
	log.Infof("Creating target: %s", targetName)
	target := common.CreateTarget(types.NamespacedName{Name: targetName, Namespace: namespace}, false, "")
	target.Spec.EnableBrowsing = true
	Expect(k8sClient.Create(ctx, target)).ShouldNot(HaveOccurred())
	Eventually(func() (v1.Status, error) {
		target, err := KubeAccessor.GetTarget(sampleTargetName, namespace)
		return target.Status.Status, err
	}, "120s", "2s").Should(Equal(v1.Available))

}

func deleteTarget(targetName string) {
	Eventually(func() error {
		target := &v1.Target{
			ObjectMeta: metav1.ObjectMeta{
				Name:      targetName,
				Namespace: namespace,
			},
		}
		return k8sClient.Delete(ctx, target)
	}, common.Timeout, common.Interval).Should(Succeed())
}

func createApplication(appCRFilePath, applicationName string) {
	log.Infof("Creating application: %s", applicationName)
	Expect(KubeAccessor.Apply(namespace, filepath.Join(projectRoot, testYamls, appCRFilePath))).To(BeNil())
	Expect(com.SetBackupPlanStatus(KubeAccessor, applicationName, namespace, v1.Available)).ShouldNot(HaveOccurred())

}

func deleteApplication(applicationName string) {
	log.Infof("Deleting application [%v]", applicationName)
	Eventually(func() error {
		application := &v1.BackupPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      applicationName,
				Namespace: namespace,
			},
		}
		return k8sClient.Delete(ctx, application)
	}, common.Timeout, common.Interval).Should(Succeed())
}

func segregateTrilioPods(pods []corev1.Pod) (ctrlPlane, webhook, exporter *corev1.Pod) {
	for index := range pods {
		pod := pods[index]
		app := pod.GetLabels()["app"]
		if app == "k8s-triliovault-control-plane" {
			ctrlPlane = &pod
		} else if app == "k8s-triliovault-admission-webhook" {
			webhook = &pod
		} else if app == "k8s-triliovault-exporter" {
			exporter = &pod
		}
	}

	return ctrlPlane, webhook, exporter
}

func verifyTrilioResources(cRes map[string][]string, nRes map[string]map[string][]string, fileName string) {

	pods, err := KubeAccessor.GetPods(namespace)
	Expect(err).ShouldNot(HaveOccurred())

	ctrlPlane, webhook, exporter := segregateTrilioPods(pods)
	verifyResourceYaml(cRes, nRes, fileName, com.PodKind, ctrlPlane, webhook, exporter)
}

func verifyTargetWebAndBackendResources(cRes map[string][]string, nRes map[string]map[string][]string, fileName string) {

	var targetBrowserPod, webPod, backendPod *corev1.Pod
	var targetBrowserSvc, webSvc, backendSvc *corev1.Service
	var targetBrowserDeploy, webDeploy, backendDeploy *appsv1.Deployment
	var serviceList corev1.ServiceList
	var deployList appsv1.DeploymentList

	// listing target browser, web and backend pods
	pods, err := KubeAccessor.GetPods(namespace)
	Expect(err).ShouldNot(HaveOccurred())

	for index := range pods {
		pod := pods[index]
		app := pod.GetLabels()["app"]
		if strings.Contains(app, targetBrowserLabel) || strings.Contains(pod.ObjectMeta.Name, targetValidatorLabel) {
			targetBrowserPod = &pod
		} else if app == trilioWebLabel {
			webPod = &pod
		} else if app == trilioBackendLabel {
			backendPod = &pod
		}
	}

	// listing target browser, web and backend services
	svcListErr := k8sClient.List(ctx, &serviceList, &client.ListOptions{
		Namespace: namespace})
	Expect(svcListErr).To(BeNil())

	for si := range serviceList.Items {
		svc := serviceList.Items[si]
		app := svc.GetLabels()["app"]
		if strings.Contains(app, targetBrowserLabel) {
			targetBrowserSvc = &svc
		} else if svc.ObjectMeta.Name == trilioWebSvcLabel {
			webSvc = &svc
		} else if svc.ObjectMeta.Name == trilioBackedSvcLabel {
			backendSvc = &svc
		}
	}

	// listing target browser, web and backend deployments
	deployListErr := k8sClient.List(ctx, &deployList, &client.ListOptions{
		Namespace: namespace})
	Expect(deployListErr).To(BeNil())

	for di := range deployList.Items {
		deploy := deployList.Items[di]
		app := deploy.GetLabels()["app"]
		if strings.Contains(app, targetBrowserLabel) {
			targetBrowserDeploy = &deploy
		} else if app == trilioWebLabel {
			webDeploy = &deploy
		} else if app == trilioBackendLabel {
			backendDeploy = &deploy
		}
	}

	verifyResourceYaml(cRes, nRes, fileName, com.PodKind, targetBrowserPod, webPod, backendPod)
	verifyResourceYaml(cRes, nRes, fileName, com.ServiceKind, targetBrowserSvc, webSvc, backendSvc)
	verifyResourceYaml(cRes, nRes, fileName, com.DeploymentKind, targetBrowserDeploy, webDeploy, backendDeploy)
}

func verifyResources(cRes map[string][]string, resourceNames []string) {

	for _, name := range resourceNames {
		log.Infof("Checking Resource directory, %s", name)
		if _, ok := cRes[name]; !ok {
			log.Errorf("Resource directory not found, %s", name)
			Fail("Resource directory not found")
		}
	}

}

func readZipResources(fileName string) (map[string][]string, map[string]map[string][]string, error) {

	var clusteredResources = make(map[string][]string)
	var namespacedResources = make(map[string]map[string][]string)

	r, err := zip.OpenReader(fileName)
	if err != nil {
		fmt.Printf("%s", err.Error())
		return clusteredResources, namespacedResources, err
	}
	defer r.Close()

	for _, f := range r.File {
		dir, fileName := filepath.Split(f.Name)
		if fileName == "" {
			continue
		}
		dirPath := strings.Split(dir, "/")
		if len(dirPath) == 3 {
			resource := dirPath[1]
			clusteredResources[resource] = append(clusteredResources[resource], fileName)
		} else if len(dirPath) == 4 {
			resource := dirPath[1]
			ns := dirPath[2]
			if _, ok := namespacedResources[resource]; !ok {
				namespacedResources[resource] = make(map[string][]string)
			}
			namespacedResources[resource][ns] = append(namespacedResources[resource][ns], fileName)
		}
	}

	return clusteredResources, namespacedResources, nil
}

func clearDir(dir string) error {
	names, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), logCollectorPrefix) {
			_ = os.RemoveAll(path.Join([]string{dir, entry.Name()}...))
		}
	}
	return nil
}

func getLogCollectorFile(dir string) (string, error) {
	names, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), logCollectorPrefix) &&
			strings.HasSuffix(entry.Name(), ".zip") {
			return entry.Name(), nil
		}
	}
	return "", errors.New("file not found")
}

func getUnzipDir(fileName string) string {
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

//nolint:unparam
func runLogCollector(isClustered bool, namespaces ...string) (string, error) {

	err := clearDir(".")
	Expect(err).Should(BeNil())

	filePath := filepath.Join(projectRoot, logCollectorFilePath)
	flags := "--clustered"
	if !isClustered {
		flags = "--namespaces " + strings.Join(namespaces, ",")
	}
	cmd := fmt.Sprintf("go run %s %s", filePath, flags)
	log.Infof("Log Collector CMD [%s]", cmd)
	var cmdOut *com.CmdOut
	Eventually(func() error {
		cmdOut, err = com.RunCmd(cmd)
		log.Infof("Log Collector Output: %s", cmdOut.Out)
		return err
	}, common.Timeout*6, common.Interval).ShouldNot(HaveOccurred())

	log.Infof("Checking log collector file")
	fileName, err := getLogCollectorFile(".")
	Expect(err).To(BeNil())

	log.Infof("Unzipping file [%v]", fileName)
	_, err = unzip(fileName, ".")
	Expect(err).Should(BeNil())

	return fileName, nil
}

func getRetentionJobAnnotations(backup *v1.Backup) map[string]string {
	return map[string]string{
		com.ControllerOwnerName:      backup.Name,
		com.ControllerOwnerNamespace: backup.Namespace,
		com.Operation:                com.RetentionOperation,
	}
}

func getMetadataJobAnnotations(restore *v1.Restore) map[string]string {
	return map[string]string{
		com.ControllerOwnerName:      restore.Name,
		com.ControllerOwnerNamespace: restore.Namespace,
		com.Operation:                com.MetadataRestoreOperation,
	}
}

func getBackupJobTemplate(backup *v1.Backup, annotations map[string]string, isSuccessful bool, isDelayed bool) *batchv1.Job {
	sleep := 0
	if isDelayed {
		sleep = 20
	}
	exitStatus := 1
	if isSuccessful {
		exitStatus = 0
	}
	command := fmt.Sprintf("sleep %v && exit %v", sleep, exitStatus)
	container := controllerHelpers.GetContainer("container", com.AlpineImage, command, false,
		com.NonDMJobResource, com.MountCapability)
	job := controllerHelpers.GetJob(backup.Name, backup.Namespace, container, []corev1.Volume{},
		controllerHelpers.GetAuthResourceName(backup.UID, com.BackupKind))
	job.SetAnnotations(annotations)
	_ = ctrl.SetControllerReference(backup, job, scheme)

	return job
}

func getRestoreJobTemplate(restore *v1.Restore, annotations map[string]string, isSuccessful bool, isDelayed bool) *batchv1.Job {
	sleep := 0
	if isDelayed {
		sleep = 20
	}
	exitStatus := 1
	if isSuccessful {
		exitStatus = 0
	}
	command := fmt.Sprintf("sleep %v && exit %v", sleep, exitStatus)
	container := controllerHelpers.GetContainer("container", com.AlpineImage, command, false,
		com.NonDMJobResource, com.MountCapability)
	job := controllerHelpers.GetJob(restore.Name, restore.Spec.RestoreNamespace, container, []corev1.Volume{},
		controllerHelpers.GetAuthResourceName(restore.UID, com.RestoreKind))
	job.SetAnnotations(annotations)
	ownerLabels := GetRestoreJobLabels(restore)
	for k, v := range ownerLabels {
		job.Labels[k] = v
	}

	return job
}

func unzip(src string, dest string) ([]string, error) {

	var filenames []string
	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()
	for _, f := range r.File {
		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)
		filenames = append(filenames, fpath)
		if f.FileInfo().IsDir() {
			// Make Folder
			_ = os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}
		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		_, err = io.Copy(outFile, rc)
		// Close the file without defer to close before next iteration of loop
		_ = outFile.Close()
		_ = rc.Close()
		if err != nil {
			return filenames, err
		}
	}

	return filenames, nil
}
