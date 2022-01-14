package preflight

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	goexec "os/exec"
	"time"

	version "github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight/exec"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"
	"k8s.io/client-go/discovery"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Options input options required for running preflight.
type Options struct {
	CommonOptions
	StorageClass         string
	SnapshotClass        string
	LocalRegistry        string
	ImagePullSecret      string
	ServiceAccountName   string
	PerformCleanupOnFail bool
}

// createResourceNameSuffix creates a unique 6-length hash for preflight check.
// All resources name created during preflight will have hash as suffix
func createResourceNameSuffix() (string, error) {
	suffix := make([]byte, 6)
	randRange := big.NewInt(int64(len(letterBytes)))
	for i := range suffix {
		randNum, err := rand.Int(rand.Reader, randRange)
		if err != nil {
			return "", err
		}
		idx := randNum.Int64()
		suffix[i] = letterBytes[idx]
	}

	return string(suffix), nil
}

// PerformPreflightChecks performs all preflight checks.
func (o *Options) PerformPreflightChecks(ctx context.Context) {
	var err error
	preflightStatus := true
	resNameSuffix, err = createResourceNameSuffix()
	if err != nil {
		o.Logger.Errorf("Error generating resource name suffix :: %s", err.Error())
		return
	}
	storageSnapshotSuccess := true

	o.Logger.Infof("Generated UID for preflight check - %s\n", resNameSuffix)

	//  check kubectl
	o.Logger.Infoln("Checking for kubectl")
	err = o.checkKubectl()
	if err != nil {
		o.Logger.Errorf("%s Preflight check for kubectl utility failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubectl utility is successful\n", check)
	}

	o.Logger.Infoln("Checking access to the default namespace of cluster")
	err = o.checkClusterAccess(ctx)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for cluster access failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubectl access is successful\n", check)
	}

	o.Logger.Infof("Checking for required Helm version (>= %s)\n", minHelmVersion)
	err = o.checkHelmVersion()
	if err != nil {
		o.Logger.Errorf("%s Preflight check for helm version failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for helm version is successful\n", check)
	}

	o.Logger.Infof("Checking for required kubernetes server version (>=%s)\n", minK8sVersion)
	err = o.checkKubernetesVersion()
	if err != nil {
		o.Logger.Errorf("%s Preflight check for kubernetes version failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubernetes version is successful\n", check)
	}

	o.Logger.Infoln("Checking Kubernetes RBAC")
	err = o.checkKubernetesRBAC()
	if err != nil {
		o.Logger.Errorf("%s Preflight check for kubernetes RBAC failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubernetes RBAC is successful\n", check)
	}

	//  Check storage snapshot class
	o.Logger.Infoln("Checking if a StorageClass and VolumeSnapshotClass are present")
	err = o.checkStorageSnapshotClass(ctx)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for SnapshotClass failed :: %s\n", cross, err.Error())
		storageSnapshotSuccess = false
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for SnapshotClass is successful\n", check)
	}

	//  Check CSI installation
	o.Logger.Infoln("Checking if CSI APIs are installed in the cluster")
	err = o.checkCSI(ctx)
	if err != nil {
		o.Logger.Errorf("Preflight check for CSI failed :: %s\n", err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for CSI is successful\n", check)
	}

	//  Check DNS resolution
	o.Logger.Infoln("Checking if DNS resolution is working in k8s cluster")
	err = o.checkDNSResolution(ctx)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for DNS resolution failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for DNS resolution is successful\n", check)
	}

	//  Check volume snapshot and restore
	if storageSnapshotSuccess {
		o.Logger.Infoln("Checking if volume snapshot and restore is enabled in cluster")
		err = o.checkVolumeSnapshot(ctx)
		if err != nil {
			o.Logger.Errorf("%s Preflight check for volume snapshot and restore failed :: %s\n", cross, err.Error())
			preflightStatus = false
		} else {
			o.Logger.Infof("%s Preflight check for volume snapshot and restore is successful\n", check)
		}
	} else {
		o.Logger.Errorf("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed")
	}

	co := &CleanupOptions{
		CommonOptions: CommonOptions{
			Kubeconfig: o.Kubeconfig,
			Namespace:  o.Namespace,
			Logger:     o.Logger,
		},
	}
	if !preflightStatus {
		o.Logger.Warnln("Some preflight checks failed")
	} else {
		o.Logger.Infoln("All preflight checks succeeded!")
	}
	if preflightStatus || o.PerformCleanupOnFail {
		err = co.CleanupPreflightResources(ctx, resNameSuffix)
		if err != nil {
			o.Logger.Errorf("%s Failed to cleanup preflight resources :: %s\n", cross, err.Error())
		}
	}
}

