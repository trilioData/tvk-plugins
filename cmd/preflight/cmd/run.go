package cmd

import (
	"context"
	"io"
	"os"

	"github.com/onsi/ginkgo/reporters/stenographer/support/go-colorable"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

// nolint:lll // ignore long line lint errors
// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   preflightRunCmdName,
	Short: "Runs preflight checks on cluster",
	Long:  `Runs all the preflight checks on cluster in a sequential manner in a particular namespace. If namespace is not provided then 'default' is used.`,
	Example: ` # run preflight checks
  kubectl tvk-preflight run --storage-class <storage-class-name>

  # run preflight checks with a particular volume snapshot class
  kubectl tvk-preflight run --storage-class <storage-class-name> --volume-snapshot-class <snapshot-class-name>

  # run preflight checks in a particular namespace
  kubectl tvk-preflight run --storage-class <storage-class-name> --namespace <namespace>

  # run preflight checks with a particular log level
  kubectl tvk-preflight run --storage-class <storage-class-name> --log-level <log-level>

  # cleanup the resources generated during preflight check if preflight check fails. Default is false.
  # If the preflight check is successful, then all resources are cleaned.
  kubectl tvk-preflight run --storage-class <storage-class-name> --cleanup-on-failure 

  # run preflight with a particular kubeconfig file
  kubectl tvk-preflight run --storage-class <storage-class-name> --kubeconfig <kubeconfig-file-path>

  # run preflight with local registry and image pull secret
  To use image-pull-secret, local-registry flag must be specified. vice-versa is not true
  kubectl tvk-preflight run --storage-class <storage-class-name> --local-registry <local registry path> --image-pull-secret <image pull secret>

  # run preflight with a particular serviceaccount
  kubectl tvk-preflight run --storage-class <storage-class-name> --service-account-name <service account name>

  # run preflight with preflight pod resource request flag
  kubectl tvk-preflight run --storage-class <storage-class-name> --request <resource1>=<value1>,<resource2>=<value2>

  # run preflight with pvc storage request flag for volume snapshot check
  kubectl tvk-preflight run --storage-class <storage-class-name> --pvc-storage-request <storage request value>
`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		err = managePreflightInputs(cmd)
		if err != nil {
			log.Fatal(err.Error())
		}
		err = setupLogger(preflightLogFilePrefix, cmdOps.Run.LogLevel)
		if err != nil {
			log.Fatalf("Failed to setup a logger :: %s", err.Error())
		}
		err = preflight.InitKubeEnv(cmdOps.Run.Kubeconfig)
		if err != nil {
			log.Fatalf("Error initializing kubernetes clients :: %s", err.Error())
		}

		logFile, err = os.OpenFile(preflightLogFilename, os.O_APPEND|os.O_WRONLY, filePermission)
		if err != nil {
			log.Fatalf("Failed to open preflight log file :: %s", err.Error())
		}
		defer logFile.Close()
		logger.SetOutput(io.MultiWriter(colorable.NewColorableStdout(), logFile))
		cmdOps.Run.Logger = logger

		err = validateRunOptions()
		if err != nil {
			logger.Fatalf(err.Error())
		}

		return cmdOps.Run.PerformPreflightChecks(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&storageClass, StorageClassFlag, "", storageClassUsage)
	runCmd.Flags().StringVar(&snapshotClass, SnapshotClassFlag, "", snapshotClassUsage)
	runCmd.Flags().StringVar(&localRegistry, LocalRegistryFlag, "", localRegistryUsage)
	runCmd.Flags().StringVar(&imagePullSecret, imagePullSecFlag, "", imagePullSecUsage)
	runCmd.Flags().StringVar(&serviceAccount, ServiceAccountFlag, "", serviceAccountUsage)
	runCmd.Flags().BoolVar(&cleanupOnFailure, CleanupOnFailureFlag, false, cleanupOnFailureUsage)
	runCmd.Flags().StringVar(&podLimits, PodLimitFlag, "", podLimitUsage)
	runCmd.Flags().StringVar(&podRequests, PodRequestFlag, "", podRequestUsage)
	runCmd.Flags().StringVar(&pvcStorageRequest, PVCStorageRequestFlag, "", pvcStorageRequestUsage)
	runCmd.Flags().StringVar(&nodeSelector, NodeSelectorFlag, "", nodeSelectorUsage)
}
