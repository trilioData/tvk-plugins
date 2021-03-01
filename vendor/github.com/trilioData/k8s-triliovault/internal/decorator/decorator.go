package decorator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	logger "github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crd "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/kube"
)

var (
	// WarningProneResources is the list of resources Kind for which the warnings is checked
	WarningProneResources = sets.NewString(
		internal.DaemonSetKind,
		internal.DeploymentKind,
		internal.PodKind,
		internal.ReplicaSetKind,
		internal.ReplicationControllerKind,
		internal.StatefulSetKind,
		internal.ServiceKind,
	)

	commonWarningsFuncList = []func(unstructured.Unstructured, helpers.WarningMap){
		checkForHostNetwork,
		checkForHostPort,
		checkForNodeSelector,
		checkForNodeAffinity,
	}

	// WarningsFuncMap is the map to store the pre defines list of functions to run to check for warning for the type of resource
	WarningsFuncMap = map[string][]func(unstructured.Unstructured, helpers.WarningMap){
		internal.DaemonSetKind:             commonWarningsFuncList,
		internal.DeploymentKind:            commonWarningsFuncList,
		internal.PodKind:                   commonWarningsFuncList,
		internal.ReplicaSetKind:            commonWarningsFuncList,
		internal.ReplicationControllerKind: commonWarningsFuncList,
		internal.StatefulSetKind:           commonWarningsFuncList,
		internal.ServiceKind:               {checkForNodePort},
	}

	runtimeScheme   = runtime.NewScheme()
	_               = corev1.AddToScheme(runtimeScheme)
	_               = appsV1.AddToScheme(runtimeScheme)
	_               = crd.AddToScheme(runtimeScheme)
	KubeAccessor, _ = kube.NewEnv(runtimeScheme)
)

// ResourceHelper exposes several utility functions to perform in the concrete type of resource which implements it
type ResourceHelper interface {
	// ChecksForStatus checks for the status of pod resources for the implementers
	ChecksForStatus(ctx context.Context, cl client.Client, ns string) bool

	// ChecksForWarnings returns the list of probable warnings for the resource which may occur at the time of restore
	ChecksForWarnings() []string

	// IsResourceDeployable runs the dry run for the resource on Create or Patch operations
	IsResourceDeployable(a *kube.Accessor, ns string, checkIfPatchPossible bool) error

	// IsResourceExists checks if the passed resource already exists in the passed namespace
	IsResourceExists(a *kube.Accessor, objName, restoreNamespace string) bool

	// Cleanup cleans the overall resource with the default sa, namespaced and clustered scope fields, hard coded ports etc
	Cleanup()

	// CleanNamespacedScopeMutations only cleans only the clustered scope and namespaced scope item from the associated resource
	CleanNamespacedScopeMutations()

	// ExportMetadata aims to implement the `kubectl export` command functionality
	ExportMetadata()

	// FromUnstructured converts the associated object from unstructured resource to the concrete type passed
	FromUnstructured(obj runtime.Object) error

	// ToUnstructured converts the concrete type object to the unstructured object
	ToUnstructured(obj interface{}) error

	// PerformPreBackupProcess aims to perform the pre-backup processing for the resource and returns the warnings if any
	PerformPreBackupProcess(ctx context.Context, cl client.Client, ns string) []string
}

// UnstructResource is the internal type of unstructured resource to implement further wrapper functions around it
type UnstructResource unstructured.Unstructured

