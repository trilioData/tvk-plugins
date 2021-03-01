package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:type=string
// LicenseState specifies the overall status of the license.
type LicenseState string

const (
	// LicenseActive means the license key is valid and has not reached expiration.
	LicenseActive LicenseState = "Active"

	// LicenseExpired means the license key is valid and has reached expiration.
	LicenseExpired LicenseState = "Expired"

	// LicenseInvalid means the license key is not valid.
	LicenseInvalid LicenseState = "Invalid"

	// LicenseError means the license key is valid and has not reached expiration but
	// cluster has crossed provided license capacity above grace period.
	LicenseError LicenseState = "Error"

	// LicenseWarning means the license key is valid and has not reached expiration but
	// cluster has crossed provided license capacity is under grace period.
	LicenseWarning LicenseState = "Warning"
)

// +kubebuilder:validation:type=string
// LicenseEdition specifies the edition of the license.
type LicenseEdition string

const (
	FreeEdition         LicenseEdition = "FreeTrial"
	BasicEdition        LicenseEdition = "Basic"
	StandardEdition     LicenseEdition = "STANDARD"
	ProfessionalEdition LicenseEdition = "PROFESSIONAL"
	EnterpriseEdition   LicenseEdition = "ENTERPRISE"
)

// LicenseProperties specifies the properties of a license based on provided license key.
type LicenseProperties struct {

	// +kubebuilder:validation:Optional
	// Company is the name of a company purchased license for.
	Company string `json:"company,omitempty"`

	// Edition is the type of license purchased to use triliovault application.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=FreeTrial;Basic;STANDARD;PROFESSIONAL;ENTERPRISE
	Edition LicenseEdition `json:"edition,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// CreationTimestamp is the time license created to use triliovault application.
	CreationTimestamp *metav1.Time `json:"creationTimestamp,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// PurchaseTimestamp is the time user purchased the license to use triliovault application.
	PurchaseTimestamp *metav1.Time `json:"purchaseTimestamp,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// ExpirationTimestamp is the time provided license going to expire and won't be able to perform backup/restore operation.
	ExpirationTimestamp *metav1.Time `json:"expirationTimestamp,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// MaintenanceExpiryTimestamp is the time maintenance support for the provided license going to expire.
	MaintenanceExpiryTimestamp *metav1.Time `json:"maintenanceExpiryTimestamp,omitempty"`

	// +kubebuilder:validation:Optional
	// KubeUID is the kubesystem or namespace uuid of the cluster the license purchased for.
	KubeUID string `json:"kubeUID,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Scope is the scope of a KubeUID the license purchased for.
	// +kubebuilder:validation:Enum=Cluster;Namespaced
	Scope Scope `json:"scope,omitempty"`

	// +kubebuilder:validation:Optional
	// Version is the version of a license.
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Optional
	// SEN is the unique serial of a license purchased.
	SEN string `json:"sen,omitempty"`

	// +kubebuilder:validation:Optional
	// NumberOfUsers is the total number of users the license valid for.
	NumberOfUsers int `json:"numberOfUsers,omitempty"`

	// +kubebuilder:validation:Optional
	// ServerID is the unique serverID of license purchased.
	ServerID string `json:"serverID,omitempty"`

	// +kubebuilder:validation:Optional
	// LicenseID is the identifier for the license.
	LicenseID string `json:"licenseID,omitempty"`

	// +kubebuilder:validation:Optional
	// Capacity is the maximum capacity to use the license in number of kube nodes.
	Capacity uint32 `json:"capacity,omitempty"`

	// +kubebuilder:validation:Optional
	// Active is the status of the license.
	Active bool `json:"active,omitempty"`
}

// LicenseCondition specifies the current condition of a license.
type LicenseCondition struct {

	// Status is the status of the condition.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Active;Expired;Invalid;Error;Warning
	Status LicenseState `json:"status,omitempty"`

	// Timestamp is the time a condition occurred.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebulder:validation:Format="date-time"
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// A brief message indicating details about why the component is in this condition.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`

	// Phase defines the current phase of the controller.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Validation
	Phase OperationType `json:"phase,omitempty"`
}

// LicenseSpec defines the desired state of License
type LicenseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Key is the product key to use triliovault application to perform backup/restore.
	Key string `json:"key"`
}

// LicenseStatus defines the observed state of License
type LicenseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Active;Expired;Invalid;Error;Warning
	// +nullable:true
	// Status is the overall status of the license based on provided key.
	Status LicenseState `json:"status,omitempty"`

	// A brief message indicating details about why the license in this state.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`

	// Properties is the details about the license based on provided license key.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Properties LicenseProperties `json:"properties,omitempty"`

	// Condition is the current condition of a license.
	// +nullable:true
	// +kubebuilder:validation:Optional
	Condition []LicenseCondition `json:"condition,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// CurrentNodeCount is the total number of nodes kubernetes cluster comprised of
	// where each node capped at 2 vCPUs/pCPUs.
	CurrentNodeCount uint32 `json:"currentNodeCount,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// GracePeriodStartTimestamp is the time grace period started to use triliovault application.
	GracePeriodStartTimestamp *metav1.Time `json:"gracePeriodStartTimestamp,omitempty"`

	// +kubebulder:validation:Format="date-time"
	// +kubebuilder:validation:Optional
	// +nullable:true
	// GracePeriodEndTimestamp is the time grace period for using the triliovault application going to end.
	GracePeriodEndTimestamp *metav1.Time `json:"gracePeriodEndTimestamp,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +k8s:openapi-gen=true

// License is the Schema for the licenses API
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Current Node Count",type=string,JSONPath=`.status.currentNodeCount`
// +kubebuilder:printcolumn:name="Grace Period End Time",type=string,JSONPath=`.status.gracePeriodEndTimestamp`
// +kubebuilder:printcolumn:name="Edition",type=string,JSONPath=`.status.properties.edition`
// +kubebuilder:printcolumn:name="Capacity",type=string,JSONPath=`.status.properties.capacity`
// +kubebuilder:printcolumn:name="Expiration Time",type=string,JSONPath=`.status.properties.expirationTimestamp`
type License struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LicenseSpec   `json:"spec,omitempty"`
	Status LicenseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LicenseList contains a list of License
type LicenseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []License `json:"items"`
}

func init() {
	SchemeBuilder.Register(&License{}, &LicenseList{})
}
