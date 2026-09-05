package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/mapepire/sshstdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/source"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var ErrUnknownHostKey = errors.New("SSH host key fingerprint is not configured")
var ErrHostKeyCaptured = errors.New("SSH host key captured; stop before authentication")
var ErrProbeDeadlineRequired = errors.New("SSH host-key inspection requires a context deadline")
var ErrHostKeyChanged = errors.New("host_key_changed")

const sftpStatusNoSuchFile = 2

type HostKeyObservation struct {
	Algorithm      string               `json:"algorithm"`
	Fingerprint    string               `json:"fingerprint"`
	Verified       bool                 `json:"verified"`
	TrustCandidate profile.HostKeyTrust `json:"trustCandidate"`
}

type HostKeyProbeFailure string

const (
	HostKeyProbeTimeout     HostKeyProbeFailure = "timeout"
	HostKeyProbeNegotiation HostKeyProbeFailure = "algorithm_negotiation"
	HostKeyProbeNoKey       HostKeyProbeFailure = "no_host_key_observed"
)

type HostKeyProbeError struct {
	Kind           HostKeyProbeFailure
	AlgorithmClass string
}

func (e *HostKeyProbeError) Error() string {
	switch e.Kind {
	case HostKeyProbeTimeout:
		return "SSH host-key inspection timed out"
	case HostKeyProbeNegotiation:
		if e.AlgorithmClass != "" {
			return "SSH host-key inspection found no mutually supported " + e.AlgorithmClass + " algorithm; weak algorithms will not be enabled"
		}
		return "SSH host-key inspection found no mutually supported secure algorithm; weak algorithms will not be enabled"
	default:
		return "SSH host-key inspection ended before a host key was observed"
	}
}

type contextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type clientHandshake func(net.Conn, string, *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error)

func secureClientConfig(user string, auth []ssh.AuthMethod, hostKeyCallback ssh.HostKeyCallback) *ssh.ClientConfig {
	algorithms := ssh.SupportedAlgorithms()
	return &ssh.ClientConfig{
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		User:              user,
		Auth:              auth,
		HostKeyCallback:   hostKeyCallback,
		HostKeyAlgorithms: algorithms.HostKeys,
	}
}

func InspectHostKey(ctx context.Context, host string, port int) (HostKeyObservation, error) {
	return inspectHostKey(ctx, host, port, &net.Dialer{}, ssh.NewClientConn)
}

func inspectHostKey(ctx context.Context, host string, port int, dialer contextDialer, handshake clientHandshake) (HostKeyObservation, error) {
	if err := profile.ValidateEndpoint(host, port); err != nil {
		return HostKeyObservation{}, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return HostKeyObservation{}, ErrProbeDeadlineRequired
	}
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return HostKeyObservation{}, classifyHostKeyProbeError(ctx, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return HostKeyObservation{}, &HostKeyProbeError{Kind: HostKeyProbeNoKey}
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	observation := HostKeyObservation{}
	config := secureClientConfig("nexus-host-key-probe", nil, func(_ string, _ net.Addr, key ssh.PublicKey) error {
		observation = HostKeyObservation{
			Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key),
			Verified: false, TrustCandidate: profile.HostKeyTrustTOFU,
		}
		return ErrHostKeyCaptured
	})
	sshConn, _, _, err := handshake(conn, address, config)
	if sshConn != nil {
		_ = sshConn.Close()
	}
	if err != nil && errors.Is(err, ErrHostKeyCaptured) && observation.Fingerprint != "" {
		return observation, nil
	}
	return HostKeyObservation{}, classifyHostKeyProbeError(ctx, err)
}

func classifyHostKeyProbeError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return &HostKeyProbeError{Kind: HostKeyProbeTimeout}
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return &HostKeyProbeError{Kind: HostKeyProbeTimeout}
	}
	var negotiation *ssh.AlgorithmNegotiationError
	if errors.As(err, &negotiation) {
		return &HostKeyProbeError{Kind: HostKeyProbeNegotiation, AlgorithmClass: sanitizeAlgorithmClass(negotiation.What)}
	}
	return &HostKeyProbeError{Kind: HostKeyProbeNoKey}
}

