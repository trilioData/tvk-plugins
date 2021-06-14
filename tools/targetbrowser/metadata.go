package targetbrowser

import (
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"
	log "github.com/sirupsen/logrus"
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
	log.Debugf("Base URL of target-Browser: %s, API endPoint is %s and Query Param is %s.", c.baseURL, metaDataEndPoint, queryParam)
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
