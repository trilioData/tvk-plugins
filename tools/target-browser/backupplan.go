package targetbrowser

import (
	"github.com/google/go-querystring/query"

	"github.com/trilioData/tvk-plugins/internal"
)

// BackupPlanListOptions for backupPlan
type BackupPlanListOptions struct {
	CommonListOptions
	TvkInstanceUID string `url:"tvkInstanceUID"`
}

// GetBackupPlans returns backupPlan list stored on mounted target with available options
func (auth *AuthInfo) GetBackupPlans(options *BackupPlanListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	return auth.TriggerAPI(queryParam, internal.BackupPlanAPIPath, backupPlanSelector)
}
