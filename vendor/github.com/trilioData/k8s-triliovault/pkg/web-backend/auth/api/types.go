package api

import (
	"time"

	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/trilioData/k8s-triliovault/pkg/web-backend/resource/tvkconfig"
)

const (
	// Resource information that are used as encryption key storage. Can be accessible by multiple replicas and target browser instance.
	EncryptionKeyHolderName = "k8s-triliovault-web-backend-key-holder"
)

// AuthManager is used for user authentication management.
type AuthManager interface {
	// Login authenticates user based on provided LoginSpec and returns AuthResponse. AuthResponse contains
	// generated token and list of non-critical errors such as 'Failed authentication'.
	Login(*LoginSpec) (*AuthResponse, error)
	// Refresh takes valid token that hasn't expired yet and returns a new one with expiration time set to TokenTTL. In
	// case provided token has expired, token expiration error is returned.
	Refresh(string) (string, error)
}

// TokenManager is responsible for generating and decrypting tokens used for authorization. Authorization is handled
// by K8S apiserver. Token contains AuthInfo structure used to create K8S api client.
type TokenManager interface {
	// Generate secure token based on AuthInfo structure and save it tokens' payload.
	Generate(*api.AuthInfo) (string, error)
	// Decrypt generated token and return AuthInfo structure that will be used for K8S api client creation.
	Decrypt(string) (*api.AuthInfo, error)
	// Refresh returns refreshed token based on provided token. In case provided token has expired, token expiration
	// error is returned.
	Refresh(string) (string, error)
	// SetTokenTTL sets expiration time (in seconds) of generated tokens.
	SetTokenTTL(time.Duration)
}

// Authenticator represents authentication methods supported by Web backend. Currently supported types are:
//    - Token based - Any bearer token accepted by apiserver
//    - Kubeconfig based - Authenticates user based on kubeconfig file. Only token/basic modes are supported within
// 		the kubeconfig file.
type Authenticator interface {
	// GetAuthInfo returns filled AuthInfo structure that can be used for K8S api client creation.
	GetAuthInfo() (api.AuthInfo, error)
}

// LoginSpec is extracted from request coming from Web UI during login request. It contains all the
// information required to authenticate user.
type LoginSpec struct {
	// Token is the bearer token for authentication to the kubernetes cluster.
	Token string `json:"token,omitempty"`
	// KubeConfig is the content of users' kubeconfig file. It will be parsed and auth data will be extracted.
	// Kubeconfig can not contain any paths. All data has to be provided within the file.
	KubeConfig string `json:"kubeconfig,omitempty"`
}

// AuthResponse is returned from our backend as a response for login/refresh requests. It contains generated JWEToken
// and a list of non-critical errors such as 'Failed authentication'.
type AuthResponse struct {
	// JWEToken is a token generated during login request that contains AuthInfo data in the payload.
	JWEToken string `json:"jweToken"`
	// Errors are a list of non-critical errors that happened during login request.
	Errors []error `json:"errors"`
	// TvkConfigList is the list of TVK configs for a auth user and his user information.
	TvkConfigList tvkconfig.TvkConfigList `json:"tvkConfigList"`
	// SessionInactivityTTL is the max time web can stay logged in without activity
	SessionInactivityTTL float64 `json:"sessionInactivityTTL"`
}

// TokenRefreshSpec contains token that is required by token refresh operation.
type TokenRefreshSpec struct {
	// JWEToken is a token generated during login request that contains AuthInfo data in the payload.
	JWEToken string `json:"jweToken"`
}
