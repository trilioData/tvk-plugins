package targetbrowser

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

type Client struct {
	apiKey     string
	baseURL    string
	HTTPClient *http.Client
}

const (
	acceptHeader = "application/json"
	contentType  = "application/json"
	MethodGet    = "GET"
	baseURL      = "http://pankaj-tb.k8s-tvk.com/sample-target.pankaj-tb/"
)

// NewClient Create new HTTP client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Content-type and body should be already added to request
func (c *Client) sendRequest(req *http.Request) (string, error) {
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("Content-Type", contentType)
	req.Header.Add("jweToken", c.apiKey)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// Try to unmarshall into errorResponse
	if res.StatusCode != http.StatusOK {
		var errRes errorResponse
		if err = json.NewDecoder(res.Body).Decode(&errRes); err == nil {
			return "", errors.New(errRes.Message)
		}
		return "", errors.Errorf("error is %s, status code: %d", err.Error(), res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
