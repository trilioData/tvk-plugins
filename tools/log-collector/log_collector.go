package logcollector

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
)

type GroupVersionKind struct {
	Group   string `json:"group,omitempty"`
	Version string `json:"version,omitempty"`
	Kind    string `json:"kind"`
}

var (
	matchExpressionOperator = map[apiv1.LabelSelectorOperator]selection.Operator{
		apiv1.LabelSelectorOpIn:           selection.In,
		apiv1.LabelSelectorOpNotIn:        selection.NotIn,
		apiv1.LabelSelectorOpExists:       selection.Exists,
		apiv1.LabelSelectorOpDoesNotExist: selection.DoesNotExist,
	}
)

type LogCollector struct {
	OutputDir         string                        `json:"outputDirectory"`
	CleanOutput       bool                          `json:"keep-source-folder"`
	Clustered         bool                          `json:"clustered"`
	Namespaces        []string                      `json:"namespaces"`
	InstallNamespace  string                        `json:"installNamespace"`
	Loglevel          string                        `json:"logLevel"`
	K8sClient         client.Client                 `json:"-"`
	DisClient         *discovery.DiscoveryClient    `json:"-"`
	K8sClientSet      *kubernetes.Clientset         `json:"-"`
	KubeConfig        string                        `json:"kubeConfig"`
	LabelSelectors    []apiv1.LabelSelector         `json:"labels,omitempty"`
	GroupVersionKinds []GroupVersionKind            `json:"gvks"`
	RestConfig        *restclient.Config            `json:"-"`
	collectedPVCs     map[types.NamespacedName]bool `json:"-"` // Track collected PVCs to find their PVs
}

const (
	// maxTarFileBytes caps extraction of any single file from a tar stream
	maxTarFileBytes int64 = 100 * 1024 * 1024 // 100 MiB
	// maxGzipFileBytes caps decompression size for any single .gz file
	maxGzipFileBytes int64 = 50 * 1024 * 1024 // 50 MiB
)

// Executor abstracts the SPDY executor for testability
type Executor interface {
	Stream(options remotecommand.StreamOptions) error
}

// SpdyExecutorFactory creates a new SPDY executor. It is exported to enable test stubbing from external packages.
var SpdyExecutorFactory = func(config *restclient.Config, method string, url *url.URL) (Executor, error) {
	return remotecommand.NewSPDYExecutor(config, method, url)
}

// InitializeKubeClients initialize clients for kubernetes environment
func (l *LogCollector) InitializeKubeClients() error {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))

	acc, err := internal.NewEnv(l.KubeConfig, nil, scheme)
	if err != nil {
		log.Errorf("Invalid Kubeconfig : %s", l.KubeConfig)
		return err
	}

	l.K8sClient, l.DisClient, l.K8sClientSet = acc.GetRuntimeClient(), acc.GetDiscoveryClient(), acc.GetClientset()
	l.RestConfig = acc.GetRestConfig()
	l.DisClient.LegacyPrefix = "/api/"

	return nil
}

// CollectLogsAndDump collects call all the related resources of triliovault
func (l *LogCollector) CollectLogsAndDump() error {

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
	if !l.CleanOutput {
		err := os.RemoveAll(l.OutputDir)
		if err != nil {
			return err
		}
	}
	return nil
}

