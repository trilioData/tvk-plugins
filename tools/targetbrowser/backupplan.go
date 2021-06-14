package targetbrowser

import (
	"github.com/google/go-querystring/query"
)

// BackupPlanListOptions for backupPlan
type BackupPlanListOptions struct {
	Page           int    `url:"page"`
	PageSize       int    `url:"pageSize"`
	Ordering       string `url:"ordering"`
	TvkInstanceUID string `url:"tvkInstanceUID"`
}

const backupPlanEndPoint = "backupplan"

// GetBackupPlans returns backupPlan with available options
func (c *Client) GetBackupPlans(options *BackupPlanListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	return c.TriggerAPI(backupPlanEndPoint, queryParam, backupPlanSelector)
}
