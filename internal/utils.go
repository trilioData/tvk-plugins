package internal

import (
	log "github.com/sirupsen/logrus"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

var (
	AllowedOutputFormats = sets.NewString(FormatJSON, FormatYAML, FormatWIDE)
)

func CheckIfAPIVersionKindAvailable(discoveryClient *discovery.DiscoveryClient, groupVersion, kind string) (found bool) {
	resources, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("%s not found", groupVersion)
			return
		}
	}

	for i := range resources.APIResources {
		resource := resources.APIResources[i]
		if resource.Kind == kind {
			found = true
			log.Debugf("%s found", groupVersion)
			break
		}
	}
	return
}
