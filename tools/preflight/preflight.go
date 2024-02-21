package preflight

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	goexec "os/exec"
	"path/filepath"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight/exec"
	"github.com/trilioData/tvk-plugins/tools/preflight/wait"

	version "github.com/hashicorp/go-version"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// RunOptions input options required for running preflight.
type RunOptions struct {
	StorageClass                string            `json:"storageClass"`
	SnapshotClass               string            `json:"snapshotClass,omitempty"`
	LocalRegistry               string            `json:"localRegistry,omitempty"`
	ImagePullSecret             string            `json:"imagePullSecret,omitempty"`
	ServiceAccountName          string            `json:"serviceAccount,omitempty"`
	PerformCleanupOnFail        bool              `json:"cleanupOnFailure,omitempty"`
	PVCStorageRequest           resource.Quantity `json:"pvcStorageRequest,omitempty"`
	corev1.ResourceRequirements `json:"resources,omitempty"`
	PodSchedOps                 podSchedulingOptions `json:"podSchedulingOptions"`
}

type Run struct {
	RunOptions
	CommonOptions
}

// CreateResourceNameSuffix creates a unique 6-length hash for preflight check.
// All resources name created during preflight will have hash as suffix
func CreateResourceNameSuffix() (string, error) {
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

func (o *Run) logPreflightOptions() {
	o.Logger.Infof("====PREFLIGHT RUN OPTIONS====")
	o.CommonOptions.logCommonOptions()
	o.Logger.Infof("STORAGE-CLASS=\"%s\"", o.StorageClass)
	o.Logger.Infof("VOLUME-SNAPSHOT-CLASS=\"%s\"", o.SnapshotClass)
	o.Logger.Infof("LOCAL-REGISTRY=\"%s\"", o.LocalRegistry)
	o.Logger.Infof("IMAGE-PULL-SECRET=\"%s\"", o.ImagePullSecret)
	o.Logger.Infof("SERVICE-ACCOUNT=\"%s\"", o.ServiceAccountName)
	o.Logger.Infof("CLEANUP-ON-FAILURE=\"%v\"", o.PerformCleanupOnFail)
	o.Logger.Infof("POD CPU REQUEST=\"%s\"", o.ResourceRequirements.Requests.Cpu().String())
	o.Logger.Infof("POD MEMORY REQUEST=\"%s\"", o.ResourceRequirements.Requests.Memory().String())
	o.Logger.Infof("POD CPU LIMIT=\"%s\"", o.ResourceRequirements.Limits.Cpu().String())
	o.Logger.Infof("POD MEMORY LIMIT=\"%s\"", o.ResourceRequirements.Limits.Memory().String())
	o.Logger.Infof("PVC STORAGE REQUEST=\"%s\"", o.PVCStorageRequest.String())
	o.Logger.Infof("====PREFLIGHT RUN OPTIONS END====")
}

// PerformPreflightChecks performs all preflight checks.
//
//nolint:gocyclo // for future ref
func (o *Run) PerformPreflightChecks(ctx context.Context) error {
	o.logPreflightOptions()
	var err error
	preflightStatus := true
	resNameSuffix, err = CreateResourceNameSuffix()
	if err != nil {
		o.Logger.Errorf("Error generating resource name suffix :: %s", err.Error())
		return err
	}
	storageSnapshotSuccess := true

	o.Logger.Infof("Generated UID for preflight check - %s\n", resNameSuffix)

	//  check kubectl
	if o.InCluster {
		o.Logger.Infoln("In cluster flag enabled. Skipping check for kubectl...")
	} else {
		o.Logger.Infoln("Checking for kubectl")
		err = o.validateKubectl(kubectlBinaryName)
		if err != nil {
			o.Logger.Errorf("%s Preflight check for kubectl utility failed :: %s\n", cross, err.Error())
			o.Logger.Errorf("%s Preflight check for kubectl utility failed :: %s\n", cross, err.Error())
			preflightStatus = false
		} else {
			o.Logger.Infof("%s Preflight check for kubectl utility is successful\n", check)
		}
	}

	// check cluster default ns access
	o.Logger.Infoln("Checking access to the default namespace of cluster")
	err = o.validateClusterAccess(ctx, internal.DefaultNs, kubeClient.ClientSet)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for cluster access failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubectl access is successful\n", check)
	}

	// Helm check
	if o.InCluster {
		o.Logger.Infoln("In cluster flag enabled. Skipping check for helm...")
	} else {
		o.Logger.Infof("Checking for required Helm version (>= %s)\n", minHelmVersion)
		err = o.validateSystemHelmVersion(HelmBinaryName, kubeClient.DiscClient)
		if err != nil {
			o.Logger.Errorf("%s Preflight check for helm version failed :: %s\n", cross, err.Error())
			preflightStatus = false
		}
	}

	// kubernetes server version check
	o.Logger.Infof("Checking for required kubernetes server version (>=%s)\n", minK8sVersion)
	err = o.validateKubernetesVersion(minK8sVersion, kubeClient.ClientSet)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for kubernetes version failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubernetes version is successful\n", check)
	}

	// rbac check
	o.Logger.Infoln("Checking Kubernetes RBAC")
	err = o.validateKubernetesRBAC(RBACAPIGroup, RBACAPIVersion, kubeClient.DiscClient)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for kubernetes RBAC failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for kubernetes RBAC is successful\n", check)
	}

	//  Check VolumeSnapshot CRDs installation
	o.Logger.Infoln("Checking if VolumeSnapshot CRDs are installed in the cluster or else create")
	var skipSnapshotCRDCheck bool
	serverVersion, sErr := kubeClient.DiscClient.ServerVersion()
	if sErr != nil {
		o.Logger.Errorf("Preflight check for VolumeSnapshot CRDs failed :: error getting server version: %s\n",
			sErr.Error())
		skipSnapshotCRDCheck = true
		preflightStatus = false
	}
	if !skipSnapshotCRDCheck {
		err = o.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion.String(), kubeClient.RuntimeClient)
		if err != nil {
			o.Logger.Errorf("Preflight check for VolumeSnapshot CRDs failed :: %s\n", err.Error())
			preflightStatus = false
		} else {
			o.Logger.Infof("%s Preflight check for VolumeSnapshot CRDs is successful\n", check)
		}
	}

	//  Check storage snapshot class
	o.Logger.Infoln("Checking if a StorageClass and VolumeSnapshotClass are present")
	var (
		skipSnapshotClassCheck bool
		prefVersion            string
	)
	sc, err := kubeClient.ClientSet.StorageV1().StorageClasses().Get(ctx, o.StorageClass, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			o.Logger.Errorf("%s Preflight check for SnapshotClass failed :: not found storageclass -"+
				" %s on cluster\n", cross, o.StorageClass)
		}
		o.Logger.Errorf("%s Preflight check for SnapshotClass failed :: %s\n", cross, err.Error())
		skipSnapshotClassCheck = true
		storageSnapshotSuccess = false
		preflightStatus = false
	}

	err = o.validateRequiredPodCapabilities(ctx, resNameSuffix, kubeClient)
	if err != nil {
		o.Logger.Errorf("%s Preflight check for pod capability failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for pod capability is successful\n", check)
	}

	if !skipSnapshotClassCheck {
		prefVersion, err = GetServerPreferredVersionForGroup(StorageSnapshotGroup, kubeClient.ClientSet)
		if err != nil {
			o.Logger.Errorf("%s Preflight check for SnapshotClass failed :: error getting preferred version for group"+
				" - %s :: %s\n", cross, StorageSnapshotGroup, err.Error())
			storageSnapshotSuccess = false
			preflightStatus = false
			skipSnapshotClassCheck = true
		}

		if !skipSnapshotClassCheck {
			err = o.validateStorageSnapshotClass(ctx, sc.Provisioner, prefVersion,
				kubeClient.ClientSet, kubeClient.RuntimeClient)
			if err != nil {
				o.Logger.Errorf("%s Preflight check for SnapshotClass failed :: %s\n", cross, err.Error())
				storageSnapshotSuccess = false
				preflightStatus = false
			} else {
				o.Logger.Infof("%s Preflight check for SnapshotClass is successful\n", check)
			}
		}
	}

	//  Check DNS resolution
	err = o.validateDNSResolution(ctx, execDNSResolutionCmd, resNameSuffix, kubeClient)
	o.Logger.Infoln("Checking if DNS resolution is working in k8s cluster")
	if err != nil {
		o.Logger.Errorf("%s Preflight check for DNS resolution failed :: %s\n", cross, err.Error())
		preflightStatus = false
	} else {
		o.Logger.Infof("%s Preflight check for DNS resolution is successful\n", check)
	}

	//  Check volume snapshot and restore
	if storageSnapshotSuccess {
		o.Logger.Infoln("Checking if volume snapshot and restore is enabled in cluster")
		err = o.validateVolumeSnapshot(ctx, resNameSuffix, kubeClient)
		if err != nil {
			o.Logger.Errorf("%s Preflight check for volume snapshot and restore failed :: %s\n", cross, err.Error())
			preflightStatus = false
		} else {
			o.Logger.Infof("%s Preflight check for volume snapshot and restore is successful\n", check)
		}
	} else {
		o.Logger.Errorf("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed")
	}

	co := &Cleanup{
		CommonOptions: CommonOptions{
			Kubeconfig: o.Kubeconfig,
			Namespace:  o.Namespace,
			Logger:     o.Logger,
		},
		CleanupOptions: CleanupOptions{
			UID: resNameSuffix,
		},
	}
	if !preflightStatus {
		o.Logger.Warnln("Some preflight checks failed")
	} else {
		o.Logger.Infoln("All preflight checks succeeded!")
	}
	if preflightStatus || o.PerformCleanupOnFail {
		err = co.CleanupPreflightResources(ctx)
		if err != nil {
			o.Logger.Errorf("%s Failed to cleanup preflight resources :: %s\n", cross, err.Error())
		}
	}

	if !preflightStatus {
		return fmt.Errorf("some preflight checks failed. Check logs for more details")
	}

	return nil
}

