package integrations

import (
	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/controller-runtime/pkg/client"

	clientapi "github.com/trilioData/k8s-triliovault/pkg/web-backend/client/api"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
)

type ListRequestParams struct {
	Paginator       *common.Paginator
	Type            []Type
	Status          *string
	OrderingField   *OrderingParam
	TimeRangeFilter *common.TimeRangeFilter
	Namespace       *string
}

type ListRequest struct {
	// AuthClient to perform Create Operations for the resources
	AuthClient client.Client

	// ClientManager is the client to perform operation on kubernetes resources.
	ClientManager clientapi.ClientManager

	// ListRequestParams specifies the parameters to filter.
	ListRequestParams
}

func (request *ListRequest) BackupList() (BackupList, error) {

	veleroIntegration, err := NewVeleroIntegration(request.ClientManager, request.AuthClient)

	if err != nil {
		return BackupList{}, err
	}

	var filteredList BackupList
	filteredList, err = veleroIntegration.BackupList(&request.ListRequestParams)
	if err != nil {
		log.Error(err, "error while getting backup list")
		return BackupList{}, err
	}

	paginatedFilteredList, err := filteredList.paginate(request.Paginator)
	if err != nil {
		return BackupList{}, err
	}

	return paginatedFilteredList, nil
}

func (request *ListRequest) RestoreList() (RestoreList, error) {

	veleroIntegration, err := NewVeleroIntegration(request.ClientManager, request.AuthClient)

	if err != nil {
		return RestoreList{}, err
	}

	var filteredList RestoreList
	filteredList, err = veleroIntegration.RestoreList(&request.ListRequestParams)
	if err != nil {
		log.Error(err, "error while getting restore list")
		return RestoreList{}, err
	}

	paginatedFilteredList, err := filteredList.paginate(request.Paginator)
	if err != nil {
		return RestoreList{}, err
	}

	return paginatedFilteredList, nil
}

func (request *ListRequest) TargetList() (TargetList, error) {

	veleroIntegration, err := NewVeleroIntegration(request.ClientManager, request.AuthClient)

	if err != nil {
		return TargetList{}, err
	}

	var filteredList TargetList
	filteredList, err = veleroIntegration.TargetList(&request.ListRequestParams)
	if err != nil {
		log.Error(err, "error while getting target list")
		return TargetList{}, err
	}

	paginatedFilteredList, err := filteredList.paginate(request.Paginator)
	if err != nil {
		return TargetList{}, err
	}

	return paginatedFilteredList, nil
}
