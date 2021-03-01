package apis

import (
	"errors"
	"io/ioutil"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/trilioData/k8s-triliovault/internal/helpers"

	helmutils "github.com/trilioData/k8s-triliovault/internal/helm_utils"

	log "github.com/sirupsen/logrus"
	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type RestoreConf struct {
	StorageBackend crd.HelmStorageBackend `json:"storageBackend"`
	HelmVersion    crd.HelmVersion        `json:"helmVersion"`
	Revision       string                 `json:"revision"`
}

// ExcludePolicy type decides what all resources are to be excluded at the time of restore
// It contains DisableIgnore flag: to override default ignore list, ValidationIgnoreList: default ignore list
// and ExcludeResourceMap: map of resources specified in the Restore CR to be excluded
type ExcludePolicy struct {
	// DisableIgnore disables the default restore ignores list
	DisableIgnore bool

	// ValidationIgnoreList is the default restore ignore list
	ValidationIgnoreList sets.String

	//  ExcludeResourceMap is the map of resources given in the Restore CR to be excluded
	ExcludeResourceMap map[crd.GroupVersionKind]sets.String
}

type TVKMeta struct {
	TVKInstanceUID string `json:"tvkInstanceUID"`
	TVKVersion     string `json:"tvkVersion"`
}

func unMarshalMeta(metadataPath string) (extractedMetaData []ComponentMetadata, err error) {
	// unMarshalMeta reads metadata from file and puts it to structure
	// Input:
	//		metadataPath: Metadata path from where unmarshal needs to be done
	// Output:
	//		extractedMetaData: List of all components GVK and metadata
	//		error: Error if any

	metaByte, readErr := ioutil.ReadFile(metadataPath)
	if readErr != nil {
		log.Errorf("Error while reading file %s", metadataPath)
		return nil, readErr
	}
	unmarshalErr := yaml.Unmarshal(metaByte, &extractedMetaData)
	if unmarshalErr != nil {
		log.Errorf("Error while unmarshalling file %s", metadataPath)
		return nil, unmarshalErr
	}

	return extractedMetaData, nil
}

func getDataSnapshots(dsPath string) (dataSnapshots []crd.DataSnapshot, err error) {
	// getDataSnapshots reads the pvc.json for each data snapshot and puts it in structure
	// Input:
	//		dsPath: DataSnapshots path where the snapshots are stored on the target
	// Output:
	//		dataSnapshots: DataSnapshot list
	//		err: Error if any

	// Get the PVC names from the child dirs
	pvcNames, readErr := internal.ReadChildDir(dsPath)
	if readErr != nil {
		log.Errorf("Error while reading directory names from path %s", dsPath)
		return dataSnapshots, readErr
	}

	// Iterate over all PVCs and put the pvc.json and pod container map in structure
	for i := 0; i < len(pvcNames); i++ {
		var podContainerMap []crd.PodContainers
		pvcName := pvcNames[i]
		customMetadata, readPVCErr := ioutil.ReadFile(path.Join(dsPath, pvcName, internal.PVCJSON))
		if readPVCErr != nil {
			log.Errorf("Error while reading file %s", path.Join(dsPath, pvcName, internal.PVCJSON))
			return dataSnapshots, readPVCErr
		}

		// Read the pod container map
		pcMap, readMapErr := ioutil.ReadFile(path.Join(dsPath, pvcName, internal.PodContainerMapFile))
		if readMapErr != nil {
			log.Errorf("Error while reading file %s", path.Join(dsPath, pvcName, internal.PodContainerMapFile))
			return dataSnapshots, readMapErr
		}

		// Unmarshal the pod container map
		if uMarshalErr := yaml.Unmarshal(pcMap, &podContainerMap); uMarshalErr != nil {
			log.Errorf("Error while unmarshalling file %s", path.Join(dsPath, pvcName, internal.PodContainerMapFile))
			return nil, uMarshalErr
		}

		// To get the location of the pvc, trim the target path and take the path from app UID
		pvcRelativePath := path.Join(strings.TrimPrefix(dsPath, internal.DefaultDatastoreBase+"/"), pvcName)
		ds := crd.DataSnapshot{
			Location:                      pvcRelativePath,
			PersistentVolumeClaimMetadata: string(customMetadata),
			PodContainersMap:              podContainerMap,
			PersistentVolumeClaimName:     pvcName,
		}
		dataSnapshots = append(dataSnapshots, ds)
	}

	return dataSnapshots, nil
}

func GetResourceNames(metadata *ComponentMetadata) ([]string, error) {
	// GetResourceNames returns the resource names list
	// Unmarshal the metadata using gvk, read the name and append it to the list
	// Input:
	//		metadata: List of GVK and metadata list
	// Output:
	//		List of all resource's name
	//		Error if any
	var resNames []string

	gvk := metadata.GroupVersionKind
	for i := range metadata.Metadata {
		md := metadata.Metadata[i]
		obj := unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind(gvk))
		if uMarshalErr := yaml.Unmarshal([]byte(md), &obj); uMarshalErr != nil {
			return nil, uMarshalErr
		}
		resNames = append(resNames, obj.GetName())
	}

	return resNames, nil
}