func sanitizeAlgorithmClass(value string) string {
	switch value {
	case "key exchange", "host key", "cipher", "MAC":
		return value
	case "client to server cipher", "server to client cipher":
		return "cipher"
	case "client to server MAC", "server to client MAC":
		return "MAC"
	case "client to server compression", "server to client compression":
		return "compression"
	default:
		return ""
	}
}

type HostKeyMismatchError struct {
	Expected string
	Actual   string
}

func (e *HostKeyMismatchError) Error() string { return "SSH host-key fingerprint mismatch" }

func FingerprintCallback(expected string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if expected == "" {
			return ErrUnknownHostKey
		}
		actual := ssh.FingerprintSHA256(key)
		if actual != expected {
			return &HostKeyMismatchError{Expected: expected, Actual: actual}
		}
		return nil
	}
}

type SecretPrompt struct {
	Input      *os.File
	Output     io.Writer
	IsTerminal func(int) bool
	Read       func(int) ([]byte, error)
}

// PromptCode is the secret-free result of terminal-only credential capture.
// It is safe for a Bubble Tea command result, audit record, or user feedback.
type PromptCode string

const (
	PromptCaptured            PromptCode = "captured"
	PromptTerminalUnavailable PromptCode = "terminal_unavailable"
	PromptEOF                 PromptCode = "eof"
	PromptCancelled           PromptCode = "cancelled"
	PromptUnavailable         PromptCode = "unavailable"
)

// Capture validates the terminal boundary before calling the injected reader.
// It returns owned bytes only on success; every rejected reader buffer is zeroed.
func (p SecretPrompt) Capture(ctx context.Context, input, output *os.File, label string) ([]byte, PromptCode) {
	if ctx == nil || ctx.Err() != nil {
		return nil, PromptCancelled
	}
	if input == nil || output == nil || p.Input != input || p.Output != output || p.IsTerminal == nil || p.Read == nil || !p.IsTerminal(int(input.Fd())) || !p.IsTerminal(int(output.Fd())) {
		return nil, PromptTerminalUnavailable
	}
	if _, err := fmt.Fprint(output, label); err != nil {
		return nil, PromptUnavailable
	}
	secret, err := p.Read(int(input.Fd()))
	if err != nil || ctx.Err() != nil || len(secret) == 0 {
		Zero(secret)
		if errors.Is(err, io.EOF) {
			return nil, PromptEOF
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return nil, PromptCancelled
		}
		return nil, PromptUnavailable
	}
	return secret, PromptCaptured
}

type PasswordPrompt = SecretPrompt

func TerminalSecretPrompt() SecretPrompt {
	return SecretPrompt{Input: os.Stdin, Output: os.Stderr, IsTerminal: term.IsTerminal, Read: term.ReadPassword}
}

func TerminalPasswordPrompt() SecretPrompt { return TerminalSecretPrompt() }

func (p SecretPrompt) Prompt(label string) ([]byte, error) {
	if p.Input == nil || p.Output == nil || p.IsTerminal == nil || p.Read == nil {
		return nil, errors.New("secret prompt is not fully configured")
	}
	fd := int(p.Input.Fd())
	if !p.IsTerminal(fd) {
		return nil, errors.New("secret input requires a real terminal")
	}
	if _, err := fmt.Fprintf(p.Output, "%s (input hidden): ", label); err != nil {
		return nil, err
	}
	password, err := p.Read(fd)
	_, _ = fmt.Fprintln(p.Output)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	if len(password) == 0 {
		return nil, errors.New("secret cannot be empty")
	}
	return password, nil
}

func Zero(bytes []byte) {
	for i := range bytes {
		bytes[i] = 0
	}
}

type Client struct {
	ssh          *ssh.Client
	sftp         *sftp.Client
	hostIdentity string
	newSession   func() (mapepireSession, error)
}

