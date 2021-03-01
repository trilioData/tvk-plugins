package helpers

import (
	"fmt"
	"os"
	"path"

	guid "github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/tvkconf"
	"github.com/trilioData/k8s-triliovault/internal/utils"
)

var (
	CreateDeviceDetectScriptCommand = fmt.Sprintf("(echo '#!/bin/sh' && stat -c 'mknod %s b 0x%%t 0x%%T' %s) >%s && chmod a+x %s",
		internal.PseudoBlockDevicePath, internal.PseudoBlockDevicePath, internal.DeviceDetectScript, internal.DeviceDetectScript)
	DetectBlockDeviceCommand = fmt.Sprintf("if [ ! -e %s ]; then sh %s; fi;", internal.PseudoBlockDevicePath, internal.DeviceDetectScript)
	ImgPullPolicy, _         = os.LookupEnv(internal.ImagePullPolicy)
)

// TODO: Fallback logic for failed job status
func IsJobConditionFailed(job *batchv1.Job) bool {
	jobConditionLength := len(job.Status.Conditions)
	if jobConditionLength > 0 {
		jobCondition := job.Status.Conditions[jobConditionLength-1]
		if jobCondition.Type == batchv1.JobFailed && jobCondition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

type JobStatus struct {
	Active, Completed, Failed bool
}

type JobListStatus struct {
	Active, Completed, Failed int
}

// Returns Status of a Job with single Pod
func GetJobStatus(job *batchv1.Job) (status JobStatus) {
	if job.Status.Failed > 0 {
		status.Failed = true
	} else if job.Status.Succeeded > 0 {
		status.Completed = true
	} else if job.Status.Active > 0 {
		status.Active = true
	} else {
		status.Failed = IsJobConditionFailed(job)
	}
	return status
}

func GetJobPhaseStatus(status JobStatus) v1.Status {
	if status.Failed {
		return v1.Failed
	} else if status.Completed {
		return v1.Completed
	} else if status.Active {
		return v1.InProgress
	} else {
		return ""
	}
}

// GenerateJobName generates the name for Job to handle label value length handling
func GenerateJobName(ownerName, containerName string) string {
	maxOwnerlength := internal.MaxNameOrLabelLen - len(containerName) - 2 - 6
	// If job name prefix is greater than 40 characters then trim
	if len(ownerName) > maxOwnerlength {
		ownerName = ownerName[:maxOwnerlength]
	}
	jobName := fmt.Sprintf("%s-%s-%s", ownerName, containerName, internal.GenerateRandomString(6, false))
	if len(validation.IsValidLabelValue(jobName)) != 0 {
		// fallback to job name
		// triliovault (11) - metamover/datamover (9) - uuid (36) + 2 = 58 < 63
		jobName = fmt.Sprintf("%s-%s-%s", internal.CategoryTriliovault, containerName, guid.New().String())
	}

	return jobName
}

func GetJob(ownerName, namespace string, container *corev1.Container,
	volumes []corev1.Volume, serviceAccount string) *batchv1.Job {

	name := GenerateJobName(ownerName, container.Name)
	recommendedLabels := internal.GetRecommendedLabels(name, internal.ManagedBy)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    recommendedLabels,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &internal.ActiveDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: volumes,
					Containers: []corev1.Container{
						*container,
					},
					ServiceAccountName: serviceAccount,
					Affinity:           internal.GetNodeAffinity(),
					HostIPC:            false,
					HostNetwork:        false,
					HostPID:            false,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: container.SecurityContext.RunAsNonRoot,
						RunAsUser:    container.SecurityContext.RunAsUser,
					},
				},
			},
		},
	}

	// Set backoff limit
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job.Spec.BackoffLimit = &internal.NeverBackoffLimit
	job.Spec.Template.Labels = recommendedLabels

	return job
}

func GetContainer(name, image, command string, isPrivileged bool, resourceType string,
	reqCapabilities []corev1.Capability) *corev1.Container {
	var (
		runAsUser    int64
		runAsNonRoot bool
	)
	if isPrivileged {
		runAsUser = internal.RunAsRootUserID
		runAsNonRoot = internal.RunAsNonRoot
	} else {
		runAsUser = internal.RunAsNonRootUserID
		runAsNonRoot = internal.RunAsNormalUser
	}

	container := &corev1.Container{
		Name:  name,
		Image: image,
		Command: []string{
			"/bin/sh",
			"-c",
			command,
		},
		Resources:       tvkconf.GetContainerResources(resourceType),
		ImagePullPolicy: corev1.PullPolicy(ImgPullPolicy),
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  reqCapabilities,
			},
			Privileged:               &isPrivileged,
			AllowPrivilegeEscalation: &isPrivileged,
			RunAsUser:                &runAsUser,
			RunAsNonRoot:             &runAsNonRoot,
			ReadOnlyRootFilesystem:   &internal.ReadOnlyRootFilesystem,
		},
	}

	// If profiling is enabled, add profiling related env var in container
	if profCollectorAddr := internal.GetProfilingCollectorAddr(); profCollectorAddr != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  internal.ProfilingCollector,
			Value: profCollectorAddr,
		})
	}

	return container
}

