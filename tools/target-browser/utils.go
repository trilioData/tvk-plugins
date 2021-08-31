package targetbrowser

import (
	"context"
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal"
)

type Config struct {
	Scheme                                          *runtime.Scheme
	KubeConfig, TargetName, TargetNamespace, CaCert string
	InsecureSkipTLS, UseHTTPS                       bool
}

// validateTarget validates if provided target is present target-namespace and browsing is enabled for it.
func (targetBrowserConfig *Config) validateTarget(ctx context.Context, cl client.Client) (*unstructured.Unstructured, error) {
	// get target
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   internal.TriliovaultGroup,
		Version: internal.V1Version,
		Kind:    internal.TargetKind,
	})

	if err := cl.Get(ctx, types.NamespacedName{Namespace: targetBrowserConfig.TargetNamespace, Name: targetBrowserConfig.TargetName},
		target); err != nil {
		return nil, err
	}

	browsingEnabled, exists, err := unstructured.NestedBool(target.Object, "status", "browsingEnabled")
	if err != nil {
		return nil, err
	}

	if !exists || !browsingEnabled {
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
			if ownerRef.Kind == internal.TargetKind && ownerRef.UID == target.GetUID() {
				tvkHost = ing.Spec.Rules[0].Host
				targetBrowserPath = ing.Spec.Rules[0].HTTP.Paths[0].Path
				if tvkHost == "" || targetBrowserPath == "" {
					log.Warnf("either tvkHost or targetBrowserPath could not retrieved from"+
						" target browser's ingress %s namespace %s", ing.Name, ing.Namespace)
					continue
				}
				hostFound = true
				break
			}
		}
		if hostFound {
			break
		}
	}

	if tvkHost == "" || targetBrowserPath == "" {
		return tvkHost, targetBrowserPath, fmt.Errorf("either tvkHost or targetBrowserPath could not retrieved for"+
			" target %s namespace %s", targetBrowserConfig.TargetName, targetBrowserConfig.TargetNamespace)
	}

	return tvkHost, targetBrowserPath, nil
}

func getTrilioResourcesAPIPath(uid string) string {
	return path.Join(internal.BackupAPIPath, uid, internal.TrilioResourcesAPIPath)
}