// recoveryRemote exposes only the exact cleanup operations required by
// ownership recovery; callers cannot use it as a general SSH or SFTP client.
type recoveryRemote struct{ client *Client }

var _ source.RecoveryRemote = (*recoveryRemote)(nil)

// NewRecoveryRemote adapts one authenticated client to exact ownership cleanup.
func NewRecoveryRemote(client *Client) source.RecoveryRemote {
	return &recoveryRemote{client: client}
}

func (r *recoveryRemote) Remove(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || r.client == nil {
		return errors.New("ownership recovery client is unavailable")
	}
	return normalizeSourceNotFound(sanitizeSourceSFTPError(r.client.remove(path)))
}

func (r *recoveryRemote) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errors.New("ownership recovery client is unavailable")
	}
	info, err := r.client.stat(path)
	return info, normalizeSourceNotFound(sanitizeSourceSFTPError(err))
}

func (r *recoveryRemote) Close() error {
	if r == nil {
		return nil
	}
	return r.client.Close()
}

// sourceAcquisitionRemote adapts a dialed Client to the bounded source acquisition contract.
// It is intentionally private so callers receive only source.AcquisitionRemote.
type sourceAcquisitionRemote struct {
	client      *Client
	home        string
	reservation sourceTemporaryReservation
}

type sourceTemporaryReservation struct {
	home     string
	path     string
	reserved bool
}

var _ source.AcquisitionRemote = (*sourceAcquisitionRemote)(nil)

func NewSourceAcquisitionRemote(client *Client) source.AcquisitionRemote {
	return &sourceAcquisitionRemote{client: client}
}

// AcquireSource retrieves one validated source member through the legacy
// bounded result shape without exposing file or command primitives.
func (c *Client) AcquireSource(ctx context.Context, candidate catalog.Candidate, maxBytes, maxLines int) (source.Result, error) {
	if err := ctx.Err(); err != nil {
		return source.Result{}, err
	}
	adapter := &sourceRetrievalRemote{client: c}
	return source.Retriever{Files: adapter, Runner: adapter}.Retrieve(ctx, candidate, maxBytes, maxLines)
}

// EnsureMapepireServerJAR activates the fixed verified Mapepire artifact and
// returns its immutable receipt without exposing its remote-files capability.
func (c *Client) EnsureMapepireServerJAR(ctx context.Context, verifiedLocalPath string) (mapepirestdio.VerifiedMapepireArtifactReceipt, error) {
	if err := ctx.Err(); err != nil {
		return mapepirestdio.VerifiedMapepireArtifactReceipt{}, err
	}
	receipt, err := mapepirestdio.EnsureServerJAR(mapepireArtifactRemote{client: c}, verifiedLocalPath)
	if err != nil && ctx.Err() != nil {
		return mapepirestdio.VerifiedMapepireArtifactReceipt{}, ctx.Err()
	}
	return receipt, err
}

type sourceRetrievalRemote struct{ client *Client }

func (r *sourceRetrievalRemote) Stat(path string) (os.FileInfo, error) { return r.client.stat(path) }
func (r *sourceRetrievalRemote) OpenRead(path string) (io.ReadCloser, error) {
	return r.client.openRead(path)
}
func (r *sourceRetrievalRemote) Remove(path string) error { return r.client.remove(path) }
func (r *sourceRetrievalRemote) CopyToUTF8(ctx context.Context, qsysPath, temporary string) error {
	command, err := source.BuildCopyCommand(qsysPath, temporary)
	if err != nil {
		return errors.New("fixed IBM i command is invalid")
	}
	return r.client.runFixedSourceCopy(ctx, command)
}

