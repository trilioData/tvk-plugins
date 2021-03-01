package kube

import (
	"context"
	"fmt"
	"github.com/trilioData/tvk-plugins/tests/common"
	"github.com/trilioData/tvk-plugins/tests/common/retry"
	"os"
	"os/signal"
	"strings"
	"time"

	//"github.com/trilioData/k8s-triliovault/internal"
	//"github.com/trilioData/k8s-triliovault/internal/utils/retry"
	"k8s.io/apimachinery/pkg/api/meta"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	v1 "k8s.io/api/rbac/v1"
	storageV1 "k8s.io/api/storage/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client "k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	ctrlRuntime "sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *Accessor) CreateStorageClass(storageClass *storageV1.StorageClass) error {
	_, err := a.set.StorageV1().StorageClasses().Create(a.context, storageClass, metav1.CreateOptions{})
	return err
}

// CreatePersistentVolumeClaim creates the given PersistentVolumeClaim in the passed namespace.
func (a *Accessor) CreatePersistentVolumeClaim(namespace string, pvc *corev1.PersistentVolumeClaim) error {
	_, err := a.set.CoreV1().PersistentVolumeClaims(namespace).Create(a.context, pvc, metav1.CreateOptions{})
	return err
}

// DeletePersistentVolumeClaim deletes the given PersistentVolumeClaim in the passed namespace.
func (a *Accessor) DeletePersistentVolumeClaim(namespace, pvc string) error {
	err := a.set.CoreV1().PersistentVolumeClaims(namespace).Delete(a.context, pvc, metav1.DeleteOptions{})
	return err
}

// GetPersistentVolumeClaims returns pvc in the given namespace, based on the selectors. If no selectors are given, then
// all PVCs are returned.
func (a *Accessor) GetPersistentVolumeClaims(namespace string, selectors ...string) ([]corev1.PersistentVolumeClaim, error) {
	s := strings.Join(selectors, ",")
	list, err := a.set.CoreV1().PersistentVolumeClaims(namespace).List(a.context, metav1.ListOptions{LabelSelector: s})

	if err != nil {
		return []corev1.PersistentVolumeClaim{}, err
	}

	return list.Items, nil
}

