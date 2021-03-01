package apis

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	log "github.com/sirupsen/logrus"
	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	helmutils "github.com/trilioData/k8s-triliovault/internal/helm_utils"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

type HelmSnapshots []HelmSnapshot

func (h *HelmSnapshot) PushToTarget(location string) error {
	log.Warn("PushToTarget method is not implemented for single helm snapshot")
	return nil
}

// PushToTarget pushes the snapshot to the given location on the target
// Input:
//		location: Location on which snapshot needs to be pushed
// Output:
//		Error if any
func (hs *HelmSnapshots) PushToTarget(location string) error {
	for i := 0; i < len(*hs); i++ {
		helm := (*hs)[i]

		helmBasePath := path.Join(location, internal.HelmBackupDir)
		helmReleasePath := path.Join(helmBasePath, helm.Release)

		log.Infof("Creating the directory release:[%s] at path [%v]", helm.Release, helmReleasePath)
		outStr, err := shell.Mkdir(helmReleasePath)
		if err != nil {
			log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", helmReleasePath, outStr)
			return err
		}

		// Adding release-config.json to the helm backup base path
		releaseConf := map[string]string{internal.HelmVersionKey: string(helm.Version),
			internal.HelmStorageBackendKey: string(helm.StorageBackend),
			internal.HelmRevisionKey:       strconv.Itoa(int(helm.Revision))}

		if err = helpers.SerializeStructToFilePath(releaseConf, helmReleasePath, internal.ReleaseConfFile); err != nil {
			log.Errorf("Error while Serializing file at %s", helmReleasePath)
			return err
		}

		// Serialize helm component metadata and write to file
		// TODO: remove null check after mandatory
		if helm.Metadata != nil {
			// Create directory for Helm Charts metadata
			helmMetadataPath := path.Join(helmReleasePath, internal.MetadataSnapshotDir)
			outStr, err = shell.Mkdir(helmMetadataPath)
			if err != nil {
				log.Errorf("Error while creating the directory %s at datastore, ERROR: %s", helmMetadataPath, outStr)
				return err
			}

			err = helpers.SerializeStructToFilePath(helm.Metadata, helmMetadataPath, internal.MetadataJSON)
			if err != nil {
				log.Errorf("Error while serializing file at %s", helmMetadataPath)
				return err
			}

			log.Infof("Backed up helm metadata for release:[%s] at path [%v]", helm.Release, helmMetadataPath)
		}

		// Upload helm sub-charts for all revisions
		tempDependencyDir := path.Join(internal.TmpDir, helm.Release)
		_, err = os.Stat(tempDependencyDir)
		if err == nil {
			if err = shell.CopyDir(path.Join(tempDependencyDir, internal.HelmDependencyDir), helmReleasePath); err != nil {
				log.Error(err.Error())
				log.Errorf("Failed to upload helm dependency sub-charts at %s for release %s",
					path.Join(helmReleasePath, internal.HelmDependencyDir), helm.Release)
				return err
			}

			log.Infof("Backed up helm sub-charts for release:[%s] at path [%v]", helm.Release,
				path.Join(helmReleasePath, internal.HelmDependencyDir))
		}

		// Create and Write metadata of data snapshot
		helmDataSnapshotPath := path.Join(helmReleasePath, internal.DataSnapshotDir)
		err = UploadPVCMetadata(helm.DataSnapshots, helmDataSnapshotPath)
		if err != nil {
			log.Errorf("Error while creating the backup pvc %s at datastore, ERROR: %s", helmDataSnapshotPath, outStr)
			return err
		}
	}

	return nil
}

func (h *HelmSnapshot) PullFromTarget(location string) error {
	log.Warn("PullFromTarget method is not implemented for single helm snapshot")
	return nil
}

// PullFromTarget reads the metadata and data snapshot from target
// Input:
//		location: Location of the backup from where data needs to be read
// Output:
//		Helm snapshot list including metadata and data
//		Error if any
func (hs *HelmSnapshots) PullFromTarget(location string) error {
	var (
		helmSnapshots []HelmSnapshot
		getHelmErr    error
	)

	// Get backed up release directories. Directory name represents the release name
	releaseNames, readErr := internal.ReadChildDir(location)
	if readErr != nil {
		log.Errorf("Error while reading directories from helm backup path %s, Error: %s", location, readErr.Error())
		return readErr
	}
	for i := 0; i < len(releaseNames); i++ {
		release := releaseNames[i]
		log.Debugf("Getting metadata and data for release %s", release)
		helmRelBackupPath := path.Join(location, release)
		if internal.DirExists(helmRelBackupPath) {
			var helmSnapshot HelmSnapshot
			helmSnapshot, getHelmErr = getHelmSnapshot(helmRelBackupPath, release)
			if getHelmErr != nil {
				log.Errorf("Error while getting snapshot of release %s, Error: %s", release, getHelmErr.Error())
				return getHelmErr
			}
			helmSnapshots = append(helmSnapshots, helmSnapshot)
		}
	}
	*hs = helmSnapshots

	return nil
}

