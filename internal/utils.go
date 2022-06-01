package internal

import (
	cryptorand "crypto/rand"
	"math/big"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func CheckIsOpenshift(disClient *discovery.DiscoveryClient, groupVersion string) bool {
	_, err := disClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
	}
	return true
}

func GenerateRandomString(n int, isOnlyAlphabetic bool) string {
	letters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "1234567890"
	rand.Seed(time.Now().UnixNano())
	letterRunes := []rune(letters)
	if !isOnlyAlphabetic {
		letterRunes = []rune(letters + numbers)
	}
	b := make([]rune, n)
	for i := range b {
		randNum, _ := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(letterRunes))))
		b[i] = letterRunes[randNum.Int64()]
	}
	return string(b)
}
