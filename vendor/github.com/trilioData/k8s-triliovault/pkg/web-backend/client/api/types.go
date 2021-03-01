// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"github.com/emicklei/go-restful"
	v1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authApi "github.com/trilioData/k8s-triliovault/pkg/web-backend/auth/api"
)

// ClientManager is responsible for initializing and creating clients to communicate with
// kubernetes apiserver on demand.
type ClientManager interface {
	DiscoveryClient() *discovery.DiscoveryClient
	CachedDiscoveryClient() discovery.CachedDiscoveryInterface
	ManagerClient() client.Client
	InClusterConfig() *rest.Config
	Scheme() *runtime.Scheme
	Client(req *restful.Request) (client.Client, error)
	AuthClient(authInfo *api.AuthInfo) (client.Client, error)
	CanI(req *restful.Request, ssar *v1.SelfSubjectAccessReview) bool
	Config(req *restful.Request) (*rest.Config, error)
	ClientCmdConfig(req *restful.Request) (clientcmd.ClientConfig, error)
	HasAccess(authInfo *api.AuthInfo) error
	HasAuthorization(authInfo *api.AuthInfo) error
	SetTokenManager(manager authApi.TokenManager)
}

// CanIResponse is used to as response to check whether or not user is allowed to access given endpoint.
type CanIResponse struct {
	Allowed bool `json:"allowed"`
}