func (r *sourceAcquisitionRemote) AuthenticatedHome(ctx context.Context) (string, error) {
	if err := r.ready(ctx); err != nil {
		return "", err
	}
	home, err := r.client.workingDirectory()
	if err = sanitizeSourceSFTPError(err); err != nil {
		return "", err
	}
	r.home = home
	return home, nil
}
func (r *sourceAcquisitionRemote) PreparePrivateDirectory(ctx context.Context, directory string) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	if !r.ownsPrivateDirectory(directory) {
		return errors.New("Nexus source temporary is invalid")
	}
	if err := sanitizeSourceSFTPError(r.client.mkdirAll(directory)); err != nil {
		return err
	}
	return sanitizeSourceSFTPError(r.client.chmod(directory, 0o700))
}
func (r *sourceAcquisitionRemote) CreateExclusive(ctx context.Context, path string) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	if !r.ownsTemporaryPath(path) {
		return errors.New("Nexus source temporary is invalid")
	}
	file, err := r.client.openWriteExclusive(path)
	if err = sanitizeSourceSFTPError(err); err != nil {
		return err
	}
	if err := sanitizeSourceSFTPError(file.Close()); err != nil {
		return err
	}
	if err := sanitizeSourceSFTPError(r.client.chmod(path, 0o600)); err != nil {
		return err
	}
	r.reservation = sourceTemporaryReservation{home: r.home, path: path, reserved: true}
	return nil
}
func (r *sourceAcquisitionRemote) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := r.ready(ctx); err != nil {
		return nil, err
	}
	info, err := r.client.lstat(path)
	return info, sanitizeSourceSFTPError(err)
}
func (r *sourceAcquisitionRemote) CopySourceMember(ctx context.Context, candidate catalog.Candidate, temporary string) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	if r.reservation.path != temporary {
		return errors.New("Nexus source temporary is invalid")
	}
	command, err := fixedSourceCopyCommand(candidate, r.reservation)
	if err != nil {
		return err
	}
	return r.client.runFixedSourceCopy(ctx, command)
}
func (r *sourceAcquisitionRemote) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := r.ready(ctx); err != nil {
		return nil, err
	}
	info, err := r.client.stat(path)
	return info, normalizeSourceNotFound(sanitizeSourceSFTPError(err))
}

func (r *sourceAcquisitionRemote) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := r.ready(ctx); err != nil {
		return nil, err
	}
	file, err := r.client.openRead(path)
	if err = normalizeSourceNotFound(sanitizeSourceSFTPError(err)); err != nil {
		return nil, err
	}
	return newSourceDownload(ctx, file), nil
}

func (r *sourceAcquisitionRemote) Remove(ctx context.Context, path string) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	return normalizeSourceNotFound(sanitizeSourceSFTPError(r.client.remove(path)))
}

func (r *sourceAcquisitionRemote) ownsPrivateDirectory(directory string) bool {
	return r != nil && r.home != "" && path.IsAbs(r.home) && path.Clean(r.home) == r.home && directory == path.Join(r.home, ".bac-nexus", "tmp")
}

func (r *sourceAcquisitionRemote) ownsTemporaryPath(temporary string) bool {
	return r.ownsPrivateDirectory(path.Dir(temporary)) && strings.HasSuffix(temporary, ".utf8") && path.Base(temporary) != ".utf8"
}

func (r *sourceAcquisitionRemote) ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || r.client == nil {
		return errors.New("source acquisition client is required")
	}
	return nil
}

func normalizeSourceNotFound(err error) error {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, source.ErrRemoteNotFound) {
		return source.ErrRemoteNotFound
	}
	return err
}

func sanitizeSourceSFTPError(err error) error {
	if err == nil {
		return nil
	}
	var status *sftp.StatusError
	if errors.Is(err, os.ErrNotExist) || (errors.As(err, &status) && status.Code == sftpStatusNoSuchFile) {
		return source.ErrRemoteNotFound
	}
	return errors.New("source transfer failed")
}

func fixedSourceCopyCommand(candidate catalog.Candidate, reservation sourceTemporaryReservation) (string, error) {
	qsysPath, err := candidate.QSYSPath()
	if err != nil {
		return "", errors.New("approved source selection is invalid")
	}
	if !reservation.reserved || !path.IsAbs(reservation.home) || path.Clean(reservation.home) != reservation.home || path.Dir(reservation.path) != path.Join(reservation.home, ".bac-nexus", "tmp") || path.Clean(reservation.path) != reservation.path || !strings.HasSuffix(reservation.path, ".utf8") {
		return "", errors.New("Nexus source temporary is invalid")
	}
	command, err := source.BuildCopyCommand(qsysPath, reservation.path)
	if err != nil {
		return "", errors.New("Nexus source temporary is invalid")
	}
	return command, nil
}

