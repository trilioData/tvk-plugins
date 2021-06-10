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

// NewClient creates new trilio backend client with given API key
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		baseURL: "http://pankaj1.k8s-tvk.com/sample-target.pankaj1/",
	}
}

type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Content-type and body should be already added to req
func (c *Client) sendRequest(req *http.Request) (string, error) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
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
		return "", errors.Errorf("error is %v, status code: %d", err, res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
