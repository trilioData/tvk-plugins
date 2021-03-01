package common

import (
	"crypto/md5"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"time"

	cryptorand "crypto/rand"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/gomega"
	crd "github.com/trilioData/k8s-triliovault/api/v1"
	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/tvk-plugins/tests/common/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilretry "k8s.io/client-go/util/retry"
)

const (

	// k8s kinds
	DeploymentKind = "Deployment"
	PodKind        = "Pod"
	ServiceKind    = "Service"
	BackupKind     = "Backup"
	RestoreKind    = "Restore"
	BackupplanKind = "BackupPlan"
	JobKind        = "Job"

	// Fixed image names via env vars
	DataStoreAttacherImage = "RELATED_IMAGE_DATASTORE_ATTACHER"

	// DataMover
	VolumeDeviceName = "raw-volume"

	ControllerOwnerName      = "controller-owner-name"
	ControllerOwnerNamespace = "controller-owner-namespace"
	ControllerOwnerUID       = "controller-owner-uid"

	// Job Operations
	Operation = "operation"

	MetadataRestoreValidationOperation = "metadata-restore-validation"
	DataRestoreOperation               = "data-restore"
	MetadataRestoreOperation           = "metadata-restore"

	SnapshotterOperation    = "snapshotter"
	DataUploadOperation     = "data-upload"
	MetadataUploadOperation = "metadata-upload"
	RetentionOperation      = "retention"

	RestorePVCName = "restore-pvc-name"
	UploadPVCName  = "upload-pvc-name"

	NonDMJobResource = "non-datamover"

	// DataMover images
	AlpineImage   = "alpine:latest"
	joinSeparator = "\n---\n"

	timeout  = time.Second * 130
	interval = time.Second * 1
)

// Required Capabilities
var (
	MountCapability = []corev1.Capability{"SYS_ADMIN"}
	// Split where the '---' appears at the very beginning of a line. This will avoid
	// accidentally splitting in cases where yaml resources contain nested yaml (which
	// is indented).
	splitRegex = regexp.MustCompile(`(^|\n)---`)
)

type SnapshotType string

const (
	Custom   SnapshotType = "custom"
	Helm     SnapshotType = "helmCharts"
	Operator SnapshotType = "operators"
)

// Data Component Functions
type ApplicationDataSnapshot struct {
	AppComponent        SnapshotType
	ComponentIdentifier string
	DataComponent       v1.DataSnapshot
	Status              v1.Status
}

// CmdOut structure contains command output & exitcode
type CmdOut struct {
	Out      string
	ExitCode int
}

func GenerateRandomString(n int, isOnlyAlphabetic bool) string {
	letters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "1234567890"
	rand.Seed(time.Now().UnixNano())
	letterRunes := []rune(letters)
	if !isOnlyAlphabetic {
		letterRunes = []rune(letters + numbers)
	}
	b := make([]rune, n)
	for i := range b {
		randNum, _ := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(letterRunes))))
		b[i] = letterRunes[randNum.Int64()]
	}
	return string(b)
}

// RunCmd Execute given shell commands
// params:
// input=>cmd: formatted command string which needs to be executed.
// output=>*cmdOut: command output struct returned after command execution.
//			error: non-nil error if command execution failed.
func RunCmd(cmd string, env ...string) (*CmdOut, error) {
	outStruct, err := Execute(env, true, "%s", cmd)
	if err != nil {
		return outStruct, err
	}
	return outStruct, nil
}

// Execute the given command.
func Execute(env []string, combinedOutput bool, format string, args ...interface{}) (*CmdOut, error) {
	s := fmt.Sprintf(format, args...)
	// TODO: escape handling
	parts := strings.Split(s, " ")

	var p []string
	for i := 0; i < len(parts); i++ {
		if parts[i] != "" {
			p = append(p, parts[i])
		}
	}

	var argStrings []string
	if len(p) > 0 {
		argStrings = p[1:]
	}
	return ExecuteArgs(env, combinedOutput, parts[0], argStrings...)
}

