package apis

import (
	"io/ioutil"
	"path"

	"github.com/trilioData/k8s-triliovault/internal/helpers"

	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type OperatorSnapshots []OperatorSnapshot

func (op *OperatorSnapshot) PushToTarget(location string) error {
	log.Warn("PushToTarget method is not implemented for single operator snapshot")
	return nil
}

// PushToTarget pushes the snapshot to the given location on the target
// Input:
//		location: Location on which snapshot needs to be pushed
// Output:
//		Error if any
func (opSnapshots *OperatorSnapshots) PushToTarget(location string) (err error) {
	var outStr string

	for i := 0; i < len(*opSnapshots); i++ {
		operator := (*opSnapshots)[i]
		operatorPath := path.Join(location, internal.OperatorBackupDir, operator.OperatorID)

		// Create directory for Operator metadata
		metadataPath := path.Join(operatorPath, internal.MetadataSnapshotDir)
		outStr, err = shell.Mkdir(metadataPath)
		if err != nil {
			log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", metadataPath, outStr)
			return err
		}

		if len(operator.OperatorResources) > 0 {
			// Serialize metadata and write to file
			err = helpers.SerializeStructToFilePath(operator.OperatorResources, metadataPath, internal.MetadataJSON)
			if err != nil {
				log.Errorf("Error while Serializing file at %s", metadataPath)
				return err
			}
		}

		if operator.Helm != nil {
			hs := HelmSnapshots{*operator.Helm}

			err = hs.PushToTarget(operatorPath)
			if err != nil {
				log.Errorf("Error while pushing helm snapshot for operator: %s", err.Error())
				return err
			}
		}

		if len(operator.CustomResources) > 0 {
			// Serialize resource metadata and write to file
			err = helpers.SerializeStructToFilePath(operator.CustomResources, metadataPath, internal.ResourceMetadataJSON)
			if err != nil {
				log.Errorf("Error while writing file at %s", metadataPath)
				return err
			}
		}

		if len(operator.CRDMetadata) > 0 {
			// Serialize crd metadata and write to file
			err = helpers.SerializeStructToFilePath(operator.CRDMetadata, metadataPath, internal.CrdMetadataJSON)
			if err != nil {
				log.Errorf("Error while writing file at %s", metadataPath)
				return err
			}
		}

		if len(operator.DataSnapshots) > 0 {
			// Create and Write metadata of data snapshot
			dataSnapshotPath := path.Join(operatorPath, internal.DataSnapshotDir)
			err = UploadPVCMetadata(operator.DataSnapshots, dataSnapshotPath)
			if err != nil {
				log.Errorf("Error while creating the backup pvc %s at datastore, ERROR: %s", dataSnapshotPath, err.Error())
				return err
			}
		}
	}

	return nil
}

func (op *OperatorSnapshot) PullFromTarget(location string) error {
	log.Warn("PullFromTarget method is not implemented for single operator snapshot")
	return nil
}

// PullFromTarget reads the metadata and data snapshot from target
// Input:
//		location: Location of the backup from where data needs to be read
// Output:
//		Operator snapshot list including metadata and data
//		Error if any
func (opSnapshots *OperatorSnapshots) PullFromTarget(location string) error {
	var (
		operatorSnapshots []OperatorSnapshot
		getOpErr          error
	)

	// Read the directory's names from operator. Each dir represent a backed up operator
	operatorIDs, readErr := internal.ReadChildDir(location)
	if readErr != nil {
		log.Errorf("Error while reading directories from operator backup path %s, Error: %s", location, readErr.Error())
		return readErr
	}
	for i := 0; i < len(operatorIDs); i++ {
		var opSnapshot OperatorSnapshot
		opSnapshot, getOpErr = getOperatorSnapshot(location, operatorIDs[i])
		if getOpErr != nil {
			log.Errorf("Error while getting snapshot of operator %s, Error: %s", operatorIDs[i], getOpErr.Error())
			return getOpErr
		}
		operatorSnapshots = append(operatorSnapshots, opSnapshot)
	}
	*opSnapshots = operatorSnapshots

	return nil
}

// ConvertToCrdSnapshots converts the backed up structures to crd structures
// It removes the metadata and keep names only while converting
// Output:
//		Converted operator snapshot
//		Error if any
func (op *OperatorSnapshot) ConvertToCrdSnapshots() (interface{}, error) {
	var (
		crdOpSnapshot     crd.Operator
		crdOpResource     crd.Resource
		CRDResource       []crd.Resource
		crdCustomResource crd.Resource
	)

	// Convert the Helm snapshot
	if op.Helm != nil {
		convertedHelmIFace, conErr := op.Helm.ConvertToCrdSnapshots()
		if conErr != nil {
			return nil, conErr
		}
		convertedHelm := convertedHelmIFace.(crd.Helm)
		crdOpSnapshot.Helm = &convertedHelm
	}

	// Convert Operator Resources
	for oResIndex := range op.OperatorResources {
		opRes := op.OperatorResources[oResIndex]
		opResNames, resErr := GetResourceNames(&opRes)
		if resErr != nil {
			log.Errorf("Error while getting resource names of operator %s metadata, Error: %s", op.OperatorID, resErr.Error())
			return nil, resErr
		}
		crdOpResource.GroupVersionKind = opRes.GroupVersionKind
		if len(opResNames) > 0 {
			crdOpResource.Objects = opResNames
		}
		crdOpSnapshot.OperatorResources = append(crdOpSnapshot.OperatorResources, crdOpResource)
	}

	// Convert operator custom resources
	for oCusResIndex := range op.CustomResources {
		opCusRes := op.CustomResources[oCusResIndex]
		cusResNames, resErr := GetResourceNames(&opCusRes)
		if resErr != nil {
			return nil, resErr
		}
		crdCustomResource.GroupVersionKind = opCusRes.GroupVersionKind
		if len(cusResNames) > 0 {
			crdCustomResource.Objects = cusResNames
		}
		crdOpSnapshot.CustomResources = append(crdOpSnapshot.CustomResources, crdCustomResource)
	}

	// Add operator CRDs GVK and name to OperatorResources
nextCRD:
	for _, CRD := range op.CRDMetadata {
		var crdRes crd.Resource
		obj := unstructured.Unstructured{}

		if uMarshalErr := yaml.Unmarshal([]byte(CRD), &obj); uMarshalErr != nil {
			return nil, uMarshalErr
		}

		crdRes.GroupVersionKind = crd.GroupVersionKind(obj.GroupVersionKind())
		crdRes.Objects = append(crdRes.Objects, obj.GetName())

		for index := range CRDResource {
			if CRDResource[index].GroupVersionKind == crdRes.GroupVersionKind {
				CRDResource[index].Objects = append(CRDResource[index].Objects, crdRes.Objects...)
				continue nextCRD
			}
		}
		CRDResource = append(CRDResource, crdRes)
	}
	crdOpSnapshot.OperatorResources = append(crdOpSnapshot.OperatorResources, CRDResource...)

	// Assign OperatorID
	crdOpSnapshot.OperatorID = op.OperatorID
	// Assign data snapshot
	crdOpSnapshot.DataSnapshots = op.DataSnapshots
	if len(op.Warnings) > 0 {
		crdOpSnapshot.Warnings = op.Warnings
	}

	if op.Helm != nil {
		s, hErr := op.Helm.ConvertToCrdSnapshots()
		if hErr != nil {
			return nil, hErr
		}

		snap := s.(crd.Helm)
		crdOpSnapshot.Helm = &snap
	}

	return crdOpSnapshot, nil
}

// ConvertToCrdSnapshots converts the backed up structures to crd structures
// It removes the metadata and keep names only while converting
// Output:
//		Converted operator snapshot
//		Error if any
func (opSnapshots *OperatorSnapshots) ConvertToCrdSnapshots() (interface{}, error) {
	var crdOpSnapshots []crd.Operator

	for i := range *opSnapshots {
		opSnapshot := (*opSnapshots)[i]
		convertedOpIFace, cErr := opSnapshot.ConvertToCrdSnapshots()
		if cErr != nil {
			log.Errorf("Error while converting operator %s to crd required format, Error: %s", opSnapshot.OperatorID, cErr.Error())
			return nil, cErr
		}
		crdOpSnapshots = append(crdOpSnapshots, convertedOpIFace.(crd.Operator))
	}

	return crdOpSnapshots, nil
}

// Transform performs the operator transformation
func (op *OperatorSnapshot) Transform(a *kube.Accessor, ns string,
	cGvkTrans map[crd.GroupVersionKind][]crd.CustomTransform, helmRelTrans map[string]crd.HelmTransform,
	excludePolicy ExcludePolicy, backupLocation string) (*crd.RestoreOperator, bool) {
	var (
		dsTransformStatus, msTransformStatus                      []crd.TransformStatus
		isAnyDSTransFailed, isAnyMSTransFailed, isHelmTransFailed bool
		opRestore                                                 = crd.RestoreOperator{
			Snapshot: new(crd.Operator),
		}
		opStatus    crd.ComponentStatus
		excludedRes []crd.Resource
	)

	opStatus.Phase = crd.RestoreValidation

	if len(op.DataSnapshots) > 0 {
		// Transform data
		dsTransformStatus, excludedRes, isAnyDSTransFailed = transformDataSnapshots(a, ns, &op.DataSnapshots, cGvkTrans, excludePolicy)
		opStatus.TransformStatus = append(opStatus.TransformStatus, dsTransformStatus...)
		opStatus.ExcludedResources = append(opStatus.ExcludedResources, excludedRes...)
	}

	if len(op.OperatorResources) > 0 {
		// Transform meta
		msTransformStatus, excludedRes, isAnyMSTransFailed = transformMetaSnapshots(a, ns, op.OperatorResources, cGvkTrans, excludePolicy)
		opStatus.TransformStatus = append(opStatus.TransformStatus, msTransformStatus...)
		opStatus.ExcludedResources = append(opStatus.ExcludedResources, excludedRes...)
	}

	if op.Helm != nil {
		if hTrans, ok := helmRelTrans[op.Helm.Release]; ok {
			helmTransStatus := crd.TransformStatus{
				TransformName: hTrans.TransformName,
				Status:        crd.Completed,
			}
			tErr := op.Helm.Transform(a.GetRestConfig(), ns, hTrans, backupLocation)
			if tErr != nil {
				helmTransStatus.Reason = tErr.Error()
				helmTransStatus.Status = crd.Failed
				isHelmTransFailed = true
			}
			opStatus.TransformStatus = append(opStatus.TransformStatus, helmTransStatus)
		}
	}

	if isAnyDSTransFailed || isAnyMSTransFailed || isHelmTransFailed {
		opStatus.PhaseStatus = crd.Failed
	}

	opRestore.Status = &opStatus
	opRestore.Snapshot.OperatorID = op.OperatorID
	return &opRestore, isAnyDSTransFailed || isAnyMSTransFailed || isHelmTransFailed
}

func (opSnapshots *OperatorSnapshots) Transform(a *kube.Accessor, ns string,
	cGvkTrans map[crd.GroupVersionKind][]crd.CustomTransform, helmRelTrans map[string]crd.HelmTransform,
	excludePolicy ExcludePolicy, backupLocation string) ([]crd.RestoreOperator, bool) {
	var (
		isAnyOpTransFailed = false
		allOpRestore       []crd.RestoreOperator
	)

	for i := range *opSnapshots {
		opSnapshot := (*opSnapshots)[i]
		opRestore, isOpTransFailed := opSnapshot.Transform(a, ns, cGvkTrans, helmRelTrans, excludePolicy, backupLocation)
		allOpRestore = append(allOpRestore, *opRestore)
		if isOpTransFailed {
			isAnyOpTransFailed = true
		}
	}

	return allOpRestore, isAnyOpTransFailed
}

// getOperatorSnapshot processes then fetching of single operator snapshot from target
// Input:
//		operatorBasePath: Operator backup directory path from target
//		operatorName: Name of the operator
// Output:
//		Fetched OperatorSnapshot from the taget
// 		Error if any
func getOperatorSnapshot(operatorBasePath, operatorID string) (
	operatorSnapshot OperatorSnapshot, err error) {
	var crdMetadata []string

	// Assign OperatorID
	operatorSnapshot.OperatorID = operatorID

	// Metadata and data snapshot paths for given operator
	operatorMetadataPath := path.Join(operatorBasePath, operatorID, internal.MetadataSnapshotDir)
	operatorDataPath := path.Join(operatorBasePath, operatorID, internal.DataSnapshotDir)

	// Operator helm Metadata snapshot
	hs := &HelmSnapshots{}
	// Read helm snapshots from target
	helmBackupPath := path.Join(operatorBasePath, operatorID, internal.HelmBackupDir)
	if internal.DirExists(helmBackupPath) {
		if err = hs.PullFromTarget(helmBackupPath); err != nil {
			log.Errorf("Error occurred while pulling from target: %s", err.Error())
			return operatorSnapshot, err
		}
		for hIndex := range *hs {
			h := *hs
			// There will always be at max one helm snapshot
			operatorSnapshot.Helm = &h[hIndex]
		}
	}

	// Operator Custom Resource Metadata
	resourceMetadataPath := path.Join(operatorMetadataPath, internal.ResourceMetadataJSON)
	if internal.FileExists(resourceMetadataPath) {
		customResources, unmarshalErr := unMarshalMeta(resourceMetadataPath)
		if unmarshalErr != nil {
			log.Errorf("Error while unmarshalling operator's custom resources metadata file %s", resourceMetadataPath)
			return operatorSnapshot, unmarshalErr
		}
		operatorSnapshot.CustomResources = customResources
	}

	// Operator CRDs Metadata
	opCrdPath := path.Join(operatorMetadataPath, internal.CrdMetadataJSON)
	if internal.FileExists(opCrdPath) {
		crdMetaByte, readErr := ioutil.ReadFile(opCrdPath)
		if readErr != nil {
			log.Errorf("Error while reading operator's CRD file %s", opCrdPath)
			return operatorSnapshot, readErr
		}
		unmarshalErr := yaml.Unmarshal(crdMetaByte, &crdMetadata)
		if unmarshalErr != nil {
			log.Errorf("Error while unmarshalling operator's CRD file %s", opCrdPath)
			return operatorSnapshot, unmarshalErr
		}
		operatorSnapshot.CRDMetadata = append(operatorSnapshot.CRDMetadata, crdMetadata...)
	}

	// Operator Resources Metadata
	metadataPath := path.Join(operatorMetadataPath, internal.MetadataJSON)
	if internal.FileExists(metadataPath) {
		opResources, unmarshalErr := unMarshalMeta(metadataPath)
		if unmarshalErr != nil {
			log.Errorf("Error while unmarshalling operator resources metadata file %s", metadataPath)
			return operatorSnapshot, unmarshalErr
		}
		operatorSnapshot.OperatorResources = opResources
	}

	// Operator DataSnapshot
	if internal.DirExists(operatorDataPath) {
		dataSnapshots, getDsErr := getDataSnapshots(operatorDataPath)
		if getDsErr != nil {
			log.Errorf("Error while getting data snapshot for operator %s from path %s", operatorID, operatorDataPath)
			return operatorSnapshot, getDsErr
		}
		operatorSnapshot.DataSnapshots = dataSnapshots
	}

	return operatorSnapshot, nil
}
