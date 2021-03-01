package helpers

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilretry "k8s.io/client-go/util/retry"
)

func getPvcFromRestoreDataComponent(dataSnapshotContent []v1.DataSnapshot, pvcName string) (dataComponentPatchPath,
	location string, pvc *corev1.PersistentVolumeClaim) {
	var tmpPvc = &corev1.PersistentVolumeClaim{}
	dataComponentPatchPath = "dataSnapshots"
	for i := 0; i < len(dataSnapshotContent); i++ {
		dc := dataSnapshotContent[i]
		err := json.Unmarshal([]byte(dc.PersistentVolumeClaimMetadata), tmpPvc)
		if err != nil {
			panic(err)
		}
		if strings.EqualFold(pvcName, tmpPvc.GetName()) {
			pvc = tmpPvc
			location = dc.Location
			dataComponentPatchPath = path.Join(dataComponentPatchPath, strconv.Itoa(i))
			break
		} else {
			tmpPvc = &corev1.PersistentVolumeClaim{}
		}
	}
	return dataComponentPatchPath, location, pvc
}

func getRestorePvcFromCustom(custom *v1.RestoreCustom, pvcName string) (customPatchPath,
	location string, pvc *corev1.PersistentVolumeClaim) {
	customPatchPath = path.Join(strings.ToLower(string(internal.Custom)), "snapshot")
	dataComponentPatchPath, location, pvc := getPvcFromRestoreDataComponent(custom.Snapshot.DataSnapshots, pvcName)
	customPatchPath = path.Join(customPatchPath, dataComponentPatchPath)
	return customPatchPath, location, pvc
}

func getRestorePvcFromHelm(helmApp []v1.RestoreHelm, helmRelease, pvcName string) (helmPatchPath,
	location string, pvc *corev1.PersistentVolumeClaim) {
	helmPatchPath = string(internal.Helm)

	var dataComponentPatchPath string
	for i := 0; i < len(helmApp); i++ {
		app := helmApp[i]
		if app.Snapshot.Release == helmRelease {
			dataComponentPatchPath, location, pvc = getPvcFromRestoreDataComponent(app.Snapshot.DataSnapshots, pvcName)
			if pvc != nil && strings.EqualFold(pvcName, pvc.GetName()) {
				helmPatchPath = path.Join(helmPatchPath, strconv.Itoa(i), "snapshot", dataComponentPatchPath)
				break
			}
		}
	}
	return helmPatchPath, location, pvc
}

func getRestorePvcFromOperator(operatorApp []v1.RestoreOperator, operatorIdentifier, pvcName string) (operatorPatchPath,
	location string, pvc *corev1.PersistentVolumeClaim) {
	var dataComponentPatchPath string
	operatorPatchPath = string(internal.Operator)
	for i := 0; i < len(operatorApp); i++ {
		app := operatorApp[i]
		if app.Snapshot.Helm != nil {
			operatorHelmID := GetOperatorHelmIdentifier(app.Snapshot.OperatorID, app.Snapshot.Helm.Release)
			if operatorHelmID == operatorIdentifier {
				dataComponentPatchPath, location, pvc = getPvcFromRestoreDataComponent(app.Snapshot.Helm.DataSnapshots, pvcName)
				if pvc != nil && strings.EqualFold(pvcName, pvc.GetName()) {
					operatorPatchPath = path.Join(operatorPatchPath, strconv.Itoa(i), "snapshot", "helm", dataComponentPatchPath)
					break
				}
			}
		}
		if app.Snapshot.OperatorID == operatorIdentifier {
			dataComponentPatchPath, location, pvc = getPvcFromRestoreDataComponent(app.Snapshot.DataSnapshots, pvcName)
			if pvc != nil && strings.EqualFold(pvcName, pvc.GetName()) {
				operatorPatchPath = path.Join(operatorPatchPath, strconv.Itoa(i), "snapshot", dataComponentPatchPath)
				break
			}
		}
	}
	return operatorPatchPath, location, pvc
}

