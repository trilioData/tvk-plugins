//nolint
package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cast"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-test/deep"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	guid "github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart/loader"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	apiTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	utilretry "k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	crd "github.com/trilioData/k8s-triliovault/api/v1"
	controllerHelpers "github.com/trilioData/k8s-triliovault/controllers/helpers"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/apis"
	"github.com/trilioData/k8s-triliovault/internal/decorator"
	helmutils "github.com/trilioData/k8s-triliovault/internal/helm_utils"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/tvkconf"
	"github.com/trilioData/k8s-triliovault/internal/utils"
	"github.com/trilioData/k8s-triliovault/internal/utils/retry"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
	backup2 "github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/backup"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/integrations"
	. "github.com/trilioData/k8s-triliovault/tests/integration/common/licensekeygen"
)

var (
	deleteArg                  = "delete"
	controlPlaneName           = "k8s-triliovault-control-plane"
	customResourceDir          = "CustomResource"
	MysqlOpDir                 = "mysql-operator"
	MysqlOpMultipleRevDir      = "mysql-operator-multiple-revisions"
	MysqlOpHelmChart           = "mysql-operator-chart"
	mysqlOperatorScript        = "mysqlOperator.sh"
	resourceCleanupScript      = "resourceCleanup.sh"
	mysqlOpCrFile              = "mysqlCluster.yaml"
	mysqlOpCrSecretFile        = "mysqlCluster-secret.yaml"
	etcdOperatorScript         = "etcdOperator.sh"
	etcdOperatorDir            = "etcd-operator"
	etcdOpCrdFile              = "etcd-cluster-crd.yaml"
	mysqlOpCrdFile             = "crd.yaml"
	commonDir                  = "common"
	UpgradeArg                 = "upgrade"
	installArg                 = "install"
	RollbackArg                = "rollback"
	licenseKeydir              = "license_keys"
	helmOpDir                  = "helm-operator"
	helmOperatorScript         = "helmOperator.sh"
	helmOpCrFile               = "helmOp-hr.yaml"
	helmOpScrtFile             = "helmOp-hr-secret.yaml"
	defaultSA                  = "k8s-triliovault"
	scheduleLabel              = "velero.io/schedule-name"
	sampleScheduleLabel        = "sample-label"
	sampleScheduleValue        = "sample-value"
	testCtx                    = context.Background()
	currentDir, _              = os.Getwd()
	licenseKeys                = []string{"triliodata.pub", "triliodata"}
	projectRoot                = GetProjectRoot()
	mySQLFillDataScript        = filepath.Join(projectRoot, "test-data", "mySQLFillData.sh")
	blockDataVerifyScript      = filepath.Join(projectRoot, "test-data", "blockDataVerify.sh")
	testCommonsDir             = filepath.Join(projectRoot, "tests", "integration", commonDir)
	err                        error
	kubeAccessor               *kube.Accessor
	ResourceDeploymentTimeout  = 600
	ResourceDeploymentInterval = 10
	IntegrationsBool           = []bool{true, false}
	LabelSelectorOperators     = []metav1.LabelSelectorOperator{metav1.LabelSelectorOpIn,
		metav1.LabelSelectorOpNotIn, metav1.LabelSelectorOpExists, metav1.LabelSelectorOpDoesNotExist}
	CloudProvders = []string{"aws", "gcp", "portworx", "seagate", "azure", "ibm"}
)

const (
	timeout  = time.Second * 130
	interval = time.Second * 1
)

func init() {
	log.Info("initializing common tests package")
	kubeAccessor = GetAccessor()
}

func GetProjectRoot() string {
	outPut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(string(outPut), "\n")
}

func GetApplicationScope() string {
	applicationScope, isPresent := os.LookupEnv(internal.AppScope)
	if !isPresent {
		panic("Application applicationScope not present.")
	}
	return applicationScope
}

func CreateTarget(targetKey apiTypes.NamespacedName, isObjectStore bool, targetDirectory string) *crd.Target {
	var target *crd.Target

	target = &crd.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetKey.Name,
			Namespace: targetKey.Namespace,
		},
	}

	if isObjectStore {
		accessKey, isAccessKeyPresent := os.LookupEnv(AWSAccessKeyID)
		secretKey, isSecretKeyPresent := os.LookupEnv(AWSSecretAccessKey)
		region, isRegionPresent := os.LookupEnv(AWSRegion)
		if !isAccessKeyPresent || !isSecretKeyPresent {
			panic("Object Store Access Key/Secret Key not present in env")
		}
		if !isRegionPresent {
			region = DefaultS3Region
		}
		target.Spec = crd.TargetSpec{
			Type:   crd.ObjectStore,
			Vendor: crd.AWS,
			ObjectStoreCredentials: crd.ObjectStoreCredentials{
				AccessKey:  accessKey,
				SecretKey:  secretKey,
				BucketName: targetDirectory,
				Region:     region,
			},
		}
	} else {
		nfsIPAddr, nfsServerPath, nfsOptions := GetNFSCredentials()
		NfsExport := fmt.Sprintf("%s:%s", nfsIPAddr, path.Join(nfsServerPath, targetDirectory))
		target.Spec = crd.TargetSpec{
			Type:   crd.NFS,
			Vendor: crd.Other,
			NFSCredentials: crd.NFSCredentials{
				NfsExport:  NfsExport,
				NfsOptions: nfsOptions,
			},
		}
	}

	return target
}

// GetInvalidTargetObject retruns an invalid target object to be used with negative test cases
func GetInvalidTargetObject(targetKey apiTypes.NamespacedName, isObjectStore bool, targetDirectory string) *crd.Target {
	target := &crd.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetKey.Name,
			Namespace: targetKey.Namespace,
		},
	}

	if isObjectStore {
		accessKey := "access"
		secretKey := "secret"
		region := "global"

		target.Spec = crd.TargetSpec{
			Type:   crd.ObjectStore,
			Vendor: crd.AWS,
			ObjectStoreCredentials: crd.ObjectStoreCredentials{
				AccessKey:  accessKey,
				SecretKey:  secretKey,
				BucketName: targetDirectory,
				Region:     region,
			},
		}
	} else {
		nfsIPAddr, nfsServerPath, nfsOptions := "0.0.0.0", "/non-existent/", "nfsvers=3"
		NfsExport := fmt.Sprintf("%s:%s", nfsIPAddr, path.Join(nfsServerPath, targetDirectory))
		target.Spec = crd.TargetSpec{
			Type:   crd.NFS,
			Vendor: crd.Other,
			NFSCredentials: crd.NFSCredentials{
				NfsExport:  NfsExport,
				NfsOptions: nfsOptions,
			},
		}
	}

	return target
}

func CreateS3Target(targetKey apiTypes.NamespacedName, bucketName string) *crd.Target {
	target := &crd.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetKey.Name,
			Namespace: targetKey.Namespace,
		},
	}
	accessKey, isAccessKeyPresent := os.LookupEnv(S3AccessKeyID)
	secretKey, isSecretKeyPresent := os.LookupEnv(S3SecretAccessKey)
	region, isRegionPresent := os.LookupEnv(S3Region)
	url := os.Getenv(S3URL)

	if !isAccessKeyPresent || !isSecretKeyPresent || !isRegionPresent {
		panic("Object Store Access Key/Secret Key/Region not present in env")
	}
	target.Spec = crd.TargetSpec{
		Type:   crd.ObjectStore,
		Vendor: crd.AWS,
		ObjectStoreCredentials: crd.ObjectStoreCredentials{
			URL:        url,
			AccessKey:  accessKey,
			SecretKey:  secretKey,
			BucketName: bucketName,
			Region:     region,
		},
	}
	return target
}

func CreateCleanupPolicy(policyKey apiTypes.NamespacedName, backupDays int) *crd.Policy {
	policy := &crd.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyKey.Name,
			Namespace: policyKey.Namespace,
		},
		Spec: crd.PolicySpec{
			Type:            crd.Cleanup,
			Default:         true,
			RetentionConfig: crd.RetentionConfig{Latest: 1},
			CleanupConfig: crd.CleanupConfig{
				BackupDays: &backupDays,
			},
		},
	}
	return policy
}

func CreateRetentionPolicy(policyKey apiTypes.NamespacedName, backupCount int) *crd.Policy {
	backupDays := 28
	policy := &crd.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyKey.Name,
			Namespace: policyKey.Namespace,
		},
		Spec: crd.PolicySpec{
			Type:            crd.Retention,
			Default:         true,
			RetentionConfig: crd.RetentionConfig{Latest: backupCount},
			CleanupConfig: crd.CleanupConfig{
				BackupDays: &backupDays,
			},
		},
	}
	return policy
}

func CreateHook(hookKey apiTypes.NamespacedName, pre, post crd.HookExecution) *crd.Hook {
	hook := &crd.Hook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hookKey.Name,
			Namespace: hookKey.Namespace,
		},
		Spec: crd.HookSpec{
			PreHook:  pre,
			PostHook: post,
		},
	}

	return hook
}

func CreateApplication(applicationKey apiTypes.NamespacedName, target *crd.Target, retentionPolicy *crd.Policy,
	hookConfig *crd.HookConfig) *crd.BackupPlan {

	targetRef := &corev1.ObjectReference{
		Kind:       target.Kind,
		Namespace:  target.Namespace,
		Name:       target.Name,
		UID:        target.UID,
		APIVersion: target.APIVersion,
	}

	var retentionRef *corev1.ObjectReference
	if retentionPolicy != nil {
		retentionRef = &corev1.ObjectReference{
			Kind:       retentionPolicy.Kind,
			Name:       retentionPolicy.Name,
			APIVersion: retentionPolicy.APIVersion,
		}
	}

	application := &crd.BackupPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationKey.Name,
			Namespace: applicationKey.Namespace,
		},
		Spec: crd.BackupPlanSpec{
			BackupConfig: crd.BackupConfig{
				Target:          targetRef,
				RetentionPolicy: retentionRef,
				SchedulePolicy: crd.SchedulePolicy{
					IncrementalCron: crd.CronSpec{Schedule: "0 10 15 * *"},
					FullBackupCron:  crd.CronSpec{Schedule: "0 10 15 * *"},
				},
			},
			BackupPlanComponents: crd.BackupPlanComponents{Custom: []metav1.LabelSelector{{MatchLabels: map[string]string{"label": "value"}}}},
			HookConfig:           hookConfig,
		},
	}

	return application
}

func CreateBackup(backupKey apiTypes.NamespacedName, backupPlan *crd.BackupPlan, isFull bool, isScheduled bool) *crd.Backup {

	applicationRef := &corev1.ObjectReference{
		Namespace: backupPlan.Namespace,
		Name:      backupPlan.Name,
	}

	backup := &crd.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupKey.Name,
			Namespace: backupKey.Namespace,
		},
		Spec: crd.BackupSpec{
			BackupPlan: applicationRef,
		},
	}

	if isFull {
		backup.Spec.Type = crd.Full
	} else {
		backup.Spec.Type = crd.Incremental
	}

	if isScheduled {
		backup.ObjectMeta.Annotations = map[string]string{
			internal.ScheduleType: string(crd.Periodic),
		}
	}

	return backup
}

func CreateRestore(restoreKey apiTypes.NamespacedName, targetKey, backupName *apiTypes.NamespacedName, location *string,
	restoreNamespace string, hookConfig *crd.HookConfig) *crd.Restore {

	restore := &crd.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreKey.Name,
			Namespace: restoreKey.Namespace,
		},
		Spec: crd.RestoreSpec{
			Source:               &crd.RestoreSource{},
			RestoreNamespace:     restoreNamespace,
			PatchIfAlreadyExists: true,
			HookConfig:           hookConfig,
		},
	}

	if backupName != nil {
		backupRef := &corev1.ObjectReference{
			Kind:      internal.BackupKind,
			Namespace: backupName.Namespace,
			Name:      backupName.Name,
		}
		restore.Spec.Source.Type = crd.BackupSource
		restore.Spec.Source.Backup = backupRef
	} else {
		restore.Spec.Source.Type = crd.LocationSource
		restore.Spec.Source.Location = *location
	}

	if targetKey != nil {
		restore.Spec.Source.Target = &corev1.ObjectReference{
			Kind:      internal.TargetKind,
			Namespace: targetKey.Namespace,
			Name:      targetKey.Name,
		}
	}

	return restore
}

// Kubernetes resources

func CreatePVC(namespace string, isBlock bool, sizeInMB uint16,
	dataSource *corev1.TypedLocalObjectReference, isCSIstorageClass bool) *corev1.PersistentVolumeClaim {

	nameIdentifier := guid.New().String()
	volumeMode := corev1.PersistentVolumeFilesystem
	if isBlock {
		volumeMode = corev1.PersistentVolumeBlock
	}
	storage := *resource.NewQuantity(int64(sizeInMB), resource.BinarySI)

	var storageClassName *string
	if !isCSIstorageClass {
		storageClassName = &StandardStorageClass
	} else {
		csiStorageClass, isCSIStorageClassPresent := os.LookupEnv(StorageClassName)
		if !isCSIStorageClassPresent {
			panic("CSI Storage Class not present as environment variable")
		}
		storageClassName = &csiStorageClass
		storage, _ = resource.ParseQuantity(strconv.Itoa(int(sizeInMB)) + "Mi")
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       internal.PersistentVolumeClaimKind,
			APIVersion: corev1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-" + nameIdentifier,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storage},
			},
			StorageClassName: storageClassName,
			VolumeMode:       &volumeMode,
			DataSource:       dataSource,
		},
	}

	return pvc
}

func CreateDataInjectionContainer(pvc *corev1.PersistentVolumeClaim, isIncremental bool) *corev1.Container {

	volumeMode := *pvc.Spec.VolumeMode

	var injectionCommand string
	if volumeMode == corev1.PersistentVolumeBlock {
		if isIncremental {
			injectionCommand = fmt.Sprintf("dd if=/dev/urandom of=%s conv=notrunc bs=1M count=20", internal.PseudoBlockDevicePath)
		} else {
			injectionCommand = fmt.Sprintf("dd if=/dev/urandom of=%s bs=1M count=20", internal.PseudoBlockDevicePath)
		}
	} else {
		if isIncremental {
			injectionCommand = "dd if=/dev/urandom of=/sample/data/base conv=notrunc bs=1M count=10"
		} else {
			injectionCommand = "dd if=/dev/urandom of=/sample/data/base bs=1M count=20"
		}
	}

	container := &corev1.Container{
		Name:      "data-insertion",
		Image:     "alpine:latest",
		Command:   []string{"/bin/sh"},
		Args:      []string{"-c", injectionCommand},
		Resources: tvkconf.GetContainerResources(internal.NonDMJobResource),
	}

	// Add volumes device/mount based on volume type
	if volumeMode == corev1.PersistentVolumeBlock {
		container.VolumeDevices = []corev1.VolumeDevice{
			{
				Name:       internal.VolumeDeviceName,
				DevicePath: internal.PseudoBlockDevicePath,
			},
		}
	} else {
		container.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      internal.VolumeDeviceName,
				MountPath: "/sample/data",
			},
		}
	}

	return container
}

func CreatePod(namespace, volumeName string, container *corev1.Container,
	restartPolicy corev1.RestartPolicy, pvc *corev1.PersistentVolumeClaim) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-" + guid.New().String(),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: internal.ServiceAccountName,
			RestartPolicy:      restartPolicy,
			Containers:         []corev1.Container{*container},
			HostPID:            false,
			HostIPC:            false,
			HostNetwork:        false,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    &internal.RunAsRootUserID,
				RunAsNonRoot: &internal.RunAsNonRoot,
			},
		},
	}
	if pvc != nil {
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}

		if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
			emptyDirVol := corev1.Volume{
				Name: internal.EmptyDirVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, emptyDirVol)
		}
	}
	return pod
}

// Third Party resources

func CreateVolumeSnapshot(namespace, pvcName string, isAlphaCSI bool) *unstructured.Unstructured {

	volumeSnapshot := &unstructured.Unstructured{}

	volumeSnapshotClass, isvolumeSnapshotClassPresent := os.LookupEnv(VolumeSnapshotClass)
	if !isvolumeSnapshotClassPresent {
		panic("CSI Volume Snapshot Class not present as environment variable")
	}
	log.Infof("VolumeSnapshot class %s", volumeSnapshotClass)
	// Create Spec
	spec := map[string]interface{}{
		"volumeSnapshotClassName": volumeSnapshotClass,
		"source": map[string]interface{}{
			"persistentVolumeClaimName": pvcName,
		},
	}
	if isAlphaCSI {
		spec = map[string]interface{}{
			"snapshotClassName": volumeSnapshotClass,
			"source": map[string]interface{}{
				"name": pvcName,
				"kind": "PersistentVolumeClaim",
			},
		}
	}

	// Create object
	volumeSnapshot.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "volume-snapshot-" + guid.New().String(),
			"namespace": namespace,
		},
		"spec": spec,
	}

	// Set GVK to object
	volumeSnapshot.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   internal.SnapshotGroup,
		Version: internal.V1beta1Version,
		Kind:    internal.VolumeSnapshotKind,
	})
	if isAlphaCSI {
		volumeSnapshot.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   internal.SnapshotGroup,
			Version: internal.V1alpha1Version,
			Kind:    internal.VolumeSnapshotKind,
		})
	}

	return volumeSnapshot
}

// Get objects

func GenerateDataSnapshotContent(pvcVolumeSnapshot *unstructured.Unstructured, backupPVC *corev1.PersistentVolumeClaim,
	isAlphaCSI bool) crd.DataSnapshot {

	podContainersMap := []crd.PodContainers{
		{
			PodName:    "pod-" + internal.GenerateRandomString(6, false),
			Containers: []string{"cont-" + internal.GenerateRandomString(6, false)},
		},
	}

	volumeSnapshotAPIVersion := fmt.Sprintf("%s/%s", internal.SnapshotGroup, internal.V1beta1Version)
	if isAlphaCSI {
		volumeSnapshotAPIVersion = fmt.Sprintf("%s/%s", internal.SnapshotGroup, internal.V1alpha1Version)
	}

	var volumeSnapshotSize resource.Quantity
	volumeSnapshot := &crd.VolumeSnapshot{
		RetryCount: 0,
		Status:     crd.Pending,
	}
	if pvcVolumeSnapshot != nil {
		volumeSnapshotName := pvcVolumeSnapshot.GetName()
		restoreSize, found, err := unstructured.NestedString(pvcVolumeSnapshot.Object, "status", "restoreSize")
		if !found || err != nil {
			panic("Restore size not present in volume snapshot")
		}
		volumeSnapshotSize, err = resource.ParseQuantity(restoreSize)
		if err != nil {
			panic("Restore size not parsable in volume snapshot")
		}
		volumeSnapshot.VolumeSnapshot = &corev1.ObjectReference{
			UID:        pvcVolumeSnapshot.GetUID(),
			Kind:       internal.VolumeSnapshotKind,
			Name:       volumeSnapshotName,
			Namespace:  pvcVolumeSnapshot.GetNamespace(),
			APIVersion: volumeSnapshotAPIVersion,
		}
	}

	dataSnapshotContent := crd.DataSnapshot{
		PersistentVolumeClaimName:     backupPVC.Name,
		PersistentVolumeClaimMetadata: MarshalStruct(backupPVC, false),
		VolumeSnapshot:                volumeSnapshot,
		PodContainersMap:              podContainersMap,
		SnapshotSize:                  volumeSnapshotSize,
	}

	return dataSnapshotContent
}

func GenerateCRDMetadata(namespace string) string {

	name := internal.GenerateRandomString(6, true)
	group := "customcrd.triliovault.trilio.io"
	crd := &apiext.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "." + group,
			Namespace: namespace,
		},
		Spec: apiext.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiext.CustomResourceDefinitionNames{
				Plural:     name + "s",
				Singular:   name,
				ShortNames: nil,
				Kind:       strings.ToTitle(name),
				ListKind:   strings.ToTitle(name) + "List",
				Categories: nil,
			},
			Scope: "",
		},
	}

	return MarshalStruct(crd, false)
}

func GenerateComponentMetadata(namespace string) []crd.Resource {

	var componentMetadataList []crd.Resource

	// For Service
	serviceComponent := crd.Resource{
		GroupVersionKind: crd.GroupVersionKind{
			Version: "v1",
			Kind:    "Service",
		},
		Objects: []string{},
	}
	serviceCount := rand.Intn(5) + 1
	for i := 0; i < serviceCount; i++ {
		service := corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-" + guid.New().String(),
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     internal.GenerateRandomString(6, true),
					Protocol: corev1.ProtocolTCP,
					Port:     80,
				}},
				Selector:        map[string]string{"app": "nginx"},
				Type:            corev1.ServiceTypeClusterIP,
				SessionAffinity: corev1.ServiceAffinityNone,
			},
		}
		metadata, err := json.Marshal(service)
		if err != nil {
			panic("Error while marshaling service")
		}
		serviceComponent.Objects = append(serviceComponent.Objects, string(metadata))
	}

	// For StatefulSet
	statefulSetComponent := crd.Resource{
		GroupVersionKind: crd.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "StatefulSet",
		},
		Objects: []string{},
	}
	statefulSetCount := rand.Intn(5) + 1
	for i := 0; i < statefulSetCount; i++ {
		replicas := int32(3)
		revisionHistoryLimit := int32(2)
		partition := int32(2)
		statefulset := appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "statefulset-" + guid.New().String(),
				Namespace: namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas:            &replicas,
				ServiceName:         "nginx",
				PodManagementPolicy: appsv1.OrderedReadyPodManagement,
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type:          appsv1.RollingUpdateStatefulSetStrategyType,
					RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{Partition: &partition},
				},
				RevisionHistoryLimit: &revisionHistoryLimit,
			},
		}

		metadata, err := json.Marshal(statefulset)
		if err != nil {
			panic("Error while marshaling service")
		}
		statefulSetComponent.Objects = append(statefulSetComponent.Objects, string(metadata))
	}
	componentMetadataList = append(componentMetadataList, serviceComponent, statefulSetComponent)

	return componentMetadataList
}

