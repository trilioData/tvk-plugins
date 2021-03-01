package helpers

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OwnerRelations interface {
	GetChildrenForOwner(owner runtime.Object) runtime.Object
	SetOwnerRefToList(owner runtime.Object)
	RemoveList(ctx context.Context, cl client.Client) error
}

type UnstructuredResourceList unstructured.UnstructuredList

func (u *UnstructuredResourceList) GetChildrenForOwner(owner runtime.Object) UnstructuredResourceList {
	children := UnstructuredResourceList{}
	log := ctrl.Log.WithName("UnstructResource Utility").WithName("GetChildrenForOwner")

	if owner == nil || len(u.Items) == 0 {
		return children
	}
	metaOwner, err := meta.Accessor(owner)
	if err != nil {
		log.Error(err, "Error while converting the owner to meta accessor format")
		return children
	}
	matchUID := metaOwner.GetUID()
	for _, item := range u.Items {
		refs := item.GetOwnerReferences()
		for i := 0; i < len(refs); i++ {
			or := refs[i]
			if or.UID == matchUID {
				children.Items = append(children.Items, item)
			}
		}
	}

	return children
}

func (u *UnstructuredResourceList) SetOwnerRefToList(owner runtime.Object) {
	log := ctrl.Log.WithName("UnstructResource Utility").WithName("SetOwnerRefToList")
	if owner == nil || len(u.Items) == 0 {
		log.Error(errors.New("invalid Params: No items in the list to process or nil owner"), "")
		return
	}
	metaOwner, err := meta.Accessor(owner)
	if err != nil {
		log.Error(err, "Error while converting the owner to meta accessor format")
		return
	}
	ownerRef := metav1.NewControllerRef(metaOwner, owner.GetObjectKind().GroupVersionKind())
	for _, item := range u.Items {
		item.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})
	}
}

func (u *UnstructuredResourceList) RemoveList(ctx context.Context, cl client.Client) error {
	log := ctrl.Log.WithName("UnstructResource Utility").WithName("RemoveList")

	for _, item := range u.Items {
		tmpItem := item
		err := cl.Delete(ctx, &tmpItem)
		if err != nil {
			log.Info(fmt.Sprintf("ERROR deleting the instance of a resource: %s", err.Error()),
				"gvk", item.GroupVersionKind().String())
			return err
		}
	}
	return nil
}

func (u *UnstructuredResourceList) CleanList(ctx context.Context, cl client.Client) int {
	log := ctrl.Log.WithName("UnstructResource Utility").WithName("CleanList")

	var cleanupCount int
	for _, item := range u.Items {
		tmpItem := item
		err := cl.Delete(ctx, &tmpItem)
		if err != nil {
			log.Error(err, "Error deleting the instance of a resource", "gvk", item.GroupVersionKind().String())
			utilruntime.HandleError(err)
			continue
		}
		cleanupCount++
	}

	return cleanupCount
}
