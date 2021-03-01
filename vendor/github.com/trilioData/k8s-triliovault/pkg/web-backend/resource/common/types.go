package common

import (
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
)

// TimeRangeField
type TimeRangeField string

const (
	Hour  TimeRangeField = "Hour"
	Day   TimeRangeField = "Day"
	Week  TimeRangeField = "Week"
	Month TimeRangeField = "Month"
)

// Status specifies whether application protected (should have at-least one successful backup) or not.
type ApplicationStatus string

const (
	Protected   ApplicationStatus = "Protected"
	UnProtected ApplicationStatus = "UnProtected"
)

const (
	// Parameter for Specifying the Request Body of the Request
	RequestBody = "requestBody"

	// Param for Sending Generic error to UI
	GenericError = "message"
)

// ApplicationPVCStatus specifies whether application has atleast one PersistentVolumeClaim.
type ApplicationPVCStatus string

const (
	Exists    ApplicationPVCStatus = "Exists"
	NotExists ApplicationPVCStatus = "NotExists"
)

// Constant for EOF error
const EOF = "EOF"

type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// PathParameters specifies parameters to filter Resources
type PathParameters string

const (
	NamePathParam      PathParameters = "name"
	NamespacePathParam PathParameters = "namespace"
)

// DeleteRequestParams for storing user input for Delete Resource
type DeleteRequestParams struct {
	// Name specifies the name of the Resource for which the resource to be Deleted
	Name      string
	Namespace string
}

// Resource defines the list of names of a Kubernetes resource of a particular GVK.
type Resource struct {

	// GroupVersionKind specifies GVK uniquely representing particular resource type.
	GroupVersionKind v1.GroupVersionKind `json:"groupVersionKind"`

	// +kubebuilder:validation:Optional
	// Objects is the list of names of all the objects of the captured GVK
	Objects []NamespacedName `json:"objects,omitempty"`
}

// ApplicationDetails specifies more details of an application.
type ApplicationDetails struct {
	// LatestBackup is the latest backup where application is involved.
	LatestBackup *v1.Backup `json:"latestBackup"`

	// LastSuccessfulBackup is the last successful backup where application was part of.
	LastSuccessfulBackup *v1.Backup `json:"lastSuccessfulBackup"`

	// LastSuccessfulRestore is the last successful restore where application was part of.
	LastSuccessfulRestore *v1.Restore `json:"lastSuccessfulRestore"`

	// IsScheduled specifies whether application is a part of backupplan which has cron schedule.
	IsScheduled bool `json:"isScheduled"`
}

// ApplicationSelectorSearchFilter
type ApplicationSelectorSearchFilter struct {
	Applications []ApplicationMetadata      `json:"applications,omitempty"`
	Labels       []ApplicationLabelMetadata `json:"labels,omitempty"`
	Objects      []ApplicationObjectMeta    `json:"objects,omitempty"`
	BackupPlans  []NamespacedName           `json:"backupPlans,omitempty"`
}

// ApplicationObjectMeta
type ApplicationObjectMeta struct {
	GroupVersionKind v1.GroupVersionKind `json:"groupVersionKind"`
	Object           v12.ObjectMeta      `json:"object"`
}

type AggregatedApplicationSearchFilter struct {
	HelmReleases []string
	Operators    []string
	Labels       map[string]string
}

// Metadata specifies identifier for an Application
type ApplicationMetadata struct {
	// Type is the type of an application (Helm/Operator)
	Type v1.ApplicationType `json:"type,omitempty"`

	// Namespace is the namespace where application exists
	Namespace string `json:"namespace,omitempty"`

	// Name is the identifier for particular application type - helm chart for Helm
	Name string `json:"name,omitempty"`
}

// ApplicationLabelMetadata specifies the data for label
type ApplicationLabelMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TimeRangeFilter Specifies the filter on which objects can be filter
type TimeRangeFilter struct {
	// TimeRangeField specifies type of Range Field on which filtering will be done
	TimeRangeField TimeRangeField

	// TimeRangeValue specifies the int value for the TimeRangeField
	TimeRangeValue int
}

// IsEmpty function will check if ApplicationSelectorSearchFilter is empty or not.
func (filter *ApplicationSelectorSearchFilter) IsEmpty() bool {
	if len(filter.Applications) == 0 && len(filter.BackupPlans) == 0 && len(filter.Labels) == 0 && len(filter.Objects) == 0 {
		return true
	}
	return false
}

// Function for validating the ApplicationType
func IsValidApplicationType(appType string) bool {
	switch appType {
	case string(v1.HelmType):
		return true
	case string(v1.OperatorType):
		return true
	case string(v1.CustomType):
		return true
	default:
		return false
	}
}

// Function to check TimeRangeFilter is empty or not
func (timeRangeFilter TimeRangeFilter) IsEmpty() bool {
	if timeRangeFilter.TimeRangeValue == 0 || timeRangeFilter.TimeRangeField == "" {
		return true
	}
	return false
}

func (details ApplicationDetails) IsApplicationStatusMatches(status ApplicationStatus) bool {
	return (status == Protected && details.LastSuccessfulBackup != nil) || (status == UnProtected &&
		details.LastSuccessfulBackup == nil)
}
