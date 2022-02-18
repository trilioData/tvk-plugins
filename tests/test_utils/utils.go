package testutils

import (
	"os"

	"github.com/trilioData/tvk-plugins/internal"
)

func GetInstallNamespace() string {
	namespace, present := os.LookupEnv(internal.InstallNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
}
