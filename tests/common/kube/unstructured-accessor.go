package kube

import (
	"fmt"
	"time"

	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	apiErrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	kScheme "k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util"

	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes/scheme"
	utilretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//"github.com/trilioData/k8s-triliovault/internal"
)

// CreateUnstructuredObject creates the given Unstructured object in the passed namespace.
func (a *Accessor) CreateUnstructuredObject(namespace string, unStr *unstructured.Unstructured, opts ...client.CreateOption) error {
	unStr.SetNamespace(namespace)
	return a.client.Create(a.context, unStr, opts...)
}

// UpdateUnstructuredObject updates the given Unstructured object in the passed namespace.
func (a *Accessor) UpdateUnstructuredObject(namespace string, unStr *unstructured.Unstructured) error {
	unStr.SetNamespace(namespace)
	return a.client.Update(a.context, unStr)
}

// MergePatchUnstructuredObject patches the given Unstructured object in the passed namespace.
func (a *Accessor) MergePatchUnstructuredObject(namespace string, unStr, newObject *unstructured.Unstructured,
	opts ...client.PatchOption) error {
	unStr.SetNamespace(namespace)
	return a.client.Patch(a.context, unStr, client.MergeFrom(newObject), opts...)
}

// ThreeWayPatchUnstructuredObject patches the given Unstructured object in the passed namespace using three way path in
// strategic merge patch or merge patch strategy.
func (a *Accessor) ThreeWayPatchUnstructuredObject(namespace string, gvk schema.GroupVersionKind, current *unstructured.Unstructured,
	opts ...client.PatchOption) error {
	var (
		getErr error
		source = current.GetName()
	)

	modified, err := util.GetModifiedConfiguration(current, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		log.Errorf("\n%+v\n", cmdutil.AddSourceToErr(fmt.Sprintf("retrieving modified configuration from:"+
			"\n%s\nfor:", source), source, err))
	}

	newObject, gErr := a.GetUnstructuredObject(types.NamespacedName{Name: source, Namespace: namespace}, gvk)
	if apiErrs.IsNotFound(gErr) {
		// Create the resource if it doesn't exist
		// First, update the annotation used by kubectl apply
		log.Warnf("error while getting the latest object to create merge, creating a new instance")
		if cErr := util.CreateApplyAnnotation(current, unstructured.UnstructuredJSONScheme); err != nil {
			log.Errorf("\n%+v\n", cmdutil.AddSourceToErr("creating", source, cErr).Error())
		}

		// Then create the resource and skip the three-way merge
		current.SetResourceVersion("")
		err = a.CreateUnstructuredObject(namespace, current)
		if err != nil {
			return cmdutil.AddSourceToErr("creating", source, err)
		}
		return nil
	} else if gErr != nil {
		return gErr
	}

	err = a.patchSimple(gvk, newObject, modified, opts...)
	retryErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
		current, getErr = a.GetUnstructuredObject(types.NamespacedName{Name: source, Namespace: namespace}, gvk)
		if getErr != nil {
			return getErr
		}
		err = a.patchSimple(gvk, current, modified, opts...)
		return err
	})

	if retryErr != nil && (apiErrs.IsConflict(retryErr) || apiErrs.IsInvalid(retryErr)) {
		//nolint
		// err = a.deleteAndCreate(types.NamespacedName{Namespace: namespace, Name: source}, gvk, current, modified)
		log.Errorf("error while applying patch; could not update the object %s, %s",
			gvk, types.NamespacedName{Namespace: namespace, Name: source})
		return retryErr
	}

	return nil
}

// ApplyPatchUnstructuredObject patches the given Unstructured object in the passed namespace.
func (a *Accessor) ApplyPatchUnstructuredObject(namespace string, unStr *unstructured.Unstructured,
	opts ...client.PatchOption) error {
	unStr.SetNamespace(namespace)
	return a.client.Patch(a.context, unStr, client.Apply, opts...)
}

// JSONPatchUnstructuredObject patches the given Unstructured object in the passed namespace.
func (a *Accessor) JSONPatchUnstructuredObject(namespace string, unStr *unstructured.Unstructured, payLoad []byte,
	opts ...client.PatchOption) error {
	unStr.SetNamespace(namespace)
	return a.client.Patch(a.context, unStr, client.RawPatch(types.JSONPatchType, payLoad), opts...)
}

// DeleteUnstructuredObject deletes the given Unstructured object in the passed namespace.
func (a *Accessor) DeleteUnstructuredObject(nsName types.NamespacedName, gvk schema.GroupVersionKind, opts ...client.DeleteOption) error {
	unStr := &unstructured.Unstructured{}
	unStr.SetNamespace(nsName.Namespace)
	unStr.SetName(nsName.Name)
	unStr.SetGroupVersionKind(gvk)
	return a.client.Delete(a.context, unStr, opts...)
}

// GetUnstructuredObject returns the unstructured object with the given namespace and name.
func (a *Accessor) GetUnstructuredObject(nsName types.NamespacedName, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	unStr := &unstructured.Unstructured{}
	unStr.SetGroupVersionKind(gvk)
	err := a.client.Get(a.context, nsName, unStr)
	return unStr, err
}

