package cmd

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/trilioData/tvk-plugins/internal"
	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

const (
	binaryName     = "log-collector"
	clusteredFlag  = "clustered"
	namespacesFlag = "namespaces"
	keepSourceFlag = "keep-source-folder"

	shortUsage = "log-collector collects the information of resources such as yaml configuration and logs from k8s cluster."
	longUsage  = "log-collector let you define what you need to log and how to log it by collecting the the logs " +
		"and events of Pod alongside the metadata of all resources related to TVK as either namespaced by providing " +
		"namespaces name separated by comma or clustered. It also collects the CRDs related to TVK and zip them " +
		"on the path you specify"

	namespacesUsage   = "specifies all the namespaces separated by commas"
	namespacesShort   = "n"
	clusteredUsage    = "specifies clustered object"
	clusteredDefault  = false
	clusteredShort    = "c"
	keepSourceUsage   = "Keep source directory and Zip both"
	keepSourceDefault = false
	keepSourceShort   = "s"
)

var (
	rootCmd           *cobra.Command
	clustered         bool
	namespaces        []string
	kubeConfig        string
	keepSource        bool
	logLevel          string
	namespacesDefault []string
)

func init() {
	rootCmd = logCollectorCommand()
}

func logCollectorCommand() *cobra.Command {
	var cmd = &cobra.Command{
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

	return cmd
}

func runLogCollector(*cobra.Command, []string) error {
	log.Info("---------    LOG COLLECTOR    --------- ")

	if !clustered && len(namespaces) == 0 {
		log.Fatalf("Either clustered option or at least one namespace in namespace option should be specified for collecting logs")
	}

	t := time.Now()
	formatted := fmt.Sprintf("%d-%02d-%02dT%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())

	logCollector := logcollector.LogCollector{
		KubeConfig:  kubeConfig,
		OutputDir:   "triliovault-" + formatted,
		CleanOutput: !keepSource,
		Clustered:   clustered,
		Namespaces:  namespaces,
		Loglevel:    logLevel,
	}
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
	// Setting Log Level
	level, lErr := log.ParseLevel(logLevel)
	if lErr != nil {
		log.Fatalf("Unable to Parse Log Level : %s", lErr.Error())
	}
	log.SetLevel(level)

	if len(namespaces) != 0 && clustered {
		log.Fatalf("Cannot use flag %s and %s scope at the same time", namespacesFlag, clusteredFlag)
	}
	return nil
}
