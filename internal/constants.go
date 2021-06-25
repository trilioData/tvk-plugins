package internal

import (
	"os"
	"path"
)

const (
	PollingPeriod               = "POLLING_PERIOD"
	TVKControlPlaneDeployment   = "k8s-triliovault-control-plane"
	ControlPlaneContainerName   = "triliovault-control-plane"
	HTTPScheme                  = "http"
	TriliovaultGroup            = "triliovault.trilio.io"
	TargetKind                  = "Target"
	V1Version                   = "v1"
	APIPath                     = "api"
	LoginPath                   = "login"
	JweToken                    = "jweToken"
	KubeConfigParam             = "kubeconfig"
	ContentType                 = "Content-Type"
	ContentApplicationJSON      = "application/json"
	BackupPlanPath              = "backupplan"
	BackupPath                  = "backup"
	MetadataPath                = "metadata"
	Results                     = "results"
	NFSServerIPAddress          = "nfs_server_ip"
	NFSServerIP                 = "NFS_SERVER_IP"
	NFSServerBasePath           = "NFS_SERVER_BASE_PATH"
	TargetBrowserDataPath       = "/src/nfs/targetbrowsertesting"
	InstallNamespace            = "INSTALL_NAMESPACE"
	TargetName                  = "sample-target"
	TargetLocation              = "/triliodata"
	TargetBrowserBinaryName     = "target-browser"
	TargetBrowserBinaryLocation = "./../../dist/target-browser_linux_amd64"
)

var (
	KubeConfigDefault = path.Join(os.Getenv("HOME"), ".kube", "config")
)
