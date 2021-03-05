package integrations

import (
	"context"
	"errors"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	clientapi "github.com/trilioData/k8s-triliovault/pkg/web-backend/client/api"
	backup2 "github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/backup"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type VeleroKind string

const (

	// Velero Group
	VeleroGroup string = "velero.io"

	// Backup
	VeleroBackupKind VeleroKind = "Backup"

	// Restore
	VeleroRestoreKind VeleroKind = "Restore"

	// Target
	VeleroBackupStorageLocationKind  VeleroKind = "BackupStorageLocation"
	VeleroVolumeSnapshotLocationKind VeleroKind = "VolumeSnapshotLocation"
)

var (
	convertStatusToCompleted   = sets.String{}.Insert("Completed")
	convertStatusToInProgress  = sets.String{}.Insert("New", "InProgress", "Deleting")
	convertStatusToFailed      = sets.String{}.Insert("FailedValidation", "PartiallyFailed", "Failed")
	convertStatusToAvailable   = sets.String{}.Insert("Available")
	convertStatusToUnavailable = sets.String{}.Insert("Unavailable")
)

type VeleroIntegration struct {
	authClient   client.Client
	groupVersion schema.GroupVersion
	kinds        sets.String
}

func (v VeleroIntegration) BackupList(listRequestParams *ListRequestParams) (BackupList, error) {

	ctrl.Log.WithName("function").WithName("integrationBackup:getIntegrationBackupList")

	var integrationBackupList BackupList
	var index int

	if !v.kinds.Has(string(VeleroBackupKind)) {
		err := errors.New("kind " + string(VeleroBackupKind) + " not present on this cluster")
		log.Warning(err)
		return BackupList{}, err
	}

	veleroBackupList := unstructured.UnstructuredList{}
	veleroBackupList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v.groupVersion.Group,
		Version: v.groupVersion.Version,
		Kind:    string(VeleroBackupKind),
	})

	listOps := &client.ListOptions{}
	if listRequestParams.Namespace != nil {
		listOps = &client.ListOptions{Namespace: *listRequestParams.Namespace}
	}
	if err := v.authClient.List(context.Background(), &veleroBackupList,
		listOps); err != nil {
		log.Error(err, "failed to get integrationRestoreList from apiServer cache")
		return BackupList{}, err
	}

	integrationBackupList.Summary.Total = len(veleroBackupList.Items)
	integrationBackupList.Summary.Result = map[string]int{}

	for _, backup := range veleroBackupList.Items {

		// filter by Time Range Requested
		if !listRequestParams.TimeRangeFilter.IsEmpty() && !common.IsTimestampInRange(backup.GetCreationTimestamp(),
			*listRequestParams.TimeRangeFilter) {
			continue
		}

		status := GetStringFromUnstructured(backup.Object, "status", "phase")
		status = covertToTrilioStatus(status, internal.BackupKind)

		integrationBackupList.Summary.Result[status]++

		// filter by status
		if listRequestParams.Status != nil && status != *listRequestParams.Status {
			continue
		}

		integrationBackupList.Results = append(integrationBackupList.Results, Backup{})

		integrationBackupList.Results[index].Metadata = getResourceMetadata(v, backup, string(VeleroBackupKind))

		// PolicyBased or onDemand
		if GetStringFromUnstructured(backup.Object, "metadata", "labels", "velero.io/schedule-name") != "" {
			integrationBackupList.Results[index].Details.Type = string(backup2.PolicyBased)
		} else {
			integrationBackupList.Results[index].Details.Type = string(backup2.OnDemand)
		}

		integrationBackupList.Results[index].Details.StartTimestamp = GetStringFromUnstructured(backup.Object, "status", "startTimestamp")
		integrationBackupList.Results[index].Details.ExpirationTimestamp = GetStringFromUnstructured(backup.Object, "status", "expiration")
		if GetStringFromUnstructured(backup.Object, "status", "completionTimestamp") != "" {
			integrationBackupList.Results[index].Details.CompletionTimestamp = GetStringFromUnstructured(backup.Object,
				"status", "completionTimestamp")
		}
		integrationBackupList.Results[index].Details.Status = status

		index++
	}

	// nolint:dupl // added to get rid of lint errors of duplicate code
	if listRequestParams.OrderingField != nil {
		switch listRequestParams.OrderingField.Field {
		case string(ExpirationTimestamp):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Details.ExpirationTimestamp < integrationBackupList.Results[j].Details.ExpirationTimestamp
				})
			} else {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Details.ExpirationTimestamp > integrationBackupList.Results[j].Details.ExpirationTimestamp
				})
			}
		case string(CreationTimestamp):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Details.StartTimestamp < integrationBackupList.Results[j].Details.StartTimestamp
				})
			} else {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Details.StartTimestamp > integrationBackupList.Results[j].Details.StartTimestamp
				})
			}
		case string(Status):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Details.Status < integrationBackupList.Results[j].Details.Status
				})
			} else {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Details.Status > integrationBackupList.Results[j].Details.Status
				})
			}
		case string(Name):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Metadata.Name < integrationBackupList.Results[j].Metadata.Name
				})
			} else {
				sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
					return integrationBackupList.Results[i].Metadata.Name > integrationBackupList.Results[j].Metadata.Name
				})
			}
		default:
			sort.SliceStable(integrationBackupList.Results, func(i, j int) bool {
				return integrationBackupList.Results[i].Metadata.Name < integrationBackupList.Results[j].Metadata.Name
			})
		}
	}

	return integrationBackupList, nil
}