func ConvertSnapshots(snap interface{}) (interface{}, error) {
	var snapMgr SnapshotMgr

	switch s := snap.(type) {
	case *HelmSnapshot:
		var helmSnapshots HelmSnapshots
		helmSnapshots = append(helmSnapshots, *s)
		snapMgr = &helmSnapshots
	case *OperatorSnapshot:
		var opSnapshots OperatorSnapshots
		opSnapshots = append(opSnapshots, *s)
		snapMgr = &opSnapshots
	case *CustomSnapshot:
		snapMgr = s
	default:
		return nil, errors.New("error Occurred: Invalid parameter")
	}

	v1alpha1Snapshots, cErr := snapMgr.ConvertToCrdSnapshots()
	if cErr != nil {
		return nil, cErr
	}
	return v1alpha1Snapshots, nil
}

// Backup PVC metadata
func UploadPVCMetadata(dataSnapshots []crd.DataSnapshot, dataSnapshotPath string) (err error) {

	for i := 0; i < len(dataSnapshots); i++ {
		dataSnapshot := dataSnapshots[i]

		dataPath := path.Join(dataSnapshotPath, dataSnapshot.PersistentVolumeClaimName)
		_, err = shell.Mkdir(dataPath)
		if err != nil {
			log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", dataPath, err.Error())
			return err
		}

		pvcPath := path.Join(dataPath, internal.PVCJSON)
		err = shell.WriteToFile(pvcPath, dataSnapshot.PersistentVolumeClaimMetadata)
		if err != nil {
			log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", pvcPath, err.Error())
			return err
		}

		err = helpers.SerializeStructToFilePath(dataSnapshot.PodContainersMap, dataPath, internal.PodContainerMapFile)
		if err != nil {
			log.Errorf("Error while Serializing pod-container map at %s", dataPath)
			return err
		}
	}

	return nil
}

// GetLatestRevAndReL returns the latest release storage backend string (from secret or config map) and its revision
// Internal Helm manager, storage backend are initialized and using the internal library functions,
// all the revisions are converted to release object and the latest one is returned with its revision
func GetLatestRevAndReL(hlmMgr helmutils.HelmMgr, componentMeta *ComponentMetadata,
	storageObjectKind string, preHelmVersion crd.HelmVersion) (latestRev int32, latestRel interface{}, err error) {
	var (
		latestRevision     int32
		latestRelInterface interface{}
		relInterface       interface{}
	)

	for hIndex := range componentMeta.Metadata {
		// Get the release using storage backend i.e. Secret/Configmap
		relInterface, err = hlmMgr.GetReleaseFromStorageObj(componentMeta.Metadata[hIndex], storageObjectKind)
		if err != nil {
			log.Errorf("couldn't get the release %s", err.Error())
			return 0, nil, err
		}

		// Get the revision and content from the release
		_, rlsVersion := hlmMgr.GetReleaseContentFromRelease(relInterface)
		if latestRevision == 0 || latestRevision < rlsVersion {
			latestRevision = rlsVersion
			latestRelInterface = relInterface
		}
	}

	return latestRevision, latestRelInterface, nil
}

// GetExcludedResStatus creates a Resource type for internal ComponentMetadata type
// 		input:
//			ComponentMetadata: GVK and metaData array
//		output:
//			Resource: GVK and objectName array
func GetExcludedResStatus(compMetaData *ComponentMetadata) (crd.Resource, error) {
	var (
		excludeRes = crd.Resource{GroupVersionKind: compMetaData.GroupVersionKind}
		resName    []string
		err        error
	)
	if resName, err = GetResourceNames(compMetaData); err != nil {
		log.Error("failed to exclude default restore ignore resources")
		return excludeRes, err
	}
	excludeRes.Objects = resName
	return excludeRes, err
}

//nolint:dupl // func params type and out param are different
// MergeResourceList combines 2 Resource lists into one unique based on GVK and their objects
func MergeResourceList(from, to []crd.Resource) []crd.Resource {
	resMap := make(map[crd.GroupVersionKind]sets.String)
	var result []crd.Resource

	for i := range to {
		if l, ok := resMap[to[i].GroupVersionKind]; ok {
			resMap[to[i].GroupVersionKind] = l.Insert(to[i].Objects...)
			continue
		}

		resMap[to[i].GroupVersionKind] = sets.NewString(to[i].Objects...)
	}

	for i := range from {
		if l, ok := resMap[from[i].GroupVersionKind]; ok {
			resMap[from[i].GroupVersionKind] = l.Insert(from[i].Objects...)
			continue
		}

		resMap[from[i].GroupVersionKind] = sets.NewString(from[i].Objects...)
	}

	for gvk, metaList := range resMap {
		result = append(result, crd.Resource{GroupVersionKind: gvk, Objects: metaList.List()})
	}

	return result
}

//nolint:dupl // func params type and out param are different
// MergeCompMetaLists combines 2 component metadata lists into one unique based on GVK and their metadata
func MergeCompMetaLists(from, to []ComponentMetadata) []ComponentMetadata {
	resMap := make(map[crd.GroupVersionKind]sets.String)
	var result []ComponentMetadata

	for i := range to {
		if l, ok := resMap[to[i].GroupVersionKind]; ok {
			resMap[to[i].GroupVersionKind] = l.Insert(to[i].Metadata...)
			continue
		}

		resMap[to[i].GroupVersionKind] = sets.NewString(to[i].Metadata...)
	}

	for i := range from {
		if l, ok := resMap[from[i].GroupVersionKind]; ok {
			resMap[from[i].GroupVersionKind] = l.Insert(from[i].Metadata...)
			continue
		}

		resMap[from[i].GroupVersionKind] = sets.NewString(from[i].Metadata...)
	}

	for gvk, metaList := range resMap {
		result = append(result, ComponentMetadata{GroupVersionKind: gvk, Metadata: metaList.List()})
	}

	return result
}