// getResourceObjects returns list of objects for given resourcePath
func (l *LogCollector) getResourceObjects(resourcePath string, resource *apiv1.APIResource) (objects unstructured.UnstructuredList) {

	if resource.Namespaced && !l.Clustered {
		for index := range l.Namespaces {
			var obj unstructured.UnstructuredList
			listPath := fmt.Sprintf("%s/namespaces/%s/%s", resourcePath, l.Namespaces[index], resource.Name)
			err := l.DisClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&obj)
			if err != nil {
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					log.Warnf("api error : %s", err.Error())
					continue
				}
				/* TODO() Currently error is ignore here, as we do not want to halt the log-collection utility because of
				single resources GET err. In future, if we add --continue-on-error type flag then, we'll update it and return
				error depending on --continue-on-error flag value
				if !discovery.IsGroupDiscoveryFailedError(err) {
					log.Errorf("%s", err.Error())
					return objects, err
				} */
				log.Warnf("%s", err.Error())
				continue
			}
			objects.Items = append(objects.Items, obj.Items...)
		}
		return objects
	}
	listPath := fmt.Sprintf("%s/%s", resourcePath, resource.Name)
	err := l.DisClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&objects)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			log.Warnf("%s", err.Error())
			return objects
		}
		/* TODO() Currently error is ignore here, as we do not want to halt the log-collection utility because of
		single resources GET err. In future, if we add --continue-on-error type flag then, we'll update it and return
		error depending on --continue-on-error flag value
		 if !discovery.IsGroupDiscoveryFailedError(err) {
			log.Errorf("%s", err.Error())
			return objects, nil
		} */
		log.Warnf("%s", err.Error())
		return unstructured.UnstructuredList{}
	}
	return objects
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

	// Sanitize NFS PVs before writing
	if obj.GetKind() == PersistentVolume && isNFSPV(obj) {
		obj = sanitizeNFSPV(obj)
		log.Infof("Sanitized NFS credentials from PV %s", objName)
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
	err := l.K8sClient.Get(context.Background(), types.NamespacedName{Name: objName, Namespace: objNs}, &podObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Warnf("%s", err.Error())
			return nil
		}
		log.Errorf("Unable to get the object : %s", err.Error())
		return err
	}
	containers := getContainers(&podObj)

	if l.isControlPlanePod(&podObj) {
		for i := range podObj.Spec.Containers {
			container := &podObj.Spec.Containers[i]
			// Only process logs from the triliovault-control-plane container
			if container.Name == internal.TriliovaultControlPlaneContainer {
				// Extract directly under the namespace path (no extra pod subfolders)
				destDir := resourcePath
				cpErr := l.CopyDirFromPod(objNs, objName, container.Name, internal.TriliovaultLogDir, destDir)
				if cpErr != nil {
					log.Warnf("Unable to copy control-plane logs from pod %s/%s: %s", objNs, objName, cpErr.Error())
				} else {
					// Decompress any .gz files in-place under destDir
					if dzErr := DecompressGzInDir(destDir); dzErr != nil {
						log.Warnf("Unable to decompress .gz files under %s: %s", destDir, dzErr.Error())
					}
				}
			}
		}
	}

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

	req := l.K8sClientSet.CoreV1().Pods(objNs).GetLogs(objName, &logOption)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		log.Errorf("Unable to get Logs for container %s : %s", container, err.Error())
		return nil
	}
	defer podLogs.Close()

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

	buf := make([]byte, 1024*1024) // 1MB Buffer
	for {
		n, err := podLogs.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Errorf("Error reading pod logs: %s", err.Error())
			return err
		}

		if n == 0 {
			break
		}

		_, err = outFile.Write(buf[:n])
		if err != nil {
			log.Errorf("Unable to write pod logs to the file: %s", err.Error())
			return err
		}
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
	if !l.CleanOutput {
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

	if nonLabeledResources.Has(resource.Kind) {
		log.Infof("Filtering '%s' Resource", resource.Kind)
		return l.getResourceObjects(resourcePath, resource), nil
	}

	if resource.Name == CRD {
		log.Infof("Filtering '%s' Resource", resource.Kind)
		allObjects, err = filterTvkSnapshotAndCSICRD(l.getResourceObjects(resourcePath, resource))
		if err != nil {
			return allObjects, err
		}
		return allObjects, nil
	}

	if resource.Name == Namespaces {
		log.Infof("Filtering '%s' Resource", resource.Kind)
		return l.filterInputNS(l.getResourceObjects(resourcePath, resource)), nil
	}

	if resource.Name == ClusterServiceVersion {
		log.Infof("Filtering '%s' Resource", resource.Kind)
		return filterTvkCSV(l.getResourceObjects(resourcePath, resource)), nil
	}

	if resource.Name == PersistentVolumeClaim {
		log.Infof("Filtering '%s' Resource", resource.Kind)
		return l.filterApplicationPVCs(resourcePath, resource), nil
	}

	if ((!nonLabeledResources.Has(resource.Kind) && resource.Namespaced) ||
		(l.Clustered && !resource.Namespaced)) && !excludeResources.Has(resource.Kind) {
		log.Infof("Filtering '%s' Resource", resource.Kind)
		allObjects = l.getResourceObjects(resourcePath, resource)
		l.filterTvkResourcesByLabel(&allObjects)
	}
	return allObjects, nil
}

