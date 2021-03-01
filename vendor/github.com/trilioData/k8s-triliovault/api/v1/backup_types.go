package v1

import (
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupType defines the type backup instance of an BackupPlan
// +kubebuilder:validation:Enum=Incremental;Full;Mixed
type BackupType string

const (
	// Incremental means the backup instance is intermediate part of sequential backups of an BackupPlan
	Incremental BackupType = "Incremental"

	// Full means that the backup instance is whole in itself and can individually restored
	Full BackupType = "Full"

	// Mixed means that the backup instance has backup ad
	Mixed BackupType = "Mixed"
)

// ApplicationType specifies type of a Backup of an application
// +kubebuilder:validation:Enum=Helm;Operator;Custom;Namespace
type ApplicationType string

const (
	// HelmType means the backup consists helm based backups
	HelmType ApplicationType = "Helm"

	// OperatorType means the backup consists operator based backups
	OperatorType ApplicationType = "Operator"

	// CustomType means the backup consists custom label based backups
	CustomType ApplicationType = "Custom"

	// TODO: To remove Namespace Application Type as it's a duplicate of Backup scope in status.
	// NamespaceType means the backup consists namespaced backups
	NamespaceType ApplicationType = "Namespace"
)

// BackupScheduleType specifies the type of schedule which triggered the backup
// +kubebuilder:validation:Enum=Periodic;OneTime
type BackupScheduleType string

const (
	// Periodic means that the backup is triggered due to cron job defined for the application to be backed up.
	Periodic BackupScheduleType = "Periodic"

	// OneTime means that the user manually called backup operation to be executed.
	OneTime BackupScheduleType = "OneTime"
)

// Snapshot defines the snapshot contents of an Application Backup.
type Snapshot struct {

	// +kubebuilder:validation:Optional
	// HelmCharts specifies the snapshot of application defined by Helm Charts.
	HelmCharts []Helm `json:"helmCharts,omitempty"`

	// +kubebuilder:validation:Optional
	// Operators specifies the snapshot of application defined by Operators.
	Operators []Operator `json:"operators,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Custom specifies the snapshot of Custom defined applications.
	Custom *Custom `json:"custom,omitempty"`
}

// BackupCondition specifies the current condition of a backup resource.
type BackupCondition struct {
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
	//nolint:lll // directive continuation
	// +kubebuilder:validation:Enum={"Unquiesce","Quiesce","MetaSnapshot","DataSnapshot","DataUpload","Snapshot", "MetadataUpload","Retention","Upload"}
	Phase OperationType `json:"phase,omitempty"`
}

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Type is the type of backup in the sequence of backups of an Application.
	Type BackupType `json:"type"`

	// BackupPlan is a reference to the BackupPlan to be backed up.
	BackupPlan *corev1api.ObjectReference `json:"backupPlan"`
}

// BackupStats specifies the stats of a Backup
type BackupStats struct {

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Target is the reference to a Target backuped up
	Target *corev1api.ObjectReference `json:"target,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// LatestInProgressRestore is the reference to the latest InProgress Restore of a Backup
	LatestInProgressRestore *corev1api.ObjectReference `json:"latestInProgressRestore,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// LatestCompletedRestore is the reference to the latest Completed Restore of a Backup
	LastCompletedRestore *corev1api.ObjectReference `json:"latestCompletedRestore,omitempty"`

	// +kubebuilder:validation:Optional
	// HookExists is a bool value that states if a backup has hooks in backup plan
	HookExists bool `json:"hookExists"`

	// +kubebuilder:validation:Optional
	// BackupNamespace is the namespace in which backup exists
	BackupNamespace string `json:"backupNamespace,omitempty"`

	// +kubebuilder:validation:Optional
	// ApplicationType is the type of Backup
	ApplicationType *ApplicationType `json:"applicationType,omitempty"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	// BackupScope indicates scope of component in backup i.e. App or Namespace.
	BackupScope ComponentScope `json:"backupScope,omitempty"`

	// +kubebuilder:validation:Optional
	// Type indicates the backup type in backup i.e. Full, Incremental or Mixed.
	Type BackupType `json:"type,omitempty"`

	// Location is the absolute path of the target where backup resides.
	Location string `json:"location,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// StartTimestamp is the time a backup was started.
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// CompletionTimestamp is the time a backup was finished.
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Phase is the current phase of the backup operation.
	Phase OperationType `json:"phase,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Pending;Error;Completed;Failed
	// +nullable:true
	// PhaseStatus is the status of phase backup operation going through.
	PhaseStatus Status `json:"phaseStatus,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Pending;Error;Completed;Failed;Available;Coalescing
	// +nullable:true
	// Status is the status of the backup operation.
	Status Status `json:"status,omitempty"`

	// +kubebuilder:validation:Optional
	// Size is the aggregate size of the data backuped up.
	Size resource.Quantity `json:"size,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// PercentageCompletion is the amount of backup operation completed.
	PercentageCompletion int8 `json:"percentageCompletion,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// ExpirationTimeStamp is the time a backup will not be available after retention.
	ExpirationTimestamp *metav1.Time `json:"expirationTimestamp,omitempty"`

	// Todo: Do we need this option as optional one?
	// Todo: This is optional because, we are allowing the custom backup as empty
	// +kubebuilder:validation:Optional
	// +nullable:true
	// Snapshot specifies the contents of captured backup.
	Snapshot *Snapshot `json:"snapshot,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Condition is the current condition of hooks while backup.
	Condition []BackupCondition `json:"condition,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// HookStatus specifies pre/post hook execution status for current backup.
	HookStatus *HookComponentStatus `json:"hookStatus,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	Stats *BackupStats `json:"stats,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true

// Backup respresents the capture of Kubernetes BackupPlan defined by user at a point in time
// +kubebuilder:printcolumn:name="BackupPlan",type=string,JSONPath=`.spec.backupPlan.name`
// +kubebuilder:printcolumn:name="Backup Type",type=string,JSONPath=`.status.type`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Data Size",type=string,JSONPath=`.status.size`
// +kubebuilder:printcolumn:name="Start Time",type=string,JSONPath=`.status.startTimestamp`
// +kubebuilder:printcolumn:name="End Time",type=string,JSONPath=`.status.completionTimestamp`
// +kubebuilder:printcolumn:name="Percentage Completed",type=number,JSONPath=`.status.percentageCompletion`
// +kubebuilder:printcolumn:name="Backup Scope",type=string,JSONPath=`.status.backupScope`
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})

}
