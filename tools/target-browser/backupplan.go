package targetbrowser

import (
	"bytes"
	"encoding/json"

	"github.com/thedevsaddam/gojsonq"

	"github.com/google/go-querystring/query"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/trilioData/tvk-plugins/internal"
)

// BackupPlanListOptions for backupPlan
type BackupPlanListOptions struct {
	CommonListOptions
	TvkInstanceUID string `url:"tvkInstanceUID,omitempty"`
}

// BackupPlan struct stores extracted fields from actual BackupPlan API GET response
type BackupPlan struct {
	Name                      string      `json:"Name"`
	Kind                      string      `json:"Kind"`
	UID                       string      `json:"UID"`
	Type                      string      `json:"Type"`
	TvkInstanceID             string      `json:"TVK Instance"`
	SuccessfulBackup          int         `json:"Successful Backup"`
	SuccessfulBackupTimestamp metav1.Time `json:"Successful Backup Timestamp"`
}

// BackupPlan struct stores extracted fields from actual BackupPlan API LIST response
type BackupPlanList struct {
	Metadata *ListMetadata `json:"metadata"`
	Results  []BackupPlan  `json:"results"`
}

// GetBackupPlans returns backupPlan list stored on mounted target with available options
func (auth *AuthInfo) GetBackupPlans(options *BackupPlanListOptions, backupPlanUIDs []string) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}

	queryParam := values.Encode()
	response, err := auth.TriggerAPIs(queryParam, internal.BackupPlanAPIPath, BackupPlanSelector, backupPlanUIDs, true, false)
	if err != nil {
		return err
	}

	return PrintFormattedResponse(internal.BackupPlanAPIPath, string(response), options.OutputFormat)
}

// normalizeBPlanDataToRowsAndColumns normalizes backupPlan API response and generates metav1.TableRow & metav1.TableColumnDefinition
// which will be used for printing table formatted output.
// If 'wideOutput=true', then all defined fields of Backup struct will be printed as output columns
// If 'wideOutput=false', then selected number of fields of Backup struct from first field will be printed as output columns
func normalizeBPlanDataToRowsAndColumns(response string, wideOutput bool) ([]metav1.TableRow, []metav1.TableColumnDefinition, error) {

	var respBytes bytes.Buffer
	gojsonq.New().FromString(response).From(internal.Results).Select(BackupPlanSelector...).Writer(&respBytes)

	var bPlanList BackupPlanList
	err := json.Unmarshal(respBytes.Bytes(), &bPlanList.Results)
	if err != nil {
		return nil, nil, err
	}

	if len(bPlanList.Results) == 0 {
		return nil, nil, nil
	}

	var rows []metav1.TableRow
	for i := range bPlanList.Results {
		bPlan := bPlanList.Results[i]
		rows = append(rows, metav1.TableRow{
			Cells: []interface{}{bPlan.Name, bPlan.Kind, bPlan.UID, bPlan.Type, bPlan.TvkInstanceID,
				bPlan.SuccessfulBackup, bPlan.SuccessfulBackupTimestamp},
		})
	}

	var columns []metav1.TableColumnDefinition
	if wideOutput {
		columns = getColumnDefinitions(bPlanList.Results[0], 0)
	} else {
		columns = getColumnDefinitions(bPlanList.Results[0], 5)
	}

	return rows, columns, err
}

// TriggerAPIs returns backup or backupPlan list stored on mounted target with available options
func (auth *AuthInfo) TriggerAPIs(queryParam, apiPath string, selector, args []string,
	pathParam, isTrilioResourcesAPI bool) ([]byte, error) {

	var (
		resp, body []byte
		err        error
	)

	if len(args) > 0 {
		var respData []interface{}
		for _, uid := range args {

			if pathParam {
				resp, err = auth.TriggerAPI(uid, queryParam, apiPath, selector)
			} else if !pathParam && isTrilioResourcesAPI {
				resp, err = auth.TriggerAPI("", queryParam, getTrilioResourcesAPIPath(uid), selector)
			} else {
				resp, err = auth.TriggerAPI("", queryParam, apiPath, selector)
			}

			if err != nil {
				return nil, err
			}

			var result interface{}
			if uErr := json.Unmarshal(resp, &result); uErr != nil {
				return nil, uErr
			}
			respData = append(respData, result)
		}

		body, err = json.MarshalIndent(respData, "", "  ")
		if err != nil {
			return nil, err
		}

		body, err = parseData(body)
		if err != nil {
			return nil, err
		}

		return body, nil
	}

	resp, err = auth.TriggerAPI("", queryParam, apiPath, selector)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