// GetPersistentVolumeClaim returns pvc with the given name & namespace.
func (a *Accessor) GetPersistentVolumeClaim(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := a.set.CoreV1().PersistentVolumeClaims(namespace).Get(a.context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

// Check if given PersistentVolumeClaim present in the passed namespace
func (a *Accessor) CheckIfPVCPresent(namespace, pvc string) bool {
	// If given PVC is present in the provided namespace then return true
	log.Infof("Checking if pvc %s exists in namespace %s", pvc, namespace)
	var pvcObj corev1.PersistentVolumeClaim
	if err := a.client.Get(a.context, types.NamespacedName{
		Namespace: namespace,
		Name:      pvc,
	}, &pvcObj); err != nil {
		log.Infof("pvc %s does not exists in namespace %s", pvc, namespace)
		return false
	}
	log.Infof("pvc %s present in the namespace %s", pvc, namespace)
	return true
}

// DeletePersistentVolume deletes the given PersistentVolume in the passed namespace.
func (a *Accessor) DeletePersistentVolume(pv string) error {
	err := a.set.CoreV1().PersistentVolumes().Delete(a.context, pv, metav1.DeleteOptions{})
	return err
}

// GetPods returns pods in the given namespace, based on the selectors. If no selectors are given, then
// all pods are returned.
func (a *Accessor) GetPods(namespace string, selectors ...string) ([]corev1.Pod, error) {
	s := strings.Join(selectors, ",")
	list, err := a.set.CoreV1().Pods(namespace).List(a.context, metav1.ListOptions{LabelSelector: s})

	if err != nil {
		return []corev1.Pod{}, err
	}

	return list.Items, nil
}

// GetCronJobs returns cron jobs in the given namespace, based on the selectors. If no selectors are given, then
// all cron jobs are returned.
func (a *Accessor) GetCronJobs(namespace string, selectors ...string) ([]batchv1beta1.CronJob, error) {
	s := strings.Join(selectors, ",")
	list, err := a.set.BatchV1beta1().CronJobs(namespace).List(a.context, metav1.ListOptions{LabelSelector: s})

	if err != nil {
		return []batchv1beta1.CronJob{}, err
	}

	return list.Items, nil
}

func (a *Accessor) PatchCronJob(namespace, name string, data []byte, subresources ...string) (*batchv1beta1.CronJob, error) {
	return a.set.BatchV1beta1().CronJobs(namespace).Patch(a.context, name, types.JSONPatchType, data, metav1.PatchOptions{})
}

// GetEvents returns events in the given namespace, based on the involvedObject.
func (a *Accessor) GetEvents(namespace, involvedObject string) ([]corev1.Event, error) {
	s := "involvedObject.name=" + involvedObject
	list, err := a.set.CoreV1().Events(namespace).List(a.context, metav1.ListOptions{FieldSelector: s})

	if err != nil {
		return []corev1.Event{}, err
	}

	return list.Items, nil
}

// CreatePod creates the given pod spec in the passed namespace.
func (a *Accessor) CreatePod(namespace string, pod *corev1.Pod) error {
	_, err := a.set.CoreV1().Pods(namespace).Create(a.context, pod, metav1.CreateOptions{})
	return err
}

// GetPod returns the pod with the given namespace and name.
func (a *Accessor) GetPod(namespace, name string) (corev1.Pod, error) {
	pod, err := a.set.CoreV1().
		Pods(namespace).Get(a.context, name, metav1.GetOptions{})

	if err != nil {
		return corev1.Pod{}, err
	}
	return *pod, nil
}

// GetCronJob returns the cronjob with the given namespace and name
func (a *Accessor) GetCronJob(namespace, name string) (batchv1beta1.CronJob, error) {
	cron, err := a.set.BatchV1beta1().CronJobs(namespace).Get(a.context, name, metav1.GetOptions{})

	if err != nil {
		return batchv1beta1.CronJob{}, err
	}
	return *cron, nil
}

// GetJob returns the job with the given namespace and name
func (a *Accessor) GetJob(namespace, name string) (*batchv1.Job, error) {
	job, err := a.set.BatchV1().Jobs(namespace).Get(a.context, name, metav1.GetOptions{})

	if err != nil {
		return &batchv1.Job{}, err
	}
	return job, nil
}

// GetReplicationController return replicaton controller with the given namespace and name.
func (a *Accessor) GetReplicationController(namespace, name string) (*corev1.ReplicationController, error) {
	return a.set.CoreV1().ReplicationControllers(namespace).Get(a.context, name, metav1.GetOptions{})
}

// GetJobs returns k8s jobs in the given namespace, based on the selectors. If no selectors are given, then
// all the jobs are returned.
func (a *Accessor) GetJobs(namespace string, selectors ...string) ([]batchv1.Job, error) {
	s := strings.Join(selectors, ",")
	list, err := a.set.BatchV1().Jobs(namespace).List(a.context, metav1.ListOptions{LabelSelector: s})

	if err != nil {
		return []batchv1.Job{}, err
	}

	return list.Items, nil
}

// DeleteCronJob deletes the cronjob with the given namespace and name
func (a *Accessor) DeleteCronJob(namespace, name string) error {
	return a.set.BatchV1beta1().CronJobs(namespace).Delete(a.context, name, metav1.DeleteOptions{})
}

// DeleteCronJobs deletes the all cronjobs with the given selection
func (a *Accessor) DeleteCronJobs(namespace string, options *metav1.DeleteOptions, selectors ...string) error {
	s := strings.Join(selectors, ",")
	return a.set.BatchV1beta1().CronJobs(namespace).DeleteCollection(a.context, *options,
		metav1.ListOptions{LabelSelector: s})
}

// DeleteJobs deletes the all jobs with the given selection
func (a *Accessor) DeleteJobs(namespace string, options *metav1.DeleteOptions, selectors ...string) error {
	s := strings.Join(selectors, ",")
	return a.set.BatchV1().Jobs(namespace).DeleteCollection(a.context, *options,
		metav1.ListOptions{LabelSelector: s})
}

// DeleteJob deletes the job with the given namespace and name
func (a *Accessor) DeleteJob(namespace, name string) error {
	return a.set.BatchV1().Jobs(namespace).Delete(a.context, name, metav1.DeleteOptions{})
}

// DeletePod deletes the given pod.
func (a *Accessor) DeletePod(namespace, name string) error {
	gracePeriod := int64(0)
	return a.set.CoreV1().Pods(namespace).Delete(a.context, name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod})
}

// FindPodBySelectors returns the first matching pod, given a namespace and a set of selectors.
func (a *Accessor) FindPodBySelectors(namespace string, selectors ...string) (corev1.Pod, error) {
	list, err := a.GetPods(namespace, selectors...)
	if err != nil {
		return corev1.Pod{}, err
	}

	if len(list) == 0 {
		return corev1.Pod{}, fmt.Errorf("no matching pod found for selectors: %v", selectors)
	}

	if len(list) > 1 {
		log.Warnf("More than one pod found matching selectors: %v", selectors)
	}

	return list[0], nil
}

// PodFetchFunc fetches pods from the Accessor.
type PodFetchFunc func() ([]corev1.Pod, error)

// SinglePodFetchFunc fetches single pod from the Accessor.
type SinglePodFetchFunc func() (corev1.Pod, error)

// NewPodFetch creates a new PodFetchFunction that fetches all pods matching the namespace and label selectors.
func (a *Accessor) NewPodFetch(namespace string, selectors ...string) PodFetchFunc {
	return func() ([]corev1.Pod, error) {
		return a.GetPods(namespace, selectors...)
	}
}

// GetSinglePodFetch creates a new SinglePodFetchFunc that fetches single pod matching the namespace & name
func (a *Accessor) GetSinglePodFetch(namespace, name string) SinglePodFetchFunc {
	return func() (corev1.Pod, error) {
		return a.GetPod(namespace, name)
	}
}

// NewSinglePodFetch creates a new PodFetchFunction that fetches a single pod matching the given label selectors.
func (a *Accessor) NewSinglePodFetch(namespace string, selectors ...string) PodFetchFunc {
	return func() ([]corev1.Pod, error) {
		pod, err := a.FindPodBySelectors(namespace, selectors...)
		if err != nil {
			return nil, err
		}
		return []corev1.Pod{pod}, nil
	}
}

// WaitUntilPodsAreReady waits until the pod with the name/namespace is in ready state.
func (a *Accessor) WaitUntilPodsAreReady(fetchFunc PodFetchFunc, opts ...retry.Option) ([]corev1.Pod, error) {
	var pods []corev1.Pod
	_, err := retry.Do(func() (interface{}, bool, error) {
		log.Infof("Checking pods ready...")

		fetched, err := a.CheckPodsAreReady(fetchFunc)
		if err != nil {
			return nil, false, err
		}
		pods = fetched
		return nil, true, nil
	}, newRetryOptions(opts...)...)

	return pods, err
}

// WaitUnitlPodCompletes waits until the pod with the name/namespace is in completed state.
func (a *Accessor) WaitUntilPodCompletes(fetchFunc SinglePodFetchFunc, opts ...retry.Option) (corev1.Pod, error) {
	var pod corev1.Pod
	_, err := retry.Do(func() (interface{}, bool, error) {
		log.Infof("Checking pods Completed...")
		var completed bool
		fetched, err := a.CheckPodsAreCompleted(fetchFunc)
		if fetched.Status.Phase == corev1.PodSucceeded ||
			fetched.Status.Phase == corev1.PodFailed ||
			fetched.Status.Phase == corev1.PodUnknown {
			completed = true
		}
		if err != nil {
			return nil, completed, err
		}
		pod = fetched
		return nil, completed, nil
	}, opts...)
	return pod, err
}

// CheckPodsAreReady checks wehther the pods that are selected by the given function is in ready state or not.
func (a *Accessor) CheckPodsAreReady(fetchFunc PodFetchFunc) ([]corev1.Pod, error) {
	log.Infof("Checking pods ready...")

	fetched, err := fetchFunc()
	if err != nil {
		log.Infof("Failed retrieving pods: %v", err)
		return nil, err
	}

	for i := range fetched {
		msg := "Ready"
		p := &fetched[i] // pinning for scopelint
		if e := CheckPodReady(p); e != nil {
			msg = e.Error()
			err = multierror.Append(err, fmt.Errorf("%s/%s: %s", p.Namespace, p.Name, msg))
		}
		log.Infof("  [%2d] %45s %15s (%v)", i, p.Name, p.Status.Phase, msg)
	}

	if err != nil {
		return nil, err
	}

	return fetched, nil
}

// CheckPodsAreCompleted checks whether the pods that are selected by the given function is in completed state.
func (a *Accessor) CheckPodsAreCompleted(fetchFunc SinglePodFetchFunc) (corev1.Pod, error) {
	fetched, err := fetchFunc()
	if err != nil {
		log.Infof("Failed retrieving pod:%v", err)
		return fetched, err
	}
	pod := &fetched
	log.Infof("checking %s pod completed. Phase:%s, Labels:%s", pod.Name, pod.Status.Phase, pod.Labels)

	msg := "container terminated successfully."
	if e := CheckPodCompleted(pod); e != nil {
		msg = e.Error()
		err = multierror.Append(e, fmt.Errorf("%s/%s: %s", pod.Namespace, pod.Name, msg))
		return fetched, err
	}
	log.Infof(" %45s %15s (%v)", pod.Name, pod.Status.Phase, msg)
	return fetched, nil
}

// WaitUntilPodsAreDeleted waits until the pod with the name/namespace no longer exist.
func (a *Accessor) WaitUntilPodsAreDeleted(fetchFunc PodFetchFunc, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		pods, err := fetchFunc()
		if err != nil {
			log.Infof("Failed retrieving pods: %v", err)
			return nil, false, err
		}

		if len(pods) == 0 {
			// All pods have been deleted.
			return nil, true, nil
		}

		return nil, false, fmt.Errorf("failed waiting to delete pod %s/%s", pods[0].Namespace, pods[0].Name)
	}, newRetryOptions(opts...)...)

	return err
}

