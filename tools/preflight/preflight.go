package preflight

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	goexec "os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight/exec"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Options input options required for running preflight.
type Options struct {
	Context              context.Context
	StorageClass         string
	SnapshotClass        string
	Kubeconfig           string
	Namespace            string
	LocalRegistry        string
	ImagePullSecret      string
	ServiceAccountName   string
	PerformCleanupOnFail bool
	Logger               *logrus.Logger
}

func InitKubeEnv(kubeconfig string) error {
	utilruntime.Must(corev1.AddToScheme(scheme))
	kubeEnv, err := internal.NewEnv(kubeconfig, scheme)
	clientSet = kubeEnv.GetClientset()
	runtimeClient = kubeEnv.GetRuntimeClient()
	discClient = kubeEnv.GetDiscoveryClient()
	restConfig = kubeEnv.GetRestConfig()
	return err
}

// createResourceNameSuffix creates a unique 6-length hash for preflight check.
// All resources name created during preflight will have hash as suffix
func createResourceNameSuffix() string {
	suffix := make([]byte, 6)
	randRange := big.NewInt(int64(len(letterBytes)))
	for i := range suffix {
		randNum, _ := rand.Int(rand.Reader, randRange)
		idx := randNum.Int64()
		suffix[i] = letterBytes[idx]
	}

	return string(suffix)
}

// PerformPreflightChecks performs all preflight checks.
func (o *Options) PerformPreflightChecks() {
	preflightStatus := true
	resNameSuffix = createResourceNameSuffix()
	storageSnapshotSuccess := true
	var err error

	o.Logger.Infoln(fmt.Sprintf("Generated UID for preflight check - %s", resNameSuffix))

	//  check kubectl
	o.Logger.Infoln("Checking for kubectl")
	err = o.checkKubectl()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for kubectl utility failed :: %s", cross, err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for kubectl utility is successful", check))
	}

	o.Logger.Infoln("Checking access to the cluster")
	err = o.checkClusterAccess()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for kubectl access failed :: %s", cross, err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for kubectl access is successful", check))
	}

	o.Logger.Infoln(fmt.Sprintf("Checking for required Helm version (>= %s)", minHelmVersion))
	err = o.checkHelmVersion()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for helm version failed :: %s", cross, err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for helm version is successful", check))
	}

	o.Logger.Infoln(fmt.Sprintf("Checking for required kubernetes server version (>=%s)", minK8sVersion))
	err = o.checkKubernetesVersion()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for kubernetes version failed :: %s", cross, err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for kubernetes version is successful", check))
	}

	o.Logger.Infoln("Checking Kubernetes RBAC")
	err = o.checkKubernetesRBAC()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for kubernetes RBAC failed :: %s", cross, err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for kubernetes RBAC is successful", check))
	}

	//  Check storage snapshot class
	o.Logger.Infoln("Checking if a StorageClass and VolumeSnapshotClass are present")
	err = o.checkStorageSnapshotClass()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for SnapshotClass failed :: %s", cross, err.Error()))
		storageSnapshotSuccess = false
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for SnapshotClass is successful", check))
	}

	//  Check CSI installation
	o.Logger.Infoln("Checking if CSI APIs are installed in the cluster")
	err = o.checkCSI()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("Preflight check for CSI failed :: %s", err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for CSI is successful", check))
	}

	//  Check DNS resolution
	o.Logger.Infoln("Checking if DNS resolution is working in k8s cluster")
	err = o.checkDNSResolution()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Preflight check for DNS resolution failed :: %s", cross, err.Error()))
		preflightStatus = false
	} else {
		o.Logger.Infoln(fmt.Sprintf("%s Preflight check for DNS resolution is successful", check))
	}

	//  Check volume snapshot and restore
	if storageSnapshotSuccess {
		o.Logger.Infoln("Checking if volume snapshot and restore is enabled in cluster")
		err = o.checkVolumeSnapshot()
		if err != nil {
			o.Logger.Errorln(fmt.Sprintf("%s Preflight check for volume snapshot and restore failed :: %s", cross, err.Error()))
			preflightStatus = false
		} else {
			o.Logger.Infoln(fmt.Sprintf("%s Preflight check for volume snapshot and restore is successful", check))
		}
	}

	co := &CleanupOptions{
		Ctx:        o.Context,
		Kubeconfig: o.Kubeconfig,
		Namespace:  o.Namespace,
		Logger:     o.Logger,
	}
	if !preflightStatus {
		o.Logger.Warnln("Some preflight checks failed")
		if o.PerformCleanupOnFail {
			if err = co.CleanupByUID(resNameSuffix); err != nil {
				o.Logger.Errorln(fmt.Sprintf("%s Failed to cleanup preflight resources :: %s", cross, err.Error()))
			}
		}
	} else {
		o.Logger.Infoln("All preflight checks succeeded!")
		if err = co.CleanupByUID(resNameSuffix); err != nil {
			o.Logger.Errorln(fmt.Sprintf("%s Failed to cleanup preflight resources :: %s", cross, err.Error()))
		}
	}
}

