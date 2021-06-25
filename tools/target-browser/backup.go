package targetbrowser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/google/go-querystring/query"
	"github.com/thedevsaddam/gojsonq"
	"github.com/trilioData/tvk-plugins/internal"
)

// BackupListOptions for backup
type BackupListOptions struct {
	Page          int    `url:"page"`
	PageSize      int    `url:"pageSize"`
	Ordering      string `url:"ordering"`
	BackupPlanUID string `url:"backupPlanUID"`
	BackupStatus  string `url:"status"`
}

// GetBackups returns backup list stored on NFS target with available options
func (auth *AuthInfo) GetBackups(options *BackupListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	return auth.TriggerAPI(queryParam, internal.BackupPath, backupSelector)
}

func (auth *AuthInfo) TriggerAPI(queryParam, apiPath string, selector []string) error {

	tvkURL, err := url.Parse(auth.TvkHost)
	if err != nil {
		return err
	}

	tvkURL.Path = path.Join(tvkURL.Path, auth.TargetBrowserPath, apiPath)
	tvkURL.Scheme = internal.HTTPScheme

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s?%s", tvkURL.String(), queryParam), nil)
	if err != nil {
		return err
	}

	req.Header.Set(internal.ContentType, internal.ContentApplicationJSON)
	req.Header.Add(internal.JweToken, auth.JWT)
	resp, err := auth.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || resp.Body == nil {
		return fmt.Errorf("%s %s did not successfully completed - %s", http.MethodGet, apiPath, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var backupBytes bytes.Buffer
	gojsonq.New().FromString(string(body)).From(internal.Results).Select(selector...).Writer(&backupBytes)

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, backupBytes.Bytes(), "", "\t")
	if err != nil {
		return fmt.Errorf("JSON parse error: %s", err.Error())
	}
	fmt.Println(prettyJSON.String())
	return nil
}