// WaitUntilPodDeletes waits until the pod with the name/namespace no longer exist.
func (a *Accessor) WaitUntilPodDeletes(fetchFunc SinglePodFetchFunc, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		pod, err := fetchFunc()
		if err != nil {
			if errors.IsNotFound(err) {
				log.Infof("%s Pod No longer running", pod.Name)
				return nil, true, nil
			}
			log.Infof("Failed retrieving pods: %v", err)
			return nil, false, err
		}

		return nil, false, fmt.Errorf("failed waiting to delete pod %s/%s", pod.Namespace, pod.Name)
	}, newRetryOptions(opts...)...)

	return err
}

// CreateDeployment creates the given pod spec in the passed namespace.
func (a *Accessor) CreateDeployment(namespace string, deployment *appsv1.Deployment) error {
	_, err := a.set.AppsV1().Deployments(namespace).Create(a.context, deployment, metav1.CreateOptions{})
	return err
}

// GetDeployment returns the deployment with the given name in the passed namespace.
func (a *Accessor) GetDeployment(namespace, deplName string) (*appsv1.Deployment, error) {
	return a.set.AppsV1().Deployments(namespace).Get(a.context, deplName, metav1.GetOptions{})
}

// GetStatefulSet returns the StatefulSet with the given name in the passed namespace.
func (a *Accessor) GetStatefulSet(namespace, stsName string) (*appsv1.StatefulSet, error) {
	return a.set.AppsV1().StatefulSets(namespace).Get(a.context, stsName, metav1.GetOptions{})
}

// GetDeployments returns deployments in the given namespace, based on the selectors. If no selectors are given, then
// all deployments are returned.
func (a *Accessor) GetDeployments(namespace string, selectors ...string) ([]appsv1.Deployment, error) {
	s := strings.Join(selectors, ",")
	list, err := a.set.AppsV1().Deployments(namespace).List(a.context, metav1.ListOptions{LabelSelector: s})

	if err != nil {
		return []appsv1.Deployment{}, err
	}

	return list.Items, nil
}

// UpdateSecret updates the secret in the given namespace
func (a *Accessor) UpdateSecret(namespace string, secret *corev1.Secret) error {
	_, err := a.set.CoreV1().Secrets(namespace).Update(a.context, secret, metav1.UpdateOptions{})
	return err
}

