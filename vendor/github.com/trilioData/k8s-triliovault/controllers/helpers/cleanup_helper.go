package helpers

import (
	"context"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getJobPVCNameList returns list of names of attached PVC's to a job
func getJobPVCNameList(job *batchv1.Job) []string {
	var pvcNames []string

	if job == nil {
		return pvcNames
	}
	jobVolumes := job.Spec.Template.Spec.Volumes
	for volumeIndex := range jobVolumes {
		volume := jobVolumes[volumeIndex]
		if volume.PersistentVolumeClaim != nil {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}

	return pvcNames
}

// CleanupJobs deletes list of jobs passed and their attached PVCs
func CleanupJobs(ctx context.Context, logger logr.Logger, cli client.Client, jobs []batchv1.Job, deletePVC bool) int {
	log := logger.WithValues("function", "CleanupJobs")
	log.Info("Cleaning up job", "jobs", len(jobs))
	var cleanCount int
	propagationPolicy := metav1.DeletePropagationForeground
	for index := range jobs {
		job := jobs[index]
		pvcNameList := getJobPVCNameList(&job)
		// Delete jobs
		log.Info("Deleting job", "name", job.Name, "namespace", job.Namespace)
		jErr := cli.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
		if jErr != nil {
			log.Error(jErr, "Error while deleting job",
				"name", job.Name, "namespace", job.Namespace)
			utilruntime.HandleError(jErr)
		} else {
			cleanCount++
		}
		if deletePVC {
			// Delete PVCs attached to a job
			cleanupPVCs(ctx, logger, cli, pvcNameList, job.Namespace)
		}
	}

	return cleanCount
}

// cleanupPVCs deletes list of PVCs in the given namespace
func cleanupPVCs(ctx context.Context, logger logr.Logger, cli client.Client, pvcNameList []string, namespace string) {
	log := logger.WithValues("function", "cleanupPVCs")
	log.Info("Cleaning up pvc", "pvc", len(pvcNameList))
	// Delete all attached PVC's of a job
	for pvcIndex := range pvcNameList {
		pvcName := pvcNameList[pvcIndex]
		pvcKey := types.NamespacedName{
			Name:      pvcName,
			Namespace: namespace,
		}

		// Get pvc by key
		pvc := &corev1.PersistentVolumeClaim{}
		pvcErr := cli.Get(ctx, pvcKey, pvc)
		if pvcErr != nil {
			log.Error(pvcErr, "Error while getting pvc",
				"pvc", pvc.Name, "namespace", pvc.Namespace)
			utilruntime.HandleError(pvcErr)
			continue
		}

		// Update reclaim policy to delete of a pv attached so pv gets deleted with pvc
		volumeName := pvc.Spec.VolumeName
		if volumeName != "" {
			_ = updatePVReclaimPolicy(ctx, logger, cli, volumeName, corev1.PersistentVolumeReclaimDelete)
		}

		log.Info("Deleting PVC", "name", pvc.Name, "namespace", pvc.Namespace)
		// Delete PVC
		pErr := cli.Delete(ctx, pvc)
		if pErr != nil {
			log.Error(pErr, "Error while deleting pvc",
				"pvc", pvc.Name, "namespace", pvc.Namespace)
			utilruntime.HandleError(pErr)
		}
	}
}

// updatePVReclaimPolicy updates reclaim policy of a pv
func updatePVReclaimPolicy(ctx context.Context, logger logr.Logger, cli client.Client, pvName string,
	reclaimPolicy corev1.PersistentVolumeReclaimPolicy) error {

	log := logger.WithValues("function", "updatePVReclaimPolicyPolicy")

	pv := &corev1.PersistentVolume{}
	pErr := cli.Get(ctx, types.NamespacedName{Name: pvName}, pv)
	if pErr != nil {
		log.Error(pErr, "Error while getting pv", "name", pvName)
		return pErr
	}

	if pv.Spec.PersistentVolumeReclaimPolicy != reclaimPolicy {
		pv.Spec.PersistentVolumeReclaimPolicy = reclaimPolicy
		log.Info("Updating PV ReclaimPolicy", "pv", pv.Name, "reclaimPolicy", reclaimPolicy)
		puErr := cli.Update(ctx, pv)
		if puErr != nil {
			log.Error(puErr, "Error while updating pv reclaim policy", "name", pvName)
			return puErr
		}
	}

	return nil
}
