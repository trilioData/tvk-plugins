package targetbrowser

import (
	"github.com/google/go-querystring/query"
	log "github.com/sirupsen/logrus"

	"github.com/trilioData/tvk-plugins/internal"
)

// ResourceMetadataListOptions for resource-metadata
type ResourceMetadataListOptions struct {
	BackupUID     string `url:"backupUID"`
	BackupPlanUID string `url:"backupPlanUID"`
	Group         string `url:"group"`
	Version       string `url:"version"`
	Kind          string `url:"kind"`
	Name          string `url:"name"`
	OutputFormat  string
}

// GetResourceMetadata returns metadata of backup on mounted target
func (auth *AuthInfo) GetResourceMetadata(options *ResourceMetadataListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	resp, apiErr := auth.TriggerAPI("", queryParam, internal.ResourceMetadataAPIPath, []string{})
	if apiErr != nil {
		log.Info(apiErr)
		return apiErr
	}

	if options.OutputFormat == "" || options.OutputFormat == internal.FormatWIDE {
		options.OutputFormat = internal.FormatYAML
	}

	return PrintFormattedResponse(internal.ResourceMetadataAPIPath, string(resp), options.OutputFormat)
}
