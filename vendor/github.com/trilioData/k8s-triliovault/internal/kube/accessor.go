package kube

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/trilioData/k8s-triliovault/internal/utils/retry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"

	extClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	client "k8s.io/client-go/kubernetes"
	ctrlRuntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

var (
	defaultRetryTimeout = retry.Timeout(time.Minute * 10)
	defaultRetryDelay   = retry.Delay(time.Second * 10)

	matchExpressionOperator = map[metav1.LabelSelectorOperator]selection.Operator{
		metav1.LabelSelectorOpIn:           selection.In,
		metav1.LabelSelectorOpNotIn:        selection.NotIn,
		metav1.LabelSelectorOpExists:       selection.Exists,
		metav1.LabelSelectorOpDoesNotExist: selection.DoesNotExist,
	}
)

// Accessor is a helper for accessing Kubernetes programmatically. It bundles some of the high-level
// operations that is frequently used by the test framework.
type Accessor struct {
	restConfig *rest.Config
	ctl        *kubectl
	set        *client.Clientset
	extSet     *extClient.Clientset
	client     ctrlRuntime.Client
	context    context.Context
}

// newAccessor returns a new instance of an accessor.
func newAccessor(kubeConfig string, scheme *runtime.Scheme) (*Accessor, error) {
	restConfig, err := LoadKubeConfigOrDie(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config. %v", err)
	}
	// uncomment below 3 lines when needed
	// restConfig.APIPath = "/api"
	// restConfig.GroupVersion = &corev1.SchemeGroupVersion
	// restConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	set, err := client.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	extSet, err := extClient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	if kubeConfig == "" {
		kubeConfig = path.Join(os.Getenv("HOME"), ".kube", "config")
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
	return &Accessor{
		restConfig: restConfig,
		ctl: &kubectl{
			kubeConfig: kubeConfig,
		},
		set:     set,
		extSet:  extSet,
		client:  cl,
		context: context.Background(),
	}, nil
}

// TODO: avoid using these type of funcs which return hidden fields. But remove it only when our helm internal library uses accessor
func (a *Accessor) GetRestConfig() *rest.Config {
	return a.restConfig
}
