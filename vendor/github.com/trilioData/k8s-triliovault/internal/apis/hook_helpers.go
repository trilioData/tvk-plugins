package apis

import (
	"context"
	"errors"
	"fmt"

	apiv1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/decorator"
	internalHelper "github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/kube"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/yaml"
)

var (
	expectedPodResources = map[string]runtime.Object{
		internal.PodKind:                   &corev1.Pod{},
		internal.DaemonSetKind:             &appsv1.DaemonSet{},
		internal.DeploymentKind:            &appsv1.Deployment{},
		internal.StatefulSetKind:           &appsv1.StatefulSet{},
		internal.ReplicaSetKind:            &appsv1.ReplicaSet{},
		internal.ReplicationControllerKind: &corev1.ReplicationController{},
		internal.JobKind:                   &batchv1.Job{},
		internal.CronJobKind:               &batchv1beta1.CronJob{},
	}

	err             error
	isOwnerRequired bool
)

func identifyAndGetTargetingHookPods(acc *kube.Accessor, compMeta []ComponentMetadata,
	ns, containerRegex string, podSelector apiv1.PodSelector) ([]runtime.Object, error) {

	var identifiedResources []runtime.Object
	var tempObjs []runtime.Object
	for i := range compMeta {
		res := compMeta[i]
		log.Debugf("Checking if identification of hook resources is needed for resource gvk: %+v", res.GroupVersionKind.Kind)

		tempObjs = []runtime.Object{}
		gvk := res.GroupVersionKind

		switch res.GroupVersionKind.Kind {
		case internal.PodKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.PodKind], ns, containerRegex, podSelector)
		case internal.DeploymentKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.DeploymentKind], ns, containerRegex, podSelector)
		case internal.DaemonSetKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.DaemonSetKind], ns, containerRegex, podSelector)
		case internal.StatefulSetKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.StatefulSetKind], ns, containerRegex, podSelector)
		case internal.ReplicaSetKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.ReplicaSetKind], ns, containerRegex, podSelector)
		case internal.ReplicationControllerKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.ReplicationControllerKind], ns, containerRegex, podSelector)
		case internal.JobKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.JobKind], ns, containerRegex, podSelector)
		case internal.CronJobKind:
			log.Infof("identifying hook resources for resource gvk: %+v", res.GroupVersionKind.Kind)
			tempObjs, err = filterObjectsFromMeta(acc, apiv1.Resource{
				GroupVersionKind: gvk,
				Objects:          res.Metadata,
			}, expectedPodResources[internal.CronJobKind], ns, containerRegex, podSelector)
		}

		if err != nil {
			return []runtime.Object{}, err
		}
		identifiedResources = append(identifiedResources, tempObjs...)
	}
	return identifiedResources, nil
}

