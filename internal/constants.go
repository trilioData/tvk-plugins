package internal

import (
	"os"
	"path"
)

const (
	TVKControlPlaneDeployment = "k8s-triliovault-control-plane"
	HTTPScheme                = "http"
	TriliovaultGroup          = "triliovault.trilio.io"
	TargetKind                = "Target"
	V1Version                 = "v1"
	APIPath                   = "api"
	LoginPath                 = "login"
	JweToken                  = "jweToken"
	KubeConfigParam           = "kubeconfig"
	ContentType               = "Content-Type"
	ContentApplicationJSON    = "application/json"
	BackupPlanAPIPath         = "backupplan"
	BackupAPIPath             = "backup"
	MetadataAPIPath           = "metadata"
	Results                   = "results"
)

var (
	KubeConfigDefault = path.Join(os.Getenv("HOME"), ".kube", "config")
)
