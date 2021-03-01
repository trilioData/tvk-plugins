package backup

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type Mode string

const (
	OnDemand    Mode = "OnDemand"
	PolicyBased Mode = "PolicyBased"
)

// OrderingField specifies backup fields to order list of backups
type OrderingField string

const (
	CreationTimestampDesc   OrderingField = "-.metadata.creationTimestamp"
	CreationTimestampAsc    OrderingField = ".metadata.creationTimestamp"
	CompletionTimestampDesc OrderingField = "-.status.completionTimestamp"
	ExpirationTimestampDesc OrderingField = "-.status.expirationTimestamp"
	ExpirationTimestampAsc  OrderingField = ".status.expirationTimestamp"
	SizeDesc                OrderingField = "-.status.size"
	SizeAsc                 OrderingField = ".status.size"
)

// OrderingField specifies backup fields to order list of backups
type OrderingParamField string

const (
	CreationTimestamp   OrderingParamField = "creationTimestamp"
	ExpirationTimestamp OrderingParamField = "expirationTimestamp"
	Size                OrderingParamField = "size"
)

type List v1.BackupList

// CustomList type, Specifies list of Custom Backup
type CustomList []*Backup

// Backup Struct
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec           v1.BackupSpec   `json:"spec,omitempty"`
	Status         v1.BackupStatus `json:"status,omitempty"`
	GeneratedField *GeneratedField `json:"generatedField"`
}

// Info defines details Backups instance list
type Info struct {
	Metadata *common.ListMetadata `json:"metadata"`
	Summary  *Summary             `json:"summary"`
	Results  []*Backup            `json:"results"`
}

// GeneratedField Struct
type GeneratedField struct {
	BackupNamespace   string             `json:"backupNamespace"`
	ApplicationType   v1.ApplicationType `json:"applicationType,omitempty"`
	Target            *metav1.ObjectMeta `json:"target,omitempty"`
	InProgressRestore *v1.Restore        `json:"inProgressRestore,omitempty"`
	ScheduleType      Mode               `json:"scheduleType,omitempty"`
}

// QueryParameters specifies parameters to filter backups.
type QueryParameters string

const (
	BackupNamespaceQueryParam QueryParameters = "backupNamespace"
	BackupNameQueryParam      QueryParameters = "backupName"
	BackupPlanNameQueryParam  QueryParameters = "backupPlanName"
	TargetNameQueryParam      QueryParameters = "targetName"
	StatusQueryParam          QueryParameters = "status"
	TypeQueryParam            QueryParameters = "type"
	ScopeQueryParam           QueryParameters = "scope"
	ComponentTypeQueryParam   QueryParameters = "componentType"
	ModeQueryParam            QueryParameters = "mode"

	TimeRangeFieldQueryParam QueryParameters = "timeRangeField"
	TimeRangeValueQueryParam QueryParameters = "timeRangeValue"

	DateOfExpiry1QueryParam        QueryParameters = "dateOfExpiry1"
	DateOfExpiry2QueryParam        QueryParameters = "dateOfExpiry2"
	DateOfExpiryOperatorQueryParam QueryParameters = "dateOfExpiryOperator"

	OrderingQueryParam QueryParameters = "ordering"
)

// PathParameters specifies parameters to filter backups.
type PathParameters string

const (
	NamePathParam      PathParameters = "name"
	NamespacePathParam PathParameters = "namespace"
)

// GetRequestParams for storing user input for Single Backup Detail
type GetRequestParams struct {
	// Name specifies the name of the Backup for which details requested
	Name      string
	Namespace string
}

// CreateRequestParams is for storing user input to create resource
type CreateRequestParams struct {
	// The body params request for resource to be created
	Resource *v1.Backup
}

// TimeMatchOperator specifies the operator match filter
type TimeMatchOperator string

const (
	Equals  TimeMatchOperator = "Equals"
	Before  TimeMatchOperator = "Before"
	After   TimeMatchOperator = "After"
	Between TimeMatchOperator = "Between"
)

// AdvancedTimeFilter specifies the advanced filter to filter time ranges
type AdvancedTimeFilter struct {
	Time1 *metav1.Time
	Time2 *metav1.Time
	TimeMatchOperator
}