func findPvcDetails(restoreApp *v1.RestoreApplication, appComponent, componentIdentifier, pvcName string) (patchPath, location string,
	pvc *corev1.PersistentVolumeClaim) {

	switch appComponent {
	case string(internal.Helm):
		patchPath, location, pvc = getRestorePvcFromHelm(restoreApp.HelmCharts, componentIdentifier, pvcName)
		if pvc != nil && strings.EqualFold(pvcName, pvc.GetName()) {
			return patchPath, location, pvc
		}
	case string(internal.Operator):
		patchPath, location, pvc = getRestorePvcFromOperator(restoreApp.Operators, componentIdentifier, pvcName)
		if pvc != nil && strings.EqualFold(pvcName, pvc.GetName()) {
			return patchPath, location, pvc
		}
	case string(internal.Custom):
		return getRestorePvcFromCustom(restoreApp.Custom, pvcName)
	}
	return patchPath, location, pvc
}

// TODO: Changes according to new restore spec
// GetRestorePVCdetails returns details of PV created by PVC includes patchPath ,absolute path in target, volume mode of a restore
func GetRestorePVCdetails(restore *v1.Restore, appComponent, componentIdentifier, pvcName, datastorePath string) (
	applicationPatchPath, absolutePath, volumeMode string, err error) {
	var location string
	var patchPatch string
	var pvc *corev1.PersistentVolumeClaim
	applicationPatchPath = "/status/restoreApplication"
	restoreApplication := restore.Status.RestoreApplication
	patchPatch, location, pvc = findPvcDetails(restoreApplication, appComponent, componentIdentifier, pvcName)
	if pvc != nil && strings.EqualFold(pvcName, pvc.GetName()) {
		absolutePath = path.Join(datastorePath, location)
		applicationPatchPath = path.Join(applicationPatchPath, patchPatch)
		if pvc.Spec.VolumeMode == nil {
			volumeMode = string(corev1.PersistentVolumeFilesystem)
		} else {
			volumeMode = string(*pvc.Spec.VolumeMode)
		}
		return applicationPatchPath, absolutePath, volumeMode, nil
	}
	return "", "", volumeMode, errors.New("PVC details not found")
}

// PatchRestoreCr patches restore-cr names restoreName using patchPath & value
func PatchRestoreCr(a *kube.Accessor, patchInfo map[string]interface{}, namespace, restoreName string) {
	log.Infof("Patching Restore cr %s", restoreName)
	payloadBytes := getRestoreCrPayload(patchInfo)

	restore, gErr := a.GetRestore(restoreName, namespace)
	if gErr != nil {
		log.Fatalf("Failed to get restore cr[%s]: %v", restoreName, gErr)
	}

	sErr := a.StatusJSONPatch(restore, payloadBytes)
	if sErr != nil {
		log.Fatalf("Failed to patch restore cr[%s]: %v", restoreName, sErr)
	}
	log.Infof("%s Restore Cr Patching successful.", restoreName)
}

// getRestoreCrPayload receives patchInfo(patchPath & value) and returns payloads for patch operation
func getRestoreCrPayload(patchInfo map[string]interface{}) []byte {
	var payload []internal.PatchOperation
	for patchPath, val := range patchInfo {
		payload = append(payload, internal.PatchOperation{
			Op:    "replace",
			Path:  patchPath,
			Value: val,
		})
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal patch payload: %+v", err)
	}
	return payloadBytes
}

// GetDirSize retrieve directory size in bytes and parse & return's in resource.Quantity format.
// input:  dirpath: 			source directory path to get directory size.
// output: resource.Quantity:	bytes in resource.Quantity format.
//         error: 				!=nil if failed to get or parse size.
func GetDirSize(dirPath string) (resource.Quantity, error) {
	// retrieving directory size in bytes.
	cmd := fmt.Sprintf("du -sb %s", dirPath)
	outStruct, err := shell.RunCmd(cmd)
	if err != nil {
		return resource.Quantity{}, nil
	}

	if len(outStruct.Out) > 0 {
		dirInfo := strings.Fields(outStruct.Out)
		return resource.ParseQuantity(dirInfo[0])
	}
	return resource.Quantity{}, nil
}