// validateKubectl checks whether kubectl utility is installed.
func (o *Run) validateKubectl(binaryName string) error {
	path, err := goexec.LookPath(binaryName)
	if err != nil {
		return fmt.Errorf("error finding '%s' binary in $PATH of the system :: %s", binaryName, err.Error())
	}
	o.Logger.Infof("kubectl found at path - %s\n", path)

	return nil
}

// validateClusterAccess Checks access to default namespace to cluster
func (o *Run) validateClusterAccess(ctx context.Context, namespace string, kubeClient *kubernetes.Clientset) error {
	_, err := kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to access default namespace of cluster :: %s", err.Error())
	}

	return nil
}

// validateSystemHelmVersion checks whether minimum helm version is present.
func (o *Run) validateSystemHelmVersion(binaryName string, cl *discovery.DiscoveryClient) error {
	if internal.CheckIsOpenshift(cl, internal.OcpAPIVersion) {
		o.Logger.Infof("%s Running an Openshift cluster. Helm check is not needed for Openshift clusters\n", check)
		return nil
	}

	o.Logger.Infof("APIVersion - %s not found on cluster, not an OCP cluster\n", internal.OcpAPIVersion)

	err := o.validateHelmBinary(binaryName)
	if err != nil {
		return err
	}

	curVersion, err := GetHelmVersion(HelmBinaryName)
	if err != nil {
		return err
	}

	if err := o.validateHelmVersion(curVersion); err != nil {
		return err
	}

	o.Logger.Infof("%s Preflight check for helm version is successful\n", check)

	return nil
}

