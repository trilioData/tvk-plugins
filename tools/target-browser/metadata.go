package targetbrowser

import (
	"fmt"

	"github.com/google/go-querystring/query"

	"github.com/trilioData/tvk-plugins/internal"
)

// MetadataListOptions for metadata
type MetadataListOptions struct {
	BackupUID     string `url:"backupUID"`
	BackupPlanUID string `url:"backupPlanUID"`
}

// GetMetadata returns metadata of backup on mounted target
func (auth *AuthInfo) GetMetadata(options *MetadataListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	resp, apiErr := auth.TriggerAPI("", queryParam, internal.MetadataAPIPath, []string{})
	fmt.Println(resp)
	return apiErr
}