// filterApplicationPVCs filters PVCs that are used by application pods or have TVK labels
func (l *LogCollector) filterApplicationPVCs(resourcePath string, resource *apiv1.APIResource) unstructured.UnstructuredList {
	var allObjects unstructured.UnstructuredList

	// Get all PVCs
	allPVCs := l.getResourceObjects(resourcePath, resource)

	// Filter PVCs with TVK labels
	var filteredPVCs unstructured.UnstructuredList
	for _, pvc := range allPVCs.Items {
		pvcLabels := pvc.GetLabels()
		if len(pvcLabels) != 0 {
			if checkLabelExist(K8STrilioVaultLabel, pvcLabels) ||
				checkLabelExist(K8STrilioVaultOpLabel, pvcLabels) ||
				checkLabelExist(K8STrilioVaultConsolePluginLabel, pvcLabels) ||
				(len(l.LabelSelectors) != 0 && MatchLabelSelectors(pvcLabels, l.LabelSelectors)) {
				filteredPVCs.Items = append(filteredPVCs.Items, pvc)
			}
		}
	}

	allObjects.Items = append(allObjects.Items, filteredPVCs.Items...)
	return allObjects
}

// collectApplicationPVCsFromPods collects PVCs that are used by collected pods
func (l *LogCollector) collectApplicationPVCsFromPods(pods []unstructured.Unstructured) error {
	if len(pods) == 0 {
		return nil
	}

	pvcSet := getPVCsUsedByPods(pods)
	if len(pvcSet) == 0 {
		return nil
	}

	log.Infof("Collecting application PVCs used by pods")

	pvcResourcePath := getAPIGroupVersionResourcePath(CoreGv)

	var pvcObjects unstructured.UnstructuredList
	for pvcNsName := range pvcSet {
		if l.collectedPVCs[pvcNsName] {
			continue
		}

		listPath := fmt.Sprintf("%s/namespaces/%s/%s/%s", pvcResourcePath, pvcNsName.Namespace, PersistentVolumeClaim, pvcNsName.Name)
		var pvc unstructured.Unstructured
		err := l.DisClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&pvc)
		if err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
				log.Warnf("PVC %s/%s not found or forbidden: %s", pvcNsName.Namespace, pvcNsName.Name, err.Error())
				continue
			}
			log.Warnf("Error getting PVC %s/%s: %s", pvcNsName.Namespace, pvcNsName.Name, err.Error())
			continue
		}

		pvcObjects.Items = append(pvcObjects.Items, pvc)
		l.collectedPVCs[pvcNsName] = true
	}

	if len(pvcObjects.Items) > 0 {
		_, err := l.writeObjectsAndLogs(pvcObjects, "PersistentVolumeClaim")
		if err != nil {
			return err
		}
	}

	return nil
}