func GenerateHelmMetadata() crd.Helm {
	release := internal.GenerateRandomString(6, true)
	helmSnapshot := crd.Helm{
		Release:  release,
		Resource: &GenerateComponentMetadata(release)[0],
	}

	helmSnapshot.Version = crd.Helm3
	helmSnapshot.StorageBackend = crd.Secret

	return helmSnapshot
}

func GenerateResourceMetadata(namespace string) string {

	// For Service only
	service := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-" + guid.New().String(),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     internal.GenerateRandomString(6, true),
				Protocol: corev1.ProtocolTCP,
				Port:     80,
			}},
			Selector:        map[string]string{"app": "nginx"},
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}

	return MarshalStruct(service, false)
}

func GenerateOperatorMetadata(namespace string, includeDatasnapshot bool) []crd.Operator {
	var operatorMetadata []crd.Operator
	for i := 0; i < internal.GenerateRandomInt(3, 6); i++ {
		operatorSnapshot := crd.Operator{
			OperatorID:        internal.GenerateRandomString(6, true),
			CustomResources:   GenerateComponentMetadata(namespace),
			OperatorResources: GenerateComponentMetadata(namespace),
		}
		if includeDatasnapshot {
			operatorSnapshot.DataSnapshots = GenerateDataComponentList(namespace)
		}
		operatorMetadata = append(operatorMetadata, operatorSnapshot)
	}

	return operatorMetadata
}

func GetOperatorSnapshotContents(namespace string) crd.Operator {
	operatorSnapshot := crd.Operator{
		OperatorID:        internal.GenerateRandomString(6, true),
		CustomResources:   GenerateComponentMetadata(namespace),
		OperatorResources: GenerateComponentMetadata(namespace),
		DataSnapshots:     []crd.DataSnapshot{},
	}
	return operatorSnapshot
}

func GeneratePVCDataComponent(namespace string) (*corev1.PersistentVolumeClaim, crd.DataSnapshot) {
	pvc := CreatePVC(namespace, false, 100, nil, true)
	return pvc, GenerateDataSnapshotContent(nil, pvc, true)
}

func GenerateDataComponentList(namespace string) []crd.DataSnapshot {
	var dataSnapshotList []crd.DataSnapshot
	for i := 0; i < internal.GenerateRandomInt(3, 6); i++ {
		_, dataSnapshot := GeneratePVCDataComponent(namespace)
		dataSnapshotList = append(dataSnapshotList, dataSnapshot)
	}
	return dataSnapshotList
}

// common functions

func GetNFSCredentials() (nfsIPAddr, nfsServerPath, nfsOptions string) {
	nfsIPAddr, isNFSIPAddrPresent := os.LookupEnv(NFSServerIPAddress)
	nfsServerPath, isNFSServerPathPresent := os.LookupEnv(NFSServerBasePath)
	nfsOptions, isNFSOptionsPresent := os.LookupEnv(NFSServerOptions)
	if !isNFSIPAddrPresent || !isNFSServerPathPresent || !isNFSOptionsPresent {
		panic("NFS Credentials not present in env")
	}

	return nfsIPAddr, nfsServerPath, nfsOptions
}

// TODO: Make this as generic solution for multiple targets and of different types
func CreateTargetSecret(targetName string) string {

	nfsIPAddr, nfsServerPath, nfsOptions := GetNFSCredentials()

	nfsMetadata := map[string]interface{}{
		"mountOptions": nfsOptions,
		"server":       nfsIPAddr,
		"share":        nfsServerPath,
	}

	nfsdatastore := map[string]interface{}{
		"name":             targetName,
		"storageType":      "nfs",
		"defaultDatastore": "yes",
		"metaData":         nfsMetadata,
	}

	datastore := []map[string]interface{}{nfsdatastore}

	return MarshalStruct(map[string]interface{}{"datastore": datastore}, true)
}

func CreateS3TargetSecret(targetName string) string {
	accessKey, isAccessKeyPresent := os.LookupEnv(AWSAccessKeyID)
	secretKey, isSecretKeyPresent := os.LookupEnv(AWSSecretAccessKey)
	region, isRegionPresent := os.LookupEnv(AWSRegion)
	if !isAccessKeyPresent || !isSecretKeyPresent {
		panic("Object Store Access Key/Secret Key not present in env")
	}
	if !isRegionPresent {
		region = DefaultS3Region
	}

	s3MetaData := map[string]interface{}{
		"storageDasDevice":  "none",
		"storageNFSSupport": "TrilioVault",
		"accessKeyID":       accessKey,
		"accessKey":         secretKey,
		"s3Bucket":          S3BucketName,
		"regionName":        region,
	}

	s3Datastore := map[string]interface{}{
		"name":        targetName,
		"storageType": "s3",
		"metaData":    s3MetaData,
	}
	datastore := []map[string]interface{}{s3Datastore}

	return MarshalStruct(map[string]interface{}{"datastore": datastore}, true)
}

func MarshalStruct(v interface{}, isYaml bool) string {

	var fstring []byte
	var err error
	if isYaml {
		fstring, err = yaml.Marshal(v)
	} else {
		fstring, err = json.Marshal(v)
	}

	if err != nil {
		panic("Error while marshaling")
	}
	return string(fstring)
}

func RetryOptions() []retry.Option {
	out := make([]retry.Option, 0, 2)
	out = append(out, DefaultRetryTimeout, DefaultRetryDelay, DefaultRetryCount)
	return out
}

func JobCheckRetryOptions() []retry.Option {
	out := make([]retry.Option, 0, 2)
	out = append(out, retry.Timeout(time.Minute*2), DefaultRetryDelay, DefaultRetryCount)
	return out
}

func CheckJobActive(k8sClient client.Client, jobKey apiTypes.NamespacedName) error {

	_, err := retry.Do(func() (result interface{}, completed bool, err error) {
		job := &batchv1.Job{}
		pErr := k8sClient.Get(context.Background(), jobKey, job)
		if pErr != nil {
			return nil, completed, pErr
		}
		if job.Status.Active > 0 {
			completed = true
			return nil, completed, pErr
		}
		return nil, completed, nil
	}, JobCheckRetryOptions()...)

	return err
}

func CheckJobCompleted(k8sClient client.Client, jobKey apiTypes.NamespacedName, ignoreNotFound bool) {

	Eventually(func() bool {
		job := &batchv1.Job{}
		log.Infof("waiting for %+v job to complete", jobKey)
		pErr := k8sClient.Get(context.Background(), jobKey, job)
		if pErr != nil && ignoreNotFound && apierrors.IsNotFound(pErr) {
			log.Infof("failed to get %+v job", jobKey)
			return true
		}
		log.Infof("%+v job status success-> %v, failed -> %v", jobKey, job.Status.Succeeded, job.Status.Failed)
		if job.Status.Failed > 0 || job.Status.Succeeded > 0 || controllerHelpers.IsJobConditionFailed(job) {
			log.Infof("%+v job is completed", jobKey)
			return true
		}
		return false
	}, 5*time.Minute, time.Millisecond*300).Should(BeTrue())

}

func CheckPodSucceeded(k8sClient client.Client, podKey apiTypes.NamespacedName) error {

	_, err := retry.Do(func() (result interface{}, completed bool, err error) {
		pod := &corev1.Pod{}
		pErr := k8sClient.Get(context.Background(), podKey, pod)
		if pErr != nil {
			return nil, completed, pErr
		}
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
			completed = true
			pErr = errors.New("pod status is not succeeded")
			return nil, completed, pErr
		}
		if pod.Status.Phase == corev1.PodSucceeded {
			completed = true
			return nil, completed, pErr
		}
		return nil, completed, nil
	}, RetryOptions()...)

	return err
}

func CheckVolumeSnapshotReadyToUse(k8sClient client.Client, volumeSnapshotKey apiTypes.NamespacedName,
	gvk schema.GroupVersionKind) (bool, error) {
	_, err = retry.Do(func() (result interface{}, completed bool, err error) {
		log.Infof("waiting for volumesnapshot %+v to be in readyToUse", volumeSnapshotKey)
		volumeSnapshot := &unstructured.Unstructured{}
		volumeSnapshot.SetGroupVersionKind(gvk)
		pErr := k8sClient.Get(context.Background(), volumeSnapshotKey, volumeSnapshot)
		if pErr != nil {
			return false, true, pErr
		}

		// Check Snapshot Status
		readyToUse, readyToUseFound, uErr := unstructured.NestedBool(volumeSnapshot.Object, "status", "readyToUse")
		if uErr != nil {
			return false, false, uErr
		}
		log.Infof("%+v readyToUse-> %v, readyToUseFound-> %v",
			volumeSnapshotKey, readyToUse, readyToUseFound)
		if readyToUseFound && readyToUse {
			return true, true, nil
		}

		// Check Snapshot Error
		_, errorFound, eErr := unstructured.NestedMap(volumeSnapshot.Object, "status", "error")
		if eErr != nil {
			return false, false, eErr
		}
		if errorFound {
			log.Infof("error in volume snapshot %s/%s", volumeSnapshot.GetName(), volumeSnapshot.GetNamespace())
			return false, true, eErr
		}
		return false, false, fmt.Errorf("volumesnapshot %+v is not ready to use", volumeSnapshotKey)
	}, retry.Timeout(time.Minute*5), retry.Delay(time.Second*5), retry.Count(60))

	if err != nil {
		log.Infof("failed to complete volumesnapshot %+v", volumeSnapshotKey)
		return false, fmt.Errorf("failed to complete volumesnapshot %+v", volumeSnapshotKey)
	}
	return true, err
}

func ScaleDeployment(k8sClient client.Client, namespacedName apiTypes.NamespacedName, replica int32) error {

	retryErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
		deployment := &appsv1.Deployment{}
		log.Infof("retriving deployment -> %+v", namespacedName)
		if err = k8sClient.Get(context.Background(), apiTypes.NamespacedName{
			Namespace: namespacedName.Namespace,
			Name:      namespacedName.Name,
		}, deployment); err != nil {
			log.Infof("failed to retrieve deployment -> %+v", namespacedName)
			return err
		}
		log.Infof("successfully retrieved deployment -> %+v", namespacedName)
		deployment.Spec.Replicas = &replica
		log.Infof("updating deployment -> %+v", namespacedName)
		if err = k8sClient.Update(context.Background(), deployment); err != nil {
			log.Infof("failed to update deployment -> %+v", namespacedName)
			return err
		}
		log.Infof("successfully updated deployment -> %+v", namespacedName)
		return nil
	})
	Expect(retryErr).ShouldNot(HaveOccurred())
	// wait for previous pod deletion
	time.Sleep(time.Second * 5)

	Eventually(func() error {
		return kubeAccessor.WaitUntilDeploymentIsReady(namespacedName.Namespace, namespacedName.Name)
	}, timeout, interval).Should(Succeed())

	Eventually(func() bool {
		var deploy *appsv1.Deployment
		deploy, err = kubeAccessor.GetDeployment(namespacedName.Namespace, namespacedName.Name)
		Expect(err).ShouldNot(HaveOccurred())
		log.Infof("deployment %+v expected replicas -> %v, Available replicas -> %v",
			namespacedName, *deploy.Spec.Replicas, deploy.Status.AvailableReplicas)
		return *deploy.Spec.Replicas == deploy.Status.AvailableReplicas
	}, timeout, interval).Should(BeTrue())

	return nil
}

func ScaleControlPlane(ns string, replica int, a *kube.Accessor) {
	controlPlaneDeploymentKey := types.NamespacedName{
		Name:      TVControlPlaneDeployment,
		Namespace: ns,
	}
	log.Infof("Setting control plane deployment replicas to %d", replica)
	Eventually(func() error {
		return ScaleDeployment(a.GetKubeClient(), controlPlaneDeploymentKey, int32(replica))
	}, timeout, interval).ShouldNot(HaveOccurred())
}

func InstallMysqlHelm(helmVersion, namespace string, extraParams []string) error {
	var binary string
	var helmCreate []string

	if helmVersion == string(crd.Helm3) {
		binary = internal.HelmVersionV3Binary
		helmCreate = []string{"install", HelmMysqlV3, "--namespace", namespace, MySQLHelmChartPath}
	} else {
		fmt.Println("No possible helm version")
	}
	if len(extraParams) > 0 {
		helmCreate = append(helmCreate, extraParams...)
	}
	cmd := exec.Command(binary, helmCreate...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func DeleteMysqlHelm(helmVersion, namespace string) error {
	var binary string
	var helmDelete []string

	if helmVersion == "v3" {
		binary = "helm"
		helmDelete = []string{"delete", HelmMysqlV3, "--namespace", namespace}
	} else {
		fmt.Println("No possible helm version")
	}

	cmd := exec.Command(binary, helmDelete...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func InstallCockroachdbHelm(release, helmVersion, namespace string) error {
	var binary string
	var helmCreate []string

	if helmVersion == string(crd.Helm3) {
		binary = internal.HelmVersionV3Binary
		helmCreate = []string{"install", release, "--namespace", namespace, MySQLHelmChartPath}
	} else {
		fmt.Println("No possible helm version")
	}
	cmd := exec.Command(binary, helmCreate...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func InstallHelmApp(helmVersion, namespace, releaseName string, appPath string, extraParams ...string) error {
	var binary string
	var helmCreate []string

	log.Infof("installing helm chart release name: %s, at path: %s, namespace: %s and version: %s",
		releaseName, appPath, namespace, helmVersion)

	if helmVersion == string(crd.Helm3) {
		binary = internal.HelmVersionV3Binary
		helmCreate = []string{"install", releaseName, appPath, "--namespace", namespace, "--wait", "--timeout", "10m"}
	} else {
		fmt.Println("No possible helm version")
	}
	if len(extraParams) > 0 {
		helmCreate = append(helmCreate, extraParams...)
	}
	cmd := exec.Command(binary, helmCreate...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func UpgradeHelmApp(helmVersion, namespace, releaseName string, appPath string, extraParams ...string) error {
	var binary string
	var helmUpgrade []string

	log.Infof("upgrading helm release name: %s, at path: %s, namespace: %s and version: %s",
		releaseName, appPath, namespace, helmVersion)

	if helmVersion == string(crd.Helm3) {
		binary = internal.HelmVersionV3Binary
		helmUpgrade = []string{"upgrade", releaseName, appPath, "--namespace", namespace, "--wait"}
	} else {
		fmt.Println("No possible helm version")
	}
	if len(extraParams) > 0 {
		helmUpgrade = append(helmUpgrade, extraParams...)
	}
	cmd := exec.Command(binary, helmUpgrade...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func DeleteHelmApp(release, helmVersion, namespace string) error {
	var binary string
	var helmDelete []string

	if helmVersion == "v3" {
		binary = "helm"
		helmDelete = []string{"delete", release, "--namespace", namespace}
	} else {
		fmt.Println("No possible helm version")
	}

	log.Infof("Deleting helm release [%s]", release)
	cmd := exec.Command(binary, helmDelete...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Errorf("[%s] helm release deletion failed - %s", release, err.Error())
		return err
	}
	log.Infof("[%s] helm chart deleted", release)

	return nil
}
func DeleteCockroachdbHelm(release, helmVersion, namespace string) error {
	var binary string
	var helmDelete []string

	if helmVersion == "v3" {
		binary = "helm"
		helmDelete = []string{"delete", release, "--namespace", namespace}
	} else {
		fmt.Println("No possible helm version")
	}

	cmd := exec.Command(binary, helmDelete...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

//nolint
func SetTargetStatus(targetName, namespace string, reqStatus crd.Status) error {
	var targetCR *crd.Target

	log.Infof("Updating %s status to %s", targetName, reqStatus)
	Eventually(func() error {
		retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
			targetCR, err = kubeAccessor.GetTarget(targetName, namespace)
			if err != nil {
				log.Error(err)
				return err
			}
			targetCR.Status.Status = reqStatus

			err = kubeAccessor.StatusUpdate(targetCR)
			if err != nil {
				log.Warnf("Failed to update target status: %+v", err)
				return err
			}

			log.Infof("Updated %s status to %s", targetName, reqStatus)
			return nil
		})
		if retErr != nil {
			log.Errorf("failed to update target status [%s/%s] ", targetName, namespace)
			return retErr
		}

		targetCR, err = kubeAccessor.GetTarget(targetName, namespace)
		Expect(err).ShouldNot(HaveOccurred())
		log.Infof("target %s/%s status -> %v", targetName, namespace, targetCR.Status.Status)
		if targetCR.Status.Status == reqStatus {
			log.Infof("target %s/%s  status -> %v updated successfully", targetName, namespace,
				targetCR.Status.Status)
			return nil
		}
		return fmt.Errorf("failed to update target status [%s/%s] ", targetName, namespace)
	}, timeout, time.Second*2).ShouldNot(HaveOccurred())

	return nil
}

func SetBackupPlanStatus(KubeAccessor *kube.Accessor, appName, namespace string, reqStatus crd.Status) error {
	var appCr *crd.BackupPlan
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

func SetBackupStatus(KubeAccessor *kube.Accessor, backupName, namespace string, reqStatus crd.Status) error {
	var backupCr *crd.Backup

	log.Infof("Updating %s status to %s", backupName, reqStatus)

	retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {

		backupCr, err = KubeAccessor.GetBackup(backupName, namespace)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}

		backupCr.Status.Status = reqStatus
		err = KubeAccessor.StatusUpdate(backupCr)
		if err != nil {
			log.Errorf("Failed to update backup status:%+v", err)
			return err
		}

		log.Infof("Updated %s status to %s", backupName, reqStatus)
		return nil
	})
	return retErr
}

func getImageRegistry() string {
	host, present := os.LookupEnv(DockerRegistry)
	if !present {
		panic("GCR Host env var 'GCR_DOCKER_REGISTRY' not found in environment")
	}
	return host
}

func GetReleaseTagForProwTests() string {
	var imageTag string
	imageTag, present := os.LookupEnv(ReleaseImageTag)
	if !present {
		cmd := "git rev-parse HEAD"
		cmdout, err := shell.RunCmd(cmd)
		if err != nil {
			panic(fmt.Sprintf("Failed to get RELEASE_TAG: %v", err))
		}
		imageTag = strings.TrimSpace(cmdout.Out)
		log.Infof("RELEASE_TAG:%s, exitcode:%d", imageTag, cmdout.ExitCode)
		return imageTag
	}
	return imageTag
}

// GetDataAttacherCommand returns the data store attacher command
func GetDataAttacherCommand(namespace, targetName string) string {
	return fmt.Sprintf("python /opt/tvk/datastore-attacher/mount_utility/mount_by_target_crd/mount_datastores.py "+
		"--namespace=%s --target-name=%s", namespace, targetName)
}

// GetDataMoverImage returns name of datamover image need to use
func GetDataMoverImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, DataMoverImageName, GetReleaseTagForProwTests())
}

func GetDataValidationImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, DataMoverValidationImageName, GetReleaseTagForProwTests())
}

func GetBackupCleanerImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, BackupCleanerImageName, GetReleaseTagForProwTests())
}

func GetDataAttacherImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, DataStoreAttacherImageName, GetReleaseTagForProwTests())
}

func GetMetamoverImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, MetaMoverImageName, GetReleaseTagForProwTests())
}

func GetResourceCleanerImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, ResourceCleanerImage, GetReleaseTagForProwTests())
}

func GetBackupSchedularImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, BackupSchedularImage, GetReleaseTagForProwTests())
}

func GetBackupRetentionImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, BackupRetentionImage, GetReleaseTagForProwTests())
}

func GetHookImage() string {
	return fmt.Sprintf("%s/%s/%s:%s", getImageRegistry(),
		GCPProject, hookExecutorImage, GetReleaseTagForProwTests())
}

func getDataMoverContainer(command string, volumeMode *corev1.PersistentVolumeMode) *corev1.Container {
	containerName := "datamover"

	dataMoverContainer := controllerHelpers.GetContainer(containerName, GetDataMoverImage(),
		command, true, internal.DMJobResource, internal.DatamoverCap)

	// Attach devices based on volumeMode
	if volumeMode != nil {
		if *volumeMode == corev1.PersistentVolumeBlock {
			dataMoverContainer.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      internal.EmptyDirVolumeName,
					MountPath: internal.EmptyDirMountPath,
				},
			}
		} else {
			dataMoverContainer.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      internal.VolumeDeviceName,
					MountPath: internal.MountPath,
				},
			}
		}
	}

	libguestfsEnv := []corev1.EnvVar{
		{Name: internal.LibguestfsDebug, Value: internal.LibGuestfsEnable},
		{Name: internal.LibguestfsTrace, Value: internal.LibGuestfsEnable},
	}
	dataMoverContainer.Env = append(dataMoverContainer.Env, libguestfsEnv...)

	return dataMoverContainer
}

