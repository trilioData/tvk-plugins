package helpers

import (
	"context"
	"crypto/md5" // #nosec
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	helmutils "github.com/trilioData/k8s-triliovault/internal/helm_utils"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
)

type WarningMap map[string]string

var WarningTypesList = []string{internal.ModifiedResourceWarning, internal.NotSupportedWarning, internal.DependantResourceWarning,
	internal.PodNotRunningWarning, internal.DependentResourceNotFoundWarning, internal.DependentCRDNotFoundWarning,
	internal.HostNetworkWarning, internal.HostPortWarning, internal.NodeSelectorWarning, internal.NodeAffinityWarning,
	internal.NodePortWarning}

// ValidateDataStorePath function validates if the DataStore where the backup
// or restore needs to be taken is mounted or not
// Input:
// 		dataStorePath: DataStore path to validate if it's mount point?
// Output:
// 		isMountPoint: true if given path is a mount point else false.
// 		errStr: error string if command execution fails else stdout.
// 		err: non-nil error if command execution failed.
func ValidateDataStorePath(dataStorePath string) (isMountPoint bool, errStr string, err error) {
	cmd := fmt.Sprintf("mountpoint %s", dataStorePath)
	log.Debugf("Mount point command: %s", cmd)
	outStruct, err := shell.RunCmd(cmd)
	if err != nil {
		errStr = outStruct.Out
		log.Errorf("DataStore path validation failed %s", outStruct.Out)
		return isMountPoint, errStr, err
	}

	if outStruct.ExitCode == 0 {
		log.Debugf("DataStore path validation successful %s", outStruct.Out)
		isMountPoint, errStr, err = true, outStruct.Out, nil
		return isMountPoint, errStr, err
	}
	errStr, err = outStruct.Out, nil

	return isMountPoint, errStr, err
}

// GetRelativeBackupPath function gets the relative backup path from restore object
// Input:
//		restoreObj: Restore object
// 		trilioResourceNs: Namespace where trilio resources are installed
// 		kubeAccessor: Accessor to get the object from k8s env
// Output:
// 		relBackupLocation: Relative backup location
// 		error: Error if any
func GetRelativeBackupPath(restoreObj *crd.Restore, trilioResourceNs string,
	kubeAccessor *kube.Accessor) (relBackupLocation string, err error) {
	// Check the type of the restore to get the location
	if restoreObj.Spec.Source.Type == crd.LocationSource {
		// Validate and fill up the restore object from the target
		relBackupLocation = restoreObj.Spec.Source.Location
	} else {
		backupObj, getBkErr := kubeAccessor.GetBackup(restoreObj.Spec.Source.Backup.Name,
			trilioResourceNs)
		if getBkErr != nil {
			log.Errorf("couldn't get the backup object")
			return "", getBkErr
		}
		if backupObj.Status.Location == "" {
			relBackupLocation = path.Join(string(backupObj.Spec.BackupPlan.UID), string(backupObj.UID))
		} else {
			relBackupLocation = backupObj.Status.Location
		}
	}

	return relBackupLocation, nil
}

// ValidateBackupLocation validates the backup location i.e. location exists or not
// Input:
//		backupLocation: Backup location which needs to be validated
// Output:
//		Error if any
func ValidateBackupLocation(backupLocation string) (err error) {
	log.Infof("Checking if location present: %s", backupLocation)
	if _, err := os.Stat(backupLocation); os.IsNotExist(err) {
		log.Errorf("location not present %s", err.Error())
		return err
	}
	log.Infof("Location %s present", backupLocation)
	return nil
}

// GetBackupLocation returns the location from the target where the backup is stored
func GetBackupLocation(restoreObj *crd.Restore, trilioResNs string, kubeAccessor *kube.Accessor) (location string, err error) {

	dataStorePath := internal.DefaultDatastoreBase
	// Validate if mount point is present i.e. Check if target is mounted
	isMountPoint, outStr, valDsErr := ValidateDataStorePath(dataStorePath)
	if valDsErr != nil {
		return "", fmt.Errorf("error while validating the mountpoint of "+
			"the datastore %s: %s", dataStorePath, outStr)
	}
	if !isMountPoint {
		return "", fmt.Errorf("could not connect to datastore mount path, invalid path: %s", outStr)
	}
	// Get the backup relative location
	relBackupLocation, relBkErr := GetRelativeBackupPath(restoreObj, trilioResNs, kubeAccessor)
	if relBkErr != nil {
		return "", relBkErr
	}
	// Absolute backup location
	backupLocation := path.Join(dataStorePath, relBackupLocation)

	return backupLocation, nil
}

