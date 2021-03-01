package helmutils

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/cli"

	"github.com/trilioData/k8s-triliovault/internal"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal/kube"

	corev1 "k8s.io/api/core/v1"
	corev1Typed "k8s.io/client-go/kubernetes/typed/core/v1"

	"helm.sh/helm/v3/pkg/action"
	helm3chart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	chartutilv3 "helm.sh/helm/v3/pkg/chartutil"
	v3Kube "helm.sh/helm/v3/pkg/kube"
	release3 "helm.sh/helm/v3/pkg/release"
	helmStoragev3 "helm.sh/helm/v3/pkg/storage"
	driverv3 "helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	v3Time "helm.sh/helm/v3/pkg/time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	b64       = base64.StdEncoding
	magicGzip = []byte{0x1f, 0x8b, 0x08}
)

type HelmV3 struct {
	config           *rest.Config
	releaseNamespace string
}

type helmV3StorageBackend struct {
	config    *rest.Config
	namespace string
	storage   *helmStoragev3.Storage
}

type subChart struct {
	Name         string
	Version      string
	Repository   string
	StoragePath  string
	ParentDepMap map[string]string
}

func newHelmV3Manager(releaseNs string, config *rest.Config) *HelmV3 {
	return &HelmV3{releaseNamespace: releaseNs, config: config}
}

func (h *HelmV3) GetHelmVersion() v1.HelmVersion {
	return v1.Helm3
}

func (h *HelmV3) GetCustomStorageBackend() (HelmStorageBackend, error) {
	tmp, err := h.InitConfigMapStorageBackend()
	if err != nil {
		return nil, err
	}
	sb := tmp.(*helmStoragev3.Storage)
	result := helmV3StorageBackend{storage: sb, config: h.config, namespace: h.releaseNamespace}
	return &result, nil
}

func (h *HelmV3) GetDefaultStorageBackend() (HelmStorageBackend, error) {
	tmp, err := h.InitSecretStorageBackend()
	if err != nil {
		return nil, err
	}
	sb := tmp.(*helmStoragev3.Storage)
	result := helmV3StorageBackend{storage: sb, config: h.config, namespace: h.releaseNamespace}
	return &result, nil
}

func (h *HelmV3) InitSecretStorageBackend() (interface{}, error) {
	clientV1, err := corev1Typed.NewForConfig(h.config)
	if err != nil {
		return nil, fmt.Errorf("failed to get core/v1 client: %s", err)
	}
	storageBackend := helmStoragev3.Init(driverv3.NewSecrets(clientV1.Secrets(h.releaseNamespace)))
	return storageBackend, nil
}

func (h *HelmV3) InitConfigMapStorageBackend() (interface{}, error) {
	clientV1, err := corev1Typed.NewForConfig(h.config)
	if err != nil {
		return nil, fmt.Errorf("failed to get core/v1 client: %s", err)
	}
	storageBackend := helmStoragev3.Init(driverv3.NewConfigMaps(clientV1.ConfigMaps(h.releaseNamespace)))
	return storageBackend, nil
}

func (h *HelmV3) GetModifiedReleaseFromStorageObject(str string, hTransform v1.HelmTransform, name, ns,
	storageKind, backupLocation string, latestRev int32) (interface{}, error) {
	rls, err := getV3ReleaseFromString(str, corev1.SchemeGroupVersion.WithKind(storageKind))
	if err != nil {
		return nil, err
	}

	// load sub-chart dependencies from target
	subChartLoadedRel, lErr := h.LoadDependencies(rls, int32(rls.Version), backupLocation)
	if lErr != nil {
		return nil, lErr
	}

	if rls.Version == int(latestRev) {
		// transform release with given --set values
		transformedRel, tErr := h.TransformRelease(subChartLoadedRel, hTransform)
		if tErr != nil {
			return nil, lErr
		}
		subChartLoadedRel = transformedRel
	}

	// modify release with new release name
	modifiedRls, mErr := h.ModifyRelease(subChartLoadedRel, name, ns)
	if mErr != nil {
		return nil, mErr
	}
	return modifiedRls, nil
}

func (h *HelmV3) ModifyRelease(rls interface{}, name, ns string) (interface{}, error) {
	var (
		modifiedRls *release3.Release
		err         error
	)

	oldRls := rls.(*release3.Release)
	chrt := oldRls.Chart
	vals := oldRls.Config

	actionConfig := &action.Configuration{
		RESTClientGetter: genericclioptions.NewConfigFlags(true),
		KubeClient:       v3Kube.New(nil),
		Log:              func(_ string, _ ...interface{}) {},
	}

	install := action.NewInstall(actionConfig)
	install.ReleaseName = name
	install.Namespace = ns
	install.DryRun = true
	install.ClientOnly = true

	modifiedRls, err = install.Run(chrt, vals)
	if err != nil {
		return nil, fmt.Errorf("failed to get the release: %s", err)
	}

	modifiedRls.Version = oldRls.Version
	modifiedRls.Info = oldRls.Info
	return modifiedRls, nil
}