func GetRetentionContainer(backupInfo map[string]string) *corev1.Container {
	namespace, backupName, targetName := backupInfo["namespace"], backupInfo["backupName"], backupInfo["targetName"]
	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	retentionCommand := fmt.Sprintf("/opt/tvk/retention --backup-name %s --namespace %s",
		backupName, namespace)
	command := fmt.Sprintf("%s && %s", dataAttacherCommand, retentionCommand)

	return controllerHelpers.GetContainer(internal.RetentionOperation, GetBackupRetentionImage(), command, true,
		internal.NonDMJobResource, internal.MountCapability)
}

func GetDataUploadContainer(backupInfo map[string]string, pvc *corev1.PersistentVolumeClaim,
	dataSnapshot *helpers.ApplicationDataSnapshot) *corev1.Container {
	namespace, backupName, previousBackupName, targetName := backupInfo["namespace"], backupInfo["backupName"],
		backupInfo["preBackupName"], backupInfo["targetName"]
	volumeMode := pvc.Spec.VolumeMode
	volumePath := internal.MountPath
	blockDeviceDetect := ""
	if *volumeMode == corev1.PersistentVolumeBlock {
		volumePath = internal.PseudoBlockDevicePath
		blockDeviceDetect = controllerHelpers.DetectBlockDeviceCommand
	}
	dataAttacherCommmand := GetDataAttacherCommand(namespace, targetName)
	dataMoverCommmand := fmt.Sprintf("/opt/tvk/datamover --action=%s --namespace=%s --backup-name=%s --previous-backup-name=%s"+
		" --target-name=%s --app-component=%s --component-identifier=%s --pvc-name=%s --volume-path=%s",
		internal.BackupDataAction, namespace, backupName, previousBackupName,
		targetName, string(dataSnapshot.AppComponent), dataSnapshot.ComponentIdentifier,
		pvc.Name, volumePath)
	command := fmt.Sprintf("%s %s && %s", blockDeviceDetect, dataAttacherCommmand, dataMoverCommmand)
	fmt.Println(command)
	return getDataMoverContainer(command, volumeMode)
}

func GetRestoreDatamoverContainer(restoreInfo map[string]string, pvc *corev1.PersistentVolumeClaim,
	appDs *helpers.ApplicationDataSnapshot) *corev1.Container {
	namespace, restoreName, targetName := restoreInfo["namespace"], restoreInfo["restoreName"], restoreInfo["targetName"]
	volumeMode := pvc.Spec.VolumeMode
	volumePath := internal.MountPath
	blockDeviceDetect := ""
	if *volumeMode == corev1.PersistentVolumeBlock {
		volumePath = internal.PseudoBlockDevicePath
		blockDeviceDetect = controllerHelpers.DetectBlockDeviceCommand
	}
	dataAttacherCommmand := GetDataAttacherCommand(namespace, targetName)
	dataMoverCommmand := fmt.Sprintf("/opt/tvk/datamover --action=%s --namespace=%s --restore-name=%s"+
		" --target-name=%s --app-component=%s --component-identifier=%s --pvc-name=%s --volume-path=%s",
		internal.RestoreDataAction, namespace, restoreName,
		targetName, appDs.AppComponent, appDs.ComponentIdentifier, pvc.Name, volumePath)
	command := fmt.Sprintf("%s %s && %s", blockDeviceDetect, dataAttacherCommmand, dataMoverCommmand)
	return getDataMoverContainer(command, volumeMode)
}

func GetStorageClass() string {
	storageClass, isStorageClassPresent := os.LookupEnv(StorageClassName)
	if !isStorageClassPresent {
		panic("Storage Class not present as environment variable")
	}

	return storageClass
}

func GetGVKString(gvk *crd.GroupVersionKind) string {
	// TODO: should use the below method to convert gvk to string
	// schGvk := schema.GroupVersionKind{Version: gvk.Version, Group: gvk.Group, Kind: gvk.Kind}
	// apiVersion, kind := schGvk.ToAPIVersionAndKind()
	// return apiVersion + kind
	if gvk.Group == "" {
		return gvk.Version + ",kind=" + gvk.Kind
	}
	return gvk.Group + "/" + gvk.Version + ",kind=" + gvk.Kind
}

func GetNewResourceMap(newRes []crd.Resource) map[string][]string {
	resMap := make(map[string][]string)
	for i := range newRes {
		res := newRes[i]
		gvk := GetGVKString(&res.GroupVersionKind)
		if val, ok := resMap[gvk]; ok {
			resMap[gvk] = append(val, res.Objects...)
			continue
		}
		resMap[gvk] = res.Objects
	}
	return resMap
}
func GetGVKObject(gvkStr string) schema.GroupVersionKind {
	gvkSlice := strings.Split(gvkStr, ",")
	kind := strings.Split(gvkSlice[1], "=")[1]
	return schema.FromAPIVersionAndKind(gvkSlice[0], kind)
}

func CheckResourceName(key string, gvkMetaList []string) bool {
	for _, val := range gvkMetaList {
		if val == key {
			return true
		}
	}
	return false
}

func GetBackupObject(cl client.Client, name, backupNamespace string) (*crd.Backup, error) {
	var backup crd.Backup
	if err := cl.Get(testCtx, apiTypes.NamespacedName{
		Namespace: backupNamespace,
		Name:      name,
	}, &backup); err != nil {
		log.Infof("ERROR:: %s", err.Error())
		return &crd.Backup{}, err
	}
	return &backup, nil
}

func CheckIfAllReqPvcPresent(reqPvcList []string, resPvcList []string) bool {
	if len(resPvcList) != len(reqPvcList) {
		log.Errorf("Invalid number of items in req and res pvc list. ReqPvcList: [%d], ResPvcList: [%d]",
			len(reqPvcList), len(resPvcList))
		log.Infof("required pvc list: =====> %+v", reqPvcList)
		log.Infof("response pvc list: =====> %+v", resPvcList)
		return false
	}

	var isPresent bool
	for reqPvc := range reqPvcList {
		isPresent = false
		for resPvc := range resPvcList {
			if reqPvcList[reqPvc] == resPvcList[resPvc] {
				isPresent = true
				break
			}
		}
		if !isPresent {
			log.Errorf("%s PVC not found in the data snapshot PVC list", reqPvcList[reqPvc])
			return false
		}
	}

	return true
}

func InstallEtcdOperator(acc *kube.Accessor, ns, release string) {
	// apply EtcdCluster crd
	Expect(acc.Apply(ns, filepath.Join(projectRoot, testDataDir, etcdOperatorDir, etcdOpCrdFile))).NotTo(HaveOccurred())

	cmd, err := shell.RunCmd(strings.Join([]string{filepath.Join(projectRoot, testDataDir, etcdOperatorDir,
		etcdOperatorScript), installArg, release, ns}, internal.Space))
	log.Infof("Etcd operator installation: %s", cmd.Out)
	if err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}

	time.Sleep(30 * time.Second) // -> It should be 30 seconds atleast, pod takes a lot of time to come up
	Eventually(func() error {
		_, err = acc.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = acc.GetPods(ns, "etcd_cluster=etcd-cluster")
			return pods, lErr
		})
		return err
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(BeNil())
	Expect(err).To(BeNil())
}

func DeleteEtcdOperator(acc *kube.Accessor, ns, release string) {
	cmd, err := shell.RunCmd(strings.Join([]string{filepath.Join(projectRoot, testDataDir, etcdOperatorDir,
		etcdOperatorScript), deleteArg, release, ns}, internal.Space))
	log.Infof("Etcd operator deletion: %s", cmd.Out)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Error(fmt.Sprintf("Failed to delete the etcd operator release: %s", err.Error()))
		defer Fail("Failed to delete etcd operator")
	}

	pod := &corev1.Pod{}
	cl := acc.GetKubeClient()
	_ = cl.DeleteAllOf(testCtx, pod, client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(labels.Set{
		"etcd_cluster": "etcd-cluster"})}, client.InNamespace(ns), client.GracePeriodSeconds(0))

	// delete EtcdCluster crd
	// Expect(acc.Delete(ns, filepath.Join(projectRoot, testDataDir, etcdOperatorDir, etcdOpCrdFile))).NotTo(HaveOccurred())
}

func CheckResourcesInSnapshot(resourceMap map[string][]string, resources ...crd.Resource) {
	log.Infof("checking resource of len: %d", len(resources))
	for i := range resources {
		res := resources[i]
		if res.GroupVersionKind.Kind == "" && res.GroupVersionKind.Version == "" {
			continue
		}
		gvkStr := GetGVKString(&res.GroupVersionKind)
		log.Info(gvkStr)
		if _, exists := resourceMap[gvkStr]; !exists {
			gErr := fmt.Errorf("gvk not found in passed resourceMap, %s", gvkStr)
			log.Errorf(gErr.Error())
			Fail(gErr.Error())
		}
		for j := range res.Objects {
			obj := res.Objects[j]
			if !CheckResourceName(obj, resourceMap[gvkStr]) {
				log.Errorf("meta not found in resourceMap for GVK: %s, name: %s", gvkStr, obj)
				Fail("meta not found in resourceMap")
			}
		}
	}
}

//nolint:unparam
func getPodStatus(cl client.Client, ns, selectorKey, selectorVal string, numReqReplicas int) bool {
	var podList corev1.PodList
	if err := cl.List(testCtx, &podList,
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				selectorKey: selectorVal,
			}),
		}, client.InNamespace(ns)); err != nil {
		log.Error(err)
		return false
	}
	if len(podList.Items) != numReqReplicas {
		return false
	}

	// Check the all pods status
	for podIndex := range podList.Items {
		pod := podList.Items[podIndex]
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
	}
	return true
}

func CheckCustomResourcesStatus(cl client.Client, ns, uniqID string) bool {
	var (
		appLabel           string
		deploymentObj      appsv1.Deployment
		deploymentReplicas = 1
		deploymentName     = uniqID + "-" + "nginx-deployment"
		podObj             corev1.Pod
		podName            = uniqID + "-" + "nginx-pod"
		rawPodName         = uniqID + "-" + "pod-raw"
		stsObj             appsv1.StatefulSet
		stsReplicas        = 1
		stsName            = uniqID + "-" + "nginx-sts"
		rcObj              corev1.ReplicationController
		rcReplicas         = 1
		rcName             = uniqID + "-" + "nginx-rc"
		rsObj              appsv1.ReplicaSet
		rsReplicas         = 1
		rsName             = uniqID + "-" + "nginx-rs"
	)

	appLabel = "app"
	// Check Deployment status
	if err := cl.Get(testCtx, apiTypes.NamespacedName{Namespace: ns, Name: deploymentName}, &deploymentObj); err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}
	if !getPodStatus(cl, ns, appLabel, deploymentName, deploymentReplicas) {
		log.Infof("Component %s not up", deploymentName)
		return false
	}

	// Check pod status
	if err := cl.Get(testCtx, apiTypes.NamespacedName{
		Namespace: ns,
		Name:      podName,
	}, &podObj); err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}
	if podObj.Status.Phase != corev1.PodRunning {
		log.Infof("Component %s not up", podName)
		return false
	}

	// Check raw-pod status
	if err := cl.Get(testCtx, apiTypes.NamespacedName{
		Namespace: ns,
		Name:      rawPodName,
	}, &podObj); err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}
	if podObj.Status.Phase != corev1.PodRunning {
		log.Infof("Component %s not up", rawPodName)
		return false
	}

	// Check statefulset status
	if err := cl.Get(testCtx, apiTypes.NamespacedName{
		Namespace: ns,
		Name:      stsName,
	}, &stsObj); err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}
	if !getPodStatus(cl, ns, appLabel, stsName, stsReplicas) {
		log.Infof("Component %s not up", stsName)
		return false
	}

	if err := cl.Get(testCtx, apiTypes.NamespacedName{
		Namespace: ns,
		Name:      rcName,
	}, &rcObj); err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}
	if !getPodStatus(cl, ns, appLabel, rcName, rcReplicas) {
		log.Infof("Component %s not up", rcName)
		return false
	}

	if err := cl.Get(testCtx, apiTypes.NamespacedName{
		Namespace: ns,
		Name:      rsName,
	}, &rsObj); err != nil {
		log.Infof("ERROR:: %s", err.Error())
	}
	if !getPodStatus(cl, ns, appLabel, rsName, rsReplicas) {
		log.Infof("Component %s not up", rsName)
		return false
	}

	log.Info("All components in custom application are up")
	return true
}

func CheckResourceObjects(responseMap, resourceMap map[string][]string) bool {
	for i := range resourceMap {
		sort.Strings(resourceMap[i])
	}
	log.Info("Resources map from restore CR: ", responseMap)
	log.Info("Expected map for verification: ", resourceMap)

	if len(responseMap) != len(resourceMap) {
		log.Error("response and expected resource map len mismatch")
		return false
	}

	return reflect.DeepEqual(responseMap, resourceMap)
}

func CheckResources(resourceMap map[string][]string, resources ...crd.Resource) {
	log.Info("verifying Resources")
	responseMap := make(map[string][]string)
	for rIndex := range resources {
		res := resources[rIndex]
		gvkStr := GetGVKString(&res.GroupVersionKind)
		log.Info(gvkStr)
		if _, exists := resourceMap[gvkStr]; !exists {
			log.Errorf("Gvk not found in resourceMap, %s", gvkStr)
			if res.GroupVersionKind.Kind == internal.CRDKind {
				log.Warnf("CRD is added in the skipped list but not in resourceMap, skipping it: %s", gvkStr)
				continue
			}
			log.Info("Resources map : ", resources)
			log.Info("Expected map : ", resourceMap)
			Fail("GVK not found in resourceMap")
		}
		resourceObjects := res.Objects
		sort.Strings(resourceObjects)
		responseMap[gvkStr] = resourceObjects
	}
	if !CheckResourceObjects(responseMap, resourceMap) {
		Fail("meta name not found in resourceMap")
	}
	log.Info("Resources verification passed")
}

func CheckOperatorMetaSnapshot(resourceMap map[string]interface{}, opSnaps []crd.Operator) {
	log.Info("Verifying operator metadata snapshot with the resourceMap")
	for i := range opSnaps {
		snap := opSnaps[i]
		opExpectedMap := resourceMap[snap.OperatorID].(map[string]interface{})
		if len(snap.OperatorResources) > 0 {
			opResources := snap.OperatorResources
			CheckResources(opExpectedMap["operatorResources"].(map[string][]string), opResources...)
		}
		if snap.Helm != nil {
			if val, exists := opExpectedMap["helmSnapshot"]; exists {
				CheckHelmMetaSnapshot(val.(map[string]interface{}), []crd.Helm{*snap.Helm}, false)
				return
			}
			Fail("Helm present in snapshot but not in resourceMap")
		}
		opCustomResources := snap.CustomResources
		if len(opCustomResources) > 0 {
			CheckResources(opExpectedMap["customResources"].(map[string][]string), opCustomResources...)
		}
	}
	log.Info("Verified operator metadata snapshot with the resourceMap")
}

func InstallHelm3App(ns, releaseName, chartPath, scPath, storageClass string) {
	log.Infof("Installing %s helm v3 chart", releaseName)
	helmCmd := fmt.Sprintf("helm install %s %s -n %s --set %s=%s --wait --timeout=%ds",
		releaseName, chartPath, ns, scPath, storageClass, ResourceDeploymentTimeout)
	out, err := shell.RunCmd(helmCmd)
	if out != nil {
		log.Info(out.Out)
	}
	Expect(err).To(BeNil(), fmt.Sprintf("%s helm chart installation failed", releaseName))
	log.Infof("Installed %s helm chart", releaseName)
}

func UpgradeMysqlHelm3(ns, releaseName, chartPath, storageClass string) {
	mySQLCmd := fmt.Sprintf("helm upgrade %s %s -n %s --set persistence.storageClass=%s", releaseName,
		filepath.Join(projectRoot, testDataDir, chartPath), ns, storageClass)
	out, err := shell.RunCmd(mySQLCmd)
	log.Info(out.Out)
	Expect(err).To(BeNil(), "Mysql helm chart upgrading failed")
}

func RollbackMysqlHelm3(ns, releaseName string) {
	mySQLCmd := fmt.Sprintf("helm rollback %s -n %s", releaseName, ns)
	out, err := shell.RunCmd(mySQLCmd)
	log.Info(out.Out)
	Expect(err).To(BeNil(), "Mysql helm chart rollback failed")
}

func DeleteHelm3App(releaseName, ns string) {
	helmCmd := fmt.Sprintf("helm delete %s -n %s", releaseName, ns)
	out, _ := shell.RunCmd(helmCmd)
	log.Info(out.Out)
}

func InstallMysqlOperator(acc *kube.Accessor, ns, release string) {
	cmdo, err := shell.RunCmd(strings.Join([]string{filepath.Join(projectRoot, testDataDir, MysqlOpDir, mysqlOperatorScript),
		installArg, release, ns}, internal.Space))
	log.Info(cmdo.Out)
	if err != nil {
		log.Infof("ERROR:: %s", err.Error())
		Fail(fmt.Sprintf("ERROR:: %s", err.Error()))
	}
	// install MysqlCluster crd, it will be required
	Expect(acc.Apply(ns, filepath.Join(projectRoot, testDataDir, MysqlOpDir, mysqlOpCrdFile))).NotTo(HaveOccurred())
	Expect(acc.Apply(ns, filepath.Join(projectRoot, testDataDir, MysqlOpDir, mysqlOpCrSecretFile))).To(BeNil())
	Expect(acc.Apply(ns, filepath.Join(projectRoot, testDataDir, MysqlOpDir, mysqlOpCrFile))).To(BeNil())

	time.Sleep(30 * time.Second) // -> giving it 30 sec to start all pods

	Eventually(func() error {
		_, err = acc.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = acc.GetPods(ns, "app.kubernetes.io/managed-by=mysql.presslabs.org", "app.kubernetes.io/name=mysql")
			otherPods, _ := acc.GetPods(ns, "app=mysql-operator", fmt.Sprintf("release=%s", release))
			pods = append(pods, otherPods...)
			return pods, lErr
		})
		return err
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(BeNil())
}

