package common

import (
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/trilioData/k8s-triliovault/internal/utils/retry"
)

const (
	parentDir    = ".."
	testDataDir  = "test-data"
	mysqlHelmDir = "mysql-helm"

	HelmMysqlV3 = "mysql-v3"
	HelmMysqlV2 = "mysql-v2"

	Timeout            = time.Minute * 3
	Interval           = time.Second * 1
	ReconcileSleepTime = time.Second * 4

	PodTimeout  = time.Minute * 7
	Podinterval = time.Second * 5

	RetentionPolicy              = "retention-policy-sample"
	TargetName                   = "target-sample"
	ApplicationName              = "application-sample"
	ApplicationWithHooksName     = "application-sample-with-hooks"
	ApplicationWithRetentionName = "application-sample-with-retention-policy"
	BackupName                   = "backup-sample"
	RestoreName                  = "restore-sample"
	LicenseName                  = "license-sample"
	S3BucketName                 = "trilio-fuse-bucket"

	DockerRegistry  = "GCR_DOCKER_REGISTRY"
	GCPProject      = "amazing-chalice-243510"
	ReleaseImageTag = "PULL_PULL_SHA"

	DataMoverImageName           = "datamover"
	BackupCleanerImageName       = "backup-cleaner"
	DataMoverValidationImageName = "datamover-validation"
	DataStoreAttacherImageName   = "datastore-attacher"
	MetaMoverImageName           = "metamover"

	DefaultNs            = "default"
	ResourceCleanerImage = "resource-cleaner"
	BackupSchedularImage = "backup-scheduler"
	BackupRetentionImage = "backup-retention"
	hookExecutorImage    = "hook-executor"

	RestoreNamespace        = "RESTORE_NAMESPACE"
	BackupNamespace         = "BACKUP_NAMESPACE"
	BackupLocation          = "BACKUP_LOCATION"
	Backup                  = "BACKUP_NAME"
	UniqueID                = "UNIQUE_ID"
	InstallNamespace        = "INSTALL_NAMESPACE"
	ReleaseName             = "RELEASE_NAME"
	HostName                = "HOST_NAME"
	StorageClassPlaceholder = "STORAGE_CLASS_NAME"
	UseStoredBackup         = "USE_STORED_BACKUP"
	LicenseKey              = "LICENSE_KEY"
	Scope                   = "SCOPE"

	APIURLHost = "k8s-tvk.com"
)

var (
	DefaultRetryTimeout = retry.Timeout(time.Minute * 10)
	DefaultRetryDelay   = retry.Delay(time.Second * 1)
	DefaultRetryCount   = retry.Count(30)
	Namespace           = "INSTALL_NAMESPACE"

	StandardStorageClass = "standard"

	StorageClassName    = "STORAGE_CLASS"
	VolumeSnapshotClass = "VOLUME_SNAPSHOT_CLASS"

	AWSAccessKeyID     = "AWS_ACCESS_KEY_ID"     // #nosec
	AWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY" // #nosec
	AWSRegion          = "AWS_REGION"

	S3AccessKeyID     = "S3_ACCESS_KEY_ID"     // #nosec
	S3SecretAccessKey = "S3_SECRET_ACCESS_KEY" // #nosec
	S3Region          = "S3_REGION"
	S3URL             = "S3_URL"
	DefaultS3Region   = "us-east-1"

	NFSServerIPAddress = "NFS_SERVER_IP_ADDR"
	NFSServerBasePath  = "NFS_SERVER_BASE_PATH"
	NFSServerOptions   = "NFS_SERVER_OPTIONS"

	True                        = "true"
	DataStoreAttacherPath       = "/triliodata"
	TempDataStoreBasePath       = "/triliodata-temp"
	DataStoreAttacherHostPath   = "/mnt/test/"
	DataStoreAttacherSecretPath = "/etc/secret"
	TargetBrowserDataPath       = "/src/nfs/targetbrowser"
	// TargetLocationPrefix = "trilio-test-target-"

	TVControlPlaneDeployment = "k8s-triliovault-control-plane"
	TVWebhookDeployment      = "k8s-triliovault-admission-webhook"
	TVBackendDeployment      = "k8s-triliovault-backend"
	TVWebDeployment          = "k8s-triliovault-web"
	TVExporterDeployment     = "k8s-triliovault-exporter"
	TVIngressContDeployment  = "k8s-triliovault-ingress-controller"

	SkipCleanup        = "SKIP_CLEANUP"
	MySQLHelmChartPath = filepath.Join(parentDir, parentDir, parentDir, parentDir, testDataDir, mysqlHelmDir)

	ConflictBackoff = wait.Backoff{
		Steps:    10,
		Duration: 2 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}
)

type HelmChart struct {
	Release  string
	Revision int32
	Path     string
}
