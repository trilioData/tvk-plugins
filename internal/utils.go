package internal

import (
	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

var (
	AllowedOutputFormats = sets.NewString(FormatJSON, FormatYAML, FormatWIDE)
)

func CheckIfAPIVersionKindAvailable(discoveryClient *discovery.DiscoveryClient, gvk schema.GroupVersionKind) (found bool) {
	resources, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		log.Debugf("%s not found on cluster - %s", gvk.GroupVersion().String(), err.Error())
		return
	}

	for i := range resources.APIResources {
		resource := resources.APIResources[i]
		if resource.Kind == gvk.Kind {
			found = true
			log.Debugf("%s found on cluster", gvk.String())
			break
		}
	}
	return
}
