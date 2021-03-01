package v1

import (
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GroupVersionKind defines the Kubernetes resource type
type GroupVersionKind struct {
	Group string `json:"group,omitempty"`

	// +kubebuilder:validation:Optional
	Version string `json:"version,omitempty"`

	Kind string `json:"kind,omitempty"`
}

// Resource defines the list of names of a Kubernetes resource of a particular GVK.
type Resource struct {

	// GroupVersionKind specifies GVK uniquely representing particular resource type.
	GroupVersionKind GroupVersionKind `json:"groupVersionKind"`

	// +kubebuilder:validation:Optional
	// Objects is the list of names of all the objects of the captured GVK
	Objects []string `json:"objects,omitempty"`
}

// VolumeSnapshot defines the CSI snapshot of a Persistent Volume.
type VolumeSnapshot struct {

	// +kubebuilder:validation:Optional
	// VolumeSnapshot is a reference to the Persistent Volume Snapshot captured.
	VolumeSnapshot *corev1api.ObjectReference `json:"volumeSnapshot,omitempty"`

	// +kubebuilder:validation:Optional
	// RetryCount is the number of attempts made to capture Volume Snapshot.
	RetryCount int8 `json:"retryCount,omitempty"`

	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=InProgress;Pending;Error;Completed;Failed
	// Status is the status defining the progress of Volume Snapshot capture.
	Status Status `json:"status,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Error is the error occurred while capturing Volume Snapshot if any.
	Error string `json:"error,omitempty"`
}

// PodContainers defines Pod and containers running in that Pod.
type PodContainers struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Optional
	// +nullable:true
	// PodName is the name of pod which will be the key for the map between pod containers list
	PodName string `json:"podName,omitempty"`

	// +nullable:true
	// +kubebuilder:validation:Optional
	// Containers is the list of containers inside a pod
	Containers []string `json:"containers,omitempty"`
}

type Conditions struct {
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

	// Phase defines the current phase of the data components.
	// +nullable:true
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Snapshot;Upload;DataRestore
	Phase OperationType `json:"phase,omitempty"`
}

// DataSnapshot defines Snapshot of a Persistent Volume
type DataSnapshot struct {

	// +nullable:true
	// +kubebuilder:validation:Optional
	// BackupType is the type of Volume backup in the sequence of backups.
	BackupType BackupType `json:"backupType,omitempty"`

	// +kubebuilder:validation:Optional
	// Location is the absolute path of qcow2 image of a volume in the target.
	Location string `json:"location,omitempty"`

	// PersistentVolumeClaimName is the name of PersistentVolumeClaim which is bound to Volume.
	PersistentVolumeClaimName string `json:"persistentVolumeClaimName"`

	// +kubebuilder:validation:MinLength=1
	// PersistentVolumeClaimMetadata is the metadata of PersistentVolumeClaim which is bound to Volume.
	PersistentVolumeClaimMetadata string `json:"persistentVolumeClaimMetadata"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// VolumeSnapshot specifies the CSI snapshot of a Persistent Volume.
	VolumeSnapshot *VolumeSnapshot `json:"volumeSnapshot,omitempty"`

	// +kubebuilder:validation:Optional
	// SnapshotSize is the size of captured snapshot of a Persistent Volume.
	SnapshotSize resource.Quantity `json:"snapshotSize,omitempty"`

	// +kubebuilder:validation:Optional
	// Size is the size of complete backup/restore.
	Size resource.Quantity `json:"size,omitempty"`

	// +kubebuilder:validation:Optional
	// Uploaded is to imply whether volume snapshot taken is uploaded to target.
	Uploaded bool `json:"uploaded,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Error is the error occurred while backing up data component if any.
	Error string `json:"error,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// PodContainersMap is the set of Pod-Containers which share Persistent Volume.
	PodContainersMap []PodContainers `json:"podContainersMap,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Conditions are the current statuses for backup and restore PVCs.
	Conditions []Conditions `json:"conditions,omitempty"`
}

// Helm defines the snapshot of application defined by a Helm.
type Helm struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Required
	// Release string is the name of release
	Release string `json:"release"`

	// +kubebuilder:validation:Optional
	// NewRelease string is the new release name which will get used while validation and restore process
	NewRelease string `json:"newRelease,omitempty"`

	// +Kubebuilder:validation:Minimum=1
	// +Kubebuilder:validation:Required
	// Revision defines the version of deployed release backed up
	Revision int32 `json:"revision"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Deprecated: Resource is the captured GVK (secret or configmap) and corresponding object names slice.
	Resource *Resource `json:"resource,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Resources are the helm release resources with their GVK and Name
	Resources []Resource `json:"resources,omitempty"`

	// StorageBackend is the enum which can be either configmaps and secrets
	StorageBackend HelmStorageBackend `json:"storageBackend"`

	// +Kubebuilder:validation:Required
	// Version represents the Helm binary version used at the time of snapshot
	Version HelmVersion `json:"version"`

	// +kubebuilder:validation:Optional
	// DataSnapshot specifies the Snapshot of the Volumes defined in the helm chart resources.
	DataSnapshots []DataSnapshot `json:"dataSnapshots,omitempty"`

	// +kubebuilder:validation:Optional
	// Warnings is the list of warnings captured during backup or restore of an application
	Warnings []string `json:"warnings,omitempty"`
}

// Operator defines the snapshot of application defined by an Operator.
type Operator struct {
	// +kubebuilder:validation:MinLength=1
	// OperatorId is unique ID for a particular operator
	OperatorID string `json:"operatorId"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// CustomResources is the list of all custom resource's GVK and names list
	CustomResources []Resource `json:"customResources,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// Helm represents the snapshot of the helm chart for helm based operator
	Helm *Helm `json:"helm,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// OperatorResources defines the a kubernetes resources found from Operator resources.
	OperatorResources []Resource `json:"operatorResources,omitempty"`

	// +kubebuilder:validation:Optional
	// DataSnapshot specifies the Snapshot of the Volumes defined in the operator resources.
	DataSnapshots []DataSnapshot `json:"dataSnapshots,omitempty"`

	// +kubebuilder:validation:Optional
	// Warnings is the list of warnings captured during backup or restore of an application
	Warnings []string `json:"warnings,omitempty"`
}

// Custom defines the snapshot of Custom defined application.
type Custom struct {

	// +kubebuilder:validation:Optional
	// Resources defines the Kubernetes resources found from Custom application.
	Resources []Resource `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// DataSnapshot specifies the Snapshot of the Volumes resources in the Custom defined application.
	DataSnapshots []DataSnapshot `json:"dataSnapshots,omitempty"`

	// +kubebuilder:validation:Optional
	// Warnings is the list of warnings captured during backup or restore of an application
	Warnings []string `json:"warnings,omitempty"`
}
