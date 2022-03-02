package cmd

const (
	preflightCmdName    = "preflight"
	preflightRunCmdName = "run"
	cleanupCmdName      = "cleanup"

	namespaceFlag          = "namespace"
	namespaceFlagShorthand = "n"
	namespaceUsage         = "Namespace of the cluster in which the preflight checks will be performed"

	storageClassFlag  = "storage-class"
	storageClassUsage = "Name of storage class to use for preflight checks"

	snapshotClassFlag  = "volume-snapshot-class"
	snapshotClassUsage = "Name of volume snapshot class to use for preflight checks"

	localRegistryFlag  = "local-registry"
	localRegistryUsage = "Name of the local registry from where the images will be pulled"

	imagePullSecFlag  = "image-pull-secret"
	imagePullSecUsage = "Name of the secret for authentication while pulling the images from the local registry"

	serviceAccountFlag  = "service-account"
	serviceAccountUsage = "Name of the service account to use for preflight checks and creating preflight resources"

	cleanupOnFailureFlag  = "cleanup-on-failure"
	cleanupOnFailureUsage = "Cleanup the resources on cluster if preflight checks fail. By-default it is false"

	configFileFlag      = "config-file"
	configFileUsage     = "Specify the name of the yaml file for inputs to the preflight Run and Cleanup commands"
	configFlagShorthand = "f"

	podLimitFlag  = "limits"
	podLimitUsage = "Pod memory and cpu resource limits for volume snapshot preflight check"

	podRequestFlag  = "requests"
	podRequestUsage = "Pod memory and cpu resource requests for volume snapshot preflight check"

	pvcStorageRequestFlag  = "pvc-storage-request"
	pvcStorageRequestUsage = "PVC storage request for volume snapshot preflight check"

	uidFlag  = "uid"
	uidUsage = "UID of the preflight check whose resources must be cleaned"

	defaultCleanupMode = "all"
	uidCleanupMode     = "uid"

	preflightLogFilePrefix = "preflight"
	cleanupLogFilePrefix   = "preflight_cleanup"
	preflightUIDLength     = 6

	defaultPodRequestCPU    = "250m"
	defaultPodRequestMemory = "64Mi"
	defaultPodLimitCPU      = "500m"
	defaultPodLimitMemory   = "128Mi"
	defaultPVCStorage       = "1Gi"

	filePermission = 0644
)

var (
	kubeconfig        string
	namespace         string
	logLevel          string
	storageClass      string
	snapshotClass     string
	localRegistry     string
	imagePullSecret   string
	serviceAccount    string
	cleanupOnFailure  bool
	inputFileName     string
	podLimits         string
	podRequests       string
	pvcStorageRequest string
	cleanupUID        string
)
