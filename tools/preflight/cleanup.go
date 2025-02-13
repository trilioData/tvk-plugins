package preflight

import (
	"context"
	"errors"
	"strings"

	"github.com/trilioData/tvk-plugins/internal"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CleanupOptions struct {
	UID string `json:"uid,omitempty"`
}

type Cleanup struct {
	CleanupOptions
	CommonOptions
}

func (co *Cleanup) logCleanupOptions() {
	co.Logger.Infoln("====PREFLIGHT CLEANUP OPTIONS====")
	co.CommonOptions.logCommonOptions()
	co.Logger.Infof("UID=\"%s\"", co.UID)
	co.Logger.Infoln("====PREFLIGHT CLEANUP OPTIONS END====")
}

// CleanupPreflightResources cleans the preflight resources.
// if uid is provided then preflight resources of particular uid are cleaned.
// if uid is empty then all preflight resources are cleaned.
func (co *Cleanup) CleanupPreflightResources(ctx context.Context) error {
	co.logCleanupOptions()
	co.Logger.Infoln("Cleaning all preflight resources")
	var (
		allSuccess = true
		err        error
		deleteNs   = internal.DefaultNs
	)
	gvkList, err := getCleanupResourceGVKList(kubeClient.ClientSet)
	if err != nil {
		return err
	}
	resLabels := map[string]string{
		LabelTrilioKey: LabelTvkPreflightValue,
	}
	if co.Namespace != "" {
		deleteNs = co.Namespace
	}

	for _, gvk := range gvkList {
		var resList = unstructured.UnstructuredList{}
		resList.SetGroupVersionKind(gvk)
		if err = kubeClient.RuntimeClient.List(ctx, &resList, client.MatchingLabels(resLabels), client.InNamespace(deleteNs)); err != nil {
			co.Logger.Errorf("Error fetching %s(s)  :: %s\n", gvk.Kind, err.Error())
			allSuccess = false
			continue
		}
		for _, res := range resList.Items {
			res := res
			co.Logger.Infof("Cleaning %s - %s", res.GetKind(), res.GetName())
			err = deleteK8sResource(ctx, &res, kubeClient.RuntimeClient)
			if err != nil {
				if !k8serrors.IsNotFound(err) {
					allSuccess = false
					co.Logger.Errorf("problem occurred deleting %s - %s :: %s", res.GetKind(), res.GetName(), err.Error())
				}
			}
		}
	}

	// Delete the backup namespace and all the resources in it, created by preflight tvk-plugin.
	namespaceList := &corev1.NamespaceList{}
	err = kubeClient.RuntimeClient.List(ctx, namespaceList, client.MatchingLabels(resLabels))
	if err != nil {
		co.Logger.Errorf("Error fetching namespaces with label %s :: %s\n", resLabels, err.Error())
		allSuccess = false
	}

	for _, ns := range namespaceList.Items {
		if strings.HasPrefix(ns.Name, BackupNamespacePrefix) {
			backupNs := ns
			co.Logger.Infof("Cleaning namespace - %s", ns.GetName())
			err = deleteK8sResource(ctx, &backupNs, kubeClient.RuntimeClient)
			if err != nil {
				if !k8serrors.IsNotFound(err) {
					allSuccess = false
					co.Logger.Errorf("problem occurred deleting namespace - %s :: %s", ns.GetName(), err.Error())
				}
			}
		}
	}

	if allSuccess {
		co.Logger.Infoln("All preflight resources cleaned")
		return nil
	}

	return errors.New("deletion of some resources failed in cleanup process")
}

func getCleanupResourceGVKList(cl *kubernetes.Clientset) ([]schema.GroupVersionKind, error) {
	cleanupResourceList := make([]schema.GroupVersionKind, 0)

	snapVerList, err := getVersionsOfGroup(StorageSnapshotGroup, cl)
	if err != nil {
		return nil, err
	}
	for _, ver := range snapVerList {
		cleanupResourceList = append(cleanupResourceList, schema.GroupVersionKind{
			Group:   StorageSnapshotGroup,
			Version: ver,
			Kind:    internal.VolumeSnapshotKind,
		})
	}
	// i need namespace kind as
	cleanupResourceList = append(cleanupResourceList,
		corev1.SchemeGroupVersion.WithKind(internal.PersistentVolumeClaimKind),
		corev1.SchemeGroupVersion.WithKind(internal.PodKind))

	return cleanupResourceList, nil
}
