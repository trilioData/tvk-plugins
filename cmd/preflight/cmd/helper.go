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
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/tools/preflight"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/yaml"
)

type preflightCmdOps struct {
	PreflightOps preflight.Options        `json:"preflightOptions"`
	CleanupOps   preflight.CleanupOptions `json:"cleanupOptions"`
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

func logRootCmdFlagsInfo(nspace, kubeConf string) {
	logger.Infof(fmt.Sprintf("Using '%s' namespace of the cluster", nspace))
	logger.Infof(fmt.Sprintf("Using kubeconfig file path - %s", kubeConf))
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
	if inputFileName != "" {
		err = readFileInputOptions(inputFileName)
		if err != nil {
			return fmt.Errorf("failed to read preflight input from file :: %s", err.Error())
		}
		overridePreflightFileInputsFromCLI(cmd)
	} else {
		cmdOps.PreflightOps = preflight.Options{
			CommonOptions: preflight.CommonOptions{
				Kubeconfig: kubeconfig,
				Namespace:  namespace,
				LogLevel:   logLevel,
			},
			StorageClass:         storageClass,
			SnapshotClass:        snapshotClass,
			LocalRegistry:        localRegistry,
			ImagePullSecret:      imagePullSecret,
			ServiceAccountName:   serviceAccount,
			PerformCleanupOnFail: cleanupOnFailure,
		}
		updateResReqFromCLI(cmd)
	}

	return nil
}

func overridePreflightFileInputsFromCLI(cmd *cobra.Command) {
	updateCommonInputsFromCLI(cmd, &cmdOps.PreflightOps.CommonOptions)

	if cmd.Flags().Changed(storageClassFlag) {
		cmdOps.PreflightOps.StorageClass = storageClass
	}
	if cmd.Flags().Changed(snapshotClassFlag) {
		cmdOps.PreflightOps.SnapshotClass = snapshotClass
	}
	if cmd.Flags().Changed(localRegistryFlag) {
		cmdOps.PreflightOps.LocalRegistry = localRegistry
	}
	if cmd.Flags().Changed(imagePullSecFlag) {
		cmdOps.PreflightOps.ImagePullSecret = imagePullSecret
	}
	if cmd.Flags().Changed(serviceAccountFlag) {
		cmdOps.PreflightOps.ServiceAccountName = serviceAccount
	}
	if cmd.Flags().Changed(cleanupOnFailureFlag) {
		cmdOps.PreflightOps.PerformCleanupOnFail = cleanupOnFailure
	}

	updateResReqFromCLI(cmd)
}

func updateCommonInputsFromCLI(cmd *cobra.Command, comnOps *preflight.CommonOptions) {
	if cmd.Flags().Changed(namespaceFlag) || comnOps.Namespace == "" {
		comnOps.Namespace = namespace
	}
	if cmd.Flags().Changed(internal.KubeconfigFlag) || comnOps.Kubeconfig == "" {
		comnOps.Kubeconfig = kubeconfig
	}
	if cmd.Flags().Changed(internal.LogLevelFlag) || comnOps.LogLevel == "" {
		comnOps.LogLevel = logLevel
	}
}

func updateResReqFromCLI(cmd *cobra.Command) {
	if cmd.Flags().Changed(requestMemoryFlag) {
		if cmdOps.PreflightOps.PodResourceRequirements.Requests == nil {
			cmdOps.PreflightOps.PodResourceRequirements.Requests = corev1.ResourceList{}
		}
		cmdOps.PreflightOps.PodResourceRequirements.Requests["memory"] = resource.MustParse(requestMemory)
	}
	if cmd.Flags().Changed(limitMemoryFlag) {
		if cmdOps.PreflightOps.PodResourceRequirements.Limits == nil {
			cmdOps.PreflightOps.PodResourceRequirements.Limits = corev1.ResourceList{}
		}
		cmdOps.PreflightOps.PodResourceRequirements.Limits["memory"] = resource.MustParse(limitMemory)
	}
	if cmd.Flags().Changed(requestCPUFlag) {
		if cmdOps.PreflightOps.PodResourceRequirements.Requests == nil {
			cmdOps.PreflightOps.PodResourceRequirements.Requests = corev1.ResourceList{}
		}
		cmdOps.PreflightOps.PodResourceRequirements.Requests["cpu"] = resource.MustParse(requestCPU)
	}
	if cmd.Flags().Changed(limitCPUFlag) {
		if cmdOps.PreflightOps.PodResourceRequirements.Limits == nil {
			cmdOps.PreflightOps.PodResourceRequirements.Limits = corev1.ResourceList{}
		}
		cmdOps.PreflightOps.PodResourceRequirements.Limits["cpu"] = resource.MustParse(limitCPU)
	}
}

func manageCleanupInputs(cmd *cobra.Command) (err error) {
	if inputFileName != "" {
		err = readFileInputOptions(inputFileName)
		if err != nil {
			return err
		}
		overrideCleanupFileInputsFromCLI(cmd)
	} else {
		cmdOps.CleanupOps = preflight.CleanupOptions{
			CommonOptions: preflight.CommonOptions{
				Kubeconfig: kubeconfig,
				Namespace:  namespace,
				LogLevel:   logLevel,
			},
		}
	}

	return nil
}

func validateResourceRequirementsField() error {
	reqMem := cmdOps.PreflightOps.PodResourceRequirements.Requests.Memory()
	limitMem := cmdOps.PreflightOps.PodResourceRequirements.Limits.Memory()
	if (reqMem.Value() == 0 && limitMem.Value() != 0) || (reqMem.Value() != 0 && limitMem.Value() == 0) {
		return fmt.Errorf("non-zero memory requirement must be specified or skipped for both requests and limits. " +
			"Memory requirement for only request or limit should not be specified")
	}
	if (reqMem != nil && limitMem != nil) && (reqMem.Value() > limitMem.Value()) {
		return fmt.Errorf("request memory cannot be greater than limit memory")
	}

	reqCPU := cmdOps.PreflightOps.PodResourceRequirements.Requests.Cpu()
	limitCPU := cmdOps.PreflightOps.PodResourceRequirements.Limits.Cpu()
	if (reqCPU.Value() == 0 && limitCPU.Value() != 0) || (reqCPU.Value() != 0 && limitCPU.Value() == 0) {
		return fmt.Errorf("non-zero CPU requirement must be specified or skipped for both requests and limits. " +
			"CPU requirement for only request or limit should not be specified")
	}
	if (reqCPU != nil && limitCPU != nil) && (reqCPU.AsApproximateFloat64() > limitCPU.AsApproximateFloat64()) {
		return fmt.Errorf("request CPU cannot be greater than limit CPU")
	}

	return nil
}

func validateCleanupFields() error {
	if cmdOps.CleanupOps.CleanupMode == uidCleanupMode && len(cmdOps.CleanupOps.UID) != preflightUIDLength {
		return fmt.Errorf("valid 6-length preflight UID must be specified when cleanup mode is %s", uidCleanupMode)
	}

	return nil
}

func overrideCleanupFileInputsFromCLI(cmd *cobra.Command) {
	updateCommonInputsFromCLI(cmd, &cmdOps.CleanupOps.CommonOptions)

	if cmd.Flags().Changed(cleanupModeFlag) || cmdOps.CleanupOps.CleanupMode == "" {
		cmdOps.CleanupOps.CleanupMode = defaultCleanupMode
	}
	if cmdOps.CleanupOps.CleanupMode == uidCleanupMode && cmd.Flags().Changed(uidFlag) {
		cmdOps.CleanupOps.UID = cleanupUID
	}
}