func (o *Run) validateHelmVersion(curVersion string) error {
	v1, err := version.NewVersion(minHelmVersion)
	if err != nil {
		return err
	}
	v2, err := version.NewVersion(curVersion)
	if err != nil {
		return err
	}
	if v2.LessThan(v1) {
		return fmt.Errorf("helm does not meet minimum version requirement.\nUpgrade helm to minimum version - %s", minHelmVersion)
	}

	o.Logger.Infof("%s Helm version %s meets required version\n", check, curVersion)

	return nil
}

func (o *Run) validateHelmBinary(binaryName string) error {
	// check whether helm exists
	path, err := goexec.LookPath(binaryName)
	if err != nil {
		return fmt.Errorf("error finding '%s' binary in $PATH of the system :: %s", binaryName, err.Error())
	}
	o.Logger.Infof("helm found at path - %s\n", path)
	return nil
}

// validateKubernetesVersion checks whether minimum k8s version requirement is met
func (o *Run) validateKubernetesVersion(minVersion string, cl *kubernetes.Clientset) error {
	serverVer, err := cl.ServerVersion()
	if err != nil {
		return err
	}

	v1, err := version.NewVersion(minVersion)
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

// validateKubernetesRBAC fetches the apiVersions present on k8s server.
// And checks whether api group and version are present.
// 'ExtractGroupVersions' func call is taken from kubectl mirror repo.
func (o *Run) validateKubernetesRBAC(apiGroup, apiVersion string, cl *discovery.DiscoveryClient) error {
	groupList, err := cl.ServerGroups()
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
		if gv.Group == apiGroup && gv.Version == apiVersion {
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

// validateStorageSnapshotClass checks whether storageclass is present.
// Checks whether storageclass and volumesnapshotclass provisioner are same.
func (o *Run) validateStorageSnapshotClass(ctx context.Context, provisioner, prefVersion string,
	kubeClient *kubernetes.Clientset, runtClient client.Client) error {
	o.Logger.Infof("%s Storageclass - %s found on cluster\n", check, o.StorageClass)
	if o.SnapshotClass == "" {
		var err error
		storageVolSnapClass, err = o.checkAndCreateSnapshotClassForProvisioner(ctx, prefVersion, provisioner, runtClient)
		if err != nil {
			o.Logger.Errorf("%s %s\n", cross, err.Error())
			return err
		}
	} else {
		storageVolSnapClass = o.SnapshotClass
		vsc, err := clusterHasVolumeSnapshotClass(ctx, o.SnapshotClass, kubeClient, runtClient)
		if err != nil {
			o.Logger.Errorf("%s %s\n", cross, err.Error())
			return err
		}

		if vsc.Object["driver"] == provisioner {
			o.Logger.Infof("%s Volume snapshot class - %s driver matches with given storage class provisioner\n",
				check, o.SnapshotClass)
		} else {
			return fmt.Errorf("volume snapshot class - %s driver does not match with given StorageClass's"+
				" provisioner=%s", o.SnapshotClass, provisioner)
		}
	}

	return nil
}

// checkAndCreateSnapshotClassForProvisioner checks whether snapshot-class exist for a provisioner, and creates if not present
func (o *Run) checkAndCreateSnapshotClassForProvisioner(ctx context.Context, prefVersion,
	provisioner string, cl client.Client) (string, error) {
	var err error

	vsscList := unstructured.UnstructuredList{}
	vsscList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: prefVersion,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = cl.List(ctx, &vsscList)
	if err != nil {
		return "", err
	} else if len(vsscList.Items) == 0 {
		o.Logger.Infof("no volume snapshot class for APIVersion - %s/%s found on cluster, attempting installation...",
			StorageSnapshotGroup, prefVersion)
		vscName, cErr := o.createVolumeSnapshotClass(ctx, provisioner, prefVersion, cl)
		if cErr != nil {
			return "", fmt.Errorf("error creating volume snapshot class having driver - %s"+
				" :: %s", provisioner, cErr.Error())
		}
		return vscName, nil
	}

	sscName := ""
	for _, vssc := range vsscList.Items {
		if vssc.Object["driver"] == provisioner {
			if v, ok, err := unstructured.NestedString(
				vssc.Object, "metadata", "annotations", SnapshotClassIsDefaultAnnotation); err == nil && ok && v == "true" {
				o.Logger.Infof("%s Default volume snapshot class - %s found in cluster", check, vssc.GetName())
				o.Logger.Infof("%s Volume snapshot class - %s driver matches with given StorageClass's provisioner=%s\n",
					check, vssc.GetName(), provisioner)
				return vssc.GetName(), nil
			}
			sscName = vssc.GetName()
		}
	}
	if sscName == "" {
		o.Logger.Infof("no matching volume snapshot class having driver "+
			"same as provisioner - %s found on cluster, attempting installation...", provisioner)
		vscName, cErr := o.createVolumeSnapshotClass(ctx, provisioner, prefVersion, cl)
		if cErr != nil {
			return "", fmt.Errorf("error creating volume snapshot class having driver - %s"+
				" :: %s", provisioner, cErr.Error())
		}
		return vscName, nil
	}

	o.Logger.Infof("%s Extracted volume snapshot class - %s found in cluster", check, sscName)
	o.Logger.Infof("%s Volume snapshot class - %s driver matches with given StorageClass's provisioner=%s\n",
		check, sscName, provisioner)
	return sscName, nil
}

func (o *Run) createVolumeSnapshotClass(ctx context.Context, driver, prefVersion string, cl client.Client) (string, error) {
	vscUnstrObj := &unstructured.Unstructured{}
	vscUnstrObj.SetUnstructuredContent(map[string]interface{}{
		"driver":         driver,
		"deletionPolicy": "Delete",
	})
	vscUnstrObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: prefVersion,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	randStr, err := CreateResourceNameSuffix()
	if err != nil {
		return "", fmt.Errorf("error generating resource name suffix: %s", err.Error())
	}
	vscName := defaultVSCNamePrefix + randStr
	vscUnstrObj.SetName(vscName)

	if cErr := cl.Create(ctx, vscUnstrObj); cErr != nil {
		return "", cErr
	}

	o.Logger.Infof("%s Volume snapshot class with driver as - %s for version - %s successfully created",
		check, driver, prefVersion)
	vscYAML, yErr := objToYAML(vscUnstrObj)
	if yErr != nil {
		o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
	} else {
		o.Logger.Warnf("Volume snapshot class object created with the following spec."+
			" User can edit fields later, if required.. \n::::::::\n%s::::::::", string(vscYAML))
	}

	return vscName, nil
}

// checkAndCreateVolumeSnapshotCRDs checks and creates volumesnapshot and related CRDs if not present on cluster.
func (o *Run) checkAndCreateVolumeSnapshotCRDs(ctx context.Context, serverVersion string, cl client.Client) error {

	prefCRDVersion, gErr := getPrefSnapshotClassVersion(serverVersion)
	if gErr != nil {
		return gErr
	}

	var errs []error
	for _, crd := range VolumeSnapshotCRDs {
		var crdObj = &apiextensions.CustomResourceDefinition{}
		if err := cl.Get(ctx, client.ObjectKey{Name: crd}, crdObj); err != nil {
			if !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error getting volume snapshot class CRD :: %s", err.Error())
			}
			o.Logger.Infof("Volume snapshot CRD: %s not found on cluster. Attempting installation...", crd)

			fileBytes, rErr := crdYamlFiles.ReadFile(filepath.Join(volumeSnapshotCRDYamlDir, prefCRDVersion, crd+".yaml"))
			if rErr != nil {
				errs = append(errs, rErr)
				continue
			}

			unmarshalCRDObj := &apiextensions.CustomResourceDefinition{}
			if uErr := yaml.Unmarshal(fileBytes, unmarshalCRDObj); uErr != nil {
				errs = append(errs, uErr)
				continue
			}

			if cErr := cl.Create(ctx, unmarshalCRDObj); cErr != nil {
				errs = append(errs, cErr)
				continue
			}

			// if we are creating the volumesnapshotclass CRD, then any user provided volumesnapshotclass name should be
			// overridden because no volumesnapshotclass will be existing without CRD.
			if crd == "volumesnapshotclasses."+StorageSnapshotGroup {
				o.SnapshotClass = ""
			}

			o.Logger.Infof("%s Volume snapshot CRD: %s successfully created", check, crd)
		} else {
			o.Logger.Infof("%s Volume snapshot CRD: %s already exists, skipping installation", check, crd)
		}
	}

	if len(errs) != 0 {
		return kerrors.NewAggregate(errs)
	}

	return nil
}

// validateDNSResolution checks whether DNS resolution is working on k8s cluster
func (o *Run) validateDNSResolution(ctx context.Context, execCommand []string, podNameSuffix string, clients ServerClients) error {
	pod, err := o.createDNSPodOnCluster(ctx, podNameSuffix, clients.ClientSet)
	if err != nil {
		return err
	}

	op := exec.Options{
		Namespace:     o.Namespace,
		Command:       execCommand,
		PodName:       pod.GetName(),
		ContainerName: dnsContainerName,
		Executor:      &exec.DefaultRemoteExecutor{},
		Config:        clients.RestConfig,
		ClientSet:     clients.ClientSet,
	}
	err = execInPod(&op, o.Logger)
	if err != nil {
		return fmt.Errorf("not able to resolve DNS '%s' service inside pods", execCommand[1])
	}

	// Delete DNS pod when resolution is successful
	err = deleteK8sResource(ctx, pod, clients.RuntimeClient)
	if err != nil {
		o.Logger.Warnf("Problem occurred deleting DNS pod - '%s' :: %s", pod.GetName(), err.Error())
	} else {
		o.Logger.Infof("Deleted DNS pod - '%s' successfully", pod.GetName())
	}

	return nil
}

func (o *Run) createDNSPodOnCluster(ctx context.Context, podNameSuffix string, clientSet *kubernetes.Clientset) (*corev1.Pod, error) {
	pod := createDNSPodSpec(o, podNameSuffix)
	_, err := clientSet.CoreV1().Pods(o.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		dnsPodYaml, yErr := objToYAML(pod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create DNS Pod. Pod yaml:\n%v", string(dnsPodYaml))
		}
		return nil, err
	}
	o.Logger.Infof("Pod %s created in cluster\n", pod.GetName())

	waitOptions := &wait.PodWaitOptions{
		Name:               pod.GetName(),
		Namespace:          o.Namespace,
		RetryBackoffParams: getDefaultRetryBackoffParams(),
		PodCondition:       corev1.PodReady,
		ClientSet:          clientSet,
	}
	o.Logger.Infoln("Waiting for dns pod to become ready")
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		o.Logger.Errorf("DNS pod - %s hasn't reached into ready state", pod.GetName())
		dnsPodYaml, yErr := objToYAML(pod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("DNS pod failed to reach into ready state. Pod yaml:\n%v", string(dnsPodYaml))
		}
		return nil, err
	}

	pod, err = clientSet.CoreV1().Pods(o.Namespace).Get(ctx, pod.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	logPodScheduleStmt(pod, o.Logger)

	return pod, nil
}

// validateVolumeSnapshot checks if volume snapshot and restore is enabled in the cluster
func (o *Run) validateVolumeSnapshot(ctx context.Context, nameSuffix string, clients ServerClients) error {
	var (
		execOp          exec.Options
		err             error
		pvc             *corev1.PersistentVolumeClaim
		srcPod          *corev1.Pod
		prefSnapshotVer string
	)

	prefSnapshotVer, err = GetServerPreferredVersionForGroup(StorageSnapshotGroup, clients.ClientSet)
	if err != nil {
		return err
	}

	// create source pod, pvc and volume snapshot
	pvc, err = o.createSourcePVC(ctx, nameSuffix, clients.ClientSet)
	if err != nil {
		return err
	}
	o.Logger.Infof("Created source pvc - %s", pvc.GetName())
	srcPod, err = o.createSourcePodFromPVC(ctx, nameSuffix, pvc.GetName(), clients.ClientSet)
	if err != nil {
		return err
	}
	volSnap, err := o.createSnapshotFromPVC(ctx, VolumeSnapSrcNamePrefix+nameSuffix,
		storageVolSnapClass, prefSnapshotVer, pvc.GetName(), nameSuffix, clients)
	if err != nil {
		return err
	}

	// create restore pod, pvc from source snapshot
	restorePod, err := o.createRestorePodFromSnapshot(ctx, volSnap, RestorePvcNamePrefix+nameSuffix,
		RestorePodNamePrefix+nameSuffix, nameSuffix, clients.ClientSet)
	if err != nil {
		return err
	}
	execOp = exec.Options{
		Namespace:     o.Namespace,
		Command:       execRestoreDataCheckCommand,
		PodName:       restorePod.GetName(),
		ContainerName: restorePod.Spec.Containers[0].Name,
		Executor:      &exec.DefaultRemoteExecutor{},
		Config:        clients.RestConfig,
		ClientSet:     clients.ClientSet,
	}
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}
	o.Logger.Infof("Restored pod - %s has expected data\n", restorePod.GetName())

	// remove source pod
	srcPodName := srcPod.GetName()
	srcPod, err = clients.ClientSet.CoreV1().Pods(o.Namespace).Get(ctx, srcPod.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	o.Logger.Infof("Deleting source pod - %s\n", srcPod.GetName())
	err = deleteK8sResource(ctx, srcPod, clients.RuntimeClient)
	if err != nil {
		return err
	}
	o.Logger.Infof("Deleted source pod - %s\n", srcPodName)

	// create unmounted pod, pvc and  snapshot from source pvc
	unmountedVolSnapSrc, err := o.createSnapshotFromPVC(ctx, UnmountedVolumeSnapSrcNamePrefix+nameSuffix,
		storageVolSnapClass, prefSnapshotVer, pvc.GetName(), nameSuffix, clients)
	if err != nil {
		return err
	}
	unmountedPodSpec, err := o.createRestorePodFromSnapshot(
		ctx, unmountedVolSnapSrc, UnmountedRestorePvcNamePrefix+nameSuffix,
		UnmountedRestorePodNamePrefix+nameSuffix, nameSuffix, clients.ClientSet)
	if err != nil {
		return err
	}
	execOp.PodName = unmountedPodSpec.GetName()
	execOp.ContainerName = unmountedPodSpec.Spec.Containers[0].Name
	err = execInPod(&execOp, o.Logger)
	if err != nil {
		return err
	}
	o.Logger.Infof("%s restored pod from volume snapshot of unmounted pv has expected data\n", check)

	return nil
}

func (o *Run) createSourcePVC(ctx context.Context, nameSuffix string,
	k8sClient *kubernetes.Clientset) (pvc *corev1.PersistentVolumeClaim, err error) {
	pvc = createVolumeSnapshotPVCSpec(o, SourcePvcNamePrefix+nameSuffix, nameSuffix)
	pvc, err = k8sClient.CoreV1().PersistentVolumeClaims(o.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		srcPVCYaml, yErr := objToYAML(pvc)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create source PVC. PVC yaml: %s\n", string(srcPVCYaml))
		}
		return nil, err
	}

	return pvc, nil
}

// createSourcePodFromPVC creates source pod and pvc for volume snapshot check
func (o *Run) createSourcePodFromPVC(ctx context.Context, nameSuffix, pvcName string,
	k8sClient *kubernetes.Clientset) (srcPod *corev1.Pod, err error) {

	srcPod = createVolumeSnapshotPodSpec(pvcName, o, nameSuffix)
	srcPod, err = k8sClient.CoreV1().Pods(o.Namespace).Create(ctx, srcPod, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		srcPodYaml, yErr := objToYAML(srcPod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create source pod. Pod yaml: \n%s", string(srcPodYaml))
		}
		return nil, err
	}
	o.Logger.Infof("Created source pod - %s", srcPod.GetName())

	//  Wait for snapshot pod to become ready.
	waitOptions := &wait.PodWaitOptions{
		Name:               srcPod.GetName(),
		Namespace:          o.Namespace,
		RetryBackoffParams: getDefaultRetryBackoffParams(),
		PodCondition:       corev1.PodReady,
		ClientSet:          k8sClient,
	}
	o.Logger.Infof("Waiting for source pod - %s to become ready\n", srcPod.GetName())
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		srcPodYaml, yErr := objToYAML(srcPod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Source pod failed to reach ready state. Pod yaml: \n%s", string(srcPodYaml))
		}
		return srcPod, fmt.Errorf("pod %s hasn't reached into ready state", srcPod.GetName())
	}
	o.Logger.Infof("Source pod - %s has reached into ready state\n", srcPod.GetName())

	srcPod, err = k8sClient.CoreV1().Pods(o.Namespace).Get(ctx, srcPod.GetName(), metav1.GetOptions{})
	if err != nil {
		return srcPod, err
	}
	logPodScheduleStmt(srcPod, o.Logger)

	return srcPod, err
}

