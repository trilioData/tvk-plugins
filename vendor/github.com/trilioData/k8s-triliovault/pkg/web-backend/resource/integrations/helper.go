package integrations

import (
	"github.com/emicklei/go-restful"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/client/api"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetIntegrationListRequest(cManager api.ClientManager, request *restful.Request, listRequestParams *ListRequestParams) ListRequest {
	clientInterface := request.Attribute(common.AuthClient)
	k8sClient := clientInterface.(client.Client)

	// Init paginator params
	listRequestParams.Paginator, _ = common.InitRequestParams(request)

	// initialize  ListRequest object with request params
	return ListRequest{
		AuthClient:        k8sClient,
		ClientManager:     cManager,
		ListRequestParams: *listRequestParams,
	}
}
