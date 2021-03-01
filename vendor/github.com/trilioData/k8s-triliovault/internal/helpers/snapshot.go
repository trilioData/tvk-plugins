package helpers

import (
	"reflect"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
)

// Data Component Functions
type ApplicationDataSnapshot struct {
	AppComponent        internal.SnapshotType
	ComponentIdentifier string
	DataComponent       v1.DataSnapshot
	Status              v1.Status
}

func (a *ApplicationDataSnapshot) GetHash() string {
	return GetHash(string(a.AppComponent), a.ComponentIdentifier, a.DataComponent.PersistentVolumeClaimName)
}

func GetBackupDataComponents(backupSnapshot *v1.Snapshot,
	isVolumeSnapshotCompleted bool) (backupDataComponents []ApplicationDataSnapshot,
	aggregateCount int) {

	backupDataComponents = []ApplicationDataSnapshot{}

	if backupSnapshot == nil {
		return backupDataComponents, aggregateCount
	}

	// Append custom data components
	if backupSnapshot.Custom != nil {
		aggregateCount += len(backupSnapshot.Custom.DataSnapshots)
		for dataComponentIndex := range backupSnapshot.Custom.DataSnapshots {
			dataComponent := backupSnapshot.Custom.DataSnapshots[dataComponentIndex]
			appDs := ApplicationDataSnapshot{AppComponent: internal.Custom, DataComponent: dataComponent}
			if isVolumeSnapshotCompleted {
				if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			} else {
				backupDataComponents = append(backupDataComponents, appDs)
			}
		}
	}

	// Append helm data components
	for i := 0; i < len(backupSnapshot.HelmCharts); i++ {
		helmApplication := backupSnapshot.HelmCharts[i]
		aggregateCount += len(helmApplication.DataSnapshots)
		for dataComponentIndex := range helmApplication.DataSnapshots {
			dataComponent := helmApplication.DataSnapshots[dataComponentIndex]
			appDs := ApplicationDataSnapshot{AppComponent: internal.Helm, ComponentIdentifier: helmApplication.Release,
				DataComponent: dataComponent}
			if isVolumeSnapshotCompleted {
				if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			} else {
				backupDataComponents = append(backupDataComponents, appDs)
			}
		}
	}

	// Append operator data components
	for i := 0; i < len(backupSnapshot.Operators); i++ {
		operatorApplication := backupSnapshot.Operators[i]
		aggregateCount += len(operatorApplication.DataSnapshots)
		for dataComponentIndex := range operatorApplication.DataSnapshots {
			dataComponent := operatorApplication.DataSnapshots[dataComponentIndex]
			appDs := ApplicationDataSnapshot{AppComponent: internal.Operator,
				ComponentIdentifier: operatorApplication.OperatorID, DataComponent: dataComponent}
			if isVolumeSnapshotCompleted {
				if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			} else {
				backupDataComponents = append(backupDataComponents, appDs)
			}
		}
		if operatorApplication.Helm != nil {
			operatorHelm := operatorApplication.Helm
			aggregateCount += len(operatorHelm.DataSnapshots)
			for dataComponentIndex := range operatorHelm.DataSnapshots {
				dataComponent := operatorHelm.DataSnapshots[dataComponentIndex]
				appDs := ApplicationDataSnapshot{AppComponent: internal.Operator,
					ComponentIdentifier: GetOperatorHelmIdentifier(operatorApplication.OperatorID, operatorHelm.Release),
					DataComponent:       dataComponent}
				if isVolumeSnapshotCompleted {
					if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
						backupDataComponents = append(backupDataComponents, appDs)
					}
				} else {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			}
		}
	}

	return backupDataComponents, aggregateCount
}
