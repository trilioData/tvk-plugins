package cmd

import (
	"context"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/trilioData/tvk-plugins/tools/preflight"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   preflightRunCmdName,
	Short: "Runs preflight checks on cluster",
	Long:  `Runs all the preflight checks on cluster in a sequential manner`,
	Example: ` # run preflight checks
  preflight run --storageclass <storage-class-name>

  # run preflight in a particular namespace
  preflight run --storageclass <storage-class-name> --namespace <namespace>

  # run preflight in a with log level
  preflight run --storageclass <storage-class-name> --log-level <log-level>

  # cleanup the resources generated during preflight check if preflight check fails. Default is false.
  # If the preflight check is successful, then all resources are cleaned.
  preflight run --storageclass <storage-class-name> --cleanup-on-failure 

  # run preflight with a particular kubeconfig
  preflight run --storageclass <storage-class-name> --kubeconfig <kubeconfig file path>

  # run preflight with local registry and image pull secret
  To use image-pull-secret, local-registry flag must be specified. vice-versa is not true
  preflight run --storageclass <storage-class-name> --local-registry <local registry path> --image-pull-secret <image pull secret>

  # run preflight with serviceaccount
  preflight run --storageclass <storage-class-name> --service-account-name <service account name>
`,
	Run: func(cmd *cobra.Command, args []string) {
		defer logFile.Close()
		if imagePullSecret != "" && localRegistry == "" {
			logger.Fatalf("Cannot give image pull secret if local registry is not provided.\nUse --local-registry flag to provide local registry")
		}
		op := &preflight.Options{
			Context:              context.Background(),
			StorageClass:         storageClass,
			SnapshotClass:        snapshotClass,
			Kubeconfig:           kubeconfig,
			Namespace:            namespace,
			LocalRegistry:        localRegistry,
			ImagePullSecret:      imagePullSecret,
			ServiceAccountName:   serviceAccount,
			PerformCleanupOnFail: cleanupOnFailure,
			Logger:               logger,
		}
		op.PerformPreflightChecks()
	},

	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		logFile, err = preflight.CreateLoggingFile(preflightLogFilePrefix)
		if err != nil {
			log.Errorln("Unable to create log file. Aborting preflight checks...")
			return err
		}
		logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
		logger.SetLevel(log.Level(getLogLevelFromString(logLevel)))

		return preflight.InitKubeEnv(kubeconfig)
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
