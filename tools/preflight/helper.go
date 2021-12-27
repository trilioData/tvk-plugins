package preflight

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	goclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	utilretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sirupsen/logrus"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight/exec"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"
)

const (
	check = "\xE2\x9C\x94"
	cross = "\xE2\x9D\x8C"

	minHelmVersion = "3.0.0"
	minK8sVersion  = "1.18.0"

	rbacAPIGroup = "rbac.authorization.k8s.io"

	letterBytes = "abcdefghijklmnopqrstuvwxyz"

	labelK8sPartOf                = "app.kubernetes.io/part-of"
	labelK8sPartOfValue           = "k8s-triliovault"
	labelTrilioKey                = "trilio"
	labelTvkPreflightValue        = "tvk-preflight"
	labelPreflightRunKey          = "preflight-run"
	labelK8sName                  = "app.kubernetes.io/name"
	labelK8sNameValue             = "k8s-triliovault"
	sourcePod                     = "source-pod-"
	sourcePvc              string = "source-pvc-"
	volumeSnapSrc                 = "snapshot-source-pvc-"

	apiExtenstionsGroup    = "apiextensions.k8s.io"
	v1StorageSnapshot      = "v1.snapshot.storage.k8s.io"
	v1beta1StorageSnapshot = "v1beta1.snapshot.storage.k8s.io"
	storageSnapshotGroup   = "snapshot.storage.k8s.io"
	restorePvc             = "restored-pvc-"
	restorePod             = "restored-pod-"
	busyboxContainerName   = "busybox"
	busyboxImageName       = "busybox"
	unmountedRestorePod    = "unmounted-restored-pod-"
	unmountedRestorePvc    = "unmounted-restored-pvc-"
	unmountedVolumeSnapSrc = "unmounted-source-pvc-"

	dnsUtils         = "dnsutils-"
	dnsContainerName = "dnsutils"
	gcrRegistryPath  = "gcr.io/kubernetes-e2e-test-images"
	dnsUtilsImage    = "dnsutils:1.3"
	ocpAPIVersion    = "security.openshift.io/v1"

	volSnapRetrySteps                  = 30
	volSnapRetryInterval time.Duration = 2 * time.Second
	volSnapRetryFactor                 = 1.1
	volSnapRetryJitter                 = 0.1
	volMountName                       = "source-data"
	volMountPath                       = "/demo/data"

	execTimeoutDuration = 3 * time.Minute
)

var (
	csiApis = [3]string{
		"volumesnapshotclasses.snapshot.storage.k8s.io",
		"volumesnapshotcontents.snapshot.storage.k8s.io",
		"volumesnapshots.snapshot.storage.k8s.io",
	}

	storageVolSnapClass    string
	scheme                 = runtime.NewScheme()
	resNameSuffix          string
	commandBinSh           = []string{"bin/sh", "-c"}
	commandSleep3600       = []string{"sleep", "3600"}
	argsTouchDataFileSleep = []string{"touch /demo/data/sample-file.txt && sleep 3000"}
	clientSet              *goclient.Clientset
	runtimeClient          client.Client
	discClient             *discovery.DiscoveryClient
	restConfig             *rest.Config
)

// CreateLoggingFile creates a log file for pre flight check.
func CreateLoggingFile(filename string) (*os.File, error) {
	year, month, day := time.Now().Date()
	hour, minute, sec := time.Now().Clock()
	logFilename := filename + "-log" + strconv.Itoa(year) + "-" + strconv.Itoa(int(month)) + "-" + strconv.Itoa(day) +
		"T" + strconv.Itoa(hour) + "-" + strconv.Itoa(minute) + "-" + strconv.Itoa(sec) + ".log"

	return os.OpenFile(logFilename, os.O_CREATE|os.O_WRONLY, 0644)
}

func getStorageSnapshotVersion(runtimeClient client.Client) (string, error) {
	ssverList := unstructured.UnstructuredList{}
	ssverList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiregistration.k8s.io",
		Version: "v1",
		Kind:    "APIService",
	})
	if err := runtimeClient.List(context.Background(), &ssverList); err != nil {
		return "", err
	}
	for _, apiServ := range ssverList.Items {
		if apiServ.GetName() == v1StorageSnapshot {
			return "v1", nil
		}
	}

	ssverList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiregistration.k8s.io",
		Version: "v1beta1",
		Kind:    "APIService",
	})
	if err := runtimeClient.List(context.Background(), &ssverList); err != nil {
		return "", err
	}
	for _, apiServ := range ssverList.Items {
		if apiServ.GetName() == v1beta1StorageSnapshot {
			return "v1beta1", nil
		}
	}

	return "", fmt.Errorf("no compatible volume snapshot version found in cluster")
}

