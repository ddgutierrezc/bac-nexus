package source

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type acquisitionRemote struct {
	data                   []byte
	info                   acquisitionInfo
	copyErr, statErr       error
	downloadErr, removeErr error
	readErr, closeErr      error
	removed                bool
	copies, downloads      int
	removes                int
	events                 *[]string
	removeContextErr       error
	copyPath               string
}

func (r *acquisitionRemote) CopyToUTF8(ctx context.Context, _, path string) error {
	r.copies++
	r.copyPath = path
	*r.events = append(*r.events, "copy")
	if r.copyErr != nil {
		return r.copyErr
	}
	return ctx.Err()
}
func (r *acquisitionRemote) Stat(_ context.Context, _ string) (os.FileInfo, error) {
	if r.statErr != nil {
		return nil, r.statErr
	}
	if r.removed {
		return nil, ErrRemoteNotFound
	}
	return r.info, nil
}
func (r *acquisitionRemote) Download(context.Context, string) (io.ReadCloser, error) {
	r.downloads++
	*r.events = append(*r.events, "download")
	if r.downloadErr != nil {
		return nil, r.downloadErr
	}
	return readCloser{Reader: bytes.NewReader(r.data), readErr: r.readErr, closeErr: r.closeErr}, nil
}
func (r *acquisitionRemote) Remove(ctx context.Context, _ string) error {
	r.removes++
	*r.events = append(*r.events, "remove")
	r.removeContextErr = ctx.Err()
	r.removed = true
	return r.removeErr
}

type readCloser struct {
	io.Reader
	readErr, closeErr error
}

func (r readCloser) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return r.Reader.Read(p)
}
func (r readCloser) Close() error { return r.closeErr }

type acquisitionInfo struct {
	size int64
	mode os.FileMode
}

func (i acquisitionInfo) Name() string       { return "source" }
func (i acquisitionInfo) Size() int64        { return i.size }
func (i acquisitionInfo) Mode() os.FileMode  { return i.mode }
func (i acquisitionInfo) ModTime() time.Time { return time.Time{} }
func (i acquisitionInfo) IsDir() bool        { return false }
func (i acquisitionInfo) Sys() any           { return nil }

func TestAcquirerAcquiresOneCompleteSnapshotAndConfirmsCleanup(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("one\ntwo\n"), info: acquisitionInfo{8, 0o600}, events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{8, 0o600}, events: &events}
	var opens int
	a := Acquirer{Open: func(context.Context) (AcquisitionRemote, io.Closer, error) {
		opens++
		if opens == 1 {
			return request, io.NopCloser(nil), nil
		}
		return cleanup, io.NopCloser(nil), nil
	}, Random: bytes.NewReader(make([]byte, 16))}

	snap, err := a.Acquire(context.Background(), candidate())
	if err != nil || snap == nil || opens != 2 || request.copies != 1 || request.downloads != 1 || cleanup.removes != 1 || request.copyPath != "/tmp/bac-nexus-catalog-00000000000000000000000000000000.utf8" {
		t.Fatalf("Acquire() = %v, %v; opens/copy/download/remove = %d/%d/%d/%d", snap, err, opens, request.copies, request.downloads, cleanup.removes)
	}
	if page, err := snap.Page(1, 2); err != nil || strings.Join(page.Lines, ",") != "one,two" || strings.Join(events, ",") != "copy,download,remove" {
		t.Fatalf("snapshot/events = %#v, %v / %v", page, err, events)
	}
}

func TestAcquirerFailsClosedAndCleansOwnedTemporary(t *testing.T) {
	boom := errors.New("boom")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	timedOut, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		set  func(*acquisitionRemote)
	}{
		{"cancelled", cancelled, func(*acquisitionRemote) {}},
		{"deadline", timedOut, func(*acquisitionRemote) {}},
		{"ambiguous copy", context.Background(), func(r *acquisitionRemote) { r.copyErr = boom }},
		{"stat", context.Background(), func(r *acquisitionRemote) { r.statErr = boom }},
		{"nonregular stat", context.Background(), func(r *acquisitionRemote) { r.info.mode = os.ModeDir }},
		{"over ceiling", context.Background(), func(r *acquisitionRemote) { r.info.size = AbsoluteMaxBytes + 1 }},
		{"download", context.Background(), func(r *acquisitionRemote) { r.downloadErr = boom }},
		{"read", context.Background(), func(r *acquisitionRemote) { r.readErr = boom }},
		{"close", context.Background(), func(r *acquisitionRemote) { r.closeErr = boom }},
		{"truncated", context.Background(), func(r *acquisitionRemote) { r.info.size++ }},
		{"growth", context.Background(), func(r *acquisitionRemote) { r.info.size-- }},
		{"invalid utf8", context.Background(), func(r *acquisitionRemote) { r.data = []byte{0xff}; r.info.size = 1 }},
		{"cleanup remove", context.Background(), func(r *acquisitionRemote) {}},
		{"cleanup confirmation", context.Background(), func(r *acquisitionRemote) {}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			request := &acquisitionRemote{data: []byte("ok\n"), info: acquisitionInfo{3, 0o600}, events: &events}
			tt.set(request)
			cleanup := &acquisitionRemote{info: request.info, events: &events}
			if tt.name == "cleanup remove" {
				cleanup.removeErr = boom
			}
			if tt.name == "cleanup confirmation" {
				cleanup.statErr = boom
			}
			a := Acquirer{Open: sequentialOpener(request, cleanup), Random: bytes.NewReader(make([]byte, 16))}
			snap, err := a.Acquire(tt.ctx, candidate())
			if err == nil || snap != nil || request.copies != 1 || cleanup.removes != 1 || cleanup.removeContextErr != nil {
				t.Fatalf("Acquire() = %v, %v; copy/remove = %d/%d", snap, err, request.copies, cleanup.removes)
			}
		})
	}
}

func TestAcquirerJoinsPrimaryAndCleanupErrors(t *testing.T) {
	boom, cleanupBoom := errors.New("copy"), errors.New("cleanup")
	events := []string{}
	request := &acquisitionRemote{info: acquisitionInfo{0, 0o600}, copyErr: boom, events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{0, 0o600}, removeErr: cleanupBoom, events: &events}
	snap, err := (Acquirer{Open: sequentialOpener(request, cleanup), Random: bytes.NewReader(make([]byte, 16))}).Acquire(context.Background(), candidate())
	if snap != nil || !errors.Is(err, boom) || !errors.Is(err, cleanupBoom) {
		t.Fatalf("Acquire() = %v, %v", snap, err)
	}
}

func TestAcquirerAcceptsAlreadyRemovedTemporaryOnlyAfterConfirmation(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("ok\n"), info: acquisitionInfo{3, 0o600}, events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{3, 0o600}, removeErr: ErrRemoteNotFound, events: &events}
	snap, err := (Acquirer{Open: sequentialOpener(request, cleanup), Random: bytes.NewReader(make([]byte, 16))}).Acquire(context.Background(), candidate())
	if err != nil || snap == nil || cleanup.removes != 1 {
		t.Fatalf("Acquire() = %v, %v; removes = %d", snap, err, cleanup.removes)
	}
}

func sequentialOpener(request, cleanup AcquisitionRemote) func(context.Context) (AcquisitionRemote, io.Closer, error) {
	opened := 0
	return func(context.Context) (AcquisitionRemote, io.Closer, error) {
		opened++
		if opened == 1 {
			return request, io.NopCloser(nil), nil
		}
		return cleanup, io.NopCloser(nil), nil
	}
}