// UpdateServiceAccount updates the Service Account in the given namespace
func (a *Accessor) UpdateServiceAccount(sa *corev1.ServiceAccount, namespace string) error {
	_, err := a.set.CoreV1().ServiceAccounts(namespace).Update(a.context, sa, metav1.UpdateOptions{})
	return err
}

// UpdateDeployment updates the deployment in the given namespace based on the given namespace
func (a *Accessor) UpdateDeployment(namespace string, deploy *appsv1.Deployment) error {
	_, err := a.set.AppsV1().Deployments(namespace).Update(a.context, deploy, metav1.UpdateOptions{})
	return err
}

// UpdateJob updates the Job in the given namespace.
func (a *Accessor) UpdateJob(namespace string, job *batchv1.Job) error {
	_, err := a.set.BatchV1().Jobs(namespace).Update(a.context, job, metav1.UpdateOptions{})
	return err
}

// UpdatePod updates the Pod in the given namespace.
func (a *Accessor) UpdatePod(namespace string, pod *corev1.Pod) error {
	_, err := a.set.CoreV1().Pods(namespace).Update(a.context, pod, metav1.UpdateOptions{})
	return err
}

func (a *Accessor) UpdateCronjob(namespace string, cj *batchv1beta1.CronJob) error {
	_, err := a.set.BatchV1beta1().CronJobs(namespace).Update(a.context, cj, metav1.UpdateOptions{})
	return err
}

// UpdateStatefulSet updates the StatefulSet in the given namespace.
func (a *Accessor) UpdateStatefulSet(namespace string, sts *appsv1.StatefulSet) error {
	_, err := a.set.AppsV1().StatefulSets(namespace).Update(a.context, sts, metav1.UpdateOptions{})
	return err
}

// DeleteDeployment delets the deployment with given name in the passed namespace.
func (a *Accessor) DeleteDeployment(namespace, name string, opts *metav1.DeleteOptions) error {
	if opts == nil {
		opts = &metav1.DeleteOptions{}
	}

	return a.set.AppsV1().Deployments(namespace).Delete(a.context, name, *opts)
}

// WaitUntilDeploymentIsDeleted waits until the deployment with the name/namespace no longer exist.
func (a *Accessor) WaitUntilDeploymentIsDeleted(ns, name string, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		_, err := a.set.AppsV1().Deployments(ns).Get(a.context, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, true, nil
			}
		}

		return nil, false, err
	}, newRetryOptions(opts...)...)

	return err
}

// CreateService creates the given spec in the passed namespace.
func (a *Accessor) CreateService(namespace string, service *corev1.Service) error {
	_, err := a.set.CoreV1().Services(namespace).Create(a.context, service, metav1.CreateOptions{})
	return err
}

// DeleteService deletes the given name in the passed namespace.
func (a *Accessor) DeleteService(namespace, name string) error {
	return a.set.CoreV1().Services(namespace).Delete(a.context, name, metav1.DeleteOptions{})
}

// CreateConfigMap creates the configMap with given spec in the passed namespace.
func (a *Accessor) CreateConfigMap(namespace string, cm *corev1.ConfigMap) error {
	_, err := a.set.CoreV1().ConfigMaps(namespace).Create(a.context, cm, metav1.CreateOptions{})
	return err
}

// GetConfigMap returns the configMap given name in the passed namespace.
func (a *Accessor) GetConfigMap(namespace, cmName string) (*corev1.ConfigMap, error) {
	return a.set.CoreV1().ConfigMaps(namespace).Get(a.context, cmName, metav1.GetOptions{})
}

// UpdateConfigMap updates given configMap.
func (a *Accessor) UpdateConfigMap(namespace string, confMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return a.set.CoreV1().ConfigMaps(namespace).Update(a.context, confMap, metav1.UpdateOptions{})
}

// GetConfigMap returns the configMap given name in the passed namespace.
func (a *Accessor) GetConfigMaps(namespace string, selectors ...string) (*corev1.ConfigMapList, error) {
	s := strings.Join(selectors, ",")
	return a.set.CoreV1().ConfigMaps(namespace).List(a.context, metav1.ListOptions{LabelSelector: s})
}

// DeleteConfigMap deletes the given condigMaps in the passed namespace.
func (a *Accessor) DeleteConfigMap(namespace, name string) error {
	return a.set.CoreV1().ConfigMaps(namespace).Delete(a.context, name, metav1.DeleteOptions{})
}

func deployRetryOptions(opts ...retry.Option) []retry.Option {
	if len(opts) != 0 {
		return opts
	}

	out := make([]retry.Option, 0, 2)
	out = append(out, retry.Timeout(time.Minute*10), retry.Delay(time.Second*5), retry.Count(90))
	return out
}

// WaitUntilDeploymentIsReady waits until the deployment with the name/namespace is in ready state.
func (a *Accessor) WaitUntilDeploymentIsReady(ns, name string, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		log.Infof("waiting for %s/%s deployment to be ready", name, ns)
		deployment, err := a.set.AppsV1().Deployments(ns).Get(a.context, name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, true, err
			}
		}

		ready := deployment.Status.ReadyReplicas == deployment.Status.UnavailableReplicas+deployment.Status.AvailableReplicas
		log.Infof("deployment %s/%s is ready-> %t", name, ns, ready)

		return nil, ready, nil
	}, deployRetryOptions(opts...)...)

	return err
}

// WaitUntilDaemonSetIsReady waits until the deployment with the name/namespace is in ready state.
func (a *Accessor) WaitUntilDaemonSetIsReady(ns, name string, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		daemonSet, err := a.set.AppsV1().DaemonSets(ns).Get(a.context, name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, true, err
			}
		}

		ready := daemonSet.Status.NumberReady == daemonSet.Status.DesiredNumberScheduled

		return nil, ready, nil
	}, newRetryOptions(opts...)...)

	return err
}

