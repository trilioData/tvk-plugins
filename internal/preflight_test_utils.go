package internal

const (
	DefaultTestStorageClass  = "csi-gce-pd"
	DefaultTestSnapshotClass = "default-snapshot-class"

	InvalidStorageClassName      = "invalid-storage-class"
	InvalidSnapshotClassName     = "invalid-snapshot-class"
	InvalidLocalRegistryName     = "invalid-local-registry"
	InvalidServiceAccountName    = "invalid-service-account"
	InvalidLogLevel              = "invalidLogLevel"
	InvalidNamespace             = "invalid-ns"
	InvalidMemoryResourceRequest = "2Ga"

	Memory128 = "128Mi"
	Memory256 = "256Mi"
	CPU300    = "300m"
	CPU400    = "400m"
	CPU600    = "600m"
)
