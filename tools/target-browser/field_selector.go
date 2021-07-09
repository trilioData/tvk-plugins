package targetbrowser

var backupSelector = []string{
	"metadata.name as Name",
	"metadata.uid  as UID",
	"status.type as Type",
	"status.status as Status",
	"status.size as Size",
	"status.location as Location",
	"status.creationTimestamp as Created At",
}

var backupPlanSelector = []string{
	"metadata.name as Name",
	"metadata.uid  as UID",
	"generatedField.applicationType as Type",
	"successfulBackupCount.", "generatedField.successfulBackupCount as Successful Backup",
	"successfulBackupCount.", "generatedField.lastSuccessfulBackupTimestamp as Successful Backup Timestamp",
	"successfulBackupCount.", "generatedField.tvkInstanceUID as TVK Instance",
}