// ChecksForStatus is the verification function which checks the conditional statuses of the Pods under the resource
//
// It works on Unstructured resource type, if the kind is Pod then it converts to concrete Pod type
// and calls the helper `VerifyPodStatus`. If resource is of any other type but in `apps` api group,
// it finds the pods and calls the verify function on them.
//
// For the resources which are not Pods and not in `apps` api group are ignored
func (u *UnstructResource) ChecksForStatus(ctx context.Context, cl client.Client, ns string, warnings helpers.WarningMap) error {
	item := unstructured.Unstructured(*u)
	kind := item.GetObjectKind().GroupVersionKind().Kind

	if kind == internal.PodKind {
		pod := &corev1.Pod{}
		if pErr := u.FromUnstructured(pod); pErr != nil {
			return fmt.Errorf("couldn't check the status of %s", item.GetName())
		}
		if !verifyPodStatus(pod) {
			helpers.CheckAndAddWarning(item.GroupVersionKind().String(), item.GetName(),
				internal.PodNotRunningWarning, item.GetLabels(), warnings)
		}
		return nil
	}

	if value, present := internal.AppsResourcesMap[kind]; present {
		if aErr := u.FromUnstructured(value); aErr != nil {
			return fmt.Errorf("couldn't check the status of %s", item.GetName())
		}

		tmpPodList, gpErr := kube.GetPodsOfAppKind(ctx, cl, ns, value)
		if gpErr != nil {
			return gpErr
		}

		if tmpPodList != nil {
			for podIndex := range tmpPodList.Items {
				var podUnstruct UnstructResource
				pod := tmpPodList.Items[podIndex]
				tErr := podUnstruct.ToUnstructured(&pod)
				if tErr != nil {
					return fmt.Errorf("failed to convert to unstruct while getting owner of the pod: %s", tErr.Error())
				}
				parentRes, err := helpers.FindParentResource(KubeAccessor, unstructured.Unstructured(podUnstruct), ns)
				if err != nil {
					return err
				}
				// Check if pod status is not empty and then check the conditions
				if !reflect.DeepEqual(pod.Status, corev1.PodStatus{}) && parentRes.GetUID() == item.GetUID() && !verifyPodStatus(&pod) {
					helpers.CheckAndAddWarning(corev1.SchemeGroupVersion.WithKind(internal.PodKind).String(), pod.GetName(),
						internal.PodNotRunningWarning, pod.GetLabels(), warnings)
				}
			}
		}
	}
	return nil
}

// ChecksForWarnings is the utility works on Unstructured resource type
// which calls the appropriate helper functions for warning checks
// such as NodePort, HostPort, Node Affinity, host Network etc
//
// Resources which needs to be checked are stored in warningProneResources
// and only for them, the functions are called
func (u *UnstructResource) ChecksForWarnings(warnings helpers.WarningMap) {
	item := unstructured.Unstructured(*u)
	needToCheck := WarningProneResources.Has(item.GetKind())
	if !needToCheck {
		return
	}

	if funcList, ok := WarningsFuncMap[item.GetKind()]; ok {
		for warnFuncIndex := range funcList {
			warnFunc := funcList[warnFuncIndex]
			warnFunc(item, warnings)
		}
	}
}

// IsResourceExists is the helper which returns true only if the resource exist in the passed namespace
// using the object name and its GVK in the restore namespace
func (u *UnstructResource) IsResourceExists(a *kube.Accessor, objName, restoreNamespace string) bool {
	// IsResourceExists checks if given resource is present or not in the given namespace

	un := unstructured.Unstructured(*u)
	gvk := un.GroupVersionKind()

	if _, err := a.GetUnstructuredObject(types.NamespacedName{Namespace: restoreNamespace, Name: objName}, gvk); err == nil {
		logger.Info("[Alert] Resource is present name: ", objName,
			"GVK: ", gvk, "restore namespace:", restoreNamespace)
		return true
	}

	logger.Info("Resource is not present name: ", objName, "GVK: ", gvk, "restore namespace:", restoreNamespace)
	return false
}

