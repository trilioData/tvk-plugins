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
  kubectl rvk-preflight run --storage-class <storage-class-name> --volume-snapshot-class <snapshot-class-name>

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
`,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		logFile, err = os.OpenFile(preflightLogFilename, os.O_APPEND|os.O_WRONLY, filePermission)
		if err != nil {
			log.Fatalf("Failed to open preflight log file :: %s", err.Error())
		}
		defer logFile.Close()
		logger.SetOutput(io.MultiWriter(colorable.NewColorableStdout(), logFile))
		logRootCmdFlagsInfo()
		if storageClass == "" {
			logger.Fatalf("storage-class is required, cannot be empty")
		}
		if imagePullSecret != "" && localRegistry == "" {
			logger.Fatalf("Cannot give image pull secret if local registry is not provided.\nUse --local-registry flag to provide local registry")
		}
		op := &preflight.Options{
			CommonOptions: preflight.CommonOptions{
				Kubeconfig: kubeconfig,
				Namespace:  namespace,
				Logger:     logger,
			},
			StorageClass:         storageClass,
			SnapshotClass:        snapshotClass,
			LocalRegistry:        localRegistry,
			ImagePullSecret:      imagePullSecret,
			ServiceAccountName:   serviceAccount,
			PerformCleanupOnFail: cleanupOnFailure,
		}
		op.PerformPreflightChecks(context.Background())
	},

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := setupLogger(preflightLogFilePrefix)
		if err != nil {
			return err
		}
		err = preflight.InitKubeEnv(kubeconfig)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&storageClass, storageClassFlag, "", storageClassUsage)
	runCmd.Flags().StringVar(&snapshotClass, snapshotClassFlag, "", snapshotClassUsage)
	runCmd.Flags().StringVar(&localRegistry, localRegistryFlag, "", localRegistryUsage)
	runCmd.Flags().StringVar(&imagePullSecret, imagePullSecFlag, "", imagePullSecUsage)
	runCmd.Flags().StringVar(&serviceAccount, serviceAccountFlag, "", serviceAccountUsage)
	runCmd.Flags().BoolVar(&cleanupOnFailure, cleanupOnFailureFlag, false, cleanupOnFailureUsage)

	err := runCmd.MarkFlagRequired(storageClassFlag)
	if err != nil {
		log.Fatalf("Error marking %s flag as required :: %s", storageClassFlag, err.Error())
	}
}