// ListRequestParams for storing user input for InfoFilter Object
type ListRequestParams struct {
	// Pagination Parameters
	Paginator *common.Paginator

	// The body params request for ApplicationFilter
	ApplicationFilter *common.ApplicationSelectorSearchFilter

	// Ordering Parameters
	Ordering *OrderingField

	// TargetName specifies, The backup's BackupPlan should have these TargetNames
	TargetName []string

	// BackupNamespaces will specifies if the Backup's BackupPlan have this backupNamespace
	BackupNamespace []string

	// BackupPlanName will specifies if the Backup have this backupPlan
	BackupPlanName []string

	// BackupName will specifies if the Backup have this name
	BackupName []string

	// Status specifies the status of Backup (InProgress, Available OR Failed)
	Status []v1.Status

	// Type specifies the type of an application Helm or Operator
	Type *v1.BackupType

	// ComponentType specifies the types of backups having type
	ComponentType []string

	// Mode specifies the mode of backup to filter for
	Mode *Mode

	// TimeRangeFilter specifies the timeRange on which objects will be filtered
	TimeRangeFilter common.TimeRangeFilter

	// ExpiryTimeFilter specifies the expiry time filter of backups
	ExpiryTimeFilter *AdvancedTimeFilter

	// Scope specifies the backup scope of a backup
	Scope *v1.ComponentScope
}

// Check Backups which have Scope is same as given in Request
func IsScopeMatches(backup *v1.Backup, namespaceBackupPlanMap map[string]map[string]v1.BackupPlan, scope *v1.ComponentScope) bool {
	if scope != nil {
		if string(backup.Status.BackupScope) != "" {
			if *scope != backup.Status.BackupScope {
				return false
			}
		} else {
			// backup scope isn't updated in status then check backupplan scope
			backupPlan := namespaceBackupPlanMap[backup.Spec.BackupPlan.Namespace][backup.Spec.BackupPlan.Name]
			if *scope != common.GetBackupPlanScope(&backupPlan) {
				return false
			}
		}
	}
	return true
}

// Convert and return list of runtime objects
func (list *List) ConvertToRuntimeObject() []runtime.Object {
	var objs []runtime.Object
	for idx := range list.Items {
		obj := list.Items[idx].DeepCopyObject()
		objs = append(objs, obj)
	}
	return objs
}

// Convert and return list of typed objects for runtime object
func (list *List) ConvertFromRuntimeObject(objs []runtime.Object, invert bool) []v1.Backup {
	lenObjs := len(objs)
	backupList := make([]v1.Backup, lenObjs)
	for idx := range objs {
		myobj := objs[idx].(*v1.Backup)
		if invert {
			backupList[lenObjs-1] = *myobj
			lenObjs--
		} else {
			backupList[idx] = *myobj
		}
	}
	return backupList
}

// Summary Struct
type Summary struct {
	Total  int     `json:"total"`
	Result *Result `json:"result"`
}

// SummaryResult Struct
type Result struct {
	Failed     int `json:"Failed"`
	Available  int `json:"Available"`
	InProgress int `json:"InProgress"`
}

