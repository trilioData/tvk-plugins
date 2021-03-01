package logcollectortest

const (
	TrilioVaultPrefix = "k8s-triliovault-"

	ControlPlane = "control-plane"
	Webhook      = "admission-webhook"
	Exporter     = "exporter"

	CRD                 = "CustomResourceDefinition"
	Pod                 = "Pod"
	Jobs                = "Job"
	StorageClass        = "StorageClass"
	VolumeSnapshotClass = "VolumeSnapshotClass"

	Backup                = "Backup"
	Restore               = "Restore"
	BackupPlan            = "BackupPlan"
	Policy                = "Policy"
	Target                = "Target"
	VolumeAttachment      = "VolumeAttachment"
	VolumeSnapshot        = "VolumeSnapshot"
	ClusterServiceVersion = "ClusterServiceVersion"

	VolumeDeviceName = "raw-volume"
)
