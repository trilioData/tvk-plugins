package targetbrowser

var backupSelector = []string{
	"metadata.name as Name",
	"metadata.uid  as UID",
	"status.type as Type",
	"status.status as Status",
	"status.size as Size",
	"spec.backupPlan.uid as BackupPlan UID",
	"status.creationTimestamp as Start Time",
	"status.completionTimestamp as End Time",
}

var backupPlanSelector = []string{
	"metadata.name as Name",
	"metadata.uid  as UID",
	"generatedField.applicationType as Type",
	"successfulBackupCount.", "generatedField.tvkInstanceUID as TVK Instance",
	"successfulBackupCount.", "generatedField.successfulBackupCount as Successful Backup",
	"successfulBackupCount.", "generatedField.lastSuccessfulBackupTimestamp as Successful Backup Timestamp",
}
