package apis

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path"

	"k8s.io/apimachinery/pkg/types"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
)

type SnapshotMgr interface {
	PushToTarget(location string) error
	PullFromTarget(location string) error
	ConvertToCrdSnapshots() (interface{}, error)
}

type FullSnapshot struct {
	HelmSnapshots     []HelmSnapshot
	OperatorSnapshots []OperatorSnapshot
	CustomSnapshot    *CustomSnapshot
}

type ComponentMetadata struct {
	GroupVersionKind crd.GroupVersionKind `json:"groupVersionKind"`
	Metadata         []string             `json:"metadata,omitempty"`
	Names            []string             `json:"-"`
}

type CustomSnapshot struct {
	Resources     []ComponentMetadata
	DataSnapshots []crd.DataSnapshot
	Warnings      []string
}

type OperatorSnapshot struct {
	OperatorID                         string
	CRDMetadata, Warnings              []string
	CustomResources, OperatorResources []ComponentMetadata
	Helm                               *HelmSnapshot
	DataSnapshots                      []crd.DataSnapshot
}

type HelmSnapshot struct {
	Release, NewRelease string
	Revision            int32
	Metadata            *ComponentMetadata
	// ReleaseResources only used to get helm release resources and populate them in status
	// helm release resources meta will not be uploaded to target
	ReleaseResources []ComponentMetadata
	StorageBackend   crd.HelmStorageBackend
	Version          crd.HelmVersion
	DataSnapshots    []crd.DataSnapshot
	Warnings         []string
}

// Transform performs the transformation on FullSnapshot and update the transformStatus in the restore CR
// It mainly requires CustomTransformationMap and HelmTransformationMap to perform transformation on all Custom, Helm
// and Operators
func (f *FullSnapshot) Transform(a *kube.Accessor, restoreObj *crd.Restore, backupLocation string,
	cGvkTrans map[crd.GroupVersionKind][]crd.CustomTransform,
	hReleaseTrans map[string]crd.HelmTransform, excludePolicy ExcludePolicy) error {
	var (
		isCusTransFailed, isOpTransFailed, isHelmTransFailed bool
		customRestore                                        *crd.RestoreCustom
		opRestore                                            []crd.RestoreOperator
		helmRestore                                          []crd.RestoreHelm
	)
	// If full snapshot is nil, return
	if f == nil {
		log.Errorf("snapshot is empty")
		return errors.New("snapshot is empty")
	}

	// If no transformation provided, return
	if restoreObj.Spec.TransformComponents == nil {
		log.Info("Transformation is not required.")
		return nil
	}

	// Allocate the RestoreApplication object as it will be nil initially
	restoreObj.Status.RestoreApplication = new(crd.RestoreApplication)

	if f.CustomSnapshot != nil && len(restoreObj.Spec.TransformComponents.Custom) > 0 {
		// Perform custom transformation
		customRestore, isCusTransFailed = f.CustomSnapshot.Transform(a, restoreObj.Spec.RestoreNamespace,
			cGvkTrans, excludePolicy)
		restoreObj.Status.RestoreApplication.Custom = customRestore
	}

	if len(f.HelmSnapshots) > 0 && len(restoreObj.Spec.TransformComponents.Helm) > 0 {
		hSnapshots := HelmSnapshots(f.HelmSnapshots)
		// Perform all helm release transformation
		helmRestore, isHelmTransFailed = hSnapshots.Transform(a, restoreObj.Spec.RestoreNamespace, hReleaseTrans,
			backupLocation)
		if len(helmRestore) > 0 {
			restoreObj.Status.RestoreApplication.HelmCharts = helmRestore
		}
	}

	if len(f.OperatorSnapshots) > 0 && (len(restoreObj.Spec.TransformComponents.Custom) > 0 ||
		len(restoreObj.Spec.TransformComponents.Helm) > 0) {
		// Perform all operators transformation
		oSnapshots := OperatorSnapshots(f.OperatorSnapshots)
		opRestore, isOpTransFailed = oSnapshots.Transform(a, restoreObj.Spec.RestoreNamespace, cGvkTrans,
			hReleaseTrans, excludePolicy, backupLocation)
		if len(opRestore) > 0 {
			restoreObj.Status.RestoreApplication.Operators = opRestore
		}
	}

	upErr := helpers.UpdateRestoreStatus(a, restoreObj)
	if upErr != nil {
		return upErr
	}

	if isCusTransFailed || isHelmTransFailed || isOpTransFailed {
		return fmt.Errorf("transformation failed")
	}

	return nil
}