func InstallHelmOperator(acc *kube.Accessor, ns, uniqId, release string) {
	log.Info("Installing helm operator")
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(projectRoot, testDataDir, helmOpDir, helmOperatorScript),
		installArg, release, ns}, internal.Space))
	log.Info(out.Out)
	Expect(err).To(BeNil())
	// install HelmRelease CR and secret
	Expect(acc.Apply(ns, filepath.Join(projectRoot, testDataDir, helmOpDir, helmOpScrtFile))).To(BeNil())
	Expect(acc.Apply(ns, filepath.Join(projectRoot, testDataDir, helmOpDir, helmOpCrFile))).To(BeNil())

	_, err = acc.WaitUntilPodsAreReady(func() ([]corev1.Pod, error) {
		return acc.GetPods(ns, "app=redis", fmt.Sprintf("release=%s", uniqId+"-redis"))
	})

	Eventually(func() error {
		sl, err := acc.GetConfigMaps(ns, "app=redis", fmt.Sprintf("release=%s", uniqId+"-redis"))
		Expect(err).To(BeNil())
		if len(sl.Items) == 3 {
			log.Info("Configmaps for redis helm release are up")
			return nil
		}

		log.Errorf("expected 2 Config maps to be up, but available: %d, %+v", len(sl.Items), sl.Items)
		return fmt.Errorf("expected 2 Config maps to be up, but available: %d, %+v", len(sl.Items), sl.Items)
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(BeNil())
}

func DeleteHelmOperator(acc *kube.Accessor, ns, uniqID, release string) {
	out, err := shell.RunCmd(strings.Join([]string{filepath.Join(projectRoot, testDataDir, helmOpDir, helmOperatorScript),
		deleteArg, release, ns}, internal.Space))
	log.Info(out.Out)
	Expect(err).To(BeNil())

	Expect(acc.Delete(ns, filepath.Join(projectRoot, testDataDir, helmOpDir, helmOpCrFile))).To(BeNil())
	Expect(acc.Delete(ns, filepath.Join(projectRoot, testDataDir, helmOpDir, helmOpScrtFile))).To(BeNil())

	// This service is creating conflict in the installation next time maybe with the same name
	acc.DeleteService(ns, uniqID+"-redis-headless")
	acc.DeleteService(ns, uniqID+"-redis-metrics")
	acc.DeleteService(ns, uniqID+"-redis-master")
	acc.DeleteUnstructuredObject(types.NamespacedName{Name: uniqID + "-redis-master", Namespace: ns},
		appsv1.SchemeGroupVersion.WithKind(internal.StatefulSetKind))

	Expect(acc.WaitUntilPodsAreDeleted(func() ([]corev1.Pod, error) {
		return acc.GetPods(ns, "app=redis",
			fmt.Sprintf("release=%s", release))
	})).To(BeNil())

	Eventually(func() error {
		sl, err := acc.GetConfigMaps(ns, "app=redis")
		if len(sl.Items) > 0 && err == nil {
			for _, s := range sl.Items {
				acc.DeleteConfigMap(ns, s.Name)
			}
			return errors.New("redis configmaps not yet deleted")
		}
		return nil
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(BeNil())

	Eventually(func() error {
		sl, err := acc.GetSecrets(ns, "app=redis")
		if len(sl.Items) > 0 && err == nil {
			for _, s := range sl.Items {
				acc.DeleteSecret(ns, s.Name)
			}
			return errors.New("redis secret not yet deleted")
		}
		return nil
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(BeNil())
}

func DeployOcpCustomApp(KubeAccessor *kube.Accessor, ns, ocpAppFilePath, uniqueID string) {
	log.Infof("Deploying ocp resources from %s", ocpAppFilePath)
	Expect(KubeAccessor.Apply(ns, ocpAppFilePath)).To(BeNil())

	// Wait till all the custom components comes in Running state
	Eventually(func() error {
		_, err := KubeAccessor.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = KubeAccessor.GetPods(ns, "app=nginx-deployment")
			stsPods, _ := KubeAccessor.GetPods(ns, "app=nginx-sts")
			rcPods, _ := KubeAccessor.GetPods(ns, "app=nginx-rc")
			rsPods, _ := KubeAccessor.GetPods(ns, "app=nginx-rs")
			otherPods, _ := KubeAccessor.GetPods(ns, fmt.Sprintf("triliobackupall=%s", uniqueID))
			pods = append(append(append(append(pods, stsPods...), rcPods...), rsPods...), otherPods...)
			return pods, lErr
		})
		return err
	}, timeout, interval).Should(gomega.BeNil())

}

func VerifyCustomResources(componentResources []crd.Resource, resourceMap map[string][]string) {
	log.Info("Verifying Resources")
	responseMap := make(map[string][]string)
	for componentIndex := range componentResources {
		resource := componentResources[componentIndex]
		gvkStr := GetGVKString(&resource.GroupVersionKind)
		log.Info(gvkStr)
		if _, exists := resourceMap[gvkStr]; !exists {
			log.Infof(fmt.Sprintf("GVK not found in the resourceMap: %s", gvkStr))
			Fail(fmt.Sprintf("GVK not found in the resourceMap: %s", gvkStr))
		}
		responseObjectList := resource.Objects
		sort.Strings(responseObjectList)
		responseMap[gvkStr] = responseObjectList
	}
	if !CheckMetaNameLists(responseMap, resourceMap) {
		Fail("object name not found in resourceMap")
	}
	log.Info("Custom Resource verification passed")
}

func CheckIfPVCExists(acc *kube.Accessor, ns string, reqPvcList []string) {
	Eventually(func() bool {
		for _, rPvc := range reqPvcList {
			pvc, err := acc.GetPersistentVolumeClaim(ns, rPvc)
			if err != nil {
				log.Error(err.Error())
				return false
			}

			if pvc.Status.Phase != corev1.ClaimBound {
				log.Warnf("PVC: [%s], is in [%s] status not Bounded", rPvc, pvc.Status.Phase)
				return false
			}

			log.Infof("PVC: [%s], Bound", rPvc)
		}

		return true
	}, time.Second*120, interval).Should(Equal(true))
}

func CheckMetaNameLists(responseMap, resourceMap map[string][]string) bool {
	if len(responseMap) == 0 && len(resourceMap) == 0 {
		return true
	}

	for i := range resourceMap {
		sort.Strings(resourceMap[i])
	}

	log.Info("responseMap=========>", responseMap)
	log.Info("resourceMap=========>", resourceMap)

	if len(responseMap) != len(resourceMap) {
		log.Error("response and expected resource map len mismatch")
		return false
	}

	return reflect.DeepEqual(responseMap, resourceMap)
}

func DeleteOcpCustomApp(KubeAccessor *kube.Accessor, ns, ocpAppFilePath string) {
	log.Infof("Deleting ocp resources from %s", ocpAppFilePath)
	Expect(KubeAccessor.Delete(ns, ocpAppFilePath)).To(BeNil())
}

func DeleteMysqlOperator(acc *kube.Accessor, ns, release string) {
	cmdo, err := shell.RunCmd(strings.Join([]string{filepath.Join(projectRoot, testDataDir, MysqlOpDir, mysqlOperatorScript),
		deleteArg, release, ns}, internal.Space))
	log.Info(fmt.Sprintf("Mysql deletion log: %s", cmdo.Out))
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Error(fmt.Sprintf("Failed to delete the mysql operator release: %s", err.Error()))
		//defer Fail("Failed to delete mysql operator")
	}

	DeleteMysqlOpResources(acc, ns)

	// delete  MysqlCluster crd, it will be required
	//Expect(acc.Delete(ns, filepath.Join(projectRoot, testDataDir, mysqlOpDir, mysqlOpCrdFile))).NotTo(HaveOccurred())
}

func DeleteMysqlOpResources(acc *kube.Accessor, ns string) {
	Expect(acc.Delete(ns, filepath.Join(projectRoot, testDataDir, MysqlOpDir, mysqlOpCrSecretFile))).To(BeNil())
	// crd cleanup is handled at the AfterSuite level
	_, err := shell.RunCmd(fmt.Sprintf("%s %s %s", filepath.Join(testCommonsDir, resourceCleanupScript), "pvc", ns))
	Expect(err).To(BeNil())
	_, err = shell.RunCmd(fmt.Sprintf("%s %s %s", filepath.Join(testCommonsDir, resourceCleanupScript), "mysqlcluster", ns))
	Expect(err).To(BeNil())
}

func DeployCustomApp(KubeAccessor *kube.Accessor, ns, customAppFileName, uniqID string) {
	filePath := filepath.Join(projectRoot, testDataDir, customResourceDir, customAppFileName)
	log.Infof("Deploying custom app from %s", filePath)
	Expect(KubeAccessor.Apply(ns, filePath)).To(BeNil())

	var timeoutDesc string
	Eventually(func() bool {
		if !CheckCustomResourcesStatus(KubeAccessor.GetKubeClient(), ns, uniqID) {
			cmdOut, _ := shell.RunCmd(fmt.Sprintf("kubectl get po -n %s", ns))
			timeoutDesc = fmt.Sprintf("resource is not up. Current scenario: %s", cmdOut.Out)
			return false
		}
		return true
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(Equal(true), timeoutDesc)
}

func DeleteCustomApp(KubeAccessor *kube.Accessor, ns, customAppFileName string) {
	err := KubeAccessor.Delete(ns, filepath.Join(projectRoot, testDataDir, customResourceDir, customAppFileName))
	Expect(err).To(BeNil())
	// deleting all the pods, which may be stuck in terminating state

	cl := KubeAccessor.GetKubeClient()
	sts := appsv1.StatefulSet{}
	sts.SetNamespace(ns)
	sts.SetName("nginx-sts")
	_ = cl.Delete(testCtx, &sts, client.GracePeriodSeconds(0))

	rs := appsv1.ReplicaSet{}
	rs.SetNamespace(ns)
	rs.SetName("nginx-rs")
	_ = cl.Delete(testCtx, &rs, client.GracePeriodSeconds(0))

	rc := corev1.ReplicationController{}
	rc.SetNamespace(ns)
	rc.SetName("nginx-rc")
	_ = cl.Delete(testCtx, &rc, client.GracePeriodSeconds(0))

	pods := corev1.Pod{}
	_ = cl.DeleteAllOf(testCtx, &pods, client.MatchingLabels{"app": "nginx-deployment"}, client.GracePeriodSeconds(0), client.InNamespace(ns))
	_ = cl.DeleteAllOf(testCtx, &pods, client.MatchingLabels{"triliobackupall": "all"}, client.GracePeriodSeconds(0), client.InNamespace(ns))
	_ = cl.DeleteAllOf(testCtx, &pods, client.MatchingLabels{"app": "nginx-sts"}, client.GracePeriodSeconds(0), client.InNamespace(ns))
	_ = cl.DeleteAllOf(testCtx, &pods, client.MatchingLabels{"name": "fluentd-elasticsearch"}, client.GracePeriodSeconds(0),
		client.InNamespace(ns))
	_ = cl.DeleteAllOf(testCtx, &pods, client.MatchingLabels{"app": "nginx-rc"}, client.GracePeriodSeconds(0), client.InNamespace(ns))
	_ = cl.DeleteAllOf(testCtx, &pods, client.MatchingLabels{"app": "nginx-rs"}, client.GracePeriodSeconds(0), client.InNamespace(ns))
	_, err = shell.RunCmd(fmt.Sprintf("%s %s %s", filepath.Join(testCommonsDir, resourceCleanupScript), "pvc", ns))
	Expect(err).To(BeNil())
}

func VerifyMetadataContent(metaMap map[string]map[string]*unstructured.Unstructured, resources ...apis.ComponentMetadata) {
	if len(resources) == 0 {
		return
	}

	var toFail bool
	for _, r := range resources {
		gvk := GetGVKString(&r.GroupVersionKind)
		m, present := metaMap[gvk]
		if !present {
			log.Error(fmt.Sprintf("gvk [%s], not present in the resource map", gvk))
			toFail = true
			continue
		}

		for _, meta := range r.Metadata {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{Group: r.GroupVersionKind.Group, Version: r.GroupVersionKind.Version,
				Kind: r.GroupVersionKind.Kind})
			Expect(yaml.Unmarshal([]byte(meta), obj)).To(BeNil())

			log.Infof("Checking resource metadata with name: %s in resourceMap", obj.GetName())

			mapObj, present := m[obj.GetName()]
			if !present {
				log.Errorf("Object: %s, not found in the map", obj.GetName())
				toFail = true
				continue
			}

			t := &decorator.UnstructResource{Object: mapObj.Object}
			t.Cleanup()
			mapObj.Object = t.Object

			t = &decorator.UnstructResource{Object: obj.Object}
			t.Cleanup()
			obj.Object = t.Object

			mapSpec, _, _ := unstructured.NestedMap(mapObj.Object, "spec")
			objSpec, _, _ := unstructured.NestedMap(obj.Object, "spec")

			obj.Object = removeIrrelevantFields(objSpec)
			mapObj.Object = removeIrrelevantFields(mapSpec)

			unstructured.RemoveNestedField(objSpec, "template", "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(objSpec, "jobTemplate", "metadata")
			unstructured.RemoveNestedField(objSpec, "jobTemplate", "spec", "template", "metadata")

			templateSpec, found, _ := unstructured.NestedMap(mapSpec, "template", "spec")
			if found {
				_ = unstructured.SetNestedField(mapSpec, removeIrrelevantFields(templateSpec), "template", "spec")
			}

			templateSpec, found, _ = unstructured.NestedMap(objSpec, "template", "spec")
			if found {
				_ = unstructured.SetNestedField(objSpec, removeIrrelevantFields(templateSpec), "template", "spec")
			}

			templateSpec, found, _ = unstructured.NestedMap(objSpec, "jobTemplate", "spec", "template", "spec")
			if found {
				_ = unstructured.SetNestedField(objSpec, removeIrrelevantFields(templateSpec), "jobTemplate", "spec", "template", "spec")
			}

			templateSpec, found, _ = unstructured.NestedMap(mapSpec, "jobTemplate", "spec", "template", "spec")
			if found {
				_ = unstructured.SetNestedField(mapSpec, removeIrrelevantFields(templateSpec), "jobTemplate", "spec", "template", "spec")
			}

			// checking them deep equal
			deep.LogErrors = true
			if !cmp.Equal(mapSpec, objSpec, cmpopts.EquateEmpty()) {
				log.Infof("map: %+v", mapSpec)
				log.Infof("obj: %+v", objSpec)
				log.Errorf("diff: %+v", cmp.Diff(mapSpec, objSpec, cmpopts.EquateEmpty()))
				toFail = true
				continue
			}
			log.Infof("Object: %s matched", obj.GetName())
		}
	}
	if toFail {
		Fail("Object are not equal")
	}
}

func removeIrrelevantFields(obj map[string]interface{}) map[string]interface{} {
	// removing metadata field from them
	// TODO: remove the below fields if either one has nil
	unstructured.RemoveNestedField(obj, "sessionAffinity")
	unstructured.RemoveNestedField(obj, "progressDeadlineSeconds")
	unstructured.RemoveNestedField(obj, "clusterIP")
	unstructured.RemoveNestedField(obj, "strategy")
	unstructured.RemoveNestedField(obj, "type")
	unstructured.RemoveNestedField(obj, "podManagementPolicy")
	unstructured.RemoveNestedField(obj, "revisionHistoryLimit")
	unstructured.RemoveNestedField(obj, "updateStrategy")
	unstructured.RemoveNestedField(obj, "reclaimPolicy")
	unstructured.RemoveNestedField(obj, "terminationGracePeriodSeconds")
	unstructured.RemoveNestedField(obj, "dnsPolicy")
	unstructured.RemoveNestedField(obj, "restartPolicy")
	unstructured.RemoveNestedField(obj, "schedulerName")
	unstructured.RemoveNestedField(obj, "privileged")
	unstructured.RemoveNestedField(obj, "allowPrivilegeEscalation")
	unstructured.RemoveNestedField(obj, "enableServiceLinks")
	unstructured.RemoveNestedField(obj, "securityContext")
	unstructured.RemoveNestedField(obj, "serviceAccountName")
	unstructured.RemoveNestedField(obj, "serviceAccount")
	unstructured.RemoveNestedField(obj, "priority")
	unstructured.RemoveNestedField(obj, "tolerations")
	unstructured.RemoveNestedField(obj, "ports", "protocol")
	unstructured.RemoveNestedField(obj, "ports", "targetPort")
	unstructured.RemoveNestedField(obj, "backoffLimit")
	unstructured.RemoveNestedField(obj, "completions")
	unstructured.RemoveNestedField(obj, "parallelism")
	unstructured.RemoveNestedField(obj, "concurrencyPolicy")
	unstructured.RemoveNestedField(obj, "failedJobsHistoryLimit")
	unstructured.RemoveNestedField(obj, "successfulJobsHistoryLimit")
	unstructured.RemoveNestedField(obj, "suspend")
	unstructured.RemoveNestedField(obj, "jobTemplate", "metadata")
	unstructured.RemoveNestedField(obj, "jobTemplate", "spec", "template", "metadata")
	val, found, _ := unstructured.NestedInt64(obj, "backoffLimit")
	if found && val == 6 {
		unstructured.RemoveNestedField(obj, "backoffLimit")
	}

	val, found, _ = unstructured.NestedInt64(obj, "parallelism")
	if found && val < 2 {
		unstructured.RemoveNestedField(obj, "parallelism")
	}

	val, found, _ = unstructured.NestedInt64(obj, "completions")
	if found && val < 2 {
		unstructured.RemoveNestedField(obj, "completions")
	}

	val, found, _ = unstructured.NestedInt64(obj, "failedJobsHistoryLimit")
	if found && val < 2 {
		unstructured.RemoveNestedField(obj, "failedJobsHistoryLimit")
	}

	val, found, _ = unstructured.NestedInt64(obj, "successfulJobsHistoryLimit")
	if found && val == 3 {
		unstructured.RemoveNestedField(obj, "successfulJobsHistoryLimit")
	}

	v, found, _ := unstructured.NestedString(obj, "concurrencyPolicy")
	if found && v == "Allow" {
		unstructured.RemoveNestedField(obj, "concurrencyPolicy")
	}

	b, found, _ := unstructured.NestedBool(obj, "suspend")
	if found && !b {
		unstructured.RemoveNestedField(obj, "suspend")
	}

	if v, exist := obj["volumes"]; !exist || v == nil {
		unstructured.RemoveNestedField(obj, "volumes")
	}

	vm, ok, _ := unstructured.NestedSlice(obj, "volumes")

	if ok {
		for i := range vm {
			vi := vm[i]
			if t := reflect.TypeOf(vi); t.String() == "string" {
				continue
			}
			v := vi.(map[string]interface{})
			unstructured.RemoveNestedField(v, "hostPath", "type")
			vm[i] = vi
		}
		_ = unstructured.SetNestedField(obj, vm, "volumes")
	}

	val, found, _ = unstructured.NestedInt64(obj, "backoffLimit")
	if found && val == 6 {
		unstructured.RemoveNestedField(obj, "backoffLimit")
	}

	val, found, _ = unstructured.NestedInt64(obj, "parallelism")
	if found && val < 2 {
		unstructured.RemoveNestedField(obj, "parallelism")
	}

	val, found, _ = unstructured.NestedInt64(obj, "completions")
	if found && val < 2 {
		unstructured.RemoveNestedField(obj, "completions")
	}

	val, found, _ = unstructured.NestedInt64(obj, "failedJobsHistoryLimit")
	if found && val < 2 {
		unstructured.RemoveNestedField(obj, "failedJobsHistoryLimit")
	}

	val, found, _ = unstructured.NestedInt64(obj, "successfulJobsHistoryLimit")
	if found && val == 3 {
		unstructured.RemoveNestedField(obj, "successfulJobsHistoryLimit")
	}

	v, found, _ = unstructured.NestedString(obj, "concurrencyPolicy")
	if found && v == "Allow" {
		unstructured.RemoveNestedField(obj, "concurrencyPolicy")
	}

	b, found, _ = unstructured.NestedBool(obj, "suspend")
	if found && !b {
		unstructured.RemoveNestedField(obj, "suspend")
	}

	vts, foundVt, _ := unstructured.NestedSlice(obj, "volumeClaimTemplates")
	if foundVt {
		for i := range vts {
			v := vts[i].(map[string]interface{})
			ds, foundInMap, _ := unstructured.NestedFieldCopy(v, "spec", "dataSource")
			if !foundInMap || ds == nil {
				unstructured.RemoveNestedField(v, "spec", "dataSource")
				vts[i] = v
			}

			vm, foundInMap, _ := unstructured.NestedFieldCopy(v, "spec", "volumeMode")
			if !foundInMap || vm == nil {
				_ = unstructured.SetNestedField(v, "Filesystem", "spec", "volumeMode")
				vts[i] = v
			}
			unstructured.RemoveNestedField(v, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(v, "status")
		}
		_ = unstructured.SetNestedSlice(obj, vts, "volumeClaimTemplates")
	}

	initConts, iFound, _ := unstructured.NestedSlice(obj, "initContainers")
	if iFound && initConts == nil || len(initConts) == 0 {
		unstructured.RemoveNestedField(obj, "initContainers")
	}

	conts, cFound, _ := unstructured.NestedSlice(obj, "containers")
	if cFound && conts == nil || len(conts) == 0 {
		unstructured.RemoveNestedField(obj, "containers")
	}

	if cFound {
		for i := range conts {
			c := conts[i].(map[string]interface{})
			unstructured.RemoveNestedField(c, "resources")
			unstructured.RemoveNestedField(c, "terminationMessagePath")
			unstructured.RemoveNestedField(c, "terminationMessagePolicy")
			unstructured.RemoveNestedField(c, "imagePullPolicy")

			if vm, exist := c["volumeMounts"]; !exist || vm == nil {
				unstructured.RemoveNestedField(c, "volumeMounts")
			}

			ports, found, _ := unstructured.NestedSlice(c, "ports")
			if found {
				for i := range ports {
					p := ports[i]
					f := p.(map[string]interface{})
					unstructured.RemoveNestedField(f, "protocol")
					unstructured.RemoveNestedField(f, "targetPort")
					ports[i] = p
				}
				_ = unstructured.SetNestedField(c, ports, "ports")
			}

			conts[i] = c
		}
		_ = unstructured.SetNestedField(obj, conts, "containers")
	}

	ports, found, _ := unstructured.NestedSlice(obj, "ports")
	if found {
		for i := range ports {
			p := ports[i]
			f := p.(map[string]interface{})
			unstructured.RemoveNestedField(f, "protocol")
			unstructured.RemoveNestedField(f, "targetPort")
			ports[i] = p
		}
		_ = unstructured.SetNestedField(obj, ports, "ports")
	}

	return obj
}

func CheckCustomMetaSnapshot(resourceMap map[string][]string, cSnap *crd.Custom) {
	log.Info("checking custom metadata snapshot with the resourceMap")
	resources := cSnap.Resources
	CheckResources(resourceMap, resources...)
}

func CheckSkippedResources(resourceMap map[string][]string, cStat *crd.ComponentStatus) {
	log.Info("checking the skipped resources")
	if len(resourceMap) == 0 && len(cStat.SkippedResources) == 0 {
		return
	}
	//if len(cStat.SkippedResources) == 0 {
	//	Fail(fmt.Sprintf("nil skiped resources list, but expected: %+v", resourceMap))
	//}
	CheckResources(resourceMap, cStat.SkippedResources...)
}

func CheckFailedResources(resourceMap map[string][]string, cStat *crd.ComponentStatus) {
	log.Info("checking the failed resources")
	resources := cStat.FailedResources
	if len(resources) == 0 && len(resourceMap) == 0 {
		return
	}

	// CheckResources(resourceMap, resources...)
	// since there will be only one failed resources, we need to check if it exists in the resourceMap
	gvkStr := GetGVKString(&resources[0].GroupVersionKind)
	log.Info(gvkStr)
	if rm, exists := resourceMap[gvkStr]; exists {
		resourceObject := resources[0].Objects[0]
		res := sets.NewString(rm...)
		if res.Has(resourceObject) {
			log.Info("Failed resources matched")
			return
		}
	}
	log.Error(fmt.Sprintf("Gvk not found in resourceMap, %s for resources: %+v", gvkStr, resources[0].Objects))
	Fail(fmt.Sprintf("GVK not found in resourceMap. Expected: %+v, Found: %+v", resourceMap, resources))
}

func CheckHelmMetaSnapshot(resourceMap map[string]interface{}, hSnaps []crd.Helm, checkOldRel bool) {
	log.Info("checking helm metadata snapshot with the resourceMap")
	for i := range hSnaps {
		hs := hSnaps[i]
		componentMeta := hs.Resources
		if _, present := resourceMap[hs.Release].(map[string]interface{}); !present {
			Fail(fmt.Sprintf("No resource map found for the release: %s", hs.Release))
		}
		helmExpectedResourceMap := resourceMap[hs.Release].(map[string]interface{})
		CheckResources(helmExpectedResourceMap["helmResources"].(map[string][]string), componentMeta...)
		r := helmExpectedResourceMap["revision"]
		if hs.Revision != r.(int32) {
			Fail("Wrong revision of the helm chart")
		}
		rel := helmExpectedResourceMap["release"]
		if checkOldRel {
			if hs.Release != rel.(string) {
				Fail("Invalid release name")
			}
		} else {
			if hs.NewRelease != rel.(string) {
				Fail("Invalid release name")
			}
		}
		version := helmExpectedResourceMap["version"]
		if string(hs.Version) != version {
			Fail("Invalid helm version")
		}

		sb := helmExpectedResourceMap["storageBackend"]
		if sb != string(hs.StorageBackend) {
			Fail("Invalid helm storage backend")
		}
	}
}

func RestartControlPlane(cl client.Client, ns string) {
	pods := corev1.PodList{}
	controlPlane := appsv1.Deployment{}
	err := cl.Get(testCtx, apiTypes.NamespacedName{Name: controlPlaneName, Namespace: ns}, &controlPlane)
	Expect(err).To(BeNil())

	ls, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      controlPlane.Spec.Selector.MatchLabels,
		MatchExpressions: controlPlane.Spec.Selector.MatchExpressions,
	})
	err = cl.List(testCtx, &pods, client.MatchingLabelsSelector{Selector: ls})
	Expect(err).To(BeNil())
	for i := range pods.Items {
		p := pods.Items[i]
		p.SetNamespace(ns)
		err = cl.Delete(testCtx, &p)
		Expect(err).To(BeNil())
	}

	Eventually(func() bool {
		dep := appsv1.Deployment{}
		err := cl.Get(testCtx, apiTypes.NamespacedName{Name: controlPlaneName, Namespace: ns}, &dep)
		Expect(err).To(BeNil())
		podList := corev1.PodList{}
		selectors, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels:      dep.Spec.Selector.MatchLabels,
			MatchExpressions: dep.Spec.Selector.MatchExpressions,
		})
		err = cl.List(testCtx, &podList, client.MatchingLabelsSelector{Selector: selectors}, client.InNamespace(ns))
		Expect(err).To(BeNil())
		return *dep.Spec.Replicas == int32(len(podList.Items))
	}, timeout, interval).Should(BeTrue())
}

func GetMetadataUploadContainer(namespace, backupName, targetName, applicationNamespace string) *corev1.Container {
	dataAttacherCommmand := GetDataAttacherCommand(namespace, targetName)
	dataMoverCommmand := fmt.Sprintf("/opt/tvk/datamover --action=%s --namespace=%s --backup-name=%s --target-name=%s --application-namespace=%s",
		internal.BackupMetadataAction, namespace, backupName, targetName, applicationNamespace)
	command := fmt.Sprintf("%s && %s", dataAttacherCommmand, dataMoverCommmand)

	return getDataMoverContainer(command, nil)
}

func CheckResourcesExists(acc *kube.Accessor, namespace string, resourceMap map[string][]string) {
	for s, list := range resourceMap {
		gvk := GetGVKFromString(s)
		for _, item := range list {
			log.Infof("Checking for gvk: %s, name: %s", gvk, item)
			obj, err := acc.GetUnstructuredObject(apiTypes.NamespacedName{Namespace: namespace, Name: item}, gvk)
			if err != nil && obj == nil {
				log.Errorf("error while getting the object: %s", err.Error())
				Fail(fmt.Sprintf("Failed to restore, GVK: %s, item: %s, namespace: %s", gvk, item, namespace))
			}
		}
	}
	log.Infof("Successfully restored all the passed resources")
}

func CheckOmitMeta(acc *kube.Accessor, namespace string, resourceMap map[string][]string) {
	log.Info("check omit meta")
	labelResSet := sets.NewString(internal.ReplicationControllerKind, internal.JobKind)
	annotationResSet := sets.NewString(internal.DeploymentKind, internal.DaemonSetKind)
	for s, list := range resourceMap {
		gvk := GetGVKFromString(s)
		if gvk.Kind == internal.CRDKind || gvk.Kind == internal.StorageClassKind {
			continue
		}

		for _, item := range list {
			log.Infof("Checking for gvk: %s, name: %s", gvk, item)
			obj, err := acc.GetUnstructuredObject(apiTypes.NamespacedName{Namespace: namespace, Name: item}, gvk)
			if err != nil && obj == nil {
				log.Errorf("error while getting the object: %s", err.Error())
				Fail(fmt.Sprintf("Failed to restore, GVK: %s, item: %s, namespace: %s", gvk, item, namespace))
			}

			Expect(obj.GetOwnerReferences()).To(BeNil())
			log.Infof("annotation for gvk: %s, name: %s is %+v", gvk, item, obj.GetAnnotations())

			// For annotationResSet resources, annotations is automatically set equal to selector labels by controllers
			if annotationResSet.Has(gvk.Kind) {
				Expect(len(obj.GetAnnotations())).To(Equal(1))
			} else {
				Expect(obj.GetAnnotations()).To(BeNil())
			}

			// For labelResSet resources, label is automatically set equal to selector labels by controllers
			if labelResSet.Has(gvk.Kind) {
				Expect(len(obj.GetLabels())).To(Equal(1))
			} else {
				Expect(obj.GetLabels()).To(BeNil())
			}

		}
	}
	log.Infof("Successfully verified omitmeta")
}

func DeleteResources(acc *kube.Accessor, ns, testSc string, resourceMap map[string][]string) {
	for s, list := range resourceMap {
		gvk := GetGVKFromString(s)
		if gvk.Kind == internal.CRDKind {
			continue
		}
		for _, item := range list {
			// do not delete testSc
			if gvk.Kind == internal.StorageClassKind && item == testSc {
				continue
			}

			log.Infof("deleting gvk: %s, name: %s", gvk, item)
			err := acc.DeleteUnstructuredObject(apiTypes.NamespacedName{Namespace: ns, Name: item}, gvk, client.GracePeriodSeconds(0))
			if err != nil && !(apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind")) {
				log.Errorf("error while deleting the object: %s", err.Error())
				Fail(fmt.Sprintf("Failed to delete, GVK: %s, item: %s, namespace: %s", gvk, item, ns))
			}
		}
	}
}

func GetGVKFromString(gvk string) schema.GroupVersionKind {
	strs := strings.Split(gvk, internal.Comma)
	apiVersion := strings.TrimSpace(strs[0])
	tmp := strings.Split(strs[1], internal.Equals)
	kind := strings.TrimSpace(tmp[1])
	return schema.FromAPIVersionAndKind(apiVersion, kind)
}

func CheckHelmReleaseExists(acc *kube.Accessor, releaseName, namespace string, revision int) {
	hlmMgr, err := helmutils.NewHelmManager(acc.GetRestConfig(), namespace)
	Expect(err).NotTo(HaveOccurred())
	log.Infof("checking release: %s, in namespace: %s", releaseName, namespace)
	defaultSb, _ := hlmMgr.GetDefaultStorageBackend()
	log.Infof("checking in default storage backend: %s", defaultSb.GetKind())
	_, err = defaultSb.GetRelease(releaseName, int32(revision))
	if err == nil {
		log.Info("release found in default storage backend")
		return
	}

	customSb, _ := hlmMgr.GetCustomStorageBackend()
	_, err = customSb.GetRelease(releaseName, int32(revision))
	if err == nil {
		log.Info("release found in custom storage backend")
		return
	}
	Fail(fmt.Sprintf("release: %s, not found in namespace: %s", releaseName, namespace))
}

func DeleteHelmRelease(acc *kube.Accessor, releaseName, namespace string, revision int) {
	hlmMgr, err := helmutils.NewHelmManager(acc.GetRestConfig(), namespace)
	Expect(err).NotTo(HaveOccurred())
	log.Infof("checking release: %s, in namespace: %s", releaseName, namespace)
	defaultSb, _ := hlmMgr.GetDefaultStorageBackend()
	log.Infof("checking in default storage backend: %s", defaultSb.GetKind())
	err = defaultSb.DeleteRelease(releaseName, revision)
	if err == nil {
		log.Info("release deleted in default storage backend")
		return
	}

	customSb, _ := hlmMgr.GetCustomStorageBackend()
	err = customSb.DeleteRelease(releaseName, revision)
	if err == nil {
		log.Info("release deleted in custom storage backend")
		return
	}

	//if !strings.Contains(err.Error(), "not found") {
	//	Fail(fmt.Sprintf("not able to delete release: %s in namespace; %s, with error: %s", releaseName, namespace, err.Error()))
	//}
}

func CleanupPvc(pvcKey types.NamespacedName) {
	var pvc *corev1.PersistentVolumeClaim
	k8sClient := kubeAccessor.GetKubeClient()

	log.Infof("cleaing up %+v pvc", pvcKey)
	err = utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
		log.Infof("removing finalizer on %+v pvc", pvcKey)
		pvc, err = kubeAccessor.GetPersistentVolumeClaim(pvcKey.Namespace, pvcKey.Name)
		Expect(err).ShouldNot(HaveOccurred())
		pvc.SetFinalizers([]string{})
		return k8sClient.Update(context.Background(), pvc)
	})
	Expect(err).ShouldNot(HaveOccurred())
	log.Infof("finalizers removed from %+v", pvcKey)

	Eventually(func() error {
		log.Infof("deleting %+v pvc", pvcKey)
		return kubeAccessor.DeletePersistentVolumeClaim(pvcKey.Namespace,
			pvcKey.Name)
	}, Timeout, interval).ShouldNot(HaveOccurred())

	Eventually(func() error {
		log.Infof("verifying is %+v pvc is deleted", pvcKey)
		_, err = kubeAccessor.GetPersistentVolumeClaim(pvcKey.Namespace, pvc.Name)
		return err
	}, Timeout, Interval).ShouldNot(Succeed())
	log.Infof("%+v pvc is deleted", pvcKey)
}

