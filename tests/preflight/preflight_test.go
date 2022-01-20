package preflighttest

import (
	"crypto/rand"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Preflight Tests", func() {

	Context("When checks are successful and volume snapshot class value is not provided as input", func() {
		var (
			once          sync.Once
			outputLogData string
		)
		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				runPreflightChecks(flagsMap)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Should be able fetch volume snapshot class with its driver matching storage class' provisioner", func() {
			Expect(outputLogData).
				To(MatchRegexp("(Extracted volume snapshot class -)(.*)(found in cluster)"))
			Expect(outputLogData).
				To(MatchRegexp("(Volume snapshot class -)(.*)(driver matches with given StorageClass's provisioner)"))
		})
	})

	Context("When preflight checks are successful and volume snapshot class value is provided as input", func() {
		var (
			once          sync.Once
			outputLogData string
			inputFlags    = make(map[string]string)
		)
		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = defaultTestSnapshotClass
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Should match volume snapshot class driver to storage class provisioner", func() {
			Expect(outputLogData).To(
				MatchRegexp("(Volume snapshot class -)(.*)( driver matches with given storage class provisioner)"))
		})
	})

	Context("When preflight checks are executed successfully", func() {
		var outputLogData string
		var once sync.Once

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				runPreflightChecks(flagsMap)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Should be able find kubectl binary on the system", func() {
			Expect(outputLogData).To(ContainSubstring("kubectl found at path - "))
			Expect(outputLogData).To(ContainSubstring("Preflight check for kubectl utility is successful"))
		})

		It("Should be able to access cluster", func() {
			Expect(outputLogData).To(ContainSubstring("Preflight check for kubectl access is successful"))
		})

		if discClient != nil && internal.CheckIsOpenshift(discClient, ocpAPIVersion) {
			It("Should be able find Openshift groupVersion on cluster", func() {
				Expect(outputLogData).To(ContainSubstring("Running OCP cluster. Helm not needed for OCP clusters"))
			})
		} else {
			It("Should be able find Helm on the system", func() {
				Expect(outputLogData).To(ContainSubstring("helm found at path - "))
			})
			It("Helm should meet minimum version requirements", func() {
				var helmVersion string
				helmVersion, err = getHelmVersion()
				Expect(err).To(BeNil())
				Expect(outputLogData).
					To(ContainSubstring(fmt.Sprintf("Helm version %s meets required version", helmVersion)))
			})
		}

		It("Kubernetes server version should meet minimum version requirement", func() {
			Expect(outputLogData).To(ContainSubstring("Preflight check for kubernetes version is successful"))
		})

		It("Kubernetes RBAC should be enabled on the cluster", func() {
			Expect(outputLogData).To(ContainSubstring("Kubernetes RBAC is enabled"))
			Expect(outputLogData).To(ContainSubstring("Preflight check for kubernetes RBAC is successful"))
		})

		It("Should be able to find storage class on cluster", func() {
			storageClass, ok := flagsMap[storageClassFlag]
			Expect(ok).To(BeTrue())
			Expect(outputLogData).
				To(ContainSubstring(fmt.Sprintf("Storageclass - %s found on cluster", storageClass)))
			Expect(outputLogData).To(ContainSubstring("Preflight check for SnapshotClass is successful"))
		})

		It("Should be able to find CSI APIs on cluster", func() {
			for _, api := range csiApis {
				Expect(outputLogData).
					To(ContainSubstring(fmt.Sprintf("Found CSI API - %s on cluster", api)))
			}
			Expect(outputLogData).To(ContainSubstring("Preflight check for CSI is successful"))
		})

		It("Should be able to perform DNS resolution on cluster", func() {
			Expect(outputLogData).To(MatchRegexp("(Pod dnsutils-)([a-z]{6})( created in cluster)"))
			Expect(outputLogData).To(ContainSubstring("Preflight check for DNS resolution is successful"))
		})

		It("Should create snapshot source pvc", func() {
			Expect(outputLogData).To(MatchRegexp("(Created source pvc - source-pvc-)([a-z]{6})"))
		})

		It("Should create snapshot source pod", func() {
			Expect(outputLogData).To(MatchRegexp("(Created source pod - source-pod-)([a-z]{6})"))
		})

		It("Should be able to create volume snapshot from source pvc", func() {
			Expect(outputLogData).To(MatchRegexp("(Created volume snapshot - snapshot-source-pvc-)([a-z]{6})( from source pvc)"))
		})

		It("should be able to create restore pvc from volume snapshot", func() {
			Expect(outputLogData).To(MatchRegexp("(Created restore pvc - restored-pvc-)([a-z]{6})" +
				"( from volume snapshot - snapshot-source-pvc-)([a-z]{6})"))
		})

		It("Should be able to create restore pod from volume snapshot", func() {
			Expect(outputLogData).To(MatchRegexp("(Created restore pod - restored-pod-)([a-z]{6})"))
		})

		It("Should be able to locate data file in container using exec command in restore pod", func() {
			Expect(outputLogData).
				To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
					"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; then exit 0; else exit 1; fi' " +
					"in container - 'busybox' of pod - 'restored-pod"))
		})

		It("Restored pod should have the expected volume snapshot data", func() {
			Expect(outputLogData).To(MatchRegexp("(Restored pod - restored-pod-)([a-z]{6})( has expected data)"))
		})

		It("Should be able to create volume snapshot from unmounted source pvc", func() {
			Expect(outputLogData).To(MatchRegexp("(Created volume snapshot - unmounted-source-pvc-)([a-z]{6})"))
		})

		It("Should be able to create restore pod from volume snapshot of unmounted pv", func() {
			Expect(outputLogData).To(MatchRegexp("(Created restore pod - unmounted-restored-pod-)([a-z]{6})( from volume snapshot of unmounted pv)"))
		})

		It("Should be able to locate data file in container of restore pod created from "+
			"unmounted pvc by using exec command", func() {
			Expect(outputLogData).
				To(ContainSubstring("Command 'exec /bin/sh -c dat=$(cat \"/demo/data/sample-file.txt\"); " +
					"echo \"${dat}\"; if [[ \"${dat}\" == \"pod preflight data\" ]]; " +
					"then exit 0; else exit 1; fi' in container - 'busybox' of pod - 'unmounted-restored-pod"))
		})

		It("Restore pod created from volume snapshot of unmounted pv should have expected source pod data", func() {
			Expect(outputLogData).To(ContainSubstring("restored pod from volume snapshot of unmounted pv has expected data"))
		})

		It("Should clean DNS pod", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning Pod - dnsutils-)([a-z]{6})"))
		})

		It("Should clean volume snapshot restore pod created using unmounted pv", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning Pod - unmounted-restored-pod-)([a-z]{6})"))
		})

		It("Should clean source pod persistent volume claim", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning PersistentVolumeClaim - source-pvc-)([a-z]{6})"))
		})

		It("Should clean restore pod persistent volume claim", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning PersistentVolumeClaim - restored-pvc-)([a-z]{6})"))
		})

		It("Should clean unmounted persistent volume claim created using unmounted pv", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning PersistentVolumeClaim - unmounted-restored-pvc-)([a-z]{6})"))
		})

		It("Should clean source volume snapshot", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning VolumeSnapshot - snapshot-source-pvc-)([a-z]{6})"))
		})

		It("Should clean unmounted volume snapshot", func() {
			Expect(outputLogData).To(MatchRegexp("(Cleaning VolumeSnapshot - unmounted-source-pvc-)([a-z]{6})"))
		})

		It("Should clean all preflight resources", func() {
			Expect(outputLogData).To(ContainSubstring("All preflight resources cleaned"))
		})
	})

	Context("When storage class is not present on cluster", func() {
		var outputLogData string
		var once sync.Once
		var inputFlags = make(map[string]string)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				copyMap(flagsMap, inputFlags)
				inputFlags[storageClassFlag] = invalidStorageClassName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Preflight check for storage snapshot class should fail", func() {
			Expect(outputLogData).To(
				ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
					"not found storageclass - %s on cluster", invalidStorageClassName)))
		})

		It("Should skip preflight check for volume snapshot and restore", func() {
			Expect(outputLogData).
				To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
		})
	})

	Context("When snapshot class is not present on cluster", func() {
		var outputLogData string
		var once sync.Once
		var inputFlags = make(map[string]string)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				copyMap(flagsMap, inputFlags)
				inputFlags[snapshotClassFlag] = invalidSnapshotClassName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Preflight check for storage snapshot class should fail", func() {
			Expect(outputLogData).
				To(ContainSubstring(fmt.Sprintf("volume snapshot class %s not found", invalidSnapshotClassName)))
			Expect(outputLogData).
				To(ContainSubstring(fmt.Sprintf("Preflight check for SnapshotClass failed :: "+
					"volume snapshot class %s not found", invalidSnapshotClassName)))
		})

		It("Should skip preflight check for volume snapshot and restore", func() {
			Expect(outputLogData).
				To(ContainSubstring("Skipping volume snapshot and restore check as preflight check for SnapshotClass failed"))
		})
	})

	Context("When invalid local registry is provided", func() {
		var outputLogData string
		var once sync.Once
		var inputFlags = make(map[string]string)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				copyMap(flagsMap, inputFlags)
				inputFlags[localRegistryFlag] = invalidLocalRegistryName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Should not have DNS pod in a ready state", func() {
			Expect(outputLogData).To(MatchRegexp("(DNS pod - dnsutils-)([a-z]{6})( hasn't reached into ready state)"))
		})

		It("Preflight check DNS resolution should fail", func() {
			Expect(outputLogData).To(ContainSubstring("Preflight check for DNS resolution failed :: timed out waiting for the condition"))
		})

		It("Preflight check for volume snapshot and restore should fail", func() {
			Expect(outputLogData).
				To(MatchRegexp("(Preflight check for volume snapshot and restore failed :: pod source-pod-)" +
					"([a-z]{6})( hasn't reached into ready state)"))
		})
	})

	Context("When service account does not exist on cluster", func() {
		var outputLogData string
		var once sync.Once
		var inputFlags = make(map[string]string)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				copyMap(flagsMap, inputFlags)
				inputFlags[serviceAccountFlag] = invalidServiceAccountName
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Preflight check for DNS resolution should fail", func() {
			Expect(outputLogData).
				To(MatchRegexp(fmt.Sprintf("(Preflight check for DNS resolution failed :: pods \"dnsutils-)([a-z]{6}\")"+
					"( is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found)",
					defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))
		})

		It("Should not be able to create volume snapshot source pod", func() {
			Expect(outputLogData).To(MatchRegexp(
				fmt.Sprintf("(pods \"source-pod-)([a-z]{6})\" is forbidden: error looking up service account %s/%s: serviceaccount \"%s\" not found",
					defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))
		})

		It("Preflight check for volume snapshot and restore should fail", func() {
			Expect(outputLogData).To(MatchRegexp(
				fmt.Sprintf("(Preflight check for volume snapshot and restore failed)(.*)"+
					"(error looking up service account %s/%s: serviceaccount \"%s\" not found)",
					defaultTestNs, invalidServiceAccountName, invalidServiceAccountName)))
		})
	})

	Context("When incorrect log-level is provided as input", func() {
		var outputLogData string
		var once sync.Once
		var inputFlags = make(map[string]string)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				copyMap(flagsMap, inputFlags)
				inputFlags[logLevelFlag] = invalidLogLevel
				runPreflightChecks(inputFlags)
				byteData, err = getLogFileData(preflightLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Should set log-level as INFO", func() {
			Expect(outputLogData).To(ContainSubstring("Failed to parse log-level flag. Setting log level as INFO"))
		})
	})

	Context("Cleanup resources according to preflight UID in a particular namespace", func() {
		var (
			once          sync.Once
			outputLogData string
			uid           string
		)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				uid = createPreflightResourcesForCleanup()
				runCleanupWithUID(uid)
				byteData, err = getLogFileData(cleanupLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It(fmt.Sprintf("Should clean source pod with uid=%s", uid), func() {
			srcPodName := strings.Join([]string{sourcePodNamePrefix, uid}, "")
			Expect(outputLogData).To(ContainSubstring("Cleaning Pod - %s", srcPodName))
		})

		It(fmt.Sprintf("Should clean dns pod with uid=%s", uid), func() {
			dnsPodName := strings.Join([]string{dnsPodNamePrefix, uid}, "")
			Expect(outputLogData).To(ContainSubstring("Cleaning Pod - %s", dnsPodName))
		})

		It(fmt.Sprintf("Should clean source pvc with uid=%s", uid), func() {
			srcPvcName := strings.Join([]string{sourcePVCNamePrefix, uid}, "")
			Expect(outputLogData).To(ContainSubstring("Cleaning PersistentVolumeClaim - %s", srcPvcName))
		})

		It(fmt.Sprintf("Should clean source volume snapshot with uid=%s", uid), func() {
			srcVolSnapName := strings.Join([]string{volSnapshotNamePrefix, uid}, "")
			Expect(outputLogData).To(ContainSubstring("Cleaning VolumeSnapshot - %s", srcVolSnapName))
		})

		It(fmt.Sprintf("Should clean all preflight resources for uid=%s", uid), func() {
			Expect(outputLogData).To(ContainSubstring("All preflight resources cleaned"))
		})
	})

	Context("Cleanup all preflight resources on the cluster in a particular namespace", func() {
		var (
			outputLogData string
			once          sync.Once
		)

		BeforeEach(func() {
			once.Do(func() {
				var byteData []byte
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				_ = createPreflightResourcesForCleanup()
				runCleanupForAllPreflightResources()
				byteData, err = getLogFileData(cleanupLogFilePrefix)
				Expect(err).To(BeNil())
				outputLogData = string(byteData)
			})
		})

		It("Should clean all preflight resources in a particular namespace", func() {
			Expect(outputLogData).To(ContainSubstring("All preflight resources cleaned"))
		})
	})
})

