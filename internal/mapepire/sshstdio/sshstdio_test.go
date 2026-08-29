package sshstdio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"bac-nexus/internal/mapepire"
)

type fakeChannel struct {
	mu       sync.Mutex
	in       io.Reader
	inWriter *io.PipeWriter
	out      bytes.Buffer
	closed   chan struct{}
}

func newChannel(input string) *fakeChannel {
	return &fakeChannel{in: strings.NewReader(input), closed: make(chan struct{})}
}
func (c *fakeChannel) Read(p []byte) (int, error) { return c.in.Read(p) }
func (c *fakeChannel) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.Write(p)
}
func (c *fakeChannel) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}

func TestTransportSendRejectsEmbeddedAndMultipleFrames(t *testing.T) {
	transport := New(newChannel(""))
	for _, value := range []string{`{"id":"x"}
`, `{"id":"x"}
{"id":"y"}`, "\n{}", "{}\r\n"} {
		if err := transport.Send(context.Background(), []byte(value)); !errors.Is(err, ErrInvalidFrame) {
			t.Errorf("Send(%q) error = %v", value, err)
		}
	}
}

func TestTransportReceiveAcceptsOneObjectAndRejectsUnsafeFrames(t *testing.T) {
	valid := New(newChannel(`{"id":"x","success":true}
`))
	frame, err := valid.Receive(context.Background())
	if err != nil || string(frame) != `{"id":"x","success":true}` {
		t.Fatalf("valid frame = %q/%v", frame, err)
	}
	for _, value := range []string{"\n", "[]\n", "not-json\n", "{}", strings.Repeat("x", mapepire.MaxFrameBytes) + "\n", "{} {}\n"} {
		if _, err := New(newChannel(value)).Receive(context.Background()); !errors.Is(err, ErrInvalidFrame) && !errors.Is(err, ErrFrameLimit) {
			t.Errorf("Receive(%q) error = %v", value[:min(len(value), 12)], err)
		}
	}
}

func TestTransportThroughTypedClientRequiresMatchingSuccessfulResponse(t *testing.T) {
	channel := newChannel("")
	reader, writer := io.Pipe()
	channel.in, channel.inWriter = reader, writer
	transport := New(channel)
	client := mapepire.NewMessageSession(transport)
	go func() {
		for {
			channel.mu.Lock()
			request := append([]byte(nil), channel.out.Bytes()...)
			channel.out.Reset()
			channel.mu.Unlock()
			if len(request) != 0 {
				request = bytes.TrimSpace(request)
				var r map[string]any
				_ = json.Unmarshal(request, &r)
				id, _ := r["id"].(string)
				if channel.inWriter != nil {
					_, _ = channel.inWriter.Write([]byte(`{"id":"` + id + `","success":true}
`))
				}
				return
			}
		}
	}()
	response, err := client.Call(context.Background(), mapepire.Request{Type: mapepire.OperationPing})
	if err != nil || !response.Success {
		t.Fatalf("typed call = %#v/%v", response, err)
	}
}

func TestTransportTypedClientTreatsFailureAsTerminal(t *testing.T) {
	channel := newChannel(`{"id":"wrong","success":false}
`)
	client := mapepire.NewMessageSession(New(channel))
	_, err := client.Call(context.Background(), mapepire.Request{Type: mapepire.OperationPing})
	if !errors.Is(err, mapepire.ErrProtocolViolation) {
		t.Fatalf("failure response error = %v", err)
	}
	select {
	case <-channel.closed:
	default:
		t.Fatal("failure response did not close the process channel")
	}
}

func TestTransportReceiveCancellationClosesChannel(t *testing.T) {
	channel := newChannel("")
	transport := New(channel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := transport.Receive(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	select {
	case <-channel.closed:
	default:
		t.Fatal("channel was not closed")
	}
	if _, err := io.ReadAll(strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
