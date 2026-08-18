package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/source"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var ErrUnknownHostKey = errors.New("SSH host key fingerprint is not configured")
var ErrHostKeyCaptured = errors.New("SSH host key captured; stop before authentication")
var ErrProbeDeadlineRequired = errors.New("SSH host-key inspection requires a context deadline")

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
	ssh  *ssh.Client
	sftp *sftp.Client
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
	client := &Client{ssh: sshClient, sftp: sftpClient}
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

func (c *Client) SFTP() *sftp.Client { return c.sftp }

func (c *Client) WorkingDirectory() (string, error) { return c.sftp.Getwd() }
func (c *Client) MkdirAll(path string) error        { return c.sftp.MkdirAll(path) }
func (c *Client) Chmod(path string, mode os.FileMode) error {
	return c.sftp.Chmod(path, mode)
}
func (c *Client) Stat(path string) (os.FileInfo, error) { return c.sftp.Stat(path) }
func (c *Client) OpenRead(path string) (io.ReadCloser, error) {
	return c.sftp.Open(path)
}
func (c *Client) OpenWriteExclusive(path string) (io.WriteCloser, error) {
	return c.sftp.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
}
func (c *Client) Rename(oldPath, newPath string) error { return c.sftp.Rename(oldPath, newPath) }
func (c *Client) Remove(path string) error             { return c.sftp.Remove(path) }

type Channel interface {
	io.Reader
	io.Writer
	io.Closer
}

type sessionChannel struct {
	stdin  io.WriteCloser
	stdout io.Reader
	sess   *ssh.Session
}

func (c *sessionChannel) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *sessionChannel) Write(p []byte) (int, error) { return c.stdin.Write(p) }
func (c *sessionChannel) Close() error {
	_ = c.stdin.Close()
	return c.sess.Close()
}

func (c *Client) StartMapepire(ctx context.Context, javaHome, remoteJar string) (Channel, error) {
	command, err := mapepire.JavaCommand(javaHome, remoteJar)
	if err != nil {
		return nil, err
	}
	session, err := c.ssh.NewSession()
	if err != nil {
		return nil, err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	session.Stderr = io.Discard
	if err := session.Start(command); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("Mapepire launch failed: %w", err)
	}
	channel := &sessionChannel{stdin: stdin, stdout: stdout, sess: session}
	go func() {
		<-ctx.Done()
		_ = channel.Close()
	}()
	return channel, nil
}

func (c *Client) CopyToUTF8(ctx context.Context, qsysPath, temporary string) error {
	command, err := source.BuildCopyCommand(qsysPath, temporary)
	if err != nil {
		return err
	}
	session, err := c.ssh.NewSession()
	if err != nil {
		return err
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
