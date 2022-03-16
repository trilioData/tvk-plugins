package preflight

import (
	"context"
	"embed"
	"fmt"
	gort "runtime"
	"strings"
	"time"

	semVersion "github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight/exec"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"
)

const (
	windowsOSTarget    = "windows"
	windowsCheckSymbol = "\u2713"
	windowsCrossSymbol = "[X]"

	minHelmVersion = "3.0.0"
	minK8sVersion  = "1.19.0"

	RBACAPIGroup   = "rbac.authorization.k8s.io"
	RBACAPIVersion = "v1"

	letterBytes = "abcdefghijklmnopqrstuvwxyz"

	LabelK8sPartOf                 = "app.kubernetes.io/part-of"
	LabelK8sPartOfValue            = "k8s-triliovault"
	LabelTrilioKey                 = "trilio"
	LabelTvkPreflightValue         = "tvk-preflight"
	LabelPreflightRunKey           = "preflight-run"
	LabelK8sName                   = "app.kubernetes.io/name"
	LabelK8sNameValue              = "k8s-triliovault"
	SourcePodNamePrefix            = "source-pod-"
	SourcePvcNamePrefix     string = "source-pvc-"
	VolumeSnapSrcNamePrefix        = "snapshot-source-pvc-"

	StorageSnapshotGroup             = "snapshot.storage.k8s.io"
	RestorePvcNamePrefix             = "restored-pvc-"
	RestorePodNamePrefix             = "restored-pod-"
	BusyboxContainerName             = "busybox"
	BusyboxImageName                 = "busybox"
	UnmountedRestorePodNamePrefix    = "unmounted-restored-pod-"
	UnmountedRestorePvcNamePrefix    = "unmounted-restored-pvc-"
	UnmountedVolumeSnapSrcNamePrefix = "unmounted-source-pvc-"

	dnsUtils         = "dnsutils-"
	dnsContainerName = "dnsutils"
	GcrRegistryPath  = "gcr.io/kubernetes-e2e-test-images"
	DNSUtilsImage    = "dnsutils:1.3"

	volSnapRetrySteps    = 30
	volSnapRetryInterval = 2 * time.Second
	volSnapRetryFactor   = 1.1
	volSnapRetryJitter   = 0.1
	VolMountName         = "source-data"
	VolMountPath         = "/demo/data"

	execTimeoutDuration       = 3 * time.Minute
	deletionGracePeriod int64 = 5

	volumeSnapshotCRDYamlDir    = "volumesnapshotcrdyamls"
	snapshotClassVersionV1      = "v1"
	snapshotClassVersionV1Beta1 = "v1beta1"
	minServerVerForV1CrdVersion = "v1.20.0"
	defaultVSCName              = "preflight-generated-snapshot-class"
)

var (
	check = "\xE2\x9C\x94"
	cross = "\xE2\x9D\x8C"

	VolumeSnapshotCRDs = [3]string{
		"volumesnapshotclasses." + StorageSnapshotGroup,
		"volumesnapshotcontents." + StorageSnapshotGroup,
		"volumesnapshots." + StorageSnapshotGroup,
	}

	storageVolSnapClass    string
	scheme                 = runtime.NewScheme()
	resNameSuffix          string
	CommandBinSh           = []string{"bin/sh", "-c"}
	CommandSleep3600       = []string{"sleep", "3600"}
	VolSnapPodFilePath     = "/demo/data/sample-file.txt"
	VolSnapPodFileData     = "pod preflight data"
	ArgsTouchDataFileSleep = []string{
		fmt.Sprintf("echo '%s' > %s && sync %s && sleep 3000",
			VolSnapPodFileData, VolSnapPodFilePath, VolSnapPodFilePath),
	}
	execRestoreDataCheckCommand = []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("dat=$(cat \"%s\"); echo \"${dat}\"; if [[ \"${dat}\" == \"%s\" ]]; then exit 0; else exit 1; fi",
			VolSnapPodFilePath, VolSnapPodFileData),
	}

	execDNSResolutionCmd = []string{"nslookup", "kubernetes.default"}

	kubectlBinaryName = "kubectl"
	HelmBinaryName    = "helm"

	clientSet     *goclient.Clientset
	runtimeClient client.Client
	discClient    *discovery.DiscoveryClient
	restConfig    *rest.Config

	//go:embed volumesnapshotcrdyamls/*
	crdYamlFiles embed.FS
)

type CommonOptions struct {
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	Namespace  string `yaml:"namespace,omitempty"`
	LogLevel   string `yaml:"logLevel,omitempty"`
	InCluster  bool   `yaml:"inCluster,omitempty"`
	Logger     *logrus.Logger
}

