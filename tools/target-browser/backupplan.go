package targetbrowser

import (
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
	return auth.GetBackupOrBackupPlan(queryParam, internal.BackupPlanAPIPath, backupPlanSelector, backupPlanUIDs)
}

// GetBackupOrBackupPlan returns backup or backupPlan list stored on mounted target with available options
func (auth *AuthInfo) GetBackupOrBackupPlan(queryParam, apiPath string, selector, args []string) error {
	if len(args) > 0 {
		var respData []string
		for _, bpID := range args {
			resp, err := auth.TriggerAPI(bpID, queryParam, apiPath, selector)
			if err != nil {
				return err
			}
			respData = append(respData, resp)
		}
		fmt.Println(respData)
		return nil
	}
	resp, err := auth.TriggerAPI("", queryParam, apiPath, selector)
	fmt.Println(resp)
	return err
}
