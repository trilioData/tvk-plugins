package helpers

import (
	"context"
	"path"
	"sort"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetBackupPath(backup *v1.Backup) string {
	applicationID := string(backup.Spec.BackupPlan.UID)
	backupID := string(backup.UID)

	targetPath := path.Join(internal.DefaultDatastoreBase, applicationID, backupID)
	return targetPath
}

// GetBackupDataSnapshots returns DataSnapshots of a backup snapshot
func GetBackupDataSnapshots(backupSnapshot *v1.Snapshot) (backupDataSnapshots []v1.DataSnapshot) {

	if backupSnapshot == nil {
		return []v1.DataSnapshot{}
	}

	// Append custom data components
	if backupSnapshot.Custom != nil {
		backupDataSnapshots = append(backupDataSnapshots, backupSnapshot.Custom.DataSnapshots...)
	}

	// Append helm data components
	for i := 0; i < len(backupSnapshot.HelmCharts); i++ {
		helmApplication := backupSnapshot.HelmCharts[i]
		backupDataSnapshots = append(backupDataSnapshots, helmApplication.DataSnapshots...)
	}

	// Append operator data components
	for i := 0; i < len(backupSnapshot.Operators); i++ {
		operatorApplication := backupSnapshot.Operators[i]
		backupDataSnapshots = append(backupDataSnapshots, operatorApplication.DataSnapshots...)
		if operatorApplication.Helm != nil {
			operatorHelm := operatorApplication.Helm
			backupDataSnapshots = append(backupDataSnapshots, operatorHelm.DataSnapshots...)
		}
	}

	return backupDataSnapshots
}

func GetBackupType(b *v1.Backup) v1.BackupType {
	var backupType v1.BackupType
	// backupType can be returned empty when the backup only has single PVC.
	backupType = b.Spec.Type

	dataSnapshots := GetBackupDataSnapshots(b.Status.Snapshot)

	if len(dataSnapshots) > 0 {
		backupType = dataSnapshots[0].BackupType
		for i := range dataSnapshots {
			if backupType != dataSnapshots[i].BackupType {
				backupType = v1.Mixed
			}
		}
	}

	return backupType
}

func GetBackupListForBplan(cl client.Client, appName, namespaceName string) (backupList []v1.Backup, err error) {
	var (
		backupObjList v1.BackupList
	)

	// Get the available backup list
	if listErr := cl.List(context.Background(), &backupObjList, &client.ListOptions{
		Namespace: namespaceName}); listErr != nil {
		return []v1.Backup{}, listErr
	}

	// Filter the list as per application name
	for backupIndex := range backupObjList.Items {
		backupObj := backupObjList.Items[backupIndex]
		if backupObj.Spec.BackupPlan.Name == appName {
			backupList = append(backupList, backupObj)
		}
	}

	// Sort the backup list as per the creation timestamp
	sort.Slice(backupList, func(i, j int) bool {
		return backupList[j].CreationTimestamp.Before(&backupList[i].CreationTimestamp)
	})

	return backupList, nil
}