func (co *CommonOptions) logCommonOptions() {
	co.Logger.Infof("LOG-LEVEL=\"%s\"", co.LogLevel)
	co.Logger.Infof("KUBECONFIG-PATH=\"%s\"", co.Kubeconfig)
	co.Logger.Infof("NAMESPACE=\"%s\"", co.Namespace)
	co.Logger.Infof("INCLUSTER=\"%t\"", co.InCluster)
}

type podSchedulingOptions struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
}

func InitKubeEnv(kubeconfig string) error {
	if gort.GOOS == windowsOSTarget {
		check = windowsCheckSymbol
		cross = windowsCrossSymbol
	}

	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiextensions.AddToScheme(scheme))
	var config *rest.Config
	if kubeconfig == "" {
		config = ctrl.GetConfigOrDie()
	}
	kubeEnv, err := internal.NewEnv(kubeconfig, config, scheme)
	if err != nil {
		return err
	}
	clientSet = kubeEnv.GetClientset()
	runtimeClient = kubeEnv.GetRuntimeClient()
	discClient = kubeEnv.GetDiscoveryClient()
	restConfig = kubeEnv.GetRestConfig()

	return nil
}

func GetHelmVersion(binaryName string) (string, error) {
	cmdOut, err := shell.RunCmd(fmt.Sprintf("%s ver"+
		"sion --template '{{.Version}}'", binaryName))
	if err != nil {
		return "", err
	}
	helmVersion := cmdOut.Out[2 : len(cmdOut.Out)-1]

	return helmVersion, nil
}

func GetServerPreferredVersionForGroup(grp string, cl *goclient.Clientset) (string, error) {
	if cl == nil {
		return "", fmt.Errorf("client object is nil, cannot fetch versions of group - %s", grp)
	}
	var (
		apiResList  *metav1.APIGroupList
		err         error
		prefVersion string
	)
	apiResList, err = cl.ServerGroups()
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

func getVersionsOfGroup(grp string, cl *goclient.Clientset) ([]string, error) {
	if cl == nil {
		return nil, fmt.Errorf("client object is nil, cannot fetch versions of group - %s", grp)
	}
	var (
		apiResList *metav1.APIGroupList
		err        error
		apiVerList []string
	)
	apiResList, err = cl.ServerGroups()
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

func getPrefSnapshotClassVersion(serverVersion string) (prefVersion string, err error) {
	currentVersion, err := getSemverVersion(serverVersion)
	if err != nil {
		return "", err
	}

	minV1SupportedVersion, err := getSemverVersion(minServerVerForV1CrdVersion)
	if err != nil {
		return "", err
	}

	prefCRDVersion := snapshotClassVersionV1
	if currentVersion.LessThan(minV1SupportedVersion) {
		prefCRDVersion = snapshotClassVersionV1Beta1
	}

	return prefCRDVersion, nil
}

func getSemverVersion(ver string) (*semVersion.Version, error) {
	semVer, err := semVersion.NewSemver(ver)

	if err != nil {
		return semVer, err
	}

	if semVer == nil {
		return nil, fmt.Errorf("invalid semver version: [%s]", ver)
	}
	return semVer, err
}

//  clusterHasVolumeSnapshotClass checks and returns volume snapshot class if present on cluster.
func clusterHasVolumeSnapshotClass(ctx context.Context, snapshotClass string, cl client.Client) (*unstructured.Unstructured, error) {
	if cl == nil {
		return nil, fmt.Errorf("runtime client object is nil, cannot fetch snapshot class from server")
	}
	var (
		prefVersion string
		err         error
	)
	prefVersion, err = GetServerPreferredVersionForGroup(StorageSnapshotGroup, clientSet)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: prefVersion,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = cl.Get(ctx, client.ObjectKey{
		Name: snapshotClass,
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
func createDNSPodSpec(op *Run, podNameSuffix string) *corev1.Pod {
	var imagePath string
	if op.LocalRegistry != "" {
		imagePath = op.LocalRegistry
	} else {
		imagePath = GcrRegistryPath
	}
	pod := getPodTemplate(dnsUtils+podNameSuffix, podNameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            dnsContainerName,
			Image:           strings.Join([]string{imagePath, DNSUtilsImage}, "/"),
			Command:         CommandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       op.ResourceRequirements,
		},
	}

	return pod
}

func createVolumeSnapshotPVCSpec(o *Run, pvcName, uid string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: getObjectMetaTemplate(pvcName, o.Namespace, uid),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &o.StorageClass,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: o.PVCStorageRequest,
				},
			},
		},
	}

	return pvc
}

