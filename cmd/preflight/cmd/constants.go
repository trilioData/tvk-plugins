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

	inputFileFlag      = "input-file"
	inputFileUsage     = "Specify the name of the yaml file for inputs to the preflight run and cleanup commands"
	inputFlagShorthand = "f"

	requestMemoryFlag  = "req-memory"
	requestMemoryUsage = "Memory request requirement of all pods for volume snapshot preflight check"

	limitMemoryFlag  = "lim-memory"
	limitMemoryUsage = "Memory limit requirement of all pods for volume snapshot preflight check"

	requestCPUFlag  = "req-cpu"
	requestCPUUsage = "CPU request requirement of all pods for volume snapshot preflight check"

	limitCPUFlag  = "lim-cpu"
	limitCPUUsage = "CPU limit requirement of all pods for volume snapshot preflight check"

	uidFlag  = "uid"
	uidUsage = "UID of the preflight check whose resources must be cleaned"

	cleanupModeFlag  = "cleanup-mode"
	cleanupModeUsage = "Specifies the mode of cleanup; " +
		"'all' to clean all generated preflight resources till date in the give namespace" +
		"'uid' to clean preflight resources of particular run in the given namespace"
	defaultCleanupMode = "all"
	uidCleanupMode     = "uid"

	preflightLogFilePrefix = "preflight"
	cleanupLogFilePrefix   = "preflight_cleanup"
	preflightUIDLength     = 6

	filePermission = 0644
)

var (
	kubeconfig       string
	namespace        string
	logLevel         string
	storageClass     string
	snapshotClass    string
	localRegistry    string
	imagePullSecret  string
	serviceAccount   string
	cleanupOnFailure bool
	inputFileName    string
	requestMemory    string
	limitMemory      string
	requestCPU       string
	limitCPU         string

	cleanupUID  string
	cleanupMode string
)
