package internal

import (
	"os"
	"path"
)

const (
	TVKControlPlaneDeployment = "k8s-triliovault-control-plane"
	HTTPscheme                = "http"
	HTTPSscheme               = "https"
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
	ResourceMetadataAPIPath   = "resource-metadata"
	TrilioResourcesAPIPath    = "trilio-resources"
	Results                   = "results"
	FormatYAML                = "yaml"
	FormatJSON                = "json"
	FormatWIDE                = "wide"
	SingleNamespace           = "SingleNamespace"
	MultiNamespace            = "MultiNamespace"
	BackupKind                = "Backup"
	BackupPlanKind            = "BackupPlan"
	ClusterBackupKind         = "ClusterBackup"
	ClusterBackupPlanKind     = "ClusterBackupPlan"
	IngressKind               = "Ingress"
)

var (
	KubeConfigDefault = path.Join(os.Getenv("HOME"), ".kube", "config")
)
