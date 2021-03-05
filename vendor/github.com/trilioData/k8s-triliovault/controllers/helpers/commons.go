package helpers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	utilretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
)

type PublishEventConfig struct {
	Publish       bool
	OperationType string
	EventType     string
	Status        v1.Status
}

func UpdateObjectStatus(ctx context.Context, cl client.Client, logger logr.Logger, recorder record.EventRecorder,
	object client.Object, eventConfig PublishEventConfig) error {
	log := logger.WithValues("function", "UpdateObjectStatus")

	if object == nil {
		return apierrs.NewBadRequest(errors.New("nil object parameter").Error())
	}
	objectKind := object.GetObjectKind().GroupVersionKind().Kind

	if eventConfig.Publish && eventConfig.OperationType == "" || eventConfig.Status == "" || eventConfig.EventType == "" {
		return apierrs.NewBadRequest(errors.New("empty parameters to publish event").Error())
	}

	metaObj, err := meta.Accessor(object)
	if err != nil {
		log.Error(err, "Error converting the runtime object to meta object")
		return apierrs.NewBadRequest(err.Error())
	}

	retryErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {
		log.Info("Getting the latest object", "gvk", object.GetObjectKind().GroupVersionKind().String())
		currObject := &unstructured.Unstructured{}
		currObject.SetGroupVersionKind(object.GetObjectKind().GroupVersionKind())
		rErr := cl.Get(ctx, types.NamespacedName{Name: metaObj.GetName(), Namespace: metaObj.GetNamespace()}, currObject)
		if rErr != nil {
			return rErr
		}
		rErr = cl.Status().Patch(ctx, object, client.MergeFrom(currObject))
		if rErr != nil {
			return rErr
		}
		return nil
	})

	if retryErr != nil {
		log.Info("warning: error occurred while patching the new resource", "error", retryErr.Error())
		uErr := cl.Status().Update(ctx, object)
		if uErr != nil {
			log.Error(uErr, "Error while updating job controller status")
			utilruntime.HandleError(fmt.Errorf("error updating the status of %s: %s", objectKind, metaObj.GetName()))
			recorder.Eventf(object, corev1.EventTypeWarning, internal.StatusUpdateFailed,
				"Updating %s status, Failed", eventConfig.Status)
			return apierrs.NewBadRequest(uErr.Error())
		}
	}

	if eventConfig.Publish {
		recorder.Eventf(object, eventConfig.EventType, fmt.Sprintf("%s%s", eventConfig.OperationType, eventConfig.Status),
			"Operation %s is in %s state", eventConfig.OperationType, eventConfig.Status)
	}
	log.Info("Updated status", "status", eventConfig.Status, "kind", objectKind,
		"name", metaObj.GetName(), "namespace", metaObj.GetNamespace())
	return nil
}

func CheckUpdateEventPredicate(e event.UpdateEvent, logger logr.Logger) bool {

	var log = logger.WithValues("function", "CheckUpdateEventPredicate")

	if e.ObjectOld == nil {
		log.Error(nil, "Update event has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "Update event has no new runtime object for update", "event", e)
		return false
	}

	return true
}

func ContinueReconcile(namespace string) bool {
	if internal.GetAppScope() == internal.NamespacedScope && internal.GetInstallNamespace() != namespace {
		return false
	}
	return true
}

func IgnoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

func GetControllerOwner(obj client.Object) (*types.NamespacedName, bool) {
	annotations := obj.GetAnnotations()
	controllerName, controllerNamePresent := annotations[internal.ControllerOwnerName]
	controllerNamespace, controllerNamespacePresent := annotations[internal.ControllerOwnerNamespace]
	if controllerNamePresent && controllerNamespacePresent {
		return &types.NamespacedName{Namespace: controllerNamespace, Name: controllerName}, true
	}

	return nil, false
}

// ImplementDataComponentConditions will add current phase conditions for restore data components
func ImplementDataComponentConditions(objectType interface{}, status v1.Status, pvcName string, phase v1.OperationType) {

	var (
		condition v1.Conditions
	)

	// check for avoiding appending of condition while status is blank
	if status == "" {
		return
	}
	condition.Phase = phase
	condition.Reason = internal.CombinedReason(string(phase), string(status))
	condition.Status = status
	condition.Timestamp = internal.CurrentTime()

	switch object := objectType.(type) {
	case *v1.Custom:
		customSnapshot := object
		for j := range customSnapshot.DataSnapshots {
			dSnapshot := &customSnapshot.DataSnapshots[j]
			if pvcName == dSnapshot.PersistentVolumeClaimName {
				if isDataComponentConditionRequired(dSnapshot.Conditions, phase, status) {
					dSnapshot.Conditions = append(dSnapshot.Conditions, condition)
				}
				return
			}
		}

	case *v1.Helm:
		helmChart := object
		for k := range helmChart.DataSnapshots {
			dSnapshot := &helmChart.DataSnapshots[k]
			if pvcName == dSnapshot.PersistentVolumeClaimName {
				if isDataComponentConditionRequired(dSnapshot.Conditions, phase, status) {
					dSnapshot.Conditions = append(dSnapshot.Conditions, condition)
				}
				return
			}
		}

	case *v1.Operator:
		operator := object
		for k := range operator.DataSnapshots {
			dSnapshot := &operator.DataSnapshots[k]
			if pvcName == dSnapshot.PersistentVolumeClaimName {
				if isDataComponentConditionRequired(dSnapshot.Conditions, phase, status) {
					dSnapshot.Conditions = append(dSnapshot.Conditions, condition)
				}
				return
			}
		}
		if operator.Helm != nil {
			for k := range operator.Helm.DataSnapshots {
				helmDS := &operator.Helm.DataSnapshots[k]
				if pvcName == helmDS.PersistentVolumeClaimName {
					if isDataComponentConditionRequired(helmDS.Conditions, phase, status) {
						helmDS.Conditions = append(helmDS.Conditions, condition)
					}
					return
				}
			}
		}
	}
}

func isDataComponentConditionRequired(conditions []v1.Conditions, phase v1.OperationType, status v1.Status) bool {
	if len(conditions) > 0 {
		for i := len(conditions) - 1; i >= 0; i-- {
			c := conditions[i]
			if c.Phase == phase && c.Status == status {
				return false
			}
		}
	}
	return true
}
