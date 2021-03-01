package common

import (
	"context"

	v12 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
)

// function to get backupList
// nolint:dupl // This function is giving duplicate lint error for GetBackupPlanList but that is not.
func GetBackupList(ctx context.Context, apiClient client.Client) (*v1.BackupList, error) {
	log := ctrl.Log.WithName("function").WithName("common:getBackupList")

	backupList := &v1.BackupList{}
	if err := apiClient.List(ctx, backupList, internal.GetTrilioResourcesDefaultListOpts()); err != nil {
		log.Error(err, "failed to get backupList from apiServer cache")
		return nil, err
	}
	return backupList, nil
}

// function to get backupPlanList
// nolint:dupl // This function is giving duplicate lint error for GetBackupList but that is not.
func GetBackupPlanList(ctx context.Context, apiClient client.Client) (*v1.BackupPlanList, error) {
	log := ctrl.Log.WithName("function").WithName("common:getBackupPlanList")

	backupPlanList := &v1.BackupPlanList{}
	if err := apiClient.List(ctx, backupPlanList, internal.GetTrilioResourcesDefaultListOpts()); err != nil {
		log.Error(err, "failed to get backupPlanList from apiServer cache")
		return nil, err
	}
	return backupPlanList, nil
}

// function to get GetTargetList
// nolint:dupl // This function is giving duplicate lint error for GetBackupList but that is not.
func GetTargetList(ctx context.Context, apiClient client.Client) (*v1.TargetList, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetTargetList")

	targetList := &v1.TargetList{}
	if err := apiClient.List(ctx, targetList, internal.GetTrilioResourcesDefaultListOpts()); err != nil {
		log.Error(err, "failed to get backupPlanList from apiServer cache")
		return nil, err
	}
	return targetList, nil
}

// function to get GetPolicyList
// nolint:dupl // This function is giving duplicate lint error for GetBackupList but that is not.
func GetPolicyList(ctx context.Context, apiClient client.Client) (*v1.PolicyList, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetPolicyList")

	policyList := &v1.PolicyList{}
	if err := apiClient.List(ctx, policyList, internal.GetTrilioResourcesDefaultListOpts()); err != nil {
		log.Error(err, "failed to get backupPlanList from apiServer cache")
		return nil, err
	}
	return policyList, nil
}

// function to get restoreList
// nolint:dupl // This function is giving duplicate lint error for GetBackupList but that is not.
func GetRestoreList(ctx context.Context, apiClient client.Client) (*v1.RestoreList, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetRestoreList")

	restoreList := &v1.RestoreList{}
	if err := apiClient.List(ctx, restoreList, internal.GetTrilioResourcesDefaultListOpts()); err != nil {
		log.Error(err, "failed to get restoreList from apiServer cache")
		return nil, err
	}
	return restoreList, nil
}

// function to get PersistentVolumeClaims
// nolint:dupl // This function is giving duplicate lint error for GetBackupList but that is not.
func GetPvcList(ctx context.Context, apiClient client.Client) (*v12.PersistentVolumeClaimList, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetPvcList")

	pvcList := &v12.PersistentVolumeClaimList{}
	opts := &client.ListOptions{}

	// For Namespace scoped installation Trilio resources only from installation namespace retrieved.
	if internal.GetAppScope() == internal.NamespacedScope {
		client.InNamespace(internal.GetInstallNamespace()).ApplyToList(opts)
	}
	if err := apiClient.List(ctx, pvcList, opts); err != nil {
		log.Error(err, "failed to get pvcList from apiServer cache")
		return nil, err
	}
	return pvcList, nil
}

// function for getting backupPlan lists from target
func GetBackupPlanListByTargetName(ctx context.Context, cli client.Client, name string) ([]v1.BackupPlan, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetBackupPlanList")
	backupPlanList := &v1.BackupPlanList{}

	backupPlanFilter := client.MatchingFields{internal.TargetToBackupplanFieldSelector: name}
	if err := cli.List(ctx, backupPlanList, internal.GetTrilioResourcesDefaultListOpts(), backupPlanFilter); err != nil {
		log.Error(err, "failed to get backupPlanList from apiServer cache")
		return backupPlanList.Items, err
	}
	return backupPlanList.Items, nil
}