// PushToTarget pushes the snapshot to the given location on the target
// Input:
//		location: Location on which snapshot needs to be pushed
// Output:
//		Error if any
func (f *FullSnapshot) PushToTarget(location string) (err error) {
	var outStr string

	if f == nil {
		log.Errorf("snapshot is empty")
		return errors.New("snapshot is empty")
	}

	outStr, err = shell.Mkdir(location)
	if err != nil {
		log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", location, outStr)
		return err
	}
	log.Infof("Created the destination directory for the backup: %s", location)

	// Upload Helm metadata
	if f.HelmSnapshots != nil {
		hSnapshots := HelmSnapshots(f.HelmSnapshots)
		err = hSnapshots.PushToTarget(location)
		if err != nil {
			log.Errorf("Error while backing up helm metadata %s at datastore, ERROR: %s", location, err.Error())
			return err
		}
	}

	// Upload Operator metadata
	if f.OperatorSnapshots != nil {
		oSnapshots := OperatorSnapshots(f.OperatorSnapshots)
		err = oSnapshots.PushToTarget(location)
		if err != nil {
			log.Errorf("Error while backing up operator metadata %s at datastore, ERROR: %s", location, err.Error())
			return err
		}
	}

	// Upload Custom metadata
	if f.CustomSnapshot != nil {
		err = f.CustomSnapshot.PushToTarget(location)
		if err != nil {
			log.Errorf("Error while backing up custom metadata %s at datastore, ERROR: %s", location, err.Error())
			return err
		}
	}

	return nil
}

// PullFromTarget reads the metadata and data snapshot from target
// Input:
//		location: Location of the backup from where data needs to be read
// Output:
//		Error
func (f *FullSnapshot) PullFromTarget(location string) error {
	var (
		helmSnapshots  = HelmSnapshots{}
		opSnapshots    = OperatorSnapshots{}
		customSnapshot = &CustomSnapshot{}
	)

	// Check if given location is present
	valErr := helpers.ValidateBackupLocation(location)
	if valErr != nil {
		log.Errorf("Error while validating backup location %s, ERROR: %s", location, valErr.Error())
		return valErr
	}

	customBackupPath := path.Join(location, internal.CustomBackupDir)
	if internal.DirExists(customBackupPath) {
		log.Infof("Reading custom snapshot from %s", customBackupPath)
		// Read custom snapshot from target
		customErr := customSnapshot.PullFromTarget(customBackupPath)
		if customErr != nil {
			log.Errorf("Error while pulling custom snapshot from target, ERROR: %s", customErr.Error())
			return customErr
		}
		f.CustomSnapshot = customSnapshot
	}

	// Read helm snapshots from target
	helmBackupPath := path.Join(location, internal.HelmBackupDir)
	if internal.DirExists(helmBackupPath) {
		log.Infof("Reading helm snapshot from %s", helmBackupPath)
		helmErr := helmSnapshots.PullFromTarget(helmBackupPath)
		if helmErr != nil {
			log.Errorf("Error while pulling helm snapshots from target, ERROR: %s", helmErr.Error())
			return helmErr
		}
		if len(helmSnapshots) > 0 {
			f.HelmSnapshots = helmSnapshots
		}
	}

	// Read operator snapshots from target
	opBackupPath := path.Join(location, internal.OperatorBackupDir)
	if internal.DirExists(opBackupPath) {
		log.Infof("Reading operator snapshot from %s", opBackupPath)
		log.Info("Reading Operator snapshot")
		opErr := opSnapshots.PullFromTarget(opBackupPath)
		if opErr != nil {
			log.Errorf("Error while pulling operator snapshots from target, ERROR: %s", opErr.Error())
			return opErr
		}
		if len(opSnapshots) > 0 {
			f.OperatorSnapshots = opSnapshots
		}
	}

	return nil
}