func runPreflightChecks(flagsMap map[string]string) {
	err = cleanDirForFiles(preflightLogFilePrefix)
	Expect(err).To(BeNil())
	var flags string

	for key, val := range flagsMap {
		switch key {
		case storageClassFlag:
			flags += fmt.Sprintf("%s %s ", storageClassFlag, val)

		case namespaceFlag:
			flags += fmt.Sprintf("%s %s ", namespaceFlag, val)

		case snapshotClassFlag:
			flags += fmt.Sprintf("%s %s ", snapshotClassFlag, val)

		case localRegistryFlag:
			flags += fmt.Sprintf("%s %s ", localRegistryFlag, val)

		case imagePullSecFlag:
			flags += fmt.Sprintf("--%s %s ", imagePullSecFlag, val)

		case serviceAccountFlag:
			flags += fmt.Sprintf("%s %s ", serviceAccountFlag, val)

		case cleanupOnFailureFlag:
			flags += fmt.Sprintf("%s ", cleanupOnFailureFlag)

		case logLevelFlag:
			flags += fmt.Sprintf("%s %s", logLevelFlag, val)
		}
	}

	cmd := fmt.Sprintf("%s run %s", preflightBinaryFilePath, flags)
	log.Infof("Preflight check CMD [%s]", cmd)
	_, err = shell.RunCmd(cmd)
	Expect(err).To(BeNil())
}

