package internal

import (
	"bytes"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Target types
	NFS = "nfs"
	S3  = "s3"

	// Saperators
	Backslash  = "/"
	Hyphen     = "-"
	Dot        = "."
	Underscore = "_"
	Star       = "*"
	Comma      = ","
	Equals     = "="
	Space      = " "

	// Code base path in images
	BasePath               = "/opt/tvk"
	DatastoreWaitUtil      = "datastore-attacher/scripts/waitUntilMount.py"
	DatastoreValidatorUtil = "datastore-attacher/scripts/target_validations.py"
	DatastoreMountUtil     = "datastore-attacher/mount_utility/mount_by_target_crd/mount_datastores.py"
	TvkConfigDir           = "config"

	// Image Pull Policy
	ImagePullPolicy = "IMAGE_PULL_POLICY"

	ImagePullSecret = "gcrcred"
	AppScope        = "APP_SCOPE"

	// Namespaces
	DefaultTrilioNamespace = "triliovault-integration"
	KubeSystemNamespace    = "kube-system"
	DefaultNamespace       = "default"
	ConversionNamespace    = "trilio-conversion"
	ServiceAccountName     = "k8s-triliovault"
	RestoreNamespaces      = "RESTORE_NAMESPACES"
	InstallNamespace       = "INSTALL_NAMESPACE"
	IsOpenshift            = "OPENSHIFT_INSTALL"
	SubscriptionName       = "triliovaultoperator"

	// Default ingress resource
	DefaultIngressName = "k8s-triliovault-ingress"

	// Categories
	CategoryAll         = "all"
	CategoryTriliovault = "triliovault"

	// API Groups
	SnapshotGroup = "snapshot.storage.k8s.io"

	// API Versions
	V1beta1Version  = "v1beta1"
	V1alpha1Version = "v1alpha1"
	V1Version       = "v1"

	// k8s kinds
	DeploymentKind                     = "Deployment"
	StatefulSetKind                    = "StatefulSet"
	PodKind                            = "Pod"
	DaemonSetKind                      = "DaemonSet"
	ReplicaSetKind                     = "ReplicaSet"
	ServiceKind                        = "Service"
	ReplicationControllerKind          = "ReplicationController"
	CRDKind                            = "CustomResourceDefinition"
	PVCKind                            = "PersistentVolumeClaim"
	BackupKind                         = "Backup"
	RestoreKind                        = "Restore"
	CronJobKind                        = "CronJob"
	StorageClassKind                   = "StorageClass"
	VolumeSnapshotKind                 = "VolumeSnapshot"
	VolumeSnapshotClassKind            = "VolumeSnapshotClass"
	VolumeSnapshotContentKind          = "VolumeSnapshotContent"
	BackupplanKind                     = "BackupPlan"
	TargetKind                         = "Target"
	PolicyKind                         = "Policy"
	HookKind                           = "Hook"
	TrilioVaultManagerKind             = "TrilioVaultManager"
	JobKind                            = "Job"
	LicenseKind                        = "License"
	ConfigMapKind                      = "ConfigMap"
	LimitRangeKind                     = "LimitRange"
	NamespaceKind                      = "Namespace"
	NodeKind                           = "Node"
	PersistentVolumeClaimKind          = "PersistentVolumeClaim"
	ResourceQuotaKind                  = "ResourceQuota"
	SecretKind                         = "Secret"
	ServiceAccountKind                 = "ServiceAccount"
	MutatingWebhookConfigurationKind   = "MutatingWebhookConfiguration"
	ValidatingWebhookConfigurationKind = "ValidatingWebhookConfiguration"
	CertificateSigningRequestKind      = "CertificateSigningRequest"
	IngressKind                        = "Ingress"
	NetworkPolicyKind                  = "NetworkPolicy"
	ClusterRoleBindingKind             = "ClusterRoleBinding"
	ClusterRoleKind                    = "ClusterRole"
	RoleBindingKind                    = "RoleBinding"
	RoleKind                           = "Role"
	PriorityClassKind                  = "PriorityClass"
	CSIDriverKind                      = "CSIDriver"
	CSINodeKind                        = "CSINode"
	PodDisruptionBudgetKind            = "PodDisruptionBudget"
	PodSecurityPolicyKind              = "PodSecurityPolicy"
	HorizontalPodAutoscalerKind        = "HorizontalPodAutoscaler"

	// OCP resources
	RouteKind                 = "Route"
	EgressNetworkPolicyKind   = "EgressNetworkPolicy"
	AlertmanagerKind          = "Alertmanager"
	PodMonitorKind            = "PodMonitor"
	PrometheusKind            = "Prometheus"
	PrometheusRuleKind        = "PrometheusRule"
	ServiceMonitorKind        = "ServiceMonitor"
	SCCKind                   = "SecurityContextConstraints"
	ClusterServiceVersionKind = "ClusterServiceVersion"
	CatalogSourceKind         = "CatalogSource"
	InstallPlanKind           = "InstallPlan"
	SubscriptionKind          = "Subscription"
	PackageManifestKind       = "PackageManifest"
	// ocp groups
	OperatorCoreOSGroup  = "operators.coreos.com"
	PackageOperatorGroup = "packages.operators.coreos.com"

	// ocp backup Ignore list
	OperatorGroupKind                      = "OperatorGroup"
	ClusterAutoscalerKind                  = "ClusterAutoscaler"
	MachineAutoscalerKind                  = "MachineAutoscaler"
	APIServerKind                          = "ApiServer"
	AuthenticationKind                     = "Authentication"
	BuildKind                              = "Build"
	ClusterOperatorKind                    = "ClusterOperator"
	ClusterVersionKind                     = "ClusterVersion"
	ConsoleKind                            = "Console"
	DNSKind                                = "DNS"
	FeatureGateKind                        = "FeatureGate"
	ImageKind                              = "Image"
	InfrastructureKind                     = "Infrastructure"
	OAuthKind                              = "OAuth"
	OperatorHubKind                        = "OperatorHub"
	ProjectKind                            = "Project"
	ProxyKind                              = "Proxy"
	SchedulerKind                          = "Scheduler"
	ImageSignatureKind                     = "ImageSignature"
	ImageStreamImageKind                   = "ImageStreamImage"
	ImageStreamImportKind                  = "ImageStreamImport"
	ImageStreamMappingKind                 = "ImageStreamMapping"
	ImageStreamKind                        = "ImageStream"
	ImageStreamTagKind                     = "ImageStreamTag"
	ConfigKind                             = "Config"
	DNSRecordKind                          = "DNSRecord"
	MachineHealthCheckKind                 = "MachineHealthCheck"
	MachineKind                            = "Machine"
	MachineSetKind                         = "MachineSet"
	ContainerRuntimeConfigKind             = "ContainerRuntimeConfig"
	ControllerConfigKind                   = "ControllerConfig"
	KubeletConfigKind                      = "KubeletConfig"
	MachineConfigPoolKind                  = "MachineConfigPool"
	MachineConfigKind                      = "MachineConfig"
	MCOConfigKind                          = "MCOConfig"
	BareMetalHostKind                      = "BareMetalHost"
	ClusterNetworkKind                     = "ClusterNetwork"
	HostSubnetKind                         = "HostSubnet"
	NetNamespaceKind                       = "NetNamespace"
	OAuthAccessTokenKind                   = "OAuthAccessToken"
	OAuthAuthorizeTokenKind                = "OAuthAuthorizeToken"
	OAuthClientAuthorizationKind           = "OAuthClientAuthorization"
	OAuthClientKind                        = "OAuthClient"
	ImageContentSourcePolicyKind           = "ImageContentSourcePolicy"
	IngressControllerKind                  = "IngressController"
	KubeAPIServerKind                      = "KubeAPIServer"
	KubeControllerManagerKind              = "KubeControllerManager"
	KubeSchedulerKind                      = "KubeScheduler"
	NetworkKind                            = "Network"
	OpenShiftAPIServerKind                 = "OpenShiftAPIServer"
	OpenShiftControllerManagerKind         = "OpenShiftControllerManager"
	ServiceCAKind                          = "ServiceCA"
	ServiceCatalogAPIServerKind            = "ServiceCatalogAPIServer"
	ServiceCatalogControllerManagerKind    = "ServiceCatalogControllerManager"
	ProjectRequestKind                     = "ProjectRequest"
	AppliedClusterResourceQuotaKind        = "AppliedClusterResourceQuota"
	ClusterResourceQuotaKind               = "ClusterResourceQuota"
	TunedKind                              = "Tuned"
	GroupKind                              = "Group"
	IdentityKind                           = "Identity"
	UserIdentityMappingKind                = "UserIdentityMapping"
	UserKind                               = "User"
	LocalResourceAccessReviewKind          = "LocalResourceAccessReview"
	SubjectRulesReviewKind                 = "SubjectRulesReview"
	ResourceAccessReviewKind               = "ResourceAccessReview"
	PodSecurityPolicyReviewKind            = "PodSecurityPolicyReview"
	PodSecurityPolicySelfSubjectReviewKing = "PodSecurityPolicySelfSubjectReview"
	PodSecurityPolicySubjectReviewKind     = "PodSecurityPolicySubjectReview"
	AdmissionReviewKind                    = "AdmissionReview"

	// ocp restore ignore list
	ConsoleCLIDownloadKind     = "ConsoleCLIDownload"
	ConsoleExternalLogLinkKind = "ConsoleExternalLogLink"
	ConsoleLinkKind            = "ConsoleLink"
	ConsoleNotificationKind    = "ConsoleNotification"
	ConsoleYAMLSampleKind      = "ConsoleYAMLSample"

	// Ignored Resources
	ComponentStatusKind            = "ComponentStatus"
	EndpointsKind                  = "Endpoints"
	EventKind                      = "Event"
	APIServiceKind                 = "APIService"
	ControllerRevisionKind         = "ControllerRevision"
	TokenReviewKind                = "TokenReview"
	LocalSubjectAccessReviewKind   = "LocalSubjectAccessReview"
	SelfSubjectAccessReviewKind    = "SelfSubjectAccessReview"
	BindingKind                    = "Binding"
	SelfSubjectRulesReviewKind     = "SelfSubjectRulesReview"
	SubjectAccessReviewKind        = "SubjectAccessReview"
	LeaseKind                      = "Lease"
	EndpointSliceKind              = "EndpointSlice"
	RuntimeClassKind               = "RuntimeClass"
	NodeProxyOptionsKind           = "NodeProxyOptions"
	PersistentVolumeKind           = "PersistentVolume"
	PodAttachOptionsKind           = "PodAttachOptions"
	EvictionKind                   = "Eviction"
	PodExecOptionsKind             = "PodExecOptions"
	PodPortForwardOptionsKind      = "PodPortForwardOptions"
	PodProxyOptionsKind            = "PodProxyOptions"
	PodTemplateKind                = "PodTemplate"
	ScaleKind                      = "Scale"
	TokenRequestKind               = "TokenRequest"
	ServiceProxyOptionsKind        = "ServiceProxyOptions"
	DeploymentRollbackKind         = "DeploymentRollback"
	ReplicationControllerDummyKind = "ReplicationControllerDummy"
	NodeMetricsKind                = "NodeMetrics"
	PodMetricsKind                 = "PodMetrics"

	// Extras
	VolumeSnapshotMaxRetryCount   = 5
	VolumeSnapshotWaitingDuration = 20 * time.Minute

	// operation parts phases
	MetadataSnapshot      = "MetadataSnapshot"
	DataSnapshot          = "DataSnapshot"
	MetadataUpload        = "MetadataUpload"
	DataUpload            = "DataUpload"
	MetadataRestore       = "MetadataRestore"
	DataRestore           = "DataRestore"
	InvalidOperationJob   = "InvalidOperationJob"
	StatusUpdateFailed    = "StatusUpdateFailed"
	OrphanCronJobsDeleted = "OrphanCronJobsDeleted"

	// Env Variables
	TVKVersion           = "RELEASE_TAG"
	TargetBrowserAPIPath = "TARGET_API_PATH"
	TargetType           = "TARGET_TYPE"
	HelmVersion          = "HELM_VERSION"
	Helm3Version         = "v3"
	TvkConfig            = "TVK_CONFIG"
	LibguestfsDebug      = "LIBGUESTFS_DEBUG"
	LibguestfsTrace      = "LIBGUESTFS_TRACE"
	LibGuestfsEnable     = "1"
	LibGuestfsDisable    = "0"

	// Fixed image names via env vars
	DataStoreAttacherImage = "RELATED_IMAGE_DATASTORE_ATTACHER"
	DataMoverImage         = "RELATED_IMAGE_DATAMOVER"
	MetaMoverImage         = "RELATED_IMAGE_METAMOVER"
	MetaProcessorImage     = "RELATED_IMAGE_METAPROCESSOR"
	BackupSchedulerImage   = "RELATED_IMAGE_BACKUP_SCHEDULER"
	BackupCleanerImage     = "RELATED_IMAGE_BACKUP_CLEANER"
	TargetBrowserImage     = "RELATED_IMAGE_TARGET_BROWSER"
	BackupRetentionImage   = "RELATED_IMAGE_BACKUP_RETENTION"
	HookImage              = "RELATED_IMAGE_HOOK"

	// Container
	MetamoverContainer       = "metamover"
	DatamoverContainer       = "datamover"
	BackupCleanupContainer   = "backup-cleanup"
	BackupSchedulerContainer = "backup-scheduler"

	// DataMover
	DataAttacherMaxTime                  = 20
	VolumeDeviceName                     = "raw-volume"
	MountPath                            = "/src/data"
	PseudoBlockDevicePath                = "/raw-dev"
	DeviceDetectScript                   = "/data/create-dev.sh"
	DefaultServiceAccountVolumeMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
	EmptyDirVolumeName                   = "empty-dir-volume"
	EmptyDirMountPath                    = "/data"

	TargetControllerFieldSelector     = ".target.controller"
	VolumeSnapshotFieldSelector       = "volsnapcontroller"
	BackupPlanControllerFieldSelector = ".backupplan.controller"
	BackupPlanRetentionFiedlSelector  = ".backupplan.retention"
	BackupControllerFieldSelector     = ".backup.controller"
	ControllerFieldSelector           = ".metadata.controller"
	ControllerOwnerName               = "controller-owner-name"
	ControllerOwnerNamespace          = "controller-owner-namespace"
	ControllerOwnerUID                = "controller-owner-uid"
	ChildDeleteFinalizer              = "child-delete-finalizer"
	BackupCleanupFinalizer            = "backup-cleanup-finalizer"
	TargetDeleteFinalizer             = "target-delete-finalizer"

	HelmVersionV3Binary = "helm"

	// BackupPlan Cron Job
	BackupType = "backup-type"

	// Job Operations
	Operation = "operation"

	TargetValidationOperation          = "target-validation"
	MetadataRestoreValidationOperation = "metadata-restore-validation"
	DataRestoreOperation               = "data-restore"
	MetadataRestoreOperation           = "metadata-restore"

	SnapshotterOperation    = "snapshotter"
	DataUploadOperation     = "data-upload"
	MetadataUploadOperation = "metadata-upload"
	RetentionOperation      = "retention"
	BackupCleanerOperation  = "backup-cleaner"
	HookOperation           = "hook-execution"
	QuiesceOperation        = "quiesce"
	UnquiesceOperation      = "unquiesce"

	AppComponent        = "app-component"
	ComponentIdentifier = "component-identifier"

	RestorePVCName = "restore-pvc-name"
	UploadPVCName  = "upload-pvc-name"

	RetryCount = "retry-count"

	// CRD scopes
	NamespacedScope = "Namespaced"
	ClusterScope    = "Cluster"

	CrdGroup       = "apiextensions.k8s.io"
	CrdResource    = "customresourcedefinitions"
	AdmissionGroup = "admission.k8s.io"

	CoreGroup          = ""
	AppsGroup          = "apps"
	TrilioVaultGroup   = "triliovault.trilio.io"
	AuthorizationGroup = "rbac.authorization.k8s.io"

	// Restore CR Conditions
	Status    = "status"
	Type      = "type"
	Timestamp = "timestamp"
	Reason    = "reason"

	// Role Verbs
	GET      = "get"
	LIST     = "list"
	WATCH    = "watch"
	CREATE   = "create"
	PATCH    = "patch"
	UPDATE   = "update"
	ESCALATE = "escalate"
	BIND     = "bind"

	// Other constants
	DefaultHelmAppRevision  = 0
	CharSetMinASCII         = 97
	CharSetMaxASCII         = 122
	HelmNameRandomStrLen    = 4
	HelmMaxNameLen          = 58
	MaxAttemptsToGetRelName = 5
	MaxNameOrLabelLen       = 63
	CronJobNameMaxLen       = 50
	ObjNameRegex            = "(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$"

	// Resource request constants
	NonDmLimitMem        = "512Mi"  // 512 MiB
	NonDmLimitCPU        = "500m"   // 500m
	NonDmRequestCPU      = "0.01"   // 10 milliCore/milliCPU
	NonDmRequestMem      = "10Mi"   // 10 MiB
	DmLimitMem           = "1536Mi" // 1536MiB
	DmLimitCPU           = "1000m"  // 1000m
	DmRequestCPU         = "0.1"    // 100 milliCore/milliCPU
	DmRequestMem         = "800Mi"  // 800 MiB
	DeploymentLimitMem   = "512Mi"  // 512 MiB
	DeploymentRequestMem = "10Mi"   // 10 MiB
	DeploymentRequestCPU = "10m"    // 10 milliCore/milliCPU
	DeploymentLimitCPU   = "200m"   // 200 milliCore/milliCPU

	// Resource request type
	DMJobResource      = "datamover"
	NonDMJobResource   = "non-datamover"
	DeploymentResource = "helm-deployed"

	// Todo Arrange following constants
	Qcow2PV                = "pv.qcow2"
	MetadataSnapshotDir    = "metadata-snapshot"
	DataSnapshotDir        = "data-snapshot"
	PVCJSON                = "pvc.json"
	TVKMetaFile            = "tvk-meta.json"
	ReleaseConfFile        = "release-config.json"
	PodContainerMapFile    = "pod-container.json"
	MetadataJSON           = "metadata.json"
	BackupJSON             = "backup.json"
	BackupPlanJSON         = "backupplan.json"
	TargetJSON             = "target.json"
	PolicyJSON             = "policy.json"
	HooksJSON              = "hooks.json"
	BackupNamespaceJSON    = "backup-namespace.json"
	HelmBackupDir          = "helm"
	HelmDependencyDir      = "dependencies"
	HelmSubChartPath       = "charts"
	OperatorBackupDir      = "operator"
	CustomBackupDir        = "custom"
	ResourceMetadataJSON   = "resource-metadata.json"
	CrdMetadataJSON        = "crd-metadata.json"
	HelmVersionKey         = "helmVersion"
	HelmStorageBackendKey  = "storageBackend"
	HelmRevisionKey        = "revision"
	DefaultDatastoreBase   = "/triliodata"
	DefaultPathFsPV        = "/src/path"
	TmpMountDir            = "/mnt/datamover"
	BackupMetadataAction   = "backup-metadata"
	BackupDataAction       = "backup-data"
	MetadataUploadAction   = "metadata-upload"
	SnapshotAction         = "snapshot"
	ValidateAction         = "validate"
	RestoreUpdatePVCAction = "restore-update-pvc"
	RestoreDataAction      = "restore-data"
	RestoreAction          = "restore"
	RestoreMetadataAction  = "restore-metadata"
	CleanupAction          = "cleanup"
	Retention              = "retention"
	TmpDir                 = "/tmp"
	OperatorKind           = "operator"
	CustomKind             = "custom"
	HelmKind               = "helm"
	Generation             = "generation"

	// Required values for recommended labels
	ManagedBy    = "k8s-triliovault"
	PartOf       = "k8s-triliovault"
	DefaultLabel = "k8s-triliovault"

	// License Keys
	LicenseCompany           = "Company"
	LicenseEdition           = "Edition"
	LicenseCreationDate      = "CreationDate"
	LicensePurchaseDate      = "PurchaseDate"
	LicenseExpiration        = "Expiration"
	LicenseMaintenanceExpiry = "MaintenanceExpiryDate"
	LicenseKubeUID           = "KubeUID"
	LicenseScope             = "Scope"
	LicenseVersion           = "licenseVersion"
	LicenseSEN               = "SEN"
	LicenseNumberOfUsers     = "NumberOfUsers"
	LicenseServerID          = "ServerID"
	LicenseLicenseID         = "LicenseID"
	LicenseCapacity          = "Capacity"
	LicenseActive            = "active"

	MasterNodeTaintKey = "node-role.kubernetes.io/master"

	CapacityKubeNodes = "Kube Nodes"
	GracePeriodInDays = 30

	ISODateFormat     = "2006-01-02"
	ISODateTimeFormat = "2006-01-02T15:04:05.000Z"

	CheckAuthInfoLabel = "checkAuthInfo"

	ReleaseKey = "release"

	NamespaceLabelKey = "trilio-label"

	ScheduleType = "scheduleType"

	// 	Resource Requirements keys
	MetaMoverJobLimits = "metadataJobLimits"
	DataMoverJobLimits = "dataJobLimits"

	// Field selectors
	TargetToBackupplanFieldSelector       = "spec.backupconfig.target.name"
	BackupplanToBackupFieldSelector       = "spec.backupplan.name"
	BackupToRestoreFieldSelector          = "spec.source.backup.name"
	BackupPlanStatsToRestoreFieldSelector = "status.stats.backupPlan.name"
	TargetStatsToBackupFieldSelector      = "status.stats.target.name"

	WebSessionAccessTTLEnv        = "WEB_SESSION_ACCESS_TTL"
	WebSessionRefreshTTLEnv       = "WEB_SESSION_REFRESH_TTL"
	WebSessionMaxInactivityTTLEnv = "WEB_SESSION_MAX_INACTIVITY_TTL"

	// Profiling
	ProfilingCollector    = "PROFILING_COLLECTOR"
	ControlPlane          = "control-plane"
	TargetBrowser         = "target-browser"
	ProfilingTickInterval = 2
)