// checkKubectl checks whether kubectl utility is installed.
func (o *Options) checkKubectl() error {
	path, err := goexec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("error finding 'kubectl' binary in $PATH of the system :: %s", err.Error())
	}
	o.Logger.Infof("kubectl found at path - %s\n", path)

	return nil
}

// checkClusterAccess Checks whether access to kubectl utility is present on the client machine.
func (o *Options) checkClusterAccess(ctx context.Context) error {
	_, err := clientSet.CoreV1().Namespaces().Get(ctx, internal.DefaultNs, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to access default namespace of cluster :: %s", err.Error())
	}

	return nil
}

// checkHelmVersion checks whether minimum helm version is present.
func (o *Options) checkHelmVersion() error {
	if internal.CheckIsOpenshift(discClient, ocpAPIVersion) {
		o.Logger.Infof("%s Running OCP cluster. Helm not needed for OCP clusters\n", check)
		return nil
	}
	o.Logger.Infof("APIVersion - %s not found on cluster, not an OCP cluster\n", ocpAPIVersion)
	// check whether helm exists
	path, err := goexec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("error finding 'helm' binary in $PATH of the system :: %s", err.Error())
	}
	o.Logger.Infof("helm found at path - %s\n", path)

	// check minimum version of helm
	cmdOut, err := shell.RunCmd("helm version --template '{{.Version}}'")
	if err != nil {
		return err
	}
	helmVersion := cmdOut.Out[2 : len(cmdOut.Out)-1]
	v1, err := version.NewVersion(minHelmVersion)
	if err != nil {
		return err
	}
	v2, err := version.NewVersion(helmVersion)
	if err != nil {
		return err
	}
	if v2.LessThan(v1) {
		return fmt.Errorf("helm does not meet minimum version requirement.\nUpgrade helm to minimum version - %s", minHelmVersion)
	}

	o.Logger.Infof("%s Helm version %s meets required version\n", check, helmVersion)

	return nil
}

// checkKubernetesVersion checks whether minimum k8s version requirement is met
func (o *Options) checkKubernetesVersion() error {
	serverVer, err := clientSet.ServerVersion()
	if err != nil {
		return err
	}

	v1, err := version.NewVersion(minK8sVersion)
	if err != nil {
		return err
	}
	v2, err := version.NewVersion(serverVer.GitVersion)
	if err != nil {
		return err
	}
	if v2.LessThan(v1) {
		return fmt.Errorf("kubernetes server version does not meet minimum requirements")
	}

	return nil
}