func cleanupPendingPod(dataMoverPodKey types.NamespacedName) {
	log.Infof("cleaning pod in pending state %+v", dataMoverPodKey)
	_, err = kubeAccessor.GetPod(dataMoverPodKey.Namespace, dataMoverPodKey.Name)
	if err == nil {
		Eventually(func() error {
			log.Infof("waiting for %+v pod deletion", dataMoverPodKey)
			return kubeAccessor.DeletePod(dataMoverPodKey.Namespace, dataMoverPodKey.Name)
		}, Timeout, Interval).ShouldNot(HaveOccurred())

		Eventually(func() error {
			log.Infof("verifying %+v pod is deleted", dataMoverPodKey)
			_, err = kubeAccessor.GetPod(dataMoverPodKey.Namespace, dataMoverPodKey.Name)
			return err
		}, Timeout, interval).ShouldNot(Succeed())
		log.Infof("%s pod is deleted", dataMoverPodKey)
	}

}

//nolint:unparam
func RunDataUploadPod(dataUploadInfo map[string]interface{}, dataSnapshot *helpers.ApplicationDataSnapshot, waitgroup *sync.WaitGroup) {
	pvc := dataUploadInfo["pvc"].(*corev1.PersistentVolumeClaim)
	pvcKey := types.NamespacedName{
		Name:      pvc.GetName(),
		Namespace: dataUploadInfo["namespace"].(string),
	}
	backup := dataUploadInfo["backup"].(*crd.Backup)
	preBackupName := dataUploadInfo["preBackupName"].(string)
	dataMoverPodKey := types.NamespacedName{
		Name:      dataUploadInfo["podName"].(string),
		Namespace: dataUploadInfo["namespace"].(string),
	}
	if waitgroup != nil {
		defer GinkgoRecover()
		defer waitgroup.Done()
	}
	var retryCnt int

	for {
		retryCnt++
		log.Infof("deleting %+v pod if already exists", dataMoverPodKey)
		cleanupPendingPod(dataMoverPodKey)
		log.Infof("Creating data mover pod, RetryCount:%d", retryCnt)
		pvc, err = kubeAccessor.GetPersistentVolumeClaim(pvcKey.Namespace, pvcKey.Name)
		Expect(err).ShouldNot(HaveOccurred())

		dataMoverContainer := GetDataUploadContainer(map[string]string{
			"namespace":     backup.GetNamespace(),
			"backupName":    backup.Name,
			"preBackupName": preBackupName,
			"targetName":    dataUploadInfo["targetName"].(string),
		}, pvc, dataSnapshot)

		dataMoverPod := CreatePod(dataMoverPodKey.Namespace, internal.VolumeDeviceName, dataMoverContainer,
			corev1.RestartPolicyOnFailure, pvc)
		dataMoverPod.SetName(dataMoverPodKey.Name)
		dataMoverPod.Spec.ServiceAccountName = controllerHelpers.GetAuthResourceName(backup.UID, internal.BackupKind)

		if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
			log.Infof("[%s] Pvc is block mode adding initContainer.", pvc.Name)
			dataMoverPod.Spec.InitContainers = []corev1.Container{*controllerHelpers.GetBlockDeviceInitContainer()}
		}

		Expect(kubeAccessor.CreatePod(dataMoverPodKey.Namespace,
			dataMoverPod)).ShouldNot(HaveOccurred())

		phase := WaitUntilPodRunning(dataMoverPodKey)

		if phase == corev1.PodPending || phase == corev1.PodUnknown {
			log.Info("creating pvc setup as pod does not able to find pvc")
			var backupDataSize uint16 = 20

			pvcInfo := map[string]interface{}{
				"isBlock":       *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock,
				"isIncremental": preBackupName != "",
			}

			cleanupPendingPod(dataMoverPodKey)
			CleanupPvc(pvcKey)
			log.Infof("cleaup for %+v pending pod completed", dataMoverPodKey)

			pvc = CreatePVC(pvcKey.Namespace, pvcInfo["isBlock"].(bool),
				backupDataSize, nil, true)
			pvc.SetName(pvcKey.Name)

			Eventually(func() error {
				return kubeAccessor.CreatePersistentVolumeClaim(pvcKey.Namespace, pvc)
			}, timeout, interval).ShouldNot(HaveOccurred())

			pvc = RunDataInjectionPod(pvcKey.Namespace, pvcKey.Name, pvcInfo["isIncremental"].(bool))
		} else {
			break
		}

		if retryCnt == 3 {
			break
		}
	}

	dataMoverPod, err := kubeAccessor.GetPod(dataMoverPodKey.Namespace, dataMoverPodKey.Name)
	Expect(err).ShouldNot(HaveOccurred())

	if dataMoverPod.Status.Phase == corev1.PodPending || dataMoverPod.Status.Phase == corev1.PodUnknown {
		Fail(fmt.Sprintf("pod %+v is in %v state", dataMoverPodKey, dataMoverPod.Status.Phase))
	}

	log.Infof("Waiting for data mover to complete: %s", dataMoverPodKey.Name)
	log.Infof("start time :%+v", time.Now())
	podKey := apiTypes.NamespacedName{Name: dataMoverPodKey.Name, Namespace: dataMoverPodKey.Namespace}
	WaitUntilPodSucceeded(podKey, corev1.PodSucceeded)
	log.Infof("End time :%+v", time.Now())
}

//nolint:unparam
func PvcSetup(pvcName, namespace string, pvcInfo map[string]interface{}) *corev1.PersistentVolumeClaim {
	log.Infof("Creating  PVC %s", pvcName)
	pvc := CreatePVC(namespace, pvcInfo["isBlock"].(bool),
		pvcInfo["backupDataSize"].(uint16), nil, true)
	pvc.SetName(pvcName)
	Expect(kubeAccessor.CreatePersistentVolumeClaim(namespace, pvc)).ShouldNot(HaveOccurred())

	return RunDataInjectionPod(namespace, pvcName, pvcInfo["isIncremental"].(bool))
}

func DeletePvc(namespace string, pvcNames ...string) {
	for i := range pvcNames {
		pvcName := pvcNames[i]
		Eventually(func() error {
			if _, err = kubeAccessor.GetPersistentVolumeClaim(namespace, pvcName); err != nil {
				log.Infof("pvc not exists %s/%s", pvcName, namespace)
				return nil
			}
			log.Infof("Deleting the pvc %s/%s", pvcName, namespace)
			return kubeAccessor.DeletePersistentVolumeClaim(namespace, pvcName)
		}, timeout, interval).Should(Succeed())

	}
}

func RunDataInjectionPod(namespace, pvcName string, isIncremental bool) (pvc *corev1.PersistentVolumeClaim) {
	k8sClient := kubeAccessor.GetKubeClient()

	pvc, err = kubeAccessor.GetPersistentVolumeClaim(namespace, pvcName)
	Expect(err).ShouldNot(HaveOccurred())

	By("Injecting data in PV bound to PVC")
	container := CreateDataInjectionContainer(pvc, isIncremental)
	injectorPod := CreatePod(namespace, internal.VolumeDeviceName,
		container, corev1.RestartPolicyOnFailure, pvc)

	injectorPod.Spec.ServiceAccountName = ""

	Eventually(func() error {
		return k8sClient.Create(context.Background(), injectorPod)
	}, Timeout, Interval).Should(Succeed())

	By("Waiting for data injector to complete")
	podKey := apiTypes.NamespacedName{Name: injectorPod.Name, Namespace: injectorPod.Namespace}
	WaitUntilPodSucceeded(podKey, corev1.PodSucceeded)
	return pvc
}

func GetPod(namespace string, container *corev1.Container) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-" + guid.New().String(),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: internal.ServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers:         []corev1.Container{*container},
			HostPID:            false,
			HostIPC:            false,
			HostNetwork:        false,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    &internal.RunAsRootUserID,
				RunAsNonRoot: &internal.RunAsNonRoot,
			},
		},
	}
	return pod
}

func runMountCmd(wg *sync.WaitGroup, targetName, mountpoint string) {
	defer GinkgoRecover()
	defer wg.Done()
	var (
		cmdErr error
		cmdOut *shell.CmdOut
	)

	dataAttacherPath := filepath.Join(projectRoot, "datastore-attacher/mount_utility/mount_by_secret/mount_datastores.py")
	dataAttacherCommand := fmt.Sprintf(dataAttacherPath+" --target-name=%s", targetName)

	if mountpoint != "" {
		dataAttacherCommand = fmt.Sprintf("%s --mountpoint=%s", dataAttacherCommand, mountpoint)
	}

	command := fmt.Sprintf("python %s", dataAttacherCommand)
	log.Info(fmt.Sprintf("Running mount command: %s", command))

	cmdOut, cmdErr = shell.RunCmd(command)
	log.Info(fmt.Sprintf("command ouput: %+v, err: %+v", cmdOut, cmdErr))
}

func removeSecretFile(secretFilePath string) {
	var cmdOut *shell.CmdOut
	cmd := fmt.Sprintf("rm -rf %s", secretFilePath)
	cmdOut, err = shell.RunCmd(cmd)
	Expect(err).ShouldNot(HaveOccurred())
	log.Info(fmt.Sprintf("cmd: %s, output: %v, err: %v", cmd, cmdOut, err))
}

func MountNfsDataAttacher(targetName, mountpoint string) {
	var (
		wg       sync.WaitGroup
		isExists bool
		out      string
	)
	log.Info("Creating Secret for data attacher")
	_, err = shell.Mkdir(DataStoreAttacherPath)
	Expect(err).Should(BeNil())

	secret := CreateTargetSecret(targetName)
	_, err = shell.Mkdir(DataStoreAttacherSecretPath)
	Expect(err).Should(BeNil())
	secretFilePath := path.Join(DataStoreAttacherSecretPath, utils.TrilioSecName)

	isExists, out, err = shell.FileExistsInDir(secretFilePath, "")
	log.Infof("%s exists: %v, out:%s, err:%+v", secretFilePath, isExists, out, err)
	if isExists {
		removeSecretFile(secretFilePath)
	}

	err = shell.WriteToFile(secretFilePath, secret)
	Expect(err).Should(BeNil())

	log.Infof("Mounting target %s", targetName)
	wg.Add(1)
	go runMountCmd(&wg, targetName, mountpoint)
	log.Infof("waiting for %s mount", targetName)
	time.Sleep(time.Second * 20)
	log.Info(fmt.Sprintf("Completed %s Go-routine", targetName))
}

func MountS3DataAttacher(targetName, mountpoint string) {
	var wg sync.WaitGroup
	By("Creating Secret for data attacher")
	_, err := shell.Mkdir(DataStoreAttacherPath)
	Expect(err).Should(BeNil())
	_, err = shell.Mkdir(TempDataStoreBasePath)
	Expect(err).Should(BeNil())

	secret := CreateS3TargetSecret(targetName)

	_, err = shell.Mkdir(DataStoreAttacherSecretPath)
	Expect(err).Should(BeNil())
	secretFilePath := path.Join(DataStoreAttacherSecretPath, utils.TrilioSecName)
	isExists, out, err := shell.FileExistsInDir(secretFilePath, "")
	By(fmt.Sprintf("%s exists: %v, out:%s, err:%+v", secretFilePath, isExists, out, err))

	if isExists {
		removeSecretFile(secretFilePath)
	}

	err = shell.WriteToFile(secretFilePath, secret)
	Expect(err).Should(BeNil())

	log.Infof("Mounting target %s", targetName)
	wg.Add(1)
	go runMountCmd(&wg, targetName, mountpoint)
	log.Infof("waiting for %s mount", targetName)
	time.Sleep(time.Second * 20)
	By(fmt.Sprintf("Completed %s Go-routine", targetName))
}

