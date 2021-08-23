package targetbrowser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
)

type CommonListOptions struct {
	Page           int    `url:"page"`
	PageSize       int    `url:"pageSize"`
	OrderBy        string `url:"ordering,omitempty"`
	OperationScope string `url:"operationScope,omitempty"`
	OutputFormat   string
}

// ListMetadata for Pagination
type ListMetadata struct {
	Total int `json:"total"`
	Next  int `json:"next"`
}

// PrintFormattedResponse formats API response into desired 'outputFormat' for given 'apiPath'
// currently supports 'yaml', 'json', 'wide' formats
// default format for 'metadata' API response will be 'yaml'
// for 'backupplan', 'backup' API responses, default format will be 'wideOutput=false' so that only few important columns are printed
func PrintFormattedResponse(apiPath, response, outputFormat string) error {
	var err error
	if outputFormat == internal.FormatWIDE {
		err = PrintTable(apiPath, response, true)
	} else if outputFormat == internal.FormatYAML {
		prettyJSON, cErr := yaml.JSONToYAML([]byte(response))
		if cErr != nil {
			return fmt.Errorf("YAML formatting error: %s", cErr.Error())
		}
		fmt.Println(string(prettyJSON))
	} else if outputFormat == internal.FormatJSON {
		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, []byte(response), "", "  ")
		if err != nil {
			return fmt.Errorf("JSON formatting error: %s", err.Error())
		}
		fmt.Println(prettyJSON.String())
	} else {
		err = PrintTable(apiPath, response, false)
	}

	return err
}

// PrintTable formats response according to API path and prints in table format considering 'wideOutput' value
func PrintTable(apiPath, response string, wideOutput bool) error {
	var (
		rows    []metav1.TableRow
		columns []metav1.TableColumnDefinition
		err     error
	)

	if apiPath == internal.BackupPlanAPIPath {
		rows, columns, err = normalizeBPlanDataToRowsAndColumns(response, wideOutput)
	} else if apiPath == internal.BackupAPIPath {
		rows, columns, err = normalizeBackupDataToRowsAndColumns(response, wideOutput)
	} else {
		return fmt.Errorf("unknown response data [%s API] received for formatting ", apiPath)
	}
	if err != nil {
		return err
	}

	table := &metav1.Table{
		ColumnDefinitions: columns,
		Rows:              rows,
	}

	// Print the table
	printer := printers.NewTablePrinter(printers.PrintOptions{})
	return printer.PrintObj(table, os.Stdout)
}

// getColumnDefinitions return custom TableColumnDefinition based on type of struct data passed.
// if columns=0, then all struct fields will be returned as TableColumnDefinition
// if columns=n, then first 'n' struct fields will be returned as TableColumnDefinition
func getColumnDefinitions(data interface{}, columns int) []metav1.TableColumnDefinition {
	var tags []metav1.TableColumnDefinition
	val := reflect.ValueOf(data)
	for i := 0; i < val.Type().NumField(); i++ {
		if columns > 0 && i == columns {
			return tags
		}

		t := val.Type().Field(i)
		fieldName := t.Name
		kind := t.Type.Kind().String()

		switch jsonTag := t.Tag.Get("json"); jsonTag {
		case "-":
		case "":
			tags = append(tags, metav1.TableColumnDefinition{Name: fieldName, Type: kind})
		default:
			parts := strings.Split(jsonTag, ",")
			name := parts[0]
			if name == "" {
				name = fieldName
			}
			tags = append(tags, metav1.TableColumnDefinition{Name: name, Type: kind})
		}
	}
	return tags
}