// collectPVsForPVCs collects PVs that are bound to collected PVCs
func (l *LogCollector) collectPVsForPVCs() error {
	if len(l.collectedPVCs) == 0 {
		return nil
	}

	log.Infof("Collecting PVs bound to application PVCs")

	pvcResourcePath := getAPIGroupVersionResourcePath(CoreGv)
	pvNames := make(map[string]bool)

	for pvcNsName := range l.collectedPVCs {
		listPath := fmt.Sprintf("%s/namespaces/%s/%s/%s", pvcResourcePath, pvcNsName.Namespace, PersistentVolumeClaim, pvcNsName.Name)
		var pvc unstructured.Unstructured
		err := l.DisClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&pvc)
		if err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
				continue
			}
			log.Warnf("Error getting PVC %s/%s: %s", pvcNsName.Namespace, pvcNsName.Name, err.Error())
			continue
		}

		pvName := getPVNameFromPVC(pvc)
		if pvName != "" {
			pvNames[pvName] = true
		}
	}

	if len(pvNames) == 0 {
		return nil
	}

	pvResourcePath := getAPIGroupVersionResourcePath(CoreGv)
	var allPVs unstructured.UnstructuredList
	listPath := fmt.Sprintf("%s/%s", pvResourcePath, "persistentvolumes")
	err := l.DisClient.RESTClient().Get().AbsPath(listPath).Do(context.TODO()).Into(&allPVs)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			log.Warnf("PVs not found or forbidden: %s", err.Error())
			return nil
		}
		return err
	}

	var filteredPVs unstructured.UnstructuredList
	for _, pv := range allPVs.Items {
		pvName := pv.GetName()
		if pvNames[pvName] {
			if isNFSPV(pv) {
				pv = sanitizeNFSPV(pv)
			}
			filteredPVs.Items = append(filteredPVs.Items, pv)
		}
	}

	if len(filteredPVs.Items) > 0 {
		_, err = l.writeObjectsAndLogs(filteredPVs, PersistentVolume)
		if err != nil {
			return err
		}
	}

	return nil
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
	l.collectedPVCs = make(map[types.NamespacedName]bool)
	var collectedPods []unstructured.Unstructured

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

			if l.checkIfMatchesInputGVKs(&resources[index], groupVersion) {
				gvkObjs := l.getResourceObjects(getAPIGroupVersionResourcePath(groupVersion), &resources[index])
				resObjects.Items = append(resObjects.Items, gvkObjs.Items...)
			}

			if internal.CheckIsOpenshift(l.DisClient, internal.OcpAPIVersion) {
				ocpObj, oErr := l.getOcpRelatedResources(getAPIGroupVersionResourcePath(groupVersion),
					&resources[index], groupVersion)
				if oErr != nil {
					return oErr
				}
				resObjects.Items = append(resObjects.Items, ocpObj.Items...)
			}

			if resources[index].Kind == internal.NetworkPolicyKind {
				gvkObjs := l.getResourceObjects(internal.NetworkPolicyAPIVersion, &resources[index])
				for _, obj := range gvkObjs.Items {
					if obj.GetNamespace() == l.InstallNamespace {
						resObjects.Items = append(resObjects.Items, obj)
					}
				}
			}

			resourceMap, err := l.writeObjectsAndLogs(resObjects, resources[index].Kind)
			if err != nil {
				return err
			}

			if resources[index].Kind == Pod {
				collectedPods = append(collectedPods, resObjects.Items...)
			}

			if resources[index].Name == PersistentVolumeClaim {
				for _, pvc := range resObjects.Items {
					pvcNs := pvc.GetNamespace()
					if pvcNs == "" {
						pvcNs = DefaultNamespace
					}
					l.collectedPVCs[types.NamespacedName{Name: pvc.GetName(), Namespace: pvcNs}] = true
				}
			}

			for kind, NsName := range resourceMap {
				resourceMapList[kind] = NsName
			}
		}
	}

	err := l.collectApplicationPVCsFromPods(collectedPods)
	if err != nil {
		return err
	}

	err = l.collectPVsForPVCs()
	if err != nil {
		return err
	}

	err = l.getResourceEvents(&eventResource, resourceMapList)
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

		// nolint:gocritic // Creating a file path
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
		objectList := l.getResourceObjects(getAPIGroupVersionResourcePath(groupVersion), &trilioGVResources[index])
		// nolint:gocritic // Creating a file path
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

func (l *LogCollector) checkIfMatchesInputGVKs(resource *apiv1.APIResource, groupVersion string) bool {

	gvSplitter := strings.Split(groupVersion, "/")
	if len(gvSplitter) == 2 {
		resource.Group = gvSplitter[0]
		resource.Version = gvSplitter[1]
	} else {
		resource.Version = groupVersion
	}

	for idx := range l.GroupVersionKinds {
		if strings.EqualFold(l.GroupVersionKinds[idx].Group, resource.Group) &&
			strings.EqualFold(l.GroupVersionKinds[idx].Version, resource.Version) &&
			strings.EqualFold(l.GroupVersionKinds[idx].Kind, resource.Kind) {
			return true
		}
	}
	return false
}

