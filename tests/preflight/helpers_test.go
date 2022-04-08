package preflighttest

// nolint // ignore dot import lint errors
import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tLog "github.com/sirupsen/logrus"
	preflightCmd "github.com/trilioData/tvk-plugins/cmd/preflight/cmd"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

// Individual assertions of successful preflight checks which do not involve any CRUD operations
func nonCRUDPreflightCheckAssertion(storageClass, snapshotClass, outputLog string) {
	assertPreflightLogFileCreateSuccess(outputLog)
	assertVolSnapClassCheckSuccess(snapshotClass, outputLog)
	assertKubectlBinaryCheckSuccess(outputLog)
	assertK8sClusterRBACCheckSuccess(outputLog)
	assertHelmVersionCheckSuccess(outputLog)
	assertK8sServerVersionCheckSuccess(outputLog)
	assertVolumeSnapshotCRDCheckSuccess(outputLog)
	assertClusterAccessCheckSuccess(outputLog)
	assertStorageClassCheckSuccess(storageClass, outputLog)
}

func assertPreflightLogFileCreateSuccess(outputLog string) {
	By("Preflight log file should be created")
	Expect(outputLog).To(ContainSubstring("Created log file with name - preflight-"))
}

func assertSuccessfulPreflightChecks(inputFlags map[string]string, outputLog string) {
	storageClass, ok := inputFlags[storageClassFlag]
	Expect(ok).To(BeTrue())
	snapshotClass := inputFlags[snapshotClassFlag]
	pvcStorageRequest := inputFlags[pvcStorageRequestFlag]
	nonCRUDPreflightCheckAssertion(storageClass, snapshotClass, outputLog)
	assertDNSResolutionCheckSuccess(outputLog)
	assertVolumeSnapshotCheckSuccess(outputLog)
	assertPVCStorageRequestCheckSuccess(outputLog, pvcStorageRequest)
}