func filterObjectsFromMeta(acc *kube.Accessor, resource apiv1.Resource,
	coreObj runtime.Object, ns, containerRegex string, podSelector apiv1.PodSelector) ([]runtime.Object, error) {
	metadata, gvk := resource.Objects, resource.GroupVersionKind
	cl := acc.GetKubeClient()
	ctx := context.Background()
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind(gvk))
	var identifiedResources []runtime.Object
	tempObj := coreObj.DeepCopyObject()

	for i := range metadata {
		objMeta := metadata[i]
		if uMarshalErr := yaml.Unmarshal([]byte(objMeta), &obj); uMarshalErr != nil {
			return []runtime.Object{}, fmt.Errorf("failed to unmarshal object for gvk: %+v, "+
				"err: %v", gvk, uMarshalErr)
		}

		unRes := decorator.UnstructResource(obj)

		err = unRes.FromUnstructured(tempObj)
		if err != nil {
			return []runtime.Object{}, fmt.Errorf("failed to get runtime object for gvk: %+v, "+
				"err: %v", gvk, err)
		}

		var podList corev1.PodList
		if tempObj.GetObjectKind().GroupVersionKind().Kind == internal.PodKind {
			pod := *tempObj.(*corev1.Pod)
			podList.Items = append(podList.Items, pod)
		} else {

			switch resType := tempObj.(type) {
			case *batchv1.Job:
				if resType.Spec.Selector == nil {

					tempObj, err = acc.GetJob(resType.GetNamespace(), resType.GetName())
					if err != nil {
						log.Errorf("failed to get job %s/%s", resType.GetName(), resType.GetNamespace())
						return []runtime.Object{}, fmt.Errorf("failed to get job %s/%s",
							resType.GetName(), resType.GetNamespace())
					}
				}
			case *corev1.ReplicationController:
				if resType.Spec.Selector == nil {
					tempObj, err = acc.GetReplicationController(resType.Namespace, resType.Name)
					if err != nil {
						log.Errorf("failed to get replication controller %s/%s", resType.GetName(), resType.GetNamespace())
						return []runtime.Object{}, fmt.Errorf("failed to get replication controller %s/%s",
							resType.GetName(), resType.GetNamespace())
					}
				}
			}

			var pods *corev1.PodList
			pods, err = kube.GetPodsOfAppKind(ctx, cl, ns, tempObj)
			if err != nil || pods == nil {
				log.Errorf("failed to get pods for resource %+v", resource)
				return []runtime.Object{}, fmt.Errorf("failed to get pods for resource %+v",
					resource)
			}

			// Filter pods based on owner
			for pIndex := range pods.Items {
				var podUnstruct decorator.UnstructResource
				pod := pods.Items[pIndex]
				tErr := podUnstruct.ToUnstructured(&pod)
				if tErr != nil {
					log.Errorf("failed to convert to unstruct while getting owner of the pod %+v", tErr)
					return []runtime.Object{}, tErr
				}
				parent, parentErr := internalHelper.FindParentResource(acc, unstructured.Unstructured(podUnstruct), ns)
				if parentErr != nil {
					log.Errorf("failed to get owner of the pod %+v", parentErr)
					return []runtime.Object{}, parentErr
				}
				if parent.GetAPIVersion() == obj.GetAPIVersion() && parent.GetName() == obj.GetName() {
					podList.Items = append(podList.Items, pod)
				}
			}
		}

		// checking if owner is required or not by running podSelector on their pods.
		isOwnerRequired, err = checkIfOwnerRequired(acc, &podList, podSelector, containerRegex)
		if err != nil {
			return []runtime.Object{}, err
		}
		if isOwnerRequired {
			err = unRes.FromUnstructured(coreObj)
			if err != nil {
				return []runtime.Object{}, fmt.Errorf("failed to convert resource %+v into runtime object", resource)
			}
			identifiedResources = append(identifiedResources, coreObj.DeepCopyObject())
		}
	}
	return identifiedResources, nil
}

func checkIfOwnerRequired(acc *kube.Accessor, podList *corev1.PodList,
	podSelector apiv1.PodSelector, containerRegex string) (bool, error) {
	if len(podSelector.Labels) == 0 && podSelector.Regex == "" {
		return false, fmt.Errorf("atlease one of podSelector.Labels & " +
			"podSelector.Regex is required")
	}

	if len(podList.Items) > 0 && acc.MatchPodLabelSelector(podList.Items[0].GetLabels(),
		podSelector.Labels) {
		for i := range podList.Items {
			pod := podList.Items[i]
			if internal.MatchRegex(pod.GetName(), podSelector.Regex) {
				_, err = CheckAndGetContainersIfRegexMatches(&pod, containerRegex)
				if err != nil {
					return false, err
				}
				return true, nil
			}
		}
	}

	return false, nil

}

