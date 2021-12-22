package cmd

import (
	"context"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/trilioData/tvk-plugins/tools/preflight"
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   cleanupCmdName,
	Short: "Cleans-up the preflight resources created during preflight checks.",
	Long:  `Cleans-up the resources that were created during preflight checks. Can delete all the .`,
	Example: ` # run cleanup
  preflight cleanup --uid <preflight run uid> --namespace <namespace>

  # run cleanup with logging level
  preflight cleanup --uid <preflight run uid> --log-level

  # run cleanup with a particular kubeconfig file
  preflight cleanup --uid <preflight run uid> --namespace <namespace> --kubeconfig <kubeconfig file path>
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		defer logFile.Close()
		var err error
		co := &preflight.CleanupOptions{
			Ctx:        context.Background(),
			Kubeconfig: kubeconfig,
			Namespace:  namespace,
			Logger:     logger,
		}
		if cleanupUID != "" {
			err = co.CleanupByUID(cleanupUID)
		} else {
			err = co.CleanAllPreflightResources()
		}

		return err
	},

	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		logFile, err = preflight.CreateLoggingFile(cleanupLogFilePrefix)
		if err != nil {
			log.Errorln("Unable to create log file. Aborting resource cleanup...")
			return err
		}
		logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
		logger.SetLevel(log.Level(getLogLevelFromString(logLevel)))

		return preflight.InitKubeEnv(kubeconfig)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().StringVar(&cleanupUID, uidFlag, "", uidUsage)
}