func assertVolSnapClassCheckSuccess(snapshotClass, outputLog string) {
	By("Check whether volume snapshot class is present on cluster")
	if snapshotClass != "" {
		Expect(outputLog).To(ContainSubstring(
			fmt.Sprintf("Volume snapshot class - %s driver matches with given storage class provisioner",
				snapshotClass)))
	} else {
		Expect(outputLog).
			To(MatchRegexp("(Extracted volume snapshot class -)(.*)(found in cluster)"))
		Expect(outputLog).To(MatchRegexp("(Volume snapshot class -)(.*)(driver matches with given StorageClass's provisioner=)"))
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
		helmVersion, err = preflight.GetHelmVersion(preflight.HelmBinaryName)
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

func assertVolumeSnapshotCRDCheckSuccess(outputLog string) {
	By("Check whether Volume Snapshot CRDs are installed on the cluster")
	for _, api := range preflight.VolumeSnapshotCRDs {
		Expect(outputLog).
			To(ContainSubstring(fmt.Sprintf("Volume snapshot CRD: %s already exists, skipping installation", api)))
	}
	Expect(outputLog).To(ContainSubstring("Preflight check for VolumeSnapshot CRDs is successful"))
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

func assertPVCStorageRequestCheckSuccess(outputLog, pvcStorageRequest string) {
	if pvcStorageRequest == "" {
		Expect(outputLog).To(ContainSubstring(fmt.Sprintf("PVC STORAGE REQUEST=\"%s\"", preflightCmd.DefaultPVCStorage)))
	} else {
		Expect(outputLog).To(ContainSubstring(fmt.Sprintf("PVC STORAGE REQUEST=\"%s\"", pvcStorageRequest)))
	}
}

func assertSuccessCleanupUID(uid, outputLog string) {
	By(fmt.Sprintf("Should clean source pod with uid=%s", uid))
	srcPodName := strings.Join([]string{preflight.SourcePodNamePrefix, uid}, "")
	Expect(outputLog).To(ContainSubstring("Cleaning Pod - %s", srcPodName))

	By(fmt.Sprintf("Should clean dns pod with uid=%s", uid))
	dnsPodName := strings.Join([]string{dnsPodNamePrefix, uid}, "")
	Expect(outputLog).To(ContainSubstring("Cleaning Pod - %s", dnsPodName))

	By(fmt.Sprintf("Should clean source pvc with uid=%s", uid))
	srcPvcName := strings.Join([]string{preflight.SourcePvcNamePrefix, uid}, "")
	Expect(outputLog).To(ContainSubstring("Cleaning PersistentVolumeClaim - %s", srcPvcName))

	By(fmt.Sprintf("Should clean source volume snapshot with uid=%s", uid))
	srcVolSnapName := strings.Join([]string{preflight.VolumeSnapSrcNamePrefix, uid}, "")
	Expect(outputLog).To(ContainSubstring("Cleaning VolumeSnapshot - %s", srcVolSnapName))

	By(fmt.Sprintf("Should clean all preflight resources for uid=%s", uid))
	Expect(outputLog).To(ContainSubstring("All preflight resources cleaned"))

	By(fmt.Sprintf("All preflight pods with uid=%s should be removed from cluster", uid))
	Eventually(func() int {
		podList := unstructured.UnstructuredList{}
		podList.SetGroupVersionKind(podGVK)
		err = runtimeClient.List(ctx, &podList,
			client.MatchingLabels(getPreflightResourceLabels(uid)), client.InNamespace(defaultTestNs))
		Expect(err).To(BeNil())
		return len(podList.Items)
	}, timeout, interval).Should(Equal(0))

	By(fmt.Sprintf("All preflight PVCs with uid=%s should be removed from cluster", uid))
	Eventually(func() int {
		pvcList := unstructured.UnstructuredList{}
		pvcList.SetGroupVersionKind(pvcGVK)
		err = runtimeClient.List(ctx, &pvcList,
			client.MatchingLabels(getPreflightResourceLabels(uid)), client.InNamespace(defaultTestNs))
		Expect(err).To(BeNil())
		return len(pvcList.Items)
	}, timeout, interval).Should(Equal(0))

	By(fmt.Sprintf("All preflight volume snapshots with uid=%s should be removed from the cluster", uid))
	Eventually(func() int {
		snapshotList := unstructured.UnstructuredList{}
		snapshotList.SetGroupVersionKind(snapshotGVK)
		err = runtimeClient.List(ctx, &snapshotList,
			client.MatchingLabels(getPreflightResourceLabels(uid)), client.InNamespace(defaultTestNs))
		Expect(err).To(BeNil())
		return len(snapshotList.Items)
	}, timeout, interval).Should(Equal(0))
}

func assertSuccessCleanupAll(outputLog string) {
	Expect(outputLog).To(ContainSubstring("All preflight resources cleaned"))

	By("All preflight pods should be removed from cluster")
	Eventually(func() int {
		podList := unstructured.UnstructuredList{}
		podList.SetGroupVersionKind(podGVK)
		err = runtimeClient.List(ctx, &podList,
			client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
		Expect(err).To(BeNil())
		return len(podList.Items)
	}, timeout, interval).Should(Equal(0))

	By("All preflight PVCs should be removed from cluster")
	Eventually(func() int {
		pvcList := unstructured.UnstructuredList{}
		pvcList.SetGroupVersionKind(pvcGVK)
		err = runtimeClient.List(ctx, &pvcList,
			client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
		Expect(err).To(BeNil())
		return len(pvcList.Items)
	}, timeout, interval).Should(Equal(0))

	By("All preflight volume snapshots should be removed from the cluster")
	Eventually(func() int {
		snapshotList := unstructured.UnstructuredList{}
		snapshotList.SetGroupVersionKind(snapshotGVK)
		err = runtimeClient.List(ctx, &snapshotList,
			client.MatchingLabels(getPreflightResourceLabels("")), client.InNamespace(defaultTestNs))
		Expect(err).To(BeNil())
		return len(snapshotList.Items)
	}, timeout, interval).Should(Equal(0))
}

func assertPodScheduleSuccess(outputLog, nodeName string) {
	Expect(outputLog).To(MatchRegexp(
		fmt.Sprintf("Pod - 'dnsutils-[a-z]{6}' scheduled on node - '%s'", nodeName)))
	Expect(outputLog).To(MatchRegexp(
		fmt.Sprintf("Pod - 'source-pod-[a-z]{6}' scheduled on node - '%s'", nodeName)))
	Expect(outputLog).To(MatchRegexp(
		fmt.Sprintf("Pod - 'restored-pod-[a-z]{6}' scheduled on node - '%s'", nodeName)))
	Expect(outputLog).To(MatchRegexp(
		fmt.Sprintf("Pod - 'unmounted-restored-pod-[a-z]{6}' scheduled on node - '%s'", nodeName)))
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
			Resources:       resourceReqs,
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
			Resources: resourceReqs,
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

func createNamespace(namespace string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err = k8sClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

func deleteNamespace(namespace string) {
	err = k8sClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

// copy map 'from' to 'to' key-by-key and value-by-value
func copyMap(from, to map[string]string) {
	for key, value := range from {
		to[key] = value
	}
}

// Executes the preflight binary in terminal
func runPreflightChecks(flagsMap map[string]string) (cmdOut *shell.CmdOut, err error) {
	var flags string

	for key, val := range flagsMap {
		if key == cleanupOnFailureFlag {
			flags = strings.Join([]string{flags, cleanupOnFailureFlag}, spaceSeparator)
		} else {
			flags = strings.Join([]string{flags, key, val}, spaceSeparator)
		}
	}

	cmd := fmt.Sprintf("%s run %s", preflightBinaryFilePath, flags)
	tLog.Infof("Preflight check CMD [%s]", cmd)
	cmdOut, err = shell.RunCmd(cmd)
	tLog.Infof("Preflight binary run execution output: %s", cmdOut.Out)
	return cmdOut, err
}

// Executes cleanup for a particular preflight run
func runCleanupWithUID(uid string) (cmdOut *shell.CmdOut, err error) {
	cmd := fmt.Sprintf("%s cleanup --uid %s -n %s -k %s",
		preflightBinaryFilePath, uid, defaultTestNs, kubeConfPath)
	tLog.Infof("Preflight cleanup CMD [%s]", cmd)
	cmdOut, err = shell.RunCmd(cmd)
	tLog.Infof("Preflight binary cleanup execution output: %s", cmdOut.Out)
	return cmdOut, err
}

// Executes cleanup for all preflight resources
func runCleanupForAllPreflightResources() (cmdOut *shell.CmdOut, err error) {
	cmd := fmt.Sprintf("%s cleanup -n %s -k %s", preflightBinaryFilePath, defaultTestNs, kubeConfPath)
	tLog.Infof("Preflight cleanup CMD [%s]", cmd)
	cmdOut, err = shell.RunCmd(cmd)
	tLog.Infof("Preflight binary cleanup all execution output: %s", cmdOut.Out)
	return cmdOut, err
}

func createPreflightServiceAccount() {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      preflightSAName,
			Namespace: defaultTestNs,
		},
	}
	err = runtimeClient.Create(ctx, sa)
	Expect(err).To(BeNil())
}

func deletePreflightServiceAccount() {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      preflightSAName,
			Namespace: defaultTestNs,
		},
	}
	err = runtimeClient.Delete(ctx, sa)
	Expect(err).To(BeNil())
}

func createAffineBusyboxPod(podName, affinity, namespace string) {
	pod := getPodTemplate(podName, "")
	podLabels := pod.GetLabels()
	podLabels[preflightPodAffinityKey] = affinity
	pod.SetLabels(podLabels)
	pod.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    preflight.BusyboxContainerName,
				Image:   preflight.BusyboxImageName,
				Command: preflight.CommandBinSh,
			},
		},
		NodeSelector: map[string]string{
			preflightNodeLabelKey: preflightNodeLabelValue,
		},
	}

	_, err = k8sClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

func deleteAffineBusyboxPod(podName, namespace string) {
	err = k8sClient.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	Expect(err).To(BeNil())

	Eventually(func() error {
		_, err = k8sClient.CoreV1().Pods(defaultTestNs).Get(ctx, podName, metav1.GetOptions{})
		return err
	}, timeout, interval).ShouldNot(BeNil())
}