// IsResourceDeployable works on the Unstructured resource which checks if the resource is deployable (create).
// and/or patchable
//
// It performs the dry run client CREATE call and returns warning if error occurred.
func (u *UnstructResource) IsResourceDeployable(a *kube.Accessor, ns string) error {

	item := unstructured.Unstructured(*u)
	dryRunErr := a.CreateUnstructuredObject(ns, &item, &client.CreateOptions{
		DryRun: []string{"All"}})
	// If dry run throws already exists error then return nil
	if errors.IsAlreadyExists(dryRunErr) {
		return nil
	}
	if dryRunErr != nil {
		logger.Error(dryRunErr)
		return fmt.Errorf("create dry run failed for object [%s : %s] with error %+v",
			item.GetObjectKind().GroupVersionKind().String(), item.GetName(), dryRunErr)
	}

	return nil
}

// IsResourcePatchable checks if resource is patchable if patch flag is provided.
// It takes a param `checkIfPatchPossible`, if true, it also checks if the resource is patchable
// by calling dry run on internal `ThreeWayPatchUnstructuredObject`
func (u *UnstructResource) IsResourcePatchable(a *kube.Accessor, ns string, checkIfPatchPossible bool) error {
	item := unstructured.Unstructured(*u)
	if checkIfPatchPossible {
		tempObj := item.DeepCopy()
		cleaner := &UnstructResource{Object: tempObj.Object}
		cleaner.Cleanup()
		tempObj.Object = cleaner.Object

		if patchErr := a.ThreeWayPatchUnstructuredObject(ns, tempObj.GroupVersionKind(), tempObj,
			&client.PatchOptions{DryRun: []string{"All"}}); patchErr != nil {
			logger.Error(patchErr)
			return fmt.Errorf("patch dry run failed for object [%s : %s]",
				item.GetObjectKind().GroupVersionKind().String(), tempObj.GetName())
		}

		logger.Info("Patch dry run successful for object GVK: ",
			item.GetObjectKind().GroupVersionKind().String(), "name: ", tempObj.GetName())
	}

	return nil
}

// Cleanup remove cluster state, status, node specific and namespaced specific fields and also default mutations
// from the Unstructured type resource
func (u *UnstructResource) Cleanup() {

	// clean using the manual --export type functionality
	logger.Info("cleaning metadata of resource to have --export functionality")
	u.ExportMetadata()

	// cleanup default token volumes from containers
	logger.Info("Cleaning the namespaced scope mutated metadata")
	u.CleanNamespacedScopeMutations()
}

// CleanNamespacedScopeMutations is the utility function on the internal Unstructured type object.
// It cleans only namespaced and node scoped fields from the resource.
// Currently it removes the default SA Token mounted as a volume in the initContainers and Containers
func (u *UnstructResource) CleanNamespacedScopeMutations() {
	var containers, initContainers []interface{}

	unStr := unstructured.Unstructured(*u)
	var volumes []interface{}

	if specMap, found, err := unstructured.NestedMap(unStr.Object, "spec"); found && err == nil {
		if v, ok := specMap["volumes"]; v != nil && ok {
			volumes, found, err = unstructured.NestedSlice(unStr.Object, "spec", "volumes")
			if found && err != nil {
				logger.Error(err)
				return
			}
		}
	}

	var found bool
	var err error

	if containers, found, err = unstructured.NestedSlice(unStr.Object, "spec",
		"containers"); containers != nil && found && err != nil {
		logger.Error(err)
		return
	}

	initContainers, found, err = unstructured.NestedSlice(unStr.Object, "spec", "initContainers")
	if initContainers != nil && found && err != nil {
		logger.Error(err)
		return
	}

	if len(containers) > 0 {
		containers, volumes = removeDefaultSaTokenVolumes(containers, volumes)
	}

	if len(initContainers) > 0 {
		initContainers, volumes = removeDefaultSaTokenVolumes(initContainers, volumes)
	}

	if len(volumes) > 0 {
		_ = unstructured.SetNestedSlice(unStr.Object, volumes, "spec", "volumes")
	} else {
		unstructured.RemoveNestedField(unStr.Object, "spec", "volumes")
	}

	if len(containers) > 0 {
		_ = unstructured.SetNestedSlice(unStr.Object, containers, "spec", "containers")
	} else {
		unstructured.RemoveNestedField(unStr.Object, "spec", "containers")
	}

	if len(initContainers) > 0 {
		_ = unstructured.SetNestedSlice(unStr.Object, initContainers, "spec", "containers")
	} else {
		unstructured.RemoveNestedField(unStr.Object, "spec", "initContainers")
	}

	u.Object = unStr.Object
}

