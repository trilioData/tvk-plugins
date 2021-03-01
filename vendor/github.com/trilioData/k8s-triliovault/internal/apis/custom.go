package apis

import (
	"fmt"
	"path"

	"github.com/trilioData/k8s-triliovault/internal/helpers"

	log "github.com/sirupsen/logrus"
	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/decorator"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

func (c *CustomSnapshot) PushToTarget(location string) error {
	// PushToTarget pushes the snapshot to the given location on the target
	// Input:
	//		location: Location on which snapshot needs to be pushed
	// Output:
	//		Error if any

	customPath := path.Join(location, internal.CustomBackupDir)

	if len(c.Resources) > 0 {
		// Create directory for Custom metadata
		metadataPath := path.Join(customPath, internal.MetadataSnapshotDir)
		outStr, err := shell.Mkdir(metadataPath)
		if err != nil {
			log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", metadataPath, outStr)
			return err
		}

		// Serialize metadata and write to file
		err = helpers.SerializeStructToFilePath(c.Resources, metadataPath, internal.MetadataJSON)
		if err != nil {
			log.Errorf("Error while Serializing file at %s", metadataPath)
			return err
		}
	}

	// Create and Write metadata of data snapshot
	customDataSnapshotPath := path.Join(customPath, internal.DataSnapshotDir)
	err := UploadPVCMetadata(c.DataSnapshots, customDataSnapshotPath)
	if err != nil {
		log.Errorf("Error while creating the backup pvc %s at datastore, ERROR: %s", customDataSnapshotPath, err.Error())
		return err
	}

	return nil
}

func (c *CustomSnapshot) PullFromTarget(location string) error {
	// PullFromTarget reads the metadata and data snapshot from target
	// Input:
	//		location: Location of the backup from where data needs to be read
	// Output:
	//		Error if any

	// Get the metadata
	customMetadataPath := path.Join(location, internal.MetadataSnapshotDir, internal.MetadataJSON)
	// Check if custom metadata file exists. If exists then unmarshal the metadata
	if internal.FileExists(customMetadataPath) {
		umCustomMetadata, unmarshalErr := unMarshalMeta(customMetadataPath)
		if unmarshalErr != nil {
			log.Errorf("Error while unmarshalling custom metadata, Error: %s", unmarshalErr.Error())
			return unmarshalErr
		}
		c.Resources = umCustomMetadata
	}

	// Get the data
	customDataPath := path.Join(location, internal.DataSnapshotDir)
	if internal.DirExists(customDataPath) {
		dataSnapshots, getDsErr := getDataSnapshots(customDataPath)
		if getDsErr != nil {
			log.Errorf("Error while getting data snapshots for custom, Error: %s", getDsErr.Error())
			return getDsErr
		}
		c.DataSnapshots = dataSnapshots
	}

	return nil
}

func (c *CustomSnapshot) ConvertToCrdSnapshots() (interface{}, error) {
	// ConvertToCrdSnapshots converts the backed up structures to crd structures
	// It removes the metadata and keep names only while converting
	// Output:
	//		Converted custom snapshot
	//		Error if any
	var (
		crdCustomSnapshot crd.Custom
		crdResources      []crd.Resource
	)

	for i := range c.Resources {
		var crdRes crd.Resource
		resource := c.Resources[i]
		resNames, resErr := GetResourceNames(&resource)
		if resErr != nil {
			log.Errorf("Error while getting custom resources names from the metadata, Error: %s", resErr.Error())
			return nil, resErr
		}

		crdRes.GroupVersionKind = resource.GroupVersionKind

		if len(resNames) > 0 {
			crdRes.Objects = resNames
		}

		crdResources = append(crdResources, crdRes)
	}

	crdCustomSnapshot.Resources = crdResources
	crdCustomSnapshot.DataSnapshots = c.DataSnapshots

	if len(c.Warnings) > 0 {
		crdCustomSnapshot.Warnings = c.Warnings
	}

	return crdCustomSnapshot, nil

}

// Transform performs the Custom transformation
func (c *CustomSnapshot) Transform(a *kube.Accessor, ns string, cGvkTrans map[crd.GroupVersionKind][]crd.CustomTransform,
	excludePolicy ExcludePolicy) (*crd.RestoreCustom, bool) {
	var (
		dsTransformStatus, msTransformStatus   []crd.TransformStatus
		isAnyDSTransFailed, isAnyMSTransFailed bool
		compStatus                             crd.ComponentStatus
		customRestore                          crd.RestoreCustom
		excludedRes                            []crd.Resource
	)

	compStatus.Phase = crd.RestoreValidation

	if len(c.DataSnapshots) > 0 {
		// Transform data
		dsTransformStatus, excludedRes, isAnyDSTransFailed = transformDataSnapshots(a, ns, &c.DataSnapshots,
			cGvkTrans, excludePolicy)
		compStatus.TransformStatus = append(compStatus.TransformStatus, dsTransformStatus...)
		compStatus.ExcludedResources = append(compStatus.ExcludedResources, excludedRes...)
	}

	if len(c.Resources) > 0 {
		// Transform meta
		msTransformStatus, excludedRes, isAnyMSTransFailed = transformMetaSnapshots(a, ns, c.Resources, cGvkTrans,
			excludePolicy)
		compStatus.TransformStatus = append(compStatus.TransformStatus, msTransformStatus...)
		compStatus.ExcludedResources = append(compStatus.ExcludedResources, excludedRes...)
	}

	if isAnyDSTransFailed || isAnyMSTransFailed {
		compStatus.PhaseStatus = crd.Failed
	}

	customRestore.Status = &compStatus
	return &customRestore, isAnyDSTransFailed || isAnyMSTransFailed

}

// transformMetaSnapshots transforms the meta as per the given json patches and excludePolicy
// Input:
//			metaSnapshots: list of component metadata i.e. list of map of [GVK: List of metadata for the same GVK]
//			gvkTransMap: Map of [GVK: List of transformation for the corresponding GVK]
//          excludePolicy: 	ExcludePolicy type containing disableIgnore flag, IgnoreList and ExcludeResourceMap
// Output:
//			transformStatus: status of transformation of all resources
//			excludedRes: slice of all excluded Resources
//			isAnyTransFailed: bool flag to determine if any transformation failed
// This function iterate over component metadata i.e. processing one GVK at a time.
// If a particular GVK/GVKN is present in excludeResMap then skip transformation for that GVK/GVKN and
// add it to excludedResources list
// If GVK is present in gvkTransMap map, perform transformation else continue with next component.
func transformMetaSnapshots(a *kube.Accessor, ns string, metaSnapshots []ComponentMetadata,
	gvkTransMap map[crd.GroupVersionKind][]crd.CustomTransform,
	excludePolicy ExcludePolicy) (transformStatus []crd.TransformStatus, excludedRes []crd.Resource,
	isAnyTransFailed bool) {
	var (
		transformAll, transformSpecific, isAnyTransHappened bool
	)

	for compIndex := range metaSnapshots {
		compMetaData := metaSnapshots[compIndex]
		gk := schema.GroupKind{
			Group: compMetaData.GroupVersionKind.Group,
			Kind:  compMetaData.GroupVersionKind.Kind,
		}

		ExcludeObjSet := sets.NewString()

		if !excludePolicy.DisableIgnore && excludePolicy.ValidationIgnoreList.Has(gk.String()) {
			exclRes, err := GetExcludedResStatus(&compMetaData)
			if err != nil {
				return
			}
			excludedRes = append(excludedRes, exclRes)
			continue
		}

		// if gvk present in excludeResMap and length of objects is 0
		// then ignore transformation and add to excludedResources in status
		if val, ok := excludePolicy.ExcludeResourceMap[compMetaData.GroupVersionKind]; ok {
			if val.Len() == 0 {
				exclRes, err := GetExcludedResStatus(&compMetaData)
				if err != nil {
					return
				}
				excludedRes = append(excludedRes, exclRes)
				continue
			}

			ExcludeObjSet.Insert(val.List()...)
		}

		if _, ok := gvkTransMap[compMetaData.GroupVersionKind]; !ok {
			// Transformation for the GVK is not provided
			continue
		}

		// Iterate of transformations of the same GVK
		for tIndex := range gvkTransMap[compMetaData.GroupVersionKind] {
			transformAll = false
			isAnyTransHappened = false
			transObjNames := sets.String{}
			transformObj := gvkTransMap[compMetaData.GroupVersionKind][tIndex]
			transformResource := crd.Resource{
				GroupVersionKind: compMetaData.GroupVersionKind,
				Objects:          []string{},
			}

			tStatus := crd.TransformStatus{
				TransformName:        transformObj.TransformName,
				Status:               crd.Completed,
				TransformedResources: []crd.Resource{},
			}
			log.Debugf("Performing transformation %s for [%+v]",
				transformObj.TransformName, compMetaData.GroupVersionKind)

			if len(transformObj.Resources.Objects) == 0 {
				// If objects are not specified then run transformation for
				// all objects of the GVK
				transformAll = true
			} else {
				transObjNames.Insert(transformObj.Resources.Objects...)
			}

			var ResToBeExcluded []string

			for metaIndex := range compMetaData.Metadata {
				transformSpecific = false
				metadata := compMetaData.Metadata[metaIndex]
				// Unmarshal the meta to get the name
				obj := unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind(compMetaData.GroupVersionKind))

				if uMarshalErr := yaml.Unmarshal([]byte(metadata), &obj); uMarshalErr != nil {
					tStatus.Status = crd.Failed
					tStatus.Reason += fmt.Sprintf("Transformation failed "+
						"for %s with error %s;", obj.GetName(), uMarshalErr.Error())
					isAnyTransFailed = true
					isAnyTransHappened = true
					continue
				}

				if ExcludeObjSet.Has(obj.GetName()) {
					ResToBeExcluded = append(ResToBeExcluded, obj.GetName())
					continue
				}

				if transObjNames.Len() > 0 && transObjNames.Has(obj.GetName()) {
					transformSpecific = true
				}

				if transformAll || transformSpecific {
					isAnyTransHappened = true
					unsObj := decorator.UnstructResource(obj)
					transformResource.Objects = append(transformResource.Objects, obj.GetName())
					log.Infof("Transforming resource [%s] with GVK [%+v]", obj.GetName(), obj.GroupVersionKind())
					transformedMeta, tErr := unsObj.TransformAndDryRun(a, ns, metadata, transformObj.JSONPatches)
					if tErr != nil {
						tStatus.Status = crd.Failed
						tStatus.Reason += fmt.Sprintf("Transformation failed "+
							"for %s: %s;", obj.GetName(), tErr.Error())
						isAnyTransFailed = true
						continue
					}
					compMetaData.Metadata[metaIndex] = transformedMeta
				}
			}

			// merge the new resources to be excluded in the main excludedRes list
			if len(ResToBeExcluded) > 0 {
				excludedRes = MergeResourceList(excludedRes, []crd.Resource{{
					GroupVersionKind: compMetaData.GroupVersionKind,
					Objects:          ResToBeExcluded,
				}})
			}

			if isAnyTransHappened {
				tStatus.TransformedResources = append(tStatus.TransformedResources, transformResource)
				transformStatus = append(transformStatus, tStatus)
			}
		}
	}

	return transformStatus, excludedRes, isAnyTransFailed
}

