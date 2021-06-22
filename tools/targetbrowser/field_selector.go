package targetbrowser

var backupSelector = []string{"metadata.name as Backup Name",
	"metadata.uid  as backupUID",
	"status.type as Backup Type",
	"status.status as Backup Status",
	"status.size as Backup Size",
	"status.location as Target Location",
	"status.completionTimestamp as Creation Date",
}

var backupPlanSelector = []string{
	"metadata.name as BackupPlan Name",
	"metadata.uid  as BackupPlanUID",
	"generatedField.applicationType as BackupPlan Type",
	"successfulBackupCount.", "generatedField.successfulBackupCount as Successful Backup",
	"successfulBackupCount.", "generatedField.lastSuccessfulBackupTimestamp as Successful Backup Timestamp",
	"successfulBackupCount.", "generatedField.tvkInstanceUID as Instance ID",
}
