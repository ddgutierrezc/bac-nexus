package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"bac-nexus/internal/profile"
)

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestFingerprintCallbackFailsClosed(t *testing.T) {
	key := testPublicKey(t)
	address := &net.TCPAddr{}
	tests := []struct {
		name     string
		expected string
		wantErr  error
		mismatch bool
	}{
		{"match", ssh.FingerprintSHA256(key), nil, false},
		{"unknown", "", ErrUnknownHostKey, false},
		{"mismatch", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FingerprintCallback(tt.expected)("host:22", address, key)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			var mismatch *HostKeyMismatchError
			if errors.As(err, &mismatch) != tt.mismatch {
				t.Fatalf("mismatch error = %v, want %v", err, tt.mismatch)
			}
			if tt.wantErr == nil && !tt.mismatch && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInspectHostKeyCapturesDuringKEXBeforeAuthentication(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	var passwordAttempts atomic.Int32
	var authenticationAttempts atomic.Int32
	serverConfig := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
		passwordAttempts.Add(1)
		return nil, errors.New("authentication rejected")
	}}
	serverConfig.AuthLogCallback = func(ssh.ConnMetadata, string, error) { authenticationAttempts.Add(1) }
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_, _, _, _ = ssh.NewServerConn(conn, serverConfig)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	port := listener.Addr().(*net.TCPAddr).Port
	observation, err := InspectHostKey(ctx, "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Algorithm != ssh.KeyAlgoED25519 || observation.Fingerprint != ssh.FingerprintSHA256(signer.PublicKey()) || observation.Verified || observation.TrustCandidate != "tofu" {
		t.Fatalf("observation = %#v", observation)
	}
	<-serverDone
	if passwordAttempts.Load() != 0 {
		t.Fatalf("password authentication attempts = %d, want 0", passwordAttempts.Load())
	}
	if authenticationAttempts.Load() != 0 {
		t.Fatalf("authentication attempts = %d, want 0", authenticationAttempts.Load())
	}
}

func TestPinnedFingerprintMismatchFailsClosedForEveryTrustProvenance(t *testing.T) {
	key := testPublicKey(t)
	for _, trust := range []profile.HostKeyTrust{profile.HostKeyTrustTOFU, profile.HostKeyTrustVerified} {
		t.Run(string(trust), func(t *testing.T) {
			p := profile.Profile{
				Name: "dev", Host: "ibmi.example.test", Port: 22, Username: "USER",
				HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				HostKeyTrust:       trust, CredentialMode: profile.CredentialModePrompt,
			}
			if err := p.Validate(); err != nil {
				t.Fatal(err)
			}
			err := FingerprintCallback(p.HostKeyFingerprint)("ignored", &net.TCPAddr{}, key)
			var mismatch *HostKeyMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("error = %v, want host-key mismatch", err)
			}
		})
	}
}

