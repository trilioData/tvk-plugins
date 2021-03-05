package backup

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	clientapi "github.com/trilioData/k8s-triliovault/pkg/web-backend/client/api"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type ListRequest struct {
	// ClientManager is the client to perform operation on kubernetes resources.
	ClientManager clientapi.ClientManager

	// ListRequestParams specifies the parameters to filter backups.
	ListRequestParams
}

type GetRequest struct {
	// ClientManager is the client to perform operation on kubernetes resources.
	ClientManager clientapi.ClientManager

	// GetRequestParams specifies the parameters for which the data is requested
	GetRequestParams
}

type WriteRequest struct {
	// AuthClient to perform Create Operations for the resources
	AuthClient client.Client

	// ClientManager is the client to perform operation on kubernetes resources.
	ClientManager clientapi.ClientManager

	// PostRequestParams specifies the parameters for which the Resource is to be created
	CreateRequestParams
}

// TO-DO: No Need for separate DeleteRequest struct
type DeleteRequest struct {
	// AuthClient to perform Create Operations for the resources
	AuthClient client.Client

	// ClientManager is the client to perform operation on kubernetes resources.
	ClientManager clientapi.ClientManager

	// PostRequestParams specifies the parameters for which the Resource is to be created
	common.DeleteRequestParams
}

func (request *GetRequest) Get() (*Backup, error) {
	log := ctrl.Log.WithName("function").WithName("BackupGetRequest:Get")
	cli := request.ClientManager.ManagerClient()
	ctx := context.Background()
	var backupPlan *v1.BackupPlan

	// Retrieve Backup
	backup, err := GetBackupByName(ctx, cli, request.Name, request.Namespace)
	if err != nil {
		log.Error(err, "error while getting backup object from api cache")
		return nil, err
	}

	// Retrieving BackupPlan for Backup
	backupPlan, err = common.GetBackupPlanByName(ctx, cli, backup.Spec.BackupPlan.Name, backup.Spec.BackupPlan.Namespace)
	if err != nil {
		return nil, err
	}

	// Getting DetailHelper
	detailHelper, listErr := getDetailHelper(ctx, cli, backup, backupPlan)
	if listErr != nil {
		log.Error(listErr, "error while getting detailHelper")
		return nil, listErr
	}

	// Populate Generated Field
	backup.populateGeneratedField(detailHelper)

	return backup, nil
}

// nolint:gocyclo // This function is giving gocyclo because it needs to handle various filter conditions
func (request *ListRequest) List() (*Info, error) {
	log := ctrl.Log.WithName("function").WithName("BackupListRequest:List")
	cli := request.ClientManager.ManagerClient()
	ctx := context.Background()
	summary := &Summary{Result: &Result{}} // Summary Object

	// Retrieve backup list
	backupList, err := GetBackupList(ctx, cli)
	if err != nil {
		log.Error(err, "error while getting backup list")
		return nil, err
	}

	// Retrieve backupPlan list
	backupPlanList, err := common.GetBackupPlanList(ctx, cli)
	if err != nil {
		log.Error(err, "error while getting backupPlan list")
		return nil, err
	}
	namespaceBackupPlanMap := common.GetNamespaceBackupPlanMap(backupPlanList)

	// Making BackupList of custom struct from API struct
	var bl CustomList
	for idx := range backupList.Items {
		var backupPlan v1.BackupPlan
		backup := CopyDataFrom(&backupList.Items[idx]) // Backup Object

		// Checking if Backup lies in Time Range Requested
		if !request.TimeRangeFilter.IsEmpty() && !common.IsTimestampInRange(backup.CreationTimestamp, request.TimeRangeFilter) {
			continue
		}

		// Retrieving BackupPlan for Backup
		backupPlan = namespaceBackupPlanMap[backup.Spec.BackupPlan.Namespace][backup.Spec.BackupPlan.Name]

		if !request.Filter.ApplicationFilter.IsEmpty() &&
			!common.IsBackupPlanApplicationFilter(namespaceBackupPlanMap, &backupPlan, &request.Filter.ApplicationFilter) {
			continue
		}

		// Getting DetailHelper
		detailHelper, listErr := getDetailHelper(ctx, cli, backup, &backupPlan)
		if listErr != nil {
			log.Error(listErr, "error while getting ListHelper")
			return nil, listErr
		}

		// Filter Backups which have BackupNamespace in List of BackupNamespaces given in request
		if len(request.BackupNamespace) != 0 && !internal.ContainsString(request.BackupNamespace,
			backup.GetNamespace()) {
			continue
		}

		// Filter Backups which have TargetName in List of TargetNames given in request
		if len(request.Filter.Targets) != 0 && backup.Status.Stats != nil && backup.Status.Stats.Target != nil &&
			!common.IsNamespacedNameExists(request.Filter.Targets, internal.GetObjectRefNamespacedName(backup.Status.Stats.Target)) {
			continue
		}

		// Filter Backups which have BackupPlan Name in List of BackupPlan Names given in request
		if len(request.Filter.BackupPlans) != 0 && !common.IsNamespacedNameExists(request.Filter.BackupPlans,
			internal.GetObjectRefNamespacedName(backup.Spec.BackupPlan)) {
			continue
		}

		// Filter Backups
		if len(request.Filter.Backups) != 0 && !common.IsNamespacedNameExists(request.Filter.Backups,
			internal.GetObjectNamespacedName(&backupList.Items[idx])) {
			continue
		}

		// Filter Backups which have Type is same as given in Request
		if request.Type != nil && *request.Type != backup.Spec.Type {
			continue
		}

		// Filter Backups which have Scope is same as given in Request
		if !IsScopeMatches(&backupList.Items[idx], namespaceBackupPlanMap, request.Scope) {
			continue
		}

		// Populate Generated Field
		backup.populateGeneratedField(detailHelper)

		// Filter based on mode/scheduleType
		if request.Mode != nil && *request.Mode != backup.GeneratedField.ScheduleType {
			continue
		}

		// Filter based on application type
		if len(request.ComponentType) != 0 && !internal.ContainsString(request.ComponentType,
			string(backup.GeneratedField.ApplicationType)) {
			continue
		}

		if request.ExpiryTimeFilter != nil {
			filter := request.ExpiryTimeFilter
			if backup.Status.ExpirationTimestamp == nil {
				continue
			}
			expirationTimestamp := backup.Status.ExpirationTimestamp
			expirationDate := time.Date(expirationTimestamp.Year(), expirationTimestamp.Month(),
				expirationTimestamp.Day(), 0, 0, 0, 0, time.UTC)
			switch filter.TimeMatchOperator {
			case Equals:
				if !filter.Time1.Time.Equal(expirationDate) {
					continue
				}
			case Before:
				if !expirationDate.Before(filter.Time1.Time) {
					continue
				}
			case After:
				t2 := filter.Time1.Time.AddDate(0, 0, 1).Add(-1 * time.Second)
				if !expirationDate.After(t2) {
					continue
				}
			case Between:
				if !(expirationDate.After(filter.Time1.Time) && expirationDate.Before(filter.Time2.Time)) {
					continue
				}
			}
		}

		// Populate Summary Object
		summary.UpdateSummary(backup.Status.Status)

		// Filter Backups which have Status in List of Status given in request
		if len(request.Status) != 0 && !common.ContainsStatus(request.Status, backup.Status.Status) {
			continue
		}
		bl = append(bl, backup)
	}

	// Perform operation, Summary, pagination and return final result
	if request.Ordering != nil {
		bl, err = bl.Sort(string(*request.Ordering))
		if err != nil {
			log.Error(err, "error while sorting backup list")
			return nil, err
		}
	}

	result, err := bl.paginate(request.Paginator)
	if err != nil {
		log.Error(err, "error while paginating backup list")
		return nil, err
	}

	// Append Summary Object
	result.Summary = summary

	return result, nil
}