func (v VeleroIntegration) RestoreList(listRequestParams *ListRequestParams) (RestoreList, error) {
	ctrl.Log.WithName("function").WithName("integrationBackup:getIntegrationRestoreList")

	var integrationRestoreList RestoreList
	var index int

	if !v.kinds.Has(string(VeleroRestoreKind)) {
		err := errors.New("kind " + string(VeleroRestoreKind) + " not present on this cluster")
		log.Warning(err)
		return RestoreList{}, err
	}

	veleroRestoreList := unstructured.UnstructuredList{}
	veleroRestoreList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v.groupVersion.Group,
		Version: v.groupVersion.Version,
		Kind:    string(VeleroRestoreKind),
	})

	listOps := &client.ListOptions{}
	if listRequestParams.Namespace != nil {
		listOps = &client.ListOptions{Namespace: *listRequestParams.Namespace}
	}
	if err := v.authClient.List(context.Background(), &veleroRestoreList,
		listOps); err != nil {
		log.Error(err, "failed to get integrationRestoreList from apiServer cache")
		return RestoreList{}, err
	}

	integrationRestoreList.Summary.Total = len(veleroRestoreList.Items)
	integrationRestoreList.Summary.Result = map[string]int{}

	for _, restore := range veleroRestoreList.Items {

		// filter by Time Range Requested
		if !listRequestParams.TimeRangeFilter.IsEmpty() && !common.IsTimestampInRange(restore.GetCreationTimestamp(),
			*listRequestParams.TimeRangeFilter) {
			continue
		}

		status := GetStringFromUnstructured(restore.Object, "status", "phase")
		status = covertToTrilioStatus(status, internal.RestoreKind)

		integrationRestoreList.Summary.Result[status]++

		// filter by status
		if listRequestParams.Status != nil && status != *listRequestParams.Status {
			continue
		}

		integrationRestoreList.Results = append(integrationRestoreList.Results, Restore{})

		integrationRestoreList.Results[index].Metadata = getResourceMetadata(v, restore, string(VeleroRestoreKind))

		integrationRestoreList.Results[index].Details.RestoreTimestamp = GetStringFromUnstructured(restore.Object, "status", "startTimestamp")

		integrationRestoreList.Results[index].Details.Backup = types.NamespacedName{
			Namespace: GetStringFromUnstructured(restore.Object, "metadata", "namespace"),
			Name:      GetStringFromUnstructured(restore.Object, "spec", "backupName"),
		}

		integrationRestoreList.Results[index].Details.Status = status

		index++

	}

	// nolint:dupl // added to get rid of lint errors of duplicate code
	if listRequestParams.OrderingField != nil {
		switch listRequestParams.OrderingField.Field {
		case string(RestoreTimestamp):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Details.RestoreTimestamp < integrationRestoreList.Results[j].Details.RestoreTimestamp
				})
			} else {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Details.RestoreTimestamp > integrationRestoreList.Results[j].Details.RestoreTimestamp
				})
			}
		case string(CreationTimestamp):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Details.RestoreTimestamp < integrationRestoreList.Results[j].Details.RestoreTimestamp
				})
			} else {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Details.RestoreTimestamp > integrationRestoreList.Results[j].Details.RestoreTimestamp
				})
			}
		case string(Status):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Details.Status < integrationRestoreList.Results[j].Details.Status
				})
			} else {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Details.Status > integrationRestoreList.Results[j].Details.Status
				})
			}
		case string(Name):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Metadata.Name < integrationRestoreList.Results[j].Metadata.Name
				})
			} else {
				sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
					return integrationRestoreList.Results[i].Metadata.Name > integrationRestoreList.Results[j].Metadata.Name
				})
			}
		default:
			sort.SliceStable(integrationRestoreList.Results, func(i, j int) bool {
				return integrationRestoreList.Results[i].Metadata.Name < integrationRestoreList.Results[j].Metadata.Name
			})
		}
	}

	return integrationRestoreList, nil
}

