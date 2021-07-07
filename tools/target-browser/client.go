package targetbrowser

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// getHTTPClient return http client based on provided config ClientCert, ClientKey, CaCert
func (targetBrowserConfig *Config) getHTTPClient() (*http.Client, error) {

	var caCertPool *x509.CertPool

	if targetBrowserConfig.CaCert != "" {
		caCert, err := ioutil.ReadFile(targetBrowserConfig.CaCert)
		if err != nil {
			return nil, fmt.Errorf("error opening ca-cert file %s, Error: %s", targetBrowserConfig.CaCert, err)
		}
		caCertPool = x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
	}

	// #nosec
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            caCertPool,
				InsecureSkipVerify: targetBrowserConfig.InsecureSkipTLS,
			},
		},
		Timeout: 30 * time.Second,
	}, nil
}