// GetDataAttacherCommand returns the data store attacher command
func GetDataAttacherCommand(namespace, targetName string) string {
	return fmt.Sprintf("python %s --namespace=%s --target-name=%s",
		path.Join(internal.BasePath, internal.DatastoreMountUtil), namespace, targetName)
}

func GetWaitUntilMountCommand() string {
	return fmt.Sprintf("python %s", path.Join(internal.BasePath, internal.DatastoreWaitUtil))
}

// GetDatastoreAttacherImage returns the docker image of datastore attacher from env vars
func GetDatastoreAttacherImage() string {
	image, present := os.LookupEnv(internal.DataStoreAttacherImage)
	if !present {
		panic("Datastore attacher image name not found in environment")
	}
	return image
}

// GetDataMoverImage returns the docker image of datamover from env vars
func GetDataMoverImage() string {
	image, present := os.LookupEnv(internal.DataMoverImage)
	if !present {
		panic("Datamover image name not found in environment")
	}
	return image
}

// GetMetaMoverImage returns the docker image of metamover from env vars
func GetMetaMoverImage() string {
	image, present := os.LookupEnv(internal.MetaMoverImage)
	if !present {
		panic("Metamover image name not found in environment")
	}
	return image
}

// GetBackupSchedulerImage returns the docker image of backup scheduler from env vars
func GetBackupSchedulerImage() string {
	image, present := os.LookupEnv(internal.BackupSchedulerImage)
	if !present {
		panic("BackupCron image name not found in environment")
	}
	return image
}

// GetBackupCleanerImage returns the docker image of backup-cleaner from env vars
func GetBackupCleanerImage() string {
	image, present := os.LookupEnv(internal.BackupCleanerImage)
	if !present {
		panic("Backup cleaner image name not found in environment")
	}
	return image
}

func GetBackupRetentionImage() string {
	image, present := os.LookupEnv(internal.BackupRetentionImage)
	if !present {
		panic("Backup retention image name not found in environment")
	}
	return image
}

func GetHookImage() string {
	image, present := os.LookupEnv(internal.HookImage)
	if !present {
		panic("Hook image name not found in environment")
	}
	return image
}

// Data/Meta mover container
func getMetaMoverContainer(command string) *corev1.Container {
	image := GetMetaMoverImage()
	containerName := internal.MetamoverContainer

	return GetContainer(containerName, image, command, true, internal.NonDMJobResource, internal.MountCapability)
}