func (request WriteRequest) Create() (backup Backup, err error) {
	log := ctrl.Log.WithName("function").WithName("BackupCreateRequest:Create")
	cli := request.AuthClient
	ctx := context.Background()
	resource := request.Resource
	err = cli.Create(ctx, resource)
	if err != nil {
		log.Error(err, "failed to create the Backup resource")
		return backup, err
	}
	backup = *CopyDataFrom(resource)
	return backup, nil
}

func (request WriteRequest) Update() (backup Backup, err error) {
	log := ctrl.Log.WithName("function").WithName("BackupUpdateRequest:Update")
	cliW := request.AuthClient
	cliR := request.ClientManager.ManagerClient()
	ctx := context.Background()
	resource := request.Resource
	currObject, err := common.GetBackupByName(ctx, cliR, resource.Name, resource.Namespace)
	if err != nil {
		log.Error(err, "failed to get the Backup resource")
		return backup, err
	}
	// TO-DO: Repetitive need to create common
	// Setting up ResourceVersion if resourceVersion not given
	if resource.ResourceVersion == "" {
		resource.ResourceVersion = currObject.ResourceVersion
	}
	// Handling APIVersion & Kind if none given
	if resource.APIVersion == "" || resource.Kind == "" {
		resource.APIVersion = currObject.APIVersion
		resource.Kind = currObject.Kind
	}
	err = cliW.Patch(ctx, resource, client.MergeFrom(currObject))
	if err != nil {
		log.Error(err, "failed to update the Backup resource")
		return backup, err
	}
	backup = *CopyDataFrom(currObject)

	// Retrieving BackupPlan for Backup
	backupPlan, err := common.GetBackupPlanByName(ctx, cliR, backup.Spec.BackupPlan.Name, backup.Spec.BackupPlan.Namespace)
	if err != nil {
		log.Error(err, "error while getting BackupPlanList")
	}

	// Getting DetailHelper
	detailHelper, listErr := getDetailHelper(ctx, cliR, &backup, backupPlan)
	if listErr != nil {
		log.Error(listErr, "error while getting detailHelper")
	}

	// Populate Generated Field
	backup.populateGeneratedField(detailHelper)
	return backup, nil
}

// TO-DO: No Need for separate Delete Function
func (request DeleteRequest) Delete() error {
	log := ctrl.Log.WithName("function").WithName("BackupDeleteRequest:Delete")
	cliW := request.AuthClient
	ctx := context.Background()
	currObject := &v1.Backup{
		ObjectMeta: common.GetObjectMeta(request.Name, request.Namespace),
	}
	if err := cliW.Delete(ctx, currObject); err != nil {
		log.Error(err, "failed to delete the Backup resource")
		return err
	}
	return nil
}