// GetDiscoveryClient initializes the discovery client using the rest config.
// It returns empty/nil discovery client when err != nil in the initialization and prints the error
func GetDiscoveryClient(restConfig *rest.Config) discovery.DiscoveryClient {
	discClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		log.Error(err, "error while getting the discovery client for the rest config")
		return discovery.DiscoveryClient{}
	}
	return *discClient
}

func GetHash(appComponent, componentIdentifier, pvcName string) string {
	str := fmt.Sprintf("%s-%s-%s", appComponent, componentIdentifier, pvcName)
	h := md5.New() // #nosec
	_, _ = h.Write([]byte(str))
	return string(h.Sum(nil))
}

// GenerateWarning generates warnings as per warningType for given set of inputs
// Use predefined warningTypes and add new warningTypes and Formats for those as needed if it can be reused.
// example:
// 1. FoundResourceWarning:
//
// Found Dependent Resource:
// GVK: [storage.k8s.io/v1, Kind=StorageClass]
// Name: [csi-gce-pd]
//
// 2. ModifiedResourceWarning:
//
// Resource Modified:
// GVK: [rbac.authorization.k8s.io/v1, Kind=RoleBinding]
// Name: [abc-rolebinding-test]
// subjects[0].namespace: [manoj] -> [ms]
//
// 3. NotSupportedWarning:
//
// Restore Not Supported:
// GVK: [/v1, Kind=Namespace]
// Name: [ms-res]
//
// 4. DependentResourceNotFoundWarning:
//
// Dependent Resource Not Found In Namespace/Cluster:
// GVK: [rbac.authorization.k8s.io/v1, Kind=RoleBinding]
// Name: [abc-rolebinding-test]
//
// 5. DependentCRDNotFoundWarning:
//
// Dependent CRD Not Found For Given CR:
// GVK: [mysql.presslabs.org/v1alpha1, Kind=MysqlCluster]
// Name: [mysqlcluster-cr]
//
// currently kept default case behavior:
//
// warningType:
// GVK: [gvkStr]
// Name: [name]
// meta
//
func GenerateWarning(warningType, name, gvkStr string, meta ...string) string {
	switch warningType {
	case internal.HostPortWarning, internal.HostNetworkWarning, internal.NodeSelectorWarning, internal.NodeAffinityWarning,
		internal.NodePortWarning, internal.PodNotRunningWarning, internal.DependantResourceWarning:
		return FormatWarning(warningType, gvkStr, name)
	case internal.NotSupportedWarning:
		return FormatWarning(strings.Join(meta, " ")+warningType, gvkStr, name)
	case internal.ModifiedResourceWarning:
		return FormatWarning(warningType, gvkStr, name, meta...)
	case internal.DependentResourceNotFoundWarning, internal.DependentCRDNotFoundWarning:
		return FormatWarning(warningType, gvkStr, name)
	default:
		return FormatWarning(warningType, gvkStr, name, meta...)
	}
}

func FormatWarning(warning, gvkStr, name string, extraMeta ...string) string {

	return fmt.Sprintf(`%s:
GVK: [%s]
Name: [%s]
%s`, warning, gvkStr, name, strings.Join(extraMeta, "\n"))
}

func GetOperatorHelmIdentifier(operatorID, helmRelease string) string {
	return strings.Join([]string{operatorID, helmRelease}, "-")
}

// getUniqueHelmRelName will give the unique name for the helm release
// Unique name will get generated using the current release name and restore namespace name
// If len of release name + restore namespace name is >58 then trim it down from the end
// Append the randomly generated string of len 4 to the release name + restore namespace name
// Max attempts to get the unique name = 5
func GetUniqueHelmRelName(currentHelmRelName, restoreNamespace string, hsb helmutils.HelmStorageBackend) (string, error) {

	var newHelmRelName string
	log.Infof("Getting the unique helm release name...")
	for i := 1; i <= internal.MaxAttemptsToGetRelName; i++ {
		newHelmRelName = helmutils.GenerateHelmReleaseName(currentHelmRelName)

		log.Infof("Checking if release %s present in namespace %s", newHelmRelName, restoreNamespace)
		// Validate if helm app already present in the namespace using release name
		_, rlsErr := hsb.GetRelease(newHelmRelName, internal.DefaultHelmAppRevision)
		if rlsErr == nil {
			// Release found in the restore namespace.
			log.Infof("Release %s found in %s", newHelmRelName, restoreNamespace)
			if i == internal.MaxAttemptsToGetRelName {
				log.Errorf("All the randomly generated names for helm release are already present in the given namespace")
				return newHelmRelName, fmt.Errorf("couldn't get the unique name for helm release")
			}
			log.Info("Retrying...")
			continue
		}
		break
	}
	log.Infof("Returning the unique helm release name %s", newHelmRelName)

	return newHelmRelName, nil
}

