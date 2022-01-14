package preflight

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/tvk-plugins/internal"
)

type CleanupOptions struct {
	CommonOptions
}

// CleanupPreflightResources cleans the preflight resources.
// if uid is provided then preflight resources of particular uid are cleaned.
// if uid is empty then all preflight resources are cleaned.
func (o *CleanupOptions) CleanupPreflightResources(ctx context.Context, uid string) error {
	o.Logger.Infoln("Cleaning all preflight resources")
	var allSuccess = true
	var err error
	gvkList, err := getCleanupResourceGVKList()
	if err != nil {
		return err
	}
	resLabels := map[string]string{
		labelTrilioKey: labelTvkPreflightValue,
	}
	if uid != "" {
		resLabels[labelPreflightRunKey] = uid
	}
	for _, gvk := range gvkList {
		var resList = unstructured.UnstructuredList{}
		resList.SetGroupVersionKind(gvk)
		if err = runtimeClient.List(ctx, &resList, client.MatchingLabels(resLabels)); err != nil {
			o.Logger.Errorf("Error fetching %s(s)  :: %s\n", gvk.Kind, err.Error())
			allSuccess = false
			continue
		}
		for _, res := range resList.Items {
			res := res
			err = o.cleanResource(ctx, &res)
			if err != nil {
				allSuccess = false
				o.Logger.Errorf("problem occurred deleting %s - %s :: %s", res.GetKind(), res.GetName(), err.Error())
			}
		}
	}
	if allSuccess {
		o.Logger.Infoln("All preflight resources cleaned")
		return nil
	}

	return errors.New("deletion of some resources failed in cleanup process")
}

func (o *CleanupOptions) cleanResource(ctx context.Context, resource *unstructured.Unstructured) error {
	var err error
	o.Logger.Infof("Cleaning %s - %s", resource.GetKind(), resource.GetName())
	err = deleteK8sResourceWithForceTimeout(ctx, resource, o.Logger)
	if err != nil {
		o.Logger.Errorf("%s Error cleaning %s - %s :: %s\n",
			cross, resource.GetKind(), resource.GetName(), err.Error())
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
			Group:   storageSnapshotGroup,
			Version: ver,
			Kind:    internal.VolumeSnapshotKind,
		})
	}

	return cleanupResourceList, nil
}
