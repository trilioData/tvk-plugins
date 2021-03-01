package backup

import (
	"context"

	"github.com/trilioData/k8s-triliovault/internal/helpers"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type DetailHelper struct {
	ApplicationType   v1.ApplicationType
	BackupPlan        *v1.BackupPlan
	Target            *v1.Target
	InProgressResotre *v1.Restore
}

func getDetailHelper(ctx context.Context, cli client.Client, backup *Backup, backupPlan *v1.BackupPlan) (*DetailHelper, error) {
	var applicationType v1.ApplicationType
	var inProgressRestore *v1.Restore
	var target *v1.Target
	var err error

	// Retrieving Target for Backup
	target, err = common.GetTargetByName(ctx, cli, backupPlan.Spec.BackupConfig.Target.Name, backupPlan.Spec.BackupConfig.Target.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Retrieving ApplicationType for backup
	applicationType = helpers.GetApplicationType(&backupPlan.Spec)

	// Retrieving InProgressRestore
	restoreList, err := common.GetRestoreListByBackupName(ctx, cli, backup.Name)
	if err != nil {
		return nil, err
	}
	for idx := range restoreList.Items {
		if restoreList.Items[idx].Status.Status == v1.InProgress {
			inProgressRestore = &restoreList.Items[idx]
			break
		}
	}

	// DetailHelper
	return &DetailHelper{
		BackupPlan:        backupPlan,
		Target:            target,
		ApplicationType:   applicationType,
		InProgressResotre: inProgressRestore,
	}, nil
}
