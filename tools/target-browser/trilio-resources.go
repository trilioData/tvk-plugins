package targetbrowser

import (
	"encoding/json"
	"errors"
	"path"

	"github.com/google/go-querystring/query"
	"github.com/trilioData/tvk-plugins/internal"
)

// TrilioResourcesListOptions for trilio-resources
type TrilioResourcesListOptions struct {
	BackupUID     string   `url:"backupUID"`
	BackupPlanUID string   `url:"backupPlanUID"`
	Kinds         []string `url:"kinds"`
	CommonListOptions
}

// GetTrilioResources returns trilio resources of particular backup on mounted target
func (auth *AuthInfo) GetTrilioResources(options *TrilioResourcesListOptions, backupUIDs []string) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	resp, apiErr := auth.TriggerTrilioResourcesAPI(queryParam, backupUIDs)
	if apiErr != nil {
		return apiErr
	}

	if options.OutputFormat == "" || options.OutputFormat == internal.FormatWIDE {
		options.OutputFormat = internal.FormatYAML
	}

	return PrintFormattedResponse(internal.TrilioResourcesAPIPath, string(resp), options.OutputFormat)
}

// TriggerTrilioResourcesAPI triggers API endpoint for getting trilio resources for one or multiple backups
func (auth *AuthInfo) TriggerTrilioResourcesAPI(queryParam string, args []string) ([]byte, error) {

	if len(args) == 0 {
		return nil, errors.New("at-least 1 backupUID is needed")
	}

	var respData []interface{}

	for _, uid := range args {

		resp, err := auth.TriggerAPI("", queryParam,
			path.Join(internal.BackupAPIPath, uid, internal.TrilioResourcesAPIPath), []string{})

		if err != nil {
			return nil, err
		}

		var result interface{}
		if uErr := json.Unmarshal(resp, &result); uErr != nil {
			return nil, uErr
		}
		respData = append(respData, result)
	}

	body, err := json.MarshalIndent(respData, "", "  ")
	if err != nil {
		return nil, err
	}

	body, err = parseData(body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