// ExecuteArgs execute given command
func ExecuteArgs(env []string, combinedOutput bool, name string, args ...string) (*CmdOut, error) {
	if log.IsLevelEnabled(log.DebugLevel) {
		cmd := strings.Join(args, " ")
		cmd = name + " " + cmd
		log.Debugf("Executing command: %s", cmd)
	}

	c := exec.Command(name, args...)
	c.Env = append(os.Environ(), env...)

	var b []byte
	var err error
	if combinedOutput {
		// Combine stderr and stdout in b.
		b, err = c.CombinedOutput()
	} else {
		// Just return stdout in b.
		b, err = c.Output()
	}

	if err != nil || !c.ProcessState.Success() {
		log.Debugf("Command[%s] => (FAILED) %s", name, string(b))
	} else {
		log.Debugf("Command[%s] => %s", name, string(b))
	}

	return &CmdOut{
		Out:      string(b),
		ExitCode: c.ProcessState.ExitCode(),
	}, err
}

func (a *ApplicationDataSnapshot) GetHash() string {
	return GetHash(string(a.AppComponent), a.ComponentIdentifier, a.DataComponent.PersistentVolumeClaimName)
}

// SplitString splits the given yaml doc if it's multipart document.
func SplitString(yamlText string) []string {
	out := make([]string, 0)
	parts := splitRegex.Split(yamlText, -1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) > 0 {
			out = append(out, part)
		}
	}
	return out
}

// JoinString joins the given yaml parts into a single multipart document.
func JoinString(parts ...string) string {
	// Assume that each part is already a multi-document. Split and trim each part,
	// if necessary.
	toJoin := make([]string, 0, len(parts))
	for _, part := range parts {
		toJoin = append(toJoin, SplitString(part)...)
	}

	return strings.Join(toJoin, joinSeparator)
}

