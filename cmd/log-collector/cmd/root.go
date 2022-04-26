package cmd

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/trilioData/tvk-plugins/internal"
)

func init() {
	rootCmd = logCollectorCommand()
}

func logCollectorCommand() *cobra.Command {
	cmd = &cobra.Command{
		Use:               binaryName,
		Short:             shortUsage,
		Long:              longUsage,
		RunE:              runLogCollector,
		PersistentPreRunE: preRun,
	}
	cmd.Flags().StringSliceVarP(&namespaces, namespacesFlag, namespacesShort, namespacesDefault, namespacesUsage)
	cmd.Flags().BoolVarP(&clustered, clusteredFlag, clusteredShort, clusteredDefault, clusteredUsage)
	cmd.Flags().StringVarP(&kubeConfig, internal.KubeconfigFlag,
		internal.KubeconfigShorthandFlag, internal.KubeConfigDefault, internal.KubeconfigUsage)
	cmd.Flags().BoolVarP(&keepSource, keepSourceFlag, keepSourceShort, keepSourceDefault, keepSourceUsage)
	cmd.Flags().StringVarP(&logLevel, internal.LogLevelFlag, internal.LogLevelFlagShorthand, internal.DefaultLogLevel, internal.LogLevelUsage)
	cmd.Flags().StringVarP(&inputFileName, configFileFlag, configFlagShorthand, "", configFileUsage)
	cmd.Flags().StringSliceVarP(&gvkSlice, gvkFlag, gvkFlagShorthand, []string{}, gvkUsage)
	cmd.Flags().StringSliceVarP(&labelSlice, labelsFlag, labelsFlagShorthand, []string{}, labelsUsage)

	return cmd
}

func runLogCollector(*cobra.Command, []string) error {
	log.Info("---------    LOG COLLECTOR    --------- ")

	err := logCollector.CollectLogsAndDump()
	if err != nil {
		log.Fatalf("Log collection failed - %s", err.Error())
	}

	log.Info("---------    FINISHED COLLECTING LOGS    --------- ")
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Unable to execute log-collector : %v", err)
	}
}

// preRun runs just before the run for any pre checks and setting up vars
func preRun(*cobra.Command, []string) error {

	err := logCollector.InitializeKubeClients()
	if err != nil {
		return err
	}

	// manage values file inputs
	iErr := manageFileInputs()
	if iErr != nil {
		log.Fatalf("%s", iErr.Error())
	}

	// Setting Log Level
	level, lErr := log.ParseLevel(logCollector.Loglevel)
	if lErr != nil {
		log.Fatalf("Unable to Parse Log Level : %s", lErr.Error())
	}
	log.SetLevel(level)

	if len(logCollector.Namespaces) != 0 && logCollector.Clustered {
		log.Fatalf("Cannot use flag %s and %s scope at the same time", namespacesFlag, clusteredFlag)
	}

	if !logCollector.Clustered && len(logCollector.Namespaces) == 0 {
		logCollector.Namespaces = append(logCollector.Namespaces, defaultNamespace)
	}

	t := time.Now()
	formatted := fmt.Sprintf("%d-%02d-%02dT%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	logCollector.OutputDir = "triliovault-" + formatted

	return nil
}