type sourceDownload struct {
	file io.ReadCloser
	done chan struct{}
	once sync.Once
}

func newSourceDownload(ctx context.Context, file io.ReadCloser) io.ReadCloser {
	download := &sourceDownload{file: file, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = download.Close()
		case <-download.done:
		}
	}()
	return download
}

func (d *sourceDownload) Read(p []byte) (int, error) { return d.file.Read(p) }

func (d *sourceDownload) Close() error {
	var err error
	d.once.Do(func() {
		close(d.done)
		err = d.file.Close()
	})
	return err
}

func Dial(ctx context.Context, p profile.Profile, password []byte) (*Client, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	address := net.JoinHostPort(p.Host, fmt.Sprintf("%d", p.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("SSH dial failed: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = conn.Close()
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}
	handshakeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-handshakeDone:
		}
	}()
	config := secureClientConfig(
		p.Username,
		[]ssh.AuthMethod{ssh.Password(string(password))},
		FingerprintCallback(p.HostKeyFingerprint),
	)
	sshConn, channels, requests, err := ssh.NewClientConn(conn, address, config)
	close(handshakeDone)
	if err != nil {
		return nil, fmt.Errorf("SSH handshake failed: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = sshConn.Close()
		return nil, err
	}
	sshClient := ssh.NewClient(sshConn, channels, requests)
	sftpClient, err := sftp.NewClient(sshClient, sftp.UseConcurrentReads(false), sftp.UseConcurrentWrites(false))
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("SFTP startup failed: %w", err)
	}
	client := &Client{ssh: sshClient, sftp: sftpClient, hostIdentity: p.HostKeyFingerprint}
	keep = true
	go func() {
		<-ctx.Done()
		_ = client.Close()
	}()
	return client, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if c.sftp != nil {
		_ = c.sftp.Close()
	}
	if c.ssh != nil {
		return c.ssh.Close()
	}
	return nil
}

func (c *Client) workingDirectory() (string, error) { return c.sftp.Getwd() }
func (c *Client) mkdirAll(path string) error        { return c.sftp.MkdirAll(path) }
func (c *Client) chmod(path string, mode os.FileMode) error {
	return c.sftp.Chmod(path, mode)
}
func (c *Client) stat(path string) (os.FileInfo, error)  { return c.sftp.Stat(path) }
func (c *Client) lstat(path string) (os.FileInfo, error) { return c.sftp.Lstat(path) }
func (c *Client) openRead(path string) (io.ReadCloser, error) {
	return c.sftp.Open(path)
}
func (c *Client) openWriteExclusive(path string) (io.WriteCloser, error) {
	return c.sftp.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
}
func (c *Client) rename(oldPath, newPath string) error { return c.sftp.Rename(oldPath, newPath) }
func (c *Client) remove(path string) error             { return c.sftp.Remove(path) }

type mapepireArtifactRemote struct{ client *Client }

func (r mapepireArtifactRemote) WorkingDirectory() (string, error) {
	return r.client.workingDirectory()
}
func (r mapepireArtifactRemote) MkdirAll(path string) error { return r.client.mkdirAll(path) }
func (r mapepireArtifactRemote) Chmod(path string, mode os.FileMode) error {
	return r.client.chmod(path, mode)
}
func (r mapepireArtifactRemote) Stat(path string) (os.FileInfo, error) { return r.client.stat(path) }
func (r mapepireArtifactRemote) OpenRead(path string) (io.ReadCloser, error) {
	return r.client.openRead(path)
}
func (r mapepireArtifactRemote) OpenWriteExclusive(path string) (io.WriteCloser, error) {
	return r.client.openWriteExclusive(path)
}
func (r mapepireArtifactRemote) Rename(oldPath, newPath string) error {
	return r.client.rename(oldPath, newPath)
}
func (r mapepireArtifactRemote) Remove(path string) error     { return r.client.remove(path) }
func (r mapepireArtifactRemote) MapepireHostIdentity() string { return r.client.hostIdentity }