func (h *HelmV3) ModifyReleaseManifest(rls interface{}, name, ns string) (string, error) {
	tmp, err := h.ModifyRelease(rls, name, ns)
	if err != nil {
		return "", err
	}
	release := tmp.(*release3.Release)
	return release.Manifest, nil
}

func (h *HelmV3) GetReleaseFromStorageObj(str, storageObjectKind string) (interface{}, error) {
	rls, err := getV3ReleaseFromString(str, corev1.SchemeGroupVersion.WithKind(storageObjectKind))
	if err != nil {
		return nil, err
	}
	return rls, nil
}

func (h *HelmV3) GetReleaseContentFromStorageObj(str, storageObjectKind string) (manifest string, v int32, err error) {
	rls, err := getV3ReleaseFromString(str, corev1.SchemeGroupVersion.WithKind(storageObjectKind))
	if err != nil {
		return "", 0, err
	}
	return rls.Manifest, int32(rls.Version), nil
}

func (h *HelmV3) GetReleaseContentFromRelease(rls interface{}) (manifest string, v int32) {
	release := rls.(*release3.Release)
	return release.Manifest, int32(release.Version)
}

func (h *helmV3StorageBackend) GetReleaseContent(releaseName string, revision int32) (manifest string, v int32, err error) {
	const DefaultRevision = 1
	tmp, err := h.GetRelease(releaseName, revision)
	if err != nil {
		return "", DefaultRevision, err
	}

	if tmp == nil {
		return "", DefaultRevision, nil
	}
	release := tmp.(*release3.Release)
	return release.Manifest, int32(release.Version), nil
}

func (h *helmV3StorageBackend) GetRelease(releaseName string, revision int32) (interface{}, error) {
	sb := *h.storage
	var (
		release *release3.Release
		err     error
	)
	if revision > 0 {
		release, err = sb.Get(releaseName, int(revision))
	} else {
		// if revision is not given, get the latest release
		release, err = sb.Deployed(releaseName)
	}
	if err != nil {
		return nil, err
	}
	return release, nil
}

func (h *helmV3StorageBackend) SaveDir(releaseName string, revision int32, destDir string) error {
	tmp, err := h.GetRelease(releaseName, revision)
	if err != nil {
		log.Error(err, "")
		return err
	}
	chart := tmp.(*release3.Release).Chart
	return chartutilv3.SaveDir(chart, destDir)
}

func (h *helmV3StorageBackend) GetKind() v1.HelmStorageBackend {
	return v1.HelmStorageBackend(h.storage.Name())
}

func (h *helmV3StorageBackend) CreateReleaseFromStorageObjectStr(secretStr string) error {
	rls, err := getV3ReleaseFromString(secretStr, corev1.SchemeGroupVersion.WithKind(string(h.GetKind())))
	if err != nil {
		return err
	}
	return h.CreateRelease(rls)
}

func (h *helmV3StorageBackend) CreateRelease(rls interface{}) error {
	release := rls.(*release3.Release)
	release.Info.LastDeployed = v3Time.Now()
	return h.storage.Create(release)
}

func (h *helmV3StorageBackend) DeleteRelease(rls string, revision int) error {
	_, err := h.storage.Delete(rls, revision)
	return err
}

func (h *helmV3StorageBackend) UpdateReleaseFromStorageObjectStr(secretStr string) error {
	rls, err := getV3ReleaseFromString(secretStr, corev1.SchemeGroupVersion.WithKind(string(h.GetKind())))
	if err != nil {
		return err
	}
	return h.UpdateRelease(rls)
}

func (h *helmV3StorageBackend) UpdateRelease(rls interface{}) error {
	release := rls.(*release3.Release)
	release.Info.LastDeployed = v3Time.Now()
	return h.storage.Update(release)
}

func (h *helmV3StorageBackend) EncodedHistory(ctx context.Context, cl client.Client, name string) ([]string, error) {
	gvk := corev1.SchemeGroupVersion.WithKind(strings.Join([]string{string(h.GetKind()), "List"}, ""))
	opts := []client.ListOption{client.InNamespace(h.namespace), client.MatchingLabels{"owner": "helm", "name": name}}
	return kube.UnstructuredListMarshalJSON(ctx, cl, gvk, opts...)
}

