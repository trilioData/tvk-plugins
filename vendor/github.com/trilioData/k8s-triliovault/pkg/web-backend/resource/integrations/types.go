package integrations

import (
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type Type string

const (
	Velero Type = "Velero"
)

type SummaryResult map[string]int

type Summary struct {
	Total  int           `json:"total"`
	Result SummaryResult `json:"result"`
}

type Metadata struct {
	GVK       v1.GroupVersionKind `json:"gvk"`
	Name      string              `json:"name"`
	Namespace string              `json:"namespace,omitempty"`
	Type      Type                `json:"type"`
}

type BackupDetails struct {
	StartTimestamp      string `json:"startTimestamp,omitempty"`
	CompletionTimestamp string `json:"completionTimestamp,omitempty"`
	ExpirationTimestamp string `json:"expirationTimestamp,omitempty"`
	Status              string `json:"status,omitempty"`
	Type                string `json:"type,omitempty"`
}

type Backup struct {
	Metadata Metadata      `json:"metadata"`
	Details  BackupDetails `json:"details"`
}

type BackupList struct {
	Metadata common.ListMetadata `json:"metadata"`
	Summary  Summary             `json:"summary"`
	Results  []Backup            `json:"results"`
}

type RestoreDetails struct {
	RestoreTimestamp string               `json:"restoreTimestamp,omitempty"`
	Status           string               `json:"status,omitempty"`
	Backup           types.NamespacedName `json:"backup,omitempty"`
}

type Restore struct {
	Metadata Metadata       `json:"metadata"`
	Details  RestoreDetails `json:"details"`
}

type RestoreList struct {
	Metadata common.ListMetadata `json:"metadata"`
	Summary  Summary             `json:"summary"`
	Results  []Restore           `json:"results"`
}

type TargetDetails struct {
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
	Status            string `json:"status,omitempty"`
	Type              string `json:"type,omitempty"`
	Vendor            string `json:"vendor,omitempty"`
}

type Target struct {
	Metadata Metadata      `json:"metadata"`
	Details  TargetDetails `json:"details"`
}

type TargetList struct {
	Metadata common.ListMetadata `json:"metadata"`
	Summary  Summary             `json:"summary"`
	Results  []Target            `json:"results"`
}

func (backList BackupList) paginate(paginator *common.Paginator) (backupList BackupList, err error) {
	log := ctrl.Log.WithName("function").WithName("BackupList:paginate")

	if err = paginator.Set(len(backList.Results)); err != nil {
		log.Error(err, "failed to calculate paginator detail")
		return BackupList{}, err
	}

	backupList.Summary = backList.Summary
	backupList.Metadata = common.ListMetadata{Total: paginator.ResultLen, Next: paginator.Next}
	backupList.Results = backList.Results[paginator.From:paginator.To]

	return backupList, err
}

func (restList RestoreList) paginate(paginator *common.Paginator) (restoreList RestoreList, err error) {
	log := ctrl.Log.WithName("function").WithName("RestoreList:paginate")

	if err = paginator.Set(len(restList.Results)); err != nil {
		log.Error(err, "failed to calculate paginator detail")
		return RestoreList{}, err
	}

	restoreList.Results = restList.Results[paginator.From:paginator.To]
	restoreList.Summary = restList.Summary
	restoreList.Metadata = common.ListMetadata{Total: paginator.ResultLen, Next: paginator.Next}

	return restoreList, nil
}

func (targList TargetList) paginate(paginator *common.Paginator) (targetList TargetList, err error) {
	log := ctrl.Log.WithName("function").WithName("TargetList:paginate")

	if err = paginator.Set(len(targList.Results)); err != nil {
		log.Error(err, "failed to calculate paginator detail")
		return TargetList{}, err
	}

	targetList.Metadata = common.ListMetadata{Total: paginator.ResultLen, Next: paginator.Next}
	targetList.Summary = targList.Summary
	targetList.Results = targList.Results[paginator.From:paginator.To]

	return targetList, nil
}
