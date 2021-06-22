package targetbrowser

import (
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"
)

const metaDataEndPoint = "metadata"

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
	req, err := http.NewRequest(MethodGet, fmt.Sprintf("%s/%s?%s", c.baseURL, metaDataEndPoint, queryParam), nil)
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
