package targetbrowser

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-querystring/query"

	"github.com/trilioData/tvk-plugins/internal"
)

// BackupPlanListOptions for backupPlan
type BackupPlanListOptions struct {
	CommonListOptions
	TvkInstanceUID string `url:"tvkInstanceUID"`
}

// GetBackupPlans returns backupPlan list stored on mounted target with available options
func (auth *AuthInfo) GetBackupPlans(options *BackupPlanListOptions, backupPlanUIDs []string) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	return auth.TriggerMultipleAPI(queryParam, internal.BackupPlanAPIPath, backupPlanSelector, backupPlanUIDs)
}

// TriggerMultipleAPI returns backup or backupPlan list stored on mounted target with available options
func (auth *AuthInfo) TriggerMultipleAPI(queryParam, apiPath string, selector, args []string) error {
	if len(args) > 0 {
		var respData []interface{}
		for _, bpID := range args {
			resp, err := auth.TriggerAPI(bpID, queryParam, apiPath, selector)
			if err != nil {
				return err
			}
			var result []interface{}
			if uErr := json.Unmarshal(resp, &result); uErr != nil {
				return uErr
			}
			respData = append(respData, result[0])
		}

		body, err := json.MarshalIndent(respData, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(body))
		return nil
	}
	resp, err := auth.TriggerAPI("", queryParam, apiPath, selector)
	if err != nil {
		return err
	}
	fmt.Println(string(resp))
	return nil
}