func getDataMoverContainer(command string, volumeMode *corev1.PersistentVolumeMode) *corev1.Container {
	containerName := internal.DatamoverContainer
	dataMoverContainer := GetContainer(containerName, GetDataMoverImage(), command, true, internal.DMJobResource, internal.DatamoverCap)

	// For block device empty dir mounted device
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

// Backup Job Containers
func GetBackupCleanupContainer(namespace, backupPath, targetName string) *corev1.Container {

	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	metaSnapshotCommand := fmt.Sprintf("/opt/tvk/cleaner --backup-path=%s", backupPath)

	command := fmt.Sprintf("%s && %s", dataAttacherCommand, metaSnapshotCommand)

	return GetContainer(internal.BackupCleanupContainer, GetBackupCleanerImage(),
		command, true, internal.NonDMJobResource, internal.MountCapability)
}

func GetBackupSnapshotterContainer(namespace, backupName, targetName string) *corev1.Container {

	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	metaSnapshotCommand := fmt.Sprintf("/opt/tvk/metamover %s --backup-name %s --namespace %s",
		internal.SnapshotAction, backupName, namespace)

	command := fmt.Sprintf("%s && %s", dataAttacherCommand, metaSnapshotCommand)

	backupSnapContainer := getMetaMoverContainer(command)
	// Add INSTALL_NAMESPACE and VERSION env vars to container
	installNS := internal.GetInstallNamespace()
	tvkVersion := internal.GetTVKVersion()
	backupSnapContainer.Env = append(backupSnapContainer.Env,
		[]corev1.EnvVar{
			{Name: internal.InstallNamespace, Value: installNS},
			{Name: internal.TVKVersion, Value: tvkVersion},
		}...,
	)

	return backupSnapContainer
}

func GetBackupMetadataUploadContainer(namespace, backupName, targetName string) *corev1.Container {

	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	metaSnapshotCommand := fmt.Sprintf("/opt/tvk/metamover %s --backup-name %s --namespace %s",
		internal.MetadataUploadAction, backupName, namespace)

	command := fmt.Sprintf("%s && %s", dataAttacherCommand, metaSnapshotCommand)

	return getMetaMoverContainer(command)
}

func GetBackupRetentionContainer(namespace, backupName, targetName string) *corev1.Container {

	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	retentionCommand := fmt.Sprintf("/opt/tvk/retention --backup-name %s --namespace %s",
		backupName, namespace)
	command := fmt.Sprintf("%s && %s", dataAttacherCommand, retentionCommand)

	return GetContainer(internal.RetentionOperation, GetBackupRetentionImage(), command, true,
		internal.DMJobResource, internal.MountCapability)
}

func GetHookContainer(namespace, action, crName, kind string) *corev1.Container {

	var hookCommand string

	if kind == internal.BackupKind {
		hookCommand = fmt.Sprintf("/opt/tvk/hook-executor --action %s --backup-name %s --namespace %s",
			action, crName, namespace)

	}

	if kind == internal.RestoreKind {
		hookCommand = fmt.Sprintf("/opt/tvk/hook-executor --action %s --restore-name %s --namespace %s",
			action, crName, namespace)
	}

	return GetContainer(action, GetHookImage(), hookCommand, false,
		internal.NonDMJobResource, internal.GeneralCap)
}

// Restore Job Containers
func GetMetaValidationContainer(namespace, restoreName, targetName string) *corev1.Container {
	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	metaValidationCommand := fmt.Sprintf("/opt/tvk/metamover %s --name %s --run-primitive-restore --namespace %s",
		internal.ValidateAction, restoreName, namespace)

	command := fmt.Sprintf("%s && %s", dataAttacherCommand, metaValidationCommand)

	return getMetaMoverContainer(command)
}

// GetMetaRestoreContainer returns the container with the images and commands spec
func GetMetaRestoreContainer(namespace, restoreName, targetName string) *corev1.Container {
	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	metaRestoreCommmand := fmt.Sprintf("/opt/tvk/metamover %s --name %s --namespace %s",
		internal.RestoreAction, restoreName, namespace)

	command := fmt.Sprintf("%s && %s", dataAttacherCommand, metaRestoreCommmand)

	return getMetaMoverContainer(command)
}

func GetRestoreDatamoverContainer(namespace, restoreName, targetName string, pvc *corev1.PersistentVolumeClaim,
	appDs *helpers.ApplicationDataSnapshot) *corev1.Container {
	var (
		volumeMode corev1.PersistentVolumeMode
	)

	if pvc.Spec.VolumeMode == nil {
		volumeMode = corev1.PersistentVolumeFilesystem
	} else {
		volumeMode = *pvc.Spec.VolumeMode
	}
	volumePath := internal.MountPath
	blockDeviceDetect := ""
	if volumeMode == corev1.PersistentVolumeBlock {
		volumePath = internal.PseudoBlockDevicePath
		blockDeviceDetect = DetectBlockDeviceCommand
	}
	dataAttacherCommmand := GetDataAttacherCommand(namespace, targetName)
	dataMoverCommmand := fmt.Sprintf("/opt/tvk/datamover --action=%s --namespace=%s --restore-name=%s"+
		" --target-name=%s --app-component=%s --component-identifier=%s --pvc-name=%s --volume-path=%s",
		internal.RestoreDataAction, namespace, restoreName, targetName, appDs.AppComponent, appDs.ComponentIdentifier,
		pvc.Name, volumePath)
	command := fmt.Sprintf("%s %s && %s", blockDeviceDetect, dataAttacherCommmand, dataMoverCommmand)

	return getDataMoverContainer(command, &volumeMode)
}

// Upload Job Container

func GetDataUploadContainer(namespace, backupName, previousBackupName, targetName string,
	pvc *corev1.PersistentVolumeClaim, dataSnapshot *helpers.ApplicationDataSnapshot) *corev1.Container {
	var (
		volumeMode corev1.PersistentVolumeMode
	)

	if pvc.Spec.VolumeMode == nil {
		volumeMode = corev1.PersistentVolumeFilesystem
	} else {
		volumeMode = *pvc.Spec.VolumeMode
	}
	volumePath := internal.MountPath
	blockDeviceDetect := ""
	if volumeMode == corev1.PersistentVolumeBlock {
		volumePath = internal.PseudoBlockDevicePath
		blockDeviceDetect = DetectBlockDeviceCommand
	}
	dataAttacherCommand := GetDataAttacherCommand(namespace, targetName)
	dataMoverCommand := fmt.Sprintf("/opt/tvk/datamover --action=%s --namespace=%s --backup-name=%s --previous-backup-name=%s"+
		" --target-name=%s --app-component=%s --component-identifier=%s --pvc-name=%s --volume-path=%s",
		internal.BackupDataAction, namespace, backupName, previousBackupName, targetName, string(dataSnapshot.AppComponent),
		dataSnapshot.ComponentIdentifier, pvc.Name, volumePath)
	command := fmt.Sprintf("%s %s && %s", blockDeviceDetect, dataAttacherCommand, dataMoverCommand)

	return getDataMoverContainer(command, &volumeMode)
}

func GetChildJobs(jobList *batchv1.JobList, owner runtime.Object) []batchv1.Job {
	var children []batchv1.Job
	log := ctrl.Log.WithName("UnstructResource Utility").WithName("GetChildJobs")

	if owner == nil || len(jobList.Items) == 0 {
		return children
	}
	metaOwner, err := meta.Accessor(owner)
	if err != nil {
		log.Error(err, "Error while converting the owner to meta accessor format")
		return children
	}
	matchUID := metaOwner.GetUID()
	for itemIndex := range jobList.Items {
		item := jobList.Items[itemIndex]
		refs := item.GetOwnerReferences()
		for i := 0; i < len(refs); i++ {
			or := refs[i]
			if or.UID == matchUID {
				children = append(children, item)
			}
		}
	}

	return children
}

func GetJobsListStatus(jobList *batchv1.JobList) JobListStatus {
	status := JobListStatus{}
	for jobIndex := range jobList.Items {
		job := jobList.Items[jobIndex]
		jobStatus := GetJobStatus(&job)
		if jobStatus.Failed {
			status.Failed++
		} else if jobStatus.Completed {
			status.Completed++
		} else {
			status.Active++
		}
	}

	return status
}

func IsJobListCompleted(jobList *batchv1.JobList, status JobListStatus) bool {
	completedCount := status.Completed + status.Failed
	return len(jobList.Items) == completedCount
}

// GetTargetValidatorJob creates a job spec to validate if the target can be mounted in a pod
func GetTargetValidatorJob(target *v1.Target) *batchv1.Job {

	validationCmd := fmt.Sprintf("python %s --target-name=%s --namespace=%s", path.Join(internal.BasePath,
		internal.DatastoreValidatorUtil), target.GetName(), target.GetNamespace())

	entryCmd := fmt.Sprintf(
		"%s && %s",
		GetDataAttacherCommand(target.Namespace, target.Name),
		validationCmd,
	)

	annotations := map[string]string{internal.Operation: internal.TargetValidationOperation}

	validationContainer := GetContainer(
		"validator",
		GetDatastoreAttacherImage(),
		entryCmd,
		true,
		internal.NonDMJobResource,
		internal.MountCapability,
	)

	job := GetJob(target.Name, target.Namespace, validationContainer, []corev1.Volume{}, internal.ServiceAccountName)

	job.ObjectMeta.Annotations = annotations
	job.ObjectMeta.Labels["owner-gen-id"] = fmt.Sprintf("%d", target.GetGeneration())

	return job
}

// Datamover Functions

func GetDatamoverVolumes(pvc *corev1.PersistentVolumeClaim, isReadOnly bool) []corev1.Volume {

	var volumes []corev1.Volume

	volume := corev1.Volume{
		Name: internal.VolumeDeviceName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
				ReadOnly:  isReadOnly,
			},
		},
	}
	volumes = append(volumes, volume)

	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
		emptyDirVolume := corev1.Volume{
			Name: internal.EmptyDirVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		volumes = append(volumes, emptyDirVolume)
	}

	return volumes
}

func GetBlockDeviceInitContainer() *corev1.Container {
	containerName := internal.DatamoverContainer + "-init"
	container := GetContainer(containerName, utils.AlpineImage, CreateDeviceDetectScriptCommand, false,
		internal.NonDMJobResource, internal.DatamoverCap)

	container.VolumeDevices = []corev1.VolumeDevice{
		{
			Name:       internal.VolumeDeviceName,
			DevicePath: internal.PseudoBlockDevicePath,
		},
	}

	container.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      internal.EmptyDirVolumeName,
			MountPath: internal.EmptyDirMountPath,
		},
	}

	return container
}