// Sort sorts backup list based on provided field name
func (list *List) Sort(orderingField string) error {
	objects := list.ConvertToRuntimeObject()
	invert := false
	if string(orderingField[0]) == "-" {
		orderingField = orderingField[1:]
		invert = true
	}
	sortedObjects, err := common.SortingObjects(objects, orderingField)
	if err != nil {
		return err
	}
	sortedTypedObjects := list.ConvertFromRuntimeObject(sortedObjects, invert)
	list.Items = sortedTypedObjects
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Backup) DeepCopyInto(out *Backup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Target.
func (in *Backup) DeepCopy() *Backup {
	if in == nil {
		return nil
	}
	out := new(Backup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Backup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// Convert and return list of runtime objects
func (list CustomList) ConvertToRuntimeObject() []runtime.Object {
	var objs []runtime.Object
	for idx := range list {
		obj := list[idx].DeepCopyObject()
		objs = append(objs, obj)
	}
	return objs
}

// Convert and return list of typed objects for runtime object
func (list CustomList) ConvertFromRuntimeObject(objs []runtime.Object, invert bool) CustomList {
	lenObjs := len(objs)
	backupList := make(CustomList, lenObjs)
	for idx := range objs {
		myobj := objs[idx].(*Backup)
		if invert {
			backupList[lenObjs-1] = myobj
			lenObjs--
		} else {
			backupList[idx] = myobj
		}
	}
	return backupList
}

// Sort sorts backup list based on provided field name
func (list CustomList) Sort(orderingField string) (CustomList, error) {
	objects := list.ConvertToRuntimeObject()
	invert := false
	if string(orderingField[0]) == "-" {
		orderingField = orderingField[1:]
		invert = true
	}

	sortedObjects, err := common.SortingObjects(objects, orderingField)
	if err != nil {
		return nil, err
	}
	sortedTypedObjects := list.ConvertFromRuntimeObject(sortedObjects, invert)
	list = sortedTypedObjects
	return list, nil
}

// Function for generate listMetaData and Paginate results
func (list CustomList) paginate(paginator *common.Paginator) (*Info, error) {
	log := ctrl.Log.WithName("function").WithName("backup:paginate")

	err := paginator.Set(len(list))
	if err != nil {
		log.Error(err, "failed to calculate paginator detail")
		return nil, err
	}

	return &Info{
		Metadata: &common.ListMetadata{Next: paginator.Next, Total: paginator.ResultLen},
		Results:  list[paginator.From:paginator.To],
	}, nil
}

// Function to populate GeneratedField struct for Backup
func (in *Backup) populateGeneratedField(helper *DetailHelper) {
	scheduleType := OnDemand
	if in.GetAnnotations()[internal.ScheduleType] == string(v1.Periodic) {
		scheduleType = PolicyBased
	}
	in.GeneratedField = &GeneratedField{
		BackupNamespace:   helper.BackupPlan.GetNamespace(),
		ApplicationType:   helper.ApplicationType,
		Target:            &helper.Target.ObjectMeta,
		InProgressRestore: helper.InProgressResotre,
		ScheduleType:      scheduleType,
	}
}

func (in *Backup) UpdateDetails(ctx context.Context, cli client.Client) error {
	// Retrieving BackupPlan for Backup
	backupPlan, err := common.GetBackupPlanByName(ctx, cli, in.Spec.BackupPlan.Name, in.Spec.BackupPlan.Namespace)
	if err != nil {
		return err
	}

	// Getting DetailHelper
	detailHelper, listErr := getDetailHelper(ctx, cli, in, backupPlan)
	if listErr != nil {
		return listErr
	}

	// Populate Generated Field
	in.populateGeneratedField(detailHelper)
	return nil
}

// GetBackupPlanMap returns map from Backup Name to BackupPlan Name
func (list *List) GetBackupPlanMap() map[string]string {
	backupPlanMap := make(map[string]string, len(list.Items))
	for index := range list.Items {
		backup := list.Items[index]
		if backup.Spec.BackupPlan != nil {
			backupPlanMap[backup.Name] = backup.Spec.BackupPlan.Name
		}
	}
	return backupPlanMap
}

func (list *List) GetBackupPlanBackupMap(isAvailableOnly bool) map[string]v1.Backup {
	backupPlanBackupMap := make(map[string]v1.Backup)
	for index := range list.Items {
		backup := list.Items[index]
		backupPlanName := backup.Spec.BackupPlan.Name
		if _, exists := backupPlanBackupMap[backupPlanName]; !exists {
			if isAvailableOnly && backup.Status.Status != v1.Available {
				continue
			}
			backupPlanBackupMap[backupPlanName] = backup
		}
	}
	return backupPlanBackupMap
}

func (list *List) GetBackupPlanBackupListMap(isAvailableOnly bool) map[string][]v1.Backup {
	backupPlanBackupMap := make(map[string][]v1.Backup)
	for index := range list.Items {
		backup := list.Items[index]
		backupPlanName := backup.Spec.BackupPlan.Name
		if _, exists := backupPlanBackupMap[backupPlanName]; !exists {
			if isAvailableOnly && backup.Status.Status != v1.Available {
				continue
			}
			backupPlanBackupMap[backupPlanName] = append(backupPlanBackupMap[backupPlanName], backup)
		}
	}
	return backupPlanBackupMap
}