func CheckIfRestoreFromResourceExists(a *kube.Accessor, resourceKind, resourceName, namespaceName string) (restoreName string,
	isUpdateAllowed, exists bool) {

	// Get the available restore list
	RestoreObjList, listErr := a.GetRestores(namespaceName)
	if listErr != nil && apierr.IsNotFound(listErr) {
		return "", false, false
	}

	switch resourceKind {
	case internal.BackupKind:
		// Iterate over restore list to check if req backup is used in it
		for restoreIndex := range RestoreObjList {
			restoreObj := RestoreObjList[restoreIndex]
			if restoreObj.Spec.Source.Type == v1.BackupSource && restoreObj.Spec.Source.Backup.Name == resourceName {
				if restoreObj.Status.Status == v1.InProgress || restoreObj.Status.Status == v1.Pending ||
					restoreObj.Status.Status == v1.Error || restoreObj.Status.Status == "" {
					return restoreObj.Name, false, true
				}
				exists = true
			}
		}
		return "", true, exists
	case internal.TargetKind:
		for restoreIndex := range RestoreObjList {
			restoreObj := RestoreObjList[restoreIndex]
			if restoreObj.Spec.Source.Type == v1.LocationSource && restoreObj.Spec.Source.Target.Name == resourceName {
				if restoreObj.Status.Status == v1.InProgress || restoreObj.Status.Status == v1.Pending ||
					restoreObj.Status.Status == v1.Error || restoreObj.Status.Status == "" {
					return restoreObj.Name, false, true
				}
				exists = true
			}
		}
		return "", true, exists
	}
	return "", false, exists

}

// UpdateRestoreStatus updates the passed restore status using `MergePatch`. It retries for 5 times
// based on the api `Conflict` error
func UpdateRestoreStatus(a *kube.Accessor, restore *v1.Restore) error {
	if restore == nil {
		return errors.New("nil restore parameter")
	}

	if retryErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
		currRestore, err := a.GetRestore(restore.Name, restore.Namespace)
		if err != nil {
			log.Warnf("error while getting the latest object before updating status: %s", err.Error())
			return err
		}

		if sErr := a.StatusMergePatch(restore, currRestore); sErr != nil {
			log.Warnf("error while patching the status: %s", sErr.Error())
			return sErr
		}
		return nil
	}); retryErr != nil {
		log.Warnf("error patching the status of restore: %s", retryErr.Error())

		if sErr := a.StatusUpdate(restore); sErr != nil {
			utilruntime.HandleError(fmt.Errorf("error updating the status of restore: %s", sErr.Error()))
			return sErr
		}
	}

	log.Info("Updated restore status: ", restore.Status.Status, " name: ", restore.GetName(), " namespace: ", restore.GetNamespace())
	return nil
}

// GetTransMaps returns the map for custom and helm transformations
func GetTransMaps(restore *v1.Restore) (cGvkTrans map[v1.GroupVersionKind][]v1.CustomTransform,
	hReleaseTrans map[string]v1.HelmTransform) {
	cGvkTrans = make(map[v1.GroupVersionKind][]v1.CustomTransform)
	hReleaseTrans = make(map[string]v1.HelmTransform)

	// Create the custom transformation map
	if restore.Spec.TransformComponents != nil && len(restore.Spec.TransformComponents.Custom) > 0 {
		for i := range restore.Spec.TransformComponents.Custom {
			ct := restore.Spec.TransformComponents.Custom[i]
			cGvkTrans[ct.Resources.GroupVersionKind] = append(cGvkTrans[ct.Resources.GroupVersionKind], ct)
		}
	}

	// Create the helm transformation map
	if restore.Spec.TransformComponents != nil && len(restore.Spec.TransformComponents.Helm) > 0 {
		for i := range restore.Spec.TransformComponents.Helm {
			ht := restore.Spec.TransformComponents.Helm[i]
			hReleaseTrans[ht.Release] = ht
		}
	}

	return cGvkTrans, hReleaseTrans
}