func Unmount(dirPath string) {
	log.Infof("unmounting %s", dirPath)
	err = syscall.Unmount(dirPath, 0)
	if err != nil {
		log.Error(err)
	}
	log.Infof("%s unmounted successfully.", dirPath)
}

func GetAccessor() *kube.Accessor {
	scheme := runtime.NewScheme()
	_ = crd.AddToScheme(scheme)
	_ = clientGoScheme.AddToScheme(scheme)

	kubeAccessor, err := kube.NewEnv(scheme)
	if err != nil {
		log.Error(err)
	}

	return kubeAccessor
}

func UploadData(pvcName, targetName string, dataMoverPodKey, backupKey apiTypes.NamespacedName,
	backupDataSize uint16) {
	var pvc *corev1.PersistentVolumeClaim
	var backup *crd.Backup

	backup, err = kubeAccessor.GetBackup(backupKey.Name, backupKey.Namespace)
	Expect(err).ShouldNot(HaveOccurred())
	k8sClient := kubeAccessor.GetKubeClient()

	pvc = PvcSetup(pvcName, dataMoverPodKey.Namespace, map[string]interface{}{
		"isBlock":        true,
		"backupDataSize": backupDataSize,
		"isIncremental":  false,
	})

	By("Getting backup for update")

	backup, err = kubeAccessor.GetBackup(backupKey.Name, backupKey.Namespace)
	if err != nil {
		panic(fmt.Sprintf("Failed to get Backup(%s)-> %+v", backupKey.Name, err))
	}

	By(fmt.Sprintf("ACCID:%v, backupID:%v", backup.Spec.BackupPlan.UID, backup.UID))

	By("Updating backup a data component in custom metadata SnapshotContent only")
	dataComponent := GenerateDataSnapshotContent(nil, pvc, true)
	snapshotContent := &crd.Snapshot{
		Custom: &crd.Custom{
			Resources:     GenerateComponentMetadata(dataMoverPodKey.Namespace),
			DataSnapshots: []crd.DataSnapshot{dataComponent},
		},
		Operators:  []crd.Operator{},
		HelmCharts: []crd.Helm{},
	}

	backup, err = kubeAccessor.GetBackup(backupKey.Name, backupKey.Namespace)
	if err != nil {
		panic(fmt.Sprintf("Failed to get Backup(%s)-> %+v", backupKey.Name, err))
	}

	Eventually(func() error {
		backup.Status.Snapshot = snapshotContent
		return k8sClient.Status().Update(context.Background(), backup)
	}, Timeout, Interval).Should(Succeed())

	appDataSnapshot := &helpers.ApplicationDataSnapshot{
		AppComponent:        internal.Custom,
		ComponentIdentifier: "",
		DataComponent:       backup.Status.Snapshot.Custom.DataSnapshots[0],
	}

	RunDataUploadPod(map[string]interface{}{
		"pvc":           pvc,
		"backup":        backup,
		"podName":       dataMoverPodKey.Name,
		"namespace":     dataMoverPodKey.Namespace,
		"preBackupName": "",
		"targetName":    targetName,
	}, appDataSnapshot, nil)
}

func CleanupResourcesInNamespace(ns, cleanupPath, uniqId, release string) {
	log.Infof("Cleaning up everything before tearing down suite from %s testNs with script %s", ns, cleanupPath)
	cmdOut, err := shell.RunCmd(strings.Join([]string{cleanupPath, ns, uniqId, release}, internal.Space))
	if err != nil {
		log.Errorf("Error while cleaning up resource in namespace %s, err:%+v, output:%s, exitcode:%d", ns,
			err, cmdOut.Out, cmdOut.ExitCode)
	}
	Expect(err).To(BeNil())
	log.Infof("Cleaned up everything %s testNs: %s", ns, cmdOut.Out)
}

func UpdatePlaceholdersInFiles(kv map[string]string, filepath ...string) error {
	var (
		file    []byte
		readErr error
	)
	for _, f := range filepath {
		for placeholder, value := range kv {
			if file, readErr = ioutil.ReadFile(f); readErr != nil {
				return readErr
			}
			if strings.Contains(string(file), placeholder) {
				updatedCustomApp := strings.Replace(string(file), placeholder, value, -1)
				log.Infof("Updated the old value: [%s] with new value: [%s] in file [%s]", placeholder, value, f)

				if writeErr := ioutil.WriteFile(f, []byte(updatedCustomApp), 0); writeErr != nil {
					return writeErr
				}
			}
		}
	}
	return nil
}

func GetChildPodFromJob(acc *kube.Accessor, ns string, job *batchv1.Job) *corev1.Pod {
	pods, _ := acc.GetPods(ns)
	for i := range pods {
		p := &pods[i]
		ls := p.Labels
		if cuid, present := ls["controller-uid"]; present {
			if jn, exists := ls["job-name"]; exists {
				if cuid == string(job.UID) && jn == job.Name {
					return p
				}
			}
		}
	}
	return nil
}

func GetPodName(podList *corev1.PodList) []string {
	var podName []string
	for podIndex := range podList.Items {
		podName = append(podName, podList.Items[podIndex].Name)
	}

	return podName
}

func AddSecretToHelmChart(chartPath, uid string) {

	fp := filepath.Join(chartPath, "templates", "secret.yaml")

	f, err := os.Create(fp)
	Expect(err).To(BeNil())

	s := `
apiVersion: v1
kind: Secret
metadata:
  name: ` + uid + "-secret\n" +
		`type: Opaque
data:
  username: YWRtaW4=
  password: MWYyZDFlMmU2N2Rm
`

	l, err := f.WriteString(s)
	if err != nil {
		log.Error(err)
		f.Close()
	}
	Expect(err).To(BeNil())

	log.Info(l, "bytes written successfully to file", fp)
	err = f.Close()
	Expect(err).To(BeNil())
}

func AddReqFileInHelmChart(chartPath, fileData string) {

	fp := filepath.Join(chartPath, "requirements.yaml")

	f, err := os.Create(fp)
	Expect(err).To(BeNil())

	l, err := f.WriteString(fileData)
	if err != nil {
		log.Error(err)
		f.Close()
	}
	Expect(err).To(BeNil())

	log.Info(l, "bytes written successfully to file", fp)
	err = f.Close()
	Expect(err).To(BeNil())
}

