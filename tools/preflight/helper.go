package preflight

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
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
	"k8s.io/apimachinery/pkg/types"
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
	"sigs.k8s.io/yaml"
)

const (
	windowsOSTarget    = "windows"
	windowsCheckSymbol = "\u2713"
	windowsCrossSymbol = "[X]"

	versionRegexpCompile = "v\\d+.\\d+.\\d+"
	minHelmVersion       = "3.0.0"
	minK8sVersion        = "1.19.0"

	RBACAPIGroup   = "rbac.authorization.k8s.io"
	RBACAPIVersion = "v1"

	letterBytes = "abcdefghijklmnopqrstuvwxyz"

	LabelK8sPartOf         = "app.kubernetes.io/part-of"
	LabelK8sPartOfValue    = "k8s-triliovault"
	LabelTrilioKey         = "trilio"
	LabelTvkPreflightValue = "tvk-preflight"
	LabelPreflightRunKey   = "preflight-run"
	LabelK8sName           = "app.kubernetes.io/name"
	LabelK8sNameValue      = "k8s-triliovault"

	TestNamespacePrefix                = "preflight-test-ns-"
	BackupNamespacePrefix              = "preflight-backup-ns-"
	RestoreNamespacePrefix             = "preflight-restore-ns-"
	SourcePodNamePrefix                = "source-pod-"
	SourcePvcNamePrefix         string = "source-pvc-"
	BackupPvcNamePrefix                = "backup-pvc-"
	RestorePvcNamePrefix               = "restored-pvc-"
	VolumeSnapSrcNamePrefix            = "snapshot-source-pvc-"
	VolumeSnapBackupNamePrefix         = "snapshot-backup-pvc-"
	VolumeSnapRestoreNamePrefix        = "snapshot-restore-pvc-"

	StorageSnapshotGroup             = "snapshot.storage.k8s.io"
	RestorePodNamePrefix             = "restored-pod-"
	BusyboxContainerName             = "busybox"
	BusyBoxRegistry                  = "quay.io/triliodata"
	BusyboxImageName                 = "busybox"
	UnmountedRestorePodNamePrefix    = "unmounted-restored-pod-"
	UnmountedRestorePvcNamePrefix    = "unmounted-restored-pvc-"
	UnmountedVolumeSnapSrcNamePrefix = "unmounted-source-pvc-"

	dnsUtils         = "dnsutils-"
	dnsContainerName = "dnsutils"
	GcrRegistryPath  = "gcr.io/kubernetes-e2e-test-images"
	DNSUtilsImage    = "dnsutils:1.3"

	defaultRetrySteps    = 120
	defaultRetryInterval = 5 * time.Second
	defaultRetryFactor   = 1.0
	defaultRetryJitter   = 0.1

	VolMountName = "source-data"
	VolMountPath = "/demo/data"

	execTimeoutDuration       = 3 * time.Minute
	deletionGracePeriod int64 = 5

	volumeSnapshotCRDYamlDir         = "volumesnapshotcrdyamls"
	snapshotClassVersionV1           = "v1"
	snapshotClassVersionV1Beta1      = "v1beta1"
	SnapshotClassIsDefaultAnnotation = "snapshot.storage.kubernetes.io/is-default-class"
	minServerVerForV1CrdVersion      = "v1.20.0"
	defaultVSCNamePrefix             = "preflight-generated-snapshot-class-"

	podCapability = "pod-capability-"
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
	execDataCheckCommand = []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("dat=$(cat %q); echo \"${dat}\"; if [[ \"${dat}\" == %q ]]; then exit 0; else exit 1; fi",
			VolSnapPodFilePath, VolSnapPodFileData),
	}

	execDNSResolutionCmd = []string{"nslookup", "kubernetes.default"}

	kubectlBinaryName = "kubectl"
	HelmBinaryName    = "helm"

	kubeClient ServerClients

	//go:embed volumesnapshotcrdyamls/*
	crdYamlFiles embed.FS
)

type ServerClients struct {
	ClientSet     *goclient.Clientset
	RuntimeClient client.Client
	DiscClient    *discovery.DiscoveryClient
	RestConfig    *rest.Config
}

type CommonOptions struct {
	Kubeconfig                 string `yaml:"kubeconfig,omitempty"`
	Namespace                  string `yaml:"namespace,omitempty"`
	LogLevel                   string `yaml:"logLevel,omitempty"`
	InCluster                  bool   `yaml:"inCluster,omitempty"`
	VolSnapshotValidationScope string `yaml:"volSnapshotValidationScope,omitempty"`
	Logger                     *logrus.Logger
}

type capability struct {
	userID                   int64
	allowPrivilegeEscalation bool
	privileged               bool
}

