package logcollector

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	TriliovaultGroup          = "triliovault.trilio.io"
	CsiStorageGroup           = "csi.storage.k8s.io"
	SnapshotStorageGroup      = "snapshot.storage.k8s.io"
	ClusterServiceVersion     = "clusterserviceversions"
	ClusterServiceVersionKind = "ClusterServiceVersion"

	OperatorGroup              = "operators.coreos.com"
	APIExtensionsGroup         = "apiextensions.k8s.io"
	AdmissionRegistrationGroup = "admissionregistration.k8s.io/v1beta1"

	StorageGv     = "storage.k8s.io/v1"
	CoreGv        = "v1"
	BatchGv       = "batch/v1"
	BatchGv1beta1 = "batch/v1beta1"
	AppsGv        = "apps/v1"
	networkingGv  = "networking.k8s.io/v1beta1"

	Namespaces          = "namespaces"
	Events              = "events"
	CRD                 = "customresourcedefinitions"
	StorageClass        = "storageclasses"
	VolumeSnapshot      = "volumesnapshots"
	VolumeSnapshotClass = "volumesnapshotclasses"
	Pod                 = "Pod"
	ControllerRevision  = "ControllerRevision"

	LicenseKind = "License"
	NodeKind    = "Node"
	Verblist    = "list"
)

var (
	scheme = runtime.NewScheme()

	// CoreGRPResources ... List of core group resources collected by log collector
	CoreGRPResources = []string{"Pod", "PersistentVolumeClaim", "PersistentVolume", "Service",
		"ConfigMap", "Namespace", "ResourceQuota", "LimitRange", "Node", "Endpoints"}
	K8STrilioVaultLabel = map[string]string{"app.kubernetes.io/part-of": "k8s-triliovault"}

	nonTrilioResources = []string{"ResourceQuota", "LimitRange"}
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
		if strings.HasPrefix(apiVersion, TriliovaultGroup) || (kindExist && nameNsExist) {

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

// filterCSV returns list of openshift csv created by triliovault
func filterCSV(csvObjects unstructured.UnstructuredList) unstructured.UnstructuredList {

	var filteredCSVObject unstructured.UnstructuredList
	for index := range csvObjects.Items {
		if strings.HasPrefix(csvObjects.Items[index].GetName(), "k8s-triliovault") {
			filteredCSVObject.Items = append(filteredCSVObject.Items, csvObjects.Items[index])
		}
	}
	return filteredCSVObject
}

// filterCRD returns list of crds created by given set of groups
func filterCRD(crdObjs unstructured.UnstructuredList) (unstructured.UnstructuredList, error) {
	crdFilterGroup := []string{TriliovaultGroup, SnapshotStorageGroup, CsiStorageGroup}
	var filteredCRDObject unstructured.UnstructuredList
	for index := range crdObjs.Items {
		for in := range crdFilterGroup {
			crdGroup, _, err := unstructured.NestedString(crdObjs.Items[index].Object, "spec", "group")
			if err != nil {
				log.Errorf("Unable to get the CRD Group field : %s", err.Error())
				return filteredCRDObject, err
			}
			if crdFilterGroup[in] == crdGroup {
				filteredCRDObject.Items = append(filteredCRDObject.Items, crdObjs.Items[index])
			}
		}
	}
	return filteredCRDObject, nil
}

// getGVByGroup returns group_version matched for given group
func getGVByGroup(apiGVList []*apiv1.APIGroup, groupName string, isPreferredVersion bool) (gvList []string) {

	for index := range apiGVList {
		if apiGVList[index].Name == groupName {
			if isPreferredVersion {
				gvList = append(gvList, apiGVList[index].PreferredVersion.GroupVersion)
			}
			for in := range apiGVList[index].Versions {
				gvList = append(gvList, apiGVList[index].Versions[in].GroupVersion)
			}
		}
	}
	return gvList
}

// getResourcesGVByName resource object and gv for given resource name
func getResourcesGVByName(resourceMap map[string][]apiv1.APIResource, name string) map[string]apiv1.APIResource {

	gvResourceMap := make(map[string]apiv1.APIResource)
	for gv, resource := range resourceMap {
		for index := range resource {
			if resource[index].Name == name {
				gvResourceMap[gv] = resource[index]
				continue
			}
		}
	}
	return gvResourceMap
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

// getObjectsNames returns list of names of objects
func getObjectsNames(objects unstructured.UnstructuredList) map[string]string {
	nameList := make(map[string]string)
	for index := range objects.Items {
		nameList[objects.Items[index].GetName()] = objects.Items[index].GetName()
	}
	return nameList
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

// getClientSet Initialize k8s Client, discovery client, k8s Client set
func getClient() (client.Client, *discovery.DiscoveryClient, *kubernetes.Clientset) {
	conFig := config.GetConfigOrDie()
	_ = corev1.AddToScheme(scheme)
	_ = clientGoScheme.AddToScheme(scheme)

	clientSet, err := kubernetes.NewForConfig(conFig)
	if err != nil {
		log.Fatalf("Unable to get access to K8S : %s", err.Error())
	}
	kClient, kErr := client.New(conFig, client.Options{Scheme: scheme})
	if kErr != nil {
		log.Fatalf("Unable to get client : %s", kErr.Error())
	}
	discClient, dErr := discovery.NewDiscoveryClientForConfig(conFig)
	if dErr != nil {
		log.Fatalf("Unable to create discovery client")
	}
	return kClient, discClient, clientSet
}

// contains checks if a string is present in a slice
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// filterGroupResources returns list of filtered resources fetched from each group
func filterGroupResources(resources []apiv1.APIResource, group string) (filteredResources []apiv1.APIResource) {

	for index := range resources {
		if group == CoreGv && contains(CoreGRPResources, resources[index].Kind) {
			filteredResources = append(filteredResources, resources[index])
		} else if group == AppsGv && resources[index].Kind != ControllerRevision {
			filteredResources = append(filteredResources, resources[index])
		} else if group == BatchGv || group == BatchGv1beta1 || group == networkingGv {
			filteredResources = append(filteredResources, resources[index])
		}
	}
	return filteredResources
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
