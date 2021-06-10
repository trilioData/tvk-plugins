package targetbrowser

import (
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"
)

// MetadataListOptions for metadata
type MetadataListOptions struct {
	BackupUID     string `url:"backupUID"`
	BackupPlanUID string `url:"backupPlanUID"`
}

func (c *Client) GetMetadata(options *MetadataListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/metadata?%s", c.baseURL, queryParam), nil)
	if err != nil {
		return err
	}

	res, err := c.sendRequest(req)
	if err != nil {
		return err
	}
	fmt.Println(res)
	return nil
}