// checkKubectl checks whether kubectl utility is installed.
func (o *Options) checkKubectl() error {
	path, err := goexec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("error finding 'kubectl' binary in $PATH of the system :: %s", err.Error())
	}
	o.Logger.Infoln(fmt.Sprintf("kubectl found at path - %s", path))

	return nil
}

// checkClusterAccess Checks whether access to kubectl utility is present on the client machine.
func (o *Options) checkClusterAccess() error {
	cmdOut, err := shell.RunCmd(fmt.Sprintf("kubectl cluster-info --kubeconfig=%s", o.Kubeconfig))
	if err != nil {
		return err
	} else if cmdOut.ExitCode != 0 {
		return fmt.Errorf("unable to access kubernetes context :: %s", err.Error())
	}
	o.Logger.Infoln(fmt.Sprintf("%s Able to access Kubernetes cluster", check))

	return nil
}

// checkHelmVersion checks whether minimum helm version is present.
func (o *Options) checkHelmVersion() error {
	if isOCPk8sCluster(o.Logger) {
		o.Logger.Infoln(fmt.Sprintf("%s Running OCP cluster. Helm not needed for OCP clusters", check))
		return nil
	}
	// check whether helm exists
	path, err := goexec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("error finding 'helm' binary in $PATH of the system :: %s", err.Error())
	}
	o.Logger.Infoln(fmt.Sprintf("helm found at path - %s", path))

	// check minimum version of elm
	cmdOut, err := shell.RunCmd("helm version --template '{{.Version}}'")
	if err != nil {
		return err
	}
	helmVersion := cmdOut.Out[1:]
	if helmVersion < minHelmVersion {
		return fmt.Errorf("helm does not meet minimum version requirement.\nUpgrade helm to minimum version - %s", minHelmVersion)
	}
	o.Logger.Infoln(fmt.Sprintf("%s Helm version %s meets required version", check, helmVersion))

	return nil
}

// checkKubernetesVersion checks whether minimum k8s version requirement is met
func (o *Options) checkKubernetesVersion() error {
	serverVer, err := clientSet.ServerVersion()
	if err != nil {
		return err
	}
	major, minor := serverVer.Major, serverVer.Minor
	rgx := regexp.MustCompile(`[^0-9.]`)
	minor = rgx.ReplaceAllString(minor, "")
	minVer := strings.Split(minK8sVersion, ".")
	minMajor, minMinor := minVer[0], minVer[1]
	if (major > minMajor) || (major == minMajor && minor >= minMinor) {
		return nil
	}

	return fmt.Errorf("kubernetes server version does not meet minimum requirements")
}

// checkKubernetesRBAC fetches the apiVersions present on k8s server.
// And checks whether api-version 'rbac.authorization.k8s.io' is present.
// 'ExtractGroupVersions' func call is taken from kubcetl mirror repo.
func (o *Options) checkKubernetesRBAC() error {
	groupList, err := discClient.ServerGroups()
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("Unable to fetch groups from server :: %s", err.Error()))
		return err
	}
	apiVersions := metav1.ExtractGroupVersions(groupList)
	found := false
	for _, apiver := range apiVersions {
		api := strings.Split(apiver, "/")[0] // discarding version.
		if api == rbacAPIGroup {
			found = true
			o.Logger.Infoln(fmt.Sprintf("%s Kubernetes RBAC is enabled", check))
			break
		}
	}
	if !found {
		return fmt.Errorf("not enabled kubernetes RBAC")
	}

	return nil
}

