package common

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal/shell"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"

	cryptorand "crypto/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
)

const (

	// k8s kinds
	DeploymentKind = "Deployment"
	PodKind        = "Pod"
	ServiceKind    = "Service"
	TargetKind     = "Target"
	BackupKind     = "Backup"
	RestoreKind    = "Restore"
	BackupplanKind = "BackupPlan"
	JobKind        = "Job"

	LicenseKey  = "LICENSE_KEY"
	LicenseName = "license-sample"

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

	KubeSystemNamespace = "kube-system"

	// CRD
	CrdVersionV1 = "v1"

	TrilioVaultGroup = "triliovault.trilio.io"

	// DataMover images
	AlpineImage   = "alpine:latest"
	joinSeparator = "\n---\n"
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

type UnstructuredResourceList unstructured.UnstructuredList

// Data Component Functions
type ApplicationDataSnapshot struct {
	AppComponent        SnapshotType
	ComponentIdentifier string
	DataComponent       v1.DataSnapshot
	Status              v1.Status
}

func (u *UnstructuredResourceList) GetChildrenForOwner(owner runtime.Object) UnstructuredResourceList {
	children := UnstructuredResourceList{}
	logger := ctrl.Log.WithName("UnstructResource Utility").WithName("GetChildrenForOwner")

	if owner == nil || len(u.Items) == 0 {
		return children
	}
	metaOwner, err := meta.Accessor(owner)
	if err != nil {
		logger.Error(err, "Error while converting the owner to meta accessor format")
		return children
	}
	matchUID := metaOwner.GetUID()
	for _, item := range u.Items {
		refs := item.GetOwnerReferences()
		for i := 0; i < len(refs); i++ {
			or := refs[i]
			if or.UID == matchUID {
				children.Items = append(children.Items, item)
			}
		}
	}

	return children
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

func CreateLicenseKey(projectPath string, args KeyGenArgs) (string, error) {
	argString := ""
	keygenFilePath := "internal/keygen.py"
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

func SetupLicense(ctx context.Context, cli client.Client, namespace string, projectRoot string) (*v1.License, error) {
	var (
		key             string
		isEnvKeyPresent bool
		err             error
	)
	log := logrus.WithFields(logrus.Fields{"namespace": namespace})
	key, isEnvKeyPresent = os.LookupEnv(LicenseKey)
	if !isEnvKeyPresent {
		log.Infof("License Key not found in env, creating new one")
		ns := &corev1.Namespace{}
		if err = cli.Get(ctx, types.NamespacedName{Name: KubeSystemNamespace}, ns); err != nil {
			return nil, err
		}
		args := KeyGenArgs{LicenseEdition: string(v1.FreeEdition), KubeUID: string(ns.GetUID()), LicensedFor: strconv.Itoa(20)}
		key, err = CreateLicenseKey(projectRoot, args)
		if err != nil {
			log.Errorf("Error while creating license key: %s", err.Error())
			return nil, err
		}
	}

	licenses := &v1.LicenseList{}
	if err = cli.List(ctx, licenses, client.InNamespace(namespace)); err != nil {
		log.Errorf("Error while listing license: %s", err.Error())
		return nil, err
	}
	if len(licenses.Items) != 0 {
		license := licenses.Items[0]
		log.Infof("Found existing license: %s", license.Name)
		if license.Status.Status != v1.LicenseActive {
			license.Spec.Key = key
			if err = cli.Update(ctx, &license); err != nil {
				log.Errorf("Error while updating key in existing license: %s, %s", license.Name, err.Error())
				return nil, err
			}
		}
		return &license, nil
	}
	license := &v1.License{ObjectMeta: metav1.ObjectMeta{Name: LicenseName, Namespace: namespace}, Spec: v1.LicenseSpec{Key: key}}
	err = cli.Create(ctx, license)
	if err != nil {
		log.Errorf("Error while creating new license: %s", err.Error())
		return nil, err
	}
	return license, nil
}
