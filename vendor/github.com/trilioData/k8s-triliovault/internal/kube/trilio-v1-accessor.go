package kube

import (
	log "github.com/sirupsen/logrus"
	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *Accessor) CreateTarget(target *v1.Target) error {
	return a.client.Create(a.context, target)
}

func (a *Accessor) CreateBackup(backup *v1.Backup) error {
	return a.client.Create(a.context, backup)
}

func (a *Accessor) CreateApplication(backupPlan *v1.BackupPlan) error {
	return a.client.Create(a.context, backupPlan)
}

func (a *Accessor) CreateRetentionPolicy(policy *v1.Policy) error {
	return a.client.Create(a.context, policy)
}

func (a *Accessor) CreateHookAction(hookAction *v1.Hook) error {
	return a.client.Create(a.context, hookAction)
}

// Backup CRUD
func (a *Accessor) GetBackup(backupName, namespace string) (*v1.Backup, error) {
	backupObj := &v1.Backup{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      backupName,
	}, backupObj); err != nil {
		return nil, err
	}

	return backupObj, nil
}

func (a *Accessor) GetBackupStatus(backupName, namespace string) (v1.Status, error) {
	backup, err := a.GetBackup(backupName, namespace)
	if err != nil {
		return "", err
	}
	return backup.Status.Status, nil
}

func (a *Accessor) GetBackupPlanStatus(bpName, namespace string) (v1.Status, error) {
	backupPlan, err := a.GetBackupPlan(bpName, namespace)
	if err != nil {
		return "", err
	}
	return backupPlan.Status.Status, nil
}

func (a *Accessor) GetBackups(namespace string, opts ...client.ListOption) ([]v1.Backup, error) {
	backupList := &v1.BackupList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := a.client.List(a.context, backupList, opts...); err != nil {
		return nil, err
	}
	return backupList.Items, nil
}

func (a *Accessor) GetPolicy(name, namespace string) (*v1.Policy, error) {
	policy := &v1.Policy{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func (a *Accessor) GetLicense(name, namespace string) (*v1.License, error) {
	license := &v1.License{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, license); err != nil {
		return nil, err
	}
	return license, nil
}

func (a *Accessor) GetLicenses(opts ...client.ListOption) ([]v1.License, error) {
	licenseList := &v1.LicenseList{}
	if err := a.client.List(a.context, licenseList, opts...); err != nil {
		return nil, err
	}
	return licenseList.Items, nil
}

func (a *Accessor) GetLicenseStatus(name, namespace string) (v1.LicenseState, error) {
	license, err := a.GetLicense(name, namespace)
	if err != nil {
		return "", err
	}
	return license.Status.Status, nil
}

func (a *Accessor) UpdateBackupStatusObject(backupCR *v1.Backup) error {
	updateErr := a.client.Status().Update(a.context, backupCR)
	if updateErr != nil {
		return updateErr
	}
	log.Infof("Updated %s status to %v", backupCR.Name, backupCR.Status)

	return nil
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func UpdateRestoreStatus
func (a *Accessor) UpdateBackupStatus(backupName, namespace string,
	reqStatus v1.Status) error {
	log.Infof("Updating %s status to %s", backupName, reqStatus)
	var backupCR v1.Backup
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      backupName,
	}, &backupCR); err != nil {
		return err
	}
	backupCR.Status.Status = reqStatus
	updateErr := a.client.Status().Update(a.context, &backupCR)
	if updateErr != nil {
		return updateErr
	}
	log.Infof("Updated %s status to %s", backupName, reqStatus)

	return nil
}

func (a *Accessor) UpdateExpiration(expiry *metav1.Time, backupName, namespace string) error {
	backupObj, err := a.GetBackup(backupName, namespace)
	if err != nil {
		return err
	}

	backupObj.Status.ExpirationTimestamp = expiry
	err = a.UpdateBackupStatusObject(backupObj)
	if err != nil {
		return err
	}

	return nil
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func UpdateBackupStatus
func (a *Accessor) UpdateRestoreStatus(restoreName, namespace string,
	reqStatus v1.Status) error {
	log.Infof("Updating %s status to %s", restoreName, reqStatus)
	var restore v1.Restore
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      restoreName,
	}, &restore); err != nil {
		return err
	}
	restore.Status.Status = reqStatus
	updateErr := a.client.Status().Update(a.context, &restore)
	if updateErr != nil {
		return updateErr
	}
	log.Infof("Updated %s status to %s", restoreName, reqStatus)

	return nil
}

//nolint:dupl // added to get rid of lint errors of duplicate code of func UpdateBackupStatus
func (a *Accessor) UpdateBackupPlanStatus(backupPlanName, namespace string,
	reqStatus v1.Status) error {
	log.Infof("Updating %s status to %s", backupPlanName, reqStatus)
	var backupPlan v1.BackupPlan
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      backupPlanName,
	}, &backupPlan); err != nil {
		return err
	}
	backupPlan.Status.Status = reqStatus
	updateErr := a.client.Status().Update(a.context, &backupPlan)
	if updateErr != nil {
		return updateErr
	}
	log.Infof("Updated %s status to %s", backupPlanName, reqStatus)

	return nil
}

// Restore CRUD
func (a *Accessor) CreateRestore(restoreCR *v1.Restore) error {
	return a.client.Create(a.context, restoreCR)
}

func (a *Accessor) DeleteRestore(nsName types.NamespacedName) error {
	restore := &v1.Restore{}
	restore.SetName(nsName.Name)
	restore.SetNamespace(nsName.Namespace)
	err := a.client.Delete(a.context, restore)
	return err
}

