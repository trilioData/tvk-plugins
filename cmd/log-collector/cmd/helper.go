package cmd

import (
	"errors"
	"io/ioutil"
	"reflect"
	"strings"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

func manageFileInputs() error {
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
	return overrideFileInputsFromCLI()
}

// overrideFileInputsFromCLI checks if external flag is given. if yes then override
func overrideFileInputsFromCLI() error {

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
		gvks, err := parseGVK(gvkSlice)
		if err != nil {
			return err
		}
		logCollector.GroupVersionKinds = deDuplicateGVKs(gvks)
	}
	if cmd.Flags().Changed(labelsFlag) {
		labels, err := parseLabelSelector(labelSlice)
		if err != nil {
			return err
		}
		logCollector.LabelSelectors = deDuplicateLabelSelector(labels)
	}

	return nil
}

func parseGVK(gvkSlice []string) ([]logcollector.GroupVersionKind, error) {
	var gvks []logcollector.GroupVersionKind
	groupList, err := logCollector.DisClient.ServerGroups()

	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			log.Error(err, "Error while getting the resource list from discovery client")
			return gvks, err
		}
		log.Warnf("The Kubernetes server has an orphaned API service. Server reports: %s", err.Error())
		log.Warn("To fix this, kubectl delete apiservice <service-name>")
	}

	for idx := range gvkSlice {
		splitGVK := strings.Split(gvkSlice[idx], "/")
		if len(splitGVK) == 3 && splitGVK[2] != "" {
			if splitGVK[1] == "" {
				for idx := range groupList.Groups {
					if strings.EqualFold(groupList.Groups[idx].Name, splitGVK[0]) {
						splitGVK[1] = groupList.Groups[idx].PreferredVersion.Version
					}
				}
			}
			gvk := logcollector.GroupVersionKind{
				Group:   splitGVK[0],
				Version: splitGVK[1],
				Kind:    splitGVK[2],
			}
			gvks = append(gvks, gvk)
		} else {
			if splitGVK[2] == "" {
				return gvks, errors.New("kind cannot be empty in gvks flag ")
			}
			return gvks, errors.New("error parsing gvks ")
		}
	}
	return gvks, nil
}

func parseLabelSelector(labelSlice []string) ([]apiv1.LabelSelector, error) {
	var lbSelectors []apiv1.LabelSelector

	for idx0 := range labelSlice {
		andLabels := strings.Split(labelSlice[idx0], ",")
		matchLabels := make(map[string]string)
		for idx := range andLabels {
			mapKeysValues := strings.Split(andLabels[idx], "=")
			if len(mapKeysValues) == 2 {
				matchLabels[mapKeysValues[0]] = mapKeysValues[1]
			} else {
				return lbSelectors, errors.New(" Error Parsing Labels ")
			}
		}
		labelSelector := apiv1.LabelSelector{
			MatchLabels: matchLabels,
		}
		lbSelectors = append(lbSelectors, labelSelector)
	}
	return lbSelectors, nil
}

func deDuplicateLabelSelector(lbSelectors []apiv1.LabelSelector) []apiv1.LabelSelector {
	var uniqueSelectors []apiv1.LabelSelector

	for idx := range lbSelectors {
		skip := false
		for idx1 := range uniqueSelectors {
			if reflect.DeepEqual(lbSelectors[idx], uniqueSelectors[idx1]) {
				skip = true
				break
			}
		}
		if !skip {
			uniqueSelectors = append(uniqueSelectors, lbSelectors[idx])
		}
	}
	return uniqueSelectors
}

func deDuplicateGVKs(gvks []logcollector.GroupVersionKind) []logcollector.GroupVersionKind {
	var uniquegvks []logcollector.GroupVersionKind
	for idx := range gvks {
		skip := false
		for idx1 := range uniquegvks {
			if reflect.DeepEqual(gvks[idx], uniquegvks[idx1]) {
				skip = true
				break
			}
		}
		if !skip {
			uniquegvks = append(uniquegvks, gvks[idx])
		}
	}
	return uniquegvks
}
