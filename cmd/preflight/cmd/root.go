package cmd

import (
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/trilioData/tvk-plugins/internal"
)

var (
	logger               *logrus.Logger
	logFile              *os.File
	preflightLogFilename string
	cmdOps               preflightCmdOps
	err                  error
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
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, internal.KubeconfigFlag,
		internal.KubeconfigShorthandFlag, internal.KubeConfigDefault, internal.KubeconfigUsage)
	rootCmd.PersistentFlags().StringVarP(&namespace, namespaceFlag, namespaceFlagShorthand, internal.DefaultNs, namespaceUsage)
	rootCmd.PersistentFlags().StringVarP(&logLevel, internal.LogLevelFlag,
		internal.LogLevelFlagShorthand, internal.DefaultLogLevel, internal.LogLevelUsage)
	rootCmd.PersistentFlags().StringVarP(&inputFileName, configFileFlag, configFlagShorthand, "", configFileUsage)

	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
}
