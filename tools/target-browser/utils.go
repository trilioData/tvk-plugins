package targetbrowser

import (
	"context"
	"fmt"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/tools"
)

type Config struct {
	Scheme                                                                          *runtime.Scheme
	KubeConfig, TargetName, TargetNamespace, OrderBy, CaCert, ClientCert, ClientKey string
	PageSize, Pages                                                                 int
	InsecureSkipTLS                                                                 bool
}

// validateTarget validates if provided target is present target-namespace and browsing is enabled for it.
func (targetBrowserConfig *Config) validateTarget(ctx context.Context, cl client.Client) (*unstructured.Unstructured, error) {
	// get target
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   tools.TriliovaultGroup,
		Version: tools.V1Version,
		Kind:    tools.TargetKind,
	})

	if err := cl.Get(ctx, types.NamespacedName{Namespace: targetBrowserConfig.TargetNamespace, Name: targetBrowserConfig.TargetName},
		target); err != nil {
		return nil, err
	}

	browsingEnabled, exists, err := unstructured.NestedBool(target.Object, "status", "browsingEnabled")
	if err != nil {
		return nil, err
	}

	if exists && !browsingEnabled {
		return nil, fmt.Errorf("browsing is not enabled for given target %s namespace %s",
			targetBrowserConfig.TargetName, targetBrowserConfig.TargetNamespace)
	}

	return target, nil
}

// getTvkHostAndTargetBrowserAPIPath gets tvkHost name and targetBrowserAPIPath from target-browser's ingress
func (targetBrowserConfig *Config) getTvkHostAndTargetBrowserAPIPath(ctx context.Context, cl client.Client,
	target *unstructured.Unstructured) (tvkHost, targetBrowserPath string, err error) {

	ingressList := v1beta1.IngressList{}
	if err := cl.List(ctx, &ingressList, client.InNamespace(targetBrowserConfig.TargetNamespace)); err != nil {
		return "", "", err
	}

	var hostFound bool

	for i := range ingressList.Items {
		ing := ingressList.Items[i]
		ownerRefs := ing.GetOwnerReferences()
		for j := range ownerRefs {
			ownerRef := ownerRefs[j]
			if ownerRef.Kind == tools.TargetKind && ownerRef.UID == target.GetUID() {
				tvkHost = ing.Spec.Rules[0].Host
				targetBrowserPath = ing.Spec.Rules[0].HTTP.Paths[0].Path
				if tvkHost == "" || targetBrowserPath == "" {
					return "", "", fmt.Errorf("either tvkHost or targetBrowserPath could not retrieved from"+
						" ingress %s namespace %s", ing.Name, ing.Namespace)
				}
				hostFound = true
				break
			}
		}
		if hostFound {
			break
		}
	}

	return tvkHost, targetBrowserPath, nil
}
