package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/source"
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
func TestSourceAcquisitionRemoteRejectsCancelledContextBeforeSFTP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewSourceAcquisitionRemote(&Client{}).Stat(ctx, "/tmp/nexus")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Stat() error = %v, want context cancellation", err)
	}
}

func TestClientPublishesOnlyNarrowSourceAndMapepireOperations(t *testing.T) {
	clientType := reflect.TypeFor[*Client]()
	for _, forbidden := range []string{"CopyToUTF8", "MapepireArtifactFiles"} {
		if _, ok := clientType.MethodByName(forbidden); ok {
			t.Fatalf("Client exposes forbidden generic method %q", forbidden)
		}
	}

	var acquire func(*Client, context.Context, catalog.Candidate, int, int) (source.Result, error) = (*Client).AcquireSource
	var ensure func(*Client, context.Context, string) (mapepirestdio.VerifiedMapepireArtifactReceipt, error) = (*Client).EnsureMapepireServerJAR
	if acquire == nil || ensure == nil {
		t.Fatal("Client narrow business operations are unavailable")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquire(&Client{}, ctx, catalog.Candidate{}, source.DefaultMaxBytes, source.DefaultMaxLines); !errors.Is(err, context.Canceled) {
		t.Fatalf("AcquireSource() error = %v, want context cancellation", err)
	}
	if _, err := ensure(&Client{}, ctx, "verified.jar"); !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureMapepireServerJAR() error = %v, want context cancellation", err)
	}
}

func TestFixedSourceCopyCommandAcceptsOnlyValidatedMemberAndNexusTemporary(t *testing.T) {
	candidate := catalog.Candidate{
		Item: "PISA061", SourceLibrary: "APP", SourceFileBase: "QRPG", ObjectType: "LESRC", SourceType: "RPGLE",
	}
	temporary := "/home/NEXUS/.bac-nexus/tmp/0123456789abcdef0123456789abcdef.utf8"

	command, err := fixedSourceCopyCommand(candidate, sourceTemporaryReservation{
		home:     "/home/NEXUS",
		path:     temporary,
		reserved: true,
	})
	if err != nil {
		t.Fatalf("fixedSourceCopyCommand() error = %v", err)
	}
	if want := "'/QOpenSys/usr/bin/system' 'CPYTOSTMF FROMMBR(" + "'\"'\"'" + "/QSYS.LIB/APP.LIB/QRPGLESRC.FILE/PISA061.MBR" + "'\"'\"'" + ") TOSTMF(" + "'\"'\"'" + temporary + "'\"'\"'" + ") STMFOPT(*REPLACE) STMFCCSID(1208)'"; command != want {
		t.Fatalf("fixedSourceCopyCommand() = %q, want %q", command, want)
	}

	unsafe := candidate
	unsafe.Item = "bad member"
	if _, err := fixedSourceCopyCommand(unsafe, sourceTemporaryReservation{}); err == nil || strings.Contains(err.Error(), "bad member") || strings.Contains(err.Error(), "attacker") {
		t.Fatalf("unsafe fixed source copy error = %v, want sanitized rejection", err)
	}
}

func TestFixedSourceCopyCommandRequiresAuthenticatedReservedTemporary(t *testing.T) {
	candidate := catalog.Candidate{Item: "PISA061", SourceLibrary: "APP", SourceFileBase: "QRPG", ObjectType: "LESRC", SourceType: "RPGLE"}
	for _, reservation := range []sourceTemporaryReservation{
		{home: "/home/nexus", path: "/attacker/.bac-nexus/tmp/x.utf8", reserved: true},
		{home: "/home/nexus", path: "/home/nexus/.bac-nexus/tmp/x.utf8"},
	} {
		if _, err := fixedSourceCopyCommand(candidate, reservation); err == nil {
			t.Fatalf("fixedSourceCopyCommand() accepted unowned reservation %#v", reservation)
		}
	}
}

type blockingReadCloser struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	select {
	case <-r.started:
	default:
		close(r.started)
	}
	<-r.closed
	return 0, errors.New("reader closed")
}

func (r *blockingReadCloser) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestSourceDownloadCancellationClosesBlockedReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &blockingReadCloser{started: make(chan struct{}), closed: make(chan struct{})}
	download := newSourceDownload(ctx, reader)
	result := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(download)
		result <- err
	}()
	<-reader.started
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("blocked source reader completed without an error")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not unblock the source reader")
	}
	select {
	case <-reader.closed:
	default:
		t.Fatal("cancellation did not close the blocked source reader")
	}
}

func TestRunFixedSourceCopyReturnsCancellationWithoutCommandDetails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (&Client{}).runFixedSourceCopy(ctx, "secret-host /secret/path")
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("runFixedSourceCopy() error = %v, want sanitized cancellation", err)
	}
}