// function for getting backupplan object from name
// TO-DO: Repetitive need to create common
// nolint:dupl // This function is giving Duplicate for GetTargetByName
func GetBackupPlanByName(ctx context.Context, apiClient client.Client, name, namespace string) (*v1.BackupPlan, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetBackupPlanByName")
	backupPlan := &v1.BackupPlan{}

	key := GetObjectKey(name, namespace)

	if err := apiClient.Get(ctx, key, backupPlan); err != nil {
		log.Error(err, "failed to get BackupPlan from apiServer cache")
		return backupPlan, err
	}

	return backupPlan, nil
}

// TO-DO: Repetitive need to create common
// function for getting backup object from name
// nolint:dupl // This function is giving Duplicate for GetTargetByName
func GetBackupByName(ctx context.Context, apiClient client.Client, name, namespace string) (*v1.Backup, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetBackupByName")
	b := &v1.Backup{}

	key := GetObjectKey(name, namespace)

	if err := apiClient.Get(ctx, key, b); err != nil {
		log.Error(err, "failed to get Backup from apiServer cache")
		return b, err
	}

	return b, nil
}

// TO-DO: Repetitive need to create common
// function for getting restore object from name
// nolint:dupl // This function is giving Duplicate for GetTargetByName
func GetRestoreByName(ctx context.Context, apiClient client.Client, name, namespace string) (*v1.Restore, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetRestoreByName")
	r := &v1.Restore{}

	key := GetObjectKey(name, namespace)

	if err := apiClient.Get(ctx, key, r); err != nil {
		log.Error(err, "failed to get Restore from apiServer cache")
		return r, err
	}

	return r, nil
}

// TO-DO: Repetitive need to create common
// function for getting target object from name
// nolint:dupl // This function is giving Duplicate for GetTargetByName
func GetTargetByName(ctx context.Context, apiClient client.Client, name, namespace string) (*v1.Target, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetTargetByName")
	t := &v1.Target{}

	key := GetObjectKey(name, namespace)

	if err := apiClient.Get(ctx, key, t); err != nil {
		log.Error(err, "failed to get target from apiServer cache")
		return t, err
	}

	return t, nil
}

// TO-DO: Repetitive need to create common
// function for getting hook object from name
// nolint:dupl // This function is giving Duplicate for GetTargetByName
func GetHookByName(ctx context.Context, apiClient client.Client, name, namespace string) (*v1.Hook, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetHookByName")
	h := &v1.Hook{}

	key := GetObjectKey(name, namespace)

	if err := apiClient.Get(ctx, key, h); err != nil {
		log.Error(err, "failed to get hook from apiServer cache")
		return h, err
	}

	return h, nil
}

// TO-DO: Repetitive need to create common
// function for getting hook object from name
// nolint:dupl // This function is giving Duplicate for GetTargetByName
func GetPolicyByName(ctx context.Context, apiClient client.Client, name, namespace string) (*v1.Policy, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetPolicyByName")
	p := &v1.Policy{}

	key := GetObjectKey(name, namespace)

	if err := apiClient.Get(ctx, key, p); err != nil {
		log.Error(err, "failed to get policy from apiServer cache")
		return p, err
	}

	return p, nil
}

// function for getting Restore lists from BackupName
func GetRestoreListByBackupName(ctx context.Context, apiClient client.Client, name string) (*v1.RestoreList, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetRestoreListByBackupName")
	restoreList := &v1.RestoreList{}

	restoreFilter := client.MatchingFields{internal.BackupToRestoreFieldSelector: name}
	if err := apiClient.List(ctx, restoreList, internal.GetTrilioResourcesDefaultListOpts(), restoreFilter); err != nil {
		log.Error(err, "failed to get restoreList from apiServer cache")
		return restoreList, err
	}
	return restoreList, nil
}

// Function for Getting Map of backupPlan to BackupList
func GetBackupPlanBackupListMap(backupList *v1.BackupList, isAvailableOnly bool) map[string]map[string][]v1.Backup {
	backupPlanBackupMap := make(map[string]map[string][]v1.Backup)
	for index := range backupList.Items {
		backup := backupList.Items[index]
		backupPlanName := backup.Spec.BackupPlan.Name
		backupPlanNamespace := backup.Spec.BackupPlan.Namespace
		if _, exists := backupPlanBackupMap[backupPlanNamespace][backupPlanName]; !exists {
			if isAvailableOnly && backup.Status.Status != v1.Available {
				continue
			}
			if _, exists := backupPlanBackupMap[backupPlanNamespace]; !exists {
				backupPlanBackupMap[backupPlanNamespace] = make(map[string][]v1.Backup)
			}
			backupPlanBackupMap[backupPlanNamespace][backupPlanName] =
				append(backupPlanBackupMap[backupPlanNamespace][backupPlanName], backup)
		}
	}
	return backupPlanBackupMap
}