// ExportMetadata removes the clustered, namespaced scope fields and also the default mutations by the api-server webhook
// UID, Namespace, SelfLink, ResourceVersion, Generation, TimeStamps, status, NodeName are the fields which are cleaned
// ClusterIP is the field handled for the ClusterIP type Service when its value is not explicitly assigned as NONE
func (u *UnstructResource) ExportMetadata() {
	logger.Info("cleaning the clustered level fields which are irrelevant and mutated by kube-apiserver")

	unStr := unstructured.Unstructured(*u)
	unStr.SetUID("")
	unStr.SetSelfLink("")
	unStr.SetResourceVersion("")
	unStr.SetCreationTimestamp(metav1.Time{})
	unStr.SetGeneration(0)
	unStr.SetNamespace("")
	unstructured.RemoveNestedField(unStr.Object, "status")
	unstructured.RemoveNestedField(unStr.Object, "spec", "nodeName")

	// Need to retain clusterIP=None configuration so that 3-way merge patch will not contain clusterIP=Null
	// as this will give conflict while patching because spec.clusterIP field is immutable
	// TODO: Need to find such conflicting fields or values and handle them properly in cleanup() func
	objSpec, found, err := unstructured.NestedMap(unStr.Object, "spec")
	if found && err == nil {
		if v, ok := objSpec["clusterIP"]; v != "None" && ok {
			unstructured.RemoveNestedField(unStr.Object, "spec", "clusterIP")
		}
	}

	unstructured.RemoveNestedField(unStr.Object, "spec", "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(unStr.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(unStr.Object, "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")
	unstructured.RemoveNestedField(unStr.Object, "metadata", "ownerReferences")
	u.Object = unStr.Object
}

// FromUnstructured is a wrapper to convert unstructured to concrete type in runtime.Object passed.
// It used `runtime.DefaultUnstructuredConverter` and works on internal `UnstructuredResource` of unstructured type
func (u *UnstructResource) FromUnstructured(obj runtime.Object) error {
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return err
	}
	return nil
}

// ToUnstructured is a wrapper to convert the concrete type of object to unstructured object in `UnstructuredResource`
// It used `runtime.DefaultUnstructuredConverter` and works on internal `UnstructuredResource` of unstructured type
func (u *UnstructResource) ToUnstructured(obj interface{}) error {
	unstructObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	u.Object = unstructObj
	return nil
}

// PerformPreBackupProcess performs the processing just like dry run on the Unstructured resource
// to check for the possible warnings based on the kind and fields of that resource
//
// It checks for the warnings based on the resources and its fields and
// also with the statuses of the pods (if any) for that resource.
//
// ChecksForWarnings is the function called to check for resource fields based warnings
// and ChecksForStatus is called to check the status based warnings
// The resource is cleaned at the end for only clustered and namespace scoped fields
func (u *UnstructResource) PerformPreBackupProcess(ctx context.Context, cl client.Client, ns string,
	warnings helpers.WarningMap) error {

	u.ChecksForWarnings(warnings)

	// Check status
	if err := u.ChecksForStatus(ctx, cl, ns, warnings); err != nil {
		return err
	}

	// Cleanup the object
	u.CleanNamespacedScopeMutations()
	return nil
}

