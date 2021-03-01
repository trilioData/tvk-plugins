package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	Timeout   PolicyType = "Timeout"
	Retention PolicyType = "Retention"
	Cleanup   PolicyType = "Cleanup"
)

// PolicySpec defines the desired state of Policy
type PolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// Type is a field of Policy spec, which defines the policy type containing only 3 values: Retention, Timeout, Cleanup.
	Type PolicyType `json:"type"`

	// +kubebuilder:validation:Required
	// Default field states if the current type of policy is default across the TV application
	Default bool `json:"default,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// RetentionConfig field defines the configuration for Retention policies
	RetentionConfig RetentionConfig `json:"retentionConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// TimeoutConfig field defines the configuration for timeout policies
	TimeoutConfig TimeoutConfig `json:"timeoutConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// CleanupConfig field defines the configuration for Cleanup policies
	CleanupConfig CleanupConfig `json:"cleanupConfig,omitempty"`
}

// PolicyStatus defines the observed state of Policy
type PolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Status PolicyState `json:"status"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="policy",type=string,JSONPath=`.spec.type`,priority=0
// +kubebuilder:printcolumn:name="default",type=string,JSONPath=`.spec.default`,priority=0
// +kubebuilder:printcolumn:name="configuration",type=string,JSONPath=`.spec.[?(@.retentionConfig)]`,priority=10
// +kubebuilder:printcolumn:name="configuration",type=string,JSONPath=`.spec.[?(@.timeoutConfig)]`,priority=10
// +kubebuilder:printcolumn:name="configuration",type=string,JSONPath=`.spec.[?(@.cleanupConfig)]`,priority=10

// Policy is the Schema for the policies API
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   PolicySpec   `json:"spec,omitempty"`
	Status PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:validation:Enum=Timeout;Retention;Cleanup
// PolicyType is the Enum for types of policies
type PolicyType string

// +kubebuilder:validation:Enum=Monday;Tuesday;Wednesday;Thursday;Friday;Saturday;Sunday
type DayOfWeek string

const (
	Monday    DayOfWeek = "Monday"
	Tuesday   DayOfWeek = "Tuesday"
	Wednesday DayOfWeek = "Wednesday"
	Thursday  DayOfWeek = "Thursday"
	Friday    DayOfWeek = "Friday"
	Saturday  DayOfWeek = "Saturday"
	Sunday    DayOfWeek = "Sunday"
)

// +kubebuilder:validation:Enum=January;February;March;April;May;June;July;August;September;October;November;December
type MonthOfYear string

const (
	January   MonthOfYear = "January"
	February  MonthOfYear = "February"
	March     MonthOfYear = "March"
	April     MonthOfYear = "April"
	May       MonthOfYear = "May"
	June      MonthOfYear = "June"
	July      MonthOfYear = "July"
	August    MonthOfYear = "August"
	September MonthOfYear = "September"
	October   MonthOfYear = "October"
	November  MonthOfYear = "November"
	December  MonthOfYear = "December"
)

// RetentionConfig is the configuration for the PolicyType: Retention
type RetentionConfig struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=int
	// +kubebuilder:validation:Minimum=1
	// Latest is the max number of latest backups to be retained
	Latest int `json:"latest,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=int
	// +kubebuilder:validation:Minimum=1
	// Weekly is max number of backups to be retained in a week
	Weekly int `json:"weekly,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=int
	// +kubebuilder:validation:Minimum=1
	// Monthly is max number of backups to be retained in a month
	Monthly int `json:"monthly,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=int
	// +kubebuilder:validation:Minimum=1
	// Yearly is max number of backups to be retained in a year
	Yearly int `json:"yearly,omitempty"`

	// +kubebuilder:validation:Optional
	// DayOfWeek is Day of the week to maintain weekly backup/restore resources
	DayOfWeek DayOfWeek `json:"dayOfWeek,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=int
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=28
	// DateOfMonth is Date of the month to maintain monthly backup/restore resources
	DateOfMonth *int `json:"dateOfMonth,omitempty"`

	// +kubebuilder:validation:Optional
	// MonthOfYear is the month of the backup to retain for yearly backups
	MonthOfYear MonthOfYear `json:"monthOfYear,omitempty"`
}

// TimeoutConfig is the configuration for the PolicyType: Timeout
type TimeoutConfig struct {
}

// CleanupConfig is the configuration for the PolicyType: Cleanup
type CleanupConfig struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Format=int
	// +kubebuilder:validation:Required
	// BackupDays is the age of backups to be cleaned
	BackupDays *int `json:"backupDays"`
}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:UniqueItems=true
	Items []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