// getAPIResourceList returns the list of all API Groups of the supported resources for all groups and versions.
func (l *LogCollector) getAPIResourceList() (map[string][]apiv1.APIResource, error) {

	resourceMapList := make(map[string][]apiv1.APIResource)
	log.Info("Fetching API Group version list")
	resourceList, err := l.DisClient.ServerPreferredResources()
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

// nolint:gocyclo // all OCP related resources' logic
// getOcpRelatedResources return all the objects which has ownerRef of CSV
func (l *LogCollector) getOcpRelatedResources(resourcePath string,
	resource *apiv1.APIResource, groupVersion string) (objects unstructured.UnstructuredList, err error) {

	//  This is the default ingress-controller
	if resource.Kind == internal.IngressController && groupVersion == OCPOperatorAPIVersion {
		l.Namespaces = append(l.Namespaces, OCPConfigNs)
	}

	allObjects := l.getResourceObjects(resourcePath, resource)

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

		labels := object.GetLabels()
		if ownerKind, exists := labels[OlmOwnerKind]; exists && ownerKind == ClusterServiceVersionKind {
			if owner, ownerExist := labels[OlmOwner]; ownerExist && strings.HasPrefix(owner, TrilioPrefix) {
				if ownerWebhook, ownerWebhookExist := labels[OlmWebhook]; ownerWebhookExist &&
					strings.Contains(ownerWebhook, TrilioDomain) {
					objects.Items = append(objects.Items, object)
				}
			}
		}

		// This contains cluster wide configuration for ingress
		if groupVersion == OCPConfigAPIVersion && object.GetKind() == internal.IngressKind && object.GetName() == OCPIngress {
			objects.Items = append(objects.Items, object)
		}

		if groupVersion == OCPOperatorAPIVersion && object.GetKind() == internal.IngressController &&
			object.GetName() == OCPConfig {
			objects.Items = append(objects.Items, object)
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
	err = l.K8sClient.List(context.Background(), &namespaces)
	if err != nil {
		log.Errorf("%s", err.Error())
		return err
	}

	for idx := range namespaces.Items {
		set.Insert(namespaces.Items[idx].Name)
		if namespaces.Items[idx].Labels != nil {
			if _, hasTrilioLabel := namespaces.Items[idx].Labels[internal.TrilioLabelKey]; hasTrilioLabel {
				l.InstallNamespace = namespaces.Items[idx].Name
			}
		}
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

	eventObjects := l.getResourceObjects(getAPIGroupVersionResourcePath(CoreGv), eventResource)
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

// filterInputNS returns list of Namespaces Object given by user input in --namespaces flag
func (l *LogCollector) filterInputNS(nsObjs unstructured.UnstructuredList) unstructured.UnstructuredList {

	if l.Clustered {
		return nsObjs
	}

	var filteredNSObjects unstructured.UnstructuredList
	nsNames := sets.NewString(l.Namespaces...)

	for _, nsObj := range nsObjs.Items {
		if nsNames.Has(nsObj.GetName()) {
			filteredNSObjects.Items = append(filteredNSObjects.Items, nsObj)
		}
	}
	return filteredNSObjects
}

// isControlPlanePod checks if pod has the specific label identifying control-plane instance
func (l *LogCollector) isControlPlanePod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	labels := pod.GetLabels()
	if labels == nil {
		return false
	}
	val, ok := labels[internal.K8sInstanceLabel]
	return ok && val == internal.ManagedByControlPlaneLabelValue
}

// execInPod executes a command in the specified pod container
func (l *LogCollector) execInPod(namespace, podName, containerName string, cmd []string) (stdout, stderr string, err error) {
	req := l.K8sClientSet.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, clientgoscheme.ParameterCodec)

	exec, err := SpdyExecutorFactory(l.RestConfig, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	if err := exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Tty:    false,
	}); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), err
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

// checkDirectoryHasFiles checks if a directory exists and has files to copy
// Returns (hasFiles, error) where:
// - hasFiles: true if directory exists and has files, false if no files found
// - error: non-nil only for genuine errors (network issues, permission problems, etc.)
func (l *LogCollector) checkDirectoryHasFiles(namespace, podName, containerName, srcDir string) (bool, error) {
	_, stderr, err := l.execInPod(namespace, podName, containerName, []string{"ls", "-la", srcDir})
	if err != nil {
		// Check if this is a "no files found" case or a genuine error
		if strings.Contains(stderr, "No such file or directory") || strings.Contains(stderr, "not found") {
			// Directory doesn't exist or is empty - this is expected when file logging is disabled
			log.Debugf("Source directory %s does not exist or is empty in pod %s/%s: %s", srcDir, namespace, podName, stderr)
			return false, nil
		}
		// This is a genuine error (permission denied, network issues, etc.)
		log.Errorf("Failed to check directory %s in pod %s/%s: %s", srcDir, namespace, podName, stderr)
		return false, fmt.Errorf("failed to check directory: %w", err)
	}

	// Directory exists and is accessible
	return true, nil
}

// CopyDirFromPod tars a directory inside the container and extracts it to destDir
//
// Directory structure in the log bundle:
// The captured log files will be organized as follows:
//   - destDir/
//     ├── <log-file-1>          (e.g., tvk-manager.log, tvk-webhook.log)
//     ├── <log-file-2.gz>       (compressed log files, will be decompressed later)
//     └── <subdirectories>/     (any subdirectories from srcDir are preserved)
//     └── <nested-files>
//
// This maintains the original directory structure from the pod's srcDir while placing
// all files under the specified destDir in the log bundle.
func (l *LogCollector) CopyDirFromPod(namespace, podName, containerName, srcDir, destDir string) error {
	if l.RestConfig == nil || l.K8sClientSet == nil {
		return fmt.Errorf("kubernetes clients are not initialized")
	}

	// Check if directory has files to copy
	hasFiles, err := l.checkDirectoryHasFiles(namespace, podName, containerName, srcDir)
	if err != nil {
		return fmt.Errorf("failed to check directory %s in pod %s/%s: %w", srcDir, namespace, podName, err)
	}
	if !hasFiles {
		log.Infof("Skipping copy from pod %q (container %q) in namespace %q: no files found in directory %q",
			podName, containerName, namespace, srcDir)
		return nil // No files to copy, skip
	}

	// Stream tar to a temp file to avoid pipe races
	if mkErr := os.MkdirAll(destDir, 0755); mkErr != nil {
		return mkErr
	}
	tarFile, tErr := os.CreateTemp(destDir, "tvklog-*.tar")
	if tErr != nil {
		return tErr
	}
	defer func() { _ = os.Remove(tarFile.Name()) }()

	// Execute tar command and stream to file
	cmd := []string{"tar", "cf", "-", "-C", srcDir, "."}
	req := l.K8sClientSet.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, clientgoscheme.ParameterCodec)

	exec, err := SpdyExecutorFactory(l.RestConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	if err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: tarFile,
		Stderr: &stderr,
		Tty:    false,
	}); err != nil {
		_ = tarFile.Close()
		return fmt.Errorf("exec stream error: %w; stderr: %s", err, stderr.String())
	}
	if cErr := tarFile.Close(); cErr != nil {
		return cErr
	}

	return l.extractTarFile(tarFile.Name(), destDir)
}

