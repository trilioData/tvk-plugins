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
	"github.com/trilioData/tvk-plugins/internal"
)

// MetadataListOptions for metadata
type MetadataListOptions struct {
	BackupUID     string `url:"backupUID"`
	BackupPlanUID string `url:"backupPlanUID"`
}

// GetMetadata returns metadata of backup
func (auth *AuthInfo) GetMetadata(options *MetadataListOptions) error {
	values, err := query.Values(options)
	if err != nil {
		return err
	}
	queryParam := values.Encode()
	tvkURL, err := url.Parse(auth.TvkHost)
	if err != nil {
		return err
	}

	tvkURL.Path = path.Join(tvkURL.Path, auth.TargetBrowserPath, internal.MetadataPath)
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
		return fmt.Errorf("%s %s did not successfully completed - %s", http.MethodGet, internal.MetadataPath, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var metadataPrettyJSON bytes.Buffer
	err = json.Indent(&metadataPrettyJSON, body, "", "\t")
	if err != nil {
		return fmt.Errorf("JSON parse error: %s", err.Error())
	}

	fmt.Println(metadataPrettyJSON.String())
	return nil
}