// transformDataSnapshots transforms the data snapshots as per the given json patches and excludePolicy
// Input:
//			dataSnapshots: list of data snapshots
//			gvkTransMap: Map of [GVK: List of transformation for the corresponding GVK]
//          excludePolicy: 	ExcludePolicy type containing disableIgnore flag, IgnoreList and ExcludeResourceMap
// Output:
//			transformStatus: status of transformation of all resources
//			excludedRes: slice of all excluded Resources
//			isAnyTransFailed: bool flag to determine if any transformation failed
// This function iterate over data snapshots i.e. processing one data snapshot at a time.
// If a particular PVC is present in excludeResMap then skip transformation for that PVC, update the dataSnapshots
// add it to excludedResources list
// If PVC is present in gvkTransMap map, perform transformation else continue with next component.
func transformDataSnapshots(a *kube.Accessor, ns string, dataSnapshots *[]crd.DataSnapshot,
	gvkTransMap map[crd.GroupVersionKind][]crd.CustomTransform,
	excludePolicy ExcludePolicy) (transformStatus []crd.TransformStatus, excludedRes []crd.Resource,
	isAnyTransFailed bool) {
	var (
		transformAll, transformSpecific, isAnyTransHappened bool
		updateDataSnapshots                                 []crd.DataSnapshot
	)

	ExcludeObjSet := sets.NewString()

	pvcGvk := crd.GroupVersionKind{
		Group:   corev1.GroupName,
		Version: "v1",
		Kind:    internal.PersistentVolumeClaimKind,
	}

	// if gvk present in excludeResMap and length of objects is 0
	// then ignore transformation and add to excludedResources in status
	if val, ok := excludePolicy.ExcludeResourceMap[pvcGvk]; ok {
		if val.Len() == 0 {
			var objects []string
			for i := range *dataSnapshots {
				d := (*dataSnapshots)[i]
				objects = append(objects, d.PersistentVolumeClaimName)
			}

			excludedRes = append(excludedRes, crd.Resource{
				GroupVersionKind: pvcGvk,
				Objects:          objects,
			})
			*dataSnapshots = updateDataSnapshots

			return
		}

		ExcludeObjSet.Insert(val.List()...)
	}

	if _, ok := gvkTransMap[pvcGvk]; !ok {
		return transformStatus, excludedRes, isAnyTransFailed
	}

	// Iterate of transformations of the PVC
	for tIndex := range gvkTransMap[pvcGvk] {
		transformAll = false
		isAnyTransHappened = false
		transObj := gvkTransMap[pvcGvk][tIndex]
		transObjNames := sets.String{}
		tStatus := crd.TransformStatus{
			TransformName:        transObj.TransformName,
			Status:               crd.Completed,
			TransformedResources: []crd.Resource{},
		}
		transformResource := crd.Resource{
			GroupVersionKind: pvcGvk,
			Objects:          []string{},
		}
		log.Debugf("Performing transformation %s for PVCs", transObj.TransformName)

		if len(transObj.Resources.Objects) == 0 {
			// If objects are not specified then run transformation for
			// all objects of the PVC i.e. all PVCs in the backup
			transformAll = true
		} else {
			transObjNames.Insert(transObj.Resources.Objects...)
		}

		var PVCToBeExcluded []string

		for dsIndex := range *dataSnapshots {
			transformSpecific = false
			ds := (*dataSnapshots)[dsIndex]

			if ExcludeObjSet.Has(ds.PersistentVolumeClaimName) {
				PVCToBeExcluded = append(PVCToBeExcluded, ds.PersistentVolumeClaimName)
				continue
			}

			if transObjNames.Has(ds.PersistentVolumeClaimName) {
				transformSpecific = true
			}

			// if transformAll and transformSpecific both are false then skip transform and update dataSnapshots
			if !transformAll && !transformSpecific {
				updateDataSnapshots = append(updateDataSnapshots, ds)
				continue
			}

			isAnyTransHappened = true
			obj := unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind(pvcGvk))

			if uMarshalErr := yaml.Unmarshal([]byte(ds.PersistentVolumeClaimMetadata), &obj); uMarshalErr != nil {
				tStatus.Status = crd.Failed
				tStatus.Reason += fmt.Sprintf("Transformation failed "+
					"for %s with error %s;", ds.PersistentVolumeClaimName, uMarshalErr.Error())
				isAnyTransFailed = true
				continue
			}
			transformResource.Objects = append(transformResource.Objects, ds.PersistentVolumeClaimName)
			unsObj := decorator.UnstructResource(obj)
			log.Infof("Transforming PVC %s", ds.PersistentVolumeClaimName)
			transformedPVC, tErr := unsObj.TransformAndDryRun(a, ns, ds.PersistentVolumeClaimMetadata,
				transObj.JSONPatches)

			if tErr != nil {
				tStatus.Status = crd.Failed
				tStatus.Reason += fmt.Sprintf("Transformation failed "+
					"for %s: %s;", ds.PersistentVolumeClaimName, tErr.Error())
				isAnyTransFailed = true
				continue
			}
			// Assign the transformed PVC meta
			ds.PersistentVolumeClaimMetadata = transformedPVC
			updateDataSnapshots = append(updateDataSnapshots, ds)
		}

		// update the existing data snapshots
		*dataSnapshots = updateDataSnapshots

		// merge the new resources to be excluded in the main excludedRes list
		if len(PVCToBeExcluded) > 0 {
			excludedRes = MergeResourceList(excludedRes, []crd.Resource{{
				GroupVersionKind: pvcGvk,
				Objects:          PVCToBeExcluded,
			}})
		}

		if isAnyTransHappened {
			tStatus.TransformedResources = append(tStatus.TransformedResources, transformResource)
			transformStatus = append(transformStatus, tStatus)
		}
	}

	return transformStatus, excludedRes, isAnyTransFailed
}
