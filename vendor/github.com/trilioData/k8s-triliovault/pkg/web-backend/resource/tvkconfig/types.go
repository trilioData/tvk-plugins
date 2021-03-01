package tvkconfig

import (
	"encoding/json"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// PolicyNameForAuthCheck is the name of policy used to retrieve user information from webhook-server
	PolicyNameForAuthCheck = "auth-check-policy"

	// TVKConfigHolderName secret holds the information regarding multiple TVK Configs per auth user.
	TVKConfigHolderName = "k8s-triliovault-web-backend-tvkconfig-holder"

	// Name is the path param used to retrieve config of a particular name.
	Name = "name"

	// Primary is the name of primary TVK config.
	Primary = "primary"
)

// TvkConfig defines details of Secondary TrilioVault instance for a Kubernetes User.
type TvkConfig struct {
	Name       string `json:"name"`
	KubeConfig string `json:"kubeconfig"`
	ServiceURL string `json:"serviceURL"`
}

// TvkConfigMap defines the map of TVK configs on name for a user, stored as a secret key
type Map struct {
	authenticationv1.UserInfo `json:"userInfo"`
	TvkConfig                 map[string]TvkConfig `json:"configMap"`
}

// TvkConfigList defines the list of TVK configs of a user.
type TvkConfigList struct {
	authenticationv1.UserInfo `json:"userInfo"`
	Results                   []TvkConfig `json:"results"`
}

func (cfgMap *TvkConfig) Validate() map[string][]string {
	errorMap := make(map[string][]string)

	// Validate name
	if cfgMap.Name == "" {
		errorMap["name"] = []string{"Name can't be empty"}
	} else {
		nameErrors := validation.IsValidLabelValue(cfgMap.Name)
		if len(nameErrors) > 0 {
			errorMap["name"] = nameErrors
		} else if cfgMap.Name == Primary {
			errorMap["name"] = []string{"The name of tvk config can't be primary"}
		}
	}

	// Validate kubeconfig
	kubeConfig, err := clientcmd.Load([]byte(cfgMap.KubeConfig))
	if err != nil {
		errorMap["kubeconfig"] = []string{"Unable to decode kubeconfig"}
	} else {
		vErr := clientcmd.Validate(*kubeConfig)
		if vErr != nil {
			errorMap["kubeconfig"] = []string{vErr.Error()}
		}
	}

	// Validate ServiceURL
	if !isValidURL(cfgMap.ServiceURL) {
		errorMap["serviceURL"] = []string{"Service URL is not valid"}
	}

	return errorMap
}

// list converts map of configs into list
func (cfgMap *Map) list() *TvkConfigList {
	cfgList := &TvkConfigList{}
	cfgList.UserInfo = cfgMap.UserInfo
	for _, cfg := range cfgMap.TvkConfig {
		cfgList.Results = append(cfgList.Results, cfg)
	}
	return cfgList
}

// marshal marshals config map
func (cfgMap *Map) marshal() ([]byte, error) {
	return json.Marshal(cfgMap)
}
