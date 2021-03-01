package helpers

import (
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// TODO This is redundant code and it needs lot of code refactoring.
// TODO Ideally we should use the method written in /pkg/metamover/snapshot/parser/commons.go
// TODO Take case of cyclic imports in the code refactoring
// FindParentResource function operates recursively to find the top most parent of the resource passed using the client and namespaces
//
// All the owner references are fetched for the passed resource, each owner is checked for the `ControlledBy == true` field,
// If it is the case it is checked if that resource exists, then same function is called for the owner resource recursively.
// If the owner ref is present but the owner resource does not exist or if there is no such owner, then the same resource os returned.
func FindParentResource(a *kube.Accessor, res unstructured.Unstructured,
	ns string) (parentRes unstructured.Unstructured, err error) {

	ownerRefs := res.GetOwnerReferences()
	cl := a.GetKubeClient()
	ctx := a.GetContext()
	for i := range ownerRefs {
		ownRef := ownerRefs[i]
		if ownRef.Controller == nil || !*ownRef.Controller {
			continue
		}
		pObj := &unstructured.Unstructured{}
		gvk := schema.FromAPIVersionAndKind(ownRef.APIVersion, ownRef.Kind)
		pObj.SetGroupVersionKind(gvk)
		err := cl.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      ownRef.Name,
		}, pObj)

		if err != nil {
			log.Warnf("Error while finding the parent resource [%s: %s] of resource: [%s]", ownRef.Name,
				pObj.GroupVersionKind().String(), res.GetName())

			return parentRes, err
		}
		return FindParentResource(a, *pObj, ns)
	}

	return res, nil
}
