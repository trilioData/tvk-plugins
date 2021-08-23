package targetbrowser

import (
	"github.com/google/go-querystring/query"

	"github.com/trilioData/tvk-plugins/internal"
)

// MetadataListOptions for metadata
type MetadataListOptions struct {
	BackupUID     string `url:"backupUID"`
	BackupPlanUID string `url:"backupPlanUID"`
	OutputFormat  string
}

// GetMetadata returns metadata of backup on mounted target
func (auth *AuthInfo) GetMetadata(options *MetadataListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	resp, apiErr := auth.TriggerAPI("", queryParam, internal.MetadataAPIPath, []string{})
	if apiErr != nil {
		return apiErr
	}

	if options.OutputFormat == "" || options.OutputFormat == internal.FormatWIDE {
		options.OutputFormat = internal.FormatYAML
	}

	return PrintFormattedResponse(internal.MetadataAPIPath, string(resp), options.OutputFormat)
}
