package licensekeygen

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
)

type KeyGenArgKey string

const (
	Organization    KeyGenArgKey = "--organization"
	KubeUID         KeyGenArgKey = "--kube_uid"
	ServerID        KeyGenArgKey = "--server_id"
	KubeScope       KeyGenArgKey = "--kube_scope"
	LicenseEdition  KeyGenArgKey = "--license_edition"
	LicenseTypeName KeyGenArgKey = "--license_type_name"
	PurchaseDate    KeyGenArgKey = "--purchase_date"
	ExpirationDate  KeyGenArgKey = "--expiration_date"
	LicensedFor     KeyGenArgKey = "--licensed_for"
)

type KeyGenArgs map[KeyGenArgKey]string

func CreateLicenseKey(projectPath string, args KeyGenArgs) (string, error) {
	argString := ""
	keygenFilePath := "tests/integration/common/licensekeygen/keygen.py"
	for key, value := range args {
		arg := fmt.Sprintf(" %s %s", string(key), value)
		argString += arg
	}

	filePath := filepath.Join(projectPath, keygenFilePath)
	cmd := fmt.Sprintf("python3 %s %s", filePath, argString)
	logrus.Infof("License Creator CMD [%s]", cmd)
	cmdOut, err := shell.RunCmd(cmd)
	logrus.Infof("License Creator Output- [%s]", cmdOut.Out)
	return cmdOut.Out, err
}
