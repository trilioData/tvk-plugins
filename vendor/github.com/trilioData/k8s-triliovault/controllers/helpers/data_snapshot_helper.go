package helpers

import (
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
)

// DataComponentListStatus specifies details of data components while data upload/restore
type DataComponentListStatus struct {
	NonChildJobDataComponents  []helpers.ApplicationDataSnapshot
	Active, Completed, Failed  bool
	CompletedCount, TotalCount int
	Size                       resource.Quantity
}

// Checks status of data components and return Data Components with no child jobs
func GetDataComponentListStatus(dataComponentList []helpers.ApplicationDataSnapshot,
	dataRestoreJobMap map[string]*batchv1.Job) *DataComponentListStatus {

	dataComponentListStatus := &DataComponentListStatus{
		Completed:  true,
		TotalCount: len(dataComponentList),
	}

	for j := 0; j < len(dataComponentList); j++ {
		dataSnapshotContent := &dataComponentList[j]
		hash := dataSnapshotContent.GetHash()
		childJob, found := dataRestoreJobMap[hash]

		if found {
			jobStatus := GetJobStatus(childJob)
			if jobStatus.Failed {
				dataComponentListStatus.Failed = true
				dataSnapshotContent.Status = v1.Failed
			}
			if !jobStatus.Completed {
				dataComponentListStatus.Completed = false
			} else {
				dataComponentListStatus.CompletedCount++
				dataComponentListStatus.Size.Add(dataSnapshotContent.DataComponent.Size)
				dataSnapshotContent.Status = v1.Completed
			}
			if jobStatus.Active {
				dataComponentListStatus.Active = true
				dataSnapshotContent.Status = v1.InProgress
			}
		} else {
			dataComponentListStatus.Completed = false
			dataComponentListStatus.NonChildJobDataComponents =
				append(dataComponentListStatus.NonChildJobDataComponents, *dataSnapshotContent)
		}
	}

	return dataComponentListStatus
}

// Returns aggregated data components of restore application
func GetRestoreApplicationDataComponents(restoreApplication *v1.RestoreApplication) []helpers.ApplicationDataSnapshot {
	var restoreDataComponents []helpers.ApplicationDataSnapshot

	if restoreApplication == nil {
		return restoreDataComponents
	}

	// Append custom data components
	if restoreApplication.Custom != nil && restoreApplication.Custom.Snapshot != nil {
		for dataComponentIndex := range restoreApplication.Custom.Snapshot.DataSnapshots {
			dataComponent := restoreApplication.Custom.Snapshot.DataSnapshots[dataComponentIndex]
			appDs := helpers.ApplicationDataSnapshot{AppComponent: internal.Custom, DataComponent: dataComponent}
			restoreDataComponents = append(restoreDataComponents, appDs)
		}
	}

	// Append helm data components
	for i := 0; i < len(restoreApplication.HelmCharts); i++ {
		helmApplication := restoreApplication.HelmCharts[i]
		for dataComponentIndex := range helmApplication.Snapshot.DataSnapshots {
			dataComponent := helmApplication.Snapshot.DataSnapshots[dataComponentIndex]
			appDs := helpers.ApplicationDataSnapshot{AppComponent: internal.Helm,
				ComponentIdentifier: helmApplication.Snapshot.Release, DataComponent: dataComponent}
			restoreDataComponents = append(restoreDataComponents, appDs)
		}
	}

	// Append operator data components
	for i := 0; i < len(restoreApplication.Operators); i++ {
		operatorApplication := restoreApplication.Operators[i]
		for dataComponentIndex := range operatorApplication.Snapshot.DataSnapshots {
			dataComponent := operatorApplication.Snapshot.DataSnapshots[dataComponentIndex]
			appDs := helpers.ApplicationDataSnapshot{AppComponent: internal.Operator, ComponentIdentifier: operatorApplication.Snapshot.OperatorID,
				DataComponent: dataComponent}
			restoreDataComponents = append(restoreDataComponents, appDs)
		}

		if operatorApplication.Snapshot.Helm != nil {
			hs := operatorApplication.Snapshot.Helm
			for di := range hs.DataSnapshots {
				dc := hs.DataSnapshots[di]
				appDs := helpers.ApplicationDataSnapshot{AppComponent: internal.Operator,
					ComponentIdentifier: helpers.GetOperatorHelmIdentifier(operatorApplication.Snapshot.OperatorID,
						operatorApplication.Snapshot.Helm.Release),
					DataComponent: dc}
				restoreDataComponents = append(restoreDataComponents, appDs)
			}
		}
	}

	return restoreDataComponents
}

func DeDuplicateDataSnapshot(appDsList []helpers.ApplicationDataSnapshot) []helpers.ApplicationDataSnapshot {
	var appDsSet []helpers.ApplicationDataSnapshot
	var pvcMap = make(map[string]bool)

	for appDsIndex := range appDsList {
		appDs := appDsList[appDsIndex]
		pvc := appDs.DataComponent.PersistentVolumeClaimName
		if pvcMap[pvc] {
			continue
		}
		pvcMap[pvc] = true
		appDsSet = append(appDsSet, appDs)
	}
	return appDsSet
}