func getVersionsOfGroup(grp string) ([]string, error) {
	var (
		apiResList []*metav1.APIResourceList
		err        error
		apiVerList []string
	)
	apiResList, err = clientSet.ServerPreferredResources()
	if err != nil {
		return nil, err
	}
	for _, api := range apiResList {
		gv := strings.Split(api.GroupVersion, "/")
		if gv[0] == grp {
			apiVerList = append(apiVerList, gv[1])
		}
	}

	return apiVerList, nil
}

//  isOCPk8sCluster checks whether the cluster is OCP cluster.
func isOCPk8sCluster(logger *logrus.Logger) bool {
	_, err := discClient.ServerResourcesForGroupVersion(ocpAPIVersion)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Infoln(fmt.Sprintf("APIVersion - %s not found on cluster, not an OCP cluster", ocpAPIVersion))
			return false
		}
		logger.Warnln(err)
		return false
	}
	return true
}

//  getAllVolumeSnapshotClass fetches all the VolumeSnapshots present in the cluster.
func getAllVolumeSnapshotClass(gvk schema.GroupVersionKind) (*unstructured.UnstructuredList, error) {
	vsscList := unstructured.UnstructuredList{}
	vsscList.SetGroupVersionKind(gvk)
	err := runtimeClient.List(context.Background(), &vsscList)

	return &vsscList, err
}

//  clusterHasVolumeSnapshotClass checks and returns volume snapshot class if present on cluster.
func clusterHasVolumeSnapshotClass(snapshotClass string) (*unstructured.Unstructured, error) {
	vsscList, err := getAllVolumeSnapshotClass(schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: "v1beta1",
		Kind:    "VolumeSnapshotClass",
	})
	if err != nil {
		return nil, err
	}

	for _, vssc := range vsscList.Items {
		if vssc.GetName() == snapshotClass {
			return &vssc, nil
		}
	}

	return nil, fmt.Errorf("volume snapshot class %s not found", snapshotClass)
}

//  createDNSPodSpec returns a corev1.Pod instance.
func createDNSPodSpec(imagePath string, op *Options) *corev1.Pod {
	pod := getPodTemplate(dnsUtils+resNameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            dnsContainerName,
			Image:           imagePath + "/" + dnsUtilsImage,
			Command:         commandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources: corev1.ResourceRequirements{
				Requests: getResourceRequirementsMap("64Mi", "250m"),
				Limits:   getResourceRequirementsMap("128Mi", "500m"),
			},
		},
	}

	return pod
}

func createVolumeSnapshotPVCSpec(storageClass, namespace string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourcePvc + resNameSuffix,
			Namespace: namespace,
			Labels:    getPreflightResourceLabels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	return pvc
}

func createVolumeSnapshotPodSpec(pvcName string, op *Options) *corev1.Pod {
	pod := getPodTemplate(sourcePod+resNameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:    busyboxContainerName,
			Image:   busyboxImageName,
			Command: commandBinSh,
			Args:    argsTouchDataFileSleep,
			Resources: corev1.ResourceRequirements{
				Requests: getResourceRequirementsMap("64Mi", "250m"),
				Limits:   getResourceRequirementsMap("128Mi", "500m"),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volMountName,
					MountPath: volMountPath,
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volMountName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
					ReadOnly:  false,
				},
			},
		},
	}

	return pod
}

// createVolumeSnapsotSpec creates pvc for volume snapshot
func createVolumeSnapsotSpec(name, namespace, snapVer, pvcName string) *unstructured.Unstructured {
	volSnap := &unstructured.Unstructured{}
	volSnap.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": storageVolSnapClass,
			"source": map[string]string{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}
	volSnap.SetName(name)
	volSnap.SetNamespace(namespace)
	volSnap.SetKind(internal.VolumeSnapshotKind)
	volSnap.SetAPIVersion(storageSnapshotGroup + "/" + snapVer)
	volSnap.SetLabels(getPreflightResourceLabels())

	return volSnap
}