// WaitUntilServiceEndpointsAreReady will wait until the service with the given name/namespace is present, and have at least
// one usable endpoint.
func (a *Accessor) WaitUntilServiceEndpointsAreReady(ns, name string, opts ...retry.Option) (*corev1.Service,
	*corev1.Endpoints, error) {
	var service *corev1.Service
	var endpoints *corev1.Endpoints
	err := retry.UntilSuccess(func() error {
		s, err := a.GetService(ns, name)

		if err != nil {
			return err
		}
		eps, err := a.GetEndpoints(ns, name, metav1.GetOptions{})

		if err != nil {
			return err
		}

		if len(eps.Subsets) == 0 {
			return fmt.Errorf("%s/%v endpoint not ready: no subsets", ns, name)
		}

		for i := range eps.Subsets {
			subset := eps.Subsets[i]
			if len(subset.Addresses) > 0 && len(subset.NotReadyAddresses) == 0 {
				service = s
				endpoints = eps
				return nil
			}
		}
		return fmt.Errorf("%s/%v endpoint not ready: no ready addresses", ns, name)
	}, newRetryOptions(opts...)...)

	if err != nil {
		return nil, nil, err
	}

	return service, endpoints, nil
}

// DeleteValidatingWebhook deletes the validating webhook with the given name.
func (a *Accessor) DeleteValidatingWebhook(name string) error {
	return a.set.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(a.context, name, deleteOptionsForeground())
}

// WaitForValidatingWebhookDeletion waits for the validating webhook with the given name to be garbage collected by kubernetes.
func (a *Accessor) WaitForValidatingWebhookDeletion(name string, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		if a.ValidatingWebhookConfigurationExists(name) {
			return nil, false, fmt.Errorf("validating webhook not deleted: %s", name)
		}

		// It no longer exists ... success.
		return nil, true, nil
	}, newRetryOptions(opts...)...)

	return err
}

// GetCustomResourceDefinition gets the CRD named "name"
func (a *Accessor) GetCustomResourceDefinition(name string) (*extv1beta1.CustomResourceDefinition, error) {
	return a.extSet.ApiextensionsV1beta1().CustomResourceDefinitions().Get(a.context, name, metav1.GetOptions{})
}

