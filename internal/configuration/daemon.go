package configuration

import (
	"bac-nexus/internal/profile"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const daemonVersion = "2.3.5"

// ManagedDaemonProbe is the production, unauthenticated daemon inspection.
type ManagedDaemonProbe struct {
	endpoint string
	client   *http.Client
	timeout  time.Duration
}

func NewManagedDaemonProbe(host string, port int, trust *profile.TrustEvidence) (*ManagedDaemonProbe, error) {
	if net.ParseIP(host) == nil && strings.TrimSpace(host) == "" {
		return nil, errors.New("daemon host is required")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("daemon port is invalid")
	}
	tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if trust != nil && trust.Pin != "" {
		if !strings.HasPrefix(trust.Pin, "sha256/") || len(trust.Pin) != 50 {
			return nil, errors.New("invalid TLS trust pin")
		}
		want := trust.Pin
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("TLS identity validation failed")
			}
			leaf := state.PeerCertificates[0]
			if err := leaf.VerifyHostname(host); err != nil || time.Now().Before(leaf.NotBefore) || time.Now().After(leaf.NotAfter) {
				return errors.New("TLS identity validation failed")
			}
			sum := sha256.Sum256(leaf.Raw)
			if "sha256/"+base64.RawURLEncoding.EncodeToString(sum[:]) != want {
				return errors.New("TLS identity validation failed")
			}
			return nil
		}
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
	return &ManagedDaemonProbe{endpoint: fmt.Sprintf("wss://%s", net.JoinHostPort(host, fmt.Sprint(port))), client: client, timeout: 5 * time.Second}, nil
}

func (p *ManagedDaemonProbe) Endpoint() string {
	if p == nil {
		return ""
	}
	return p.endpoint
}

func (p *ManagedDaemonProbe) Probe(parent context.Context) (string, error) {
	if p == nil || p.client == nil {
		return "", &ResolveError{Class: FailureAvailability}
	}
	ctx, cancel := context.WithTimeout(parent, p.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https"+strings.TrimPrefix(p.endpoint, "wss")+"/version", nil)
	if err != nil {
		return "", &ResolveError{Class: FailureProtocol}
	}
	response, err := p.client.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", &ResolveError{Class: FailureAvailability}
		}
		return "", &ResolveError{Class: FailureAvailability}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", &ResolveError{Class: FailureAvailability}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 {
		return "", &ResolveError{Class: FailureProtocol}
	}
	var value string
	if json.Unmarshal(body, &value) != nil {
		var object struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(body, &object) != nil {
			return "", &ResolveError{Class: FailureProtocol}
		}
		value = object.Version
	}
	if value == "" {
		return "", &ResolveError{Class: FailureProtocol}
	}
	return value, nil
}
