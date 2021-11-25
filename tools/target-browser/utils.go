package targetbrowser

import (
	"context"
	"fmt"
	"path"
	"strconv"

	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/api/networking/v1beta1"
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
func getTvkHostAndTargetBrowserAPIPath(ctx context.Context, cl client.Client, target *unstructured.Unstructured,
	isIngressNetworkingV1Resource bool) (tvkHost, targetBrowserPath string, err error) {

	ingressList := unstructured.UnstructuredList{}
	ingressList.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind(internal.IngressKind))
	if !isIngressNetworkingV1Resource {
		ingressList.SetGroupVersionKind(v1beta1.SchemeGroupVersion.WithKind(internal.IngressKind))
	}

	err = cl.List(ctx, &ingressList, client.InNamespace(target.GetNamespace()))
	if err != nil {
		return "", "", err
	}

	for i := range ingressList.Items {
		ing := ingressList.Items[i]
		ownerRefs := ing.GetOwnerReferences()
		for j := range ownerRefs {
			ownerRef := ownerRefs[j]
			if ownerRef.Kind == target.GetKind() && ownerRef.UID == target.GetUID() {
				if isIngressNetworkingV1Resource {
					resource := &v1.Ingress{}
					if err = runtime.DefaultUnstructuredConverter.FromUnstructured(ing.Object, resource); err != nil {
						return
					}
					tvkHost = resource.Spec.Rules[0].Host
					targetBrowserPath = resource.Spec.Rules[0].HTTP.Paths[0].Path
				} else {
					resource := &v1beta1.Ingress{}
					if err = runtime.DefaultUnstructuredConverter.FromUnstructured(ing.Object, resource); err != nil {
						return
					}
					tvkHost = resource.Spec.Rules[0].Host
					targetBrowserPath = resource.Spec.Rules[0].HTTP.Paths[0].Path
				}

				if tvkHost == "" || targetBrowserPath == "" {
					log.Warnf("either tvkHost or targetBrowserPath could not retrieved from"+
						" target browser's ingress %s namespace %s", ing.GetName(), ing.GetNamespace())
					continue
				}
				return
			}
		}
	}
	return tvkHost, targetBrowserPath, nil
}

func getTrilioResourcesAPIPath(uid string) string {
	return path.Join(internal.BackupAPIPath, uid, internal.TrilioResourcesAPIPath)
}

func getNodePortAndServiceType(ctx context.Context, cl client.Client,
	target *unstructured.Unstructured) (nodePortHTTP, nodePortHTTPS, svcType string, err error) {
	ingressService := &corev1.Service{}
	err = cl.Get(ctx, types.NamespacedName{Name: internal.IngressServiceLabel, Namespace: target.GetNamespace()}, ingressService)
	if err != nil {
		log.Error(err, "error while getting ingress service")
		return "", "", "", err
	}
	svcType = string(ingressService.Spec.Type)
	for index := range ingressService.Spec.Ports {
		port := ingressService.Spec.Ports[index]
		if port.Port == 80 {
			nodePortHTTP = strconv.Itoa(int(port.NodePort))
		} else if port.Port == 443 {
			nodePortHTTPS = strconv.Itoa(int(port.NodePort))
		}
	}
	return
}
