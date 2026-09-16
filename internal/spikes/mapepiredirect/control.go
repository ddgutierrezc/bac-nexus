package mapepiredirect

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var controlSequence atomic.Uint64

var controlIDSource = func() string {
	return fmt.Sprintf("nexus-control-%d", controlSequence.Add(1))
}

const (
	controlTimeout     = 3 * time.Second
	controlResponseMax = 8 << 10
	controlApplication = "BAC Nexus Mapepire Diagnostic"
)

type controlFailure string

const (
	controlTLSDial  controlFailure = "tls_dial"
	controlUpgrade  controlFailure = "http_upgrade"
	controlReject   controlFailure = "server_connect_rejection"
	controlProtocol controlFailure = "malformed_protocol_response"
	controlTimeouts controlFailure = "timeout"
	controlCleanup  controlFailure = "cleanup"
)

func (f controlFailure) Error() string { return "control handshake unavailable" }

func controlHandshake(cfg Config, out io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
	defer cancel()
	return controlHandshakeWithin(ctx, cfg, out, controlTimeout)
}

func controlHandshakeWithin(ctx context.Context, cfg Config, out io.Writer, timeout time.Duration) error {
	id := controlIDSource()
	endpoint := url.URL{Scheme: "wss", Host: net.JoinHostPort(cfg.Host, cfg.Port), Path: "/db/"}
	header := make(http.Header)
	header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.User+":"+cfg.Password)))
	dialer := websocket.Dialer{HandshakeTimeout: timeout, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // Disposable spike only.
	conn, response, err := dialer.DialContext(ctx, endpoint.String(), header)
	if err != nil {
		failure := controlTLSDial
		if ctx.Err() != nil {
			failure = controlTimeouts
		} else if response != nil {
			failure = controlUpgrade
		}
		return controlFailed(out, failure)
	}
	deadline := time.Now().Add(timeout)
	conn.SetReadLimit(controlResponseMax)
	if err := conn.SetWriteDeadline(deadline); err != nil {
		_ = conn.Close()
		return controlFailed(out, controlTimeouts)
	}
	if err := conn.WriteJSON(struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Technique   string `json:"technique"`
		Application string `json:"application"`
	}{ID: id, Type: "connect", Technique: "tcp", Application: controlApplication}); err != nil {
		_ = conn.Close()
		return controlFailed(out, controlTimeoutFor(ctx, err))
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		_ = conn.Close()
		return controlFailed(out, controlTimeouts)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return controlFailed(out, controlTimeoutFor(ctx, err))
	}
	var reply struct {
		ID      *string `json:"id"`
		Success *bool   `json:"success"`
	}
	if json.Unmarshal(message, &reply) != nil || reply.ID == nil || reply.Success == nil || *reply.ID != id {
		_ = conn.Close()
		return controlFailed(out, controlProtocol)
	}
	if !*reply.Success {
		_ = conn.Close()
		return controlFailed(out, controlReject)
	}
	if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline); err != nil {
		_ = conn.Close()
		return controlFailed(out, controlCleanup)
	}
	if err := conn.Close(); err != nil {
		return controlFailed(out, controlCleanup)
	}
	fmt.Fprintln(out, "control_handshake: success cleanup=success")
	return nil
}

func controlTimeoutFor(ctx context.Context, err error) controlFailure {
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		return controlTimeouts
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return controlTimeouts
	}
	return controlProtocol
}

func controlFailed(out io.Writer, failure controlFailure) error {
	fmt.Fprintf(out, "control_handshake: failure classification=%s\n", string(failure))
	return failure
}