// Write json data to a file
func SerializeStructToFilePath(structure interface{}, parentPath, fileName string) error {
	byteMetadata, err := json.MarshalIndent(structure, "", "    ")
	if err != nil {
		return err
	}
	filePath := path.Join(parentPath, fileName)
	err = shell.WriteToFile(filePath, string(byteMetadata))
	if err != nil {
		log.Errorf("Error while creating the directory %s at datastore", filePath)
		return err
	}
	return nil
}

// GetTVKInstanceID returns the UID of the namespace in which the TVK app is installed
func GetTVKInstanceID(a *kube.Accessor) string {
	nsName := internal.GetInstallNamespace()
	installNS, getNSErr := a.GetNamespace(nsName)
	if getNSErr != nil {
		panic(fmt.Sprintf("Failed to get the install namespace %s", nsName))
	}

	return string(installNS.UID)
}

// CheckAndAddWarning takes gvk, name, warningType as parameters to uniquely identify every resource's warning and
// add it to the global warning map. It also takes labels as parameters so that resources with TVK labels are not added
// in the warning map in the first place. A special case of nil label is also added, in case a resource has
// a dependant resource which couldn't be fetched from the API server, and thus resource object and consequently
// it's labels won't be available which will be passed to this function as nil.
func CheckAndAddWarning(gvk, name, warningType string, labels map[string]string, warnings WarningMap, meta ...string) {
	var isTrilioLabel bool
	if labels != nil {
		val, ok := labels[internal.K8sPartOfLabel]
		if ok && val == internal.PartOf {
			isTrilioLabel = true
		}
	}

	if !isTrilioLabel {
		warningString := GenerateWarning(warningType, name, gvk, meta...)
		warningKey := GetWarningKey(gvk, name, warningType)
		if _, ok := warnings[warningKey]; !ok {
			warnings[warningKey] = warningString
		}
	}
}

// GetWarningString iterates over all the warnings in the ignore warning map, deletes them if they exist
// in the original warning map and returns the rest of the warnings as an array of string.
func GetWarningString(ignoreWarningMap map[string]struct{}, warningMap WarningMap) (warnings []string) {
	for key := range warningMap {
		if _, ok := ignoreWarningMap[key]; ok {
			delete(warningMap, key)
			continue
		}
		warnings = append(warnings, warningMap[key])
	}
	return
}

// FilterChildResourceWarnings receives resource and its parents resources along with a global ignore warning map.
// Both child resource and parents are combined into a single resource list which is iterated and checked if the warning
// for that resource already exists or not. Since warning map's key is gvk, name and warning type, and while iterating
// through the resources, warning type is not available to check warning of that resource in the warning map so we need
// to iterate over all warning Types to construct key and check if res with a particular warning type exists in the warning
// map. If the warning exists, we check for special warnings. Here are the two cases:
//
// 1. Idea: For non special warnings, only the top most parent should have the warning and its child shouldn't.
//    Implementation: If the warning is not special, it is deleted from the global warning map in filterNonSpecialWarnings
//    function. The loop iterates only till the second last element so the topmost parent's warning persists.
//
// 2. Idea: For special warnings, only the child should have the warning and it's parent's warning should be removed.
//    Implementation: If the warning is special, the current resource warning is skipped and it's parent's gvk, name and
//    warningType are stored in the global ignore warning map. This keeps going on for every parent of
//    the current resource in an iteration. Ignore list is maintained separately instead of directly deleting the parent
//    resource warning from warning map because this function is called multiple times for each resource along
//    with it's parents and they don't have a fixed order.
//
//    E.g. In a particular iteration of component while parsing for parent, when the resource is pod along with its
//    parents (replica set and deployment), global warning map won't have warnings for replica set or deployments yet,
//    but they need to be removed in the function GetWarningString returned in the end when all the resource warnings
//    are populated in the warning map.
func FilterChildResourceWarnings(res unstructured.Unstructured, parentsResList []unstructured.Unstructured,
	ignoreResWarningList map[string]struct{}, warningMap WarningMap) {
	// append child resource to the beginning of parent list and make a single resList containing both res and its parents.
	resList := append([]unstructured.Unstructured{res}, parentsResList...)
	for i := 0; i < len(resList)-1; i++ {
		childResName := resList[i].GetName()
		childResGVK := resList[i].GroupVersionKind().String()
		for j := range WarningTypesList {
			warningKey := GetWarningKey(childResGVK, childResName, WarningTypesList[j])
			_, childWarningExist := warningMap[warningKey]
			if childWarningExist && filterNonSpecialWarnings(warningKey, warningMap) {
				// add gvk, name and warning Type of parent of resource
				parentWarningKey := GetWarningKey(resList[i+1].GroupVersionKind().String(), resList[i+1].GetName(),
					WarningTypesList[j])
				ignoreResWarningList[parentWarningKey] = struct{}{}
			}
		}
	}
}

