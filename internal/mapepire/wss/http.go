package wss

import (
	"crypto/tls"
	"net/http"
)

// NewTrustedHTTPClient builds the HTTPS half of the managed daemon boundary.
// It shares the WSS TLS policy but never carries credentials.
func NewTrustedHTTPClient(options Options) (*http.Client, error) {
	tlsConfig := options.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig = tlsConfig.Clone()
	if tlsConfig.InsecureSkipVerify {
		return nil, ErrInvalidConfiguration
	}
	if options.Pin != "" {
		if !validPin(options.Pin) {
			return nil, ErrInvalidConfiguration
		}
		previous := tlsConfig.VerifyConnection
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = verifyPin(previous, tlsConfig.ServerName, options.Pin)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	client := &http.Client{Transport: transport}
	return client, nil
}
