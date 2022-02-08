package preflighttest

// nolint // ignore dot import lint errors
import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Individual assertions of successful preflight checks which do not involve any CRUD operations
func nonCRUDPreflightCheckAssertion(inputFlags map[string]string, outputLog string) {
	storageClass, ok := inputFlags[storageClassFlag]
	Expect(ok).To(BeTrue())
	assertPreflightLogFileCreateSuccess(outputLog)
	assertVolSnapClassCheckSuccess(inputFlags, outputLog)
	assertKubectlBinaryCheckSuccess(outputLog)
	assertK8sClusterRBACCheckSuccess(outputLog)
	assertHelmVersionCheckSuccess(outputLog)
	assertK8sServerVersionCheckSuccess(outputLog)
	assertCsiAPICheckSuccess(outputLog)
	assertClusterAccessCheckSuccess(outputLog)
	assertStorageClassCheckSuccess(storageClass, outputLog)
}

func assertPreflightLogFileCreateSuccess(outputLog string) {
	By("Preflight log file should be created")
	Expect(outputLog).To(ContainSubstring("Created log file with name - preflight-"))
}

func assertSuccessfulPreflightChecks(inputFlags map[string]string, outputLog string) {
	nonCRUDPreflightCheckAssertion(inputFlags, outputLog)
	assertDNSResolutionCheckSuccess(outputLog)
	assertVolumeSnapshotCheckSuccess(outputLog)
}

func assertVolSnapClassCheckSuccess(inputFlags map[string]string, outputLog string) {
	By("Check whether volume snapshot class is present on cluster")
	snapshotClass, ok := inputFlags[snapshotClassFlag]
	if ok {
		Expect(outputLog).To(ContainSubstring(
			fmt.Sprintf("Volume snapshot class - %s driver matches with given storage class provisioner",
				snapshotClass)))
	} else {
		Expect(outputLog).
			To(MatchRegexp("(Extracted volume snapshot class -)(.*)(found in cluster)"))
		Expect(outputLog).
			To(MatchRegexp("(Volume snapshot class -)(.*)(driver matches with given StorageClass's provisioner)"))
	}
}

func assertKubectlBinaryCheckSuccess(outputLog string) {
	By("Find kubectl on client machine")
	Expect(outputLog).To(ContainSubstring("kubectl found at path - "))
	Expect(outputLog).To(ContainSubstring("Preflight check for kubectl utility is successful"))
}

func assertClusterAccessCheckSuccess(outputLog string) {
	By("Check access to cluster")
	Expect(outputLog).To(ContainSubstring("Preflight check for kubectl access is successful"))
}

func assertHelmVersionCheckSuccess(outputLog string) {
	By("Check whether helm is installed or it is an OCP cluster")
	if discClient != nil && internal.CheckIsOpenshift(discClient, internal.OcpAPIVersion) {
		Expect(outputLog).To(ContainSubstring("Running OCP cluster. Helm not needed for OCP clusters"))
	} else {
		Expect(outputLog).To(ContainSubstring("helm found at path - "))
		var helmVersion string
		helmVersion, err = preflight.GetHelmVersion()
		Expect(err).To(BeNil())
		Expect(outputLog).
			To(ContainSubstring(fmt.Sprintf("Helm version %s meets required version", helmVersion)))
	}
}

func assertK8sServerVersionCheckSuccess(outputLog string) {
	By("Check whether K8s server version is satisfied")
	Expect(outputLog).To(ContainSubstring("Preflight check for kubernetes version is successful"))
}

func assertK8sClusterRBACCheckSuccess(outputLog string) {
	By("Check whether RBAC is enabled on cluster")
	Expect(outputLog).To(ContainSubstring("Kubernetes RBAC is enabled"))
	Expect(outputLog).To(ContainSubstring("Preflight check for kubernetes RBAC is successful"))
}

func assertStorageClassCheckSuccess(storageClass, outputLog string) {
	By("Check whether storage class is present on cluster")
	Expect(outputLog).
		To(ContainSubstring(fmt.Sprintf("Storageclass - %s found on cluster", storageClass)))
	Expect(outputLog).To(ContainSubstring("Preflight check for SnapshotClass is successful"))
}

func assertCsiAPICheckSuccess(outputLog string) {
	By("Check whether CSI APIs are installed on the cluster")
	for _, api := range preflight.CsiApis {
		Expect(outputLog).
			To(ContainSubstring(fmt.Sprintf("Found CSI API - %s on cluster", api)))
	}
	Expect(outputLog).To(ContainSubstring("Preflight check for CSI is successful"))
}

func assertDNSResolutionCheckSuccess(outputLog string) {
	By("Check whether DNS can resolved on cluster")
	Expect(outputLog).To(MatchRegexp("(Pod dnsutils-)([a-z]{6})( created in cluster)"))
	Expect(outputLog).To(ContainSubstring("Preflight check for DNS resolution is successful"))
}

func assertVolumeSnapshotCheckSuccess(outputLog string) {
	By("Check whether volume snapshot and restore is possible on cluster")

	Expect(outputLog).
		To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
			"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; then exit 0; else exit 1; fi' " +
			"in container - 'busybox' of pod - 'restored-pod"))

	Expect(outputLog).To(MatchRegexp("(Restored pod - restored-pod-)([a-z]{6})( has expected data)"))

	Expect(outputLog).
		To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
			"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; " +
			"then exit 0; else exit 1; fi' in container - 'busybox' of pod - 'unmounted-restored-pod"))

	Expect(outputLog).To(ContainSubstring("restored pod from volume snapshot of unmounted pv has expected data"))
	Expect(outputLog).To(ContainSubstring("Preflight check for volume snapshot and restore is successful"))
}