func UpgradeOrRollbackMysqlOperator(a *kube.Accessor, ns, releaseName, op, chartDir string) {
	a.DeleteUnstructuredObject(types.NamespacedName{Name: releaseName + "-mysql-operator", Namespace: ns},
		schema.GroupVersionKind{Group: "policy", Version: "v1beta1", Kind: "PodDisruptionBudget"})

	a.DeleteUnstructuredObject(types.NamespacedName{Name: releaseName + "1-mysql-operator", Namespace: ns},
		schema.GroupVersionKind{Group: "policy", Version: "v1beta1", Kind: "PodDisruptionBudget"})

	a.DeleteUnstructuredObject(types.NamespacedName{Name: releaseName + "-mysql-operator-1-svc", Namespace: ns},
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"})

	a.DeleteUnstructuredObject(types.NamespacedName{Name: releaseName + "1-mysql-operator-1-svc", Namespace: ns},
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"})

	out, err := shell.RunCmd(fmt.Sprintf("%s %s %s %s", filepath.Join(projectRoot, testDataDir, chartDir,
		mysqlOperatorScript), op, releaseName, ns))
	log.Infof(out.Out)
	Expect(err).To(BeNil())

	time.Sleep(20 * time.Second) // -> giving it 20 sec to start all pods

	Eventually(func() error {
		_, err = a.WaitUntilPodsAreReady(func() (pods []corev1.Pod, lErr error) {
			pods, lErr = a.GetPods(ns, "app.kubernetes.io/managed-by=mysql.presslabs.org", "app.kubernetes.io/name=mysql")
			return pods, lErr
		})
		return err
	}, ResourceDeploymentTimeout, ResourceDeploymentInterval).Should(BeNil())

	Eventually(func() bool {
		pods, _ := a.GetPods(ns, "app=mysql-operator", fmt.Sprintf("release=%s", releaseName))
		for i := range pods {
			p := &pods[i]
			if p.Status.Phase != corev1.PodRunning {
				return false
			}
		}
		return true
	})
}

func GetAppDataSnapshotContentMap(dataSnapshotContentList []helpers.ApplicationDataSnapshot) map[string]helpers.ApplicationDataSnapshot {

	dataSnapshotContentMap := make(map[string]helpers.ApplicationDataSnapshot)

	for dataSnapshotContentIndex := range dataSnapshotContentList {
		dataSnapshotContent := dataSnapshotContentList[dataSnapshotContentIndex]
		dataSnapshotContentMap[dataSnapshotContent.GetHash()] = dataSnapshotContent
	}

	return dataSnapshotContentMap
}

func GetDataSnapshotContentMap(dataSnapshotContentList []crd.DataSnapshot) map[string]crd.DataSnapshot {

	dataSnapshotContentMap := make(map[string]crd.DataSnapshot)

	for dataSnapshotContentIndex := range dataSnapshotContentList {
		dataSnapshotContent := dataSnapshotContentList[dataSnapshotContentIndex]
		dataSnapshotContentMap[dataSnapshotContent.PersistentVolumeClaimName] = dataSnapshotContent
	}

	return dataSnapshotContentMap
}

func CheckDataContent(reqPvcList []string, dataSnapshots []crd.DataSnapshot) bool {
	var pvcFound bool
	log.Info("Validating the PVCs populated in the restore CR")
	for i := range reqPvcList {
		reqPvc := reqPvcList[i]
		pvcFound = false
		for i := range dataSnapshots {
			ds := dataSnapshots[i]
			if reqPvc == ds.PersistentVolumeClaimName {
				pvcFound = true
				break
			}
		}
		if !pvcFound {
			log.Infof("pvc %s not found", reqPvc)
			return false
		}
	}

	return true
}

func VerifyExistingResources(resourceMap map[string][]string, exResList []crd.Resource) {
	CheckResources(resourceMap, exResList...)
}

func GetContainerExitCode(podName, containerName, namespace string, accessor *kube.Accessor) (int32, error) {
	// Check the container exit status
	pod, podErr := accessor.GetPod(namespace, podName)
	if podErr != nil {
		return 0, podErr
	}
	for i := range pod.Status.ContainerStatuses {
		c := pod.Status.ContainerStatuses[i]
		if c.Name == containerName {
			return c.State.Terminated.ExitCode, nil
		}
	}

	return 0, fmt.Errorf("couldn't get the exit status for "+
		"container %s of pod %s", containerName, podName)
}

func WaitUntilPodRunning(podKey apiTypes.NamespacedName) corev1.PodPhase {
	log.Infof("start time :%+v", time.Now())

	log.Infof("Waiting for pod %+v running.", podKey)
	// checking if pod consistently is in pending state till 5min and failing immediately if pod
	// does not comes to running state.
	var pod corev1.Pod

	_, err = retry.Do(func() (result interface{}, completed bool, err error) {
		pod, err = kubeAccessor.GetPod(podKey.Namespace, podKey.Name)

		if err != nil {
			log.Errorf("%s/%s pod not found -> %+v", podKey.Name, podKey.Namespace, err)
			return nil, true, fmt.Errorf("%s/%s pod not found -> %+v", podKey.Name, podKey.Namespace, err)
		}

		log.Infof("%s/%s POD PHASE: %v", pod.Name, pod.Namespace,
			pod.Status.Phase)

		if pod.Status.Phase != corev1.PodPending && pod.Status.Phase != corev1.PodUnknown {
			log.Infof("%s/%s POD PHASE: %v ", pod.Name, pod.Namespace,
				pod.Status.Phase)
			return pod.Status.Phase, true, nil
		}
		return nil, false, fmt.Errorf("pod %+v status ->"+
			" %v not expected", podKey, pod.Status.Phase)

	}, retry.Timeout(time.Minute*7), retry.Delay(time.Second*5), retry.Count(84))

	pod, err = kubeAccessor.GetPod(podKey.Namespace, podKey.Name)
	Expect(err).ShouldNot(HaveOccurred())
	return pod.Status.Phase

}

func WaitUntilPodSucceeded(podKey apiTypes.NamespacedName, phase corev1.PodPhase) {

	By(fmt.Sprintf("start time :%+v", time.Now()))

	defer func() {
		Eventually(func() error {
			log.Infof("waiting for %+v pod deletion", podKey)
			err = kubeAccessor.DeletePod(podKey.Namespace, podKey.Name)
			if err != nil {
				return err
			}
			_, err = kubeAccessor.GetPod(podKey.Namespace, podKey.Name)
			Expect(err).Should(HaveOccurred())
			return nil
		}, timeout, interval).ShouldNot(HaveOccurred())
		log.Infof("pod %+v deleted successfully", podKey)
	}()

	_ = WaitUntilPodRunning(podKey)
	var podRunningRetryCheck uint8
	log.Info("Waiting for pod success.")
	var pod corev1.Pod

	for {

		_, err = retry.Do(func() (result interface{}, completed bool, err error) {
			pod, err = kubeAccessor.GetPod(podKey.Namespace, podKey.Name)
			if err != nil {
				log.Errorf("%s/%s pod failed -> %+v", podKey.Name, podKey.Namespace, err)
				return nil, true, err
			}

			log.Infof("%s/%s pod expected phase -> %v, actual phase -> %v", pod.Name, pod.Namespace,
				phase, pod.Status.Phase)

			if phase == corev1.PodSucceeded && pod.Status.Phase == corev1.PodFailed {
				log.Infof("%+v pod expected phase -> %v, actual phase -> %v", podKey, phase, pod.Status.Phase)
				return pod.Status.Phase, true, fmt.Errorf("%v pod status is not expected -> %v",
					podKey, pod.Status.Phase)
			}
			log.Println(pod.Status.Phase == phase)
			if pod.Status.Phase == phase {
				log.Infof("%+v pod is in expected phase -> %v, actual phase -> %v", podKey, phase, pod.Status.Phase)
				return nil, true, nil
			}
			return nil, false, fmt.Errorf("%+v pod status phase -> %v, not expected -> %v",
				podKey, pod.Status.Phase, phase)
		}, retry.Timeout(PodTimeout), retry.Delay(Podinterval), retry.Count(84))

		if err != nil {
			log.Infof("checking if %+v pod is still running", podKey)
			pod, err = kubeAccessor.GetPod(podKey.Namespace, podKey.Name)
			Expect(err).To(BeNil())
			podRunningRetryCheck++
			if pod.Status.Phase != corev1.PodRunning || podRunningRetryCheck == uint8(3) {
				break
			}

		} else {
			break
		}
	}

	pod, err = kubeAccessor.GetPod(podKey.Namespace, podKey.Name)
	Expect(err).To(BeNil())
	Expect(pod.Status.Phase).To(Equal(phase))

	log.Infof(fmt.Sprintf("End time :%+v", time.Now()))
}

// Update old YAML values with new values
// fileorDirPath can be a single file path or a directory
// kv is map of old value to new value
func UpdateYAMLs(kv map[string]string, fileOrDirPath string) error {
	var files []string
	info, err := os.Stat(fileOrDirPath)

	if os.IsNotExist(err) {
		return err
	}

	var walkFn = func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		files = append(files, path)

		return nil
	}

	if info.IsDir() {
		if err := filepath.Walk(fileOrDirPath, walkFn); err != nil {
			return err
		}
	} else {
		files = append(files, fileOrDirPath)
	}

	if len(files) > 1 {
		files = files[1:]
	}

	for _, yamlPath := range files {

		// this does not change elements in test-data/CustomResource/storageClass.yaml file
		// because we dont have any placeholders
		if strings.Contains(yamlPath, "storageClass.yaml") {
			continue
		}
		read, readErr := ioutil.ReadFile(yamlPath)
		if readErr != nil {
			return readErr
		}

		updatedFile := string(read)

		for placeholder, value := range kv {
			if strings.Contains(updatedFile, placeholder) {
				updatedFile = strings.ReplaceAll(updatedFile, placeholder, value)
				log.Infof("Updated the old value: [%s] with new value: [%s] in file [%s]",
					placeholder, value, yamlPath)
			}
		}

		if writeErr := ioutil.WriteFile(yamlPath, []byte(updatedFile), 0); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func GetBackupNamespace() string {
	namespace, present := os.LookupEnv(BackupNamespace)
	if !present {
		panic("Backup Namespace not found in environment")
	}
	return namespace
}

func GetRestoreNamespace() string {
	namespace, present := os.LookupEnv(RestoreNamespace)
	if !present {
		panic("Restore Namespace not found in environment")
	}
	return namespace
}

func GetInstallNamespace() string {
	namespace, present := os.LookupEnv(InstallNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
}

func GetUniqueID(suiteName string) string {
	return suiteName + "-" + internal.GenerateRandomString(4, true)
}

func CopyLicenseKeyFile(projectRoot string) error {
	sourceDir := filepath.Join(projectRoot, "docker-images/control-plane/license_keys")
	for _, key := range licenseKeys {
		sourceFile := filepath.Join(sourceDir, key)
		destinationFile := filepath.Join(projectRoot, "controllers", "license", key)
		err := copyFile(sourceFile, destinationFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(sourceFile, destinationFile string) error {
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = ioutil.WriteFile(destinationFile, input, 0644)
	if err != nil {
		fmt.Println("Error creating", destinationFile)
		fmt.Println(err)
		return err
	}
	return nil
}

func CleanupLicenseKey(projectRoot string) {
	sourceDir := filepath.Join(projectRoot, "controllers")
	for _, key := range licenseKeys {
		sourceFile := filepath.Join(sourceDir, key)
		_ = os.Remove(sourceFile)
		cwd, _ := os.Getwd()
		sourceFile = filepath.Join(cwd, key)
		_ = os.Remove(sourceFile)
	}
}

func SetupLicense(ctx context.Context, cli client.Client, namespace string, projectRoot string) (*crd.License, error) {
	var (
		key             string
		isEnvKeyPresent bool
		err             error
	)

	key, isEnvKeyPresent = os.LookupEnv(LicenseKey)
	if !isEnvKeyPresent {
		log.Infof("License Key not found in env, creating new one")
		ns := &corev1.Namespace{}
		if err = cli.Get(ctx, apiTypes.NamespacedName{Name: internal.KubeSystemNamespace}, ns); err != nil {
			return nil, err
		}
		args := KeyGenArgs{LicenseEdition: string(crd.FreeEdition), KubeUID: string(ns.GetUID()), LicensedFor: strconv.Itoa(20)}
		key, err = CreateLicenseKey(projectRoot, args)
		if err != nil {
			log.Errorf("Error while creating license key: %s", err.Error())
			return nil, err
		}
	}

	licenses := &crd.LicenseList{}
	if err = cli.List(ctx, licenses, client.InNamespace(namespace)); err != nil {
		log.Errorf("Error while listing license: %s", err.Error())
		return nil, err
	}
	if len(licenses.Items) != 0 {
		license := licenses.Items[0]
		log.Infof("Found existing license: %s", license.Name)
		if license.Status.Status != crd.LicenseActive {
			license.Spec.Key = key
			if err = cli.Update(ctx, &license); err != nil {
				log.Errorf("Error while updating key in existing license: %s, %s", license.Name, err.Error())
				return nil, err
			}
		}
		return &license, nil
	}
	license := &crd.License{ObjectMeta: metav1.ObjectMeta{Name: LicenseName, Namespace: namespace}, Spec: crd.LicenseSpec{Key: key}}
	err = cli.Create(ctx, license)
	if err != nil {
		log.Errorf("Error while creating new license: %s", err.Error())
		return nil, err
	}
	return license, nil
}

func TearDownLicense(ctx context.Context, cli client.Client, namespace string) error {
	return cli.DeleteAllOf(ctx, &crd.License{}, client.InNamespace(namespace))
}

func WaitForRestoreToComplete(acc *kube.Accessor, restoreName string, ns string) {
	Eventually(func() (crd.Status, error) {
		return acc.GetRestoreStatus(restoreName, ns)
	}, "120s", "2s").Should(Equal(crd.Completed))
}

func WaitForRestoreToDelete(acc *kube.Accessor, restoreName, ns string) {
	Eventually(func() bool {
		_, err := acc.GetRestore(restoreName, ns)
		if err != nil && apierrors.IsNotFound(err) {
			return true
		}
		if err == nil {
			acc.DeleteRestore(types.NamespacedName{Name: restoreName, Namespace: ns})
		}
		return false
	}, "120s", "2s").Should(BeTrue())
}

func WaitForLicenseToState(ctx context.Context, cli client.Client, licenseKey apiTypes.NamespacedName, status crd.LicenseState) {
	Eventually(func() crd.LicenseState {
		license := &crd.License{}
		_ = cli.Get(ctx, licenseKey, license)
		return license.Status.Status
	}, Timeout, Interval).Should(Equal(status))
}

func SetLicenseStatus(ctx context.Context, cli client.Client, licenseKey apiTypes.NamespacedName, status crd.LicenseState) {
	Eventually(func() error {
		license := &crd.License{}
		_ = cli.Get(ctx, licenseKey, license)
		license.Status.Status = status
		return cli.Status().Update(ctx, license)
	}, Timeout, Interval).ShouldNot(HaveOccurred())
}

func TearDownBackup(acc *kube.Accessor, backupName, backupNamespace string) {
	By("Deleting the backup")
	SetBackupStatus(acc, backupName, backupNamespace, crd.Failed)

	acc.DeleteBackup(types.NamespacedName{Name: backupName, Namespace: backupNamespace})
}

func VerifyCustomMetaContent(componentMetaList []apis.ComponentMetadata, resourceMap map[string][]string) {
	log.Info("verifying MetaData")
	responseMap := make(map[string][]string)
	for componentIndex := range componentMetaList {
		compMetaData := componentMetaList[componentIndex]
		gvkStr := GetGVKString(&compMetaData.GroupVersionKind)
		log.Info(gvkStr)
		if _, exists := resourceMap[gvkStr]; !exists {
			log.Errorf("Gvk not found in resourceMap, %s", gvkStr)
			log.Info("Resources map : ", componentMetaList)
			log.Info("Expected map : ", resourceMap)
			Fail(fmt.Sprintf("GVK not found in resourceMap: %s", gvkStr))
		}
		var responseGVKMetaList []string
		obj := unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind(compMetaData.GroupVersionKind))
		for metaIndex := range compMetaData.Metadata {
			meta := compMetaData.Metadata[metaIndex]

			if err := json.Unmarshal([]byte(meta), &obj); err != nil {
				log.Info(err)
				Fail(err.Error())
			}
			responseGVKMetaList = append(responseGVKMetaList, obj.GetName())
		}
		sort.Strings(responseGVKMetaList)
		responseMap[gvkStr] = responseGVKMetaList
	}
	if !CheckMetaNameLists(responseMap, resourceMap) {
		Fail("meta name not found in resourceMap")
	}
	log.Info("Meta verification passed")
}

func VerifyOperatorMetaSnapshot(targetSnapShot *apis.FullSnapshot, rm map[string]interface{}) {
	log.Info("verifying OperatorMeta")
	operatorSnapshot := targetSnapShot.OperatorSnapshots

	if len(operatorSnapshot) != len(rm) {
		log.Info(fmt.Sprintf("snapshot length: %+v", len(operatorSnapshot)))
		log.Info(fmt.Sprintf("resourceMap length: %+v", len(rm)))
		Fail("Number of Operator snapshot mismatch")
	}

	for i := range operatorSnapshot {
		snap := operatorSnapshot[i]
		val, exists := rm[snap.OperatorID].(map[string]interface{})
		if !exists {
			Fail(fmt.Sprintf("Expected operatorId not found: %s", snap.OperatorID))
		}

		// verify operatorResources
		VerifyCustomMetaContent(snap.OperatorResources, val["componentMetadata"].(map[string][]string))

		// verify operator customResources
		VerifyCustomMetaContent(snap.CustomResources, val["customResource"].(map[string][]string))

		CRDs := val["crd"].(map[string][]string)
		var CRDNames []string
		for _, crdList := range CRDs {
			CRDNames = append(CRDNames, crdList...)
		}

		for _, crdMeta := range snap.CRDMetadata {
			MysqlCRDObj := unstructured.Unstructured{}
			if err := json.Unmarshal([]byte(crdMeta), &MysqlCRDObj); err != nil {
				log.Error(err)
				Fail(err.Error())
			}
			if !checkMetaName(MysqlCRDObj.GetName(), CRDNames) {
				log.Errorf("meta name not found in resourceMap, gvk: %s, name: %s",
					MysqlCRDObj.GroupVersionKind().String(), MysqlCRDObj.GetName())
				Fail("meta name not found in resourceMap")
			}
		}

		if hs, exist := val["helmSnapshot"].(map[string]interface{}); exist {
			VerifyHelmMetaSnapshot(&apis.FullSnapshot{HelmSnapshots: apis.HelmSnapshots{*snap.Helm}}, hs)
		}

	}
	log.Info("OperatorMeta verification passed")
}

func checkMetaName(key string, gvkMetaList []string) bool {
	for _, val := range gvkMetaList {
		if val == key {
			return true
		}
	}
	return false
}

func VerifyHelmMetaSnapshot(targetSnapshot *apis.FullSnapshot, resourceMap map[string]interface{}) {
	log.Info("verifying HelmMetaData")

	hSnaps := targetSnapshot.HelmSnapshots

	if len(hSnaps) != len(resourceMap) {
		log.Warnf(fmt.Sprintf("snapshot length: %+v", len(hSnaps)))
		log.Warnf(fmt.Sprintf("resourceMap length: %+v", len(resourceMap)))
		Fail("Number of Helm snapshot mismatch")
	}

	for i := range hSnaps {
		hs := hSnaps[i]
		val, exists := resourceMap[hs.Release].(map[string]interface{})
		if !exists {
			Fail(fmt.Sprintf("Expected Release not found. In resourceMap: %+v, in snapshot: %s", resourceMap,
				hs.Release))
		}
		componentMeta := *hs.Metadata
		var componentMetaDateList []apis.ComponentMetadata
		componentMetaDateList = append(componentMetaDateList, componentMeta)
		VerifyCustomMetaContent(componentMetaDateList, val["componentMetadata"].(map[string][]string))
		r := val["revision"]
		if hs.Revision != r.(int32) {
			Fail("Wrong revision of the helm chart")
		}
		rel := val["release"]
		if hs.Release != rel.(string) {
			Fail(fmt.Sprintf("Invalid release name. In Smapshot: %s, in ResourceMap: %s", hs.Release, rel.(string)))
		}
		version := val["version"]
		if string(hs.Version) != version {
			Fail("Invalid helm version")
		}
		sb := val["storageBackend"]
		if sb != string(hs.StorageBackend) {
			Fail("Invalid helm storage backend")
		}

	}
	log.Info("HelmMeta verification passed")

}

func VerifyHelmSubCharts(targetSnapshot *apis.FullSnapshot, resourceMap map[string]interface{}, backupLocation string) {
	log.Info("Verifying helm dependency subcharts")

	hSnaps := targetSnapshot.HelmSnapshots

	if len(hSnaps) != len(resourceMap) {
		log.Warnf(fmt.Sprintf("snapshot length: %+v", len(hSnaps)))
		log.Warnf(fmt.Sprintf("resourceMap length: %+v", len(resourceMap)))
		Fail("Number of Helm snapshot mismatch")
	}

	for i := range hSnaps {
		hs := hSnaps[i]
		val, exists := resourceMap[hs.Release].(map[string]interface{})
		if !exists {
			Fail(fmt.Sprintf("Expected Release not found. In resourceMap: %+v, in snapshot: %s", resourceMap,
				hs.Release))
		}

		if dep, exists := val["dependencies"]; exists {
			dependencies := dep.(map[int][]string)
			for rev, value := range dependencies {
				for _, d := range value {
					charLoc := path.Join(backupLocation, internal.HelmBackupDir, hs.Release, internal.HelmDependencyDir,
						strconv.Itoa(rev), d)
					_, err := loader.Load(charLoc)
					Expect(err).To(BeNil())
				}
			}
		}
	}
	log.Info("helm dependency subcharts verification passed")

}
func CleanupResources(cleanupScriptPath, backupNs, installNs string) {
	// Clean up before start of the test suit
	log.Infof("Cleaning up everything before setting up suite from %s backupNs", backupNs)
	_, err := shell.RunCmd(fmt.Sprintf("%s %s", cleanupScriptPath, backupNs))
	Expect(err).To(BeNil())
	log.Infof("Cleaning up everything before setting up suite from %s installNs", installNs)
	_, err = shell.RunCmd(fmt.Sprintf("%s %s", cleanupScriptPath, installNs))
	Expect(err).To(BeNil())
}

func GetCustomWithPVCResourceMap(uniqueID, testSc string) map[string][]string {
	return map[string][]string{
		"apps/v1,kind=Deployment": {uniqueID + "-nginx-deployment"},
		"v1,kind=Service": {uniqueID + "-nginx-deployment-svc", uniqueID + "-nginx-rc-svc",
			uniqueID + "-nginx-pod-svc", uniqueID + "-nginx-rs-svc", uniqueID + "-nginx-sts-svc"},
		"v1,kind=Pod":                                          {uniqueID + "-nginx-pod", uniqueID + "-pod-raw"},
		"apps/v1,kind=StatefulSet":                             {uniqueID + "-nginx-sts"},
		"v1,kind=ReplicationController":                        {uniqueID + "-nginx-rc"},
		"apps/v1,kind=ReplicaSet":                              {uniqueID + "-nginx-rs"},
		"storage.k8s.io/v1,kind=StorageClass":                  {uniqueID + "-sc-test", testSc},
		"rbac.authorization.k8s.io/v1,kind=ClusterRole":        {uniqueID + "-clusterrole-test"},
		"v1,kind=ServiceAccount":                               {uniqueID + "-sa-test"},
		"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {uniqueID + "-clusterrolebinding-test"},
		"v1,kind=ConfigMap":                                    {uniqueID + "-configmap-test"},
		"v1,kind=Secret":                                       {uniqueID + "-secret-test"},
		"apps/v1,kind=DaemonSet":                               {uniqueID + "-fluentd-elasticsearch"},
		"policy/v1beta1,kind=PodSecurityPolicy":                {uniqueID + "-pod-sec-policy"},
		"networking.k8s.io/v1,kind=NetworkPolicy":              {uniqueID + "-network-policy"},
		"batch/v1,kind=Job":                                    {uniqueID + "-ubuntu-job"},
		"batch/v1beta1,kind=CronJob":                           {uniqueID + "-ubuntu-cronjob"},
		"admissionregistration.k8s.io/v1,kind=ValidatingWebhookConfiguration": {uniqueID + "-validation-webhook"},
		"admissionregistration.k8s.io/v1,kind=MutatingWebhookConfiguration":   {uniqueID + "-mutation-webhook"},
		"scheduling.k8s.io/v1,kind=PriorityClass":                             {uniqueID + "-high-priority"},
		"networking.k8s.io/v1beta1,kind=Ingress":                              {uniqueID + "-test-ingress"},
		"autoscaling/v1,kind=HorizontalPodAutoscaler":                         {uniqueID + "-demo-hpa"},
	}
}

func GetCustomWithoutPVCResourceMap(uniqueID, testSc string) map[string][]string {
	return map[string][]string{
		"apps/v1,kind=Deployment": {uniqueID + "-nginx-deployment"},
		"v1,kind=Service": {uniqueID + "-nginx-deployment-svc", uniqueID + "-nginx-rc-svc",
			uniqueID + "-nginx-pod-svc", uniqueID + "-nginx-rs-svc", uniqueID + "-nginx-sts-svc"},
		"v1,kind=Pod":                                          {uniqueID + "-nginx-pod", uniqueID + "-pod-raw"},
		"apps/v1,kind=StatefulSet":                             {uniqueID + "-nginx-sts"},
		"v1,kind=ReplicationController":                        {uniqueID + "-nginx-rc"},
		"apps/v1,kind=ReplicaSet":                              {uniqueID + "-nginx-rs"},
		"storage.k8s.io/v1,kind=StorageClass":                  {uniqueID + "-sc-test"},
		"rbac.authorization.k8s.io/v1,kind=ClusterRole":        {uniqueID + "-clusterrole-test"},
		"v1,kind=ServiceAccount":                               {uniqueID + "-sa-test"},
		"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {uniqueID + "-clusterrolebinding-test"},
		"v1,kind=ConfigMap":                                    {uniqueID + "-configmap-test"},
		"v1,kind=Secret":                                       {uniqueID + "-secret-test"},
		"apps/v1,kind=DaemonSet":                               {uniqueID + "-fluentd-elasticsearch"},
		"policy/v1beta1,kind=PodSecurityPolicy":                {uniqueID + "-pod-sec-policy"},
		"networking.k8s.io/v1,kind=NetworkPolicy":              {uniqueID + "-network-policy"},
		"batch/v1,kind=Job":                                    {uniqueID + "-ubuntu-job"},
		"batch/v1beta1,kind=CronJob":                           {uniqueID + "-ubuntu-cronjob"},
		"admissionregistration.k8s.io/v1,kind=ValidatingWebhookConfiguration": {uniqueID + "-validation-webhook"},
		"admissionregistration.k8s.io/v1,kind=MutatingWebhookConfiguration":   {uniqueID + "-mutation-webhook"},
		"scheduling.k8s.io/v1,kind=PriorityClass":                             {uniqueID + "-high-priority"},
		"networking.k8s.io/v1beta1,kind=Ingress":                              {uniqueID + "-test-ingress"},
		"autoscaling/v1,kind=HorizontalPodAutoscaler":                         {uniqueID + "-demo-hpa"},
	}
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func GetConfigMapNamesListForNamespaceBackup
func GetSecretNamesListForNamespaceBackup(acc *kube.Accessor, ns string) []string {
	var (
		secretList  *corev1.SecretList
		secretNames []string
		err         error
	)

	// get secret list from install namespace as autogenerated secret names cannot be identified beforehand to put in resourceMap
	secretList, err = acc.GetSecrets(ns)
	if err != nil {
		Fail(fmt.Sprintf("failed to get secret list from Namespace=%s", ns))
	}
	if secretList != nil {
		for i := range secretList.Items {
			secret := secretList.Items[i]
			// drop helm secrets from list
			res := decorator.UnstructResource{}
			Expect(res.ToUnstructured(&secret)).NotTo(HaveOccurred())
			unStr := unstructured.Unstructured(res)
			// check if it's helm secret
			if helpers.IsHelmSecretOrConfigMap(unStr) || helpers.CheckIfTVKLabelExists(unStr) || helpers.IsTvkSAReferredSecret(unStr) {
				continue
			}

			secretNames = append(secretNames, secret.GetName())
		}
	}

	return secretNames
}

func PopulateBackupFromFile(acc *kube.Accessor, backupName, ns, filePath string) error {
	var (
		backup       *crd.Backup
		parsedBackup *crd.Backup
		content      []byte
	)
	backup, err = acc.GetBackup(backupName, ns)
	if err != nil {
		return fmt.Errorf("error while getting the backup cr to update, error: %s", err.Error())
	}

	content, err = ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error while reading the file: %s", err.Error())
	}

	parsedBackup = &crd.Backup{}
	err = yaml.Unmarshal(content, parsedBackup)
	if err != nil {
		return fmt.Errorf("error while unmarshalling the backup from file: %s", err.Error())
	}

	backup.Status = parsedBackup.Status
	err = acc.StatusUpdate(backup)
	if err != nil {
		return fmt.Errorf("error while updating the backup: %s", err.Error())
	}

	return nil
}

func CheckIfDataPersists(ns, releaseName string) (string, error) {
	log.Infof("Checking data in pod of release: [%s]", releaseName)
	checkData := fmt.Sprintf("%s check %s %s", mySQLFillDataScript, ns, releaseName)
	cmdOut, err := shell.RunCmd(checkData)
	log.Infof("Table entries after restore %s\n", cmdOut.Out)
	if err != nil {
		return "", err
	}

	return cmdOut.Out, nil
}

func GetBackupSnapshotterContainer(namespace, backupName, targetName, containerName string) *corev1.Container {

	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	metaSnapshotCommand := fmt.Sprintf("/opt/tvk/metamover %s --backup-name %s --namespace %s",
		internal.SnapshotAction, backupName, namespace)

	command := fmt.Sprintf("%s && %s", dataAttacherCommand, metaSnapshotCommand)

	image := GetMetamoverImage()

	snapshotterContainer := controllerHelpers.GetContainer(containerName, image, command, true, internal.NonDMJobResource, internal.MountCapability)
	snapshotterContainer.Env = append(snapshotterContainer.Env, []corev1.EnvVar{
		{
			Name:  internal.TVKVersion,
			Value: GetReleaseTagForProwTests(),
		},
		{
			Name:  internal.InstallNamespace,
			Value: GetInstallNamespace(),
		},
	}...)

	return snapshotterContainer
}

func GetHookContainer(namespace, action, crName, kind string) *corev1.Container {

	var hookCommand string

	if kind == internal.BackupKind {
		hookCommand = fmt.Sprintf("/opt/tvk/hook-executor --action %s --backup-name %s", action, crName)
		if namespace != "" {
			hookCommand += " --namespace " + namespace
		}

	}

	if kind == internal.RestoreKind {
		hookCommand = fmt.Sprintf("/opt/tvk/hook-executor --action %s --restore-name %s", action, crName)
		if namespace != "" {
			hookCommand += " --namespace " + namespace
		}
	}

	return controllerHelpers.GetContainer(action, GetHookImage(), hookCommand, false,
		internal.NonDMJobResource, internal.GeneralCap)
}

func CreateSnapshotterPod(snapshotPodKey, backupKey types.NamespacedName,
	targetName, snapshotContainer string) {
	var backup *crd.Backup
	snapshotterContainer := GetBackupSnapshotterContainer(snapshotPodKey.Namespace, backupKey.Name,
		targetName, snapshotContainer)

	snapshotterPod := CreatePod(snapshotPodKey.Namespace, "",
		snapshotterContainer, corev1.RestartPolicyNever, nil)

	// Overwrite the pod name
	snapshotterPod.Name = snapshotPodKey.Name

	backup, err = kubeAccessor.GetBackup(backupKey.Name, backupKey.Namespace)
	Expect(err).ShouldNot(HaveOccurred())

	Expect(ctrl.SetControllerReference(backup, snapshotterPod, scheme.Scheme)).To(BeNil())
	Expect(kubeAccessor.GetKubeClient().Create(testCtx, snapshotterPod)).To(BeNil())

	Eventually(func() error {
		_, err = kubeAccessor.GetPod(snapshotPodKey.Namespace, snapshotPodKey.Name)
		return err
	}, time.Second*30, time.Second*1).ShouldNot(HaveOccurred())

}

func GetDeploymentRetryOptions() []retry.Option {
	opts := make([]retry.Option, 0, 2)
	opts = append(opts, retry.Timeout(time.Minute*5), retry.Delay(time.Millisecond*300), retry.Count(60))
	return opts
}

func CheckDataInBlockVolume(podName, namespace string) (string, error) {
	log.Infof("Checking data in pod of block volume: [%s]", podName)
	checkData := fmt.Sprintf("%s %s %s", blockDataVerifyScript, namespace, podName)
	log.Infof(checkData)
	cmdOut, err := shell.RunCmd(checkData)
	log.Infof("Data after restore %s\n", cmdOut.Out)
	if err != nil {
		log.Info(err.Error())
		return "", err
	}

	return cmdOut.Out, nil
}

func GetOCPNamespaceResourceMap(uniqueSuitID string, secretNames []string) map[string][]string {

	var filteredSecrets []string
	for i := range secretNames {
		if strings.Contains(secretNames[i], "dockercfg") {
			continue
		}
		filteredSecrets = append(filteredSecrets, secretNames[i])
	}

	return map[string][]string{
		"route.openshift.io/v1,kind=Route":           {uniqueSuitID + "-route"},
		"monitoring.coreos.com/v1,kind=Alertmanager": {uniqueSuitID + "-am"},
		"apiextensions.k8s.io/v1,kind=CustomResourceDefinition": {"alertmanagers.monitoring.coreos.com", "podmonitors.monitoring.coreos.com",
			"prometheuses.monitoring.coreos.com", "prometheusrules.monitoring.coreos.com", "servicemonitors.monitoring.coreos.com"},
		"monitoring.coreos.com/v1,kind=Prometheus":     {uniqueSuitID + "-prom"},
		"monitoring.coreos.com/v1,kind=ServiceMonitor": {uniqueSuitID + "-kube-state-metrics"},
		"monitoring.coreos.com/v1,kind=PrometheusRule": {uniqueSuitID + "-prom-rules"},
		"monitoring.coreos.com/v1,kind=PodMonitor":     {uniqueSuitID + "-pm"},

		"networking.k8s.io/v1beta1,kind=Ingress": {uniqueSuitID + "-ingress"},
		"v1,kind=Service": {uniqueSuitID + "-route-svc", uniqueSuitID + "-nginx-deployment-svc",
			uniqueSuitID + "-nginx-rc-svc", uniqueSuitID + "-nginx-pod-svc", uniqueSuitID + "-nginx-rs-svc", uniqueSuitID + "-nginx-sts-svc"},
		"apps/v1,kind=Deployment":       {uniqueSuitID + "-nginx-deployment"},
		"v1,kind=Pod":                   {uniqueSuitID + "-nginx-pod", uniqueSuitID + "-pod-raw"},
		"apps/v1,kind=StatefulSet":      {uniqueSuitID + "-nginx-sts"},
		"v1,kind=ReplicationController": {uniqueSuitID + "-nginx-rc"},
		"apps/v1,kind=ReplicaSet":       {uniqueSuitID + "-nginx-rs"},

		"rbac.authorization.k8s.io/v1,kind=RoleBinding": {"system:deployers", "system:image-builders",
			"system:image-pullers"},
		"rbac.authorization.k8s.io/v1,kind=ClusterRole": {uniqueSuitID + "-clusterrole-test", "system:deployer",
			"system:image-builder", "system:image-puller"},

		"v1,kind=ServiceAccount": {uniqueSuitID + "-sa-test", "builder",
			"default", "deployer"},

		"rbac.authorization.k8s.io/v1,kind=ClusterRoleBinding": {uniqueSuitID + "-clusterrolebinding-test"},
		"v1,kind=ConfigMap":                       {uniqueSuitID + "-configmap-test"},
		"v1,kind=Secret":                          filteredSecrets,
		"apps/v1,kind=DaemonSet":                  {uniqueSuitID + "-fluentd-elasticsearch"},
		"policy/v1beta1,kind=PodSecurityPolicy":   {uniqueSuitID + "-pod-sec-policy"},
		"networking.k8s.io/v1,kind=NetworkPolicy": {uniqueSuitID + "-network-policy"},
		"batch/v1,kind=Job":                       {uniqueSuitID + "-ubuntu-job"},
		"batch/v1beta1,kind=CronJob":              {uniqueSuitID + "-ubuntu-cronjob"},
		"admissionregistration.k8s.io/v1,kind=ValidatingWebhookConfiguration": {uniqueSuitID + "-validation-webhook"},
		"admissionregistration.k8s.io/v1,kind=MutatingWebhookConfiguration":   {uniqueSuitID + "-mutation-webhook"},
		"autoscaling/v1,kind=HorizontalPodAutoscaler":                         {uniqueSuitID + "-demo-hpa"},
	}
}

func CreateVeleroBackup(namespace, storageLocation, ttl, formatVersion, phase, backupType string,
	version int) *unstructured.Unstructured {

	backup := &unstructured.Unstructured{}

	// Create Spec
	spec := make(map[string]interface{})

	includedNamespaces := GenerateStringList(rand.Intn(5))
	excludedNamespaces := GenerateStringList(rand.Intn(5))
	includedResources := GenerateStringList(rand.Intn(5))
	excludedResources := GenerateStringList(rand.Intn(5))
	volumeSnapshotLocations := GenerateStringList(rand.Intn(5))
	defaultVolumesToRestic := IntegrationsBool[rand.Intn(2)]
	includeClusterResources := IntegrationsBool[rand.Intn(2)]
	snapshotVolumes := IntegrationsBool[rand.Intn(2)]

	populateIncludedExcludedNamespacesAndResources(&spec, includedNamespaces, excludedNamespaces,
		includedResources, excludedResources)

	// create matchLabels with random number of elements
	matchLabelMaker := rand.Intn(5)
	matchLabels := map[string]string{}
	for i := 0; i < matchLabelMaker; i++ {
		matchLabels[internal.GenerateRandomString(5, true)] = internal.GenerateRandomString(10, true)
	}

	// create matchExpressions with random number of elements
	matchExpressionMaker := rand.Intn(5)
	matchExpressions := make([]metav1.LabelSelectorRequirement, matchExpressionMaker)
	for i := 0; i < matchExpressionMaker; i++ {
		matchExpressionValueMaker := rand.Intn(5) + 1
		matchExpressionValues := make([]string, matchExpressionValueMaker)
		op := LabelSelectorOperators[rand.Intn(4)]
		if op != metav1.LabelSelectorOpDoesNotExist && op != metav1.LabelSelectorOpExists {
			for j := 0; j < matchExpressionValueMaker; j++ {
				matchExpressionValues[j] = internal.GenerateRandomString(rand.Intn(20)+3, true)
			}
			matchExpressions[i] = metav1.LabelSelectorRequirement{
				Key:      internal.GenerateRandomString(rand.Intn(20)+3, true),
				Operator: op,
				Values:   matchExpressionValues,
			}
		} else {
			matchExpressions[i] = metav1.LabelSelectorRequirement{
				Key:      internal.GenerateRandomString(rand.Intn(20)+3, true),
				Operator: op,
			}
		}
	}

	if matchExpressionMaker > 0 || matchLabelMaker > 0 {
		spec["labelSelector"] = &metav1.LabelSelector{
			MatchLabels:      matchLabels,
			MatchExpressions: matchExpressions,
		}
	}

	spec["snapshotVolumes"] = &snapshotVolumes

	duration, _ := time.ParseDuration(ttl)
	spec["ttl"] = metav1.Duration{Duration: duration}

	spec["includeClusterResources"] = &includeClusterResources

	hooks := make([]map[string]interface{}, matchExpressionMaker)
	for i := 0; i < matchExpressionMaker; i++ {
		hookTimeout, _ := time.ParseDuration(strconv.Itoa(rand.Intn(10)+1) + "m")
		commands := make([]string, matchExpressionMaker+1)
		for j := 0; j < matchExpressionMaker+1; j++ {
			commands[j] = internal.GenerateRandomString(rand.Intn(20)+3, true)
		}
		onError := []string{"Continue", "Fail"}[rand.Intn(5)%2]

		hooks[i] = map[string]interface{}{
			"exec": &map[string]interface{}{
				"container": internal.GenerateRandomString(rand.Intn(20)+3, true),
				"command":   commands,
				"onError":   onError,
				"timeout":   metav1.Duration{Duration: hookTimeout},
			},
		}
	}

	if matchExpressionMaker > 0 {
		spec["hooks"] = map[string]interface{}{
			"resources": []map[string]interface{}{
				{
					"name":               internal.GenerateRandomString(rand.Intn(20)+3, true),
					"includedNamespaces": GenerateStringList(rand.Intn(5) + 1),
					"excludedNamespaces": GenerateStringList(rand.Intn(5) + 1),
					"includedResources":  GenerateStringList(rand.Intn(5) + 1),
					"excludedResources":  GenerateStringList(rand.Intn(5) + 1),
					"labelSelector": &metav1.LabelSelector{
						MatchLabels:      matchLabels,
						MatchExpressions: matchExpressions,
					},
					"pre":  hooks,
					"post": hooks,
				},
			},
		}
	}

	spec["storageLocation"] = storageLocation

	if len(volumeSnapshotLocations) > 0 {
		spec["volumeSnapshotLocations"] = volumeSnapshotLocations
	}

	spec["defaultVolumesToRestic"] = &defaultVolumesToRestic

	spec["orderedResources"] = map[string]string{
		// for namespaced resources use 'namespace/resourcename'
		"KindOne": "my-namespace1/my-backup1,my-namespace2/my-backup2,my-namespace3/my-backup100",
		"KindTwo": "my-namespace1/my-backup5,my-namespace2/my-backup7,my-namespace3/my-backup400",
		// for cluster-scoped resources use 'resourcename'
		"ClusterScopeKindOne": "clusterResourceOne,clusterResourceTwo",
	}

	// Create Status
	status := make(map[string]interface{})

	status["version"] = version

	status["formatVersion"] = formatVersion

	status["expiration"] = &metav1.Time{Time: clock.RealClock{}.Now().Add(duration)}

	status["phase"] = phase

	duration, _ = time.ParseDuration("-5m")
	status["startTimestamp"] = &metav1.Time{Time: clock.RealClock{}.Now().Add(duration)}

	volumeSnapshotsCompleted := rand.Intn(10)
	volumeSnapshotsAttempted := volumeSnapshotsCompleted + rand.Intn(10) + 1

	if phase == "Completed" {
		status["completionTimestamp"] = &metav1.Time{Time: clock.RealClock{}.Now().Add(duration)}
		volumeSnapshotsCompleted = volumeSnapshotsAttempted
	} else {
		status["errors"] = rand.Intn(10)
	}

	if phase != "FailedValidation" {
		itemsBackedUp := rand.Intn(100) + 1
		totalItems := itemsBackedUp + rand.Intn(50) + 1
		if phase == "Completed" {
			totalItems = itemsBackedUp
		}
		status["progress"] = map[string]interface{}{
			"totalItems":    totalItems,
			"itemsBackedUp": itemsBackedUp,
		}
	} else {
		status["validationErrors"] = []string{
			"Assume this to be some validation error - 1.",
			"Assume this to be some validation error - 2.",
			"Assume this to be some validation error - n.",
		}
	}

	status["volumeSnapshotsAttempted"] = volumeSnapshotsAttempted

	status["volumeSnapshotsCompleted"] = volumeSnapshotsCompleted

	warnings := rand.Intn(10)
	if warnings > 0 {
		status["warnings"] = warnings
	}

	scheduleLab := scheduleLabel
	if backupType == string(backup2.OnDemand) {
		scheduleLab = sampleScheduleLabel
	}

	// Create object
	backup.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "sample-velero-backup-" + internal.GenerateRandomString(rand.Intn(6)+3, true),
			"namespace": namespace,
			"labels": map[string]string{
				scheduleLab: sampleScheduleValue,
			},
		},
		"spec":   spec,
		"status": status,
	}

	// Set GVK to object
	backup.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   integrations.VeleroGroup,
		Version: internal.V1Version,
		Kind:    string(integrations.VeleroBackupKind),
	})

	return backup
}

func CreateVeleroRestore(namespace, phase string, backupsNames *[]string) *unstructured.Unstructured {

	includedNamespaces := GenerateStringList(rand.Intn(24) % 5)
	excludedNamespaces := GenerateStringList(rand.Intn(24) % 5)
	includedResources := GenerateStringList(rand.Intn(24) % 5)
	excludedResources := GenerateStringList(rand.Intn(24) % 5)

	restore := &unstructured.Unstructured{}

	// Create Spec
	spec := make(map[string]interface{})

	backupName := (*backupsNames)[rand.Intn(len(*backupsNames))]

	isScheduled := IntegrationsBool[rand.Intn(2)]
	if isScheduled {
		spec["scheduleName"] = backupName
		spec["backupName"] = backupName + "-" + strconv.Itoa(rand.Intn(3578)+10000)
	} else {
		spec["backupName"] = backupName
	}

	populateIncludedExcludedNamespacesAndResources(&spec, includedNamespaces, excludedNamespaces,
		includedResources, excludedResources)

	mapToSameNamespace := GenerateStringList(rand.Intn(3))
	if len(mapToSameNamespace) > 0 {
		for _, ns := range mapToSameNamespace {
			spec["namespaceMapping"] = map[string]string{
				ns: internal.GenerateRandomString(rand.Intn(6)+3, true),
			}
		}
	}

	// create matchLabels with random number of elements
	matchLabelMaker := rand.Intn(5)
	matchLabels := map[string]string{}
	for i := 0; i < matchLabelMaker; i++ {
		matchLabels[internal.GenerateRandomString(5, true)] = internal.GenerateRandomString(10, true)
	}

	// create matchExpressions with random number of elements
	matchExpressionMaker := rand.Intn(5)
	matchExpressions := make([]metav1.LabelSelectorRequirement, matchExpressionMaker)
	for i := 0; i < matchExpressionMaker; i++ {
		matchExpressionValueMaker := rand.Intn(5) + 1
		matchExpressionValues := make([]string, matchExpressionValueMaker)
		op := LabelSelectorOperators[rand.Intn(4)]
		if op != metav1.LabelSelectorOpDoesNotExist && op != metav1.LabelSelectorOpExists {
			for j := 0; j < matchExpressionValueMaker; j++ {
				matchExpressionValues[j] = internal.GenerateRandomString(rand.Intn(20)+3, true)
			}
			matchExpressions[i] = metav1.LabelSelectorRequirement{
				Key:      internal.GenerateRandomString(rand.Intn(20)+3, true),
				Operator: op,
				Values:   matchExpressionValues,
			}
		} else {
			matchExpressions[i] = metav1.LabelSelectorRequirement{
				Key:      internal.GenerateRandomString(rand.Intn(20)+3, true),
				Operator: op,
			}
		}
	}

	if matchExpressionMaker > 0 || matchLabelMaker > 0 {
		spec["labelSelector"] = &metav1.LabelSelector{
			MatchLabels:      matchLabels,
			MatchExpressions: matchExpressions,
		}
	}

	spec["restorePVs"] = &IntegrationsBool[rand.Intn(2)]

	spec["preserveNodePorts"] = &IntegrationsBool[rand.Intn(2)]

	spec["includeClusterResources"] = &IntegrationsBool[rand.Intn(2)]

	randomNumberOfHooks := rand.Intn(5)
	restoreResourceHooksSpec := make([]map[string]interface{}, randomNumberOfHooks)
	if randomNumberOfHooks > 0 {
		for j := 0; j < randomNumberOfHooks; j++ {
			randomNumberOfRestoreHooks := rand.Intn(5) + 1
			restoreResourceHooks := make([]map[string]interface{}, randomNumberOfRestoreHooks)
			for i := 0; i < randomNumberOfRestoreHooks; i++ {

				restoreResourceHooks[i] = make(map[string]interface{})

				if cast.ToBool(rand.Intn(2)) {
					waitTimeout, _ := time.ParseDuration(strconv.Itoa(rand.Intn(10)+1) + "m")
					restoreResourceHooks[i]["init"] = &map[string]interface{}{
						"initContainers": getDummyContainerStructList(rand.Intn(5) + 1),
						"timeout":        metav1.Duration{Duration: waitTimeout},
					}
				}

				execTimeout, _ := time.ParseDuration(strconv.Itoa(rand.Intn(10)+1) + "m")
				waitTimeout, _ := time.ParseDuration(strconv.Itoa(rand.Intn(10)+1) + "m")
				commands := make([]string, matchExpressionMaker+1)
				for j := 0; j < matchExpressionMaker+1; j++ {
					commands[j] = internal.GenerateRandomString(rand.Intn(20)+3, true)
				}
				onError := []string{"Continue", "Fail"}[rand.Intn(5)%2]

				restoreResourceHooks[i]["exec"] = &map[string]interface{}{
					"container":   internal.GenerateRandomString(rand.Intn(20)+3, true),
					"command":     commands,
					"onError":     onError,
					"execTimeout": metav1.Duration{Duration: execTimeout},
					"waitTimeout": metav1.Duration{Duration: waitTimeout},
				}

			}

			restoreResourceHooksSpec[j] = map[string]interface{}{}

			restoreResourceHooksSpec[j]["name"] = internal.GenerateRandomString(rand.Intn(20)+3, true)

			populateIncludedExcludedNamespacesAndResources(&restoreResourceHooksSpec[j], includedNamespaces, excludedNamespaces,
				includedResources, excludedResources)

			if matchExpressionMaker > 0 || matchLabelMaker > 0 {
				restoreResourceHooksSpec[j]["labelSelector"] = &metav1.LabelSelector{
					MatchLabels:      matchLabels,
					MatchExpressions: matchExpressions,
				}
			}

			restoreResourceHooksSpec[j]["postHooks"] = restoreResourceHooks

		}
		spec["hooks"] = map[string]interface{}{
			"resources": restoreResourceHooksSpec,
		}
	}

	// Create Status
	status := make(map[string]interface{})
	status["phase"] = phase

	if phase == "FailedValidation" {
		status["validationErrors"] = []string{
			"Assume this to be some validation error - 1.",
			"Assume this to be some validation error - 2.",
			"Assume this to be some validation error - n.",
		}
	}

	warnings := rand.Intn(10)
	if warnings > 0 {
		status["warnings"] = warnings
	}

	duration, _ := time.ParseDuration("-5m")
	status["startTimestamp"] = &metav1.Time{Time: clock.RealClock{}.Now().Add(duration)}

	if phase == "Completed" {
		status["completionTimestamp"] = &metav1.Time{Time: clock.RealClock{}.Now()}
	} else {
		status["errors"] = rand.Intn(10)
	}

	if phase == "Failed" {
		status["failureReason"] = "Assume this to be some failure reason"
	}

	// Create object
	restore.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "sample-velero-restore-" + internal.GenerateRandomString(rand.Intn(6)+3, true),
			"namespace": namespace,
		},
		"spec":   spec,
		"status": status,
	}

	// Set GVK to object
	restore.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   integrations.VeleroGroup,
		Version: internal.V1Version,
		Kind:    string(integrations.VeleroRestoreKind),
	})

	return restore
}

