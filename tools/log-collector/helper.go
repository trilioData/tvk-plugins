package logcollector

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/trilioData/tvk-plugins/internal"
)

const (
	CsiStorageGroup           = "csi.storage.k8s.io"
	SnapshotStorageGroup      = "snapshot.storage.k8s.io"
	ClusterServiceVersion     = "clusterserviceversions"
	ClusterServiceVersionKind = "ClusterServiceVersion"
	TriliovaultGroupVersion   = "triliovault.trilio.io/v1"

	CoreGv           = "v1"
	Events           = "events"
	CRD              = "customresourcedefinitions"
	Namespaces       = "namespaces"
	Pod              = "Pod"
	SubscriptionKind = "Subscription"
	InstallPlanKind  = "InstallPlan"

	LicenseKind = "License"
	Verblist    = "list"

	TrilioPrefix = "k8s-triliovault"
)

var (
	scheme = runtime.NewScheme()

	K8STrilioVaultLabel = map[string]string{"app.kubernetes.io/part-of": TrilioPrefix}
	nonLabeledResources = sets.NewString("ResourceQuota", "LimitRange", "VolumeSnapshot", "ClusterServiceVersion")
	clusteredResources  = sets.NewString("Node", "Namespace", "CustomResourceDefinition", "StorageClass",
		"VolumeSnapshotClass")
	excludeResources = sets.NewString("Secret", "PackageManifest")
)

type containerStat struct {
	prev bool
	curr bool
}

// aggregateEvents aggregates events based on involved objects
func aggregateEvents(eventObjects unstructured.UnstructuredList,
	resourceMap map[string][]types.NamespacedName) (map[string][]map[string]interface{}, error) {

	eventsData := make(map[string][]map[string]interface{})
	for _, eve := range eventObjects.Items {
		apiVersion, _, aErr := unstructured.NestedString(eve.Object, "involvedObject", "apiVersion")
		if aErr != nil {
			log.Errorf("Unable to get event data of Object : %s", aErr.Error())
			return nil, aErr
		}

		namespace, _, nErr := unstructured.NestedString(eve.Object, "involvedObject", "namespace")
		if nErr != nil {
			log.Errorf("Unable to get event data of Object : %s", nErr.Error())
			return nil, nErr
		}
		if namespace == "" {
			namespace = "default"
		}

		kind, _, kErr := unstructured.NestedString(eve.Object, "involvedObject", "kind")
		if kErr != nil {
			log.Errorf("Unable to get event data of Object : %s", kErr.Error())
			return nil, kErr
		}

		name, _, naErr := unstructured.NestedString(eve.Object, "involvedObject", "name")
		if naErr != nil {
			log.Errorf("Unable to get event data of Object : %s", naErr.Error())
			return nil, naErr
		}
		namespacedName := getNamespacedName(namespace, name)

		// checking if kind and Namespaced Name exist in resourceMap
		kindExist := false
		nameNsExist := false
		if value, ok := resourceMap[kind]; ok {
			kindExist = true
			for _, nameNs := range value {
				if cmp.Equal(namespacedName, nameNs) {
					nameNsExist = true
				}
			}
		}
		if strings.HasPrefix(apiVersion, internal.TriliovaultGroup) || (kindExist && nameNsExist) {

			_, ok := eve.Object["metadata"]
			if ok {
				delete(eve.Object, "metadata")
			}
			_, ok = eve.Object["involvedObject"]

			if ok {
				delete(eve.Object, "involvedObject")
			}

			kindNameKey := fmt.Sprintf("%s/%s", strings.ToLower(kind), name)

			tempMap := make(map[string]interface{})
			tempMap[kindNameKey] = eve.Object
			eventsData[namespace] = append(eventsData[namespace], tempMap)
		}
	}
	return eventsData, nil
}

