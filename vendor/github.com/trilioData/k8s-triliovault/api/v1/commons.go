package v1

// +kubebuilder:validation:type=string
// Status specifies the status of WorkloadJob operating on
type Status string

const (
	// Pending means the process is created and yet to be processed
	Pending Status = "Pending"

	// InProgress means the process is under execution
	InProgress Status = "InProgress"

	// Completed means the process execution successfully completed.
	Completed Status = "Completed"

	// Failed means the process is unsuccessful due to an error.
	Failed Status = "Failed"

	// Available means the resources blocked for the process execution are now available.
	Available Status = "Available"

	// Unavailable means the resources blocked for the process execution
	Unavailable Status = "Unavailable"

	// Error means the resources blocked for the process execution
	Error Status = "Error"

	// Coalescing means the backup is in intermediate state
	Coalescing Status = "Coalescing"
)

// +kubebuilder:validation:type=string
// OperationType specifies the type of operation for Job
type OperationType string

const (
	// Snapshot means the snapshot operation of kubernetes resources
	SnapshotOperation OperationType = "Snapshot"

	// Upload means the operation where resources uploaded to target
	UploadOperation OperationType = "Upload"

	// Upload means the operation where resources uploaded to target
	MetadataUploadOperation OperationType = "MetadataUpload"

	// Restore means the restoring resources from target back to cluster
	RestoreOperation OperationType = "Restore"

	//	Retention means the on successful backup going to maintain backups based on policy
	RetentionOperation OperationType = "Retention"

	// Validation will be used in case of validation operations on resources
	ValidationOperation OperationType = "Validation"

	// QuiesceOperation will be used in pre hook execution on identified Pod and containers
	QuiesceOperation OperationType = "Quiesce"

	// UnquiesceOperation will be used in post hook execution on identified Pod and containers
	UnquiesceOperation OperationType = "Unquiesce"

	// MetaSnapshotOperation means operation where metadata resources are under snapshot operation
	MetaSnapshotOperation OperationType = "MetaSnapshot"

	// DataSnapshotOperation means operation where data resources are under snapshot operation
	DataSnapshotOperation OperationType = "DataSnapshot"

	// MetaSnapshotOperation means operation where metadata resource are under snapshot operation
	DataUploadOperation OperationType = "DataUpload"

	// TargetBrowsingOperation means operation where target browsing is toggled for target instance
	TargetBrowsingOperation OperationType = "TargetBrowsing"

	// DataRestoreOperation means the restore of particular data component
	DataRestoreOperation OperationType = "DataRestore"

	// DataUploadUnquiesceOperation means the data upload and unquiesce are going in parallel
	DataUploadUnquiesceOperation OperationType = "DataUploadUnquiesce"
)

// +kubebuilder:validation:Enum=Validation;DataRestore;MetadataRestore;PrimitiveMetadataRestore;Unquiesce
// +kubebuilder:validation:type=string
// RestorePhase specifies the one of phase of Restore operation
type RestorePhase string

const (
	// RestoreValidation means the validation of backed up resources for the restore
	RestoreValidation RestorePhase = "Validation"

	// DataRestore means the restore operation of volumes from backed up images
	DataRestore RestorePhase = "DataRestore"

	// MetadataRestore means the restore operation of backed up validated metadata
	MetadataRestore RestorePhase = "MetadataRestore"

	// MetadataRestore means the restore operation of backed up validated metadata
	UnquiesceRestore RestorePhase = "Unquiesce"

	// PrimitiveMetadataRestore means the restore operation of primitive backed up resources
	// This RestorePhase will occur after validation phase
	PrimitiveMetadataRestore RestorePhase = "PrimitiveMetadataRestore"
)

// +kubebuilder:validation:Enum=v3
// +kubebuilder:validation:type=string
// HelmVersion defines the version of helm binary used while backup; currently supported version is v3
type HelmVersion string

const (
	// HelmV3 specifies the helm 2 binary version
	Helm3 HelmVersion = "v3"
)

// +kubebuilder:validation:Enum=ConfigMap;Secret
// +kubebuilder:validation:type=string
// HelmStorageBackend defines the enum for the types of storage backend from where the helm release is backed-up
type HelmStorageBackend string

const (
	ConfigMap HelmStorageBackend = "ConfigMap"
	Secret    HelmStorageBackend = "Secret"
)

// +kubebuilder:validation:type=string
// Scope specifies the scope of a resource.
type Scope string

const (
	ClusterScoped   Scope = "Cluster"
	NamespaceScoped Scope = "Namespaced"
)

// ComponentScope indicates scope of components i.e. [App or Namespace] present in backup or restore
// +kubebuilder:validation:type=string
// +kubebuilder:validation:Enum=App;Namespace
type ComponentScope string

const (
	// App ComponentScope indicates that component in backup/restore is application specific i.e. custom, helm, operator
	App ComponentScope = "App"

	// Namespace ComponentScope indicates that component in backup/restore is specific namespace
	Namespace ComponentScope = "Namespace"
)

// +kubebuilder:validation:Enum=test;add;remove;replace;copy;move
// Op indicates the Json Patch operations
type Op string

const (
	AddOp     Op = "add"
	RemoveOp  Op = "remove"
	ReplaceOp Op = "replace"
	CopyOp    Op = "copy"
	MoveOp    Op = "move"
	TestOp    Op = "test"
)
