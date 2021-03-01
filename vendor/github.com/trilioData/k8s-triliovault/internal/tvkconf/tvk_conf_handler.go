package tvkconf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"github.com/trilioData/k8s-triliovault/internal/kube"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
)

func getAccessor() (*kube.Accessor, error) {
	// Create client to access kubernetes resources
	scheme := runtime.NewScheme()
	utilruntime.Must(clientGoScheme.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))

	var err error
	var acc *kube.Accessor
	acc, err = kube.NewEnv(scheme)
	if err != nil {
		log.Errorf("Failed to setup kube accessor with error: %s", err.Error())
		return nil, fmt.Errorf("failed to setup kube accessor")
	}
	return acc, nil
}

func validateResourceRequirements(resReq corev1.ResourceRequirements) error {

	if errList := validation.ValidateResourceRequirements(&resReq, field.NewPath("")); len(errList) > 0 {
		log.Errorf("resource requirements validaition failed. using default limits")
		return fmt.Errorf("resource requirements validaition failed. " +
			"using default resource requirements")
	}
	return nil
}

func GetDefaultTVKResourceConf() (map[string]string, error) {

	metaLimit, err := json.Marshal(getDefaultResourceLimits(internal.NonDMJobResource))
	if err != nil {
		return nil, err
	}

	dataLimit, err := json.Marshal(getDefaultResourceLimits(internal.DMJobResource))
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		internal.MetaMoverJobLimits: string(metaLimit),
		internal.DataMoverJobLimits: string(dataLimit),
	}
	return data, nil
}

func getDefaultResourceLimits(resourceType string) corev1.ResourceList {
	var defaultMem, defaultCPU resource.Quantity
	switch resourceType {
	case internal.DMJobResource:
		// defaults
		defaultMem, _ = resource.ParseQuantity(internal.DmLimitMem) // 1536MiB
		defaultCPU, _ = resource.ParseQuantity(internal.DmLimitCPU) // 1000m
	case internal.NonDMJobResource:
		// defaults
		defaultMem, _ = resource.ParseQuantity(internal.NonDmLimitMem) // 512 MiB
		defaultCPU, _ = resource.ParseQuantity(internal.NonDmLimitCPU) // 500m

	}
	return corev1.ResourceList{
		corev1.ResourceMemory: defaultMem,
		corev1.ResourceCPU:    defaultCPU,
	}
}

func getDefaultResourceRequests(resourceType string) corev1.ResourceList {
	var reqMem, reqCPU resource.Quantity
	switch resourceType {
	case internal.DMJobResource:
		// defaults
		reqCPU, _ = resource.ParseQuantity(internal.DmRequestCPU) // 0.1 i.e. 100 milliCore/milliCPU
		reqMem, _ = resource.ParseQuantity(internal.DmRequestMem) // 800 MiB
	case internal.NonDMJobResource:
		// defaults
		reqCPU, _ = resource.ParseQuantity(internal.NonDmRequestCPU) // 0.01 i.e. 10 milliCore/milliCPU
		reqMem, _ = resource.ParseQuantity(internal.NonDmRequestMem) // 10 MiB

	}
	return corev1.ResourceList{
		corev1.ResourceMemory: reqMem,
		corev1.ResourceCPU:    reqCPU,
	}
}

