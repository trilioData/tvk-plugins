package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/reporters/stenographer/support/go-colorable"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

type preflightCmdOps struct {
	Run     preflight.Run     `json:"run"`
	Cleanup preflight.Cleanup `json:"cleanup"`
}

func setupLogger(logFilePrefix, logLvl string) error {
	var err error
	preflightLogFilename = generateLogFileName(logFilePrefix)
	logFile, err = os.OpenFile(preflightLogFilename, os.O_CREATE|os.O_WRONLY, filePermission)
	if err != nil {
		log.Errorf("Unable to create log file - %s. Aborting preflight checks...", preflightLogFilename)
		return err
	}
	defer logFile.Close()
	logger.SetOutput(io.MultiWriter(colorable.NewColorableStdout(), logFile))
	logger.Infof("Created log file with name - %s", logFile.Name())
	lvl, err := log.ParseLevel(logLvl)
	if err != nil {
		logger.SetLevel(log.InfoLevel)
		logger.Errorf("Failed to parse log-level flag. Setting log level as %s\n", internal.DefaultLogLevel)
		return nil
	}
	logger.Infof("Setting log level as %s\n", strings.ToLower(logLvl))
	logger.SetLevel(lvl)

	return nil
}

func generateLogFileName(logFilePrefix string) string {
	year, month, day := time.Now().Date()
	hour, minute, sec := time.Now().Clock()
	ts := strconv.Itoa(year) + "-" + strconv.Itoa(int(month)) + "-" + strconv.Itoa(day) +
		"T" + strconv.Itoa(hour) + "-" + strconv.Itoa(minute) + "-" + strconv.Itoa(sec)

	return logFilePrefix + "-" + ts + ".log"
}

// reads preflight and cleanup inputs from file
// override the flag inputs of file if given through CLI
func readFileInputOptions(filename string) error {
	var data []byte
	data, err = ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.UnmarshalStrict(data, &cmdOps)
	if err != nil {
		return err
	}

	return nil
}

func managePreflightInputs(cmd *cobra.Command) (err error) {
	setResReqDefaultValues()
	if inputFileName != "" {
		err = readFileInputOptions(inputFileName)
		if err != nil {
			return fmt.Errorf("failed to read preflight input from file :: %s", err.Error())
		}
	}

	return overridePreflightFileInputsFromCLI(cmd)
}

func overridePreflightFileInputsFromCLI(cmd *cobra.Command) error {
	updateCommonInputsFromCLI(cmd, &cmdOps.Run.CommonOptions)

	if cmd.Flags().Changed(StorageClassFlag) {
		cmdOps.Run.StorageClass = storageClass
	}
	if cmd.Flags().Changed(SnapshotClassFlag) {
		cmdOps.Run.SnapshotClass = snapshotClass
	}
	if cmd.Flags().Changed(LocalRegistryFlag) {
		cmdOps.Run.LocalRegistry = localRegistry
	}
	if cmd.Flags().Changed(imagePullSecFlag) {
		cmdOps.Run.ImagePullSecret = imagePullSecret
	}
	if cmd.Flags().Changed(ServiceAccountFlag) {
		cmdOps.Run.ServiceAccountName = serviceAccount
	}
	if cmd.Flags().Changed(CleanupOnFailureFlag) {
		cmdOps.Run.PerformCleanupOnFail = cleanupOnFailure
	}
	if cmd.Flags().Changed(PVCStorageRequestFlag) {
		cmdOps.Run.PVCStorageRequest = resource.MustParse(pvcStorageRequest)
	} else if cmdOps.Run.PVCStorageRequest.Value() == 0 {
		cmdOps.Run.PVCStorageRequest = resource.MustParse(DefaultPVCStorage)
	}

	err = updateNodeSelectorLabelsFromCLI(cmd)
	if err != nil {
		log.Fatalf("problem updating node selector labels :: %s", err.Error())
	}
	return updateResReqFromCLI()
}

func updateCommonInputsFromCLI(cmd *cobra.Command, comnOps *preflight.CommonOptions) {
	if cmd.Flags().Changed(NamespaceFlag) || comnOps.Namespace == "" {
		comnOps.Namespace = namespace
	}
	if cmd.Flags().Changed(internal.KubeconfigFlag) || comnOps.Kubeconfig == "" {
		comnOps.Kubeconfig = kubeconfig
	}
	if cmd.Flags().Changed(internal.LogLevelFlag) || comnOps.LogLevel == "" {
		comnOps.LogLevel = logLevel
	}
	if cmd.Flags().Changed(InClusterFlag) || comnOps.InCluster {
		comnOps.InCluster = inCluster
	}
}

