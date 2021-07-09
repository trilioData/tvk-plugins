package targetbrowser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-querystring/query"
	"github.com/thedevsaddam/gojsonq"

	"github.com/trilioData/tvk-plugins/internal"
)

// BackupListOptions for backup API calls
type BackupListOptions struct {
	BackupPlanUID string `url:"backupPlanUID,omitempty"`
	BackupStatus  string `url:"status,omitempty"`
	CommonListOptions
}

// Backup struct stores extracted fields from actual Backup API GET response
type Backup struct {
	Name      string `json:"Name"`
	UID       string `json:"UID"`
	Type      string `json:"Type"`
	Size      string `json:"Size"`
	Status    string `json:"Status"`
	CreatedAt string `json:"Created At"`
	Location  string `json:"Location"`
}

// BackupList struct stores extracted fields from actual Backup API LIST response
type BackupList struct {
	Metadata *ListMetadata `json:"metadata"`
	Results  []Backup      `json:"results"`
}

// GetBackups returns backup list stored on mounted target with available options
func (auth *AuthInfo) GetBackups(options *BackupListOptions, backupUIDs []string) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	response, err := auth.TriggerMultipleAPI(queryParam, internal.BackupAPIPath, backupSelector, backupUIDs)
	if err != nil {
		return err
	}

	return PrintFormattedResponse(internal.BackupAPIPath, response, options.OutputFormat)
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

// normalizeBackupDataToRowsAndColumns normalizes backup API response and generates metav1.TableRow & metav1.TableColumnDefinition
// which will be used for printing table formatted output.
// If 'wideOutput=true', then all defined fields of Backup struct will be printed as output columns
// If 'wideOutput=false', then selected number of fields of Backup struct from first field will be printed as output columns
func normalizeBackupDataToRowsAndColumns(response string, wideOutput bool) ([]metav1.TableRow, []metav1.TableColumnDefinition, error) {
	var backupList BackupList

	err := json.Unmarshal([]byte(response), &backupList.Results)
	if err != nil {
		return nil, nil, err
	}

	if len(backupList.Results) == 0 {
		return nil, nil, nil
	}

	var rows []metav1.TableRow
	for _, backup := range backupList.Results {
		rows = append(rows, metav1.TableRow{
			Cells: []interface{}{backup.Name, backup.UID, backup.Type, backup.Size, backup.Status, backup.CreatedAt, backup.Location},
		})
	}

	var columns []metav1.TableColumnDefinition
	if wideOutput {
		columns = getColumnDefinitions(backupList.Results[0], 0)
	} else {
		columns = getColumnDefinitions(backupList.Results[0], 5)
	}

	return rows, columns, err
}