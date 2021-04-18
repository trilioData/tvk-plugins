package logcollector

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type LogCollector struct {
	OutputDir    string
	CleanOutput  bool
	Clustered    bool
	Namespaces   []string
	Loglevel     string
	k8sClient    client.Client
	disClient    *discovery.DiscoveryClient
	k8sClientSet *kubernetes.Clientset
}

// setClient initialize clients
func (l *LogCollector) setClient() {
	l.k8sClient, l.disClient, l.k8sClientSet = getClient()
	l.disClient.LegacyPrefix = "/api/"
}

// CollectLogsAndDump collects call all the related resources of triliovault
func (l *LogCollector) CollectLogsAndDump() error {

	l.setClient()
	l.getClusterNodes()

	nsErr := l.checkNamespaces()
	if nsErr != nil {
		log.Errorf("%s", nsErr.Error())
		return nil
	}

	apiGroups, apiErr := l.fetchAPIGroups()
	if apiErr != nil {
		return apiErr
	}

	cErr := l.clusterServiceVersion(apiGroups)
	if cErr != nil {
		return cErr
	}

	apErr := l.apiExtensionGroup(apiGroups)
	if apErr != nil {
		return apErr
	}

	sErr := l.snapshotStorageGroup(apiGroups)
	if sErr != nil {
		return sErr
	}

	arErr := l.admissionRegistrationGroup(apiGroups)
	if arErr != nil {
		return arErr
	}

	tErr := l.trilioGroup(apiGroups)
	if tErr != nil {
		return tErr
	}

	log.Info("Checking Storage Group")
	storageGVResources, stErr := l.getAPIGVResources(StorageGv)
	if stErr != nil {
		return stErr
	}
	scResource := getResourceByName(storageGVResources, StorageClass)
	scObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(StorageGv), &scResource)

	for _, sc := range scObjects.Items {
		resourceDir := filepath.Join(scResource.Kind)
		eLrr := l.writeYaml(resourceDir, sc)
		if eLrr != nil {
			return eLrr
		}
	}

	resourceGroup, rErr := l.getResourceGroup()
	if rErr != nil {
		return rErr
	}

	log.Info("Checking Core Group")
	coreGVResources, cgvErr := l.getAPIGVResources(CoreGv)
	if cgvErr != nil {
		return cgvErr
	}
	resourceGroup[CoreGv] = coreGVResources

	log.Info("Writing and Filtering Logs")
	resourceMap, fErr := l.filteringWithLabels(resourceGroup)
	if fErr != nil {
		log.Errorf("Unable to get labeled Objects : %s", fErr.Error())
		return fErr
	}

	log.Info("Fetching Resources Events")
	eventResource := getResourceByName(coreGVResources, Events)
	eventObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(CoreGv), &eventResource)
	events, aErr := aggregateEvents(eventObjects, resourceMap)
	if aErr != nil {
		log.Errorf("Unable to process Events : %s", aErr.Error())
		return aErr
	}

	eErr := l.writeEvents(events)
	if eErr != nil {
		log.Errorf("Unable to Write Events : %s", eErr.Error())
		return eErr
	}

	// Zip Directory
	zErr := l.zipDir()
	if zErr != nil {
		log.Errorf("Unable zip Directory : %s", zErr.Error())
		return zErr
	}

	// check for clean output flag. if true, clean.
	if l.CleanOutput {
		err := os.RemoveAll(l.OutputDir)
		if err != nil {
			log.Errorf("Unable to clean directory : %s", err.Error())
			return err
		}
	}
	return nil
}

// getApiGVResources returns list of resources for given group version
func (l *LogCollector) getAPIGVResources(apiGroupVersion string) (gVResources []apiv1.APIResource, err error) {

	var gVResourcesList *apiv1.APIResourceList
	gVResourcesList, err = l.disClient.ServerResourcesForGroupVersion(apiGroupVersion)
	if err != nil {
		return gVResources, err
	}

	for index := range gVResourcesList.APIResources {
		for in := range gVResourcesList.APIResources[index].Verbs {
			if gVResourcesList.APIResources[index].Verbs[in] == "list" {
				gVResources = append(gVResources, gVResourcesList.APIResources[index])
			}
		}
	}
	return gVResources, nil
}

