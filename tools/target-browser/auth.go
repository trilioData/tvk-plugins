package targetbrowser

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	v1 "k8s.io/api/networking/v1"

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
	var err error

	targetBrowserConfig.KubeConfig, err = internal.NewConfigFromCommandline(targetBrowserConfig.KubeConfig)
	if err != nil {
		return nil, err
	}

	acc, err := internal.NewAccessor(targetBrowserConfig.KubeConfig, targetBrowserConfig.Scheme)
	if err != nil {
		return nil, err
	}

	cl := acc.GetRuntimeClient()

	target, err := targetBrowserConfig.validateTarget(ctx, cl)
	if err != nil {
		return nil, err
	}

	isIngressNetworkingV1Resource := internal.CheckIfAPIVersionKindAvailable(acc.GetDiscoveryClient(),
		v1.SchemeGroupVersion.WithKind(internal.IngressKind))

	tvkHost, targetBrowserPath, err := getTvkHostAndTargetBrowserAPIPath(ctx, cl, target, isIngressNetworkingV1Resource)
	if err != nil {
		return nil, err
	}
	if targetBrowserPath == "" {
		return nil, fmt.Errorf("targetBrowserPath could not retrieved for"+
			" target %s namespace %s", targetBrowserConfig.TargetName, targetBrowserConfig.TargetNamespace)
	}

	nodePortHTTP, nodePortHTTPS, svcType, tvkHostIP, err := getNodePortAndServiceTypeAndTvkHostIP(ctx, cl, target)
	if err != nil {
		return nil, err
	}
	if tvkHost == "" {
		tvkHost = tvkHostIP
	}
	tvkURL, err := url.Parse(tvkHost)
	if err != nil {
		return nil, err
	}
	if targetBrowserConfig.UseHTTPS {
		tvkURL.Scheme = internal.HTTPSscheme
		if svcType == internal.ServiceTypeNodePort {
			tvkURL.Path = fmt.Sprintf("%s:%s", tvkHost, nodePortHTTPS)
		}
	} else {
		tvkURL.Scheme = internal.HTTPscheme
		if svcType == internal.ServiceTypeNodePort {
			tvkURL.Path = fmt.Sprintf("%s:%s", tvkHost, nodePortHTTP)
		}
	}

	tvkHost = tvkURL.String()

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