func (v VeleroIntegration) TargetList(listRequestParams *ListRequestParams) (TargetList, error) {
	ctrl.Log.WithName("function").WithName("integrationTarget:getIntegrationTargetList")

	var index int

	if !v.kinds.Has(string(VeleroBackupStorageLocationKind)) {
		err := errors.New("kind " + string(VeleroBackupStorageLocationKind) + " not present on this cluster")
		log.Warning(err)
		return TargetList{}, err
	} else if !v.kinds.Has(string(VeleroVolumeSnapshotLocationKind)) {
		err := errors.New("kind " + string(VeleroVolumeSnapshotLocationKind) + " not present on this cluster")
		log.Warning(err)
		return TargetList{}, err
	}

	integrationTargetList, err := veleroBackupLocationList(&index, v, &TargetList{}, listRequestParams)
	if err != nil {
		log.Error(err, "error while getting backupStorageLocation list")
		return TargetList{}, err
	}

	integrationTargetList, err = veleroSnapshotLocationList(&index, v, integrationTargetList, listRequestParams)
	if err != nil {
		log.Error(err, "error while getting backupSnapshotLocation list")
		return TargetList{}, err
	}

	// nolint:dupl // added to get rid of lint errors of duplicate code
	if listRequestParams.OrderingField != nil {
		switch listRequestParams.OrderingField.Field {
		case string(ProviderName):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.Vendor < integrationTargetList.Results[j].Details.Vendor
				})
			} else {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.Vendor > integrationTargetList.Results[j].Details.Vendor
				})
			}
		case string(StorageType):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.Type < integrationTargetList.Results[j].Details.Type
				})
			} else {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.Type > integrationTargetList.Results[j].Details.Type
				})
			}
		case string(CreationTimestamp):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.CreationTimestamp < integrationTargetList.Results[j].Details.CreationTimestamp
				})
			} else {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.CreationTimestamp > integrationTargetList.Results[j].Details.CreationTimestamp
				})
			}
		case string(Status):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.Status < integrationTargetList.Results[j].Details.Status
				})
			} else {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Details.Status > integrationTargetList.Results[j].Details.Status
				})
			}
		case string(Name):
			if listRequestParams.OrderingField.Ascending {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Metadata.Name < integrationTargetList.Results[j].Metadata.Name
				})
			} else {
				sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
					return integrationTargetList.Results[i].Metadata.Name > integrationTargetList.Results[j].Metadata.Name
				})
			}
		default:
			sort.SliceStable(integrationTargetList.Results, func(i, j int) bool {
				return integrationTargetList.Results[i].Metadata.Name < integrationTargetList.Results[j].Metadata.Name
			})
		}
	}

	return *integrationTargetList, nil

}

