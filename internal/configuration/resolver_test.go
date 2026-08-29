package configuration

import (
	"bac-nexus/internal/profile"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type probeFake struct {
	version string
	err     error
	calls   int
}

func (f *probeFake) Probe(context.Context) (string, error) { f.calls++; return f.version, f.err }

type fallbackFake struct {
	trustErr, startErr     error
	trustCalls, startCalls int
}

func (f *fallbackFake) Trust(context.Context) error { f.trustCalls++; return f.trustErr }
func (f *fallbackFake) Start(context.Context) error { f.startCalls++; return f.startErr }

type transportAuditFake struct{ events []TransportAuditEvent }

func (f *transportAuditFake) RecordTransport(_ context.Context, e TransportAuditEvent) error {
	f.events = append(f.events, e)
	return nil
}

func TestResolverClassifiesDaemonAndNeverDowngradesTerminalFailures(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		want     FailureClass
		fallback int
	}{
		{"timeout", &ResolveError{Class: FailureAvailability}, FailureAvailability, 1},
		{"identity", &ResolveError{Class: FailureIdentity}, FailureIdentity, 0},
		{"credentials", &ResolveError{Class: FailureCredentials}, FailureCredentials, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &probeFake{err: tc.err}
			s := &fallbackFake{}
			got, _ := (Resolver{Daemon: d, SSH: s}).Resolve(context.Background(), ProfilePolicy{DaemonAllowed: true, FallbackAllowed: true, FallbackConsent: true, SSHTrustValid: true})
			if (tc.name == "timeout" && got.FallbackReason != tc.want) || (tc.name != "timeout" && got.Class != tc.want) || s.trustCalls != tc.fallback {
				t.Fatalf("result=%+v trust=%d", got, s.trustCalls)
			}
		})
	}
}

func TestResolverWSSVersionAndFallbackTrustGate(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		transport     Transport
		fallback      bool
		want          FailureClass
	}{
		{"supported", "2.3.5", TransportWSS, false, FailureNone},
		{"unsupported fallback", "2.2.0", TransportSSH, true, FailureNone},
		{"missing trust", "", TransportSSH, true, FailureTrust},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &probeFake{version: tc.version}
			s := &fallbackFake{trustErr: errors.New("untrusted")}
			p := ProfilePolicy{FallbackAllowed: tc.fallback, FallbackConsent: true, SSHTrustValid: tc.name != "missing trust"}
			got, _ := (Resolver{Daemon: d, SSH: s}).Resolve(context.Background(), p)
			if got.Transport != tc.transport || (tc.name == "missing trust" && got.Class != tc.want) {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestResolverConsentCredentialTerminalityAndSanitizedAudit(t *testing.T) {
	a := &transportAuditFake{}
	d := &probeFake{err: &ResolveError{Class: FailureAvailability}}
	s := &fallbackFake{}
	got, _ := (Resolver{Daemon: d, SSH: s, Audit: a}).Resolve(context.Background(), ProfilePolicy{FallbackAllowed: true, SSHTrustValid: true})
	if got.Class != FailureConsent || s.startCalls != 0 {
		t.Fatalf("result=%+v starts=%d", got, s.startCalls)
	}
	if len(a.events) != 1 || a.events[0].Transport != string(TransportSSH) || a.events[0].Reason != string(FailureConsent) {
		t.Fatalf("audit=%+v", a.events)
	}
	if err := (&ResolveError{Class: FailureCredentials}).Error(); err != "credential failure" {
		t.Fatal(err)
	}
}

func TestLocalReadinessIsOfflineAndExplicit(t *testing.T) {
	r := LocalReadiness(ProfilePolicy{FallbackAllowed: true})
	if r.RemoteContacted || r.AuthenticationPending != true || r.Transport != TransportUnknown {
		t.Fatalf("readiness=%+v", r)
	}
}

func TestTrustEnrollmentIsExplicitAndTransportSpecific(t *testing.T) {
	pin := "sha256/" + strings.Repeat("A", 43)
	e, err := EnrollTrust(TransportWSS, profile.TrustModePin, pin, "operator", "enroll "+pin)
	if err != nil || e.Mode != profile.TrustModePin {
		t.Fatalf("evidence=%+v err=%v", e, err)
	}
	if _, err := EnrollTrust(TransportSSH, profile.TrustModeCA, "", "operator", "enroll "); err == nil {
		t.Fatal("SSH accepted TLS CA trust")
	}
}

func TestManagedDaemonProbeUsesBoundedHTTPSVersionEndpoint(t *testing.T) {
	probe, err := NewManagedDaemonProbe("127.0.0.1", 8076, nil)
	if err != nil || probe == nil {
		t.Fatalf("probe=%v err=%v", probe, err)
	}
	if probe.Endpoint() != "wss://127.0.0.1:8076" {
		t.Fatalf("endpoint=%q", probe.Endpoint())
	}
}

func TestManagedDaemonProbeReadsVersionWithoutCredentials(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" || r.Header.Get("Authorization") != "" {
			t.Fatalf("unsafe probe request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"version":"2.3.5"}`))
	}))
	defer server.Close()
	probe, err := NewManagedDaemonProbe("127.0.0.1", 8076, nil)
	if err != nil {
		t.Fatal(err)
	}
	probe.endpoint = strings.Replace(server.URL, "https://", "wss://", 1)
	probe.client = server.Client()
	version, err := probe.Probe(context.Background())
	if err != nil || version != daemonVersion {
		t.Fatalf("version=%q err=%v", version, err)
	}
}