// getApiGVResourcesMap returns list of resources for given group version
func (l *LogCollector) getAPIGVResourcesMap(gvList []string) (map[string][]apiv1.APIResource, error) {

	resourceMap := make(map[string][]apiv1.APIResource)
	for index := range gvList {
		resources, err := l.getAPIGVResources(gvList[index])
		if err != nil {
			return resourceMap, err
		}
		resourceMap[gvList[index]] = resources
	}
	return resourceMap, nil
}

// TODO()
// getGVResourcesObjects returns list of objects for given resource_path
func (l *LogCollector) getResourceObjects(resourcePath string, resource *apiv1.APIResource) (objects unstructured.UnstructuredList) {

	if resource.Namespaced && !l.Clustered {
		for index := range l.Namespaces {
			var obj unstructured.UnstructuredList
			listPath := fmt.Sprintf("%s/namespaces/%s/%s", resourcePath, l.Namespaces[index], resource.Name)
			err := l.disClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&obj)
			if err != nil {
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					return objects
				}
				return unstructured.UnstructuredList{}
			}
			objects.Items = append(objects.Items, obj.Items...)
		}
		return objects
	}
	listPath := fmt.Sprintf("%s/%s", resourcePath, resource.Name)
	err := l.disClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&objects)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			return objects
		}
		return unstructured.UnstructuredList{}
	}
	return objects
}

// getResourceObjects returns list of objects for given resource_path
func (l *LogCollector) getGVResourceObjects(gvResourceMap map[string]apiv1.APIResource) unstructured.UnstructuredList {

	resourceObject := unstructured.UnstructuredList{}
	for gv := range gvResourceMap {
		gvResource := gvResourceMap[gv]
		resourceObject.Items = append(resourceObject.Items, l.getResourceObjects(getAPIGroupVersionResourcePath(gv), &gvResource).Items...)
	}
	return resourceObject
}

// writeEvents writes events
func (l *LogCollector) writeEvents(events map[string][]map[string]interface{}) error {

	for k, v := range events {
		resourceDir := filepath.Join(l.OutputDir, "Events", k)
		if _, err := os.Stat(resourceDir); os.IsNotExist(err) {
			mErr := os.MkdirAll(resourceDir, 0755)
			if mErr != nil {
				log.Errorf("Unable to create the directory : %s", mErr.Error())
				return mErr
			}
		}

		for _, obj := range v {
			for key, value := range obj {
				key = strings.Replace(key, "/", ".", 1)
				objectFilePath := filepath.Join(resourceDir, key)
				fp, err := os.Create(objectFilePath + ".yaml")
				if err != nil {
					log.Errorf("Unable to create the file : %s", err.Error())
					return err
				}
				buf, bErr := yaml.Marshal(value)
				if bErr != nil {
					log.Errorf("Unable to marshal the content : %s", bErr.Error())
					return bErr
				}
				_, fErr := fp.Write(buf)
				if fErr != nil {
					log.Errorf("Unable to write the contents : %s", fErr.Error())
					return fErr
				}
			}
		}
	}
	return nil
}

// writeYaml writes yaml for given k8s object
func (l *LogCollector) writeYaml(resourceDir string, obj unstructured.Unstructured) error {

	objNs := obj.GetNamespace()
	objName := obj.GetName()
	resourcePath := filepath.Join(l.OutputDir, resourceDir, objNs)
	err := os.MkdirAll(resourcePath, 0755)
	if err != nil {
		log.Errorf("Unable to create the directory : %s", err.Error())
		return err
	}
	objFilepath := filepath.Join(resourcePath, objName)
	fp, fErr := os.Create(objFilepath + ".yaml")
	if fErr != nil {
		log.Errorf("Unable to create the file : %s", fErr.Error())
		return fErr
	}
	defer fp.Close()
	buf, mErr := yaml.Marshal(obj.Object)
	if mErr != nil {
		log.Errorf("Unable to marshal the content : %s", mErr.Error())
		return mErr
	}
	_, bErr := fp.Write(buf)
	if bErr != nil {
		log.Errorf("Unable to write the content : %s", bErr.Error())
		return bErr
	}
	return nil
}

