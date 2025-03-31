package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

func manageFileInputs() error {
	if inputFileName != "" {
		data, err := os.ReadFile(inputFileName)
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
	var err error

	if cmd.Flags().Changed(internal.KubeconfigFlag) || logCollector.KubeConfig == "" {
		logCollector.KubeConfig = kubeConfig
	}

	err = logCollector.InitializeKubeClients()
	if err != nil {
		return err
	}

	// By default log collector will always run in clustered mode.
	logCollector.Clustered = true
	if len(namespaces) > 0 {
		logCollector.Clustered = false
	}

	if cmd.Flags().Changed(internal.LogLevelFlag) || logCollector.Loglevel == "" {
		logCollector.Loglevel = logLevel
	}

	if cmd.Flags().Changed(namespacesFlag) || !logCollector.Clustered {
		logCollector.Namespaces = namespaces
	}

	if cmd.Flags().Changed(keepSourceFlag) {
		logCollector.CleanOutput = keepSource
	}

	if cmd.Flags().Changed(gvkFlag) {
		gvks, gErr := parseGVK(gvkSlice)
		if gErr != nil {
			return gErr
		}
		logCollector.GroupVersionKinds = gvks
	}

	logCollector.GroupVersionKinds, err = deDuplicateAndFixGVKs(logCollector.GroupVersionKinds)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed(labelsFlag) {
		labels, err := parseLabelSelector(labelSlice)
		if err != nil {
			return err
		}
		logCollector.LabelSelectors = labels
	}
	logCollector.LabelSelectors = deDuplicateLabelSelector(logCollector.LabelSelectors)

	return nil
}

func parseGVK(gvkSlice []string) ([]logcollector.GroupVersionKind, error) {
	var gvks []logcollector.GroupVersionKind

	for idx := range gvkSlice {
		splitGVK := strings.Split(gvkSlice[idx], "/")
		if len(splitGVK) == 3 && splitGVK[2] != "" {
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
		andLabels := strings.Split(labelSlice[idx0], "|")
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
	labelSet := sets.NewString()
	for idx := range lbSelectors {
		if labelSet.Has(lbSelectors[idx].String()) {
			continue
		}
		labelSet.Insert(lbSelectors[idx].String())
		uniqueSelectors = append(uniqueSelectors, lbSelectors[idx])
	}
	return uniqueSelectors
}

func deDuplicateAndFixGVKs(gvks []logcollector.GroupVersionKind) ([]logcollector.GroupVersionKind, error) {
	var uniquegvks []logcollector.GroupVersionKind
	gvkSet := sets.NewString()

	groupList, err := logCollector.DisClient.ServerGroups()

	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			log.Error(err, "Error while getting the resource list from discovery client")
			return gvks, err
		}
		log.Warnf("The Kubernetes server has an orphaned API service. Server reports: %s", err.Error())
		log.Warn("To fix this, kubectl delete apiservice <service-name>")
	}

	for idx := range gvks {
		gvkString := strings.ToLower(fmt.Sprintf("%s", gvks[idx]))
		if gvkSet.Has(gvkString) {
			continue
		}
		gvkSet.Insert(gvkString)
		if gvks[idx].Kind == "" {
			return gvks, errors.New("kind cannot be empty in gvks, check your config file")
		}
		if gvks[idx].Version == "" {
			for index := range groupList.Groups {
				if strings.EqualFold(groupList.Groups[index].Name, gvks[idx].Group) {
					gvks[idx].Version = groupList.Groups[index].PreferredVersion.Version
				}
			}
		}
		uniquegvks = append(uniquegvks, gvks[idx])
	}
	return uniquegvks, nil
}
