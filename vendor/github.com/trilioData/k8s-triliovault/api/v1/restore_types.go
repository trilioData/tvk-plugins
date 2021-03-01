package v1

import (
	corev1api "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupSourceType defines the type of source for restore
// +kubebuilder:validation:Enum=Backup;Location;BackupPlan
type RestoreSourceType string

const (
	// BackupSource means that the restore is performed from backup instance
	BackupSource RestoreSourceType = "Backup"

	// LocationSource means that the restore is performed from remote target location
	LocationSource RestoreSourceType = "Location"

	// BackupPlanSource means that the restore is performed from backup instance
	BackupPlanSource RestoreSourceType = "BackupPlan"
)

// ComponentStatus defines the details of restore of application component.
type ComponentStatus struct {

	// ExistingResource specifies the resources already existing in cluster defined in application.
	// +kubebuilder:validation:Optional
	ExistingResources []Resource `json:"existingResource,omitempty"`

	// SkippedResources specifies the resources skipped while restoring.
	// +kubebuilder:validation:Optional
	SkippedResources []Resource `json:"skippedResources,omitempty"`

	// FailedResources specifies the resources for which the restore operation failed
	// +kubebuilder:validation:Optional
	FailedResources []Resource `json:"failedResources,omitempty"`

	// NewResourcesAdded specifies the resources added(duplicated and modified) during restore.
	// +kubebuilder:validation:Optional
	NewResourcesAdded []Resource `json:"newResourcesAdded,omitempty"`

	// ExcludedResources specifies the resources excluded during restore
	// +kubebuilder:validation:Optional
	ExcludedResources []Resource `json:"excludedResources,omitempty"`

	// TransformStatus is the status of transformation performed
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +nullable:true
	TransformStatus []TransformStatus `json:"transformStatus,omitempty"`

	// Phase is the current phase of the application component while restore.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Phase RestorePhase `json:"phase,omitempty"`

	// PhaseStatus is the status of phase restore operation going through.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Pending;Error;Completed;Failed
	PhaseStatus Status `json:"phaseStatus,omitempty"`

	// A brief message indicating details about why the application component is in this state.
	// +kubebuilder:validation:Optional
	Reason string `json:"reason,omitempty"`
}

// TransformStatus specifies the details of transform operation
type TransformStatus struct {
	// TransformName is the name of transformation
	TransformName string `json:"transformName"`

	// +kubebuilder:validation:Enum=Completed;Failed
	// Status is the status of transform operation
	Status Status `json:"status"`

	// +kubebuilder:validation:Optional
	// TransformedResources Specifies the resources transformed as part of transformation
	TransformedResources []Resource `json:"transformedResources,omitempty"`

	// +kubebuilder:validation:Optional
	// Reason is reason for status in case of failure
	Reason string `json:"reason,omitempty"`
}

// RestoreHelm defines the backed up helm application to be restored.
type RestoreHelm struct {

	// +kubebuilder:validation:Optional
	// Snapshot defines the snapshot of application to be restored by a Helm.
	Snapshot *Helm `json:"snapshot,omitempty"`

	// +kubebuilder:validation:Optional
	// Status specifies the details of component restore in a namespace
	Status *ComponentStatus `json:"status,omitempty"`
}

// RestoreOperator defines the backed up operator application to be restored.
type RestoreOperator struct {

	// +kubebuilder:validation:Optional
	// Snapshot defines the snapshot of application to be restored by a Operator.
	Snapshot *Operator `json:"snapshot,omitempty"`

	// +kubebuilder:validation:Optional
	// Status specifies the details of component restore in a namespace
	Status *ComponentStatus `json:"status,omitempty"`
}

// RestoreCustom defines the backed up kubernetes resources.
type RestoreCustom struct {

	// +kubebuilder:validation:Optional
	// Snapshot defines the snapshot of custom application to be restored.
	Snapshot *Custom `json:"snapshot,omitempty"`

	// +kubebuilder:validation:Optional
	// Status specifies the details of component restore in a namespace
	Status *ComponentStatus `json:"status,omitempty"`
}

// RestoreApplication defines the snapshot contents of an Application Backup.
type RestoreApplication struct {

	// +kubebuilder:validation:Optional
	// HelmCharts specifies the backed up helm resources restored as Helm Charts.
	HelmCharts []RestoreHelm `json:"helmCharts,omitempty"`

	// +kubebuilder:validation:Optional
	// Operators specifies the backed up operator resources restored as Operators.
	Operators []RestoreOperator `json:"operators,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Custom specifies the backup up kubernetes resources.
	Custom *RestoreCustom `json:"custom,omitempty"`
}

// RestoreSource defines the source from where the restore is to be done
type RestoreSource struct {

	// Type is the type of source for restore
	Type RestoreSourceType `json:"type"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Backup is a reference to the Backup instance restored if type is Backup.
	Backup *corev1api.ObjectReference `json:"backup,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Target is a reference to the Target instance where from restore is performed if type is Location.
	Target *corev1api.ObjectReference `json:"target,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Location is an absolute path to remote target from where restore is performed if type is Location.
	Location string `json:"location,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// BackupPlan is a reference to the BackupPlan whose latest successful backup is to be restored.
	BackupPlan *corev1api.ObjectReference `json:"backupPlan,omitempty"`
}

// RestoreSpec defines the desired state of Restore
type RestoreSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Source defines the source referred for performing restore operation
	Source *RestoreSource `json:"source"`

	// Namespace is a name of namespace in cluster where backed
	// resources will be restored
	RestoreNamespace string `json:"restoreNamespace"`

	// +kubebuilder:validation:Optional
	// SkipIfAlreadyExists specifies whether to skip restore of a resource
	// if already exists in the namespace restored.
	SkipIfAlreadyExists bool `json:"skipIfAlreadyExists,omitempty"`

	// +kubebuilder:validation:Optional
	// PatchIfAlreadyExists specifies whether to patch spec of a already
	// exists resource in the namespace restored.
	PatchIfAlreadyExists bool `json:"patchIfAlreadyExists,omitempty"`

	// +kubebuilder:validation:Optional
	// PatchCRD specifies whether to patch spec of a already exists crd.
	PatchCRD bool `json:"patchCRD,omitempty"`

	// +kubebuilder:validation:Optional
	// OmitMetadata specifies whether to omit metadata like labels,
	// annotations of resources while restoring them.
	OmitMetadata bool `json:"omitMetadata,omitempty"`

	// +kubebuilder:validation:Optional
	// SkipOperatorResources specifies whether to skip operator resources or not at the time of restore.
	// (for the use case when operator is already present and the application of that operator needs to be restored)
	SkipOperatorResources bool `json:"skipOperatorResources,omitempty"`

	// +kubebuilder:validation:Optional
	// DisableIgnoreResources is responsible for the behavior of default list of resources being ignored at the restore.
	// If set to true, those resources will not be ignored
	DisableIgnoreResources bool `json:"disableIgnoreResources,omitempty"`

	// +kubebuilder:validation:Optional
	// Env is the List of environment variables to set in the container.
	// Cannot be updated.
	Env []corev1api.EnvVar `json:"env,omitempty"`

	// +kubebuilder:validation:Optional
	// TransformComponents specifies the component-wise transformation configuration
	TransformComponents *TransformComponents `json:"transformComponents,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// ExcludeResources specifies the resources to be excluded from backup while restoring
	ExcludeResources []Resource `json:"excludeResources,omitempty"`

	// +kubebuilder:validation:Optional
	// HookConfig specifies the Post Restore Hooks
	// Executed in reverse sequence of the sequence specified here
	HookConfig *HookConfig `json:"hookConfig,omitempty"`
}

// TransformComponents specifies component wise transformation configuration
type TransformComponents struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// HelmTransform specifies the Transformation configuration for Helm charts
	Helm []HelmTransform `json:"helm,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// CustomTransform specifies the Transformation configuration for Custom label-based backup
	Custom []CustomTransform `json:"custom,omitempty"`
}

// KeyValue specifies key-value pair for helm transformation
type KeyValue struct {
	// Key denotes the key for which value is to be set
	Key string `json:"key"`

	// Value denotes the value to be set
	Value string `json:"value"`
}

// HelmTransform specifies transformation configuration for Helm
type HelmTransform struct {
	// TransformName specifies the name of transformation
	TransformName string `json:"transformName"`

	// Release specifies the release name for which the transformation is to be done
	Release string `json:"release"`

	// +kubebuilder:validation:MinItems=1
	// Set specifies the key-value pair to be set
	Set []KeyValue `json:"set"`
}

// CustomTransform specifies transformation configuration for Custom label-based resources
type CustomTransform struct {
	// TransformName specifies the name of transformation
	TransformName string `json:"transformName"`

	// Resources specifies the resources for which transformation needs to be applied
	Resources *Resource `json:"resources"`

	// +kubebuilder:validation:MinItems=1
	// JSONPatches specifies the JSON patches to be applied
	JSONPatches []Patch `json:"jsonPatches"`
}

type Patch struct {
	// Op specifies the operation to perform, can be test/add/remove/replace/copy/move
	Op Op `json:"op"`

	// +kubebuilder:validation:Optional
	// From specifies the source element path. This field is mandatory for copy/move operation
	From string `json:"from,omitempty"`

	// Path specifies the destination element path which needs to be transformed
	Path string `json:"path"`

	// +kubebuilder:validation:Optional
	// Values specifies the value for any operation. This field is mandatory for test/add/replace operation
	Value *apiextensions.JSON `json:"value,omitempty"`
}

// RestoreCondition specifies the current condition of a restore resource.
type RestoreCondition struct {
	// Status is the status of the condition.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Error;Completed;Failed
	Status Status `json:"status,omitempty"`

	// Timestamp is the time a condition occurred.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebulder:validation:Format="date-time"
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// A brief message indicating details about why the component is in this condition.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Reason string `json:"reason,omitempty"`

	// Phase defines the current phase of the controller.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Phase RestorePhase `json:"phase,omitempty"`
}

// RestoreStats defines the stats for a Restore
type RestoreStats struct {

	// +kubebuilder:validation:Optional
	// BackupPlan is the reference to BackupPlan associated with Restore
	BackupPlan *corev1api.ObjectReference `json:"backupPlan,omitempty"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	// RestoreScope indicates scope of component being restored i.e. App or Namespace.
	RestoreScope ComponentScope `json:"restoreScope,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// StartTimestamp is the time a restore was started.
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// CompletionTimestamp is the time a restore was finished.
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Phase is the current phase of the restore operation.
	Phase RestorePhase `json:"phase,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Pending;Error;Completed;Failed
	// +nullable:true
	// PhaseStatus is the status of phase restore operation going through.
	PhaseStatus Status `json:"phaseStatus,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Pending;Error;Completed;Failed
	// +nullable:true
	// Status is the status of the restore operation.
	Status Status `json:"status,omitempty"`

	// +kubebuilder:validation:Optional
	// Size is the aggregate size of the data restored back.
	Size resource.Quantity `json:"size,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// PercentageCompletion is the amount of restore operation completed.
	PercentageCompletion int8 `json:"percentageCompletion,omitempty"`

	// RestoreApplication defines the information about the different applications restored back to cluster.
	RestoreApplication *RestoreApplication `json:"restoreApplication,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// HookStatus specifies pre/post hook execution status for current backup.
	HookStatus *HookComponentStatus `json:"hookStatus,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Condition is the current condition of restore resource.
	Condition []RestoreCondition `json:"condition,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	Stats *RestoreStats `json:"stats,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +k8s:openapi-gen=true

// Restore is the Schema for the restores API
// Backup represents the capture of Kubernetes Application defined by user at a point in time
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.source.backup.name`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Data Size",type=string,JSONPath=`.status.size`
// +kubebuilder:printcolumn:name="Start Time",type=string,JSONPath=`.status.startTimestamp`
// +kubebuilder:printcolumn:name="End Time",type=string,JSONPath=`.status.completionTimestamp`
// +kubebuilder:printcolumn:name="Percentage Completed",type=number,JSONPath=`.status.percentageCompletion`
// +kubebuilder:printcolumn:name="Restore Namespace",type=string,JSONPath=`.spec.restoreNamespace`
// +kubebuilder:printcolumn:name="Restore Scope",type=string,JSONPath=`.status.restoreScope`
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
