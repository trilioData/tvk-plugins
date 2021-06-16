package tools

import (
	"os"
	"path"
)

const (
	HTTPScheme             = "http"
	TriliovaultGroup       = "triliovault.trilio.io"
	TargetKind             = "Target"
	V1Version              = "v1"
	APIPath                = "api"
	LoginPath              = "login"
	JweToken               = "jweToken"
	KubeConfigParam        = "kubeconfig"
	ContentType            = "Content-Type"
	ContentApplicationJSON = "application/json"
)

var (
	KubeConfigDefault = path.Join(os.Getenv("HOME"), ".kube", "config")
)