// GetCustomResourceDefinitionList gets the CRDs
func (a *Accessor) GetCustomResourceDefinitionList() ([]extv1beta1.CustomResourceDefinition, error) {
	crd, err := a.extSet.ApiextensionsV1beta1().CustomResourceDefinitions().List(a.context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return crd.Items, nil
}

// DeleteCustomResourceDefinition deletes the CRD with the given name.
func (a *Accessor) DeleteCustomResourceDefinition(name string) error {
	return a.extSet.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(a.context, name, deleteOptionsForeground())
}

// UpdateCustomResourceDefinition updates the CRD with the given name.
func (a *Accessor) UpdateCustomResourceDefinition(crd *extv1beta1.CustomResourceDefinition) (
	*extv1beta1.CustomResourceDefinition, error) {
	return a.extSet.ApiextensionsV1beta1().CustomResourceDefinitions().Update(a.context, crd, metav1.UpdateOptions{})
}

// GetService returns the service entry with the given name/namespace.
func (a *Accessor) GetService(ns, name string) (*corev1.Service, error) {
	return a.set.CoreV1().Services(ns).Get(a.context, name, metav1.GetOptions{})
}

// GetSecret returns secret resource with the given namespace.
func (a *Accessor) GetSecret(ns, name string) (*corev1.Secret, error) {
	return a.set.CoreV1().Secrets(ns).Get(a.context, name, metav1.GetOptions{})
}

// GetSecrets returns secrets with the given namespace.
func (a *Accessor) GetSecrets(ns string, selectors ...string) (*corev1.SecretList, error) {
	s := strings.Join(selectors, ",")
	return a.set.CoreV1().Secrets(ns).List(a.context, metav1.ListOptions{LabelSelector: s})
}

// CreateSecret takes the representation of a secret and creates it in the given namespace.
// Returns an error if there is any.
func (a *Accessor) CreateSecret(namespace string, secret *corev1.Secret) (err error) {
	_, err = a.set.CoreV1().Secrets(namespace).Create(a.context, secret, metav1.CreateOptions{})
	return err
}

// CreateSecretFromFile takes the representation of a secret from file and creates it in the given namespace.
// Returns an error if there is any.
func (a *Accessor) CreateSecretFromFile(namespace, secretName, fileName string) (err error) {
	return a.ctl.createSecretFromFile(namespace, secretName, fileName)
}

// DeleteSecret deletes secret by name in namespace.
func (a *Accessor) DeleteSecret(namespace, name string) (err error) {
	var immediate int64
	err = a.set.CoreV1().Secrets(namespace).Delete(a.context, name, metav1.DeleteOptions{GracePeriodSeconds: &immediate})
	return err
}

// GetEndpoints returns the endpoints for the given service.
func (a *Accessor) GetEndpoints(ns, service string, options metav1.GetOptions) (*corev1.Endpoints, error) {
	return a.set.CoreV1().Endpoints(ns).Get(a.context, service, options)
}

// CreateNamespace with the given name. Also adds an "datamover-testing" annotation.
func (a *Accessor) CreateNamespace(ns string) error {
	log.Infof("Creating namespace sa: %s", ns)

	n := a.newNamespace(ns)
	_, err := a.set.CoreV1().Namespaces().Create(a.context, &n, metav1.CreateOptions{})
	return err
}

func (a *Accessor) SetNamespaceLabels(namespace string, label map[string]string) error {
	ns, err := a.GetNamespace(namespace)
	if err != nil {
		return err
	}
	ns.SetLabels(label)
	_, err = a.set.CoreV1().Namespaces().Update(a.context, ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (a *Accessor) newNamespace(ns string) corev1.Namespace {
	n := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	return n
}

func (a *Accessor) CatchInterrupt(namespace string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		err := a.DeleteNamespace(namespace)
		if err != nil {
			log.Errorf("Error while deleting the Namespace: %s", namespace)
		}
		os.Exit(2)
	}()
	log.Debugf("Caught an Interrupt, deleted the namespace")
}

// NamespaceExists returns true if the given namespace exists.
func (a *Accessor) NamespaceExists(ns string) bool {
	allNs, err := a.set.CoreV1().Namespaces().List(a.context, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for i := range allNs.Items {
		n := allNs.Items[i]
		if n.Name == ns {
			return true
		}
	}
	return false
}

// DeleteNamespace with the given name
func (a *Accessor) DeleteNamespace(ns string) error {
	log.Debugf("Deleting namespace: %s", ns)
	return a.set.CoreV1().Namespaces().Delete(a.context, ns, deleteOptionsForeground())
}

// WaitForNamespaceDeletion waits until a namespace is deleted.
func (a *Accessor) WaitForNamespaceDeletion(ns string, opts ...retry.Option) error {
	_, err := retry.Do(func() (interface{}, bool, error) {
		_, err2 := a.set.CoreV1().Namespaces().Get(a.context, ns, metav1.GetOptions{})
		if err2 == nil {
			return nil, false, nil
		}

		if errors.IsNotFound(err2) {
			return nil, true, nil
		}

		return nil, true, err2
	}, newRetryOptions(opts...)...)

	return err
}

// GetJobs returns k8s jobs in the given namespace, based on the selectors. If no selectors are given, then
// all the jobs are returned.
func (a *Accessor) GetNodes() ([]corev1.Node, error) {
	list, err := a.set.CoreV1().Nodes().List(a.context, metav1.ListOptions{})

	if err != nil {
		return []corev1.Node{}, err
	}

	return list.Items, nil
}

// GetNamespace returns the K8s namespaceresource with the given name.
func (a *Accessor) GetNamespace(ns string) (*corev1.Namespace, error) {
	n, err := a.set.CoreV1().Namespaces().Get(a.context, ns, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (a *Accessor) GetObject(namespace, object string) (string, error) {
	return a.ctl.get(namespace, object)
}

// ApplyContents applies the given config contents using kubectl.
func (a *Accessor) ApplyContents(namespace, contents string) ([]string, error) {
	return a.ctl.applyContents(namespace, contents)
}

// Apply the config in the given filename using kubectl.
func (a *Accessor) Apply(namespace, filename string) error {
	return a.ctl.apply(namespace, filename)
}

// DeleteContents deletes the given config contents using kubectl.
func (a *Accessor) DeleteContents(namespace, contents string) error {
	return a.ctl.deleteContents(namespace, contents)
}

// Delete the config in the given filename using kubectl.
func (a *Accessor) Delete(namespace, filename string) error {
	return a.ctl.delete(namespace, filename)
}

// Logs calls the logs command for the specified pod, with -c, if container is specified.
func (a *Accessor) Logs(namespace, pod, container string, previousLog bool) (string, error) {
	return a.ctl.logs(namespace, pod, container, previousLog)
}

// Exec executes the provided command on the specified pod/container.
func (a *Accessor) Exec(namespace, pod, container, command string) (string, error) {
	return a.ctl.exec(namespace, pod, container, command)
}

func (a *Accessor) Cp(namespace, podName, containerName, srcPath, destPath string) error {
	return a.ctl.cp(namespace, podName, containerName, srcPath, destPath)
}

func (a *Accessor) GetKubeClient() ctrlRuntime.Client {
	return a.client
}

func (a *Accessor) GetClientSet() *client.Clientset {
	return a.set
}

func (a *Accessor) GetContext() context.Context {
	return a.context
}

func (a *Accessor) GetValidatingWebhookConfiguration(name string) (*admissionregistrationv1beta1.ValidatingWebhookConfiguration,
	error) {
	return a.set.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Get(a.context, name, metav1.GetOptions{})
}

func (a *Accessor) GetMutatingWebhookConfiguration(name string) (*admissionregistrationv1beta1.MutatingWebhookConfiguration,
	error) {
	return a.set.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(a.context, name, metav1.GetOptions{})
}

// ValidatingWebhookConfigurationExists indicates whether a mutating validating with the given name exists.
func (a *Accessor) ValidatingWebhookConfigurationExists(name string) bool {
	_, err := a.set.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Get(a.context, name, metav1.GetOptions{})
	return err == nil
}

// MutatingWebhookConfigurationExists checks if mutating configuration with given name exists.
func (a *Accessor) MutatingWebhookConfigurationExists(name string) bool {
	_, err := a.set.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(a.context, name, metav1.GetOptions{})
	return err == nil
}

func (a *Accessor) DeleteValidatingWebhookConfiguration(name string) error {
	return a.set.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(a.context, name, metav1.DeleteOptions{})
}

func (a *Accessor) DeleteMutatingWebhookConfiguration(name string) error {
	return a.set.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(a.context, name, metav1.DeleteOptions{})
}

func (a *Accessor) GetServiceAccount(name, namespace string) (*corev1.ServiceAccount, error) {
	return a.set.CoreV1().ServiceAccounts(namespace).Get(a.context, name, metav1.GetOptions{})
}

// DeleteServiceAccount deletes the given sa in the passed namespace.
func (a *Accessor) DeleteServiceAccount(name, namespace string) error {
	return a.set.CoreV1().ServiceAccounts(namespace).Delete(a.context, name, metav1.DeleteOptions{})
}

// CheckPodReady returns nil if the given pod and all of its containers are ready.
func CheckPodReady(pod *corev1.Pod) error {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return nil
	case corev1.PodRunning:
		// Wait until all containers are ready.
		for i := range pod.Status.ContainerStatuses {
			containerStatus := pod.Status.ContainerStatuses[i]
			if !containerStatus.Ready {
				return fmt.Errorf("container not ready: '%s'", containerStatus.Name)
			}
		}
		return nil
	default:
		return fmt.Errorf("%s", pod.Status.Phase)
	}
}

// func CheckPodCompleted checks if pod completed successfully.
func CheckPodCompleted(pod *corev1.Pod) error {
	if pod.Status.Phase == corev1.PodSucceeded {
		return nil
	}
	return fmt.Errorf("%s", pod.Status.Phase)
}

func deleteOptionsForeground() metav1.DeleteOptions {
	propagationPolicy := metav1.DeletePropagationForeground
	var deleteImmediately int64
	return metav1.DeleteOptions{
		PropagationPolicy:  &propagationPolicy,
		GracePeriodSeconds: &deleteImmediately,
	}
}

func newRetryOptions(opts ...retry.Option) []retry.Option {
	out := make([]retry.Option, 0, 2+len(opts))
	out = append(out, defaultRetryTimeout, defaultRetryDelay)
	out = append(out, opts...)
	return out
}

// TODO associate GetPodsOfAppKind function with accessor
//nolint:gocritic
func GetPodsOfAppKind(ctx context.Context, cl ctrlRuntime.Client, ns string,
	resource runtime.Object) (*corev1.PodList, error) {
	var (
		matchLabels map[string]string
		matchExp    []metav1.LabelSelectorRequirement
		selectors   labels.Selector
		// ownerRef    metav1.OwnerReference
		podList = corev1.PodList{
			TypeMeta: metav1.TypeMeta{
				Kind:       common.PodKind,
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
		}
		tmpPodList = podList
	)

	err := fmt.Errorf("failed to get list of pods by label selector as label selector is nil")

	switch obj := resource.(type) {
	case *appsv1.DaemonSet:
		if obj.Spec.Selector == nil {
			return nil, err
		}
		matchLabels = obj.Spec.Selector.MatchLabels
		matchExp = obj.Spec.Selector.MatchExpressions
		// ownerRef = *metav1.NewControllerRef(obj, appsv1.SchemeGroupVersion.WithKind(internal.DaemonSetKind))

	case *appsv1.Deployment:
		if obj.Spec.Selector == nil {
			return nil, err
		}
		matchLabels = obj.Spec.Selector.MatchLabels
		matchExp = obj.Spec.Selector.MatchExpressions
		// ownerRef = *metav1.NewControllerRef(obj, appsv1.SchemeGroupVersion.WithKind(internal.DeploymentKind))

	case *appsv1.StatefulSet:
		if obj.Spec.Selector == nil {
			return nil, err
		}
		matchLabels = obj.Spec.Selector.MatchLabels
		matchExp = obj.Spec.Selector.MatchExpressions
		// ownerRef = *metav1.NewControllerRef(obj, appsv1.SchemeGroupVersion.WithKind(internal.StatefulSetKind))

	case *appsv1.ReplicaSet:
		if obj.Spec.Selector == nil {
			return nil, err
		}
		matchLabels = obj.Spec.Selector.MatchLabels
		matchExp = obj.Spec.Selector.MatchExpressions
		// ownerRef = *metav1.NewControllerRef(obj, appsv1.SchemeGroupVersion.WithKind(internal.ReplicaSetKind))

	case *corev1.ReplicationController:
		if obj.Spec.Selector == nil {
			return nil, err
		}
		matchLabels = obj.Spec.Selector
		// ownerRef = *metav1.NewControllerRef(obj, corev1.SchemeGroupVersion.WithKind(internal.ReplicationControllerKind))

	case *batchv1.Job:
		if obj.Spec.Selector == nil {
			return nil, err
		}
		matchLabels = obj.Spec.Selector.MatchLabels
		matchExp = obj.Spec.Selector.MatchExpressions
		// ownerRef = *metav1.NewControllerRef(obj, batchv1.SchemeGroupVersion.WithKind(internal.JobKind))

	case *batchv1beta1.CronJob:

		if obj.Spec.JobTemplate.Spec.Selector != nil {
			matchLabels = obj.Spec.JobTemplate.Spec.Selector.MatchLabels
			matchExp = obj.Spec.JobTemplate.Spec.Selector.MatchExpressions
		} else {
			jobList := batchv1.JobList{
				TypeMeta: metav1.TypeMeta{
					Kind:       common.JobKind,
					APIVersion: batchv1.SchemeGroupVersion.String(),
				},
			}

			err = cl.List(ctx, &jobList, ctrlRuntime.InNamespace(ns))
			if err != nil {
				if errors.IsNotFound(err) {
					log.Error(err, "error while getting the job list for CronJob", "in namespace", ns)
					return &tmpPodList, err
				}
			}

			jobs := GetChildJobs(&jobList, obj)
			var pods *corev1.PodList
			for i := range jobs {
				pods, err = GetPodsOfAppKind(ctx, cl, ns, &jobs[i])
				if err != nil {
					return nil, err
				}
				podList.Items = append(podList.Items, pods.Items...)
			}
			return &podList, nil
		}
	}

	// ownerRef = *metav1.NewControllerRef(obj, batchv1beta1.SchemeGroupVersion.WithKind(internal.CronJobKind))

	selectors, _ = metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      matchLabels,
		MatchExpressions: matchExp,
	})

	err = cl.List(ctx, &podList, ctrlRuntime.MatchingLabelsSelector{Selector: selectors},
		ctrlRuntime.InNamespace(ns))
	if err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "error while getting the pods of the deployment", "in namespace", ns)
			return &tmpPodList, err
		}
	}

	// TODO Add support to verify the owner ref
	// for i := 0; i < len(podList.Items); i++ {
	//	pod := podList.Items[i]
	//	// check if owner ref UID is same as that of this object to be parsed
	//	podOwners := pod.GetOwnerReferences()
	//	for j := 0; j < len(podOwners); j++ {
	//		po := podOwners[j]
	//		if strings.Compare(po.Kind, ownerRef.Kind) == 0 && strings.Compare(po.APIVersion, ownerRef.APIVersion) == 0 &&
	//			strings.Compare(po.Name, ownerRef.Name) == 0 {
	//			// TODO In some cases the owner UID is coming nil that's why following check is failing
	//			// TODO Check why this is failing?
	//			//strings.Compare(string(po.UID), string(ownerRef.UID)) == 0 {
	//			tmpPodList.Items = append(tmpPodList.Items, pod)
	//			break
	//		}
	//	}
	//}

	return &podList, nil
}