// Function for Getting BackupPlan Name to BackupPlan Map
func GetBackupPlanNameBackupPlanMap(list *v1.BackupPlanList) map[string]map[string]v1.BackupPlan {
	backupPlanNameBackupPlanMap := make(map[string]map[string]v1.BackupPlan)
	for idx := range list.Items {
		backupPlan := list.Items[idx]
		if _, exists := backupPlanNameBackupPlanMap[backupPlan.Namespace]; !exists {
			backupPlanNameBackupPlanMap[backupPlan.Namespace] = make(map[string]v1.BackupPlan)
		}
		backupPlanNameBackupPlanMap[backupPlan.Namespace][backupPlan.Name] = backupPlan
	}
	return backupPlanNameBackupPlanMap
}

// Function for Getting GetNamespaceBackupPlanMap BackupPlan Name to BackupPlan Map
func GetNamespaceBackupPlanMap(list *v1.BackupPlanList) map[string]map[string]v1.BackupPlan {
	backupPlanMap := make(map[string]map[string]v1.BackupPlan)
	for idx := range list.Items {
		backupPlan := list.Items[idx]
		if _, exists := backupPlanMap[backupPlan.Namespace]; !exists {
			backupPlanMap[backupPlan.Namespace] = make(map[string]v1.BackupPlan)
		}
		backupPlanMap[backupPlan.Namespace][backupPlan.Name] = backupPlan
	}
	return backupPlanMap
}

// Get PVC map with name
func GetPvcNameToPvcMap(list *v12.PersistentVolumeClaimList) map[string]v12.PersistentVolumeClaim {
	pvcMap := make(map[string]v12.PersistentVolumeClaim)
	for idx := range list.Items {
		pvc := list.Items[idx]
		pvcMap[pvc.Name] = pvc
	}
	return pvcMap
}

// Function to calculate Map for backupName to BackupPlan, so that we can not call client every time
func GetBackupToBackupPlanMap(backupList *v1.BackupList, backupPlanList *v1.BackupPlanList) (
	backupBackupPlanMap map[string]map[string]v1.BackupPlan, backupMap map[string]map[string]v1.Backup) {

	backupPlanMap := GetNamespaceBackupPlanMap(backupPlanList)
	backupBackupPlanMap = make(map[string]map[string]v1.BackupPlan)
	backupMap = make(map[string]map[string]v1.Backup)

	for idx := range backupList.Items {
		backupResource := backupList.Items[idx]
		if _, exists := backupBackupPlanMap[backupResource.Namespace]; !exists {
			backupBackupPlanMap[backupResource.Namespace] = make(map[string]v1.BackupPlan)
		}
		if _, exists := backupMap[backupResource.Namespace]; !exists {
			backupMap[backupResource.Namespace] = make(map[string]v1.Backup)
		}
		backupBackupPlanMap[backupResource.Namespace][backupResource.Name] =
			backupPlanMap[backupResource.Spec.BackupPlan.Namespace][backupResource.Spec.BackupPlan.Name]
		backupMap[backupResource.Namespace][backupResource.Name] = backupResource
	}
	return backupBackupPlanMap, backupMap
}

// GetBackupNamespaceBackupListMap returns the list of backups for a namespace
func GetBackupNamespaceBackupListMap(backupList *v1.BackupList) map[string][]v1.Backup {
	backupNamespaceBackupMap := make(map[string][]v1.Backup)
	for idx := range backupList.Items {
		backup := backupList.Items[idx]
		backupNamespace := backup.Namespace
		backupNamespaceBackupMap[backupNamespace] = append(backupNamespaceBackupMap[backupNamespace], backup)
	}

	return backupNamespaceBackupMap
}

// GetRestoreNamespaceRestoreListMap returns the list of restores for a namespace
func GetRestoreNamespaceRestoreListMap(restoreList *v1.RestoreList) map[string][]v1.Restore {
	restoreNamespaceRestoreMap := make(map[string][]v1.Restore)
	for idx := range restoreList.Items {
		restore := restoreList.Items[idx]
		restoreNamespace := restore.Spec.RestoreNamespace
		restoreNamespaceRestoreMap[restoreNamespace] = append(restoreNamespaceRestoreMap[restoreNamespace], restore)
	}

	return restoreNamespaceRestoreMap
}
