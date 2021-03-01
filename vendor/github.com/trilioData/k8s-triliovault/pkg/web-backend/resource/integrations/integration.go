package integrations

type Other interface {
	BackupList(requiredStatus string) (BackupList, error)
	RestoreList(requiredStatus string) (RestoreList, error)
	TargetList(requiredStatus string) (TargetList, error)
}