// writeLogs creates log for given pod object
func (l *LogCollector) writeLogs(resourceDir string, obj unstructured.Unstructured) error {

	objNs := obj.GetNamespace()
	objName := obj.GetName()
	resourcePath := filepath.Join(l.OutputDir, resourceDir, objNs)
	if _, err := os.Stat(resourcePath); os.IsNotExist(err) {
		mErr := os.MkdirAll(resourcePath, 0755)
		if mErr != nil {
			log.Errorf("Unable to Create the Directory : %s", mErr.Error())
			return mErr
		}
	}

	var podObj corev1.Pod
	err := l.k8sClient.Get(context.Background(), types.NamespacedName{Name: objName, Namespace: objNs}, &podObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Errorf("%s", err.Error())
			return nil
		}
		log.Errorf("Unable to get the object : %s", err.Error())
		return err
	}
	containers := getContainers(&podObj)

	for name, statuses := range containers {
		if statuses.curr {
			eLrr := l.writeLog(resourcePath, objNs, objName, name, false)
			if eLrr != nil {
				return eLrr
			}
		}
		if statuses.prev {
			eLrr := l.writeLog(resourcePath, objNs, objName, name, true)
			if eLrr != nil {
				return eLrr
			}
		}
	}
	return nil
}

// isSubset checks whether the given namespaces is a subset of all Namespaces in cluster
func (l *LogCollector) isSubset(second []string) bool {
	set := make(map[string]string)
	for _, value := range second {
		set[value] = value
	}
	for _, v := range l.Namespaces {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}

// writeLog writes logs of a pod object
func (l *LogCollector) writeLog(resourceDir, objNs, objName, container string, isPrevious bool) error {

	logOption := corev1.PodLogOptions{
		Container: container,
		Previous:  isPrevious,
	}

	req := l.k8sClientSet.CoreV1().Pods(objNs).GetLogs(objName, &logOption)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		log.Errorf("Unable to get Logs for container %s : %s", container, err.Error())
		return nil
	}
	defer podLogs.Close()

	buf, err := ioutil.ReadAll(podLogs)
	if err != nil {
		log.Errorf("Error in copy information from podLogs to buffer : %s", err.Error())
		return err
	}

	var subPath string
	if isPrevious {
		subPath = "previous"
	} else {
		subPath = "current"
	}
	objectFilepath := fmt.Sprintf("%s.%s.%s.log", filepath.Join(resourceDir, objName), container, subPath)
	outFile, err := os.Create(objectFilepath)
	if err != nil {
		log.Errorf("Error Creating Log File : %s", err.Error())
		return err
	}
	defer outFile.Close()
	_, err = outFile.Write(buf)
	if err != nil {
		log.Errorf("Unable to Write Pod Logs to the File : %s", err.Error())
		return err
	}

	return nil
}

