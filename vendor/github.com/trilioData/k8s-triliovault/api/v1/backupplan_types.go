package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupPlanSpec defines the desired state of BackupPlan
type BackupPlanSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Namespace is the namespace from where the components of backupPlan to be selected
	// Deprecated: After removal of cluster scope CRD support, Backup namespace will be same as BackupPlan namespace.
	BackupNamespace string `json:"backupNamespace,omitempty"`

	// +kubebuilder:validation:Required
	// BackupConfig is the type containing the object references for all the configurations needed for backup operation
	BackupConfig BackupConfig `json:"backupConfig"`

	// +kubebuilder:validation:Optional
	// BackupPlanComponents includes all the components which defines this BackupPlan i.e Helm charts, operators and
	// label based resources
	BackupPlanComponents BackupPlanComponents `json:"backupPlanComponents,omitempty"`

	// +kubebuilder:validation:Optional
	// hookConfig defines backup pre/post hooks and their configurations.
	HookConfig *HookConfig `json:"hookConfig,omitempty"`
}

// BackupConfig defines the require configuration for taking the backup such as target and retention policy
type BackupConfig struct {
	// +kubebuilder:validation:Required
	// Target is the object reference for the backup target resources
	Target *corev1.ObjectReference `json:"target"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// RetentionPolicy is the object reference for the policy of type retention defined
	RetentionPolicy *corev1.ObjectReference `json:"retentionPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// SchedulePolicy includes the 2 type of cron schedule specs: incremental and full
	SchedulePolicy SchedulePolicy `json:"schedulePolicy,omitempty"`
}

// SchedulePolicy defines the cronjob specs for incremental or full backup types
type SchedulePolicy struct {
	// +kubebuilder:validation:Optional
	// +nullable:true
	// IncrementalCron is the cronspec schedule for incremental backups
	IncrementalCron CronSpec `json:"incrementalCron,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// FullBackupCron is the cronspec schedule for full backups
	FullBackupCron CronSpec `json:"fullBackupCron,omitempty"`
}

// CronSpec defines the Schedule string and the cronjob reference. The Schedule string will only be visible to the user
// to be configured, the reference will be set by the controller
type CronSpec struct {
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`
}

// BackupPlanComponents contains the 3 types of components, helm charts, operators and custom label-based resources
type BackupPlanComponents struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// HelmReleases is the list of release names
	HelmReleases []string `json:"helmReleases,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// Operators is the list of operator names and their selectors
	Operators []OperatorSelector `json:"operators,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// Custom is the combination of label selectors including match labels and match expressions
	Custom []metav1.LabelSelector `json:"custom,omitempty"`
}