// checkKubernetesRBAC fetches the apiVersions present on k8s server.
// And checks whether api-version 'rbac.authorization.k8s.io' is present.
// 'ExtractGroupVersions' func call is taken from kubcetl mirror repo.
func (o *Options) checkKubernetesRBAC() error {
	groupList, err := discClient.ServerGroups()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			o.Logger.Errorf("Unable to fetch groups from server :: %s\n", err.Error())
			return err
		}
		o.Logger.Warnf("The Kubernetes server has an orphaned API service. Server reports: %s\n", err.Error())
		o.Logger.Warnln("To fix this, kubectl delete api service <service-name>")
	}
	apiVersions := metav1.ExtractGroupVersions(groupList)
	found := false
	for _, apiver := range apiVersions {
		gv, err := schema.ParseGroupVersion(apiver)
		if err != nil {
			return nil
		}
		if gv.Group == rbacAPIGroup && gv.Version == rbacAPIVersion {
			found = true
			o.Logger.Infof("%s Kubernetes RBAC is enabled\n", check)
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
func (o *Options) checkStorageSnapshotClass(ctx context.Context) error {
	sc, err := clientSet.StorageV1().StorageClasses().Get(ctx, o.StorageClass, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("not found storageclass - %s on cluster", o.StorageClass)
		}
		return err
	}
	o.Logger.Infof("%s Storageclass - %s found on cluster\n", check, o.StorageClass)
	provisioner := sc.Provisioner
	if o.SnapshotClass == "" {
		storageVolSnapClass, err = o.checkSnapshotclassForProvisioner(ctx, provisioner)
		if err != nil {
			o.Logger.Errorf("%s %s\n", cross, err.Error())
			return err
		}
		o.Logger.Infof("%s Extracted volume snapshot class - %s found in cluster", check, storageVolSnapClass)
		o.Logger.Infof("%s Volume snapshot class - %s driver matches with given StorageClass's provisioner=%s\n",
			check, storageVolSnapClass, provisioner)
	} else {
		storageVolSnapClass = o.SnapshotClass
		vssc, err := clusterHasVolumeSnapshotClass(ctx, o.SnapshotClass, o.Namespace)
		if err != nil {
			o.Logger.Errorf("%s %s\n", cross, err.Error())
			return err
		}
		if vssc.Object["driver"] == provisioner {
			o.Logger.Infof("%s Volume snapshot class - %s driver matches with given StorageClass\n", check, vssc.Object["driver"])
		} else {
			return fmt.Errorf("volume snapshot class - %s "+
				"driver does not match with given StorageClass's provisioner=%s", o.SnapshotClass, provisioner)
		}
	}

	return nil
}

//  checkSnapshotclassForProvisioner checks whether snapshot-class exist for a provisioner
func (o *Options) checkSnapshotclassForProvisioner(ctx context.Context, provisioner string) (string, error) {
	var (
		prefVersion string
		err         error
	)
	prefVersion, err = getServerPreferredVersionForGroup(storageSnapshotGroup)
	if err != nil {
		return "", err
	}

	vsscList := unstructured.UnstructuredList{}
	vsscList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: prefVersion,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = runtimeClient.List(ctx, &vsscList)
	if err != nil {
		return "", err
	} else if len(vsscList.Items) == 0 {
		return "", fmt.Errorf("no volume snapshot class for APIVersion - %s/%s found on cluster",
			storageSnapshotGroup, prefVersion)
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

	o.Logger.Infof("volume snapshot class having driver "+
		"same as provisioner - %s found on cluster for version - %s", provisioner, prefVersion)
	return sscName, nil
}

//  checkCSI checks whether CSI APIs are installed in the k8s cluster
func (o *Options) checkCSI(ctx context.Context) error {
	prefVersion, err := getServerPreferredVersionForGroup(apiExtenstionsGroup)
	if err != nil {
		return err
	}
	var apiFoundCnt = 0
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   apiExtenstionsGroup,
		Version: prefVersion,
		Kind:    customResourceDefinition,
	})
	for _, api := range csiApis {
		err := runtimeClient.Get(ctx, client.ObjectKey{Name: api}, u)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		} else if k8serrors.IsNotFound(err) {
			o.Logger.Errorf("%s Not found CSI API - %s\n", cross, api)
		} else {
			o.Logger.Infof("%s Found CSI API - %s on cluster\n", check, api)
			apiFoundCnt++
		}
	}

	if apiFoundCnt != len(csiApis) {
		return fmt.Errorf("some CSI APIs not found in cluster. Check logs for details")
	}
	return nil
}