// zipDir creates zip directory of collected info
func (l *LogCollector) zipDir() error {

	file, err := os.Create(l.OutputDir + ".zip")
	log.Infof("Creating Zip : %s.zip\n", l.OutputDir)

	if err != nil {
		log.Errorf("Error Creating zip File : %s", err.Error())
		return err
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	walker := func(path string, info os.FileInfo, err error) error {
		log.Debugf("Crawling: %#v\n", path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		f, err := w.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}

		return nil
	}
	err = filepath.Walk(l.OutputDir, walker)
	if err != nil {
		log.Errorf("Unable to walk thorugh directory : %s", err.Error())
		return err
	}
	if l.CleanOutput {
		err = os.RemoveAll(l.OutputDir)
		if err != nil {
			log.Errorf("Unable to remove directory : %s", err.Error())
			return err
		}
	}
	return nil
}

// TODO()
func (l *LogCollector) getResourceObjectsWithLabel(resourcePath string,
	resource *apiv1.APIResource) (objects unstructured.UnstructuredList) {
	allObjects := l.getResourceObjects(resourcePath, resource)
	for _, object := range allObjects.Items {
		objectLabel := object.GetLabels()
		if len(objectLabel) != 0 {
			if checkLabelExist(objectLabel, K8STrilioVaultLabel) {
				objects.Items = append(objects.Items, object)
			}
		} else if contains(nonTrilioResources, object.GetKind()) {
			objects.Items = append(objects.Items, object)
		}
	}
	return objects
}

func (l *LogCollector) filteringWithLabels(resourceGroup map[string][]apiv1.APIResource) (map[string][]types.NamespacedName, error) {
	// These operations are performed in the following lines:
	// 1. Iterating through all the resources from batch, apps and core groups.
	// 2. Filtering only those resources that we need from all the available resources obtained above.
	// 3. Iterating through the filtered resources to fetch their respective objects based on the group from which
	//    they belong and labels
	//    e.g. fetching all pods from core group with the label 'app.kubernetes.io/part-of':'k8s-triliovault'
	// 4. Collecting pod names specifically that is later required by events
	// 5. Collecting list of all resource objects and printing their yamls in their respective resource folder under
	//    their respective namespaces. In case of pods, logs are also collected
	resourceMap := make(map[string][]types.NamespacedName)
	var nsName []types.NamespacedName
	for group, resList := range resourceGroup {
		resources := filterGroupResources(resList, group)

		for index := range resources {
			var resObjects unstructured.UnstructuredList
			res := getResourceByName(resources, resources[index].Name)
			resObject := l.getResourceObjectsWithLabel(getAPIGroupVersionResourcePath(group), &res)
			resObjects.Items = append(resObjects.Items, resObject.Items...)

			if l.CheckIsOpenshift() {
				olmObj := l.getResourceObjectsWithOwnerRef(getAPIGroupVersionResourcePath(group), &res)
				resObjects.Items = append(resObjects.Items, olmObj.Items...)
			}

			for _, obj := range resObjects.Items {

				oName := obj.GetName()
				oNs := obj.GetNamespace()
				nsName = append(nsName, types.NamespacedName{Name: oName, Namespace: oNs})
				resourceMap[res.Kind] = nsName

				resourceDir := filepath.Join(res.Kind)
				if res.Kind == Pod {
					eLrr := l.writeLogs(resourceDir, obj)
					if eLrr != nil {
						return nil, eLrr
					}
				}
				eLrr := l.writeYaml(resourceDir, obj)
				if eLrr != nil {
					return nil, eLrr
				}
			}
		}
	}
	return resourceMap, nil
}

// admissionRegistrationGroup gets all the resources related admissionRegistration and writes their YAML
func (l *LogCollector) admissionRegistrationGroup(apiGroups []*apiv1.APIGroup) error {
	log.Info("Checking Admission Registration Group")
	admissionGV := getGVByGroup(apiGroups, AdmissionRegistrationGroup, true)
	if len(admissionGV) != 0 {
		admissionGVResources, agErr := l.getAPIGVResources(admissionGV[0])
		if agErr != nil {
			return agErr
		}
		for index := range admissionGVResources {
			objectList := l.getResourceObjects(getAPIGroupVersionResourcePath(admissionGV[0]), &admissionGVResources[index])
			resourceDir := filepath.Join(admissionGVResources[index].Kind)

			for _, obj := range objectList.Items {
				if strings.HasPrefix(obj.GetName(), "k8s-triliovault") {
					eLrr := l.writeYaml(resourceDir, obj)
					if eLrr != nil {
						return eLrr
					}
				}
			}
		}
	}
	return nil
}

// snapshotStorageGroup gets all the resources related snapshot storage and writes their YAML
func (l *LogCollector) snapshotStorageGroup(apiGroups []*apiv1.APIGroup) error {
	log.Info("Checking Snapshot Storage Group")
	snapGV := getGVByGroup(apiGroups, SnapshotStorageGroup, true)
	if snapGV[0] != "" {
		snapGVResources, err := l.getAPIGVResources(snapGV[0])
		if err != nil {
			return err
		}
		volSnapResource := getResourceByName(snapGVResources, VolumeSnapshot)
		volSnapObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(snapGV[0]), &volSnapResource)
		for _, obj := range volSnapObjects.Items {
			resourceDir := filepath.Join(obj.GetKind())
			eLrr := l.writeYaml(resourceDir, obj)
			if eLrr != nil {
				return eLrr
			}
		}

		volSnapClassResource := getResourceByName(snapGVResources, VolumeSnapshotClass)
		volSnapClassObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(snapGV[0]), &volSnapClassResource)
		for _, obj := range volSnapClassObjects.Items {
			resourceDir := filepath.Join(obj.GetKind())
			eLrr := l.writeYaml(resourceDir, obj)
			if eLrr != nil {
				return eLrr
			}
		}
	}
	return nil
}

