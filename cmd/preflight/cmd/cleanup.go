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

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   cleanupCmdName,
	Short: "Cleans-up the preflight resources created during preflight checks.",
	Long: `Cleans-up the resources that were created during preflight checks.
If uid flag is not specified then all preflight resources created till date are deleted.`,
	Example: ` # clean preflight resources with a particular uid
  kubectl tvk-preflight cleanup --uid <preflight run uid> --namespace <namespace>

  # clean all preflight resources created till date
  kubectl tvk-preflight cleanup --namespace <namespace>

  # clean preflight resource with a specified logging level
  kubectl tvk-preflight cleanup --uid <preflight run uid> --log-level <log-level>

  # cleanup preflight resources with a particular kubeconfig file
  kubectl tvk-preflight cleanup --uid <preflight run uid> --namespace <namespace> --kubeconfig <kubeconfig-file-path>
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		logFile, err = os.OpenFile(preflightLogFilename, os.O_APPEND|os.O_WRONLY, filePermission)
		if err != nil {
			log.Fatalf("Failed to open preflight log file :: %s", err.Error())
		}
		defer logFile.Close()
		logger.SetOutput(io.MultiWriter(colorable.NewColorableStdout(), logFile))
		co := &preflight.CleanupOptions{
			CommonOptions: preflight.CommonOptions{
				Kubeconfig: kubeconfig,
				Namespace:  namespace,
				Logger:     logger,
			},
		}
		err = co.CleanupPreflightResources(context.Background(), cleanupUID)

		return err
	},

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := setupLogger(cleanupLogFilePrefix)
		if err != nil {
			return err
		}
		return preflight.InitKubeEnv(kubeconfig)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().StringVar(&cleanupUID, uidFlag, "", uidUsage)
}