func (o *Run) createSnapshotFromPVC(ctx context.Context, volSnapName, volSnapClass, snapshotVer,
	pvcName, uid string, clients ServerClients) (*unstructured.Unstructured, error) {
	volSnap := createVolumeSnapsotSpec(volSnapName, volSnapClass, o.Namespace, snapshotVer, pvcName, uid)
	if err := clients.RuntimeClient.Create(ctx, volSnap); err != nil {
		volSnapYAML, yErr := objToYAML(volSnap)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create volume snapshot. Volume snapshot yaml: \n%s", string(volSnapYAML))
		}
		return nil, fmt.Errorf("%s error creating volume snapshot from pvc :: %s", cross, err.Error())
	}
	o.Logger.Infof("Created volume snapshot - %s from pvc", volSnap.GetName())

	o.Logger.Infof("Waiting for volume snapshot - %s created from pvc to become 'readyToUse:true'", volSnap.GetName())
	err := waitUntilVolSnapReadyToUse(volSnap, snapshotVer, getDefaultRetryBackoffParams(), clients.RuntimeClient)
	if err != nil {
		if err == k8swait.ErrWaitTimeout {
			volSnapYAML, yErr := objToYAML(volSnap)
			if yErr != nil {
				o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
			} else {
				o.Logger.Warnf("Volume snapshot failed to reach into ready state. Volume snapshot yaml: \n%s", string(volSnapYAML))
			}
			return nil, fmt.Errorf("volume snapshot - %s not readyToUse (waited 600 sec) :: %s",
				volSnap.GetName(), err.Error())
		}
		return nil, err
	}
	o.Logger.Infof("%s volume snapshot - %s is ready-to-use", check, volSnap.GetName())

	return volSnap, err
}

