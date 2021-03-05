package backup

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
)

// function for copying data into custom struct from api struct
func CopyDataFrom(backup *v1.Backup) *Backup {
	return &Backup{
		ObjectMeta: backup.ObjectMeta,
		TypeMeta:   backup.TypeMeta,
		Spec:       backup.Spec,
		Status:     backup.Status,
	}
}

// Function to populate Summary Struct for Backup
func (summary *Summary) UpdateSummary(status v1.Status) {
	switch status {
	case v1.Failed:
		summary.Result.Failed++
	case v1.Available:
		summary.Result.Available++
	}

	summary.Total++
	summary.Result.InProgress = summary.Total - (summary.Result.Failed + summary.Result.Available)
}

// function for getting backup object from name
func GetBackupByName(ctx context.Context, apiClient client.Client, name, namespace string) (*Backup, error) {
	log := ctrl.Log.WithName("function").WithName("backup:GetBackupByName")
	b := &v1.Backup{}

	key := types.NamespacedName{Name: name, Namespace: namespace}

	if err := apiClient.Get(ctx, key, b); err != nil {
		log.Error(err, "failed to get backup from apiServer cache")
		return CopyDataFrom(b), err
	}

	// Moving data from v1.Backup data to Backup struct
	return CopyDataFrom(b), nil
}

// function to get backupList
func GetBackupList(ctx context.Context, apiClient client.Client) (*v1.BackupList, error) {
	log := ctrl.Log.WithName("function").WithName("backup:getBackupList")

	backupList := &v1.BackupList{}
	if err := apiClient.List(ctx, backupList, internal.GetTrilioResourcesDefaultListOpts()); err != nil {
		log.Error(err, "failed to get backupList from apiServer cache")
		return nil, err
	}
	return backupList, nil
}