// nolint:interfacer // Need to use unstructured as we don't know the type of object to be patched
func (a *Accessor) patchSimple(gvk schema.GroupVersionKind, obj *unstructured.Unstructured,
	modified []byte, opts ...client.PatchOption) error {
	source := obj.GetName()

	// Serialize the current configuration of the object from the server.
	current, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return cmdutil.AddSourceToErr(fmt.Sprintf("serializing current configuration from:\n%v\nfor:", obj), source, err)
	}

	// Retrieve the original configuration of the object from the annotation.
	original, err := util.GetOriginalConfiguration(obj)
	if err != nil {
		return cmdutil.AddSourceToErr(fmt.Sprintf("retrieving original configuration from:\n%v\nfor:", obj), source, err)
	}

	var patchType types.PatchType
	var patch []byte
	var lookupPatchMeta strategicpatch.LookupPatchMeta
	createPatchErrFormat := "creating patch with:\noriginal:\n%s\nmodified:\n%s\ncurrent:\n%s\nfor:"

	// Create the versioned struct from the type defined in the restmapping
	// (which is the API version we'll be submitting the patch to)
	versionedObject, vErr := kScheme.Scheme.New(gvk)

	gv := gvk.GroupVersion()
	newVersionedObject, _ := runtime.ObjectConvertor(scheme.Scheme).ConvertToVersion(obj, gv)

	// Unstructured objects, such as CRDs, may not have an not registered error
	// returned from ConvertToVersion. Anything that's unstructured should
	// use the jsonpatch.CreateMergePatch. Strategic Merge Patch is not supported
	// on objects like CRDs.
	_, isUnstructured := newVersionedObject.(runtime.Unstructured)

	// On newer K8s versions, CRDs aren't unstructured but has this dedicated type
	_, isCRD := newVersionedObject.(*extv1beta1.CustomResourceDefinition)

	switch {
	case isCRD || isUnstructured:
		var resopts []client.PatchOption
		for _, i := range opts {
			copts, ok := i.(*client.PatchOptions)
			if ok {
				resopts = []client.PatchOption{&client.PatchOptions{
					DryRun: copts.DryRun,
				}}
			}
		}
		opts = resopts
		patchType = types.MergePatchType
		patch, err = jsonpatch.CreateMergePatch(original, current)
		if err != nil {
			return cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
		}
	case runtime.IsNotRegisteredError(vErr):
		// fall back to generic JSON merge patch
		patchType = types.MergePatchType
		preconditions := []mergepatch.PreconditionFunc{mergepatch.RequireKeyUnchanged("apiVersion"),
			mergepatch.RequireKeyUnchanged("kind"), mergepatch.RequireMetadataKeyUnchanged("name")}
		patch, err = jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current, preconditions...)
		if err != nil {
			if mergepatch.IsPreconditionFailed(err) {
				return fmt.Errorf("%s", "At least one of apiVersion, kind and name was changed")
			}
			return cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
		}
	case vErr != nil:
		return cmdutil.AddSourceToErr(fmt.Sprintf("getting instance of versioned object for %v:", gvk), source, err)
	default:
		// Compute a three way strategic merge patch to send to server.
		patchType = types.StrategicMergePatchType
		lookupPatchMeta, err = strategicpatch.NewPatchMetaFromStruct(versionedObject)
		if err != nil {
			return cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
		}
		patch, err = strategicpatch.CreateThreeWayMergePatch(original, modified, current, lookupPatchMeta, true)
		if err != nil {
			return cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
		}
	}

	if string(patch) == "{}" {
		return nil
	}

	return a.client.Patch(a.context, obj, client.RawPatch(patchType, patch), opts...)
}

//nolint
func (a *Accessor) deleteAndCreate(nsName types.NamespacedName, gvk schema.GroupVersionKind, original runtime.Object,
	modified []byte) error {
	log.Infof("conflicting error while updating the object, force deleting and recreating. %s, %s", gvk, nsName)
	if err := a.DeleteUnstructuredObject(nsName, gvk, client.GracePeriodSeconds(0)); err != nil {
		log.Errorf(fmt.Sprintf("Error while deleting the object: %s", err.Error()))
		return err
	}
	// TODO: use wait
	if err := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		var gErr error
		if _, gErr = a.GetUnstructuredObject(nsName, gvk); gErr != nil && apiErrs.IsNotFound(gErr) {
			return true, nil
		}
		log.Error(fmt.Sprintf("polled to check the object with error: %+v", gErr))
		return false, gErr
	}); err != nil {
		log.Error(fmt.Sprintf("Error while checking the deletion of object: %s", err.Error()))
		return err
	}

	versionedObject, _, err := unstructured.UnstructuredJSONScheme.Decode(modified, nil, nil)
	if err != nil {
		return err
	}

	temp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(versionedObject)
	if err != nil {
		return err
	}

	vo := &unstructured.Unstructured{Object: temp}
	vo.SetGroupVersionKind(gvk)
	vo.SetResourceVersion("")

	log.Info("deleted the old one with conflicting error, now creating a new object")
	err = a.client.Create(a.context, vo)
	if err != nil {
		log.Warnf("error while re-creating modified object, trying to create original")
		// restore the original object if we fail to create the new one
		// but still propagate and advertise error to user
		temp, cErr := runtime.DefaultUnstructuredConverter.ToUnstructured(original)
		if cErr != nil {
			return cErr
		}
		vo := &unstructured.Unstructured{Object: temp}
		vo.SetGroupVersionKind(gvk)

		vo.SetResourceVersion("")
		recreateErr := a.client.Create(a.context, vo)
		if recreateErr != nil {
			return fmt.Errorf("error occurred force-replacing the existing object with the newly provided one:\n"+
				"\n%v.\n\nAdditionally, an error occurred attempting to restore the original object:\n\n%v", err, recreateErr)
		}
	}

	log.Info("checking if the object is created")
	if err := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		var gErr error
		if _, gErr = a.GetUnstructuredObject(nsName, gvk); gErr == nil {
			return true, nil
		}
		log.Error(fmt.Sprintf("polled to check the object with error: %+v", gErr))
		return false, gErr
	}); err != nil {
		log.Error(fmt.Sprintf("Error while checking the creation of object: %s", err.Error()))
		return err
	}
	return nil
}