func createVolumeSnapshotPodSpec(pvcName string, op *Run, nameSuffix string) *corev1.Pod {
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = strings.Join([]string{op.LocalRegistry, "/", BusyboxImageName}, "")
	} else {
		containerImage = BusyboxImageName
	}
	pod := getPodTemplate(SourcePodNamePrefix+nameSuffix, nameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:      BusyboxContainerName,
			Image:     containerImage,
			Command:   CommandBinSh,
			Args:      ArgsTouchDataFileSleep,
			Resources: op.ResourceRequirements,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      VolMountName,
					MountPath: VolMountPath,
				},
			},
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 30,
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: execRestoreDataCheckCommand,
					},
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: VolMountName,
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
func createVolumeSnapsotSpec(name, snapshotClass, namespace, snapVer, pvcName, uid string) *unstructured.Unstructured {
	volSnap := &unstructured.Unstructured{}
	volSnap.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": snapshotClass,
			"source": map[string]string{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}
	volSnap.SetName(name)
	volSnap.SetNamespace(namespace)
	volSnap.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: snapVer,
		Kind:    internal.VolumeSnapshotKind,
	})
	volSnap.SetLabels(getPreflightResourceLabels(uid))

	return volSnap
}

// createRestorePVCSpec creates pvc for restore (unmounted pvc as well)
func createRestorePVCSpec(pvcName, dsName, uid string, o *Run) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: o.Namespace,
			Labels:    getPreflightResourceLabels(uid),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &o.StorageClass,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					"storage": o.PVCStorageRequest,
				},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				Kind:     internal.VolumeSnapshotKind,
				Name:     dsName,
				APIGroup: func() *string { str := StorageSnapshotGroup; return &str }(),
			},
		},
	}

	return pvc
}

// createRestorePodSpec creates a restore pod
func createRestorePodSpec(podName, pvcName, uid string, op *Run) *corev1.Pod {
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = strings.Join([]string{op.LocalRegistry, "/", BusyboxImageName}, "")
	} else {
		containerImage = BusyboxImageName
	}
	pod := getPodTemplate(podName, uid, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            BusyboxContainerName,
			Image:           containerImage,
			Command:         CommandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       op.ResourceRequirements,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      VolMountName,
					MountPath: VolMountPath,
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: VolMountName,
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

func getPodTemplate(name, uid string, op *Run) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: getObjectMetaTemplate(name, op.Namespace, uid),
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: op.ImagePullSecret},
			},
			ServiceAccountName: op.ServiceAccountName,
			NodeSelector:       op.PodSchedOps.NodeSelector,
			Affinity:           op.PodSchedOps.Affinity,
			Tolerations:        op.PodSchedOps.Tolerations,
		},
	}

	return pod
}

func getObjectMetaTemplate(name, namespace, uid string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    getPreflightResourceLabels(uid),
	}
}

func getPreflightResourceLabels(uid string) map[string]string {
	return map[string]string{
		LabelK8sName:         LabelK8sNameValue,
		LabelTrilioKey:       LabelTvkPreflightValue,
		LabelPreflightRunKey: uid,
		LabelK8sPartOf:       LabelK8sPartOfValue,
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
			Group:   StorageSnapshotGroup,
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
			return fmt.Errorf("volume snapshot - '%s' not readyToUse (waited 300 sec) :: %s",
				volSnap.GetName(), retErr.Error())
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

func deleteK8sResource(ctx context.Context, obj client.Object) error {
	var err error

	err = runtimeClient.Delete(ctx, obj, client.DeleteOption(client.GracePeriodSeconds(deletionGracePeriod)))
	if err != nil {
		return fmt.Errorf("problem occurred deleting %s - %s :: %s", obj.GetName(), obj.GetNamespace(), err.Error())
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		gvk = GetObjGVKFromStructuredType(obj)
	}
	updatedRes := &unstructured.Unstructured{}
	updatedRes.SetGroupVersionKind(gvk)
	err = runtimeClient.Get(ctx, client.ObjectKey{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, updatedRes)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = removeFinalizer(ctx, updatedRes)
	if err != nil {
		return err
	}

	return nil
}

// GetObjGVKFromStructuredType returns gvk for structured object kind
func GetObjGVKFromStructuredType(obj client.Object) schema.GroupVersionKind {

	switch obj.(type) {
	case *corev1.Pod:
		return corev1.SchemeGroupVersion.WithKind(internal.PodKind)

	case *corev1.PersistentVolumeClaim:
		return corev1.SchemeGroupVersion.WithKind(internal.PersistentVolumeClaimKind)
	}

	return schema.GroupVersionKind{}
}

// getDefaultRetryBackoffParams returns a backoff object with timeout of approx. 5 min
func getDefaultRetryBackoffParams() k8swait.Backoff {
	return k8swait.Backoff{
		Steps: volSnapRetrySteps, Duration: volSnapRetryInterval,
		Factor: volSnapRetryFactor, Jitter: volSnapRetryJitter,
	}
}

func logPodScheduleStmt(pod *corev1.Pod, logger *logrus.Logger) {
	logger.Debugf("Pod - '%s' scheduled on node - '%s'", pod.GetName(), pod.Spec.NodeName)
}