// ProcessDependencies pulls the dependency sub-charts for all the revision of a release.
// It saves the charts at `/tmp` path and maintains the relationship between parent and it's sub-charts.
func (h *helmV3StorageBackend) ProcessDependencies(release string, deployedRev int32) error {

	var (
		goRoutineCount   int
		wg               sync.WaitGroup
		subChartBasePath = path.Join(internal.TmpDir, release, internal.HelmDependencyDir)
	)

	if _, err := os.Stat(subChartBasePath); err == nil {
		log.Infof("dependency sub-charts for release [%s] already exist, skipping ProcessDependencies", release)
		return nil
	}

	// Maximum goRoutines allowed to run
	const MaxGoRoutines = 5

	errChannel := make(chan error, MaxGoRoutines)
	depChannel := make(chan *subChart, 20)

	for i := deployedRev; i > 0; i-- {

		tmp, err := h.GetRelease(release, i)
		if err != nil {
			log.Errorf("revision %d no found in storage backend for release: %s ", i, release)
			return err
		}

		rel := tmp.(*release3.Release)
		revision := rel.Version

		chartDep := rel.Chart.Metadata.Dependencies
		// if .lock file exits then use that instead of Metadata.Dependencies
		if rel.Chart.Lock != nil {
			chartDep = rel.Chart.Lock.Dependencies
		}

		if len(chartDep) > 0 {
			if goRoutineCount < MaxGoRoutines {
				wg.Add(1)
				go pullWorker(depChannel, errChannel, &wg)
				goRoutineCount++
			}

			revSubChartPath := path.Join(subChartBasePath, strconv.Itoa(revision))
			dependencies := getDepList(chartDep, revSubChartPath)

			for j := range dependencies {
				depChannel <- &dependencies[j]
			}
		}
	}

	wg.Wait()

	if len(errChannel) != 0 {
		err := <-errChannel

		// Wrap the error if error is returned from more than 1 goRoutine
		for i := 1; i <= len(errChannel)-1; i++ {
			err = errors.Wrap(err, (<-errChannel).Error())
		}
		return err
	}
	return nil
}

func getV3ReleaseFromString(str string, sbGvk schema.GroupVersionKind) (*release3.Release, error) {
	rlsStr, err := getEncodedReleaseString(str, sbGvk)
	if err != nil {
		return nil, err
	}
	// found the secret, decode the base64 data string
	return DecodeV3Release(rlsStr)
}

//nolint:deadcode,unused // for future ref
// encodeV3Release encodes a release returning a base64 encoded
// gzipped binary protobuf encoding representation, or error.
func encodeV3Release(rls *release3.Release) (string, error) {
	b, err := json.Marshal(rls)
	if err != nil {
		return "", err
	}
	return encodeRelease(b)
}

