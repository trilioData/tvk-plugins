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
	log "github.com/sirupsen/logrus"
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
	return auth.GetBackupOrBackupPlan(queryParam, internal.BackupAPIPath, backupSelector, backupUIDs)
}

func (auth *AuthInfo) TriggerAPI(queryPath, queryParam, apiPath string, selector []string) (string, error) {

	tvkURL, err := url.Parse(auth.TvkHost)
	if err != nil {
		return "", err
	}

	tvkURL.Path = path.Join(tvkURL.Path, auth.TargetBrowserPath, apiPath)
	tvkURL.Scheme = internal.HTTPscheme
	if auth.UseHTTPS {
		tvkURL.Scheme = internal.HTTPSscheme
	}
	var tvkURLPath string
	if queryPath == "" {
		tvkURLPath = fmt.Sprintf("%s?%s", tvkURL.String(), queryParam)
	} else {
		tvkURLPath = fmt.Sprintf("%s/%s?%s", tvkURL.String(), queryPath, queryParam)
	}
	req, err := http.NewRequest(http.MethodGet, tvkURLPath, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set(internal.ContentType, internal.ContentApplicationJSON)
	req.Header.Add(internal.JweToken, auth.JWT)
	resp, err := auth.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || resp.Body == nil {
		return "", fmt.Errorf("%s %s did not successfully completed - %s", http.MethodGet, apiPath, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if len(selector) > 0 {
		if queryPath != "" {
			body = parseData(body)
		}
		var backupBytes bytes.Buffer
		gojsonq.New().FromString(string(body)).From(internal.Results).Select(selector...).Writer(&backupBytes)
		if queryPath != "" {
			var result []interface{}
			if uErr := json.Unmarshal(backupBytes.Bytes(), &result); uErr != nil {
				return "", uErr
			}
			body, err = json.Marshal(result[0])
			if err != nil {
				return "", nil
			}
		} else {
			body = backupBytes.Bytes()
		}
	}
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		return "", fmt.Errorf("JSON parse error: %s", err.Error())
	}
	return prettyJSON.String(), nil
}

func parseData(respData []byte) []byte {
	type result struct {
		Result []interface{} `json:"results"`
	}
	var r result
	r.Result = []interface{}{respData}
	err := json.Unmarshal(respData, &r.Result[0])
	if err != nil {
		log.Fatal(err.Error())
	}
	respDataByte, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Fatal(err.Error())
	}
	return respDataByte
}