func (o *Run) createRestorePodFromSnapshot(ctx context.Context, volSnapshot *unstructured.Unstructured,
	pvcName, podName, uid string, k8slient *kubernetes.Clientset) (*corev1.Pod, error) {
	var err error
	restorePVC := createRestorePVCSpec(pvcName, volSnapshot.GetName(), uid, o)
	restorePVC, err = k8slient.CoreV1().PersistentVolumeClaims(o.Namespace).
		Create(ctx, restorePVC, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		restorePVCYaml, yErr := objToYAML(restorePVC)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create restore PVC. Restore PVC yaml:\n%s", string(restorePVCYaml))
		}
		return nil, err
	}
	o.Logger.Infof("Created restore pvc - %s from volume snapshot - %s\n", restorePVC.GetName(), volSnapshot.GetName())
	restorePod := createRestorePodSpec(podName, restorePVC.GetName(), uid, o)
	restorePod, err = k8slient.CoreV1().Pods(o.Namespace).
		Create(ctx, restorePod, metav1.CreateOptions{})
	if err != nil {
		o.Logger.Errorln(err.Error())
		restorePodYaml, yErr := objToYAML(restorePod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create Restore pod. Pod yaml:\n%s", string(restorePodYaml))
		}
		return nil, err
	}
	o.Logger.Infof("Created restore pod - %s\n", restorePod.GetName())

	//  Wait for snapshot pod to become ready.
	waitOptions := &wait.PodWaitOptions{
		Name:               restorePod.GetName(),
		Namespace:          o.Namespace,
		RetryBackoffParams: getDefaultRetryBackoffParams(),
		PodCondition:       corev1.PodReady,
		ClientSet:          k8slient,
	}
	o.Logger.Infof("Waiting for restore pod - %s to become ready\n", restorePod.GetName())
	waitOptions.Name = restorePod.GetName()
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		restorePodYaml, yErr := objToYAML(restorePod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Restore Pod failed to reach into ready state. Pod yaml:\n%s", string(restorePodYaml))
		}
		return nil, err
	}
	o.Logger.Infof("%s Restore pod - %s has reached into ready state\n", check, restorePod.GetName())

	restorePod, err = k8slient.CoreV1().Pods(o.Namespace).Get(ctx, restorePod.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	logPodScheduleStmt(restorePod, o.Logger)

	return restorePod, nil
}