var DeleteRetry = wait.Backoff{
	Steps:    5,
	Duration: 5 * time.Second,
	Factor:   1.5,
	Jitter:   0.1,
}

var DefaultTime, _ = time.Parse(time.RFC3339, "0001-01-01T00:00:00Z")

var CoreResources = []string{"services/finalizers"}
var AppsResources = []string{"deployments/finalizers"}

var PodExecResource = []string{"pods/exec"}
var PodLogResource = []string{"pods/log"}

var ProfilingBuffer bytes.Buffer

// Required Capabilities
var (
	MountCapability              = []corev1.Capability{"SYS_ADMIN"}
	GeneralCap                   = []corev1.Capability{"KILL", "AUDIT_WRITE"}
	DatamoverCap                 = []corev1.Capability{"SYS_ADMIN"}
	IngressCapability            = []corev1.Capability{"SYS_ADMIN", "NET_BIND_SERVICE"}
	IngressNonRootUserID   int64 = 101
	RunAsNonRoot                 = false
	ReadOnlyRootFilesystem       = false
	RunAsRootUserID        int64
	UnPrivileged                 = false
	Privileged                   = true
	RunAsNonRootUserID     int64 = 1001
	RunAsNormalUser              = true
)

type SnapshotType string

const (
	Custom   SnapshotType = "custom"
	Helm     SnapshotType = "helmCharts"
	Operator SnapshotType = "operators"
)

// Warning Types and respective Formats
const (
	ModifiedResourceWarning          = "Resource Modified"
	NotSupportedWarning              = " Not Supported"
	PodNotRunningWarning             = "Found Pods are not in Running/Succeeded state"
	DependantResourceWarning         = "Found Dependant Resource"
	DependentResourceNotFoundWarning = "Dependent Resource Not Found In Namespace/Cluster"
	DependentCRDNotFoundWarning      = "Dependent CRD Not Found For Given CR"
	HostNetworkWarning               = "Found HostNetwork set to true"
	HostPortWarning                  = "Found HostPort"
	NodeSelectorWarning              = "Found NodeSelector"
	NodeAffinityWarning              = "Found NodeAffinity"
	NodePortWarning                  = "Found NodePort"
)

const (
	K8sPartOfLabel        = "app.kubernetes.io/part-of"
	DefaultServiceAccount = "default"
	// nolint:gosec // this OcpSecretAnnotation does not contain any secrets or credential info
	OcpSecretAnnotation = "kubernetes.io/service-account.name"
)

// IgnoreExtInDownloadZip ignore the qcow2 extension files while creating a metadata zip file from target browser
const IgnoreExtInDownloadZip = ".qcow2"
