package tvkconfig

import (
	"context"
	"encoding/json"

	authenticationv1 "k8s.io/api/authentication/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/k8s-triliovault/pkg/web-backend/errors"
)

// AuthManager is used for user authentication management.
type ConfigHandler interface {
	Create(tvkconfig *TvkConfig) (*TvkConfig, error)
	List() (tvkConfigList *TvkConfigList, err error)
	Put(name string, tvkconfig *TvkConfig) (*TvkConfig, error)
	Remove(name string) (err error)
}

// handler performs various operations on tvk config.
type handler struct {
	client.Client
	*authenticationv1.UserInfo
}

func (handler *handler) Create(tvkconfig *TvkConfig) (*TvkConfig, error) {
	log := ctrl.Log.WithName("function").WithName("ConfigHandler:Create")

	log.Info("Getting secret for storing tvk config")
	secret, err := getTVKConfigHolerSecret(handler.Client)
	if err != nil {
		log.Error(err, "error while getting secret")
		return nil, err
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	userNameHash := getHashString(handler.Username)

	// Update tvk config in a existing or new map
	tvkConfigMap := &Map{}
	if value, ok := secret.Data[userNameHash]; ok {
		err = json.Unmarshal(value, tvkConfigMap)
		if err != nil {
			return nil, err
		}
		tvkConfigMap.UserInfo = *handler.UserInfo
		if _, configExists := tvkConfigMap.TvkConfig[tvkconfig.Name]; configExists {
			return nil, errors.NewBadRequest("TVK config already exists with provided name")
		}
		tvkConfigMap.TvkConfig[tvkconfig.Name] = *tvkconfig
	} else {
		log.Info("Creating new config map for user", "user", handler.Username)
		tvkConfigMap.UserInfo = *handler.UserInfo
		tvkConfigMap.TvkConfig = make(map[string]TvkConfig)
		tvkConfigMap.TvkConfig[tvkconfig.Name] = *tvkconfig
	}

	tvkConfigBytes, err := tvkConfigMap.marshal()
	if err != nil {
		log.Error(err, "error while marshaling tvk config map")
		return nil, err
	}
	secret.Data[userNameHash] = tvkConfigBytes
	log.Info("Updating tvk config map in secret")
	err = handler.Update(context.TODO(), secret)
	if err != nil {
		log.Error(err, "error while updating secret")
		return nil, err
	}
	return tvkconfig, nil
}

func (handler *handler) List() (tvkConfigList *TvkConfigList, err error) {
	log := ctrl.Log.WithName("function").WithName("ConfigHandler:List")

	secret, err := getTVKConfigHolerSecret(handler.Client)
	if err != nil {
		log.Error(err, "error while getting secret")
		return tvkConfigList, err
	}

	userNameHash := getHashString(handler.Username)
	if value, ok := secret.Data[userNameHash]; ok {
		tvkConfigMap := &Map{}
		err = json.Unmarshal(value, tvkConfigMap)
		if err != nil {
			log.Error(err, "error while unmarshalling config map")
			return tvkConfigList, err
		}
		return tvkConfigMap.list(), nil
	}

	tvkConfigList = &TvkConfigList{UserInfo: *handler.UserInfo}
	return tvkConfigList, nil
}

func (handler *handler) Put(name string, tvkconfig *TvkConfig) (*TvkConfig, error) {
	log := ctrl.Log.WithName("function").WithName("ConfigHandler:Put")

	secret, err := getTVKConfigHolerSecret(handler.Client)
	if err != nil {
		log.Error(err, "error while getting secret")
		return nil, err
	}

	userNameHash := getHashString(handler.Username)

	// If config exists for given name of a user then update
	tvkConfigMap := &Map{}
	if value, ok := secret.Data[userNameHash]; ok {
		err = json.Unmarshal(value, tvkConfigMap)
		if err != nil {
			return nil, err
		}
		if _, exists := tvkConfigMap.TvkConfig[name]; exists {
			tvkConfigMap.TvkConfig[name] = *tvkconfig
		} else {
			return nil, errors.NewNotFound("Tvk config found for user")
		}

	} else {
		return nil, errors.NewNotFound("User Config map not found in secret")
	}

	log.Info("Update tvkConfig in the secret")
	tvkConfigBytes, err := tvkConfigMap.marshal()
	if err != nil {
		log.Error(err, "error while marshaling config map")
		return nil, err
	}
	secret.Data[userNameHash] = tvkConfigBytes
	err = handler.Update(context.TODO(), secret)
	if err != nil {
		log.Error(err, "error while updating secret")
		return nil, err
	}
	return tvkconfig, nil
}

func (handler *handler) Remove(name string) (err error) {
	log := ctrl.Log.WithName("function").WithName("ConfigHandler:Delete")

	secret, err := getTVKConfigHolerSecret(handler.Client)
	if err != nil {
		log.Error(err, "error while getting secret")
		return err
	}

	userNameHash := getHashString(handler.Username)

	if value, ok := secret.Data[userNameHash]; ok {
		tvkConfigMap := &Map{}
		err = json.Unmarshal(value, tvkConfigMap)
		if err != nil {
			log.Error(err, "error while unmarshalling config map")
			return err
		}
		if _, present := tvkConfigMap.TvkConfig[name]; present {
			delete(tvkConfigMap.TvkConfig, name)
			tvkConfigBytes, err := tvkConfigMap.marshal()
			if err != nil {
				log.Error(err, "error while marshaling config map")
				return err
			}
			secret.Data[userNameHash] = tvkConfigBytes
			return handler.Update(context.TODO(), secret)
		}
	}
	return errors.NewNotFound("TVK Config not found")
}

func NewTVKConfigHandler(managerClient, k8sClient client.Client) (ConfigHandler, error) {
	log := ctrl.Log.WithName("function").WithName("NewTVKConfigHandler")
	authInfo, err := GetAuthUserInfo(k8sClient)
	if err != nil {
		log.Error(err, "error while getting auth info")
		return nil, err
	}

	return &handler{
		Client:   managerClient,
		UserInfo: authInfo,
	}, nil
}