func runCleanupWithUID(uid string) {
	err = cleanDirForFiles(cleanupLogFilePrefix)
	Expect(err).To(BeNil())
	cmd := fmt.Sprintf("%s cleanup --uid %s", preflightBinaryFilePath, uid)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	_, err = shell.RunCmd(cmd)
	Expect(err).To(BeNil())
}

func runCleanupForAllPreflightResources() {
	err = cleanDirForFiles(cleanupLogFilePrefix)
	Expect(err).To(BeNil())
	cmd := fmt.Sprintf("%s cleanup", preflightBinaryFilePath)
	log.Infof("Preflight cleanup CMD [%s]", cmd)
	_, err = shell.RunCmd(cmd)
	Expect(err).To(BeNil())
}

func cleanDirForFiles(filePrefix string) error {
	var names []fs.FileInfo
	names, err = ioutil.ReadDir(preflightBinaryDir)
	if err != nil {
		return err
	}
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			err = os.RemoveAll(path.Join([]string{preflightBinaryDir, entry.Name()}...))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getLogFileData(filePrefix string) ([]byte, error) {
	var (
		foundFile   = false
		logFilename string
		names       []fs.FileInfo
	)
	names, err = ioutil.ReadDir(preflightBinaryDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			logFilename = entry.Name()
			foundFile = true
			break
		}
	}

	if !foundFile {
		return nil, fmt.Errorf("preflight log file not found")
	}
	return ioutil.ReadFile(logFilename)
}