func veleroBackupLocationList(index *int, veleroIntegration VeleroIntegration, targetList *TargetList,
	listRequestParams *ListRequestParams) (*TargetList, error) {

	var veleroBackupLocList unstructured.UnstructuredList

	veleroBackupLocList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   veleroIntegration.groupVersion.Group,
		Version: veleroIntegration.groupVersion.Version,
		Kind:    string(VeleroBackupStorageLocationKind),
	})

	listOps := &client.ListOptions{}
	if listRequestParams.Namespace != nil {
		listOps = &client.ListOptions{Namespace: *listRequestParams.Namespace}
	}
	if err := veleroIntegration.authClient.List(context.Background(), &veleroBackupLocList,
		listOps); err != nil {
		log.Error(err, "failed to get integrationRestoreList from apiServer cache")
		return targetList, err
	}

	targetList.Summary.Total = len(veleroBackupLocList.Items)
	targetList.Summary.Result = map[string]int{}

	for _, location := range veleroBackupLocList.Items {

		// filter by Time Range Requested
		if !listRequestParams.TimeRangeFilter.IsEmpty() && !common.IsTimestampInRange(location.GetCreationTimestamp(),
			*listRequestParams.TimeRangeFilter) {
			continue
		}

		status := GetStringFromUnstructured(location.Object, "status", "phase")
		status = covertToTrilioStatus(status, internal.TargetKind)

		// filter by status
		if listRequestParams.Status != nil && status != *listRequestParams.Status {
			continue
		}

		targetList.Summary.Result[status]++

		targetList.Results = append(targetList.Results, Target{})

		targetList.Results[*index].Metadata = getResourceMetadata(veleroIntegration, location, string(VeleroBackupStorageLocationKind))

		targetList.Results[*index].Details.Status = status
		targetList.Results[*index].Details.CreationTimestamp = GetStringFromUnstructured(location.Object, "metadata", "creationTimestamp")
		targetList.Results[*index].Details.Type = string(VeleroBackupStorageLocationKind)
		targetList.Results[*index].Details.Vendor = GetStringFromUnstructured(location.Object, "spec", "provider")

		*index++
	}

	return targetList, nil
}

func veleroSnapshotLocationList(index *int, veleroIntegration VeleroIntegration, targetList *TargetList,
	listRequestParams *ListRequestParams) (*TargetList, error) {

	var veleroSnapshotLocList unstructured.UnstructuredList

	veleroSnapshotLocList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   veleroIntegration.groupVersion.Group,
		Version: veleroIntegration.groupVersion.Version,
		Kind:    string(VeleroVolumeSnapshotLocationKind),
	})

	listOps := &client.ListOptions{}
	if listRequestParams.Namespace != nil {
		listOps = &client.ListOptions{Namespace: *listRequestParams.Namespace}
	}
	if err := veleroIntegration.authClient.List(context.Background(), &veleroSnapshotLocList,
		listOps); err != nil {
		log.Error(err, "failed to get integrationRestoreList from apiServer cache")
		return targetList, err
	}

	targetList.Summary.Total += len(veleroSnapshotLocList.Items)
	if targetList.Summary.Result == nil {
		targetList.Summary.Result = map[string]int{}
	}

	for _, location := range veleroSnapshotLocList.Items {

		// filter by Time Range Requested
		if !listRequestParams.TimeRangeFilter.IsEmpty() && !common.IsTimestampInRange(location.GetCreationTimestamp(),
			*listRequestParams.TimeRangeFilter) {
			continue
		}

		status := GetStringFromUnstructured(location.Object, "status", "phase")
		if status == "" {
			status = string(v1.Unavailable)
		}
		status = covertToTrilioStatus(status, internal.TargetKind)

		// filter by status
		if listRequestParams.Status != nil && status != *listRequestParams.Status {
			continue
		}

		targetList.Summary.Result[status]++

		targetList.Results = append(targetList.Results, Target{})

		targetList.Results[*index].Metadata = getResourceMetadata(veleroIntegration, location, string(VeleroVolumeSnapshotLocationKind))

		targetList.Results[*index].Details.Status = status
		targetList.Results[*index].Details.CreationTimestamp = GetStringFromUnstructured(location.Object, "metadata", "creationTimestamp")
		targetList.Results[*index].Details.Type = string(VeleroVolumeSnapshotLocationKind)
		targetList.Results[*index].Details.Vendor = GetStringFromUnstructured(location.Object, "spec", "provider")

		*index++
	}

	return targetList, nil
}

