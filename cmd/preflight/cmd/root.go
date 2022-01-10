package cmd

import (
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/trilioData/tvk-plugins/internal"
)

var (
	logger  *logrus.Logger
	logFile *os.File
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   preflightCmdName,
	Short: "A kubectl plugin to do preflight checks for installation of Triliovault for kubernetes",
	Long: `tvk-preflight is a kubectl plugin which checks if all the pre-requisites are met before installing Triliovault for Kubernetes
application in a Kubernetes cluster.
`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln("preflight command execution failed -", err.Error())
	}
}

//  initializes flags and logger for the application
func init() {
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, kubeconfigFlag, kubeconfigShorthandFlag, internal.KubeConfigDefault, kubeconfigUsage)
	rootCmd.PersistentFlags().StringVarP(&namespace, namespaceFlag, namespaceFlagShorthand, internal.DefaultNs, namespaceUsage)
	rootCmd.PersistentFlags().StringVarP(&logLevel, logLevelFlag, logLevelFlagShorthand, defaultLogLevel, logLevelUsage)

	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
}