func filterNonSpecialWarnings(warningKey string, warningMap WarningMap) (specialWarningExist bool) {
	warningType := strings.Split(warningKey, "@")[1]
	switch warningType {
	case internal.HostNetworkWarning, internal.HostPortWarning, internal.NodeAffinityWarning, internal.NodeSelectorWarning,
		internal.NodePortWarning:
		specialWarningExist = true
	default:
		delete(warningMap, warningKey)
	}
	return
}

func GetWarningKey(gvk, name, warningType string) string {
	return gvk + "/" + name + "@" + warningType
}

func CheckIsOpenshift(config *rest.Config) bool {
	discoveryClient := GetDiscoveryClient(config)
	_, err := discoveryClient.ServerResourcesForGroupVersion("security.openshift.io/v1")
	if err != nil {
		if apierrs.IsNotFound(err) {
			return false
		}
	}
	return true
}

func IsOlmDeployment(k8sclient client.Client, namespace string) bool {
	deployment := &appsv1.Deployment{}
	if err := k8sclient.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      "k8s-triliovault-admission-webhook",
	}, deployment); err != nil {
		panic(err)
	}

	if len(deployment.OwnerReferences) == 0 {
		return false
	} else if deployment.OwnerReferences[0].Kind == internal.ClusterServiceVersionKind {
		return true
	}

	return false
}

// TODO: To remove NamespaceType check
// Function to Retrieve ApplicationType from backupPlanSpec object
func GetApplicationType(spec *crd.BackupPlanSpec) crd.ApplicationType {
	var applicationType crd.ApplicationType

	if reflect.DeepEqual(spec.BackupPlanComponents, crd.BackupPlanComponents{}) {
		applicationType = crd.NamespaceType
	} else {
		if len(spec.BackupPlanComponents.HelmReleases) > 0 && spec.BackupPlanComponents.HelmReleases != nil &&
			spec.BackupPlanComponents.Operators == nil && spec.BackupPlanComponents.Custom == nil {
			applicationType = crd.HelmType
		} else if len(spec.BackupPlanComponents.Operators) > 0 && spec.BackupPlanComponents.Operators != nil &&
			spec.BackupPlanComponents.HelmReleases == nil && spec.BackupPlanComponents.Custom == nil {
			applicationType = crd.OperatorType
		} else {
			applicationType = crd.CustomType
		}
	}

	return applicationType
}

// IsHelmSecretOrConfigMap checks if given secret or configmap is used in helm storage backend.
// Returns true if yes else returns false
func IsHelmSecretOrConfigMap(item unstructured.Unstructured) bool {
	l := item.GetLabels()

	if value, present := l["OWNER"]; present && value == "TILLER" {
		return true
	}

	if value, present := l["owner"]; present && value == "helm" {
		return true
	}

	return false
}

// CheckIfTVKLabelExists returns true if given resource has tvk label i.e. ["app.kubernetes.io/part-of": "k8s-triliovault"]
func CheckIfTVKLabelExists(res unstructured.Unstructured) bool {
	label := res.GetLabels()
	if val, ok := label[internal.K8sPartOfLabel]; ok && val == internal.PartOf {
		return true
	}
	return false
}

// IsTvkSAReferredSecret returns true if if secret has annotation having prefix 'triliovault-backup' or 'k8s-triliovault'.
func IsTvkSAReferredSecret(secret unstructured.Unstructured) bool {

	anno := secret.GetAnnotations()

	tvkBackupPrefix := strings.ToLower(internal.CategoryTriliovault + "-" + internal.BackupKind)
	tvkSAPrefix := internal.ServiceAccountName

	if val, ok := anno[internal.OcpSecretAnnotation]; ok && (strings.HasPrefix(val, tvkBackupPrefix) || strings.HasPrefix(val, tvkSAPrefix)) {
		return true
	}

	return false
}
