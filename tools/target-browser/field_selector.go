package targetbrowser

var BackupSelector = []string{
	"metadata.name as Name",
	"kind as Kind",
	"metadata.uid  as UID",
	"status.type as Type",
	"status.status as Status",
	"status.size as Size",
	"spec.backupPlan.uid as BackupPlan UID",
	"status.startTimestamp as Start Time",
	"status.completionTimestamp as End Time",
	"spec.clusterBackupPlan.uid as BackupPlan UID",
	"generatedField.tvkInstanceUID as TVK Instance",
	"status.expirationTimestamp as Expiration Time",
}

var BackupPlanSelector = []string{
	"metadata.name as Name",
	"kind as Kind",
	"metadata.uid  as UID",
	"generatedField.applicationType as Type",
	"generatedField.tvkInstanceUID as TVK Instance",
	"generatedField.successfulBackupCount as Successful Backup",
	"generatedField.lastSuccessfulBackupTimestamp as Successful Backup Timestamp",
	"metadata.creationTimestamp as Start Time",
}