func (co *CommonOptions) logCommonOptions() {
	co.Logger.Infof("LOG-LEVEL=\"%s\"", co.LogLevel)
	co.Logger.Infof("KUBECONFIG-PATH=\"%s\"", co.Kubeconfig)
	co.Logger.Infof("NAMESPACE=\"%s\"", co.Namespace)
	co.Logger.Infof("INCLUSTER=\"%t\"", co.InCluster)
	co.Logger.Infof("VOLUME-SNAPSHOT-VALIDATION-SCOPE=\"%s\"", co.VolSnapshotValidationScope)
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
	kubeClient.ClientSet = kubeEnv.GetClientset()
	if kubeClient.ClientSet == nil {
		return fmt.Errorf("client-set object initialized to nil, cannot perform CRUD operation for preflight resources")
	}
	kubeClient.RuntimeClient = kubeEnv.GetRuntimeClient()
	if kubeClient.RuntimeClient == nil {
		return fmt.Errorf("runtime-client object initialized to nil, cannot perform CRUD operation for preflight resources")
	}
	kubeClient.DiscClient = kubeEnv.GetDiscoveryClient()
	kubeClient.RestConfig = kubeEnv.GetRestConfig()

	return nil
}

func GetHelmVersion(binaryName string) (string, error) {
	var (
		helmVersion string
		cmdOut      *shell.CmdOut
		err         error
	)
	cmdOut, err = shell.RunCmd(fmt.Sprintf("%s version "+
		"--template '{{.Version}}'", binaryName))
	if err != nil {
		return "", err
	}

	helmVersion, err = extractVersionFromString(cmdOut.Out)
	if err != nil {
		return "", err
	}

	return helmVersion, nil
}

// extractVersionFromString extracts version satisfying the regex compile rule
func extractVersionFromString(str string) (string, error) {
	verexp := regexp.MustCompile(versionRegexpCompile)
	matches := verexp.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no version of type vX.Y.Z found in the string")
	}
	// pick the last match if there are multiple versions mentioned in the output.
	// because warnings and errors are printed before the actual version output in most cases.
	version := matches[len(matches)-1][0]
	version = version[1:]

	return version, nil
}