// checkStorageSnapshotClass checks whether storageclass is present.
// Checks whether storageclass and volumesnapshotclass provisioner are same.
func (o *Options) checkStorageSnapshotClass() error {
	sc, err := clientSet.StorageV1().StorageClasses().Get(o.Context, o.StorageClass, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("not found storageclass - %s on cluster", o.StorageClass)
		}
		return err
	}
	o.Logger.Infoln(fmt.Sprintf("%s Storageclass - %s found on cluster", check, o.StorageClass))
	provisioner := sc.Provisioner
	if o.SnapshotClass == "" {
		storageVolSnapClass, err = o.checkSnapshotclassForProvisioner(provisioner)
		if err != nil {
			o.Logger.Errorln(fmt.Sprintf("%s %s", cross, err.Error()))
			return err
		}
		o.Logger.Infoln(fmt.Sprintf("%s Extracted volume snapshot class - %s found in cluster", check, storageSnapshotGroup))
		o.Logger.Infoln(fmt.Sprintf("%s Volume snapshot class - %s driver matches with given StorageClass's provisioner=%s",
			check, storageVolSnapClass, provisioner))
	} else {
		vssc, err := clusterHasVolumeSnapshotClass(o.SnapshotClass)
		if err != nil {
			o.Logger.Errorln(fmt.Sprintf("%s %s", cross, err.Error()))
			return err
		}
		if vssc.Object["driver"] == provisioner {
			o.Logger.Infoln(fmt.Sprintf("%s Volume snapshot class - %s driver matches with given StorageClass ", check, vssc.Object["driver"]))
		} else {
			return fmt.Errorf("volume snapshot class - %s "+
				"driver does not match with given StorageClass's provisioner=%s", o.SnapshotClass, provisioner)
		}
	}

	return nil
}

//  checkSnapshotclassForProvisioner checks whether snapshot-class exist for a provisioner
func (o *Options) checkSnapshotclassForProvisioner(provisioner string) (string, error) {
	vsscList := unstructured.UnstructuredList{}
	vsscList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1beta1",
		Kind:    "VolumeSnapshotClass",
	})
	err := runtimeClient.List(o.Context, &vsscList)
	if err != nil {
		return "", err
	} else if len(vsscList.Items) == 0 {
		return "", fmt.Errorf("no volume snapshot class found on cluster")
	}

	sscName := ""
	for _, vssc := range vsscList.Items {
		if vssc.Object["driver"] == provisioner {
			if vssc.Object["snapshot.storage.kubernetes.io/is-default-class"] == "true" {
				return vssc.GetName(), nil
			}
			sscName = vssc.GetName()
		}
	}
	if sscName == "" {
		return "", fmt.Errorf("no matching volume snapshot class having driver "+
			"same as provisioner - %s found on cluster", provisioner)
	}

	return sscName, nil
}

//  checkCSI checks whether CSI APIs are installed in the k8s cluster
func (o *Options) checkCSI() error {
	apiExtVer, err := getVersionsOfGroup(apiExtenstionsGroup)
	if err != nil {
		return err
	}
	gvkList := make([]schema.GroupVersionKind, 0)
	for _, ver := range apiExtVer {
		gvkList = append(gvkList, schema.GroupVersionKind{
			Group:   apiExtenstionsGroup,
			Version: ver,
			Kind:    "CustomResourceDefinition",
		})
	}
	crdList := unstructured.UnstructuredList{}
	resList := make([]unstructured.Unstructured, 0)
	for _, gvk := range gvkList {
		crdList.SetGroupVersionKind(gvk)
		err := runtimeClient.List(o.Context, &crdList)
		if err != nil {
			o.Logger.Errorln(fmt.Sprintf("%s %s", cross, err.Error()))
			return err
		}
		resList = append(resList, crdList.Items...)
	}

	csiAPISet := make(map[string]bool)
	for _, api := range csiApis {
		csiAPISet[api] = true
	}

	for _, crd := range resList {
		if len(csiAPISet) == 0 {
			break
		}
		if _, ok := csiAPISet[crd.GetName()]; ok {
			delete(csiAPISet, crd.GetName())
			o.Logger.Infoln(fmt.Sprintf("Found CSI API - %s on cluster", crd.GetName()))
		}
	}
	if len(csiAPISet) == 0 {
		return nil
	}

	for api := range csiAPISet {
		o.Logger.Errorln(fmt.Sprintf("%s Not found CSI API - %s", cross, api))
	}
	return fmt.Errorf("some CSI APIs not found in cluster. Check logs for details")
}

