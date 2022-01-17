package preflight

import (
	"context"
	"fmt"
	gort "runtime"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	goclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight/exec"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"
)

const (
	windowsOSTarget    = "windows"
	windowsCheckSymbol = "\u2713"
	windowsCrossSymbol = "[X]"

	minHelmVersion = "3.0.0"
	minK8sVersion  = "1.18.0"

	rbacAPIGroup   = "rbac.authorization.k8s.io"
	rbacAPIVersion = "v1"

	letterBytes = "abcdefghijklmnopqrstuvwxyz"

	labelK8sPartOf                  = "app.kubernetes.io/part-of"
	labelK8sPartOfValue             = "k8s-triliovault"
	labelTrilioKey                  = "trilio"
	labelTvkPreflightValue          = "tvk-preflight"
	labelPreflightRunKey            = "preflight-run"
	labelK8sName                    = "app.kubernetes.io/name"
	labelK8sNameValue               = "k8s-triliovault"
	sourcePod                       = "source-pod-"
	sourcePvc                string = "source-pvc-"
	volumeSnapSrc                   = "snapshot-source-pvc-"
	customResourceDefinition        = "CustomResourceDefinition"

	apiExtenstionsGroup    = "apiextensions.k8s.io"
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

	execTimeoutDuration       = 3 * time.Minute
	deletionGracePeriod int64 = 5
)

var (
	check = "\xE2\x9C\x94"
	cross = "\xE2\x9D\x8C"

	csiApis = [3]string{
		"volumesnapshotclasses." + storageSnapshotGroup,
		"volumesnapshotcontents." + storageSnapshotGroup,
		"volumesnapshots." + storageSnapshotGroup,
	}

	resourceRequirements = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse("64Mi"),
			corev1.ResourceCPU:    resource.MustParse("250m"),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse("128Mi"),
			corev1.ResourceCPU:    resource.MustParse("500m"),
		},
	}

	storageVolSnapClass    string
	scheme                 = runtime.NewScheme()
	resNameSuffix          string
	commandBinSh           = []string{"bin/sh", "-c"}
	commandSleep3600       = []string{"sleep", "3600"}
	volSnapPodFilePath     = "/demo/data/sample-file.txt"
	volSnapPodFileData     = "pod preflight data"
	argsTouchDataFileSleep = []string{
		fmt.Sprintf("echo '%s' > %s && sleep 3000", volSnapPodFileData, volSnapPodFilePath),
	}
	execRestoreDataCheckCommand = []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("dat=$(cat \"%s\"); echo \"${dat}\"; if [[ \"${dat}\" == \"%s\" ]]; then exit 0; else exit 1; fi",
			volSnapPodFilePath, volSnapPodFileData),
	}

	clientSet     *goclient.Clientset
	runtimeClient client.Client
	discClient    *discovery.DiscoveryClient
	restConfig    *rest.Config
)

type CommonOptions struct {
	Kubeconfig string
	Namespace  string
	Logger     *logrus.Logger
}

func InitKubeEnv(kubeconfig string) error {
	if gort.GOOS == windowsOSTarget {
		check = windowsCheckSymbol
		cross = windowsCrossSymbol
	}

	utilruntime.Must(corev1.AddToScheme(scheme))
	kubeEnv, err := internal.NewEnv(kubeconfig, scheme)
	if err != nil {
		return err
	}
	clientSet = kubeEnv.GetClientset()
	runtimeClient = kubeEnv.GetRuntimeClient()
	discClient = kubeEnv.GetDiscoveryClient()
	restConfig = kubeEnv.GetRestConfig()

	return nil
}

func getServerPreferredVersionForGroup(grp string) (string, error) {
	var (
		apiResList  *metav1.APIGroupList
		err         error
		prefVersion string
	)
	apiResList, err = clientSet.ServerGroups()
	if err != nil {
		return "", err
	}
	for idx := range apiResList.Groups {
		api := apiResList.Groups[idx]
		if api.Name == grp {
			prefVersion = api.PreferredVersion.Version
			break
		}
	}

	if prefVersion == "" {
		return "", fmt.Errorf("no preferred version for group - %s found on cluster", grp)
	}
	return prefVersion, nil
}

func getVersionsOfGroup(grp string) ([]string, error) {
	var (
		apiResList *metav1.APIGroupList
		err        error
		apiVerList []string
	)
	apiResList, err = clientSet.ServerGroups()
	if err != nil {
		return nil, err
	}
	for idx := range apiResList.Groups {
		api := apiResList.Groups[idx]
		if api.Name == grp {
			for _, gv := range api.Versions {
				apiVerList = append(apiVerList, gv.Version)
			}
		}
	}

	return apiVerList, nil
}

