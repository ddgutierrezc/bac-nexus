// Package sshstdio adapts an authenticated Mapepire single-mode process to
// the transport-neutral application client.
package sshstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"bac-nexus/internal/mapepire"
)

var (
	ErrInvalidFrame = errors.New("Mapepire SSH frame is invalid")
	ErrFrameLimit   = errors.New("Mapepire SSH frame exceeds the release limit")
)

type Channel interface {
	io.Reader
	io.Writer
	io.Closer
}

type Transport struct {
	channel Channel
	reader  *bufio.Reader
	write   sync.Mutex
	close   sync.Once
}

var _ mapepire.MessageTransport = (*Transport)(nil)

func New(channel Channel) *Transport {
	return &Transport{channel: channel, reader: bufio.NewReader(channel)}
}

func (t *Transport) Send(ctx context.Context, frame []byte) error {
	if err := ctx.Err(); err != nil {
		t.Close()
		return err
	}
	if t == nil || t.channel == nil || len(frame) == 0 || len(frame) > mapepire.MaxFrameBytes || bytes.IndexByte(frame, '\n') >= 0 || bytes.IndexByte(frame, '\r') >= 0 {
		return ErrInvalidFrame
	}
	var object map[string]json.RawMessage
	if err := decodeObject(frame, &object); err != nil {
		return err
	}
	t.write.Lock()
	defer t.write.Unlock()
	if err := ctx.Err(); err != nil {
		t.Close()
		return err
	}
	if _, err := t.channel.Write(append(append([]byte(nil), frame...), '\n')); err != nil {
		t.Close()
		return err
	}
	return nil
}

func (t *Transport) Receive(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		t.Close()
		return nil, err
	}
	if t == nil || t.channel == nil {
		return nil, ErrInvalidFrame
	}
	result := make(chan struct {
		line []byte
		err  error
	}, 1)
	go func() {
		line, err := t.reader.ReadBytes('\n')
		result <- struct {
			line []byte
			err  error
		}{line, err}
	}()
	var line []byte
	var err error
	select {
	case value := <-result:
		line, err = value.line, value.err
	case <-ctx.Done():
		t.Close()
		return nil, ctx.Err()
	}
	if err != nil {
		t.Close()
		if errors.Is(err, io.EOF) {
			return nil, ErrInvalidFrame
		}
		return nil, err
	}
	if len(line) > mapepire.MaxFrameBytes+1 || len(line) == 1 {
		t.Close()
		return nil, ErrFrameLimit
	}
	frame := line[:len(line)-1]
	var object map[string]json.RawMessage
	if err := decodeObject(frame, &object); err != nil {
		t.Close()
		return nil, err
	}
	return frame, nil
}

func decodeObject(frame []byte, target *map[string]json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(frame))
	if err := decoder.Decode(target); err != nil || *target == nil {
		return ErrInvalidFrame
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ErrInvalidFrame
	}
	return nil
}

func (t *Transport) Close() error {
	if t == nil || t.channel == nil {
		return nil
	}
	var err error
	t.close.Do(func() { err = t.channel.Close() })
	return err
}