// OperatorSelector defines the mapping of operator name and their selectors
type OperatorSelector struct {
	// +kubebuilder:validation:MinLength=1
	// OperatorId is any unique ID for a particular operator
	OperatorID string `json:"operatorId,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// CustomResources list resources where each resource contains custom resource gvk and metadata
	CustomResources []Resource `json:"customResources,omitempty"`

	// +kubebuilder:validation:Optional
	// HelmRelease is the release name of the helm based operator
	HelmRelease string `json:"helmRelease,omitempty"`

	// +kubebuilder:validation:Optional
	// OLMSubscription is the subscription name for the olm based operator
	OLMSubscription string `json:"olmSubscription,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// OperatorResourceSelector is the selector for operator resources
	OperatorResourceSelector []metav1.LabelSelector `json:"operatorResourceSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// ApplicationResourceSelector is the selector for instances deployed by the operator resources
	ApplicationResourceSelector []metav1.LabelSelector `json:"applicationResourceSelector,omitempty"`
}

// BackupSummary comprises of backup object references and count of backups with different statuses
type BackupSummary struct {

	// +kubebuilder:validation:Optional
	// InProgressBackup is the reference to an InProgress backup of a BackupPlan
	InProgressBackup *corev1.ObjectReference `json:"inProgressBackup,omitempty"`

	// +kubebuilder:validation:Optional
	// LastSuccessfulBackup is the reference to Latest available Backup of a BackupPlan
	LastSuccessfulBackup *corev1.ObjectReference `json:"lastSuccessfulBackup,omitempty"`

	// +kubebuilder:validation:Optional
	// LatestBackup is the reference to Latest Backup in any state, of a BackupPlan
	LatestBackup *corev1.ObjectReference `json:"latestBackup,omitempty"`

	// +kubebuilder:validation:Optional
	// TotalInProgressBackups is the count of total number of InProgress Backups
	TotalInProgressBackups uint32 `json:"totalInProgressBackups,omitempty"`

	// +kubebuilder:validation:Optional
	// TotalAvailableBackups is the count of total number of Available Backups
	TotalAvailableBackups uint32 `json:"totalAvailableBackups,omitempty"`

	// +kubebuilder:validation:Optional
	// TotalFailedBackups is the count of total number of InProgress Backups
	TotalFailedBackups uint32 `json:"totalFailedBackups,omitempty"`
}

// RestoreSummary comprises of restore object references and count of restore with different statuses
type RestoreSummary struct {

	// +kubebuilder:validation:Optional
	// LastSuccessfulRestore is the reference to Latest completed Restore of a BackupPlan
	LastSuccessfulRestore *corev1.ObjectReference `json:"lastSuccessfulRestore,omitempty"`

	// +kubebuilder:validation:Optional
	// TotalInProgressRestores is the count of total number of InProgress Restores
	TotalInProgressRestores uint32 `json:"totalInProgressRestores,omitempty"`

	// +kubebuilder:validation:Optional
	// TotalCompletedRestores is the count of total number of Completed Restores
	TotalCompletedRestores uint32 `json:"totalCompletedRestores,omitempty"`

	// +kubebuilder:validation:Optional
	// TotalFailedRestores is the count of total number of Failed Restores
	TotalFailedRestores uint32 `json:"totalFailedRestores,omitempty"`
}

// BackupPlanStats defines the stats for a BackupPlan
type BackupPlanStats struct {

	// +kubebuilder:validation:Optional
	BackupSummary BackupSummary `json:"backupSummary,omitempty"`

	// +kubebuilder:validation:Optional
	RestoreSummary RestoreSummary `json:"restoreSummary,omitempty"`
}

// BackupPlanStatus defines the observed state of BackupPlan
type BackupPlanStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Enum=Available;InProgress;Pending;Error
	// Status defines the status oif the application resource as available when no operation is running
	// and unavailable when a backup or restore operation is in progress
	Status Status `json:"status"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	IncrementalCron *corev1.ObjectReference `json:"incrementalCron,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	FullBackupCron *corev1.ObjectReference `json:"fullBackupCron,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	Stats *BackupPlanStats `json:"stats,omitempty"`
}

// nolint:lll // directive continuation
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="target",type=string,JSONPath=`.spec.backupConfig.target.name`,priority=0
// +kubebuilder:printcolumn:name="retention policy",type=string,JSONPath=`.spec.backupConfig.retentionPolicy.name`,priority=0
// +kubebuilder:printcolumn:name="incremental schedule",type=string,JSONPath=`.spec.backupConfig.schedulePolicy.incrementalCron.schedule`,priority=0
// +kubebuilder:printcolumn:name="full backup schedule",type=string,JSONPath=`.spec.backupConfig.schedulePolicy.fullBackupCron.schedule`,priority=0
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`,priority=0
// +kubebuilder:printcolumn:name="quiesce mode",type=string,JSONPath=`.spec.quiesceMode`,priority=10
// +kubebuilder:printcolumn:name="quiesce sequence",type=string,JSONPath=`.spec.quiesceSequence`,priority=10
// +kubebuilder:printcolumn:name="helm release",type=string,JSONPath=`.spec.backupPlanComponents.helms[*]`,priority=10
// nolint:lll // directive continuation
// +kubebuilder:printcolumn:name="operators",type=string,JSONPath=`.spec.backupPlanComponents.operators[*].CustomResources[*].Objects[*]`,priority=10
// +kubebuilder:printcolumn:name="custom components",type=string,JSONPath=`.spec.backupPlanComponents.custom`,priority=10
// +kubebuilder:printcolumn:name="hooks",type=string,JSONPath=`.spec.hooks[*].hookAction.name`,priority=10

// BackupPlan is the Schema for the applications API
type BackupPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   BackupPlanSpec   `json:"spec"`
	Status BackupPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true

// BackupPlanList contains a list of BackupPlan
type BackupPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:MinItems=0
	Items []BackupPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPlan{}, &BackupPlanList{})
}

func (s *SchedulePolicy) IsEmpty() bool {
	return s.FullBackupCron.Schedule == "" && s.IncrementalCron.Schedule == ""
}