func (o *Run) validateRequiredPodCapabilities(ctx context.Context, podNameSuffix string, clients ServerClients) error {
	validationCases := []capability{
		{
			userID:                   0,
			allowPrivilegeEscalation: true,
			privileged:               true,
		},
		{
			userID:                   1001,
			allowPrivilegeEscalation: false,
			privileged:               false,
		},
		{
			userID:                   101,
			allowPrivilegeEscalation: true,
			privileged:               false,
		},
	}
	for index, validationCase := range validationCases {
		o.Logger.Infof("Checking pod capability validation case %d/3", index+1)
		err := o.validatePodCapability(ctx, fmt.Sprintf("%d-%s", index, podNameSuffix), clients, validationCase)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Run) validatePodCapability(ctx context.Context, podNameSuffix string, clients ServerClients, validationCase capability) error {
	capabilityValidatorPod := createPodSpecWithCapability(o, podNameSuffix, validationCase)
	_, err := clients.ClientSet.CoreV1().Pods(o.Namespace).Create(ctx, capabilityValidatorPod, metav1.CreateOptions{})
	if err != nil {
		podYaml, yErr := objToYAML(capabilityValidatorPod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("Failed to create capability validator pod. Pod yaml:\n%v", string(podYaml))
		}
		return err
	}
	o.Logger.Infof("Pod %s created in cluster\n", capabilityValidatorPod.GetName())

	waitOptions := &wait.PodWaitOptions{
		Name:               capabilityValidatorPod.GetName(),
		Namespace:          o.Namespace,
		RetryBackoffParams: getDefaultRetryBackoffParams(),
		PodCondition:       corev1.PodReady,
		ClientSet:          clients.ClientSet,
	}
	o.Logger.Infoln("Waiting for capability validator pod to become ready")
	err = waitUntilPodCondition(ctx, waitOptions)
	if err != nil {
		o.Logger.Errorf("capability validator pod - %s hasn't reached into ready state", capabilityValidatorPod.GetName())
		podYaml, yErr := objToYAML(capabilityValidatorPod)
		if yErr != nil {
			o.Logger.Errorf("error converting object to yaml :: %s", yErr.Error())
		} else {
			o.Logger.Warnf("capability validator pod failed to reach into ready state. Pod yaml:\n%v", string(podYaml))
		}
		return err
	}

	capabilityValidatorPod, err = clients.ClientSet.CoreV1().Pods(o.Namespace).Get(
		ctx,
		capabilityValidatorPod.GetName(),
		metav1.GetOptions{},
	)
	if err != nil {
		return err
	}
	logPodScheduleStmt(capabilityValidatorPod, o.Logger)
	// Delete capability validator pod when validation is successful
	err = deleteK8sResource(ctx, capabilityValidatorPod, clients.RuntimeClient)
	if err != nil {
		o.Logger.Warnf("Problem occurred deleting capability validator pod - '%s' :: %s", capabilityValidatorPod.GetName(), err.Error())
	} else {
		o.Logger.Infof("Deleted capability validator pod - '%s' successfully", capabilityValidatorPod.GetName())
	}
	return nil

}