//  checkDNSResolution checks whether DNS resolution is working on k8s cluster
func (o *Options) checkDNSResolution() error {
	var imagePath string
	if o.LocalRegistry != "" {
		imagePath = o.LocalRegistry
	} else {
		imagePath = gcrRegistryPath
	}
	pod := createDNSPodSpec(imagePath, o)
	_, err := clientSet.CoreV1().Pods(o.Namespace).Create(o.Context, pod, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorln(string(cross), err.Error())
		return err
	}
	o.Logger.Infoln(fmt.Sprintf("Pod %s created in cluster", pod.GetName()))

	defer func() {
		o.Logger.Infoln(fmt.Sprintf("Deleting dns pod - %s", pod.GetName()))
		err = deleteK8sResourceWithForceTimeout(o.Context, pod, o.Logger)
		if err != nil {
			o.Logger.Errorln(fmt.Sprintf("error occurred deleting DNS pod :: %s", err.Error()))
		} else {
			o.Logger.Infoln("DNS pod deleted successfully")
		}
	}()

	waitOptions := &wait.PodWaitOptions{
		Name:         pod.GetName(),
		Namespace:    o.Namespace,
		Timeout:      3 * time.Minute,
		PodCondition: corev1.PodReady,
		ClientSet:    clientSet,
	}
	o.Logger.Infoln("Waiting for source pod to become ready")
	err = waitUntilPodCondition(o.Context, waitOptions)
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s %s", cross, err.Error()))
	}

	op := exec.Options{
		Namespace:     o.Namespace,
		Command:       []string{"nslookup", "kubernetes.default"},
		PodName:       pod.GetName(),
		ContainerName: dnsContainerName,
		Executor:      &exec.DefaultRemoteExecutor{},
		Config:        restConfig,
		ClientSet:     clientSet,
	}
	err = execInPod(&op, o.Logger)
	if err != nil {
		return fmt.Errorf("not able to resolve DNS 'kubernetes.default' service inside pods")
	}

	return nil
}