// updateResReqFromCLI update the pod resource requirements from CLI
// if pod resource requirements are not set from file or CLI then, default values are set
func updateResReqFromCLI() error {
	var (
		requests corev1.ResourceList
		limits   corev1.ResourceList
	)

	if podRequests != "" {
		requests, err = populateResourceList(podRequests)
		if err != nil {
			return err
		}

		if requests.Cpu().Value() != 0 {
			cmdOps.Run.Requests[corev1.ResourceCPU] = requests[corev1.ResourceCPU]
		}
		if requests.Memory().Value() != 0 {
			cmdOps.Run.Requests[corev1.ResourceMemory] = requests[corev1.ResourceMemory]
		}
	}

	if podLimits != "" {
		limits, err = populateResourceList(podLimits)
		if err != nil {
			return err
		}

		if limits.Cpu().Value() != 0 {
			cmdOps.Run.Limits[corev1.ResourceCPU] = limits[corev1.ResourceCPU]
		}
		if limits.Memory().Value() != 0 {
			cmdOps.Run.Limits[corev1.ResourceMemory] = limits[corev1.ResourceMemory]
		}
	}

	return nil

}

func populateResourceList(resourceStr string) (corev1.ResourceList, error) {
	var (
		rs            = corev1.ResourceList{}
		resourceName  corev1.ResourceName
		resourceValue resource.Quantity
	)
	resourceStatements := strings.Split(resourceStr, ",")
	for _, resourceStatement := range resourceStatements {
		tokens := strings.Split(resourceStatement, "=")
		if len(tokens) != 2 {
			return nil, fmt.Errorf("invalid argument syntax %v, expected format <resource>=<value>", resourceStatement)
		}
		resourceName = corev1.ResourceName(strings.Trim(tokens[0], " "))
		resourceValue, err = resource.ParseQuantity(strings.Trim(tokens[1], " "))
		if err != nil {
			return nil, err
		}
		rs[resourceName] = resourceValue
	}

	return rs, nil
}

func updateNodeSelectorLabelsFromCLI(cmd *cobra.Command) error {
	if !cmd.Flags().Changed(NodeSelectorFlag) {
		return nil
	}
	var nodeSelLabels map[string]string
	nodeSelLabels, err = parseNodeSelectorLabels(nodeSelector)
	if err != nil {
		return err
	}
	cmdOps.Run.PodSchedOps.NodeSelector = nodeSelLabels

	return nil
}

func parseNodeSelectorLabels(labels string) (map[string]string, error) {
	selectorMap := make(map[string]string)
	kvPairs := strings.Split(labels, ",")
	for _, kv := range kvPairs {
		tokens := strings.Split(kv, "=")
		if len(tokens) != 2 {
			return nil, fmt.Errorf("invalid argument syntax %v, expected format <key>=<value>", kv)
		}
		selectorMap[tokens[0]] = tokens[1]
	}

	return selectorMap, nil
}

func manageCleanupInputs(cmd *cobra.Command) (err error) {
	if inputFileName != "" {
		err = readFileInputOptions(inputFileName)
		if err != nil {
			return err
		}
	}
	overrideCleanupFileInputsFromCLI(cmd)

	return nil
}

func validateRunOptions() error {
	if cmdOps.Run.StorageClass == "" {
		logger.Fatalf("storage-class is required, cannot be empty")
	}
	if cmdOps.Run.ImagePullSecret != "" && cmdOps.Run.LocalRegistry == "" {
		logger.Fatalf("Cannot give image pull secret if local registry is not provided.\nUse --local-registry flag to provide local registry")
	}

	reqMem := cmdOps.Run.Requests.Memory()
	limitMem := cmdOps.Run.Limits.Memory()
	if (reqMem != nil && limitMem != nil) && (reqMem.Value() > limitMem.Value()) {
		return fmt.Errorf("request memory cannot be greater than limit memory")
	}

	reqCPU := cmdOps.Run.Requests.Cpu()
	limitCPU := cmdOps.Run.Limits.Cpu()
	if (reqCPU != nil && limitCPU != nil) && (reqCPU.AsApproximateFloat64() > limitCPU.AsApproximateFloat64()) {
		return fmt.Errorf("request CPU cannot be greater than limit CPU")
	}

	return nil
}

func validateCleanupFields() error {
	if cmdOps.Cleanup.UID != "" && len(cmdOps.Cleanup.UID) != preflightUIDLength {
		return fmt.Errorf("valid 6-length preflight UID must be specified")
	}

	return nil
}

func overrideCleanupFileInputsFromCLI(cmd *cobra.Command) {
	updateCommonInputsFromCLI(cmd, &cmdOps.Cleanup.CommonOptions)

	if cmd.Flags().Changed(uidFlag) {
		cmdOps.Cleanup.UID = cleanupUID
	}
}

func setResReqDefaultValues() {
	cmdOps.Run.ResourceRequirements = corev1.ResourceRequirements{}
	cmdOps.Run.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse(DefaultPodRequestCPU),
		corev1.ResourceMemory: resource.MustParse(DefaultPodRequestMemory),
	}
	cmdOps.Run.Limits = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse(DefaultPodLimitCPU),
		corev1.ResourceMemory: resource.MustParse(DefaultPodLimitMemory),
	}
}