//  clusterHasVolumeSnapshotClass checks and returns volume snapshot class if present on cluster.
func clusterHasVolumeSnapshotClass(ctx context.Context, snapshotClass, namespace string) (*unstructured.Unstructured, error) {
	var (
		prefVersion string
		err         error
	)
	prefVersion, err = getServerPreferredVersionForGroup(storageSnapshotGroup)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: prefVersion,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = runtimeClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      snapshotClass,
	}, u)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("volume snapshot class %s not found on cluster :: %s", snapshotClass, err.Error())
		}
		return nil, err
	}

	return u, nil
}

//  createDNSPodSpec returns a corev1.Pod instance.
func createDNSPodSpec(op *Options) *corev1.Pod {
	var imagePath string
	if op.LocalRegistry != "" {
		imagePath = op.LocalRegistry
	} else {
		imagePath = gcrRegistryPath
	}
	pod := getPodTemplate(dnsUtils+resNameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            dnsContainerName,
			Image:           strings.Join([]string{imagePath, dnsUtilsImage}, "/"),
			Command:         commandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       resourceRequirements,
		},
	}

	return pod
}

func createVolumeSnapshotPVCSpec(storageClass, namespace string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: getObjectMetaTemplate(sourcePvc+resNameSuffix, namespace),
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
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = strings.Join([]string{op.LocalRegistry, "/", busyboxImageName}, "")
	} else {
		containerImage = busyboxImageName
	}
	pod := getPodTemplate(sourcePod+resNameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:      busyboxContainerName,
			Image:     containerImage,
			Command:   commandBinSh,
			Args:      argsTouchDataFileSleep,
			Resources: resourceRequirements,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volMountName,
					MountPath: volMountPath,
				},
			},
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 30,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"cat", volSnapPodFilePath,
						},
					},
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
	volSnap.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: snapVer,
		Kind:    internal.VolumeSnapshotKind,
	})
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
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = strings.Join([]string{op.LocalRegistry, "/", busyboxImageName}, "")
	} else {
		containerImage = busyboxImageName
	}
	pod := getPodTemplate(podName, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            busyboxContainerName,
			Image:           containerImage,
			Command:         commandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       resourceRequirements,
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
	return &corev1.Pod{
		ObjectMeta: getObjectMetaTemplate(name, op.Namespace),
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: op.ImagePullSecret},
			},
			ServiceAccountName: op.ServiceAccountName,
		},
	}
}

func getObjectMetaTemplate(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    getPreflightResourceLabels(),
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
	logger.Infof("Executing command 'exec %s' in container - '%s' of pod - '%s'\n",
		strings.Join(execOp.Command, " "), execOp.ContainerName, execOp.PodName)
	go execOp.ExecInContainer(execChan)
	select {
	case execRes = <-execChan:
		if execRes != nil && execRes.Err != nil {
			logger.Warnf("exec command failed on %s in pod %s :: %s\n",
				execOp.ContainerName, execOp.PodName, execRes.Stderr)
			return execRes.Err
		}

	case <-time.After(execTimeoutDuration):
		return fmt.Errorf("exec operation took too long on container %s in pod %s", execOp.ContainerName, execOp.PodName)
	}

	logger.Infof("%s Command 'exec %s' in container - '%s' of pod - '%s' executed successfully\n",
		check, strings.Join(execOp.Command, " "), execOp.ContainerName, execOp.PodName)

	return nil
}

func removeFinalizer(ctx context.Context, obj client.Object) error {
	var err error
	obj.SetFinalizers([]string{})
	err = runtimeClient.Update(ctx, obj)
	if err != nil {
		return err
	}
	return nil
}

func deleteK8sResourceWithForceTimeout(ctx context.Context, obj client.Object, logger *logrus.Logger) error {
	var err error
	err = removeFinalizer(ctx, obj)
	if err != nil {
		logger.Warnf("problem occurred while removing finalizers of %s - %s :: %s",
			obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
	}
	err = runtimeClient.Delete(ctx, obj, client.DeleteOption(client.GracePeriodSeconds(deletionGracePeriod)))
	if err != nil {
		return fmt.Errorf("problem occurred deleting %s - %s :: %s", obj.GetName(), obj.GetNamespace(), err.Error())
	}

	return nil
}

// getDefaultRetryBackoffParams returns a backoff object with timeout of approx. 5 min
func getDefaultRetryBackoffParams() k8swait.Backoff {
	return k8swait.Backoff{
		Steps: volSnapRetrySteps, Duration: volSnapRetryInterval,
		Factor: volSnapRetryFactor, Jitter: volSnapRetryJitter,
	}
}
