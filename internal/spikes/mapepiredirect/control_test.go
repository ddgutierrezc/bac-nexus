package mapepiredirect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestControlHandshakeSendsOfficialStyleRequest(t *testing.T) {
	closed := make(chan int, 1)
	requests := make(chan struct {
		authorization string
		body          map[string]string
	}, 1)
	server := controlServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/db/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Sec-WebSocket-Protocol") != "" {
			t.Error("unexpected WebSocket subprotocol")
		}
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetCloseHandler(func(code int, _ string) error { closed <- code; return nil })
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		body := map[string]string{}
		if err := json.Unmarshal(message, &body); err != nil {
			t.Error(err)
			return
		}
		requests <- struct {
			authorization string
			body          map[string]string
		}{r.Header.Get("Authorization"), body}
		_ = conn.WriteJSON(map[string]any{"success": true, "id": body["id"]})
		_, _, _ = conn.ReadMessage()
	})
	var output bytes.Buffer
	if err := controlHandshakeWithin(context.Background(), server, &output, time.Second); err != nil {
		t.Fatal(err)
	}
	request := <-requests
	if request.authorization != "Basic dXNlcjpzZWNyZXQ=" {
		t.Fatalf("unexpected Basic authorization")
	}
	if request.body["id"] == "" || request.body["type"] != "connect" || request.body["technique"] != "tcp" || request.body["application"] != controlApplication {
		t.Fatalf("request = %#v", request.body)
	}
	if code := <-closed; code != websocket.CloseNormalClosure {
		t.Fatalf("close code = %d", code)
	}
	if output.String() != "control_handshake: success cleanup=success\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestControlHandshakeClassifiesFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		timeout time.Duration
		want    controlFailure
	}{
		{"upgrade rejection", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, redactionSentinel, http.StatusForbidden) }, time.Second, controlUpgrade},
		{"connect rejection", websocketReply(map[string]any{"success": false, "error": redactionSentinel}), time.Second, controlReject},
		{"malformed response", websocketReply(redactionSentinel), time.Second, controlProtocol},
		{"mismatched id", websocketReply(map[string]any{"success": true, "id": "wrong"}), time.Second, controlProtocol},
		{"oversized response", websocketReply(strings.Repeat("x", controlResponseMax)), time.Second, controlProtocol},
		{"read timeout", func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
			if err == nil {
				defer conn.Close()
				time.Sleep(100 * time.Millisecond)
			}
		}, 20 * time.Millisecond, controlTimeouts},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			err := controlHandshakeWithin(context.Background(), controlServer(t, tt.handler), &output, tt.timeout)
			if !errorsIsControl(err, tt.want) || !strings.Contains(output.String(), "classification="+string(tt.want)) || strings.Contains(output.String(), redactionSentinel) {
				t.Fatalf("err=%v output=%q", err, output.String())
			}
		})
	}
}

func TestControlHandshakeRedactsAndDiagnosticReportsSDKConnectionFailure(t *testing.T) {
	var output bytes.Buffer
	cfg := Config{Host: "private-host", Port: "8076", User: "private-user", Password: "private-password"}
	control := func(Config, io.Writer) error {
		_, _ = output.WriteString("control_handshake: success cleanup=success\n")
		return nil
	}
	sdk := func(Config, io.Writer) error { return fmt.Errorf("%w: %s", sdkFailure("connect"), redactionSentinel) }
	if err := runDiagnostic(cfg, &output, control, sdk); err == nil {
		t.Fatal("diagnostic unexpectedly succeeded")
	}
	text := output.String()
	if !strings.Contains(text, "conclusion=unofficial_sdk_path_failed_after_control_success") {
		t.Fatalf("missing diagnostic conclusion: %q", text)
	}
	for _, secret := range []string{cfg.Host, cfg.User, cfg.Password, "Authorization", redactionSentinel} {
		if strings.Contains(text, secret) {
			t.Fatalf("leaked %q in %q", secret, text)
		}
	}
}

const redactionSentinel = "host=private-host port=8076 user=private-user password=private-password Authorization=Basic-private SQL=VALUES-1 row=private-row job=private-job"

func TestDiagnosticDoesNotRunSDKAfterControlFailure(t *testing.T) {
	called := false
	err := runDiagnostic(Config{}, io.Discard,
		func(Config, io.Writer) error { return controlTLSDial },
		func(Config, io.Writer) error { called = true; return nil },
	)
	if err == nil || called {
		t.Fatalf("err=%v sdk_called=%t", err, called)
	}
}

func websocketReply(reply any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			return
		}
		defer conn.Close()
		_, message, _ := conn.ReadMessage()
		if object, ok := reply.(map[string]any); ok && object["id"] == nil {
			request := map[string]string{}
			_ = json.Unmarshal(message, &request)
			object["id"] = request["id"]
		}
		_ = conn.WriteJSON(reply)
	}
}

func controlServer(t *testing.T, handler http.HandlerFunc) Config {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	return Config{Host: host, Port: port, User: "user", Password: "secret"}
}

func errorsIsControl(err error, want controlFailure) bool {
	actual, ok := err.(controlFailure)
	return ok && actual == want
}