// extractTarFile extracts a tar file to the destination directory
func (l *LogCollector) extractTarFile(tarPath, destDir string) error {
	in, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer in.Close()

	tr := tar.NewReader(in)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(hdr.Name)
		targetPath, err := ensureWithinDir(destDir, filepath.Join(destDir, cleanName))
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if hdr.Size < 0 {
				return fmt.Errorf("invalid negative file size for %s", hdr.Name)
			}
			if hdr.Size > maxTarFileBytes {
				return fmt.Errorf("tar file %s exceeds allowed size of %d bytes", hdr.Name, maxTarFileBytes)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err = io.CopyN(outFile, tr, hdr.Size); err != nil {
				_ = outFile.Close()
				return err
			}
			_ = outFile.Close()
		}
	}
	return nil
}

// ensureWithinDir ensures the target path stays within base to avoid path traversal
func ensureWithinDir(base, target string) (string, error) {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("illegal file path outside destination: %s", target)
	}
	return target, nil
}

// DecompressGzInDir walks a directory and decompresses all .gz files in-place
func DecompressGzInDir(root string) error {
	var walkFn = func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".gz") {
			return nil
		}
		return decompressGzFile(filePath)
	}

	return filepath.Walk(root, walkFn)
}

// decompressGzFile decompresses a single .gz file
func decompressGzFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	// The size limit enforcement happens below using io.LimitedReader.
	// #nosec G110 -- inputs are cluster-generated logs; see bounded copy via LimitedReader below
	gzr, err := gzip.NewReader(in)
	if err != nil {
		// Not a valid gzip file; skip
		log.Debugf("Skipping gzip decompression for %s: %s", path, err.Error())
		return nil
	}
	defer gzr.Close()

	outPath := strings.TrimSuffix(path, ".gz")
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer os.Remove(path) // Remove source .gz file in all cases
	defer outFile.Close()

	// Cap the number of decompressed bytes per file (decompression bomb protection)
	lr := &io.LimitedReader{R: gzr, N: maxGzipFileBytes + 1}
	written, err := io.Copy(outFile, lr)
	if err != nil && !errors.Is(err, io.EOF) {
		_ = os.Remove(outPath) // Clean up on error
		return err
	}
	if written > maxGzipFileBytes {
		_ = os.Remove(outPath) // Clean up on error
		return fmt.Errorf("decompressed output exceeds limit of %d bytes: %s", maxGzipFileBytes, outPath)
	}

	return nil
}
