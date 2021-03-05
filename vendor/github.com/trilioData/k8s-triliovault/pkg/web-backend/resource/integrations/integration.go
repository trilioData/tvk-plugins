package integrations

type Other interface {
	BackupList(listRequestParams *ListRequestParams) (BackupList, error)
	RestoreList(listRequestParams *ListRequestParams) (RestoreList, error)
	TargetList(listRequestParams *ListRequestParams) (TargetList, error)
}

type QueryParameters string
type OrderingParamField string

type OrderingParam struct {
	Field     string
	Ascending bool
}

const (
	StatusQueryParam         QueryParameters = "status"
	OrderingQueryParam       QueryParameters = "ordering"
	TimeRangeFieldQueryParam QueryParameters = "timeRangeField"
	TimeRangeValueQueryParam QueryParameters = "timeRangeValue"
	NamespaceQueryParam      QueryParameters = "namespace"

	CreationTimestamp OrderingParamField = "creationTimestamp"
	Status            OrderingParamField = "status"
	Name              OrderingParamField = "name"

	ExpirationTimestamp OrderingParamField = "expirationTimestamp"
	// Restore CompletionTimestamp
	RestoreTimestamp OrderingParamField = "restoreTimestamp"
	StorageType      OrderingParamField = "storageType"
	ProviderName     OrderingParamField = "vendorName"

	CreationTimestampAsc  string = "creationTimestamp"
	CreationTimestampDesc string = "-creationTimestamp"
	NameAsc               string = "name"
	NameDesc              string = "-name"
	StatusAsc             string = "phase"
	StatusDesc            string = "-phase"

	ExpirationTimestampAsc  string = ".expirationTimestamp"
	ExpirationTimestampDesc string = "-expirationTimestamp"
	RestoreTimestampAsc     string = "restoreTimestamp"
	RestoreTimestampDesc    string = "-restoreTimestamp"
	ProviderNameAsc         string = "vendorName"
	ProviderNameDesc        string = "-vendorName"

	// not sure about this
	StorageTypeAsc  string = "objectStorage"
	StorageTypeDesc string = "-objectStorage"
)