// covertToTrilioStatus convert velero status to trilio status (for making filtering easy)
func covertToTrilioStatus(veleroStatus, kind string) (trilioStatus string) {

	switch kind {
	case internal.BackupKind:
		if convertStatusToCompleted.Has(veleroStatus) {
			trilioStatus = string(v1.Available)
		} else if convertStatusToInProgress.Has(veleroStatus) {
			trilioStatus = string(v1.InProgress)
		} else if convertStatusToFailed.Has(veleroStatus) {
			trilioStatus = string(v1.Failed)
		}
	case internal.RestoreKind:
		if convertStatusToCompleted.Has(veleroStatus) {
			trilioStatus = string(v1.Completed)
		} else if convertStatusToInProgress.Has(veleroStatus) {
			trilioStatus = string(v1.InProgress)
		} else if convertStatusToFailed.Has(veleroStatus) {
			trilioStatus = string(v1.Failed)
		}
	case internal.TargetKind:
		if convertStatusToAvailable.Has(veleroStatus) {
			trilioStatus = string(v1.Available)
		} else if convertStatusToUnavailable.Has(veleroStatus) {
			trilioStatus = string(v1.Unavailable)
		}
	}
	return
}

func getResourceMetadata(v VeleroIntegration, obj unstructured.Unstructured, kind string) (meta Metadata) {
	meta.GVK.Group = v.groupVersion.Group
	meta.GVK.Version = v.groupVersion.Version
	meta.GVK.Kind = kind
	meta.Name = GetStringFromUnstructured(obj.Object, "metadata", "name")
	meta.Namespace = GetStringFromUnstructured(obj.Object, "metadata", "namespace")
	meta.Type = Velero
	return
}

func NewVeleroIntegration(clientManager clientapi.ClientManager, authClient client.Client) (Other, error) {
	cachedDiscoveryClient := clientManager.CachedDiscoveryClient()
	allServerResources, err := cachedDiscoveryClient.ServerPreferredResources()
	if err != nil {
		return &VeleroIntegration{}, err
	}

	present := false

	// check if velero group present in resources returned by server
	var preferredVersion string
	kinds := sets.String{}
	for i := range allServerResources {
		resource := allServerResources[i]

		// check if current resource is of core GV, if true skip
		if !strings.Contains(resource.GroupVersion, "/") {
			continue
		}
		group, version := strings.Split(resource.GroupVersion, "/")[0], strings.Split(resource.GroupVersion, "/")[1]
		if group == VeleroGroup {
			preferredVersion = version
			present = true
			for ind := range resource.APIResources {
				kinds.Insert(resource.APIResources[ind].Kind)
			}
			break
		}
	}

	// if not present return err
	if !present {
		err = errors.New("velero not installed on this cluster")
		log.Warning(err)
		return &VeleroIntegration{}, err
	}

	return &VeleroIntegration{
		authClient: authClient,
		groupVersion: schema.GroupVersion{
			Group:   VeleroGroup,
			Version: preferredVersion,
		},
		kinds: kinds,
	}, nil
}

// GetStringFromUnstructured to get value from Unstructured.Object
func GetStringFromUnstructured(object map[string]interface{}, fields ...string) string {
	result, present, err := unstructured.NestedString(object, fields...)
	if !present || err != nil {
		return ""
	}
	return result
}