func (a *Accessor) GetRestore(restoreName, namespace string) (*v1.Restore, error) {
	restoreObj := &v1.Restore{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      restoreName,
	}, restoreObj); err != nil {
		return nil, err
	}

	return restoreObj, nil
}

func (a *Accessor) GetRestoreStatus(restoreName, namespace string) (v1.Status, error) {
	restore, err := a.GetRestore(restoreName, namespace)
	if err != nil {
		return "", err
	}
	return restore.Status.Status, nil
}

func (a *Accessor) GetRestores(namespace string, opts ...client.ListOption) ([]v1.Restore, error) {
	restoreList := &v1.RestoreList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := a.client.List(a.context, restoreList, opts...); err != nil {
		return nil, err
	}
	return restoreList.Items, nil
}

// Target CRUD
func (a *Accessor) GetTarget(targetName, namespace string) (*v1.Target, error) {
	targetObj := &v1.Target{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      targetName,
	}, targetObj); err != nil {
		return nil, err
	}

	return targetObj, nil
}

func (a *Accessor) GetTargets(namespace string, opts ...client.ListOption) ([]v1.Target, error) {
	targetList := &v1.TargetList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := a.client.List(a.context, targetList, opts...); err != nil {
		return nil, err
	}
	return targetList.Items, nil
}

func (a *Accessor) GetBackupPlan(backupPlanName, namespace string) (*v1.BackupPlan, error) {
	appObj := &v1.BackupPlan{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      backupPlanName,
	}, appObj); err != nil {
		return nil, err
	}

	return appObj, nil
}

func (a *Accessor) GetTargetStatus(targetName, namespace string) (v1.Status, error) {
	target, err := a.GetTarget(targetName, namespace)
	if err != nil {
		return "", err
	}
	return target.Status.Status, nil
}

func (a *Accessor) GetBackupPlans(namespace string, opts ...client.ListOption) ([]v1.BackupPlan, error) {
	bckPlanList := &v1.BackupPlanList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := a.client.List(a.context, bckPlanList, opts...); err != nil {
		return nil, err
	}
	return bckPlanList.Items, nil
}

func (a *Accessor) GetPolicies(namespace string, opts ...client.ListOption) ([]v1.Policy, error) {
	policyList := &v1.PolicyList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := a.client.List(a.context, policyList, opts...); err != nil {
		return nil, err
	}
	return policyList.Items, nil
}

func (a *Accessor) GetHook(hookName, namespace string) (*v1.Hook, error) {
	hookAction := &v1.Hook{}
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      hookName,
	}, hookAction); err != nil {
		return nil, err
	}
	return hookAction, nil
}

func (a *Accessor) DeleteTarget(key types.NamespacedName) error {
	target := &v1.Target{}
	target.SetName(key.Name)
	target.SetNamespace(key.Namespace)

	return a.client.Delete(a.context, target)
}

func (a *Accessor) DeleteBackup(key types.NamespacedName) error {
	backup := &v1.Backup{}
	backup.SetName(key.Name)
	backup.SetNamespace(key.Namespace)
	return a.client.Delete(a.context, backup)
}

func (a *Accessor) DeleteBackupPlan(key types.NamespacedName) error {
	backupPlan := &v1.BackupPlan{}
	backupPlan.SetName(key.Name)
	backupPlan.SetNamespace(key.Namespace)
	return a.client.Delete(a.context, backupPlan)
}

func (a *Accessor) DeleteRetentionPolicy(key types.NamespacedName) error {
	policy := &v1.Policy{}
	policy.SetName(key.Name)
	policy.SetNamespace(key.Namespace)
	return a.client.Delete(a.context, policy)
}

func (a *Accessor) DeleteApplication(key types.NamespacedName) error {
	application := &v1.BackupPlan{}
	application.SetName(key.Name)
	application.SetNamespace(key.Namespace)
	return a.client.Delete(a.context, application)
}

func (a *Accessor) DeleteHookAction(key types.NamespacedName) error {
	hookAction := &v1.Hook{}
	hookAction.SetName(key.Name)
	hookAction.SetNamespace(key.Namespace)
	return a.client.Delete(a.context, hookAction)
}

func (a *Accessor) UpdateRestore(namespace string, restore *v1.Restore) error {
	err := a.client.Update(a.context, restore)
	return err
}

func (a *Accessor) UpdateTarget(target *v1.Target) error {
	return a.client.Update(a.context, target)
}

func (a *Accessor) UpdateBackup(backup *v1.Backup) error {
	return a.client.Update(a.context, backup)
}

func (a *Accessor) UpdateRetentionPolicy(policy *v1.Policy) error {
	return a.client.Update(a.context, policy)
}

func (a *Accessor) UpdateBackupPlan(backupPlan *v1.BackupPlan) error {
	return a.client.Update(a.context, backupPlan)
}

func (a *Accessor) StatusJSONPatch(object client.Object, payloadBytes []byte) error {
	return a.client.Status().Patch(a.context, object, client.RawPatch(types.JSONPatchType, payloadBytes))
}

func (a *Accessor) StatusMergePatch(object, newObject client.Object) error {
	return a.client.Status().Patch(a.context, object, client.MergeFrom(newObject))
}

func (a *Accessor) StatusUpdate(object client.Object) error {
	return a.client.Status().Update(a.context, object)
}

func (a *Accessor) ForceDelete(object client.Object) error {
	patch := []byte(`{"metadata": {"finalizers": null}}`)
	return a.client.Patch(a.context, object, client.RawPatch(types.MergePatchType, patch))
}
