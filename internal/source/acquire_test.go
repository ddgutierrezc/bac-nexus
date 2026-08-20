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
	stats                  int
	trackStats             bool
	events                 *[]string
	removeContextErr       error
	copyPath, copySource   string
	home                   string
	prepared               string
	prepareErr             error
	created                string
	creates                int
	directoryInfo          acquisitionInfo
}

func (r *acquisitionRemote) CopyToUTF8(ctx context.Context, source, path string) error {
	r.copies++
	r.copySource = source
	r.copyPath = path
	if r.events != nil {
		*r.events = append(*r.events, "copy")
	}
	if r.copyErr != nil {
		return r.copyErr
	}
	return ctx.Err()
}
func (r *acquisitionRemote) Stat(_ context.Context, _ string) (os.FileInfo, error) {
	r.stats++
	if r.trackStats {
		*r.events = append(*r.events, "stat")
	}
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
func (r *acquisitionRemote) AuthenticatedHome(context.Context) (string, error) { return r.home, nil }
func (r *acquisitionRemote) PreparePrivateDirectory(_ context.Context, directory string) error {
	r.prepared = directory
	return r.prepareErr
}
func (r *acquisitionRemote) Lstat(_ context.Context, path string) (os.FileInfo, error) {
	if path == r.prepared && r.directoryInfo.mode != 0 {
		return r.directoryInfo, nil
	}
	return r.info, nil
}
func (r *acquisitionRemote) CreateExclusive(_ context.Context, path string) error {
	r.created = path
	r.creates++
	if r.events != nil {
		*r.events = append(*r.events, "reserve")
	}
	return nil
}

type acquisitionLedger struct {
	events  *[]string
	record  OwnershipRecord
	deleted OwnershipRecord
	deletes int
}

func (l *acquisitionLedger) Admit(_ context.Context, record OwnershipRecord) error {
	l.record = record
	*l.events = append(*l.events, "admit")
	return nil
}
func (l *acquisitionLedger) Delete(_ context.Context, record OwnershipRecord) error {
	l.deleted = record
	l.deletes++
	*l.events = append(*l.events, "delete")
	return nil
}
func (*acquisitionLedger) Close() error { return nil }

func newAcquirer(request, cleanup *acquisitionRemote, events *[]string) Acquirer {
	request.home = "/home/nexus"
	request.directoryInfo = acquisitionInfo{mode: os.ModeDir | 0o700}
	return Acquirer{Open: sequentialOpener(request, cleanup), Random: bytes.NewReader(make([]byte, 16)), Ownership: &acquisitionLedger{events: events}, Profile: "test", TargetDigest: make([]byte, 32)}
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
func (i acquisitionInfo) IsDir() bool        { return i.mode.IsDir() }
func (i acquisitionInfo) Sys() any           { return nil }

func TestAcquirerAcquiresOneCompleteSnapshotAndConfirmsCleanup(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("one\ntwo\n"), info: acquisitionInfo{8, 0o600}, events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{8, 0o600}, events: &events}
	var opens int
	a := newAcquirer(request, cleanup, &events)
	a.Open = func(context.Context) (AcquisitionRemote, io.Closer, error) {
		opens++
		if opens == 1 {
			return request, io.NopCloser(nil), nil
		}
		return cleanup, io.NopCloser(nil), nil
	}

	snap, err := a.Acquire(context.Background(), candidate())
	if err != nil || snap == nil || opens != 2 || request.copies != 1 || request.downloads != 1 || cleanup.removes != 1 || request.copyPath != "/home/nexus/.bac-nexus/tmp/00000000000000000000000000000000.utf8" {
		t.Fatalf("Acquire() = %v, %v; opens/copy/download/remove = %d/%d/%d/%d", snap, err, opens, request.copies, request.downloads, cleanup.removes)
	}
	if page, err := snap.Page(1, 2); err != nil || strings.Join(page.Lines, ",") != "one,two" || strings.Join(events, ",") != "admit,reserve,copy,download,remove" {
		t.Fatalf("snapshot/events = %#v, %v / %v", page, err, events)
	}
}

func TestPrivateDirectoryRequiresAuthenticatedAbsoluteHome(t *testing.T) {
	for _, home := range []string{"", "relative", "/home/../other"} {
		t.Run(home, func(t *testing.T) {
			if _, err := privateDirectory(home); err == nil {
				t.Fatalf("privateDirectory(%q) succeeded", home)
			}
		})
	}
}

func TestPreparePrivateDirectoryCreatesAndVerifies0700(t *testing.T) {
	remote := &acquisitionRemote{home: "/home/nexus", info: acquisitionInfo{mode: os.ModeDir | 0o700}}
	directory, err := preparePrivateDirectory(context.Background(), remote)
	if err != nil || directory != "/home/nexus/.bac-nexus/tmp" || remote.prepared != directory {
		t.Fatalf("preparePrivateDirectory() = %q, %v; prepared %q", directory, err, remote.prepared)
	}
}

func TestReservePrivateFileCreatesExclusive0600RegularFile(t *testing.T) {
	remote := &acquisitionRemote{info: acquisitionInfo{mode: 0o600}}
	path, err := reservePrivateFile(context.Background(), remote, "/home/nexus/.bac-nexus/tmp", bytes.NewReader(make([]byte, 16)))
	if err != nil || path != "/home/nexus/.bac-nexus/tmp/00000000000000000000000000000000.utf8" || remote.created != path || remote.creates != 1 {
		t.Fatalf("reservePrivateFile() = %q, %v; created %q (%d)", path, err, remote.created, remote.creates)
	}
}

func TestReservePrivateFileRejectsTraversalAndSymlinkEscape(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directory string
		mode      os.FileMode
	}{
		{"traversal", "/home/nexus/.bac-nexus/tmp/../../escape", 0o600},
		{"symlink", "/home/nexus/.bac-nexus/tmp", os.ModeSymlink | 0o600},
	} {
		t.Run(tt.name, func(t *testing.T) {
			remote := &acquisitionRemote{info: acquisitionInfo{mode: tt.mode}}
			if _, err := reservePrivateFile(context.Background(), remote, tt.directory, bytes.NewReader(make([]byte, 16))); err == nil {
				t.Fatal("reservePrivateFile() succeeded")
			}
			if tt.name == "traversal" && remote.creates != 0 {
				t.Fatalf("CreateExclusive calls = %d, want 0", remote.creates)
			}
		})
	}
}