// getNamespacedName returns namespaced name representation of a resource
func getNamespacedName(namespace, name string) types.NamespacedName {

	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

// filterTvkCSV returns list of openshift csv created by triliovault
func filterTvkCSV(csvObjects unstructured.UnstructuredList) unstructured.UnstructuredList {

	var filteredCSVObject unstructured.UnstructuredList
	for index := range csvObjects.Items {
		if strings.HasPrefix(csvObjects.Items[index].GetName(), TrilioPrefix) {
			filteredCSVObject.Items = append(filteredCSVObject.Items, csvObjects.Items[index])
		}
	}
	return filteredCSVObject
}

// filterTvkSnapshotAndCSICRD returns list of crds created by given set of groups
func filterTvkSnapshotAndCSICRD(crdObjs unstructured.UnstructuredList) (unstructured.UnstructuredList, error) {
	crdFilterGroup := sets.NewString(internal.TriliovaultGroup, SnapshotStorageGroup, CsiStorageGroup)
	var filteredCRDObjects unstructured.UnstructuredList
	for index := range crdObjs.Items {
		crdGroup, _, err := unstructured.NestedString(crdObjs.Items[index].Object, "spec", "group")
		if err != nil {
			log.Errorf("Unable to get the CRD Group field : %s", err.Error())
			return filteredCRDObjects, err
		}
		if crdFilterGroup.Has(crdGroup) {
			filteredCRDObjects.Items = append(filteredCRDObjects.Items, crdObjs.Items[index])
		}
	}
	return filteredCRDObjects, nil
}

// filterInputNS returns list of Namespaces Object given by user input in --namespaces flag
func filterInputNS(nsObjs unstructured.UnstructuredList, namespaces []string) unstructured.UnstructuredList {
	var filteredNSObjects unstructured.UnstructuredList

	nsNames := sets.NewString(namespaces...)

	for _, nsObj := range nsObjs.Items {
		if nsNames.Has(nsObj.GetName()) {
			filteredNSObjects.Items = append(filteredNSObjects.Items, nsObj)
		}
	}
	return filteredNSObjects
}

// getContainerStatusValue returns whether current and previous container present to capture logs
func getContainerStatusValue(containerStatus *corev1.ContainerStatus) (conStatObj containerStat) {

	lastState := containerStatus.LastTerminationState
	currentState := containerStatus.State

	if lastState.Waiting == nil {
		if lastState.Terminated != nil || lastState.Running != nil {
			conStatObj.prev = true
		}
	} else {
		log.Errorf("Container %s Previous State is in Waiting", containerStatus.Name)
	}

	if currentState.Waiting == nil {
		if currentState.Terminated != nil || currentState.Running != nil {
			conStatObj.curr = true
		}
	} else {
		log.Errorf("Container %s Current State is in Waiting", containerStatus.Name)
	}

	return conStatObj
}

// getAPIGroupVersionResourcePath returns api resource path for given groupVersion
func getAPIGroupVersionResourcePath(apiGroupVersion string) string {
	if apiGroupVersion == "v1" {
		return "/api/v1"
	}
	return "/apis/" + apiGroupVersion

}

// getContainers returns containers of a pod with their current and previous statuses
func getContainers(podObject *corev1.Pod) map[string]containerStat {
	containers := make(map[string]containerStat)
	containerStatuses := podObject.Status.ContainerStatuses
	for index := range containerStatuses {
		status := getContainerStatusValue(&containerStatuses[index])
		if status.curr || status.prev {
			containers[containerStatuses[index].Name] = status
		}
	}
	containerStatuses = podObject.Status.InitContainerStatuses
	for index := range containerStatuses {
		status := getContainerStatusValue(&containerStatuses[index])
		if status.curr || status.prev {
			containers[containerStatuses[index].Name] = status
		}
	}
	return containers
}

// checkLabelExist check if key [value] exist in other map
func checkLabelExist(givenLabel, toCheckInLabel map[string]string) (exist bool) {

	for key, value := range givenLabel {
		if _, ok := toCheckInLabel[key]; ok {
			if toCheckInLabel[key] == value {
				exist = true
			} else {
				exist = false
			}
		}
	}
	return exist
}

// filterTvkResourcesByLabel filter objects on the basis of Labels
func filterTvkResourcesByLabel(allObjects *unstructured.UnstructuredList) {
	var objects unstructured.UnstructuredList

	for _, object := range allObjects.Items {
		objectLabel := object.GetLabels()
		if len(objectLabel) != 0 && checkLabelExist(objectLabel, K8STrilioVaultLabel) {
			objects.Items = append(objects.Items, object)
		}
	}
	allObjects.Items = objects.Items
}