type Channel interface {
	io.Reader
	io.Writer
	io.Closer
}

type FixedProofMetadata struct {
	Rows     int
	Revision string
}

type FixedProofFailure string

const (
	FixedProofUnknownFailure  FixedProofFailure = "unknown"
	FixedProofProtocolFailure FixedProofFailure = "protocol"
	FixedProofFramingFailure  FixedProofFailure = "framing"
	FixedProofLimitFailure    FixedProofFailure = "limit"
)

var ErrFixedProofSession = errors.New("Mapepire fixed proof session unavailable")

type FixedProofStage string

const (
	FixedProofLaunchStage  FixedProofStage = "launch"
	FixedProofSessionStage FixedProofStage = "session"
	FixedProofRunStage     FixedProofStage = "proof"
)

type FixedProofError struct {
	Stage FixedProofStage
	Err   error
}

func (e *FixedProofError) Error() string { return "Mapepire fixed proof failed" }
func (e *FixedProofError) Unwrap() error { return e.Err }

func FixedProofStageFor(err error) FixedProofStage {
	var proofErr *FixedProofError
	if errors.As(err, &proofErr) {
		return proofErr.Stage
	}
	if errors.Is(err, ErrFixedProofSession) {
		return FixedProofSessionStage
	}
	return ""
}

func ClassifyFixedProofError(err error) FixedProofFailure {
	if errors.Is(err, mapepire.ErrLimitExceeded) || errors.Is(err, sshstdio.ErrFrameLimit) {
		return FixedProofLimitFailure
	}
	if errors.Is(err, sshstdio.ErrInvalidFrame) {
		return FixedProofFramingFailure
	}
	if errors.Is(err, mapepire.ErrProtocolViolation) {
		return FixedProofProtocolFailure
	}
	return FixedProofUnknownFailure
}

type sessionChannel struct {
	stdin   io.WriteCloser
	stdout  io.Reader
	sess    mapepireSession
	done    chan struct{}
	watcher <-chan struct{}
	close   sync.Once
}

type mapepireSession interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	Start(string) error
	Close() error
	SetStderr(io.Writer)
}

type sshMapepireSession struct{ *ssh.Session }

func (s sshMapepireSession) SetStderr(writer io.Writer) { s.Stderr = writer }

type MapepireLaunchStage string

const (
	MapepireLaunchSession MapepireLaunchStage = "session"
	MapepireLaunchStdin   MapepireLaunchStage = "stdin"
	MapepireLaunchStdout  MapepireLaunchStage = "stdout"
	MapepireLaunchStart   MapepireLaunchStage = "start"
)

type MapepireLaunchError struct{ Stage MapepireLaunchStage }

func (e *MapepireLaunchError) Error() string {
	return "Mapepire SSH launch unavailable: " + string(e.Stage)
}

// MapepireLaunchStageFor returns only an allowlisted launch diagnostic stage.
func MapepireLaunchStageFor(err error) MapepireLaunchStage {
	var launchErr *MapepireLaunchError
	if errors.As(err, &launchErr) && validMapepireLaunchStage(launchErr.Stage) {
		return launchErr.Stage
	}
	return ""
}

func validMapepireLaunchStage(stage MapepireLaunchStage) bool {
	switch stage {
	case MapepireLaunchSession, MapepireLaunchStdin, MapepireLaunchStdout, MapepireLaunchStart:
		return true
	}
	return false
}

func (c *sessionChannel) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *sessionChannel) Write(p []byte) (int, error) { return c.stdin.Write(p) }
func (c *sessionChannel) Close() error {
	var err error
	c.close.Do(func() {
		close(c.done)
		_ = c.stdin.Close()
		err = c.sess.Close()
	})
	return err
}