// Transform transforms the given metadata as per json patches
func (u *UnstructResource) Transform(meta string,
	patches []crd.Patch) (transformedMeta string, err error) {
	marshaledPatches, mErr := json.Marshal(patches)
	if mErr != nil {
		logger.Error("Error while marshaling json patches: ", mErr)
		return meta, mErr
	}
	decodedPatch, dErr := jsonpatch.DecodePatch(marshaledPatches)
	if dErr != nil {
		logger.Error("Error while decoding patches: ", dErr)
		return meta, dErr
	}
	modifiedMeta, modifyErr := decodedPatch.Apply([]byte(meta))
	if modifyErr != nil {
		logger.Error(modifyErr, "Error while applying patches: ", modifyErr)
		return meta, modifyErr
	}

	return string(modifiedMeta), nil
}

// TransformAndDryRun transforms the resource as per given patches and dry runs it
// to check if transformation happened properly
func (u *UnstructResource) TransformAndDryRun(a *kube.Accessor, ns, meta string,
	patches []crd.Patch) (transformedMeta string, err error) {
	var unstructObj unstructured.Unstructured
	modifiedMeta, tErr := u.Transform(meta, patches)
	if tErr != nil {
		return meta, tErr
	}
	uErr := unstructObj.UnmarshalJSON([]byte(modifiedMeta))
	if uErr != nil {
		logger.Error("Error while unmarshalling", uErr)
		return meta, uErr
	}
	// assign the transformed object to unstructured obj for dry run
	u.Object = unstructObj.Object
	depErr := u.IsResourceDeployable(a, ns)
	if depErr != nil {
		logger.Error("Error while performing dry run", depErr)
		return modifiedMeta, depErr
	}

	return modifiedMeta, nil
}

// removeDefaultSaTokenVolumes is the helper which removes the sa token volumes from the containers and volumes list
//
// It takes the volumes and containers as the input list.
// For each container it checks for the `VolumeMounts` with the mountPath as internal.DefaultServiceAccountVolumeMountPath
// and removes them from the container and same volume is then removed from the list of volumes passed
func removeDefaultSaTokenVolumes(containers, volumes []interface{}) (con, vol []interface{}) {

	for i := range containers {
		var saVolName string
		c, ok := containers[i].(map[string]interface{})
		if !ok {
			continue
		}

		vms, ok := c["volumeMounts"].([]interface{})
		if !ok {
			if vms == nil {
				delete(c, "volumeMounts")
			}
			continue
		}

		var volMounts []interface{}
		for j := range vms {
			vm := vms[j].(map[string]interface{})
			if vm["mountPath"] == internal.DefaultServiceAccountVolumeMountPath {
				saVolName = vm["name"].(string)
				break
			}
			volMounts = append(volMounts, vms[j])
		}

		// removed the volumeMount of default token
		c["volumeMounts"] = volMounts
		containers[i] = c

		if saVolName == "" {
			continue
		}

		var vols []interface{}
		for i := range volumes {
			v, ok := volumes[i].(map[string]interface{})
			if !ok || v["name"].(string) == saVolName {
				continue
			}
			vols = append(vols, v)
		}
		volumes = vols
	}

	return containers, volumes
}

// checkForNodePort returns the warnings string for checking the `NodePort` on the passed resource, if it is `Service` kind
func checkForNodePort(item unstructured.Unstructured, warnings helpers.WarningMap) {
	if svcType, isPresent, err := unstructured.NestedFieldCopy(item.Object, "spec", "type"); isPresent && err == nil {
		if serviceType := svcType.(string); serviceType == string(corev1.ServiceTypeNodePort) {
			helpers.CheckAndAddWarning(item.GroupVersionKind().String(), item.GetName(), internal.NodePortWarning,
				item.GetLabels(), warnings)
		}
	}
}