// createRestorePVCSpec creates pvc for restore (unmounted pvc as well)
func createRestorePVCSpec(pvcName, dsName, storageClass, namespace string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels:    getPreflightResourceLabels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					"storage": resource.MustParse("1Gi"),
				},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				Kind:     internal.VolumeSnapshotKind,
				Name:     dsName,
				APIGroup: func() *string { str := storageSnapshotGroup; return &str }(),
			},
		},
	}

	return pvc
}

// createRestorePodSpec creates a restore pod
func createRestorePodSpec(podName, pvcName string, op *Options) *corev1.Pod {
	pod := getPodTemplate(podName, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            busyboxContainerName,
			Image:           busyboxImageName,
			Command:         commandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources: corev1.ResourceRequirements{
				Requests: getResourceRequirementsMap("64Mi", "250m"),
				Limits:   getResourceRequirementsMap("128Mi", "500m"),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volMountName,
					MountPath: volMountPath,
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volMountName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
					ReadOnly:  false,
				},
			},
		},
	}

	return pod
}

func getPodTemplate(name string, op *Options) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: getObjectMetaTemplate(name, op.Namespace),
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: op.ImagePullSecret},
			},
			ServiceAccountName: op.ServiceAccountName,
		},
	}

	return pod
}

func getObjectMetaTemplate(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    getPreflightResourceLabels(),
	}
}

func getResourceRequirementsMap(memory, cpu string) map[corev1.ResourceName]resource.Quantity {
	return map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceMemory: resource.MustParse(memory),
		corev1.ResourceCPU:    resource.MustParse(cpu),
	}
}

func getPreflightResourceLabels() map[string]string {
	return map[string]string{
		labelK8sName:         labelK8sNameValue,
		labelTrilioKey:       labelTvkPreflightValue,
		labelPreflightRunKey: resNameSuffix,
		labelK8sPartOf:       labelK8sPartOfValue,
	}
}

// waitUntilPodCondition waits until pod reaches the given condition or timeouts.
func waitUntilPodCondition(ctx context.Context, wop *wait.PodWaitOptions) error {
	res := wop.WaitOnPod(ctx, getDefaultRetryBackoffParams())
	if res.Err != nil {
		return res.Err
	} else if !res.ReachedCondn {
		return fmt.Errorf("pod %s hasn't reached into %s state", wop.Name, string(wop.PodCondition))
	}

	return nil
}

// waitUntilVolSnapReadyToUse waits until volume snapshot becomes ready or timeouts
func waitUntilVolSnapReadyToUse(volSnap *unstructured.Unstructured, snapshotVer string, retryBackoff k8swait.Backoff) error {
	retErr := k8swait.ExponentialBackoff(retryBackoff, func() (done bool, err error) {
		volSnapSrc := &unstructured.Unstructured{}
		volSnapSrc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   storageSnapshotGroup,
			Version: snapshotVer,
			Kind:    internal.VolumeSnapshotKind,
		})
		err = runtimeClient.Get(context.Background(), client.ObjectKey{
			Namespace: volSnap.GetNamespace(),
			Name:      volSnap.GetName(),
		}, volSnapSrc)
		if err != nil {
			return false, err
		}

		ready, found, err := unstructured.NestedBool(volSnapSrc.Object, "status", "readyToUse")
		if found && err == nil && ready {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
	if retErr != nil {
		if retErr == k8swait.ErrWaitTimeout {
			return fmt.Errorf("volume snapshot from source pvc not readyToUse (waited 300 sec) :: %s", retErr.Error())
		}
		return retErr
	}
	return nil

}

// execInPod executes exec command on a container of a pod.
func execInPod(execOp *exec.Options, logger *logrus.Logger) error {
	var execRes *exec.Response
	var execChan = make(chan *exec.Response)
	var retErr error
	logger.Infoln(fmt.Sprintf("Executing command 'exec %s' in container - '%s' of pod - '%s'",
		strings.Join(execOp.Command, " "), execOp.ContainerName, execOp.PodName))
	go execOp.ExecInContainer(execChan)
	select {
	case execRes = <-execChan:
		if execRes != nil && execRes.Err != nil {
			logger.Warnln(fmt.Sprintf("exec command failed on %s in pod %s :: %s",
				execOp.ContainerName, execOp.PodName, execRes.Err.Error()))
		}
		break
	case <-time.After(execTimeoutDuration):
		retErr = fmt.Errorf("exec operation took too long on container %s in pod %s", execOp.ContainerName, execOp.PodName)
	}

	return retErr
}

func removeFinalizer(ctx context.Context, obj interface{}) error {
	var err error
	switch objType := obj.(type) {
	case *corev1.Pod:
		key := getObjNamespacedName(objType)
		retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
			err = runtimeClient.Get(ctx, key, objType)
			if err != nil {
				return err
			}
			objType.ObjectMeta.Finalizers = []string{}
			err = runtimeClient.Update(ctx, objType)
			if err != nil {
				return err
			}
			return nil
		})
		return retErr

	case *corev1.PersistentVolumeClaim:
		key := getObjNamespacedName(objType)
		retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
			err = runtimeClient.Get(ctx, key, objType)
			if err != nil {
				return err
			}
			objType.ObjectMeta.Finalizers = []string{}
			err = runtimeClient.Update(ctx, objType)
			if err != nil {
				return err
			}
			return nil
		})
		return retErr

	case *unstructured.Unstructured:
		key := getObjNamespacedName(objType)
		retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
			err = runtimeClient.Get(ctx, key, objType)
			if err != nil {
				return err
			}
			objType.SetFinalizers([]string{})
			err = runtimeClient.Update(ctx, objType)
			if err != nil {
				return err
			}
			return nil
		})
		return retErr
	}

	return nil
}