// decodeV3Release decodes the bytes in data into a release
// type. Data must contain a base64 encoded string of a
// valid protobuf encoding of a release, otherwise
// an error is returned.
func DecodeV3Release(data string) (*release3.Release, error) {
	b, err := decodeRelease(data)
	if err != nil {
		return nil, err
	}
	var rls release3.Release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

func (h *HelmV3) GetModifiedPvcMetaMap(newRelease, oldRelease interface{}) (map[string]PvcMetadataMap, error) {
	rls := oldRelease.(*release3.Release)
	oldRlsManifests := rls.Manifest

	newRls := newRelease.(*release3.Release)
	newRlsManifests := newRls.Manifest
	resultMap := createModifiedPvcMetaMap(oldRlsManifests, newRlsManifests)

	return resultMap, nil
}

// LoadDependencies loads all the dependency sub-charts as per revision for a release
func (h *HelmV3) LoadDependencies(rls interface{}, revision int32, backupLocation string) (interface{}, error) {
	release := rls.(*release3.Release)

	dependencies := release.Chart.Metadata.Dependencies
	// if .lock file exits then use that over Metadata.Dependencies
	if release.Chart.Lock != nil {
		dependencies = release.Chart.Lock.Dependencies
	}

	if len(dependencies) > 0 {

		revDependencyPath := path.Join(backupLocation, internal.HelmBackupDir, release.Name, internal.HelmDependencyDir,
			strconv.Itoa(int(revision)))

		for i := range dependencies {
			dep := dependencies[i]
			subChartPath := path.Join(revDependencyPath, dep.Name)

			subChart, err := loader.Load(subChartPath)
			if err != nil {
				log.Errorf("Could not load dependency sub-chart for release: %s revision : %d, "+
					"error: %s", release.Name, revision, err.Error())
				return nil, err
			}

			// Only one level of chart alias functionality is handled
			if dep.Alias != "" {
				subChart.Metadata.Name = dep.Alias
			}
			release.Chart.AddDependency(subChart)
		}
	}
	return release, nil
}

func pullDependency(depChannel chan *subChart, dep *subChart, errChannel chan error) {

	var (
		alreadyExistsDepMap map[string]string
		chartPath           string
		tarName             string
	)

	log.Infof("Processing dependency: [%s]", dep.Name)

	tarName = strings.Join([]string{dep.Name, "-", "%s", ".tgz"}, "")

	if !checkIfChartAlreadyExists(dep, dep.ParentDepMap) {

		pull := action.NewPull()
		pull.Version = dep.Version
		pull.DestDir = dep.StoragePath
		pull.Settings = cli.New()
		pull.Untar = true
		chartRef := strings.Join([]string{dep.Repository, "/", fmt.Sprintf(tarName, dep.Version)}, "")

		// if dependency is given in the form of x (eg. 7.x.x) then pull latest chart as per version constraints
		if strings.Contains(dep.Version, "x") {
			pull.RepoURL = dep.Repository
			chartRef = dep.Name
		}

		if _, err := pull.Run(chartRef); err != nil {
			log.Infof(err.Error())
			errChannel <- err
			return
		}
	}

	c, err := loader.Load(path.Join(dep.StoragePath, dep.Name))
	if err != nil {
		errChannel <- err
		return
	}

	tgzPath := path.Join(dep.StoragePath, fmt.Sprintf(tarName, c.Metadata.Version))
	chartPath = c.ChartPath()
	alreadyExistsDepMap = createDependencyMap(c.Dependencies())

	err = os.RemoveAll(tgzPath)
	if err != nil {
		log.Errorf("Failed to delete %s", tgzPath)
		errChannel <- err
		return
	}

	chartDep := c.Metadata.Dependencies
	// if .lock file exits then use that over Metadata.Dependencies
	if c.Lock != nil {
		chartDep = c.Lock.Dependencies
	}
	dependencies := getDepList(chartDep, path.Join(dep.StoragePath, chartPath, internal.HelmSubChartPath))

	for i := range dependencies {
		dep := dependencies[i]
		dep.ParentDepMap = alreadyExistsDepMap
		depChannel <- &dep
	}
}

func checkIfChartAlreadyExists(keyChart *subChart, chartMap map[string]string) bool {
	if val, exists := chartMap[keyChart.Name]; exists {
		// IsCompatibleRange checks if the actual existing version is compatible with the version specified in yaml
		// IsCompatibleRange is used in case if dependency version specified as 7.x.x and existing version is 7.3.14(latest)
		// keyChart version empty means that charts already exists in parent's charts/ directory
		if val == keyChart.Version || chartutilv3.IsCompatibleRange(keyChart.Version, val) ||
			keyChart.Version == "" {
			return true
		}
	}
	return false
}

func createDependencyMap(charts []*helm3chart.Chart) map[string]string {
	depMap := make(map[string]string)
	for i := range charts {
		depMap[charts[i].Name()] = charts[i].Metadata.Version
	}
	return depMap
}

func getDepList(deps []*helm3chart.Dependency, storagePath string) []subChart {
	var depSlice []subChart
	for i := range deps {
		depSlice = append(depSlice, subChart{
			Name:        deps[i].Name,
			Version:     deps[i].Version,
			Repository:  strings.TrimRight(deps[i].Repository, "/"),
			StoragePath: storagePath,
		})
	}
	return depSlice
}

func pullWorker(depChannel chan *subChart, errChannel chan error, waitGroup *sync.WaitGroup) {
	const Timeout = 10
	for {
		if len(errChannel) != 0 {
			waitGroup.Done()
			return
		}

		select {
		// wait for 10 sec to get any work, otherwise shut down the go routine
		case <-time.After(Timeout * time.Second):
			waitGroup.Done()
			return
		case dep := <-depChannel:
			pullDependency(depChannel, dep, errChannel)
		}
	}
}

func (h *HelmV3) TransformRelease(rls interface{}, helmTransform v1.HelmTransform) (interface{}, error) {
	release := rls.(*release3.Release)

	// if --set values Config map is not already present then create a new map
	if len(release.Config) == 0 {
		release.Config = make(map[string]interface{})
	}

	for i := range helmTransform.Set {
		set := helmTransform.Set[i]
		if err := strvals.ParseInto(set.Key+"="+set.Value, release.Config); err != nil {
			log.Errorf("failed parsing --set data. error: %s", err.Error())
			return nil, err
		}
	}
	return release, nil
}
