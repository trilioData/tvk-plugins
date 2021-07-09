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
	OrderBy  string `url:"ordering,omitempty"`
}

// BackupListOptions for backup
type BackupListOptions struct {
	BackupPlanUID string `url:"backupPlanUID,omitempty"`
	BackupStatus  string `url:"status,omitempty"`
	CommonListOptions
}

// GetBackups returns backup list stored on mounted target with available options
func (auth *AuthInfo) GetBackups(options *BackupListOptions, backupUIDs []string) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	return auth.TriggerMultipleAPI(queryParam, internal.BackupAPIPath, backupSelector, backupUIDs)
}

func (auth *AuthInfo) TriggerAPI(pathParam, queryParam, apiPath string, selector []string) ([]byte, error) {
	tvkURL, err := url.Parse(auth.TvkHost)
	if err != nil {
		return nil, err
	}

	tvkURL.Path = path.Join(tvkURL.Path, auth.TargetBrowserPath, apiPath, pathParam)
	tvkURL.Scheme = internal.HTTPscheme
	if auth.UseHTTPS {
		tvkURL.Scheme = internal.HTTPSscheme
	}
	tvkURL.RawQuery = queryParam
	req, err := http.NewRequest(http.MethodGet, tvkURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set(internal.ContentType, internal.ContentApplicationJSON)
	req.Header.Add(internal.JweToken, auth.JWT)
	resp, err := auth.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || resp.Body == nil {
		return nil, fmt.Errorf("%s %s did not successfully completed - %s", http.MethodGet, apiPath, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(selector) > 0 {
		if pathParam != "" {
			body, err = parseData(body)
			if err != nil {
				return nil, err
			}
		}
		var respBytes bytes.Buffer
		gojsonq.New().FromString(string(body)).From(internal.Results).Select(selector...).Writer(&respBytes)
		if pathParam != "" {
			return respBytes.Bytes(), nil
		}
		body = respBytes.Bytes()
	}
	var prettyJSON bytes.Buffer
	if indErr := json.Indent(&prettyJSON, body, "", "  "); indErr != nil {
		return nil, fmt.Errorf("JSON parse error: %s", indErr.Error())
	}
	return prettyJSON.Bytes(), nil
}

func parseData(respData []byte) ([]byte, error) {
	type result struct {
		Result []interface{} `json:"results"`
	}
	var r result
	r.Result = []interface{}{respData}
	if err := json.Unmarshal(respData, &r.Result[0]); err != nil {
		return nil, err
	}
	respDataByte, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return respDataByte, nil
}
