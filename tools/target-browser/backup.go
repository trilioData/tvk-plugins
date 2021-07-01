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

type CommonListOptions struct {
	Page     int    `url:"page"`
	PageSize int    `url:"pageSize"`
	OrderBy  string `url:"ordering"`
}

// BackupListOptions for backup
type BackupListOptions struct {
	BackupPlanUID string `url:"backupPlanUID"`
	BackupStatus  string `url:"status"`
	CommonListOptions
}

// GetBackups returns backup list stored on mounted target with available options
func (auth *AuthInfo) GetBackups(options *BackupListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	return auth.TriggerAPI(queryParam, internal.BackupAPIPath, backupSelector)
}

func (auth *AuthInfo) TriggerAPI(queryParam, apiPath string, selector []string) error {

	tvkURL, err := url.Parse(auth.TvkHost)
	if err != nil {
		return err
	}

	tvkURL.Path = path.Join(tvkURL.Path, auth.TargetBrowserPath, apiPath)
	tvkURL.Scheme = internal.HTTPscheme
	if auth.UseHTTPS {
		tvkURL.Scheme = internal.HTTPSscheme
	}

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
	if len(selector) > 0 {
		var backupBytes bytes.Buffer
		gojsonq.New().FromString(string(body)).From(internal.Results).Select(selector...).Writer(&backupBytes)
		body = backupBytes.Bytes()
	}
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, body, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON parse error: %s", err.Error())
	}
	fmt.Println(prettyJSON.String())

	return nil
}
