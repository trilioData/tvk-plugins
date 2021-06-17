package logcollector

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/trilioData/tvk-plugins/tools"
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
	KubeConfig   string
}

// initializeKubeClients initialize clients for kubernetes environment
func (l *LogCollector) initializeKubeClients() error {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))

	acc, err := tools.NewEnv(l.KubeConfig, scheme)
	if err != nil {
		return err
	}
	l.k8sClient, l.disClient, l.k8sClientSet = acc.GetRuntimeClient(), acc.GetDiscoveryClient(), acc.GetClientset()
	l.disClient.LegacyPrefix = "/api/"

	return nil
}

// CollectLogsAndDump collects call all the related resources of triliovault
func (l *LogCollector) CollectLogsAndDump() error {

	if err := l.initializeKubeClients(); err != nil {
		return err
	}

	nsErr := l.checkIfNamespacesExist()
	if nsErr != nil {
		return nsErr
	}

	resourceMapList, apiErr := l.getAPIResourceList()
	if apiErr != nil {
		return apiErr
	}

	fErr := l.filteringResources(resourceMapList)
	if fErr != nil {
		return fErr
	}

	// Zip Directory
	zErr := l.zipDir()
	if zErr != nil {
		return zErr
	}

	// check for clean output flag. if true, clean.
	if l.CleanOutput {
		err := os.RemoveAll(l.OutputDir)
		if err != nil {
			return err
		}
	}
	return nil
}

// getResourceObjects returns list of objects for given resourcePath
func (l *LogCollector) getResourceObjects(resourcePath string, resource *apiv1.APIResource) (objects unstructured.UnstructuredList,
	err error) {

	if resource.Namespaced && !l.Clustered {
		for index := range l.Namespaces {
			var obj unstructured.UnstructuredList
			listPath := fmt.Sprintf("%s/namespaces/%s/%s", resourcePath, l.Namespaces[index], resource.Name)
			err = l.disClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&obj)
			if err != nil {
				if !discovery.IsGroupDiscoveryFailedError(err) {
					log.Errorf("%s", err.Error())
					return objects, err
				}
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					log.Warnf("%s", err.Error())
					return objects, nil
				}
				/* TODO() Currently error is ignore here, as we do not want to halt the log-collection utility because of
				single resources GET err. In future, if we add --continue-on-error type flag then, we'll update it and return
				error depending on --continue-on-error flag value */
				log.Warnf("%s", err.Error())
				return unstructured.UnstructuredList{}, nil
			}
			objects.Items = append(objects.Items, obj.Items...)
		}
		return objects, nil
	}
	listPath := fmt.Sprintf("%s/%s", resourcePath, resource.Name)
	err = l.disClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&objects)
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			log.Errorf("%s", err.Error())
			return objects, err
		}
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			log.Warnf("%s", err.Error())
			return objects, nil
		}
		/* TODO() Currently error is ignore here, as we do not want to halt the log-collection utility because of
		single resources GET err. In future, if we add --continue-on-error type flag then, we'll update it and return
		error depending on --continue-on-error flag value */
		log.Warnf("%s", err.Error())
		return unstructured.UnstructuredList{}, nil
	}
	return objects, nil
}

