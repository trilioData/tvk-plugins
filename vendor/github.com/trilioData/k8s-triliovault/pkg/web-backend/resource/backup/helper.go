package backup

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type DetailHelper struct {
	ApplicationType v1.ApplicationType
	BackupPlan      *v1.BackupPlan
	Target          *v1.Target
}

func getDetailHelper(ctx context.Context, cli client.Client, backup *Backup, backupPlan *v1.BackupPlan) (*DetailHelper, error) {
	var (
		applicationType v1.ApplicationType
		target          *v1.Target
		err             error
	)

	// Retrieving Target for Backup
	target, err = common.GetTargetByName(ctx, cli, backupPlan.Spec.BackupConfig.Target.Name, backupPlan.Spec.BackupConfig.Target.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Retrieving ApplicationType and InProgressRestore
	if backup.Status.Stats != nil {
		if backup.Status.Stats.ApplicationType != nil {
			applicationType = *backup.Status.Stats.ApplicationType
		} else {
			applicationType = v1.NamespaceType
		}
	}

	// DetailHelper
	return &DetailHelper{
		BackupPlan:      backupPlan,
		Target:          target,
		ApplicationType: applicationType,
	}, nil
}