func closeOnCancellation(ctx context.Context, done <-chan struct{}, closeProcess func()) <-chan struct{} {
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		select {
		case <-ctx.Done():
			closeProcess()
		case <-done:
		}
	}()
	return finished
}

func (c *Client) mapepireSession() (mapepireSession, error) {
	if c != nil && c.newSession != nil {
		return c.newSession()
	}
	if c == nil || c.ssh == nil {
		return nil, errors.New("SSH session client is unavailable")
	}
	session, err := c.ssh.NewSession()
	if err != nil {
		return nil, err
	}
	return sshMapepireSession{session}, nil
}

func (c *Client) StartMapepire(ctx context.Context, receipt mapepirestdio.VerifiedMapepireArtifactReceipt) (Channel, error) {
	if c == nil || c.hostIdentity == "" {
		return nil, errors.New("Mapepire SSH launch identity is invalid")
	}
	var launch sshstdio.Launch
	if err := receipt.AdmitFixedStart(mapepireArtifactRemote{client: c}, func() error {
		var renderErr error
		launch, renderErr = sshstdio.VerifiedSingleMode(receipt)
		return renderErr
	}); err != nil {
		return nil, &MapepireLaunchError{Stage: MapepireLaunchSession}
	}
	return c.startMapepireSession(ctx, launch.Command)
}

func (c *Client) startMapepireSession(ctx context.Context, command string) (Channel, error) {
	session, err := c.mapepireSession()
	if err != nil {
		return nil, &MapepireLaunchError{Stage: MapepireLaunchSession}
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, &MapepireLaunchError{Stage: MapepireLaunchStdin}
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		_ = session.Close()
		return nil, &MapepireLaunchError{Stage: MapepireLaunchStdout}
	}
	session.SetStderr(io.Discard)
	if err := session.Start(command); err != nil {
		_ = stdin.Close()
		_ = session.Close()
		return nil, &MapepireLaunchError{Stage: MapepireLaunchStart}
	}
	channel := &sessionChannel{stdin: stdin, stdout: stdout, sess: session, done: make(chan struct{})}
	channel.watcher = closeOnCancellation(ctx, channel.done, func() { _ = channel.Close() })
	return channel, nil
}

// StartMapepireTransport exposes only the typed transport boundary to callers.
func (c *Client) StartMapepireTransport(ctx context.Context, receipt mapepirestdio.VerifiedMapepireArtifactReceipt) (mapepire.MessageTransport, error) {
	channel, err := c.StartMapepire(ctx, receipt)
	if err != nil {
		return nil, err
	}
	return sshstdio.New(channel), nil
}

// FixedMapepireProof exposes only the release-owned typed proof lifecycle.
func (c *Client) FixedMapepireProof(ctx context.Context, receipt mapepirestdio.VerifiedMapepireArtifactReceipt, username string, password []byte) (FixedProofMetadata, error) {
	transport, err := c.StartMapepireTransport(ctx, receipt)
	if err != nil {
		return FixedProofMetadata{}, &FixedProofError{Stage: FixedProofLaunchStage, Err: err}
	}
	if transport == nil {
		return FixedProofMetadata{}, &FixedProofError{Stage: FixedProofSessionStage, Err: ErrFixedProofSession}
	}
	proof, err := mapepire.NewMessageSession(transport).FixedProof(ctx, username, password)
	if err != nil {
		return FixedProofMetadata{}, &FixedProofError{Stage: FixedProofRunStage, Err: err}
	}
	return FixedProofMetadata{Rows: proof.Rows, Revision: proof.Revision}, nil
}

func (c *Client) runFixedSourceCopy(ctx context.Context, command string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.ssh == nil {
		return errors.New("fixed IBM i command unavailable")
	}
	session, err := c.ssh.NewSession()
	if err != nil {
		return errors.New("fixed IBM i command unavailable")
	}
	defer session.Close()
	session.Stdout = io.Discard
	session.Stderr = io.Discard
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return errors.New("fixed IBM i command failed")
		}
		return nil
	}
}