func getHelmVersion() (string, error) {
	var cmdOut *shell.CmdOut
	cmdOut, err = shell.RunCmd("helm version --template '{{.Version}}'")
	if err != nil {
		return "", err
	}
	helmVersion := cmdOut.Out[2 : len(cmdOut.Out)-1]
	return helmVersion, nil
}

func generatePreflightUID() (string, error) {
	var randNum *big.Int
	uid := make([]byte, 6)
	randRange := big.NewInt(int64(len(letterBytes)))
	for i := range uid {
		randNum, err = rand.Int(rand.Reader, randRange)
		if err != nil {
			return "", err
		}
		idx := randNum.Int64()
		uid[i] = letterBytes[idx]
	}

	return string(uid), nil
}

func createPreflightResourcesForCleanup() string {
	var uid string
	uid, err = generatePreflightUID()
	Expect(err).To(BeNil())
	createPreflightPVC(uid)
	srcPvcName := strings.Join([]string{sourcePVCNamePrefix, uid}, "")
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
			Image:           strings.Join([]string{gcrRegistryPath, dnsUtilsImage}, "/"),
			Command:         commandSleep3600,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       resourceRequirements,
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
	pod := getPodTemplate(sourcePodNamePrefix, preflightUID)
	pod.Spec.Containers = []corev1.Container{
		{
			Name:      busyboxContainerName,
			Image:     busyboxImageName,
			Command:   commandBinSh,
			Args:      argsTouchDataFileSleep,
			Resources: resourceRequirements,
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

func createPreflightPVC(preflightUID string) {
	pvc := createPreflightPVCSpec(preflightUID)
	err = runtimeClient.Create(ctx, pvc)
	Expect(err).To(BeNil())
}

func createPreflightPVCSpec(preflightUID string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{sourcePVCNamePrefix, preflightUID}, ""),
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
	snapshotVersion, err := getServerPreferredVersionForGroup(storageSnapshotGroup)
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
	volSnap.SetName(strings.Join([]string{volSnapshotNamePrefix, preflightUID}, ""))
	volSnap.SetNamespace(defaultTestNs)
	volSnap.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageSnapshotGroup,
		Version: snapshotVersion,
		Kind:    internal.VolumeSnapshotKind,
	})
	volSnap.SetLabels(getPreflightResourceLabels(preflightUID))

	return volSnap
}

func getPodTemplate(name, preflightUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, preflightUID}, ""),
			Namespace: defaultTestNs,
			Labels:    getPreflightResourceLabels(preflightUID),
		},
	}
}

func getPreflightResourceLabels(preflightUID string) map[string]string {
	return map[string]string{
		labelK8sName:         labelK8sNameValue,
		labelTrilioKey:       labelTvkPreflightValue,
		labelPreflightRunKey: preflightUID,
		labelK8sPartOf:       labelK8sPartOfValue,
	}
}

func getServerPreferredVersionForGroup(grp string) (string, error) {
	var (
		apiResList  *metav1.APIGroupList
		err         error
		prefVersion string
	)
	apiResList, err = k8sClient.ServerGroups()
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

func copyMap(from, to map[string]string) {
	for key, value := range from {
		to[key] = value
	}
}