//  checkDNSResolution checks whether DNS resolution is working on k8s cluster
func (o *Options) checkDNSResolution(ctx context.Context) error {
	pod := createDNSPodSpec(o)
	_, err := clientSet.CoreV1().Pods(o.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("%s %s\n", cross, err.Error())
		return err
	}
	o.Logger.Infof("Pod %s created in cluster\n", pod.GetName())

	waitOptions := &wait.PodWaitOptions{
		Name:         pod.GetName(),
		Namespace:    o.Namespace,
		Timeout:      3 * time.Minute,
		PodCondition: corev1.PodReady,
		ClientSet:    clientSet,
	}
	o.Logger.Infoln("Waiting for dns pod to become ready")
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		o.Logger.Errorf("DNS pod - %s hasn't reached into ready state", pod.GetName())
		return err
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
func (o *Options) checkVolumeSnapshot(ctx context.Context) error {
	var (
		execOp      exec.Options
		waitOptions *wait.PodWaitOptions
		err         error
	)

	pvc := createVolumeSnapshotPVCSpec(o.StorageClass, o.Namespace)
	pvc, err = clientSet.CoreV1().PersistentVolumeClaims(o.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	o.Logger.Infof("Created source pvc - %s", pvc.GetName())
	srcPod := createVolumeSnapshotPodSpec(pvc.GetName(), o)
	srcPod, err = clientSet.CoreV1().Pods(o.Namespace).Create(ctx, srcPod, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	o.Logger.Infof("Created source pod - %s", srcPod.GetName())

	//  Wait for snapshot pod to become ready.
	waitOptions = &wait.PodWaitOptions{
		Name:         srcPod.GetName(),
		Namespace:    o.Namespace,
		Timeout:      3 * time.Minute,
		PodCondition: corev1.PodReady,
		ClientSet:    clientSet,
	}
	o.Logger.Infof("Waiting for source pod - %s to become ready\n", srcPod.GetName())
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		return fmt.Errorf("pod %s hasn't reached into ready state", srcPod.GetName())
	}
	o.Logger.Infof("Source pod - %s has reached into ready state\n", srcPod.GetName())

	execOp = exec.Options{
		Namespace:     o.Namespace,
		Command:       execRestoreDataCheckCommand,
		PodName:       srcPod.GetName(),
		ContainerName: busyboxContainerName,
		Executor:      &exec.DefaultRemoteExecutor{},
		Config:        restConfig,
		ClientSet:     clientSet,
	}
	o.Logger.Infof("Checking for file - '%s' in source pod - '%s'\n", volSnapPodFilePath, srcPod.GetName())
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}

	//  Create volume snapshot
	snapshotVer, err := getServerPreferredVersionForGroup(storageSnapshotGroup)
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	volSnap := createVolumeSnapsotSpec(volumeSnapSrc+resNameSuffix, o.Namespace, snapshotVer, pvc.GetName())
	if err = runtimeClient.Create(ctx, volSnap); err != nil {
		o.Logger.Errorf("%s Error creating volume snapshot from source pvc :: %s\n", cross, err.Error())
		return err
	}
	o.Logger.Infof("Created volume snapshot - %s from source pvc", volSnap.GetName())

	o.Logger.Infof("Waiting for volume snapshot - %s from source pvc to become 'readyToUse:true'", volSnap.GetName())
	err = waitUntilVolSnapReadyToUse(volSnap, snapshotVer, getDefaultRetryBackoffParams())
	if err != nil {
		return err
	}
	o.Logger.Infof("%s volume snapshot - %s is ready-to-use\n", check, volSnap.GetName())

	//  Create restore pod and pvc
	restorePvcSpec := createRestorePVCSpec(restorePvc+resNameSuffix, volumeSnapSrc+resNameSuffix, o.StorageClass, o.Namespace)
	restorePvcSpec, err = clientSet.CoreV1().PersistentVolumeClaims(o.Namespace).
		Create(ctx, restorePvcSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	o.Logger.Infof("Created restore pvc - %s from volume snapshot - %s\n", restorePvcSpec.GetName(), volSnap.GetName())
	restorePodSpec := createRestorePodSpec(restorePod+resNameSuffix, restorePvcSpec.GetName(), o)
	restorePodSpec, err = clientSet.CoreV1().Pods(o.Namespace).
		Create(ctx, restorePodSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	o.Logger.Infof("Created restore pod - %s\n", restorePodSpec.GetName())

	//  Wait for snapshot pod to become ready.
	o.Logger.Infof("Waiting for restore pod - %s to become ready\n", restorePodSpec.GetName())
	waitOptions.Name = restorePodSpec.GetName()
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		return err
	}
	o.Logger.Infof("%s Restore pod - %s has reached into ready state\n", check, restorePodSpec.GetName())

	execOp = exec.Options{
		Namespace:     o.Namespace,
		Command:       execRestoreDataCheckCommand,
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
	o.Logger.Infof("Restored pod - %s has expected data\n", restorePodSpec.GetName())

	o.Logger.Infof("Deleting source pod - %s\n", srcPod.GetName())
	err = deleteK8sResourceWithForceTimeout(ctx, srcPod, o.Logger)
	if err != nil {
		return err
	}

	unmountedVolSnapSrcSpec := createVolumeSnapsotSpec(unmountedVolumeSnapSrc+resNameSuffix, o.Namespace, snapshotVer, pvc.GetName())
	if err = runtimeClient.Create(ctx, unmountedVolSnapSrcSpec); err != nil {
		o.Logger.Errorf("%s error creating volume snapshot from unmounted source pvc :: %s\n", cross, err.Error())
		return err
	}
	o.Logger.Infof("Created volume snapshot - %s\n", unmountedVolSnapSrcSpec.GetName())
	o.Logger.Infof("Waiting for volume snapshot - %s from unmounted source pvc to become 'readyToUse:true'\n",
		unmountedVolSnapSrcSpec.GetName())
	err = waitUntilVolSnapReadyToUse(unmountedVolSnapSrcSpec, snapshotVer, getDefaultRetryBackoffParams())
	if err != nil {
		return err
	}
	o.Logger.Infof("%s Volume snapshot - %s from unmounted source pvc and is ready-to-use\n",
		check, unmountedVolSnapSrcSpec.GetName())

	// create unmounted restore pvc and pod
	unmountedPvcSpec := createRestorePVCSpec(unmountedRestorePvc+resNameSuffix,
		unmountedVolumeSnapSrc+resNameSuffix,
		o.StorageClass, o.Namespace)
	_, err = clientSet.CoreV1().PersistentVolumeClaims(o.Namespace).
		Create(ctx, unmountedPvcSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	o.Logger.Infof("Created restore pvc - %s from unmounted volume snapshot - %s\n",
		unmountedPvcSpec.GetName(), unmountedVolSnapSrcSpec.GetName())
	unmountedPodSpec := createRestorePodSpec(unmountedRestorePod+resNameSuffix, unmountedPvcSpec.GetName(), o)
	unmountedPodSpec, err = clientSet.CoreV1().Pods(o.Namespace).Create(ctx, unmountedPodSpec, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		return err
	}
	o.Logger.Infof("Created restore pod - %s from volume snapshot of unmounted pv\n", unmountedPodSpec.GetName())
	// wait for unmounted restore pod to become ready
	waitOptions.Name = unmountedPodSpec.GetName()
	o.Logger.Infof("Waiting for unmounted restore pod - %s to become ready\n", unmountedPodSpec.GetName())
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		return err
	}
	o.Logger.Infof("%s Restore pod - %s has reached into ready state\n", check, unmountedPodSpec.GetName())

	execOp.PodName = unmountedPodSpec.GetName()
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}
	o.Logger.Infof("%s restored pod from volume snapshot of unmounted pv has expected data\n", check)

	return nil
}