func GetServerPreferredVersionForGroup(grp string, cl *goclient.Clientset) (string, error) {
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

// clusterHasVolumeSnapshotClass checks and returns volume snapshot class if present on cluster.
func clusterHasVolumeSnapshotClass(ctx context.Context, snapshotClass string,
	kubeClient *goclient.Clientset, runtClient client.Client) (*unstructured.Unstructured, error) {
	var (
		prefVersion string
		err         error
	)
	prefVersion, err = GetServerPreferredVersionForGroup(StorageSnapshotGroup, kubeClient)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: prefVersion,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = runtClient.Get(ctx, client.ObjectKey{
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

// createDNSPodSpec returns a corev1.Pod instance.
func createDNSPodSpec(op *Run, podNameSuffix string) *corev1.Pod {
	var imagePath string
	if op.LocalRegistry != "" {
		imagePath = op.LocalRegistry
	} else {
		imagePath = GcrRegistryPath
	}
	nsName := types.NamespacedName{
		Name:      dnsUtils + podNameSuffix,
		Namespace: op.Namespace,
	}
	pod := getPodTemplate(nsName, podNameSuffix, op)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            dnsContainerName,
			Image:           imagePath + "/" + DNSUtilsImage,
			Command:         CommandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       op.ResourceRequirements,
		},
	}

	return pod
}

func createVolumeSnapshotPVCSpec(o *Run, pvcNsName types.NamespacedName, uid string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: getObjectMetaTemplate(pvcNsName.Name, pvcNsName.Namespace, uid),
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

func createVolumeSnapshotPodSpec(pvcNsName types.NamespacedName, op *Run, nameSuffix string) *corev1.Pod {
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = op.LocalRegistry + "/" + BusyboxImageName
	} else {
		containerImage = BusyBoxRegistry + "/" + BusyboxImageName
	}
	nsName := types.NamespacedName{
		Name:      SourcePodNamePrefix + nameSuffix,
		Namespace: pvcNsName.Namespace,
	}
	pod := getPodTemplate(nsName, nameSuffix, op)
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
						Command: execDataCheckCommand,
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
					ClaimName: pvcNsName.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	return pod
}

// createVolumeSnapsotSpec creates pvc for volume snapshot
func createVolumeSnapsotSpec(nsName types.NamespacedName, snapshotClass, snapVer, pvcName, uid string) *unstructured.Unstructured {
	volSnap := &unstructured.Unstructured{}

	volSnap.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": snapshotClass,
			"source": map[string]interface{}{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}
	volSnap.SetName(nsName.Name)
	volSnap.SetNamespace(nsName.Namespace)
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

// createPodForPVCSpec creates a pod which is attached to given PVC
func createPodForPVCSpec(podNameNs types.NamespacedName, pvcName, uid string, op *Run) *corev1.Pod {
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = op.LocalRegistry + "/" + BusyboxImageName
	} else {
		containerImage = BusyBoxRegistry + "/" + BusyboxImageName
	}

	pod := getPodTemplate(podNameNs, uid, op)
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

func getPodTemplate(nsName types.NamespacedName, uid string, op *Run) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: getObjectMetaTemplate(nsName.Name, nsName.Namespace, uid),
		Spec: corev1.PodSpec{
			ServiceAccountName: op.ServiceAccountName,
			NodeSelector:       op.PodSchedOps.NodeSelector,
			Affinity:           op.PodSchedOps.Affinity,
			Tolerations:        op.PodSchedOps.Tolerations,
		},
	}

	if op.ImagePullSecret != "" {
		pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: op.ImagePullSecret},
		}
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
func waitUntilVolSnapReadyToUse(volSnap *unstructured.Unstructured, snapshotVer string,
	retryBackoff k8swait.Backoff, runtimeClient client.Client) error {
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

func removeFinalizer(ctx context.Context, obj client.Object, cl client.Client) error {
	var (
		payload      []internal.PatchOperation
		payloadBytes []byte
		err          error
	)

	if obj.GetFinalizers() == nil {
		return nil
	}

	payload = []internal.PatchOperation{{
		Op:   "remove",
		Path: "/metadata/finalizers",
	}}
	payloadBytes, err = json.Marshal(payload)
	if err != nil {
		return err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		gvk = GetObjGVKFromStructuredType(obj)
	}
	updatedRes := &unstructured.Unstructured{}
	updatedRes.SetGroupVersionKind(gvk)
	// fetch the latest object
	if gErr := cl.Get(ctx, client.ObjectKey{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, updatedRes); gErr != nil {
		return gErr
	}

	if pErr := cl.Patch(ctx, updatedRes, client.RawPatch(types.JSONPatchType, payloadBytes)); pErr != nil {
		return pErr
	}

	return nil
}

func deleteK8sResource(ctx context.Context, obj client.Object, cl client.Client) error {
	if dErr := cl.Delete(ctx, obj, client.DeleteOption(client.GracePeriodSeconds(deletionGracePeriod))); dErr != nil {
		return dErr
	}

	return removeFinalizer(ctx, obj, cl)
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

// getDefaultRetryBackoffParams returns a backoff object with timeout of approx. 300s ~ 5 min
func getDefaultRetryBackoffParams() k8swait.Backoff {
	return k8swait.Backoff{
		Steps: defaultRetrySteps, Duration: defaultRetryInterval,
		Factor: defaultRetryFactor, Jitter: defaultRetryJitter,
	}
}

func logPodScheduleStmt(pod *corev1.Pod, logger *logrus.Logger) {
	logger.Debugf("Pod - '%s' scheduled on node - '%s'", pod.GetName(), pod.Spec.NodeName)
}

// function to convert Object to YAML
func objToYAML(o interface{}) ([]byte, error) {
	objYAML, yErr := yaml.Marshal(o)
	if yErr != nil {
		return nil, yErr
	}
	return objYAML, nil
}

func createPodSpecWithCapability(op *Run, podName string, capability capability) *corev1.Pod {
	var containerImage string
	if op.LocalRegistry != "" {
		containerImage = op.LocalRegistry + "/" + BusyboxImageName
	} else {
		containerImage = BusyBoxRegistry + "/" + BusyboxImageName
	}
	nsName := types.NamespacedName{
		Name:      podCapability + podName,
		Namespace: op.Namespace,
	}
	pod := getPodTemplate(nsName, podName, op)
	readOnlyRootFSFlag := false
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            BusyboxContainerName,
			Image:           containerImage,
			Command:         CommandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       op.ResourceRequirements,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{
						"KILL",
						"AUDIT_WRITE",
						"NET_BIND_SERVICE",
						"CHOWN",
						"FOWNER",
						"DAC_OVERRIDE",
						"SETGID",
						"SETUID",
						"SYS_ADMIN",
					},
				},
				AllowPrivilegeEscalation: &capability.allowPrivilegeEscalation,
				ReadOnlyRootFilesystem:   &readOnlyRootFSFlag,
				Privileged:               &capability.privileged,
			},
		},
	}
	if capability.userID == 0 {
		runAsNonRootFlag := false
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: &runAsNonRootFlag,
		}
	} else {
		runAsNonRootFlag := true
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: &runAsNonRootFlag,
			RunAsUser:    &capability.userID,
		}
	}
	return pod
}