// writeEventsToFile writes events to the file
func (l *LogCollector) writeEventsToFile(events map[string][]map[string]interface{}) error {

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
			log.Warnf("%s", err.Error())
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

// filterResourceObjects filter objects on the basis of resource Type.
func (l *LogCollector) filterResourceObjects(resourcePath string,
	resource *apiv1.APIResource) (allObjects unstructured.UnstructuredList, err error) {

	if (!resource.Namespaced && clusteredResources.Has(resource.Kind)) ||
		(resource.Namespaced && !excludeResources.Has(resource.Kind)) {
		log.Infof("Fetching '%s' Resource", resource.Kind)
		allObjects, err = l.getResourceObjects(resourcePath, resource)
		if err != nil {
			return allObjects, err
		}

		if resource.Name == CRD {
			allObjects, err = filterTvkSnapshotAndCSICRD(allObjects)
			if err != nil {
				return allObjects, err
			}
		}

		if resource.Name == Namespaces && !l.Clustered {
			allObjects = filterInputNS(allObjects, l.Namespaces)
		}

		if resource.Name == ClusterServiceVersion {
			allObjects = filterTvkCSV(allObjects)
		}
	}

	if !nonLabeledResources.Has(resource.Kind) &&
		!clusteredResources.Has(resource.Kind) {
		filterTvkResourcesByLabel(&allObjects)
	}
	return allObjects, nil
}

func (l *LogCollector) filteringResources(resourceGroup map[string][]apiv1.APIResource) error {
	// These operations are performed in the following lines:
	// 1. Iterating through all the resources.
	// 2. Filtering only those resources that we need from all the available resources obtained above.
	// 3. Iterating through the filtered resources to fetch their respective objects based on the group from which
	//    they belong and labels
	//    e.g. fetching all pods from core group with the label 'app.kubernetes.io/part-of':'k8s-triliovault'
	// 4. Collecting pod names specifically that is later required by events
	// 5. Collecting list of all resource objects and printing their YAML's in their respective resource folder under
	//    their respective namespaces. In case of pods, logs are also collected

	log.Info("Filtering Resources")

	resourceMapList := make(map[string][]types.NamespacedName)
	var eventResource apiv1.APIResource

	for groupVersion, resources := range resourceGroup {

		if groupVersion == TriliovaultGroupVersion {
			err := l.getTrilioGroupResources(resources, groupVersion)
			if err != nil {
				return err
			}
			continue
		}

		for index := range resources {

			if resources[index].Name == Events {
				eventResource = resources[index]
				continue
			}

			var resObjects unstructured.UnstructuredList
			resObject, err := l.filterResourceObjects(getAPIGroupVersionResourcePath(groupVersion), &resources[index])
			if err != nil {
				return err
			}
			resObjects.Items = append(resObjects.Items, resObject.Items...)

			if l.CheckIsOpenshift() {
				ocpObj, oErr := l.getOcpResourcesByOwnerRef(getAPIGroupVersionResourcePath(groupVersion), &resources[index])
				if oErr != nil {
					return oErr
				}
				resObjects.Items = append(resObjects.Items, ocpObj.Items...)
			}

			resourceMap, err := l.writeObjectsAndLogs(resObjects, resources[index].Kind)
			if err != nil {
				return err
			}

			for kind, NsName := range resourceMap {
				resourceMapList[kind] = NsName
			}
		}
	}

	err := l.getResourceEvents(&eventResource, resourceMapList)
	if err != nil {
		return err
	}

	return nil
}

// writeObjectsAndLogs writes objects YAML and logs to file
func (l *LogCollector) writeObjectsAndLogs(objects unstructured.UnstructuredList, kind string) (map[string][]types.NamespacedName, error) {

	var nsName []types.NamespacedName
	resourceMap := make(map[string][]types.NamespacedName)

	for _, obj := range objects.Items {
		oName := obj.GetName()
		oNs := obj.GetNamespace()

		nsName = append(nsName, types.NamespacedName{Name: oName, Namespace: oNs})
		resourceMap[kind] = nsName

		resourceDir := filepath.Join(kind)
		if kind == Pod {
			eLrr := l.writeLogs(resourceDir, obj)
			if eLrr != nil {
				return resourceMap, eLrr
			}
		}
		eLrr := l.writeYaml(resourceDir, obj)
		if eLrr != nil {
			return resourceMap, eLrr
		}
	}

	return resourceMap, nil
}

// getTrilioGroupResources collects all the resources related to trilio and writes the YAML
func (l *LogCollector) getTrilioGroupResources(trilioGVResources []apiv1.APIResource, groupVersion string) error {
	log.Info("Checking Trilio Group")
	for index := range trilioGVResources {
		objectList, err := l.getResourceObjects(getAPIGroupVersionResourcePath(groupVersion), &trilioGVResources[index])
		if err != nil {
			return err
		}
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
	return nil
}

// getAPIResourceList returns the list of all API Groups of the supported resources for all groups and versions.
func (l *LogCollector) getAPIResourceList() (map[string][]apiv1.APIResource, error) {

	resourceMapList := make(map[string][]apiv1.APIResource)
	log.Info("Fetching API Group version list")
	_, resourceList, err := l.disClient.ServerGroupsAndResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			log.Error(err, "Error while getting the resource list from discovery client")
			return resourceMapList, err
		}
		log.Warnf("The Kubernetes server has an orphaned API service. Server reports: %s", err.Error())
		log.Warn("To fix this, kubectl delete apiservice <service-name>")
	}

	for _, resources := range resourceList {
		for idx := range resources.APIResources {
			for _, verb := range resources.APIResources[idx].Verbs {
				if verb == Verblist {
					resourceMapList[resources.GroupVersion] = append(resourceMapList[resources.GroupVersion],
						resources.APIResources[idx])
				}
			}
		}
	}

	return resourceMapList, nil
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

// getOcpResourcesByOwnerRef return all the objects which has ownerRef of CSV
func (l *LogCollector) getOcpResourcesByOwnerRef(resourcePath string,
	resource *apiv1.APIResource) (objects unstructured.UnstructuredList, err error) {

	allObjects, err := l.getResourceObjects(resourcePath, resource)
	if err != nil {
		return objects, err
	}

	for _, object := range allObjects.Items {

		if object.GetKind() == SubscriptionKind {
			startingCSV, _, err := unstructured.NestedString(object.Object, "spec", "startingCSV")
			if err != nil {
				log.Errorf("Unable to get startingCSV : %s", err.Error())
				return objects, err
			}
			name, _, nErr := unstructured.NestedString(object.Object, "spec", "name")
			if nErr != nil {
				log.Errorf("Unable to get name : %s", nErr.Error())
				return objects, err
			}

			if strings.HasPrefix(startingCSV, TrilioPrefix) &&
				strings.HasPrefix(name, TrilioPrefix) {
				objects.Items = append(objects.Items, object)
			}
		}

		ownerRefs := object.GetOwnerReferences()
		for idx := range ownerRefs {
			// Condition Check for CSV as OwnerRef or Subscription as OwnerRef (In Case of InstallPlan)
			if (strings.HasPrefix(ownerRefs[idx].Name, TrilioPrefix) &&
				ownerRefs[idx].Kind == ClusterServiceVersionKind &&
				!excludeResources.Has(object.GetKind())) ||
				(ownerRefs[idx].Kind == SubscriptionKind &&
					strings.HasPrefix(ownerRefs[idx].Name, TrilioPrefix) &&
					object.GetKind() == InstallPlanKind) {
				objects.Items = append(objects.Items, object)
			}
		}
	}
	return objects, nil
}

// checkIfNamespacesExist take all given namespaces from user and checks the same in cluster if it exist
func (l *LogCollector) checkIfNamespacesExist() (err error) {

	log.Info("Checking if given namespaces are valid")
	set := make(sets.String)
	var nonExistNs []string

	var namespaces corev1.NamespaceList
	err = l.k8sClient.List(context.Background(), &namespaces)
	if err != nil {
		log.Errorf("%s", err.Error())
		return err
	}

	for idx := range namespaces.Items {
		set.Insert(namespaces.Items[idx].Name)
	}

	for _, ns := range l.Namespaces {
		ns = strings.Trim(ns, " ")
		if !set.Has(ns) {
			nonExistNs = append(nonExistNs, ns)
		}
	}

	if len(nonExistNs) != 0 {
		return errors.Errorf("specified namespaces doesn't exists in the cluster : %s", nonExistNs)
	}

	return nil
}

// getResourceEvents write YAML's for all events of resources related to trilio
func (l *LogCollector) getResourceEvents(eventResource *apiv1.APIResource, resourceMap map[string][]types.NamespacedName) error {

	eventObjects, err := l.getResourceObjects(getAPIGroupVersionResourcePath(CoreGv), eventResource)
	if err != nil {
		log.Errorf("Unable to Write Events : %s", err.Error())
		return err
	}
	events, aErr := aggregateEvents(eventObjects, resourceMap)
	if aErr != nil {
		log.Errorf("Unable to process Events : %s", aErr.Error())
		return aErr
	}

	eErr := l.writeEventsToFile(events)
	if eErr != nil {
		log.Errorf("Unable to Write Events : %s", eErr.Error())
		return eErr
	}
	return nil
}
