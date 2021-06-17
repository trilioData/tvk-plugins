package tools

import (
	"errors"
	"fmt"
	"os"

	ctrlRuntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	client "k8s.io/client-go/kubernetes"

	// To authenticate kubeEnv on gke cluster
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

// Accessor is a helper for accessing Kubernetes programmatically. It bundles some of the high-level
// operations that is frequently used by the test framework.
type Accessor struct {
	restConfig      *rest.Config
	set             *client.Clientset
	client          ctrlRuntime.Client
	discoveryClient *discovery.DiscoveryClient
}

// NewEnv returns a new Kubernetes environment with accessor
func NewEnv(kubeConfig string, scheme *runtime.Scheme) (*Accessor, error) {
	confPath, err := NewConfigFromCommandline(kubeConfig)
	if err != nil {
		return nil, err
	}

	accessor, err := newAccessor(confPath, scheme)
	if err != nil {
		return nil, err
	}
	return accessor, nil
}

// newAccessor returns a new instance of an accessor.
func newAccessor(kubeConfig string, scheme *runtime.Scheme) (*Accessor, error) {
	restConfig, err := LoadKubeConfigOrDie(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config. %v", err)
	}

	set, err := client.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := func(c *rest.Config) (meta.RESTMapper, error) {
		return apiutil.NewDynamicRESTMapper(c)
	}(restConfig)
	if err != nil {
		return nil, err
	}
	cl, err := ctrlRuntime.New(restConfig, ctrlRuntime.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
	if err != nil {
		return nil, err
	}

	discClient, dErr := discovery.NewDiscoveryClientForConfig(restConfig)
	if dErr != nil {
		return nil, dErr
	}

	return &Accessor{
		restConfig:      restConfig,
		set:             set,
		client:          cl,
		discoveryClient: discClient,
	}, nil
}

// NewConfigFromCommandline returns Config obtained from command-line flags
func NewConfigFromCommandline(kubeConfig string) (string, error) {
	var kubeConf string

	if kubeConfig != "" {
		kubeConf = kubeConfig
		if err := normalizeFile(&kubeConf); err != nil {
			return "", err
		}
		return kubeConf, nil
	}

	var exist bool
	kubeConf, exist = os.LookupEnv("KUBECONFIG")
	if exist {
		if err := normalizeFile(&kubeConf); err != nil {
			return "", err
		}
		return kubeConf, nil
	}

	if kubeConf == "" {
		return "", errors.New("failed to fetch kubeconfig file from specified 'kubeconfig' path and 'KUBECONFIG' env variable")
	}

	return "", nil
}

func normalizeFile(path *string) error {
	// If the path uses the homedir ~, expand the path.
	var err error
	*path, err = homedir.Expand(*path)
	if err != nil {
		return err
	}

	return checkFileExists(*path)
}

func checkFileExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadKubeConfigOrDie will load the kube-config from the path of kubeConfig parameter passed
func LoadKubeConfigOrDie(kubeConfig string) (*rest.Config, error) {
	defaultConf := config.GetConfigOrDie()
	if kubeConfig != "" {
		info, err := os.Stat(kubeConfig)
		if err != nil || info.Size() == 0 {
			// If the specified kubeconfig doesn't exists / empty file / any other error
			// from file stat, fall back to default
			return defaultConf, err
		}
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			log.Errorf("Cannot read kubeConfig: %s", err)
			return nil, err
		}
		return cfg, nil
	}

	return defaultConf, nil
}

func (a *Accessor) GetRestConfig() *rest.Config {
	return a.restConfig
}

func (a *Accessor) GetRuntimeClient() ctrlRuntime.Client {
	return a.client
}

func (a *Accessor) GetClientset() *client.Clientset {
	return a.set
}

func (a *Accessor) GetDiscoveryClient() *discovery.DiscoveryClient {
	return a.discoveryClient
}