// ConvertToCrdSnapshots converts the backed up structures to crd structures
// It removes the metadata and keep names only while converting
// Output:
//		Converted helm snapshot
//		Error if any
func (h *HelmSnapshot) ConvertToCrdSnapshots() (interface{}, error) {
	var (
		crdHS        crd.Helm
		crdResources []crd.Resource
	)

	resMetaList := MergeCompMetaLists(h.ReleaseResources, []ComponentMetadata{*h.Metadata})

	for i := range resMetaList {
		var crdRes crd.Resource
		resource := resMetaList[i]

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

	// Assign the vales to crd helm snapshot structure
	crdHS.Resources = crdResources
	crdHS.DataSnapshots = h.DataSnapshots
	crdHS.Release = h.Release
	crdHS.NewRelease = h.NewRelease
	crdHS.Revision = h.Revision
	crdHS.StorageBackend = h.StorageBackend
	crdHS.Version = h.Version
	if len(h.Warnings) > 0 {
		crdHS.Warnings = h.Warnings
	}

	return crdHS, nil
}

// Transform performs the helm transformation
func (hs *HelmSnapshots) Transform(a *kube.Accessor, restoreNs string, hReleaseTrans map[string]crd.HelmTransform,
	backupLocation string) ([]crd.RestoreHelm, bool) {

	var (
		TrHelmCharts      []crd.RestoreHelm
		isTransformFailed bool
	)

	for hIndex := range *hs {
		h := &(*hs)[hIndex]
		if hTrans, ok := hReleaseTrans[h.Release]; ok {
			log.Infof("Performing transformation for helm release: [%s]", h.Release)
			resHelm := crd.RestoreHelm{Status: new(crd.ComponentStatus),
				Snapshot: new(crd.Helm)}
			ts := crd.TransformStatus{
				TransformName: hTrans.TransformName,
				Status:        crd.Completed,
			}

			err := h.Transform(a.GetRestConfig(), restoreNs, hTrans, backupLocation)
			if err != nil {
				isTransformFailed = true
				ts.Reason = err.Error()
				ts.Status = crd.Failed
				resHelm.Status.PhaseStatus = crd.Failed
			}

			resHelm.Status.TransformStatus = append(resHelm.Status.TransformStatus, ts)
			resHelm.Snapshot.Release = h.Release
			resHelm.Snapshot.NewRelease = h.NewRelease
			resHelm.Snapshot.Version = h.Version
			resHelm.Snapshot.Revision = h.Revision
			resHelm.Snapshot.StorageBackend = h.StorageBackend
			TrHelmCharts = append(TrHelmCharts, resHelm)
		}
	}

	return TrHelmCharts, isTransformFailed
}

func (h *HelmSnapshot) Transform(restConfig *rest.Config, ns string, tf crd.HelmTransform, backupLocation string) error {

	hlmMgr, err := helmutils.NewHelmManager(restConfig, ns)
	if err != nil {
		return err
	}

	defaultHsb, customHsb, nErr := helmutils.NewHelmStorageBackends(hlmMgr)
	if nErr != nil {
		log.Warnf("error while initializing storage backend for helm: %s", nErr.Error())
		return nErr
	}

	hsb := defaultHsb

	if h.StorageBackend == customHsb.GetKind() {
		hsb = customHsb
	}

	if h.NewRelease == "" {
		newHelmRelName, getRelNmErr := helpers.GetUniqueHelmRelName(h.Release, ns, hsb)
		if getRelNmErr != nil {
			return getRelNmErr
		}
		h.NewRelease = newHelmRelName
	}

	// get the latest release
	latestRev, latestRel, gErr := GetLatestRevAndReL(hlmMgr, h.Metadata, string(hsb.GetKind()), h.Version)
	if gErr != nil {
		return gErr
	}

	// load the dependency sub-charts if required
	subChartLoadedRel, lErr := hlmMgr.LoadDependencies(latestRel, latestRev, backupLocation)
	if lErr != nil {
		return lErr
	}

	// add transform --set values in old release
	tfRel, tErr := hlmMgr.TransformRelease(subChartLoadedRel, tf)
	if tErr != nil {
		log.Errorf("failed parsing --set data, error: %s", tErr.Error())
		return tErr
	}

	// render release with transform values and new name
	if _, lErr = hlmMgr.ModifyRelease(tfRel, h.NewRelease, ns); lErr != nil {
		log.Errorf("failed to modify the release with transform values and new name, error: %s", lErr.Error())
		return lErr
	}

	return nil
}

func (hs *HelmSnapshots) ConvertToCrdSnapshots() (interface{}, error) {
	// ConvertToCrdSnapshots converts the backed up structures to crd structures
	// It removes the metadata and keep names only while converting
	// Output:
	//		Converted helm snapshot
	//		Error if any
	var crdHelmSnapshots []crd.Helm

	for i := range *hs {
		helmSnapshot := (*hs)[i]
		convertedHSIFace, cErr := helmSnapshot.ConvertToCrdSnapshots()
		if cErr != nil {
			log.Errorf("Error while converting helm %s to crd required format, Error: %s", helmSnapshot.Release, cErr.Error())
			return nil, cErr
		}
		crdHelmSnapshots = append(crdHelmSnapshots, convertedHSIFace.(crd.Helm))
	}

	return crdHelmSnapshots, nil
}

func getHelmSnapshot(helmReleasePath, helmRelease string) (helmSnapshot HelmSnapshot, err error) {
	// getHelmSnapshot gets the helm application content from target
	// Input:
	//		helmReleasePath: Helm application path on the target
	//		helmRelease: Helm application release name
	// Output:
	//		Fetched HelmSnapshot from the target
	//		Error if any
	var (
		releaseConf  RestoreConf
		helmMetadata ComponentMetadata
	)

	// Get the helm metadata path
	helmMetadataPath := path.Join(helmReleasePath, internal.MetadataSnapshotDir, internal.MetadataJSON)
	// Get the helm data snapshot path
	helmDataPath := path.Join(helmReleasePath, internal.DataSnapshotDir)
	// Get the release configuration path. Release conf contains the
	// helm version and helm storage backend
	helmReleaseConfPath := path.Join(helmReleasePath, internal.ReleaseConfFile)

	// Read the release conf to get the storage backend and version\
	if internal.FileExists(helmReleaseConfPath) {
		releaseConfBytes, readErr := ioutil.ReadFile(helmReleaseConfPath)
		if readErr != nil {
			log.Errorf("Error while reading release conf file %s", helmReleaseConfPath)
			return HelmSnapshot{}, readErr
		}
		unmarshalErr := yaml.Unmarshal(releaseConfBytes, &releaseConf)
		if unmarshalErr != nil {
			log.Errorf("Error while unmarshalling release conf file %s", helmReleaseConfPath)
			return HelmSnapshot{}, unmarshalErr
		}
	} else {
		return HelmSnapshot{}, fmt.Errorf("couldn't locate the helm "+
			"release configuration %s", helmReleaseConfPath)
	}
	log.Debugf("releaseConf %+v", releaseConf)

	// Read the metadata snapshot
	if internal.FileExists(helmMetadataPath) {
		metaBytes, metaReadErr := ioutil.ReadFile(helmMetadataPath)
		if metaReadErr != nil {
			log.Errorf("Error while reading metadata file %s", helmMetadataPath)
			return HelmSnapshot{}, metaReadErr
		}
		unmarshalMetaErr := yaml.Unmarshal(metaBytes, &helmMetadata)
		if unmarshalMetaErr != nil {
			log.Errorf("Error while unmarshalling metadata file %s", helmMetadataPath)
			return HelmSnapshot{}, unmarshalMetaErr
		}
	} else {
		log.Warnf("couldn't locate the helm metadata %s", helmMetadataPath)
	}

	// Get the helm application data snapshot from the target
	if internal.DirExists(helmDataPath) {
		dataSnapshots, getDsErr := getDataSnapshots(helmDataPath)
		if getDsErr != nil {
			log.Errorf("Error while getting data snapshots of helm release %s from path %s", helmRelease, helmDataPath)
			return helmSnapshot, getDsErr
		}
		helmSnapshot.DataSnapshots = dataSnapshots
	}

	// Assign the extracted information from target to helm snapshot object
	helmSnapshot.Release = helmRelease
	helmSnapshot.StorageBackend = releaseConf.StorageBackend
	helmSnapshot.Version = releaseConf.HelmVersion
	helmSnapshot.Metadata = &helmMetadata
	revision, _ := strconv.ParseInt(releaseConf.Revision, 10, 32)
	helmSnapshot.Revision = int32(revision)

	return helmSnapshot, nil
}