// ConvertToCrdSnapshots converts the backed up structures to crd structures
// It removes the metadata and keep names only while converting
// Output:
//		Converted full snapshot
//		Error
func (f *FullSnapshot) ConvertToCrdSnapshots() (interface{}, error) {
	var crdSnapshot crd.Snapshot

	if f == nil {
		log.Error("snapshot is empty")
		return nil, errors.New("snapshot is empty")
	}

	// Convert Helm snapshot
	if f.HelmSnapshots != nil {
		hSnapshots := HelmSnapshots(f.HelmSnapshots)
		convertedHS, hErr := hSnapshots.ConvertToCrdSnapshots()
		if hErr != nil {
			log.Errorf("Error while converting helm snapshots to crd required format, ERROR: %s", hErr.Error())
			return nil, hErr
		}
		crdSnapshot.HelmCharts = convertedHS.([]crd.Helm)
	}

	// Convert Operator snapshot
	if f.OperatorSnapshots != nil {
		oSnapshots := OperatorSnapshots(f.OperatorSnapshots)
		convertedOpS, opErr := oSnapshots.ConvertToCrdSnapshots()
		if opErr != nil {
			log.Errorf("Error while converting operator snapshots to crd required format, ERROR: %s", opErr.Error())
			return nil, opErr
		}
		crdSnapshot.Operators = convertedOpS.([]crd.Operator)
	}

	// Convert custom snapshot
	if f.CustomSnapshot != nil {
		convertedCS, cusErr := f.CustomSnapshot.ConvertToCrdSnapshots()
		if cusErr != nil {
			log.Errorf("Error while converting custom snapshot to crd required format, ERROR: %s", cusErr.Error())
			return nil, cusErr
		}
		crdCustomSnapshot := convertedCS.(crd.Custom)
		crdSnapshot.Custom = &crdCustomSnapshot
	}

	return crdSnapshot, nil
}

func (f *FullSnapshot) CheckAndGetHookStatus(acc *kube.Accessor, kind string, namespacedName types.NamespacedName,
	hookConfig *crd.HookConfig) (crd.HookComponentStatus, error) {

	log.Infof("checking if hook implementation needed for %s %s", kind, namespacedName.Name)

	var hookComponentStatus crd.HookComponentStatus

	var hookPriority crd.HookPriority
	if hookConfig != nil {
		hookMode := hookConfig.Mode
		hooks := hookConfig.Hooks
		log.Infof("%s -> %s needs hook[mode %s] implementation", kind, namespacedName.Name, hookMode)

		hookComponentStatus.PodReadyWaitSeconds = hookConfig.PodReadyWaitSeconds

		var hookPriorityStatus []crd.HookPriorityStatus

		if hookMode == crd.Sequential {
			hookPriorityStatus = make([]crd.HookPriorityStatus, len(hooks))
		} else {
			hookPriorityStatus = make([]crd.HookPriorityStatus, 1)
		}

		for i := range hooks {
			hookInfo := hooks[i]
			var identifiedResources []runtime.Object

			// identifying targeting hook resources i.e owners,pods from custom,helm & operator.
			identifiedResources, err = f.identifyHookResources(acc, *hookInfo.PodSelector,
				namespacedName.Namespace, hookInfo.ContainerRegex)
			if err != nil {
				return hookComponentStatus, err
			}

			if len(identifiedResources) == 0 {
				errStr := fmt.Sprintf("no matching resources found for hook %s, mode %s", hookInfo.Hook.Name,
					hookMode)
				log.Error(errStr)
				return hookComponentStatus, errors.New(errStr)
			}

			if hookMode == crd.Sequential {
				hookPriority, err = convertToHookStatus(acc, identifiedResources, hookInfo)
				if err != nil {
					return hookComponentStatus, err
				}
				hookPriorityStatus[i].Priority = uint8(i)
				hookPriorityStatus[i].Hooks = append(hookPriorityStatus[i].Hooks, hookPriority)
			} else {
				hookPriority, err = convertToHookStatus(acc, identifiedResources, hookInfo)
				if err != nil {
					return hookComponentStatus, err
				}

				hookPriorityStatus[0].Priority = uint8(0)
				hookPriorityStatus[0].Hooks = append(hookPriorityStatus[0].Hooks, hookPriority)
			}
		}

		hookComponentStatus.HookPriorityStatuses = hookPriorityStatus
		return hookComponentStatus, nil
	}
	log.Infof("hook implementation not required for %s %s", kind, namespacedName.Name)

	return hookComponentStatus, nil
}