func GetHookByObjRef(acc *kube.Accessor, ref *corev1.ObjectReference) (*apiv1.Hook, error) {
	if ref == nil {
		err = errors.New("nil object reference of Hook resource in BackupPlan CR")
		log.Error(err, "")
		return nil, err
	}

	var hook *apiv1.Hook
	hook, err = acc.GetHook(ref.Name, ref.Namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error getting the instance of Hook: %s", ref.Name))
		return nil, err
	}
	return hook, nil
}

func convertToHookStatus(acc *kube.Accessor, resources []runtime.Object, hookInfo apiv1.HookInfo) (apiv1.HookPriority, error) {
	var hook *apiv1.Hook
	ref := hookInfo.Hook
	st := apiv1.HookPriority{}
	hook, err = GetHookByObjRef(acc, ref)
	if err != nil {
		log.Errorf("failed to get hook")
		return apiv1.HookPriority{}, fmt.Errorf("failed to get hook object")
	}

	st.Hook = ref
	st.PreHookConf = &apiv1.HookConfiguration{
		MaxRetryCount:  hook.Spec.PreHook.MaxRetryCount,
		TimeoutSeconds: hook.Spec.PreHook.TimeoutSeconds,
		IgnoreFailure:  hook.Spec.PreHook.IgnoreFailure,
	}
	st.PostHookConf = &apiv1.HookConfiguration{
		MaxRetryCount:  hook.Spec.PostHook.MaxRetryCount,
		TimeoutSeconds: hook.Spec.PostHook.TimeoutSeconds,
		IgnoreFailure:  hook.Spec.PostHook.IgnoreFailure,
	}
	st.HookTarget, err = convertAndGetHookTargets(resources, hookInfo.ContainerRegex)
	if err != nil {
		return apiv1.HookPriority{}, err
	}
	return st, nil
}

func convertAndGetHookTargets(resources []runtime.Object, containerRegex string) ([]apiv1.HookTarget, error) {
	var hookTarget []apiv1.HookTarget
	var matchingContainers []apiv1.ContainerHookStatus

	for i := range resources {
		var target apiv1.HookTarget
		res := resources[i]
		gvk := res.GetObjectKind().GroupVersionKind()
		var resAccessor metav1.Object
		resAccessor, err = meta.Accessor(res)
		if err != nil {
			log.Errorf("Failed to get Accessor")
			return nil, fmt.Errorf("failed to get Accessor for gvk %+v", gvk)
		}

		if gvk.Kind == internal.PodKind {
			target.ContainerRegex = containerRegex
			matchingContainers, err = CheckAndGetContainersIfRegexMatches(res.(*corev1.Pod), containerRegex)
			if err != nil {
				return nil, err
			}
			if len(matchingContainers) > 0 {
				target.PodHookStatus = append(target.PodHookStatus, apiv1.PodHookStatus{
					PodName:             resAccessor.GetName(),
					ContainerHookStatus: matchingContainers,
				})
			}

		} else {

			target.Owner = &apiv1.Owner{
				GroupVersionKind: apiv1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
				Name: resAccessor.GetName(),
			}
			target.ContainerRegex = containerRegex
		}

		hookTarget = append(hookTarget, target)
	}

	return hookTarget, nil
}

// CheckAndGetContainersIfRegexMatches returns if specified pod contains containers with matching containerRegex.
func CheckAndGetContainersIfRegexMatches(pod *corev1.Pod, containerRegex string) ([]apiv1.ContainerHookStatus, error) {
	var containerHookStatus []apiv1.ContainerHookStatus
	containers := pod.Spec.Containers
	for i := range containers {
		cont := containers[i]
		if internal.MatchRegex(cont.Name, containerRegex) {
			containerHookStatus = append(containerHookStatus, apiv1.ContainerHookStatus{
				ContainerName: cont.Name,
			})
		}
	}
	if len(containerHookStatus) == 0 {
		return nil, fmt.Errorf("no matching container found for containerRegex=%s in pod %s", containerRegex, pod.Name)
	}
	return containerHookStatus, nil
}