func GetAndValidateResourceRequirements(resourceType string) (corev1.ResourceList, error) {

	// TODO : in case of ocp we've required to retrieve tvk configMap from an API call as right now
	//  we are not able to create configMap through CSV. So we are not able to use configMap as mount in control-plane.
	//  For this reason go-client is created here instead of passing client from every other function
	//  where this function is called. we will remove this go-client creation in next release as we only need to
	//  read configMap from mounted volume.
	acc, err := getAccessor()
	if err != nil {
		return corev1.ResourceList{}, err
	}

	var (
		metaDataJobLimits  = make(corev1.ResourceList)
		dataJobLimits      = make(corev1.ResourceList)
		validatedResLimits = make(corev1.ResourceList)
	)

	installNs, present := os.LookupEnv("INSTALL_NAMESPACE")
	if !present {
		panic("Install Namespace not found in environment")
	}

	isOcp := helpers.CheckIsOpenshift(acc.GetRestConfig())
	if isOcp {
		tvkConfig, _ := os.LookupEnv(internal.TvkConfig)

		var conf *corev1.ConfigMap
		conf, err = acc.GetConfigMap(installNs, tvkConfig)
		if err != nil {
			log.Errorf("failed to get configMap %s/%s required for tvk configuration.",
				tvkConfig, installNs)

			return corev1.ResourceList{}, err
		}

		if resourceType == internal.NonDMJobResource {
			err = json.Unmarshal([]byte(conf.Data[internal.MetaMoverJobLimits]), &metaDataJobLimits)
			if err != nil {
				log.Errorf("failed to unmarshal tvk config metadataJobLimits")
				return corev1.ResourceList{}, err
			}

		} else if resourceType == internal.DMJobResource {
			err = json.Unmarshal([]byte(conf.Data[internal.DataMoverJobLimits]), &dataJobLimits)
			if err != nil {
				log.Errorf("failed to unmarshal tvk config dataJobLimits")
				return corev1.ResourceList{}, err
			}
		}

	} else {

		var (
			isExists bool
			metaConf []byte
			dataConf []byte
		)

		configDir := filepath.Join(internal.BasePath, internal.TvkConfigDir)

		if resourceType == internal.NonDMJobResource {
			metaResReqConfPath := filepath.Join(configDir, internal.MetaMoverJobLimits)
			isExists, _, err = shell.FileExistsInDir(configDir, internal.MetaMoverJobLimits)
			if err != nil || !isExists {
				log.Errorf("required mounted tvk configuration file %s not exists.", metaResReqConfPath)
				return corev1.ResourceList{}, err
			}

			metaConf, err = ioutil.ReadFile(metaResReqConfPath)
			if err != nil {
				log.Errorf("failed to read tvk configuration file %s", metaResReqConfPath)
				return corev1.ResourceList{}, err
			}

			if err = yaml.Unmarshal(metaConf, &metaDataJobLimits); err != nil {
				log.Errorf("failed to unmarshal tvk config metadataJobLimits")
				return corev1.ResourceList{}, err
			}
		} else if resourceType == internal.DMJobResource {

			dataResReqConfPath := filepath.Join(configDir, internal.DataMoverJobLimits)
			isExists, _, err = shell.FileExistsInDir(configDir, internal.DataMoverJobLimits)
			if err != nil || !isExists {
				log.Errorf("required mounted tvk configuration file %s not exists.", dataResReqConfPath)
				return corev1.ResourceList{}, err
			}

			dataConf, err = ioutil.ReadFile(dataResReqConfPath)
			if err != nil {
				log.Errorf("failed to read tvk configuration file %s", dataResReqConfPath)
				return corev1.ResourceList{}, err
			}

			if err = yaml.Unmarshal(dataConf, &dataJobLimits); err != nil {
				log.Errorf("failed to unmarshal tvk config dataJobLimits")
				return corev1.ResourceList{}, err
			}
		}
	}

	switch resourceType {
	case internal.DMJobResource:
		validatedResLimits = dataJobLimits
	case internal.NonDMJobResource:
		validatedResLimits = metaDataJobLimits
	}

	if isOcp {
		resReq := corev1.ResourceRequirements{
			Limits:   validatedResLimits,
			Requests: getDefaultResourceLimits(resourceType),
		}

		if err = validateResourceRequirements(resReq); err != nil {
			log.Errorf("resource requirements validation failed.")
			return corev1.ResourceList{}, err
		}
	}

	return validatedResLimits, nil
}

// GetContainerResources returns resource requirements for all jobs
func GetContainerResources(resourceType string) corev1.ResourceRequirements {
	var resLimit corev1.ResourceList
	var err error

	if resourceType == internal.DeploymentResource {
		limitMem, _ := resource.ParseQuantity(internal.DeploymentLimitMem) // 512 MiB
		limitCPU, _ := resource.ParseQuantity(internal.DeploymentLimitCPU) // 200m
		reqCPU, _ := resource.ParseQuantity(internal.DeploymentRequestCPU) // 10m
		reqMem, _ := resource.ParseQuantity(internal.DeploymentRequestMem) // 10MiB

		return corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: limitMem,
				corev1.ResourceCPU:    limitCPU,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: reqMem,
				corev1.ResourceCPU:    reqCPU,
			},
		}
	}

	resLimit, err = GetAndValidateResourceRequirements(resourceType)
	if err != nil {
		log.Info("resource requirements validation failed. using default resource requirements.")
		resLimit = getDefaultResourceLimits(resourceType)
	}

	return corev1.ResourceRequirements{
		Limits:   resLimit,
		Requests: getDefaultResourceRequests(resourceType),
	}
}