func deleteK8sResourceWithForceTimeout(ctx context.Context, obj interface{}, logger *logrus.Logger) error {
	retErr := k8swait.ExponentialBackoff(getDefaultRetryBackoffParams(), func() (done bool, e error) {
		var err error
		switch objType := obj.(type) {
		case *corev1.Pod:
			if err = removeFinalizer(ctx, objType); err != nil {
				logger.Warnln(fmt.Sprintf("problem occurred while removing finalizers of %s - %s :: %s",
					objType.Kind, objType.GetName(), err.Error()))
			}
			if err = clientSet.CoreV1().Pods(objType.Namespace).Delete(ctx, objType.GetName(), metav1.DeleteOptions{
				GracePeriodSeconds: func() *int64 { var i int64; return &i }(),
			}); err != nil {
				logger.Errorln(fmt.Sprintf("problem occurred deleting %s - %s :: %s", objType.GetName(), objType.GetNamespace(), err.Error()))
				return false, nil
			}

		case *corev1.PersistentVolumeClaim:
			if err = removeFinalizer(ctx, objType); err != nil {
				logger.Warnln(fmt.Sprintf("problem occurred while removing finalizers of %s - %s :: %s",
					objType.Kind, objType.GetName(), err.Error()))
			}
			if err = clientSet.CoreV1().PersistentVolumeClaims(objType.Namespace).Delete(ctx, objType.Name, metav1.DeleteOptions{
				GracePeriodSeconds: func() *int64 { var i int64; return &i }(),
			}); err != nil {
				logger.Errorln(fmt.Sprintf("problem occurred deleting %s - %s :: %s", objType.GetName(), objType.GetNamespace(), err.Error()))
				return false, nil
			}

		case *unstructured.Unstructured:
			if err = removeFinalizer(ctx, objType); err != nil {
				logger.Warnln(fmt.Sprintf("problem occurred while removing finalizers of %s - %s :: %s",
					objType.GetKind(), objType.GetName(), err.Error()))
			}
			if err = runtimeClient.Delete(ctx, objType, client.DeleteOption(&client.DeleteOptions{
				GracePeriodSeconds: func() *int64 { var i int64; return &i }(),
			})); err != nil {
				logger.Errorln(fmt.Sprintf("problem occurred deleting %s - %s :: %s", objType.GetName(), objType.GetNamespace(), err.Error()))
				return false, nil
			}
		}

		return true, nil
	})

	return retErr
}

func getObjNamespacedName(obj client.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

// getDefaultRetryBackoffParams returns a backoff object with timeout of approx. 5 min
func getDefaultRetryBackoffParams() k8swait.Backoff {
	return k8swait.Backoff{
		Steps: volSnapRetrySteps, Duration: volSnapRetryInterval,
		Factor: volSnapRetryFactor, Jitter: volSnapRetryJitter,
	}
}
