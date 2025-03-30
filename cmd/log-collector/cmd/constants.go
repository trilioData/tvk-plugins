package cmd

import (
	"github.com/spf13/cobra"
	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

const (
	binaryName       = "log-collector"
	clusteredFlag    = "clustered"
	namespacesFlag   = "namespaces"
	keepSourceFlag   = "keep-source-folder"
	gvkFlag          = "gvks"
	configFileFlag   = "config-file"
	labelsFlag       = "labels"
	defaultNamespace = "default"

	shortUsage = "log-collector collects the information of resources such as yaml configuration and logs from k8s cluster."
	longUsage  = "log-collector let you define what you need to log and how to log it by collecting the the logs " +
		"and events of Pod alongside the metadata of all resources related to TVK as either namespaced by providing " +
		"namespaces name separated by comma or clustered. It also collects the CRDs related to TVK and zip them " +
		"on the path you specify"

	namespacesUsage     = "specifies all the namespaces separated by commas"
	namespacesShort     = "n"
	clusteredUsage      = "specifies clustered object"
	clusteredDefault    = true
	clusteredShort      = "c"
	keepSourceUsage     = "Keep source directory and Zip both"
	keepSourceDefault   = false
	keepSourceShort     = "s"
	configFileUsage     = "specifies the name of the yaml file for inputs to the log collector flags"
	configFlagShorthand = "f"
	gvkUsage            = "specifies the gvk(s) string of all gvk other than log collector handles by default"
	gvkFlagShorthand    = "g"
	labelsUsage         = "specifies the label(s) string of all labels other than log collector handles by default"
	labelsFlagShorthand = "r"
)

var (
	rootCmd           *cobra.Command
	clustered         bool
	namespaces        []string
	kubeConfig        string
	keepSource        bool
	logLevel          string
	namespacesDefault []string
	inputFileName     string
	gvkSlice          []string
	labelSlice        []string
	logCollector      logcollector.LogCollector
	cmd               *cobra.Command
)
