package targetbrowser

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"
	"github.com/thedevsaddam/gojsonq"
)

// ListOptions for backupPlan
type ListOptions struct {
	Page           int    `url:"page"`
	PageSize       int    `url:"pageSize"`
	Ordering       string `url:"ordering"`
	TvkInstanceUID string `url:"tvkInstanceUID"`
}


// GetBackupPlans returns backupPlan with available options
func (c *Client) GetBackupPlans(options *ListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	req, err := http.NewRequest("GET",fmt.Sprintf("%sbackupplan?%s", c.baseURL, queryParam), nil)
	if err != nil {
		return err
	}

	res, err := c.sendRequest(req)
	if err != nil {
		return err
	}
	var backupPlanBytes bytes.Buffer
	gojsonq.New().FromString(res).From("results").Select(backupPlanSelector...).Writer(&backupPlanBytes)
	fmt.Printf("%s\n", backupPlanBytes.String())

	return  nil
}
