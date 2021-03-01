package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TargetType is the type of target.
// +kubebuilder:validation:Enum=ObjectStore;NFS
type TargetType string

const (
	ObjectStore TargetType = "ObjectStore"

	NFS TargetType = "NFS"
)

// Vendor is the third party storage vendor hosting the target
// +kubebuilder:validation:Enum=AWS;RedhatCeph;Ceph;IBMCleversafe;Cloudian;Scality;NetApp;Cohesity;SwiftStack;Wassabi;MinIO;DellEMC;Other
type Vendor string

const (
	AWS           Vendor = "AWS"
	RedhatCeph    Vendor = "RedHatCeph"
	Ceph          Vendor = "Ceph"
	IBMCleversafe Vendor = "IBMCleversafe"
	Cloudian      Vendor = "Cloudian"
	Scality       Vendor = "Scality"
	NetApp        Vendor = "NetApp"
	Cohesity      Vendor = "Cohesity"
	SwiftStack    Vendor = "SwiftStack"
	Wassabi       Vendor = "Wassabi"
	MinIO         Vendor = "MinIO"
	DellEMC       Vendor = "DellEMC"
	Other         Vendor = "Other"
)

// ObjectStoreCredentials defines the credentials to use Object Store as a target type.
type ObjectStoreCredentials struct {

	// Url to connect the Object Store.
	// +kubebuilder:validation:Optional
	URL string `json:"url,omitempty"`

	// Access Key is to authenticate access to Object Store.
	AccessKey string `json:"accessKey"`

	// Secret Key is to authenticate access to Object Store.
	SecretKey string `json:"secretKey"`

	// Name of a bucket within Object Store.
	BucketName string `json:"bucketName"`

	// Region where the Object Store resides.
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
}

// NFSCredentials defines the credentials to use NFS as a target type.
type NFSCredentials struct {

	// A NFS location in format trilio.net:/data/location/abcde or 192.156.13.1:/user/keeth/data.
	NfsExport string `json:"nfsExport"`

	// An additional options passed to mount NFS directory e.g. rw, suid, hard, intr, timeo, retry.
	// +kubebuilder:validation:Optional
	NfsOptions string `json:"nfsOptions,omitempty"`
}

// TargetCondition specifies the current condition of a target resource.
type TargetCondition struct {
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
	// +kubebuilder:validation:Enum=Validation;TargetBrowsing
	Phase OperationType `json:"phase,omitempty"`
}

// TargetSpec defines the specification of a Target.
type TargetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Type is the type of target for backup storage.
	Type TargetType `json:"type"`

	// Vendor is the third party storage vendor hosting the target
	Vendor Vendor `json:"vendor"`

	// NfsCredentials specifies the credentials for TargetType NFS
	// +kubebuilder:validation:Optional
	NFSCredentials NFSCredentials `json:"nfsCredentials,omitempty"`

	// ObjectStoreCredentials specifies the credentials for TargetType ObjectStore
	// +kubebuilder:validation:Optional
	ObjectStoreCredentials ObjectStoreCredentials `json:"objectStoreCredentials,omitempty"`

	// EnableBrowsing specifies if target browser feature should be enabled for this target or not
	// +kubebuilder:validation:Optional
	EnableBrowsing bool `json:"enableBrowsing,omitempty"`

	// ThresholdCapacity is the maximum threshold capacity to store backup data.
	// +kubebuilder:validation:Optional
	ThresholdCapacity *resource.Quantity `json:"thresholdCapacity,omitempty"`
}

// TargetStats defines the stats for a Target
type TargetStats struct {

	// +kubebuilder:validation:Optional
	// TotalBackupPlans is the count of total number of BackupPlans of a Target
	TotalBackupPlans uint32 `json:"totalBackupPlans,omitempty"`

	// +kubebuilder:validation:Optional
	// CapacityOccupied is the aggregate of total size occupied on the Target by Backups
	CapacityOccupied resource.Quantity `json:"capacityOccupied,omitempty"`

	// +kubebuilder:validation:Optional
	ApplicationCapacity resource.Quantity `json:"applicationCapacity,omitempty"`

	// +kubebuilder:validation:Optional
	ApplicationCapacityConsumed resource.Quantity `json:"applicationCapacityConsumed,omitempty"`
}

// TargetStatus defines the observed state of Target
type TargetStatus struct {
	// Condition is the current condition of a target.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Condition []TargetCondition `json:"condition,omitempty"`

	// Status is the final Status of target Available/Unavailable
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Available;Unavailable
	Status Status `json:"status,omitempty"`

	// BrowsingEnabled specifies if target browser feature is enabled for this target or not
	// +kubebuilder:validation:Optional
	BrowsingEnabled bool `json:"browsingEnabled,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	Stats *TargetStats `json:"stats,omitempty"`
}

// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Target is a location where TrilioVault stores backup.
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Threshold Capacity",type=string,JSONPath=`.spec.thresholdCapacity`
// +kubebuilder:printcolumn:name="Vendor",type=string,JSONPath=`.spec.vendor`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Browsing Enabled",type=string,JSONPath=`.status.browsingEnabled`
type Target struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetSpec   `json:"spec,omitempty"`
	Status TargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TargetList contains a list of Target.
type TargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Target `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Target{}, &TargetList{})
}
