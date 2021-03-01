package kube

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	// To authenticate kubeEnv on gke cluster
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

var kubeConfigFromCommandLine string

// NewConfigFromCommandline returns Config obtained from command-line flags. flag.Parse must be called before calling this function.
func NewConfigFromCommandline() (string, error) {
	var kubeConf string

	if kubeConfigFromCommandLine != "" {
		kubeConf = kubeConfigFromCommandLine
		if err := normalizeFile(&kubeConf); err != nil {
			return "", err
		}
		return kubeConf, nil
	}

	kubeconfig, exist := os.LookupEnv("KUBECONFIG")
	if exist {
		if err := normalizeFile(&kubeconfig); err != nil {
			return "", err
		}
		return kubeconfig, nil
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
	defaultConf := ctrl.GetConfigOrDie()
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

// NewEnv returns a new Kubernetes accessor
func NewEnv(scheme *runtime.Scheme) (*Accessor, error) {
	confPath, err := NewConfigFromCommandline()
	if err != nil {
		return nil, err
	}

	accessor, err := newAccessor(confPath, scheme)
	if err != nil {
		return nil, err
	}
	return accessor, nil
}