func TestSanitizeSourceSFTPErrorPreservesOnlyNotFoundClassification(t *testing.T) {
	if err := sanitizeSourceSFTPError(errors.New("secret-host /secret/path")); err == nil || err.Error() != "source transfer failed" {
		t.Fatalf("sanitizeSourceSFTPError(secret) = %v, want deterministic sanitized error", err)
	}
	if err := sanitizeSourceSFTPError(os.ErrNotExist); !errors.Is(err, source.ErrRemoteNotFound) {
		t.Fatalf("sanitizeSourceSFTPError(not found) = %v, want ErrRemoteNotFound", err)
	}
	if err := sanitizeSourceSFTPError(&sftp.StatusError{Code: 2}); !errors.Is(err, source.ErrRemoteNotFound) {
		t.Fatalf("sanitizeSourceSFTPError(SFTP not found) = %v, want ErrRemoteNotFound", err)
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

func TestInspectAndLiveConfigsShareSupportedSecureAlgorithmPolicy(t *testing.T) {
	inspect := secureClientConfig("nexus-host-key-probe", nil, FingerprintCallback("inspect"))
	live := secureClientConfig("USER", []ssh.AuthMethod{ssh.Password("secret")}, FingerprintCallback("live"))
	supported, insecure := ssh.SupportedAlgorithms(), ssh.InsecureAlgorithms()

	policies := []struct {
		name      string
		inspect   []string
		live      []string
		supported []string
		insecure  []string
	}{
		{"host keys", inspect.HostKeyAlgorithms, live.HostKeyAlgorithms, supported.HostKeys, insecure.HostKeys},
		{"key exchanges", inspect.KeyExchanges, live.KeyExchanges, supported.KeyExchanges, insecure.KeyExchanges},
		{"ciphers", inspect.Ciphers, live.Ciphers, supported.Ciphers, insecure.Ciphers},
		{"MACs", inspect.MACs, live.MACs, supported.MACs, insecure.MACs},
	}
	for _, policy := range policies {
		t.Run(policy.name, func(t *testing.T) {
			assertAlgorithmsEqual(t, "inspect and live ordering", policy.inspect, policy.live)
			assertAlgorithmsEqual(t, "supported ordering", policy.live, policy.supported)
			assertNoAlgorithmsOverlap(t, policy.inspect, policy.insecure)
			assertNoAlgorithmsOverlap(t, policy.live, policy.insecure)
		})
	}
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

func TestMapepireLaunchRejectsMismatchedHostIdentityBeforeSessionUse(t *testing.T) {
	client := &Client{hostIdentity: "SHA256:observed-host"}
	if _, err := client.StartMapepire(context.Background(), mapepirestdio.VerifiedMapepireArtifactReceipt{}); err == nil {
		t.Fatal("zero receipt was accepted before session use")
	}
}

func TestCloseOnCancellationStopsWatchingAfterChannelCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	closed := make(chan struct{}, 1)
	watcher := closeOnCancellation(ctx, done, func() { closed <- struct{}{} })
	close(done)
	select {
	case <-watcher:
	case <-time.After(time.Second):
		t.Fatal("cancellation watcher did not stop after channel close")
	}
	select {
	case <-closed:
		t.Fatal("channel close triggered a second process close")
	default:
	}
}

func TestCloseOnCancellationClosesProcessWhenContextCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	closed := make(chan struct{}, 1)
	watcher := closeOnCancellation(ctx, done, func() { closed <- struct{}{} })
	cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not close the process")
	}
	select {
	case <-watcher:
	case <-time.After(time.Second):
		t.Fatal("cancellation watcher did not exit")
	}
}

func TestStartMapepireSessionClosesResources(t *testing.T) {
	session := &fakeMapepireSession{closeObserved: make(chan struct{})}
	client := &Client{newSession: func() (mapepireSession, error) { return session, nil }}
	channel, err := client.startMapepireSession(context.Background(), "verified command")
	if err != nil {
		t.Fatal(err)
	}
	if session.command != "verified command" {
		t.Fatalf("Start command = %q", session.command)
	}
	if err := channel.Close(); err != nil {
		t.Fatal(err)
	}
	if !session.stdin.closed || !session.closed {
		t.Fatalf("resources not closed: stdin=%t session=%t", session.stdin.closed, session.closed)
	}
	select {
	case <-channel.(*sessionChannel).watcher:
	case <-time.After(time.Second):
		t.Fatal("normal close leaked the cancellation watcher")
	}
}

func TestStartMapepireCancellationClosesSessionAndWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	session := &fakeMapepireSession{closeObserved: make(chan struct{})}
	client := &Client{newSession: func() (mapepireSession, error) { return session, nil }}
	channel, err := client.startMapepireSession(ctx, "verified command")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-session.closeObserved:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close the SSH session")
	}
	if err := channel.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-channel.(*sessionChannel).watcher:
	case <-time.After(time.Second):
		t.Fatal("cancellation leaked the watcher")
	}
}

