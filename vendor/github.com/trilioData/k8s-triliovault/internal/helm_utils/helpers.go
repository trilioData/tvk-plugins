package helmutils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/utils/yml"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/yaml"
)

type PvcMetadataMap struct {
	Name     string
	Metadata string
}

//nolint:deadcode,unused // for future ref
// encodeRelease encodes a release returning a base64 encoded
// gzipped string representation, or error.
func encodeRelease(b []byte) (string, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err = w.Write(b); err != nil {
		return "", err
	}
	w.Close()

	return b64.EncodeToString(buf.Bytes()), nil
}

// decodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func decodeRelease(data string) ([]byte, error) {
	// base64 decode string
	b, err := b64.DecodeString(data)
	if err != nil {
		return []byte(""), err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return []byte(""), err
		}
		b2, err := ioutil.ReadAll(r)
		if err != nil {
			return []byte(""), err
		}
		b = b2
	}
	return b, nil
}

func getEncodedReleaseString(str string, sbGvk schema.GroupVersionKind) (string, error) {
	if sbGvk.Kind == internal.ConfigMapKind {
		obj := corev1.ConfigMap{}
		err := json.Unmarshal([]byte(str), &obj)
		if err != nil {
			return "", err
		}
		return obj.Data["release"], nil
	}
	obj := corev1.Secret{}
	err := json.Unmarshal([]byte(str), &obj)
	if err != nil {
		return "", err
	}
	return string(obj.Data["release"]), nil
}

func GenerateHelmReleaseName(currentHelmRelName string) (newHelmRelName string) {
	// Generate random string of given length to make the helm release Name unique
	randomStr := internal.GenerateRandomString(internal.HelmNameRandomStrLen, true)

	// If current release name length is < 4(length of randomly generated string)
	// then replace that many characters else remove the last 4 chars from
	// current release name and append the randomly generated string
	if len(currentHelmRelName) < len(randomStr) {
		newHelmRelName = randomStr[:len(currentHelmRelName)]
	} else {
		newHelmRelName = currentHelmRelName[:len(currentHelmRelName)-len(randomStr)]
		newHelmRelName += randomStr
	}

	return newHelmRelName
}

func collectPvcListMap(renderFile string) (pvcList []string, pvcMeta map[string]string) {
	pvcList = []string{}
	pvcMeta = map[string]string{}

	resources, parseErr := yml.Parse(renderFile)
	if parseErr != nil {
		log.Error(parseErr, "")
		return pvcList, pvcMeta
	}
	for i := 0; i < len(resources); i++ {
		part := resources[i]
		if part.Descriptor.Kind == internal.PersistentVolumeClaimKind {
			resource := unstructured.Unstructured{}
			unMarshalErr := yaml.Unmarshal([]byte(part.Contents), &resource)
			if unMarshalErr != nil {
				log.Warnf("error while un-marshaling the rendered pvc: %s", unMarshalErr.Error())
				continue
			}
			pvcList = append(pvcList, resource.GetName())
			// we need string json in the cr. Convert the yaml to json
			jsonContent, convErr := yaml.YAMLToJSON([]byte(part.Contents))
			if convErr != nil {
				log.Error(convErr, "")
				return nil, nil
			}
			pvcMeta[resource.GetName()] = string(jsonContent)
		} else if part.Descriptor.Kind == internal.StatefulSetKind {
			var stsObj appsv1.StatefulSet
			stsUnMarshalErr := yaml.Unmarshal([]byte(part.Contents), &stsObj)
			if stsUnMarshalErr != nil {
				log.Warnf("error while un-marshaling the rendered pvc: %s", stsUnMarshalErr.Error())
				continue
			}
			// Process each volume claim template
			for vctIndex := range stsObj.Spec.VolumeClaimTemplates {
				stsPvcMeta := stsObj.Spec.VolumeClaimTemplates[vctIndex]
				stsPvcMeta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PersistentVolumeClaimKind))

				replicaCount := 1
				if stsObj.Spec.Replicas != nil {
					replicaCount = int(*stsObj.Spec.Replicas)
				}
				for rep := 0; rep < replicaCount; rep++ {
					repPvcMeta := stsPvcMeta
					stsPvcName := fmt.Sprintf("%s-%s-%d", repPvcMeta.Name, stsObj.Name, rep)
					pvcList = append(pvcList, stsPvcName)
					repPvcMeta.Name = stsPvcName
					marshaledPvc, marshalErr := json.Marshal(repPvcMeta)
					if marshalErr != nil {
						log.Warnf("error while marshaling the pvc: %s", marshalErr.Error())
						continue
					}
					pvcMeta[stsPvcName] = string(marshaledPvc)
				}
			}
		}
	}
	return pvcList, pvcMeta
}

//nolint:deadcode,unused // for future ref
func getVersionSet(dc discovery.DiscoveryClient) []string {
	versions := []string{}

	groups, resources, err := dc.ServerGroupsAndResources()
	if err != nil {
		return versions
	}

	// FIXME: The Kubernetes test fixture for cli appears to always return nil
	// for calls to Discovery().ServerGroupsAndResources(). So in this case, we
	// return the default API list. This is also a safe value to return in any
	// other odd-ball case.
	if len(groups) == 0 && len(resources) == 0 {
		return versions
	}

	versionMap := make(map[string]interface{})

	// Extract the groups
	for _, g := range groups {
		for _, gv := range g.Versions {
			versionMap[gv.GroupVersion] = struct{}{}
		}
	}

	// Extract the resources
	var id string
	var ok bool
	for i := range resources {
		r := resources[i]
		for j := range r.APIResources {
			rl := r.APIResources[j]
			// A Kind at a GroupVersion can show up more than once. We only want
			// it displayed once in the final output.
			id = path.Join(r.GroupVersion, rl.Kind)
			if _, ok = versionMap[id]; !ok {
				versionMap[id] = struct{}{}
			}
		}
	}

	// Convert to a form that NewVersionSet can use
	for k := range versionMap {
		versions = append(versions, k)
	}

	return versions
}

func createModifiedPvcMetaMap(oldRlsManifests, newRlsManifests string) map[string]PvcMetadataMap {
	var (
		oldPvcList []string
		newPvcList []string
		resultMap  = map[string]PvcMetadataMap{}
		newPvcMap  map[string]string
	)

	oldPvcList, _ = collectPvcListMap(oldRlsManifests)
	log.Infof("[Debug] Old Pvc list: %+v", oldPvcList)

	newPvcList, newPvcMap = collectPvcListMap(newRlsManifests)
	log.Infof("[Debug] New Pvc list: %+v", newPvcList)
	for i := range oldPvcList {
		newPvcName := newPvcList[i]
		oldPvcName := oldPvcList[i]
		log.Infof("Mapping old PVC [%s] with new PVC [%s]", oldPvcName, newPvcName)
		resultMap[oldPvcName] = PvcMetadataMap{Metadata: newPvcMap[newPvcName], Name: newPvcName}
	}

	return resultMap
}
