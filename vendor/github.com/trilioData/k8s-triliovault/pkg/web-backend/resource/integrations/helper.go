package integrations

import (
	"github.com/emicklei/go-restful"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/client/api"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	status = "status"
)

func GetIntegrationListRequest(cManager api.ClientManager, request *restful.Request) ListRequest {
	clientInterface := request.Attribute(common.AuthClient)
	k8sClient := clientInterface.(client.Client)

	// Init remaining params
	requestParams := ListRequestParams{}
	requestParams.Paginator, _ = common.InitRequestParams(request)
	requestParams.Status = request.QueryParameter(status)

	// initialize  ListRequest object with request params
	return ListRequest{
		AuthClient:        k8sClient,
		ClientManager:     cManager,
		ListRequestParams: requestParams,
	}
}
