package wss

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWSSLoopbackTextAndTLSIdentity(t *testing.T) {
	s, roots := testServer(t, func(c *websocket.Conn) {
		m, p, _ := c.Read(context.Background())
		if m != websocket.MessageText || string(p) != `{"id":"1"}` {
			return
		}
		_ = c.Write(context.Background(), websocket.MessageText, []byte(`{"id":"1","success":true}`))
	})
	client := &http.Client{}
	original := client.Transport
	tr, err := Dial(context.Background(), wssURL(s), Options{HTTPClient: client, TLSConfig: tlsConfig(roots)})
	if err != nil {
		t.Fatal(err)
	}
	if client.Transport != original {
		t.Fatal("Dial mutated caller HTTP client")
	}
	if err := tr.Send(context.Background(), []byte(`{"id":"1"}`)); err != nil {
		t.Fatal(err)
	}
	p, err := tr.Receive(context.Background())
	if err != nil || string(p) != `{"id":"1","success":true}` {
		t.Fatalf("receive=%q err=%v", p, err)
	}
}
func TestWSSRejectsBinaryAndMalformedPayloads(t *testing.T) {
	for _, typ := range []websocket.MessageType{websocket.MessageBinary, websocket.MessageText} {
		s, roots := testServer(t, func(c *websocket.Conn) {
			_ = c.Write(context.Background(), typ, []byte("not-json"))
		})
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig(roots)}}
		tr, err := Dial(context.Background(), wssURL(s), Options{HTTPClient: client, TLSConfig: client.Transport.(*http.Transport).TLSClientConfig})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tr.Receive(context.Background()); err == nil {
			t.Errorf("accepted %v payload", typ)
		}
	}
}
func TestWSSPinTOFURejectsMismatchAndRotation(t *testing.T) {
	s, roots := testServer(t, func(c *websocket.Conn) { c.CloseNow() })
	good := leafPin(s)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig(roots)}}
	for _, pin := range []string{"sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", good} {
		tr, err := Dial(context.Background(), wssURL(s), Options{HTTPClient: client, TLSConfig: client.Transport.(*http.Transport).TLSClientConfig, Pin: pin})
		if pin != good && err == nil {
			t.Error("mismatched pin accepted")
		} else if pin != good && !errors.Is(err, ErrIdentityFailure) {
			t.Errorf("mismatch classification=%v", err)
		}
		if pin == good && err != nil {
			t.Fatal(err)
		} else if tr != nil {
		}
	}
	if _, err := Dial(context.Background(), wssURL(s), Options{HTTPClient: client, TLSConfig: client.Transport.(*http.Transport).TLSClientConfig, TOFU: true}); err == nil {
		t.Fatal("implicit TOFU accepted without evidence")
	}
	if _, err := Dial(context.Background(), wssURL(s), Options{Pin: good}); err != nil {
		t.Fatalf("pinned self-signed leaf: %v", err)
	}
	if _, err := Dial(context.Background(), wssURL(s), Options{}); !errors.Is(err, ErrIdentityFailure) {
		t.Errorf("untrusted CA classification=%v", err)
	}
	for _, pin := range []string{"sha256/" + strings.Repeat("A", 42) + "+", "sha256/%%%"} {
		if _, err := Dial(context.Background(), wssURL(s), Options{Pin: pin}); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("pin %q error=%v", pin, err)
		}
	}
	for _, endpoint := range []string{"ws://127.0.0.1:1", "http://127.0.0.1:1", "wss://MP_UNSECURE/endpoint", "wss://user:pass/host"} {
		if _, err := Dial(context.Background(), endpoint, Options{}); !errors.Is(err, ErrInvalidEndpoint) {
			t.Errorf("endpoint %q error=%v", endpoint, err)
		}
	}
	if _, err := Dial(context.Background(), "wss://127.0.0.1:1", Options{HTTPClient: &http.Client{Transport: roundTripper{}}}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("custom transport error=%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := Dial(ctx, "wss://127.0.0.1:1", Options{}); !errors.Is(err, ErrTimeout) && !errors.Is(err, ErrAvailability) {
		t.Errorf("dial classification=%v", err)
	}
}
func TestWSSBoundsCompressionAndTerminalCancellation(t *testing.T) {
	s, roots := testServer(t, func(c *websocket.Conn) { defer c.CloseNow(); _, _, _ = c.Read(context.Background()) })
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig(roots)}}
	tr, err := Dial(context.Background(), wssURL(s), Options{HTTPClient: client, TLSConfig: client.Transport.(*http.Transport).TLSClientConfig, ReadLimit: 8})
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Send(context.Background(), []byte(`{"id":"123456789"}`)); !errors.Is(err, ErrFrameLimit) {
		t.Fatalf("limit=%v", err)
	}
	_ = tr.Close()
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tr.Send(ctx, []byte("x")); err == nil {
		t.Fatal("cancelled send succeeded")
	}
	s2, roots2 := testServer(t, func(c *websocket.Conn) { defer c.CloseNow(); _, _, _ = c.Read(context.Background()) })
	client2 := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig(roots2)}}
	tr2, err := Dial(context.Background(), wssURL(s2), Options{HTTPClient: client2, TLSConfig: client2.Transport.(*http.Transport).TLSClientConfig})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err = tr2.Receive(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	if err = tr2.Send(context.Background(), []byte(`{}`)); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("reusable=%v", err)
	}
}
func tlsConfig(roots *x509.CertPool) *tls.Config {
	return &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
}
func testServer(t *testing.T, fn func(*websocket.Conn)) (*httptest.Server, *x509.CertPool) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
		if err == nil {
			fn(c)
		}
	}))
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	return srv, roots
}
func wssURL(s *httptest.Server) string { return "wss://" + strings.TrimPrefix(s.URL, "https://") }
func leafPin(s *httptest.Server) string {
	sum := sha256.Sum256(s.Certificate().Raw)
	return "sha256/" + base64.RawURLEncoding.EncodeToString(sum[:])
}

type roundTripper struct{}

func (roundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unused")
}