func RunCmdWithOutput(command string) error {
	parts := strings.Split(command, " ")
	var argStrings []string
	if len(parts) > 0 {
		argStrings = parts[1:]
	}

	// suppress linter here issue -> G204: Subprocess launched with function call as argument or cmd arguments
	cmd := exec.Command(parts[0], argStrings...) //nolint:gosec // no other options here
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func GetHash(appComponent, componentIdentifier, pvcName string) string {
	str := fmt.Sprintf("%s-%s-%s", appComponent, componentIdentifier, pvcName)
	h := md5.New() // #nosec
	_, _ = h.Write([]byte(str))
	return string(h.Sum(nil))
}

func GetBackupDataComponents(backupSnapshot *v1.Snapshot,
	isVolumeSnapshotCompleted bool) (backupDataComponents []ApplicationDataSnapshot,
	aggregateCount int) {

	backupDataComponents = []ApplicationDataSnapshot{}

	if backupSnapshot == nil {
		return backupDataComponents, aggregateCount
	}

	// Append custom data components
	if backupSnapshot.Custom != nil {
		aggregateCount += len(backupSnapshot.Custom.DataSnapshots)
		for dataComponentIndex := range backupSnapshot.Custom.DataSnapshots {
			dataComponent := backupSnapshot.Custom.DataSnapshots[dataComponentIndex]
			appDs := ApplicationDataSnapshot{AppComponent: Custom, DataComponent: dataComponent}
			if isVolumeSnapshotCompleted {
				if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			} else {
				backupDataComponents = append(backupDataComponents, appDs)
			}
		}
	}

	// Append helm data components
	for i := 0; i < len(backupSnapshot.HelmCharts); i++ {
		helmApplication := backupSnapshot.HelmCharts[i]
		aggregateCount += len(helmApplication.DataSnapshots)
		for dataComponentIndex := range helmApplication.DataSnapshots {
			dataComponent := helmApplication.DataSnapshots[dataComponentIndex]
			appDs := ApplicationDataSnapshot{AppComponent: Helm, ComponentIdentifier: helmApplication.Release,
				DataComponent: dataComponent}
			if isVolumeSnapshotCompleted {
				if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			} else {
				backupDataComponents = append(backupDataComponents, appDs)
			}
		}
	}

	// Append operator data components
	for i := 0; i < len(backupSnapshot.Operators); i++ {
		operatorApplication := backupSnapshot.Operators[i]
		aggregateCount += len(operatorApplication.DataSnapshots)
		for dataComponentIndex := range operatorApplication.DataSnapshots {
			dataComponent := operatorApplication.DataSnapshots[dataComponentIndex]
			appDs := ApplicationDataSnapshot{AppComponent: Operator,
				ComponentIdentifier: operatorApplication.OperatorID, DataComponent: dataComponent}
			if isVolumeSnapshotCompleted {
				if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			} else {
				backupDataComponents = append(backupDataComponents, appDs)
			}
		}
		if operatorApplication.Helm != nil {
			operatorHelm := operatorApplication.Helm
			aggregateCount += len(operatorHelm.DataSnapshots)
			for dataComponentIndex := range operatorHelm.DataSnapshots {
				dataComponent := operatorHelm.DataSnapshots[dataComponentIndex]
				appDs := ApplicationDataSnapshot{AppComponent: Operator,
					ComponentIdentifier: GetOperatorHelmIdentifier(operatorApplication.OperatorID, operatorHelm.Release),
					DataComponent:       dataComponent}
				if isVolumeSnapshotCompleted {
					if dataComponent.VolumeSnapshot != nil && reflect.DeepEqual(dataComponent.VolumeSnapshot.Status, v1.Completed) {
						backupDataComponents = append(backupDataComponents, appDs)
					}
				} else {
					backupDataComponents = append(backupDataComponents, appDs)
				}
			}
		}
	}

	return backupDataComponents, aggregateCount
}

func GetOperatorHelmIdentifier(operatorID, helmRelease string) string {
	return strings.Join([]string{operatorID, helmRelease}, "-")
}

func WaitForRestoreToDelete(acc *kube.Accessor, restoreName, ns string) {
	Eventually(func() bool {
		_, err := acc.GetRestore(restoreName, ns)
		if err != nil && apierrors.IsNotFound(err) {
			return true
		}
		if err == nil {
			_ = acc.DeleteRestore(types.NamespacedName{Name: restoreName, Namespace: ns})
		}
		return false
	}, "120s", "2s").Should(BeTrue())
}

func SetBackupPlanStatus(KubeAccessor *kube.Accessor, appName, namespace string, reqStatus crd.Status) error {
	var appCr *crd.BackupPlan
	var err error
	log.Infof("Updating %s status to %s", appName, reqStatus)
	Eventually(func() error {
		retErr := utilretry.RetryOnConflict(utilretry.DefaultRetry, func() error {

			appCr, err = KubeAccessor.GetBackupPlan(appName, namespace)
			if err != nil {
				log.Errorf(err.Error())
				return err
			}
			log.Infof("requested backupPlan status %v, actual status: %v",
				reqStatus, appCr.Status.Status)
			appCr.Status.Status = reqStatus
			err = KubeAccessor.StatusUpdate(appCr)
			if err != nil {
				log.Errorf("Failed to update application status:%+v", err)
				return err
			}

			return nil
		})
		appCr, err = KubeAccessor.GetBackupPlan(appName, namespace)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
		if appCr.Status.Status != reqStatus {
			log.Errorf("failed to update backupplan status reqStatus: %v, "+
				"actualStatus: %v", reqStatus, appCr.Status.Status)
			return fmt.Errorf("failed to update backupplan status")
		}
		log.Infof("Updated %s  requestedStatus %v to %s", appName,
			reqStatus, appCr.Status.Status)
		return retErr
	}, timeout, interval).ShouldNot(HaveOccurred())

	return nil
}