// checkVolumeSnapshot checks if volume snapshot and restore is enabled in the cluster
func (o *Options) checkVolumeSnapshot() error {
	var (
		execOp      exec.Options
		waitOptions *wait.PodWaitOptions
		err         error
	)

	pvc := createVolumeSnapshotPVCSpec(o.StorageClass, o.Namespace)
	pvc, err = clientSet.CoreV1().PersistentVolumeClaims(o.Namespace).Create(o.Context, pvc, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	srcPod := createVolumeSnapshotPodSpec(pvc.GetName(), o)
	srcPod, err = clientSet.CoreV1().Pods(o.Namespace).Create(o.Context, srcPod, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}

	//  Wait for snapshot pod to become ready.
	waitOptions = &wait.PodWaitOptions{
		Name:         srcPod.GetName(),
		Namespace:    o.Namespace,
		Timeout:      3 * time.Minute,
		PodCondition: corev1.PodReady,
		ClientSet:    clientSet,
	}
	o.Logger.Infoln("Waiting for snapshot source pod to become ready")
	err = waitUntilPodCondition(o.Context, waitOptions)
	if err != nil {
		return fmt.Errorf("pod %s hasn't reached into ready state", srcPod.GetName())
	}
	o.Logger.Infoln("Created sources pod and pvc")

	execOp = exec.Options{
		Namespace:     o.Namespace,
		Command:       []string{"ls", "/demo/data/sample-file.txt"},
		PodName:       srcPod.GetName(),
		ContainerName: busyboxContainerName,
		Executor:      &exec.DefaultRemoteExecutor{},
		Config:        restConfig,
		ClientSet:     clientSet,
	}
	o.Logger.Infoln(fmt.Sprintf("Checking for file - '/demo/data/sample-file.txt' in source pod - '%s'", srcPod.GetName()))
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}

	//  Create volume snapshot
	snapshotVer, err := getStorageSnapshotVersion(runtimeClient)
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	volSnap := createVolumeSnapsotSpec(volumeSnapSrc+resNameSuffix, o.Namespace, snapshotVer, pvc.GetName())
	if err = runtimeClient.Create(o.Context, volSnap); err != nil {
		o.Logger.Errorln(err.Error())
		o.Logger.Errorln(fmt.Sprintf("%s Error creating volume snapshot from source pvc", cross))
		return err
	}

	o.Logger.Infoln("Waiting for Volume snapshot from source pvc to become 'readyToUse:true'")
	err = waitUntilVolSnapReadyToUse(volSnap, snapshotVer, getDefaultRetryBackoffParams())
	if err != nil {
		return err
	}
	o.Logger.Infoln(fmt.Sprintf("%s Created volume snapshot from source pvc and is ready-to-use", check))

	//  Create restore pod and pvc
	restorePvcSpec := createRestorePVCSpec(restorePvc+resNameSuffix, volumeSnapSrc+resNameSuffix, o.StorageClass, o.Namespace)
	restorePvcSpec, err = clientSet.CoreV1().PersistentVolumeClaims(o.Namespace).
		Create(o.Context, restorePvcSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	restorePodSpec := createRestorePodSpec(restorePod+resNameSuffix, restorePvcSpec.GetName(), o)
	restorePodSpec, err = clientSet.CoreV1().Pods(o.Namespace).
		Create(o.Context, restorePodSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}

	//  Wait for snapshot pod to become ready.
	o.Logger.Infoln("Waiting for restore pod to become ready")
	waitOptions.Name = restorePodSpec.GetName()
	err = waitUntilPodCondition(o.Context, waitOptions)
	if err != nil {
		return err
	}
	o.Logger.Infoln(fmt.Sprintf("%s Created restore pod from volume snapshot", check))

	execOp = exec.Options{
		Namespace:     o.Namespace,
		Command:       []string{"ls", "/demo/data/sample-file.txt"},
		PodName:       restorePodSpec.GetName(),
		ContainerName: busyboxContainerName,
		Executor:      &exec.DefaultRemoteExecutor{},
		Config:        restConfig,
		ClientSet:     clientSet,
	}
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}
	o.Logger.Infoln("restored pod has expected data")

	// TODO - add timeout for delete
	o.Logger.Infoln(fmt.Sprintf("Deleting source pod - %s", srcPod.GetName()))
	err = deleteK8sResourceWithForceTimeout(o.Context, srcPod, o.Logger)
	if err != nil {
		o.Logger.Warnln(fmt.Sprintf("problem occurred deleting source pod - %s :: %s", srcPod.GetName(), err.Error()))
	}
	o.Logger.Infoln(fmt.Sprintf("%s Deleted source pod", check))

	unmountedVolSnapSrcSpec := createVolumeSnapsotSpec(unmountedVolumeSnapSrc+resNameSuffix, o.Namespace, snapshotVer, pvc.GetName())
	if err = runtimeClient.Create(o.Context, unmountedVolSnapSrcSpec); err != nil {
		o.Logger.Errorln(err.Error())
		o.Logger.Errorln(fmt.Sprintf("%s error creating volume snapshot from unmounted source pvc", cross))
		return err
	}
	o.Logger.Infoln("Waiting for Volume snapshot from source pvc to become 'readyToUse:true'")
	err = waitUntilVolSnapReadyToUse(unmountedVolSnapSrcSpec, snapshotVer, getDefaultRetryBackoffParams())
	if err != nil {
		return err
	}
	o.Logger.Infoln(fmt.Sprintf("%s Created volume snapshot from source pvc and is ready-to-use", check))

	// create unmounted restore pvc and pod
	unmountedPvcSpec := createRestorePVCSpec(unmountedRestorePvc+resNameSuffix,
		unmountedVolumeSnapSrc+resNameSuffix,
		o.StorageClass, o.Namespace)
	_, err = clientSet.CoreV1().PersistentVolumeClaims(o.Namespace).
		Create(o.Context, unmountedPvcSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	unmountedPodSpec := createRestorePodSpec(unmountedRestorePod+resNameSuffix, unmountedPvcSpec.GetName(), o)
	unmountedPodSpec, err = clientSet.CoreV1().Pods(o.Namespace).Create(o.Context, unmountedPodSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	// wait for unmounted restore pod to become ready
	waitOptions.Name = unmountedPodSpec.GetName()
	o.Logger.Infoln("Waiting for unmounted restore pod to become ready")
	err = waitUntilPodCondition(o.Context, waitOptions)
	if err != nil {
		return err
	}
	o.Logger.Infoln("Created restore pod from volume snapshot of unmounted pv")

	execOp.PodName = unmountedPodSpec.GetName()
	execOp.ContainerName = busyboxContainerName
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}
	o.Logger.Infoln(fmt.Sprintf("%s restored pod from volume snapshot of unmounted pv has expected data", check))

	return nil
}