// getResourceGroup collects all the resources related to basic group such as batch and apps
func (l *LogCollector) getResourceGroup() (map[string][]apiv1.APIResource, error) {

	resourceGroup := make(map[string][]apiv1.APIResource)
	log.Info("Checking Batch Group")
	batchGV, bgErr := l.getAPIGVResources(BatchGv)
	if bgErr != nil {
		return resourceGroup, bgErr
	}
	resourceGroup[BatchGv] = batchGV

	batchGV1beta1, bg1Err := l.getAPIGVResources(BatchGv1beta1)
	if bg1Err != nil {
		return resourceGroup, bg1Err
	}
	resourceGroup[BatchGv1beta1] = batchGV1beta1

	log.Info("Checking Apps Group")
	appsGv, agErr := l.getAPIGVResources(AppsGv)
	if agErr != nil {
		return resourceGroup, agErr
	}
	resourceGroup[AppsGv] = appsGv

	return resourceGroup, nil
}

// clusterServiceVersion collects all the resources related to CSV and writes the YAML
func (l *LogCollector) clusterServiceVersion(apiGroups []*apiv1.APIGroup) error {

	log.Info("Checking Cluster Service Version")
	operatorGVList := getGVByGroup(apiGroups, OperatorGroup, false)
	operatorGVResourceMap, oErr := l.getAPIGVResourcesMap(operatorGVList)
	if oErr != nil {
		return oErr
	}
	csvResourceMap := getResourcesGVByName(operatorGVResourceMap, ClusterServiceVersion)
	csvObjects := l.getGVResourceObjects(csvResourceMap)
	csvObjects = filterCSV(csvObjects)

	for _, csv := range csvObjects.Items {
		resourceDir := filepath.Join(csv.GetKind())
		eLrr := l.writeYaml(resourceDir, csv)
		if eLrr != nil {
			return eLrr
		}
	}

	return nil
}

// apiExtensionGroup collects all the resources related to api extension and writes the YAML
func (l *LogCollector) apiExtensionGroup(apiGroups []*apiv1.APIGroup) error {
	log.Info("Checking API Extension Group")
	apiExtGV := getGVByGroup(apiGroups, APIExtensionsGroup, true)
	if len(apiExtGV) != 0 {
		apiExtGVResources, apErr := l.getAPIGVResources(apiExtGV[0])
		if apErr != nil {
			return apErr
		}
		crdResource := getResourceByName(apiExtGVResources, CRD)
		crdObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(apiExtGV[0]), &crdResource)
		crdObjects, cErr := filterCRD(crdObjects)
		if cErr != nil {
			return cErr
		}

		for _, crd := range crdObjects.Items {
			resourceDir := filepath.Join(crd.GetKind())
			eLrr := l.writeYaml(resourceDir, crd)
			if eLrr != nil {
				return eLrr
			}
		}
	}
	return nil
}

// trilioGroup collects all the resources related to trilio and writes the YAML
func (l *LogCollector) trilioGroup(apiGroups []*apiv1.APIGroup) error {
	log.Info("Checking Trilio Group")
	trilioGV := getGVByGroup(apiGroups, TriliovaultGroup, true)

	if len(trilioGV) != 0 {
		trilioGVResources, err := l.getAPIGVResources(trilioGV[0])
		if err != nil {
			return err
		}

		for index := range trilioGVResources {
			objectList := l.getResourceObjects(getAPIGroupVersionResourcePath(trilioGV[0]), &trilioGVResources[index])
			resourceDir := filepath.Join(trilioGVResources[index].Kind)
			for _, obj := range objectList.Items {
				if obj.GetKind() == LicenseKind {
					unstructured.RemoveNestedField(obj.Object, "spec", "key")
					unstructured.RemoveNestedField(obj.Object, "metadata", "annotations")
				}
				eLrr := l.writeYaml(resourceDir, obj)
				if eLrr != nil {
					return eLrr
				}
			}
		}
	}
	return nil
}