func TestHostKeyProbeUsesOnlySupportedSecureAlgorithmsAndNoAuth(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	dialer := dialerFunc(func(context.Context, string, string) (net.Conn, error) { return client, nil })
	var captured *ssh.ClientConfig
	handshake := func(_ net.Conn, _ string, config *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
		captured = config
		return nil, nil, nil, &ssh.AlgorithmNegotiationError{What: "key exchange", RequestedAlgorithms: []string{"peer-only"}, SupportedAlgorithms: config.KeyExchanges}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := inspectHostKey(ctx, "example.test", 22, dialer, handshake)
	var probeError *HostKeyProbeError
	if !errors.As(err, &probeError) || probeError.Kind != HostKeyProbeNegotiation || probeError.AlgorithmClass != "key exchange" {
		t.Fatalf("error = %#v, want sanitized negotiation failure", err)
	}
	if captured == nil || len(captured.Auth) != 0 || captured.User != "nexus-host-key-probe" {
		t.Fatalf("probe config = %#v", captured)
	}
	supported, insecure := ssh.SupportedAlgorithms(), ssh.InsecureAlgorithms()
	assertAlgorithmsEqual(t, "key exchanges", captured.KeyExchanges, supported.KeyExchanges)
	assertAlgorithmsEqual(t, "ciphers", captured.Ciphers, supported.Ciphers)
	assertAlgorithmsEqual(t, "MACs", captured.MACs, supported.MACs)
	assertAlgorithmsEqual(t, "host keys", captured.HostKeyAlgorithms, supported.HostKeys)
	assertNoAlgorithmsOverlap(t, captured.KeyExchanges, insecure.KeyExchanges)
	assertNoAlgorithmsOverlap(t, captured.Ciphers, insecure.Ciphers)
	assertNoAlgorithmsOverlap(t, captured.MACs, insecure.MACs)
	assertNoAlgorithmsOverlap(t, captured.HostKeyAlgorithms, insecure.HostKeys)
}

func TestHostKeyProbeTimeoutClosesConnection(t *testing.T) {
	client, server := net.Pipe()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		defer server.Close()
		buffer := make([]byte, 1)
		_, _ = server.Read(buffer)
	}()
	dialer := dialerFunc(func(context.Context, string, string) (net.Conn, error) { return client, nil })
	handshake := func(conn net.Conn, _ string, _ *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
		buffer := make([]byte, 1)
		_, err := conn.Read(buffer)
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := inspectHostKey(ctx, "example.test", 22, dialer, handshake)
	var probeError *HostKeyProbeError
	if !errors.As(err, &probeError) || probeError.Kind != HostKeyProbeTimeout {
		t.Fatalf("error = %#v, want timeout", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("peer did not observe probe connection closure")
	}
}

func TestHostKeyProbeFailuresAreTypedAndRedacted(t *testing.T) {
	tests := []struct {
		name      string
		failure   error
		wantKind  HostKeyProbeFailure
		wantClass string
	}{
		{"negotiation", &ssh.AlgorithmNegotiationError{What: "host key", RequestedAlgorithms: []string{"sensitive-peer-algorithm"}}, HostKeyProbeNegotiation, "host key"},
		{"no key", errors.New("raw server banner and host details"), HostKeyProbeNoKey, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer server.Close()
			dialer := dialerFunc(func(context.Context, string, string) (net.Conn, error) { return client, nil })
			handshake := func(net.Conn, string, *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
				return nil, nil, nil, tt.failure
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := inspectHostKey(ctx, "secret-host.example", 22, dialer, handshake)
			var probeError *HostKeyProbeError
			if !errors.As(err, &probeError) || probeError.Kind != tt.wantKind || probeError.AlgorithmClass != tt.wantClass {
				t.Fatalf("error = %#v", err)
			}
			for _, forbidden := range []string{"secret-host", "sensitive-peer-algorithm", "raw server banner"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatalf("error exposed %q: %v", forbidden, err)
				}
			}
		})
	}
}

func assertAlgorithmsEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func assertNoAlgorithmsOverlap(t *testing.T, safe, insecure []string) {
	t.Helper()
	for _, value := range insecure {
		for _, candidate := range safe {
			if value == candidate {
				t.Fatalf("insecure algorithm %q is enabled", value)
			}
		}
	}
}

func TestPasswordPromptRequiresTerminal(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	readCalled := false
	prompt := PasswordPrompt{
		Input: input, Output: io.Discard,
		IsTerminal: func(int) bool { return false },
		Read:       func(int) ([]byte, error) { readCalled = true; return nil, nil },
	}
	if _, err := prompt.Prompt("test"); err == nil {
		t.Fatal("expected non-terminal rejection")
	}
	if readCalled {
		t.Fatal("password reader called for non-terminal input")
	}
}

func TestPasswordPromptReadsHiddenTerminalWithoutLoggingValue(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var output strings.Builder
	prompt := PasswordPrompt{
		Input: input, Output: &output,
		IsTerminal: func(int) bool { return true },
		Read:       func(int) ([]byte, error) { return []byte{'x', 'y'}, nil },
	}
	password, err := prompt.Prompt("test-profile")
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(password)
	if len(password) != 2 {
		t.Fatalf("password length = %d", len(password))
	}
	if strings.Contains(output.String(), string(password)) {
		t.Fatal("password value was written to prompt output")
	}
}

func TestPasswordBytesAreZeroed(t *testing.T) {
	value := []byte{'x', 'y'}
	Zero(value)
	if value[0] != 0 || value[1] != 0 {
		t.Fatal("password bytes were not zeroed")
	}
}
