package wss

import (
	"bac-nexus/internal/mapepire"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/coder/websocket"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidEndpoint      = errors.New("Mapepire WSS endpoint is invalid")
	ErrInvalidConfiguration = errors.New("Mapepire WSS configuration is invalid")
	ErrIdentityFailure      = errors.New("Mapepire WSS identity validation failed")
	ErrAvailability         = errors.New("Mapepire WSS endpoint unavailable")
	ErrTimeout              = errors.New("Mapepire WSS handshake timed out")
	ErrFrameLimit           = errors.New("Mapepire WSS message exceeds release limit")
	ErrTransportClosed      = errors.New("Mapepire WSS transport is closed")
)

const handshakeTimeout = 5 * time.Second

type Options struct {
	HTTPClient *http.Client
	TLSConfig  *tls.Config
	Pin        string
	TOFU       bool
	ReadLimit  int
}

type Transport struct {
	conn  *websocket.Conn
	read  sync.Mutex
	write sync.Mutex
	once  sync.Once
	limit int
}

func Dial(ctx context.Context, endpoint string, options Options) (*Transport, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "wss" || u.Host == "" || u.User != nil || strings.Contains(strings.ToUpper(endpoint), "MP_UNSECURE") {
		return nil, ErrInvalidEndpoint
	}
	tlsConfig := options.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig = tlsConfig.Clone()
	if tlsConfig.InsecureSkipVerify {
		return nil, ErrInvalidConfiguration
	}
	if options.TOFU && options.Pin == "" || options.Pin != "" && !validPin(options.Pin) {
		return nil, ErrInvalidConfiguration
	}
	if options.Pin != "" {
		previous := tlsConfig.VerifyConnection
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = verifyPin(previous, u.Hostname(), options.Pin)
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	} else {
		if client.Transport != nil {
			if _, ok := client.Transport.(*http.Transport); !ok {
				return nil, ErrInvalidConfiguration
			}
		}
		copy := *client
		client = &copy
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if options.HTTPClient != nil && options.HTTPClient.Transport != nil {
		transport = options.HTTPClient.Transport.(*http.Transport).Clone()
	}
	transport.TLSClientConfig = tlsConfig
	client.Transport = transport
	limit := options.ReadLimit
	if limit == 0 {
		limit = mapepire.MaxFrameBytes
	}
	if limit < 1 || limit > mapepire.MaxFrameBytes {
		return nil, ErrFrameLimit
	}
	dialCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, endpoint, &websocket.DialOptions{HTTPClient: client, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return nil, classifyDialError(dialCtx, err)
	}
	conn.SetReadLimit(int64(limit))
	return &Transport{conn: conn, limit: limit}, nil
}
func validPin(pin string) bool {
	if !strings.HasPrefix(pin, "sha256/") || len(pin) != len("sha256/")+43 {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(pin, "sha256/"))
	return err == nil && len(b) == sha256.Size && "sha256/"+base64.RawURLEncoding.EncodeToString(b) == pin
}
func verifyPin(previous func(tls.ConnectionState) error, hostname, want string) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) == 0 {
			return ErrIdentityFailure
		}
		leaf := state.PeerCertificates[0]
		if err := leaf.VerifyHostname(hostname); err != nil || time.Now().Before(leaf.NotBefore) || time.Now().After(leaf.NotAfter) {
			return ErrIdentityFailure
		}
		h := sha256.Sum256(leaf.Raw)
		if "sha256/"+base64.RawURLEncoding.EncodeToString(h[:]) != want {
			return ErrIdentityFailure
		}
		if previous != nil {
			return previous(state)
		}
		return nil
	}
}
func classifyDialError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return ErrTimeout
	}
	if errors.Is(err, ErrIdentityFailure) {
		return ErrIdentityFailure
	}
	if strings.Contains(err.Error(), "certificate") {
		return ErrIdentityFailure
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return ErrAvailability
	}
	return ErrIdentityFailure
}
func (t *Transport) Send(ctx context.Context, payload []byte) error {
	if len(payload) > t.limit {
		t.terminate()
		return ErrFrameLimit
	}
	if !json.Valid(payload) {
		t.terminate()
		return ErrTransportClosed
	}
	t.write.Lock()
	defer t.write.Unlock()
	if err := ctx.Err(); err != nil {
		t.terminate()
		return err
	}
	if err := t.conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.terminate()
		return transportError(ctx, err)
	}
	return nil
}
func (t *Transport) Receive(ctx context.Context) ([]byte, error) {
	t.read.Lock()
	defer t.read.Unlock()
	if err := ctx.Err(); err != nil {
		t.terminate()
		return nil, err
	}
	typ, payload, err := t.conn.Read(ctx)
	if err != nil {
		t.terminate()
		if errors.Is(err, websocket.ErrMessageTooBig) || strings.Contains(err.Error(), "message too big") {
			return nil, ErrFrameLimit
		}
		return nil, transportError(ctx, err)
	}
	if typ != websocket.MessageText || !json.Valid(payload) {
		t.terminate()
		return nil, ErrTransportClosed
	}
	if len(payload) > t.limit {
		t.terminate()
		return nil, ErrFrameLimit
	}
	return payload, nil
}
func (t *Transport) terminate()   { t.once.Do(func() { _ = t.conn.CloseNow() }) }
func (t *Transport) Close() error { t.terminate(); return nil }
func transportError(ctx context.Context, err error) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	return ErrTransportClosed
}
