package targetbrowser

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"
	"github.com/thedevsaddam/gojsonq"
)

// BackupListOptions for backup
type BackupListOptions struct {
	Page          int    `url:"page"`
	PageSize      int    `url:"pageSize"`
	Ordering      string `url:"ordering"`
	BackupPlanUID string `url:"backupPlanUID"`
	BackupStatus  string `url:"backupStatus"`
}

// GetBackups returns backup with available options
func (c *Client) GetBackups(options *BackupListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	fmt.Printf("\n Backup URL: %s\n", fmt.Sprintf("%s/backup?%s", c.baseURL, queryParam))
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/backup?%s", c.baseURL, queryParam), nil)
	if err != nil {
		return err
	}

	res, err := c.sendRequest(req)
	if err != nil {
		return err
	}
	var backupBytes bytes.Buffer
	gojsonq.New().FromString(res).From("results").Select(backupSelector...).Writer(&backupBytes)
	fmt.Printf("%s\n", backupBytes.String())
	return nil
}
