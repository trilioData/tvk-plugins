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
	ExpirationStartTimestamp string `url:"expirationStartTimestamp,omitempty"`
	ExpirationEndTimestamp   string `url:"expirationEndTimestamp,omitempty"`
}

// Backup struct stores extracted fields from actual Backup API GET response
type Backup struct {
	Name            string `json:"Name"`
	Kind            string `json:"Kind"`
	UID             string `json:"UID"`
	Type            string `json:"Type"`
	Size            string `json:"Size"`
	Status          string `json:"Status"`
	BackupPlanUID   string `json:"BackupPlan UID"`
	TvkInstanceID   string `json:"TVK Instance UID"`
	TvkInstanceName string `json:"TVK Instance Name"`
	CreationTime    string `json:"Start Time"`
	CompletionTime  string `json:"End Time"`
	ExpirationTime  string `json:"Expiration Time"`
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
	response, err := auth.TriggerAPIs(queryParam, internal.BackupAPIPath, backupUIDs)
	if err != nil {
		return err
	}

	return PrintFormattedResponse(internal.BackupAPIPath, string(response), options.OutputFormat)
}

func (auth *AuthInfo) TriggerAPI(apiPath, queryParam string) ([]byte, error) {
	tvkURL, err := url.Parse(auth.TvkHost)
	if err != nil {
		return nil, err
	}

	tvkURL.Path = path.Join(tvkURL.Path, auth.TargetBrowserPath, apiPath)
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
		return nil, fmt.Errorf("%s %s did not successfully completed - %s", http.MethodGet, req.URL.String(), resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func parseData(respData []byte) ([]byte, error) {
	type result struct {
		Result interface{} `json:"results"`
	}
	var r result
	r.Result = []interface{}{respData}
	if err := json.Unmarshal(respData, &r.Result); err != nil {
		return nil, err
	}
	respDataByte, err := json.Marshal(r)
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
	var respBytes bytes.Buffer
	gojsonq.New().FromString(response).From(internal.Results).Select(BackupSelector...).Writer(&respBytes)

	var backupList BackupList
	err := json.Unmarshal(respBytes.Bytes(), &backupList.Results)
	if err != nil {
		return nil, nil, err
	}

	if len(backupList.Results) == 0 {
		return nil, nil, nil
	}

	var rows []metav1.TableRow
	for i := range backupList.Results {
		backup := backupList.Results[i]
		rows = append(rows, metav1.TableRow{
			Cells: []interface{}{backup.Name, backup.Kind, backup.UID, backup.Type, backup.Size, backup.Status,
				backup.BackupPlanUID, backup.TvkInstanceID, backup.TvkInstanceName, backup.CreationTime,
				backup.CompletionTime, backup.ExpirationTime},
		})
	}

	var columns []metav1.TableColumnDefinition
	if wideOutput {
		columns = getColumnDefinitions(backupList.Results[0], 0)
	} else {
		columns = getColumnDefinitions(backupList.Results[0], 6)
	}

	return rows, columns, err
}