// checkForHostNetwork returns the warnings string for checking the `HostNetwork` on the passed resource
func checkForHostNetwork(item unstructured.Unstructured, warnings helpers.WarningMap) {
	var (
		isPresent bool
		err       error
		hnFlag    interface{}
		reqPath   []string
	)
	if item.GetKind() == internal.PodKind {
		reqPath = append(reqPath, "spec", "hostNetwork")
	} else {
		reqPath = append(reqPath, "spec", "template", "spec", "hostNetwork")
	}

	if hnFlag, isPresent, err = unstructured.NestedFieldCopy(item.Object, reqPath...); isPresent && err == nil {
		if hostNetworkFlag := hnFlag.(bool); hostNetworkFlag {
			helpers.CheckAndAddWarning(item.GroupVersionKind().String(), item.GetName(), internal.HostNetworkWarning,
				item.GetLabels(), warnings)
			return
		}
	}
}

// checkForHostPort returns the warnings string for checking the `HostPort` on the passed resource
// It finds the port from the containers and checks if the type is HostPort and returns warnings
func checkForHostPort(item unstructured.Unstructured, warnings helpers.WarningMap) {
	var (
		isPresent  bool
		containers interface{}
		reqPath    []string
	)
	if item.GetKind() == internal.PodKind {
		reqPath = append(reqPath, "spec", "containers")
	} else {
		reqPath = append(reqPath, "spec", "template", "spec", "containers")
	}

	containers, isPresent, _ = unstructured.NestedFieldCopy(item.Object, reqPath...)
	if !isPresent {
		return
	}

	containerList := containers.([]interface{})
	for containerIndex := range containerList {
		container := containerList[containerIndex]

		ports, arePortsPresent, portErr := unstructured.NestedFieldCopy(container.(map[string]interface{}), "ports")
		if arePortsPresent && portErr == nil {
			portList := ports.([]interface{})
			for portIndex := range portList {
				port := portList[portIndex]
				if _, isHpPresent, _ := unstructured.NestedFieldCopy(port.(map[string]interface{}), "hostPort"); isHpPresent {
					helpers.CheckAndAddWarning(item.GroupVersionKind().String(), item.GetName(), internal.HostPortWarning,
						item.GetLabels(), warnings)
					return
				}
			}
		}
	}
}

// checkForNodeSelector returns the warnings string for checking the `Node Selector` on the passed resource
func checkForNodeSelector(item unstructured.Unstructured, warnings helpers.WarningMap) {
	var (
		isPresent bool
		err       error
		nsReqPath []string
	)
	if item.GetKind() == internal.PodKind {
		nsReqPath = append(nsReqPath, "spec", "nodeSelector")
	} else {
		nsReqPath = append(nsReqPath, "spec", "template", "spec", "nodeSelector")
	}
	// Check for node selector
	if _, isPresent, err = unstructured.NestedFieldCopy(item.Object, nsReqPath...); isPresent && err == nil {
		helpers.CheckAndAddWarning(item.GroupVersionKind().String(), item.GetName(), internal.NodeSelectorWarning,
			item.GetLabels(), warnings)
		return
	}
}

// checkForNodeAffinity returns the warnings string for checking the `Node Affinity` on the passed resource
func checkForNodeAffinity(item unstructured.Unstructured, warnings helpers.WarningMap) {
	var (
		isPresent    bool
		err          error
		nsAffReqPath []string
	)
	if item.GetKind() == internal.PodKind {
		nsAffReqPath = append(nsAffReqPath, "spec", "affinity", "nodeAffinity")
	} else {
		nsAffReqPath = append(nsAffReqPath, "spec", "template", "spec", "affinity", "nodeAffinity")
	}
	// Check node affinity
	if _, isPresent, err = unstructured.NestedFieldCopy(item.Object, nsAffReqPath...); isPresent && err == nil {
		helpers.CheckAndAddWarning(item.GroupVersionKind().String(), item.GetName(), internal.NodeAffinityWarning,
			item.GetLabels(), warnings)
		return
	}
}

// verifyPodStatus is the helper which returns false if any of the pod conditions is not True `(ConditionTrue)`
func verifyPodStatus(pod *corev1.Pod) bool {
	for condIndex := range pod.Status.Conditions {
		if pod.Status.Conditions[condIndex].Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}
