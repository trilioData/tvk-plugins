package preflight

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal"
)

type CleanupOptions struct {
	Ctx        context.Context
	Kubeconfig string
	Namespace  string
	Logger     *logrus.Logger
}

func (o *CleanupOptions) CleanupByUID(uid string) error {
	o.Logger.Infoln(fmt.Sprintf("Cleaning preflight resources with uid - %s", uid))
	var err error
	var allSuccess = true
	gvkList, err := getCleanupResourceGVKList()
	if err != nil {
		return err
	}
	resList := unstructured.UnstructuredList{}

	for _, gvk := range gvkList {
		resList.SetGroupVersionKind(gvk)
		if err = runtimeClient.List(o.Ctx, &resList, client.ListOption(client.InNamespace(o.Namespace))); err != nil {
			o.Logger.Errorln(fmt.Sprintf("Error fetching %s(s)  :: %s", gvk.Kind, err.Error()))
			continue
		}
		for _, res := range resList.Items {
			res := res
			resUID, ok := res.GetLabels()[labelPreflightRunKey]
			if ok && resUID == uid {
				err = o.cleanResource(&res)
				if err != nil {
					allSuccess = false
				}
			}
		}
	}
	if allSuccess {
		o.Logger.Infoln(fmt.Sprintf("preflight resources cleaned for uid - %s", uid))
		return nil
	}

	return errors.New("deletion of some resources failed in cleanup process")
}

func (o *CleanupOptions) CleanAllPreflightResources() error {
	o.Logger.Infoln("Cleaning all preflight resources")
	var allSuccess = true
	var err error
	gvkList, err := getCleanupResourceGVKList()
	if err != nil {
		return err
	}
	resList := unstructured.UnstructuredList{}
	for _, gvk := range gvkList {
		resList.SetGroupVersionKind(gvk)
		if err = runtimeClient.List(o.Ctx, &resList, client.ListOption(client.InNamespace(o.Namespace))); err != nil {
			o.Logger.Errorln(fmt.Sprintf("Error fetching %s(s)  :: %s", gvk.Kind, err.Error()))
			allSuccess = false
			continue
		}
		for _, res := range resList.Items {
			res := res
			if _, ok := res.GetLabels()[labelPreflightRunKey]; ok {
				err = o.cleanResource(&res)
				if err != nil {
					allSuccess = false
				}
			}
		}
	}
	if allSuccess {
		o.Logger.Infoln("All preflight resources cleaned")
		return nil
	}

	return errors.New("deletion of some resources failed in cleanup process")
}

func (o *CleanupOptions) cleanResource(resource *unstructured.Unstructured) error {
	var err error
	o.Logger.Infoln(fmt.Sprintf("Cleaning %s - %s",
		resource.GetKind(), resource.GetName()))
	err = deleteK8sResourceWithForceTimeout(o.Ctx, resource, o.Logger)
	if err != nil {
		o.Logger.Errorln(fmt.Sprintf("%s Error cleaning %s - %s :: %s",
			cross, resource.GetKind(), resource.GetName(), err.Error()))
	}
	return err
}

func getCleanupResourceGVKList() ([]schema.GroupVersionKind, error) {
	cleanupResourceList := make([]schema.GroupVersionKind, 0)
	cleanupResourceList = append(cleanupResourceList, schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.PodKind,
	}, schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.PersistentVolumeClaimKind,
	})

	snapVerList, err := getVersionsOfGroup(storageSnapshotGroup)
	if err != nil {
		return nil, err
	}
	for _, ver := range snapVerList {
		cleanupResourceList = append(cleanupResourceList, schema.GroupVersionKind{
			Group:   "snapshot.storage.k8s.io",
			Version: ver,
			Kind:    internal.VolumeSnapshotKind,
		})
	}

	return cleanupResourceList, nil
}