func TestStartMapepireSanitizesFailuresAndClosesPartialResources(t *testing.T) {
	for _, tt := range []struct {
		name string
		make func() *fakeMapepireSession
		want MapepireLaunchStage
	}{
		{"new session", func() *fakeMapepireSession { return nil }, MapepireLaunchNewSessionFailure},
		{"stdin", func() *fakeMapepireSession { return &fakeMapepireSession{stdinErr: errors.New("secret stdin failure")} }, MapepireLaunchStdin},
		{"stdout", func() *fakeMapepireSession {
			return &fakeMapepireSession{stdoutErr: errors.New("secret stdout failure")}
		}, MapepireLaunchStdout},
		{"start", func() *fakeMapepireSession { return &fakeMapepireSession{startErr: errors.New("secret start failure")} }, MapepireLaunchStart},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := tt.make()
			client := &Client{newSession: func() (mapepireSession, error) {
				if session == nil {
					return nil, errors.New("secret session failure")
				}
				return session, nil
			}}
			_, err := client.startMapepireSession(context.Background(), "verified command")
			var launchErr *MapepireLaunchError
			if !errors.As(err, &launchErr) || launchErr.Stage != tt.want || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error = %#v", err)
			}
			if session != nil && !session.closed {
				t.Fatal("failed launch left session open")
			}
		})
	}
}

func TestMapepireLaunchStagesAreClosedAndRedacted(t *testing.T) {
	admissions := []struct {
		stage mapepirestdio.AdmissionStage
		want  MapepireLaunchStage
	}{
		{mapepirestdio.AdmissionReceiptBindingInvalid, MapepireLaunchReceiptBindingInvalid},
		{mapepirestdio.AdmissionReverifyStatFailure, MapepireLaunchReverifyStatFailure},
		{mapepirestdio.AdmissionReverifyArtifactInvalid, MapepireLaunchReverifyArtifactInvalid},
		{mapepirestdio.AdmissionReverifyOpenFailure, MapepireLaunchReverifyOpenFailure},
		{mapepirestdio.AdmissionReverifyReadFailure, MapepireLaunchReverifyReadFailure},
		{mapepirestdio.AdmissionReverifySizeChanged, MapepireLaunchReverifySizeChanged},
		{mapepirestdio.AdmissionReverifyHashMismatch, MapepireLaunchReverifyHashMismatch},
		{mapepirestdio.AdmissionCommandPolicyFailure, MapepireLaunchCommandPolicyFailure},
	}
	for _, tt := range admissions {
		if got := launchStageForAdmission(tt.stage); got != tt.want {
			t.Fatalf("admission %q = %q, want %q", tt.stage, got, tt.want)
		}
	}
	for _, tt := range []struct {
		name string
		err  error
		want MapepireLaunchStage
	}{
		{"prohibited", &ssh.OpenChannelError{Reason: ssh.Prohibited, Message: "untrusted message canary"}, MapepireLaunchNewSessionProhibited},
		{"connection failed", &ssh.OpenChannelError{Reason: ssh.ConnectionFailed, Message: "untrusted message canary"}, MapepireLaunchNewSessionConnectionFailed},
		{"unknown channel", &ssh.OpenChannelError{Reason: ssh.UnknownChannelType, Message: "untrusted message canary"}, MapepireLaunchNewSessionUnknownChannelType},
		{"resource shortage", &ssh.OpenChannelError{Reason: ssh.ResourceShortage, Message: "untrusted message canary"}, MapepireLaunchNewSessionResourceShortage},
		{"unknown reason", &ssh.OpenChannelError{Reason: 99, Message: "untrusted message canary"}, MapepireLaunchNewSessionFailure},
		{"arbitrary", errors.New("untrusted message canary"), MapepireLaunchNewSessionFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := newSessionLaunchError(tt.err)
			if got := MapepireLaunchStageFor(err); got != tt.want || strings.Contains(err.Error(), "untrusted message canary") {
				t.Fatalf("stage/error = %q/%v", got, err)
			}
		})
	}
	if got := launchStageForAdmission("arbitrary"); got != MapepireLaunchFailure {
		t.Fatalf("unknown admission = %q", got)
	}
}

type fakeMapepireSession struct {
	stdin         fakeMapepireStdin
	stdinErr      error
	stdoutErr     error
	startErr      error
	command       string
	closed        bool
	closeObserved chan struct{}
}

func (s *fakeMapepireSession) StdinPipe() (io.WriteCloser, error) { return &s.stdin, s.stdinErr }
func (s *fakeMapepireSession) StdoutPipe() (io.Reader, error) {
	return strings.NewReader(""), s.stdoutErr
}
func (s *fakeMapepireSession) Start(command string) error { s.command = command; return s.startErr }
func (s *fakeMapepireSession) SetStderr(io.Writer)        {}
func (s *fakeMapepireSession) Close() error {
	s.closed = true
	if s.closeObserved != nil {
		select {
		case <-s.closeObserved:
		default:
			close(s.closeObserved)
		}
	}
	return nil
}

type fakeMapepireStdin struct{ closed bool }

func (s *fakeMapepireStdin) Write(p []byte) (int, error) { return len(p), nil }
func (s *fakeMapepireStdin) Close() error                { s.closed = true; return nil }
