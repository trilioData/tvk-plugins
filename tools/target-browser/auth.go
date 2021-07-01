package targetbrowser

import (
	"context"
	"net/http"

	"github.com/trilioData/tvk-plugins/internal"
)

// AuthInfo contains http client, JWT, TvkHost, TargetBrowserPath for further use in sub-commands of getCmd
type AuthInfo struct {
	Client                          *http.Client
	UseHTTPS                        bool
	JWT, TvkHost, TargetBrowserPath string
}

// Authenticate generates AuthInfo which is required for further operations which are sub-commands of getCmd[backup,
// backupPlan, metadata].
func (targetBrowserConfig *Config) Authenticate(ctx context.Context) (*AuthInfo, error) {

	acc, err := internal.NewEnv(targetBrowserConfig.KubeConfig, targetBrowserConfig.Scheme)
	if err != nil {
		return nil, err
	}

	cl := acc.GetRuntimeClient()

	target, err := targetBrowserConfig.validateTarget(ctx, cl)
	if err != nil {
		return nil, err
	}

	tvkHost, targetBrowserPath, err := targetBrowserConfig.getTvkHostAndTargetBrowserAPIPath(ctx, cl, target)
	if err != nil {
		return nil, err
	}

	jweToken, httpClient, err := targetBrowserConfig.Login(tvkHost)
	if err != nil {
		return nil, err
	}

	return &AuthInfo{
		UseHTTPS:          targetBrowserConfig.UseHTTPS,
		Client:            httpClient,
		JWT:               jweToken,
		TvkHost:           tvkHost,
		TargetBrowserPath: targetBrowserPath,
	}, nil
}
