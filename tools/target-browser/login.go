package targetbrowser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/thedevsaddam/gojsonq"

	"github.com/trilioData/tvk-plugins/internal"
)

// Login generates '/login' endpoint path from TvkHost and returns JWT by and calling that API endpoint. Also returns http
// client for further use
func (targetBrowserConfig *Config) Login(tvkHost string) (string, *http.Client, error) {
	tvkURL, err := url.Parse(tvkHost)
	if err != nil {
		return "", nil, err
	}

	tvkURL.Path = path.Join(tvkURL.Path, internal.APIPath, internal.V1Version, internal.LoginPath)
	tvkURL.Scheme = internal.HTTPscheme
	if targetBrowserConfig.UseHTTPS {
		tvkURL.Scheme = internal.HTTPSscheme
	}

	kubeConfigBytes, err := ioutil.ReadFile(targetBrowserConfig.KubeConfig)
	if err != nil {
		return "", nil, err
	}

	postBody, err := json.Marshal(map[string]string{
		internal.KubeConfigParam: string(kubeConfigBytes),
	})
	if err != nil {
		return "", nil, err
	}

	jweToken, client, err := targetBrowserConfig.GetAuthJWT(tvkURL.String(), postBody)
	if err != nil {
		return "", nil, err
	}

	return jweToken, client, nil
}

// GetAuthJWT returns JWT by calling web-backend api '/login' and also returns generated http client for further use.
func (targetBrowserConfig *Config) GetAuthJWT(loginURL string, postBody []byte) (string, *http.Client, error) {

	client, err := targetBrowserConfig.getHTTPClient()
	if err != nil {
		return "", nil, err
	}

	// Get request to check redirected url path
	res, err := client.Get(loginURL)
	if err != nil {
		return "", nil, err
	}

	// POST req for /login endpoint
	req, err := http.NewRequest(http.MethodPost, res.Request.URL.String(), bytes.NewBuffer(postBody))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set(internal.ContentType, internal.ContentApplicationJSON)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || resp.Body == nil {
		return "", nil, fmt.Errorf("%s %s did not successfully completed - %s", http.MethodPost, loginURL, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	jweBytes := gojsonq.New().FromString(string(body)).Find(internal.JweToken)
	if jweBytes == nil {
		return "", nil, fmt.Errorf("%s %s failed to retrieve %s from response body", http.MethodPost, loginURL, internal.JweToken)
	}

	return jweBytes.(string), client, nil
}