func TestAcquirerAdmitsOwnershipBeforeReserveAndCopy(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("ok\n"), info: acquisitionInfo{3, 0o600}, directoryInfo: acquisitionInfo{mode: os.ModeDir | 0o700}, home: "/home/nexus", events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{3, 0o600}, events: &events}
	ledger := &acquisitionLedger{events: &events}
	a := Acquirer{Open: sequentialOpener(request, cleanup), Random: bytes.NewReader(make([]byte, 16)), Ownership: ledger, Profile: "test", TargetDigest: make([]byte, 32), Now: func() time.Time { return time.Unix(0, 0) }}
	snap, err := a.Acquire(context.Background(), candidate())
	if err != nil || snap == nil || strings.Join(events, ",") != "admit,reserve,copy,download,remove" || ledger.record.RemotePath != request.created {
		t.Fatalf("Acquire() = %v, %v; events=%v record=%#v", snap, err, events, ledger.record)
	}
}

func TestAcquirerDeletesExactOwnershipOnlyAfterConfirmedPrivateCleanup(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("ok\n"), info: acquisitionInfo{3, 0o600}, directoryInfo: acquisitionInfo{mode: os.ModeDir | 0o700}, home: "/home/nexus", events: &events, trackStats: true}
	cleanup := &acquisitionRemote{info: acquisitionInfo{3, 0o600}, events: &events, trackStats: true}
	ledger := &acquisitionLedger{events: &events}
	a := Acquirer{Open: sequentialOpener(request, cleanup), Random: bytes.NewReader(make([]byte, 16)), Ownership: ledger, Profile: "test", TargetDigest: make([]byte, 32), Now: func() time.Time { return time.Unix(0, 0) }}

	snap, err := a.Acquire(context.Background(), candidate())
	if err != nil || snap == nil || cleanup.removes != 1 || cleanup.stats != 1 || ledger.deletes != 1 || ledger.deleted.RemotePath != request.created || strings.Join(events, ",") != "admit,reserve,copy,stat,download,remove,stat,delete" {
		t.Fatalf("Acquire() = %v, %v; cleanup/delete/events = %d/%d/%v", snap, err, cleanup.removes, ledger.deletes, events)
	}
}

func TestAcquirerApprovalCopiesFromButNeverWritesSourceMember(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("ok\n"), info: acquisitionInfo{3, 0o600}, events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{3, 0o600}, events: &events}
	selection := candidate()
	snap, err := newAcquirer(request, cleanup, &events).Acquire(context.Background(), selection)
	wantSource, sourceErr := selection.QSYSPath()
	if sourceErr != nil || err != nil || snap == nil || request.copySource != wantSource || request.copyPath == wantSource {
		t.Fatalf("Acquire() = %v, %v; copy source/target = %q/%q", snap, err, request.copySource, request.copyPath)
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
			a := newAcquirer(request, cleanup, &events)
			snap, err := a.Acquire(tt.ctx, candidate())
			wantRemoteWork := tt.name != "nonregular stat"
			if err == nil || snap != nil || (request.copies == 1) != wantRemoteWork || (cleanup.removes == 1) != wantRemoteWork || cleanup.removeContextErr != nil {
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
	snap, err := newAcquirer(request, cleanup, &events).Acquire(context.Background(), candidate())
	if snap != nil || !errors.Is(err, boom) || !errors.Is(err, cleanupBoom) {
		t.Fatalf("Acquire() = %v, %v", snap, err)
	}
}

func TestAcquirerAcceptsAlreadyRemovedTemporaryOnlyAfterConfirmation(t *testing.T) {
	events := []string{}
	request := &acquisitionRemote{data: []byte("ok\n"), info: acquisitionInfo{3, 0o600}, events: &events}
	cleanup := &acquisitionRemote{info: acquisitionInfo{3, 0o600}, removeErr: ErrRemoteNotFound, events: &events}
	snap, err := newAcquirer(request, cleanup, &events).Acquire(context.Background(), candidate())
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