func GetChildJobs(jobList *batchv1.JobList, owner runtime.Object) []batchv1.Job {
	var children []batchv1.Job

	if owner == nil || len(jobList.Items) == 0 {
		return children
	}
	metaOwner, err := meta.Accessor(owner)
	if err != nil {
		log.Error(err, "Error while converting the owner to meta accessor format")
		return children
	}
	matchUID := metaOwner.GetUID()
	for itemIndex := range jobList.Items {
		item := jobList.Items[itemIndex]
		refs := item.GetOwnerReferences()
		for i := 0; i < len(refs); i++ {
			or := refs[i]
			if or.UID == matchUID {
				children = append(children, item)
			}
		}
	}

	return children
}

func (a *Accessor) GetClusterRole(name string) (*v1.ClusterRole, error) {
	cr, err := a.set.RbacV1().ClusterRoles().Get(a.context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cr, nil
}

func (a *Accessor) GetClusterRoleBinding(name string) (*v1.ClusterRoleBinding, error) {
	crb, err := a.set.RbacV1().ClusterRoleBindings().Get(a.context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return crb, nil
}

func (a *Accessor) GetRoleBinding(name, ns string) (*v1.RoleBinding, error) {
	rb, err := a.set.RbacV1().RoleBindings(ns).Get(a.context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return rb, nil
}

func (a *Accessor) DeleteClusterRole(name string) error {
	if err := a.set.RbacV1().ClusterRoles().Delete(a.context, name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (a *Accessor) DeleteClusterRoleBinding(name string) error {
	if err := a.set.RbacV1().ClusterRoleBindings().Delete(a.context, name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (a *Accessor) GetIngress(name, namespace string) (*networkingv1beta1.Ingress, error) {
	return a.set.NetworkingV1beta1().Ingresses(namespace).Get(a.context, name, metav1.GetOptions{})
}

func (a *Accessor) DeleteIngress(name, namespace string) error {
	return a.set.NetworkingV1beta1().Ingresses(namespace).Delete(a.context, name, metav1.DeleteOptions{})
}

// ClusterRoleBindingExists checks if ClusterRoleBindings with given name exists.
func (a *Accessor) ClusterRoleBindingExists(name string) bool {
	_, err := a.set.RbacV1().ClusterRoleBindings().Get(a.context, name, metav1.GetOptions{})
	return err == nil
}

// RoleBindingExists checks if RoleBindings with given name exists.
func (a *Accessor) RoleBindingExists(name, ns string) bool {
	_, err := a.set.RbacV1().RoleBindings(ns).Get(a.context, name, metav1.GetOptions{})
	return err == nil
}

// GetStorageClass returns sc of given name.
func (a *Accessor) GetStorageClass(name string) (*storageV1.StorageClass, error) {
	sc, err := a.set.StorageV1().StorageClasses().Get(a.context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (a *Accessor) MatchPodLabels(podLabels, matchLabels map[string]string) bool {
	for k, v := range matchLabels {
		value, ok := podLabels[k]
		if !ok {
			return false
		}
		if value != v {
			return false
		}
	}
	return true
}

func (a *Accessor) MatchPodExpr(podLabels map[string]string, matchExpr []metav1.LabelSelectorRequirement) bool {

	for i := range matchExpr {
		expr := matchExpr[i]
		matchReq, err := labels.NewRequirement(expr.Key, matchExpressionOperator[expr.Operator], expr.Values)
		if err != nil {
			log.Errorf("failed to create requirement")
		}
		if !matchReq.Matches(labels.Set(podLabels)) {
			return false
		}
	}
	return true
}

func (a *Accessor) MatchPodLabelSelector(podLabels map[string]string, labelSelector []metav1.LabelSelector) bool {
	if len(labelSelector) == 0 {
		return true
	}

	for i := range labelSelector {
		ls := labelSelector[i]
		if a.MatchPodLabels(podLabels, ls.MatchLabels) && a.MatchPodExpr(podLabels, ls.MatchExpressions) {
			return true
		}
	}
	return false
}