func CreateVeleroBSL(namespace, phase string) *unstructured.Unstructured {

	bsl := &unstructured.Unstructured{}

	// Create Spec
	spec := make(map[string]interface{})

	spec["provider"] = CloudProvders[rand.Intn(5)%len(CloudProvders)]

	randomInt := rand.Intn(5) + 1
	configMap := map[string]string{}
	for i := 0; i < randomInt; i++ {
		configMap[internal.GenerateRandomString(rand.Intn(6)+3, true)] = internal.GenerateRandomString(rand.Intn(6)+3, true)
	}
	spec["config"] = configMap

	caCert := make([]byte, rand.Intn(50)+1000)
	rand.Read(caCert)
	spec["objectStorage"] = map[string]interface{}{
		"bucket": internal.GenerateRandomString(rand.Intn(6)+3, true),
		"prefix": internal.GenerateRandomString(rand.Intn(6)+3, true),
		"caCert": caCert,
	}

	spec["default"] = IntegrationsBool[rand.Intn(2)]

	spec["credential"] = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: internal.GenerateRandomString(rand.Intn(6)+3, true),
		},
		Key:      internal.GenerateRandomString(rand.Intn(6)+3, true),
		Optional: &IntegrationsBool[rand.Intn(2)],
	}

	spec["accessMode"] = "ReadWrite"

	duration, _ := time.ParseDuration(strconv.Itoa(randomInt) + "m")
	spec["backupSyncPeriod"] = &metav1.Duration{Duration: duration}

	duration, _ = time.ParseDuration(strconv.Itoa(randomInt) + "m")
	spec["validationFrequency"] = &metav1.Duration{Duration: duration}

	// Create Status
	status := make(map[string]interface{})

	status["phase"] = phase

	randomInt = rand.Intn(5) + 1
	duration, _ = time.ParseDuration("-" + strconv.Itoa(randomInt) + "m")
	status["lastSyncedTime"] = &metav1.Time{Time: clock.RealClock{}.Now().Add(duration)}

	randomInt = rand.Intn(5) + 1
	duration, _ = time.ParseDuration("-" + strconv.Itoa(randomInt) + "m")
	status["lastValidationTime"] = &metav1.Time{Time: clock.RealClock{}.Now().Add(duration)}

	// Create object
	bsl.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "sample-velero-target-" + internal.GenerateRandomString(rand.Intn(6)+3, true),
			"namespace": namespace,
		},
		"spec":   spec,
		"status": status,
	}

	// Set GVK to object
	bsl.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   integrations.VeleroGroup,
		Version: internal.V1Version,
		Kind:    string(integrations.VeleroBackupStorageLocationKind),
	})

	return bsl
}

func CreateVeleroVSL(namespace, phase string) *unstructured.Unstructured {

	vsl := &unstructured.Unstructured{}

	// Create Spec
	spec := make(map[string]interface{})

	spec["provider"] = CloudProvders[rand.Intn(5)%len(CloudProvders)]

	randomInt := rand.Intn(5) + 1
	configMap := map[string]string{}
	for i := 0; i < randomInt; i++ {
		configMap[internal.GenerateRandomString(rand.Intn(6)+3, true)] = internal.GenerateRandomString(rand.Intn(6)+3, true)
	}
	spec["config"] = configMap

	spec["credential"] = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: internal.GenerateRandomString(rand.Intn(6)+3, true),
		},
		Key:      internal.GenerateRandomString(rand.Intn(6)+3, true),
		Optional: &IntegrationsBool[rand.Intn(2)],
	}

	// Create Spec
	status := make(map[string]interface{})

	status["phase"] = phase

	// Create object
	vsl.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "sample-velero-target-" + internal.GenerateRandomString(rand.Intn(6)+3, true),
			"namespace": namespace,
		},
		"spec":   spec,
		"status": status,
	}

	// Set GVK to object
	vsl.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   integrations.VeleroGroup,
		Version: internal.V1Version,
		Kind:    string(integrations.VeleroVolumeSnapshotLocationKind),
	})

	return vsl
}

func GenerateStringList(n int) (stringList []string) {
	for i := 0; i < n; i++ {
		stringList = append(stringList, internal.GenerateRandomString(rand.Intn(20)+3, true))
	}
	return
}

func getDummyContainerStructList(randomContainers int) (containers []corev1.Container) {

	for i := 0; i < randomContainers; i++ {
		containers = append(containers, corev1.Container{
			Name:            internal.GenerateRandomString(rand.Intn(6)+3, true),
			Image:           internal.GenerateRandomString(rand.Intn(6)+3, true),
			Command:         GenerateStringList(rand.Intn(5) + 1),
			Args:            GenerateStringList(rand.Intn(5) + 1),
			Ports:           getDummyContainerPortList(rand.Intn(5) + 1),
			ImagePullPolicy: corev1.PullAlways,
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu": *resource.NewQuantity(50, resource.DecimalSI),
				},
				Requests: corev1.ResourceList{
					"cpu": *resource.NewQuantity(50, resource.DecimalSI),
				},
			},
		})
	}

	return
}

func getDummyContainerPortList(number int) (containerPorts []corev1.ContainerPort) {
	for i := 0; i < number; i++ {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          internal.GenerateRandomString(rand.Intn(6)+3, true),
			HostPort:      rand.Int31n(60000) + rand.Int31n(1000) + int32(1000),
			ContainerPort: rand.Int31n(60000) + rand.Int31n(1000) + int32(1000),
			Protocol:      corev1.ProtocolSCTP,
			HostIP:        getDummyIP(),
		})
	}
	return
}

func getDummyIP() string {
	return strconv.Itoa(rand.Intn(255)+1) + "." + strconv.Itoa(rand.Intn(255)+1) + "." +
		strconv.Itoa(rand.Intn(255)+1) + "." + strconv.Itoa(rand.Intn(255)+1)
}

func populateIncludedExcludedNamespacesAndResources(obj *map[string]interface{}, includedNamespaces []string, excludedNamespaces []string,
	includedResources []string, excludedResources []string) {
	if len(includedNamespaces) > 0 {
		(*obj)["includedNamespaces"] = includedNamespaces
	}

	if len(excludedNamespaces) > 0 {
		(*obj)["excludedNamespaces"] = excludedNamespaces
	}

	if len(includedResources) > 0 {
		(*obj)["includedResources"] = includedResources
	}

	if len(excludedResources) > 0 {
		(*obj)["excludedResources"] = excludedResources
	}
}
