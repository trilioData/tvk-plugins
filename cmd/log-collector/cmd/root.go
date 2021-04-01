package cmd

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE

	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

const (
	binaryName     = "log-collector"
	clusteredFlag  = "clustered"
	namespacesFlag = "namespaces"
	kubeConfigFlag = "kubeconfig"
	noCleanFlag    = "no-clean"
	logLevelFlag   = "log-level"

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
	kubeconfigUsage   = "specifies the custom path for your kubeconfig"
	kubeconfigDefault = "~/.kube/config"
	kubeconfigShort   = "k"
	noCleanUsage      = "Do not Clean the Directory ( Keep both directory and Zip )"
	nocleanDefault    = false
	nocleanShort      = "e"
	logLevelUsage     = "LogLevel specify the logging level the logger should log at. This is typically info " +
		", error, fatal."
	loglevelDefault = "INFO"
	loglevelShort   = "l"
)

var (
	rootCmd           *cobra.Command
	clustered         bool
	namespaces        []string
	kubeConfig        string
	noClean           bool
	logLevel          string
	namespacesDefault []string
)

func init() {
	rootCmd = logCollectorCommand()
}

func logCollectorCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   binaryName,
		Short: shortUsage,
		Long:  longUsage,
		RunE:  runLogCollector,
	}

	cmd.Flags().StringSliceVarP(&namespaces, namespacesFlag, namespacesShort, namespacesDefault, namespacesUsage)
	cmd.Flags().BoolVarP(&clustered, clusteredFlag, clusteredShort, clusteredDefault, clusteredUsage)
	cmd.Flags().StringVarP(&kubeConfig, kubeConfigFlag, kubeconfigShort, kubeconfigDefault, kubeconfigUsage)
	cmd.Flags().BoolVarP(&noClean, noCleanFlag, nocleanShort, nocleanDefault, noCleanUsage)
	cmd.Flags().StringVarP(&logLevel, logLevelFlag, loglevelShort, loglevelDefault, logLevelUsage)

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
		OutputDir:   "triliovault-" + formatted,
		CleanOutput: !noClean,
		Clustered:   clustered,
		Namespaces:  namespaces,
		Loglevel:    logLevel,
	}
	err := logCollector.CollectLogsAndDump()
	if err != nil {
		return err
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Unable to execute log-collector : %v", err)
	}
}
