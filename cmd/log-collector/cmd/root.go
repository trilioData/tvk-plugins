package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

const (
	binaryName        = "log-collector"
	clusteredFlag     = "clustered"
	namespacesFlag    = "namespaces"
	keepSourceFlag    = "keep-source-folder"
	gvkFlag           = "gvk"
	configFileFlag    = "config-file"
	labelSelectorFlag = "label-selector"

	shortUsage = "log-collector collects the information of resources such as yaml configuration and logs from k8s cluster."
	longUsage  = "log-collector let you define what you need to log and how to log it by collecting the the logs " +
		"and events of Pod alongside the metadata of all resources related to TVK as either namespaced by providing " +
		"namespaces name separated by comma or clustered. It also collects the CRDs related to TVK and zip them " +
		"on the path you specify"

	namespacesUsage            = "specifies all the namespaces separated by commas"
	namespacesShort            = "n"
	clusteredUsage             = "specifies clustered object"
	clusteredDefault           = false
	clusteredShort             = "c"
	keepSourceUsage            = "Keep source directory and Zip both"
	keepSourceDefault          = false
	keepSourceShort            = "s"
	configFileUsage            = "specifies the name of the yaml file for inputs to the logCollector flags"
	configFlagShorthand        = "f"
	gvkUsage                   = "specifies the json string of all gvk other than log collector handles by default"
	gvkFlagShorthand           = "g"
	labelSelectorUsage         = "specifies the json string of all gvk other than log collector handles by default"
	labelSelectorFlagShorthand = "r"
)

var (
	rootCmd             *cobra.Command
	clustered           bool
	namespaces          []string
	kubeConfig          string
	keepSource          bool
	logLevel            string
	namespacesDefault   []string
	inputFileName       string
	gvkString           string
	labelSelectorString string
	logCollector        logcollector.LogCollector
	cmd                 *cobra.Command
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
	cmd.Flags().StringVarP(&gvkString, gvkFlag, gvkFlagShorthand, "", gvkUsage)
	cmd.Flags().StringVarP(&labelSelectorString, labelSelectorFlag, labelSelectorFlagShorthand, "", labelSelectorUsage)

	return cmd
}

func runLogCollector(*cobra.Command, []string) error {
	log.Info("---------    LOG COLLECTOR    --------- ")
	t := time.Now()
	formatted := fmt.Sprintf("%d-%02d-%02dT%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	logCollector.OutputDir = "triliovault-" + formatted

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
		log.Fatalf("Either clustered option or at least one namespace in namespace option should be specified for collecting logs")
	}
	return nil
}

func manageFileInputs() (err error) {
	if inputFileName != "" {
		data, err := ioutil.ReadFile(inputFileName)
		if err != nil {
			log.Infof("Unable to read file %s : %s", inputFileName, err.Error())
			return err
		}
		err = yaml.UnmarshalStrict(data, &logCollector)
		if err != nil {
			log.Infof("Unable to unmarshal data %s", err.Error())
			return err
		}
	}
	overrideFileInputsFromCLI()
	return nil
}

// overrideFileInputsFromCLI checks if external flag is given. if yes then override
func overrideFileInputsFromCLI() {

	if cmd.Flags().Changed(internal.KubeconfigFlag) || logCollector.KubeConfig == "" {
		logCollector.KubeConfig = kubeConfig
	}
	if cmd.Flags().Changed(internal.LogLevelFlag) || logCollector.Loglevel == "" {
		logCollector.Loglevel = logLevel
	}
	if cmd.Flags().Changed(clusteredFlag) {
		logCollector.Clustered = clustered
	}
	if cmd.Flags().Changed(namespacesFlag) {
		logCollector.Namespaces = namespaces
	}
	if cmd.Flags().Changed(keepSourceFlag) {
		logCollector.CleanOutput = keepSource
	}

	if cmd.Flags().Changed(gvkFlag) {
		parseGVK(gvkString)
	}

	if cmd.Flags().Changed(labelSelectorFlag) {
		parseLabelSelector(labelSelectorString)
	}
}

func parseGVK(gvkstring string) {
	var gvks []logcollector.GroupVersionKind
	err := json.Unmarshal([]byte(gvkstring), &gvks)
	if err != nil {
		log.Fatalf("Error while parsing GVK : %s", err.Error())
	}
	logCollector.GroupVersionKind = gvks
}

func parseLabelSelector(selectorString string) {
	var lbSelectors []apiv1.LabelSelector
	err := json.Unmarshal([]byte(selectorString), &lbSelectors)
	if err != nil {
		log.Fatalf("Error while parsing label selector : %s", err.Error())
	}
	logCollector.LabelSelector = lbSelectors
}