func createPreflightResourcesForCleanup() string {
	var uid string
	uid, err = preflight.CreateResourceNameSuffix()
	Expect(err).To(BeNil())
	createPreflightPVC(uid)
	srcPvcName := strings.Join([]string{preflight.SourcePvcNamePrefix, uid}, "")
	createPreflightVolumeSnapshot(srcPvcName, uid)
	createPreflightPods(srcPvcName, uid)

	return uid
}

func createPreflightPods(pvcName, preflightUID string) {
	createDNSPod(preflightUID)
	createSourcePod(pvcName, preflightUID)
}

func createDNSPod(preflightUID string) {
	dnsPod := createDNSPodSpec(preflightUID)
	err = runtimeClient.Create(ctx, dnsPod)
	Expect(err).To(BeNil())
}

func createDNSPodSpec(preflightUID string) *corev1.Pod {
	pod := getPodTemplate(dnsPodNamePrefix, preflightUID)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:            dnsContainerName,
			Image:           strings.Join([]string{preflight.GcrRegistryPath, preflight.DNSUtilsImage}, "/"),
			Command:         preflight.CommandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       preflight.ResourceRequirements,
		},
	}

	return pod
}

func createSourcePod(pvcName, preflightUID string) {
	srcPod := createSourcePodSpec(pvcName, preflightUID)
	err = runtimeClient.Create(ctx, srcPod)
	Expect(err).To(BeNil())
}

func createSourcePodSpec(pvcName, preflightUID string) *corev1.Pod {
	pod := getPodTemplate(preflight.SourcePodNamePrefix, preflightUID)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:      preflight.BusyboxContainerName,
			Image:     preflight.BusyboxImageName,
			Command:   preflight.CommandBinSh,
			Args:      preflight.ArgsTouchDataFileSleep,
			Resources: preflight.ResourceRequirements,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      preflight.VolMountName,
					MountPath: preflight.VolMountPath,
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: preflight.VolMountName,
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

func createPreflightPVC(preflightUID string) {
	pvc := createPreflightPVCSpec(preflightUID)
	err = runtimeClient.Create(ctx, pvc)
	Expect(err).To(BeNil())
}

func createPreflightPVCSpec(preflightUID string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{preflight.SourcePvcNamePrefix, preflightUID}, ""),
			Namespace: defaultTestNs,
			Labels:    getPreflightResourceLabels(preflightUID),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: func() *string { var storageClass = defaultTestStorageClass; return &storageClass }(),
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}

func createPreflightVolumeSnapshot(pvcName, preflightUID string) {
	volSnap := createPreflightVolumeSnapshotSpec(pvcName, preflightUID)
	err = runtimeClient.Create(ctx, volSnap)
	Expect(err).To(BeNil())
}

func createPreflightVolumeSnapshotSpec(pvcName, preflightUID string) *unstructured.Unstructured {
	var snapshotVersion string
	snapshotVersion, err = preflight.GetServerPreferredVersionForGroup(preflight.StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	volSnap := &unstructured.Unstructured{}
	volSnap.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": defaultTestSnapshotClass,
			"source": map[string]string{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}
	volSnap.SetName(strings.Join([]string{preflight.VolumeSnapSrcNamePrefix, preflightUID}, ""))
	volSnap.SetNamespace(defaultTestNs)
	volSnap.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   preflight.StorageSnapshotGroup,
		Version: snapshotVersion,
		Kind:    internal.VolumeSnapshotKind,
	})
	volSnap.SetLabels(getPreflightResourceLabels(preflightUID))

	return volSnap
}

func createVolumeSnapshotClass() {
	vsc := &unstructured.Unstructured{}
	vsc.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": sampleVolSnapClassName,
		},
		"driver":         invalidVolSnapDriver,
		"deletionPolicy": "Delete",
	}
	vsc.SetGroupVersionKind(snapshotClassGVK)

	err = runtimeClient.Create(ctx, vsc)
	Expect(err).To(BeNil())
}

func deleteVolumeSnapshotClass() {
	vsc := &unstructured.Unstructured{}
	vsc.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": sampleVolSnapClassName,
		},
	}
	vsc.SetGroupVersionKind(snapshotClassGVK)

	err = runtimeClient.Delete(ctx, vsc)
	Expect(err).To(BeNil())
}

// A basic pod template for any pod to be created for testing purpose
func getPodTemplate(name, preflightUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, preflightUID}, ""),
			Namespace: defaultTestNs,
			Labels:    getPreflightResourceLabels(preflightUID),
		},
	}
}

// Labels of any preflight resource will have the below labels
func getPreflightResourceLabels(uid string) map[string]string {
	labels := map[string]string{
		preflight.LabelK8sName:   preflight.LabelK8sNameValue,
		preflight.LabelTrilioKey: preflight.LabelTvkPreflightValue,
		preflight.LabelK8sPartOf: preflight.LabelK8sPartOfValue,
	}
	if uid != "" {
		labels[preflight.LabelPreflightRunKey] = uid
	}

	return labels
}

// copy map 'from' to 'to' key-by-key and value-by-value
func copyMap(from, to map[string]string) {
	for key, value := range from {
		to[key] = value
	}
}
