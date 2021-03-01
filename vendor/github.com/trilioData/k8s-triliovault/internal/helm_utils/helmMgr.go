package helmutils

import (
	"context"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HelmMgr interface {
	GetHelmVersion() v1.HelmVersion
	GetDefaultStorageBackend() (HelmStorageBackend, error)
	InitConfigMapStorageBackend() (interface{}, error)
	GetCustomStorageBackend() (HelmStorageBackend, error)
	InitSecretStorageBackend() (interface{}, error)
	ModifyRelease(rls interface{}, name, ns string) (interface{}, error)
	ModifyReleaseManifest(rls interface{}, name, ns string) (string, error)
	GetModifiedReleaseFromStorageObject(str string, helmTransform v1.HelmTransform, name, ns, storageKind,
		backupLocation string, latestRev int32) (interface{}, error)
	GetReleaseFromStorageObj(str, storageObjectKind string) (interface{}, error)
	GetReleaseContentFromStorageObj(str, storageObjectKind string) (manifest string, v int32, err error)
	GetReleaseContentFromRelease(rls interface{}) (manifest string, v int32)
	GetModifiedPvcMetaMap(newRelease, oldRelease interface{}) (map[string]PvcMetadataMap, error)
	LoadDependencies(rls interface{}, revision int32, backupLocation string) (interface{}, error)
	TransformRelease(rls interface{}, helmTransform v1.HelmTransform) (interface{}, error)
}

type HelmStorageBackend interface {
	GetKind() v1.HelmStorageBackend
	SaveDir(releaseName string, revision int32, destDir string) error
	GetReleaseContent(releaseName string, revision int32) (string, int32, error)
	GetRelease(releaseName string, revision int32) (interface{}, error)
	CreateRelease(rls interface{}) error
	UpdateRelease(rls interface{}) error
	DeleteRelease(releaseName string, revision int) error
	EncodedHistory(ctx context.Context, cl client.Client, name string) ([]string, error)
	CreateReleaseFromStorageObjectStr(cmStr string) error
	UpdateReleaseFromStorageObjectStr(cmStr string) error
	ProcessDependencies(releaseName string, revision int32) error
}

func NewHelmStorageBackends(hlm HelmMgr) (defaultSb, customSb HelmStorageBackend, gErr error) {
	defaultSb, gErr = hlm.GetDefaultStorageBackend()
	if gErr != nil {
		return nil, nil, gErr
	}
	customSb, gErr = hlm.GetCustomStorageBackend()
	return defaultSb, customSb, gErr
}

func NewHelmManager(config *rest.Config, releaseNamespace string) (HelmMgr, error) {
	return newHelmV3Manager(releaseNamespace, config), nil
}
