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

	var cert tls.Certificate
	var err error
	if targetBrowserConfig.ClientCert != "" && targetBrowserConfig.ClientKey != "" {
		cert, err = tls.LoadX509KeyPair(targetBrowserConfig.ClientCert, targetBrowserConfig.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("error creating x509 keypair from client cert file %s and client key file %s",
				targetBrowserConfig.ClientCert, targetBrowserConfig.ClientKey)
		}
	}

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
				Certificates:       []tls.Certificate{cert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: targetBrowserConfig.InsecureSkipTLS,
			},
		},
		Timeout: 60 * time.Second,
	}, nil
}