func (f *FullSnapshot) identifyHookResources(acc *kube.Accessor, podSelector crd.PodSelector,
	ns, containerRegex string) ([]runtime.Object, error) {
	log.Info("identifying hook target resources")

	var filteredResources []runtime.Object
	var tempObjs []runtime.Object

	if f.CustomSnapshot != nil {
		log.Info("identifying hook resources for custom")
		tempObjs, err = identifyAndGetTargetingHookPods(acc, f.CustomSnapshot.Resources,
			ns, containerRegex, podSelector)
		if err != nil {
			return []runtime.Object{}, err
		}
		filteredResources = append(filteredResources, tempObjs...)
	}

	if len(f.HelmSnapshots) > 0 {
		log.Info("identifying hook resources for helm")

		for i := range f.HelmSnapshots {
			snap := f.HelmSnapshots[i]
			tempObjs, err = identifyAndGetTargetingHookPods(acc, snap.ReleaseResources,
				ns, containerRegex, podSelector)
			if err != nil {
				return []runtime.Object{}, err
			}
			filteredResources = append(filteredResources, tempObjs...)
		}
	}

	if len(f.OperatorSnapshots) > 0 {
		log.Info("identifying hook resources for operator")

		for i := range f.OperatorSnapshots {
			snap := f.OperatorSnapshots[i]
			tempObjs, err = identifyAndGetTargetingHookPods(acc, snap.OperatorResources,
				ns, containerRegex, podSelector)
			if err != nil {
				return []runtime.Object{}, err
			}
			filteredResources = append(filteredResources, tempObjs...)
			if snap.Helm != nil {
				tempObjs, err = identifyAndGetTargetingHookPods(acc, snap.Helm.ReleaseResources,
					ns, containerRegex, podSelector)
				if err != nil {
					return []runtime.Object{}, err
				}
				filteredResources = append(filteredResources, tempObjs...)
			}
		}
	}

	return filteredResources, nil
}

// PushToTarget pushes the tvk meta i.e. namespace UUID and TVK version
// to the file located in /backupplan-uuid/backup-uuid/tvk-meta.json
func (t *TVKMeta) PushToTarget(location string) error {
	err = helpers.SerializeStructToFilePath(*t, location, internal.TVKMetaFile)
	if err != nil {
		log.Errorf("Error while serializing file at %s", path.Join(location, internal.TVKMetaFile))
		return err
	}

	return nil
}

// PullFromTarget pulls the tvk meta i.e. namespace UUID and TVK version
// from file /backupplan-uuid/backup-uuid/tvk-meta.json
func (t *TVKMeta) PullFromTarget(location string) error {
	tvkMetaPath := path.Join(location, internal.TVKMetaFile)
	if internal.FileExists(tvkMetaPath) {
		// Read the file
		tvkMetaBytes, readErr := ioutil.ReadFile(tvkMetaPath)
		if readErr != nil {
			log.Errorf("Error while reading file %s: %s", tvkMetaPath, readErr.Error())
			return readErr
		}
		// Unmarshal the tvk meta
		unmarshalErr := yaml.Unmarshal(tvkMetaBytes, t)
		if unmarshalErr != nil {
			log.Errorf("Error while unmarshalling: %s", unmarshalErr.Error())
			return unmarshalErr
		}
	}

	return nil
}

// This method is not required for the TVKMeta as of now
func (t *TVKMeta) ConvertToCrdSnapshots() (interface{}, error) {
	return nil, nil
}