// fetchAPIGroups returns the list of all API Groups of the supported resources for all groups and versions.
func (l *LogCollector) fetchAPIGroups() (apiGroups []*apiv1.APIGroup, err error) {

	log.Info("Fetching API Group version list")
	apiGroups, _, err = l.disClient.ServerGroupsAndResources()
	if err != nil {
		log.Errorf("Unable to fetch API group version : %s", err.Error())
		if !discovery.IsGroupDiscoveryFailedError(err) {
			log.Error(err, "Error while getting the resource list from discovery client")
			return apiGroups, err
		}
		log.Warnf("The Kubernetes server has an orphaned API service. Server reports: %s", err.Error())
		log.Warn("To fix this, kubectl delete apiservice <service-name>")
	}
	return apiGroups, nil
}

// CheckIsOpenshift checks whether the cluster is Openshift or not
func (l *LogCollector) CheckIsOpenshift() bool {
	_, err := l.disClient.ServerResourcesForGroupVersion("security.openshift.io/v1")
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
	}
	return true
}

// getResourceObjectsWithOwnerRef return all the objects which has ownerRef of CSV
func (l *LogCollector) getResourceObjectsWithOwnerRef(resourcePath string,
	resource *apiv1.APIResource) (objects unstructured.UnstructuredList) {
	allObjects := l.getResourceObjects(resourcePath, resource)

	for _, object := range allObjects.Items {
		ownerRefs := object.GetOwnerReferences()
		for idx := range ownerRefs {
			if strings.HasPrefix(ownerRefs[idx].Name, "k8s-triliovault") &&
				ownerRefs[idx].Kind == ClusterServiceVersionKind {
				objects.Items = append(objects.Items, object)
			}
		}
	}
	return objects
}

func (l *LogCollector) getClusterNodes() {

	var nodeList unstructured.UnstructuredList
	gvk := schema.GroupVersionKind{Kind: NodeKind, Version: CoreGv}
	nodeList.SetGroupVersionKind(gvk)
	err := l.k8sClient.List(context.Background(), &nodeList)
	if err != nil {
		log.Error("Unable to Fetch Node List")
	}
	for _, node := range nodeList.Items {
		resourceDir := filepath.Join(node.GetKind())
		yErr := l.writeYaml(resourceDir, node)
		if yErr != nil {
			log.Error("Unable to Write Node Yaml")
		}
	}
}

// WriteNs writes yaml of requested namespaces
func (l *LogCollector) WriteNs(namespaceObjects unstructured.UnstructuredList) error {
	for _, obj := range namespaceObjects.Items {
		resourceDir := filepath.Join(obj.GetKind())
		if !l.Clustered {
			if contains(l.Namespaces, obj.GetName()) {
				eLrr := l.writeYaml(resourceDir, obj)
				if eLrr != nil {
					return eLrr
				}
			}
		} else {
			eLrr := l.writeYaml(resourceDir, obj)
			if eLrr != nil {
				return eLrr
			}
		}
	}
	return nil
}

// checkNamespaces taken all given ns from user and checks the same in cluster and write it YAML's
func (l *LogCollector) checkNamespaces() error {
	log.Info("Checking Namespaces")
	coreGV, cgErr := l.getAPIGVResources(CoreGv)
	if cgErr != nil {
		return cgErr
	}
	namespaceResource := getResourceByName(coreGV, Namespaces)
	namespaceObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(CoreGv), &namespaceResource)
	allNamespaces := getObjectsNames(namespaceObjects)

	if len(l.Namespaces) != 0 && !l.isSubset(allNamespaces) {
		err := errors.New("specified namespaces doesn't exists in the cluster")
		return err
	}

	nErr := l.WriteNs(namespaceObjects)
	if nErr != nil {
		return nErr
	}

	return nil
}
