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

	"bac-nexus/internal/catalog"
)

type fileInfo struct{ size int64 }

func (f fileInfo) Name() string       { return "source" }
func (f fileInfo) Size() int64        { return f.size }
func (f fileInfo) Mode() os.FileMode  { return 0o600 }
func (f fileInfo) ModTime() time.Time { return time.Time{} }
func (f fileInfo) IsDir() bool        { return false }
func (f fileInfo) Sys() any           { return nil }

type fakeFiles struct {
	content     string
	removeError error
	statError   error
	openError   error
	readError   error
	removes     int
}

func (f *fakeFiles) Stat(string) (os.FileInfo, error) {
	if f.statError != nil {
		return nil, f.statError
	}
	return fileInfo{int64(len(f.content))}, nil
}
func (f *fakeFiles) OpenRead(string) (io.ReadCloser, error) {
	if f.openError != nil {
		return nil, f.openError
	}
	if f.readError != nil {
		return io.NopCloser(errorReader{err: f.readError}), nil
	}
	return io.NopCloser(strings.NewReader(f.content)), nil
}
func (f *fakeFiles) Remove(string) error { f.removes++; return f.removeError }

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

type fakeRunner struct {
	err       error
	qsysPath  string
	temporary string
}

func (f *fakeRunner) CopyToUTF8(_ context.Context, qsysPath, temporary string) error {
	f.qsysPath = qsysPath
	f.temporary = temporary
	return f.err
}

func candidate() catalog.Candidate {
	return catalog.Candidate{Item: "PISA061", SourceLibrary: "SRCLIB", SourceFileBase: "Q", ObjectType: "RPGLE", SourceType: "RPGLE"}
}

func TestBuildCopyCommandIsFixedAndQuoted(t *testing.T) {
	got, err := BuildCopyCommand("/QSYS.LIB/SRCLIB.LIB/QRPGLE.FILE/PISA061.MBR", "/tmp/bac-nexus-catalog-00.utf8")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "'/QOpenSys/usr/bin/system' 'CPYTOSTMF ") || !strings.Contains(got, "STMFCCSID(1208)") {
		t.Fatalf("unexpected command: %s", got)
	}
}

func TestRetrieveEnforcesBoundsAndCleanup(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		maxBytes   int
		maxLines   int
		want       string
		truncated  bool
		runnerErr  error
		cleanupErr error
		statErr    error
		openErr    error
		wantErr    bool
	}{
		{"byte cap", "abcdefgh", 4, 10, "abcd", true, nil, nil, nil, nil, false},
		{"multibyte byte cap", "A€B", 3, 10, "A", true, nil, nil, nil, nil, false},
		{"line cap", "one\ntwo\nthree\n", 100, 2, "one\ntwo\n", true, nil, nil, nil, nil, false},
		{"command failure still cleans", "", 100, 10, "", false, errors.New("copy failed"), nil, nil, nil, true},
		{"stat failure still cleans", "", 100, 10, "", false, nil, nil, errors.New("stat failed"), nil, true},
		{"open failure still cleans", "one", 100, 10, "", false, nil, nil, nil, errors.New("open failed"), true},
		{"cleanup failure is reported", "one\n", 100, 10, "one\n", false, nil, errors.New("remove failed"), nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := &fakeFiles{content: tt.content, removeError: tt.cleanupErr, statError: tt.statErr, openError: tt.openErr}
			runner := &fakeRunner{err: tt.runnerErr}
			result, err := (Retriever{Files: files, Runner: runner, Random: bytes.NewReader(make([]byte, 16))}).Retrieve(context.Background(), candidate(), tt.maxBytes, tt.maxLines)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if string(result.Content) != tt.want || result.Truncated != tt.truncated {
				t.Fatalf("content/truncated = %q/%v, want %q/%v", result.Content, result.Truncated, tt.want, tt.truncated)
			}
			if files.removes != 1 {
				t.Fatalf("cleanup calls = %d, want 1", files.removes)
			}
		})
	}
}

func TestRetrieveRejectsInvalidUTF8(t *testing.T) {
	files := &fakeFiles{content: "A\xffB"}
	result, err := (Retriever{Files: files, Runner: &fakeRunner{}, Random: bytes.NewReader(make([]byte, 16))}).Retrieve(context.Background(), candidate(), 100, 10)
	if err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("error = %v, want invalid UTF-8", err)
	}
	if len(result.Content) != 0 {
		t.Fatalf("invalid content returned: %q", result.Content)
	}
}

func TestRetrieveJoinsPrimaryAndCleanupFailures(t *testing.T) {
	primary := errors.New("primary failure")
	cleanup := errors.New("cleanup failure")
	tests := []struct {
		name  string
		files *fakeFiles
		run   *fakeRunner
	}{
		{name: "copy", files: &fakeFiles{removeError: cleanup}, run: &fakeRunner{err: primary}},
		{name: "stat", files: &fakeFiles{statError: primary, removeError: cleanup}, run: &fakeRunner{}},
		{name: "open", files: &fakeFiles{content: "x", openError: primary, removeError: cleanup}, run: &fakeRunner{}},
		{name: "read", files: &fakeFiles{content: "x", readError: primary, removeError: cleanup}, run: &fakeRunner{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := (Retriever{Files: tt.files, Runner: tt.run, Random: bytes.NewReader(make([]byte, 16))}).Retrieve(context.Background(), candidate(), 100, 10)
			if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
				t.Fatalf("error = %v, want both primary and cleanup failures", err)
			}
			if result.Cleanup != "failed" {
				t.Fatalf("cleanup = %q, want failed", result.Cleanup)
			}
		})
	}
}

func TestRetrieveRejectsExcessiveLimitsBeforeRemoteWork(t *testing.T) {
	files := &fakeFiles{}
	runner := &fakeRunner{}
	_, err := (Retriever{Files: files, Runner: runner}).Retrieve(context.Background(), candidate(), AbsoluteMaxBytes+1, 1)
	if err == nil {
		t.Fatal("expected bound rejection")
	}
	if runner.qsysPath != "" || files.removes != 0 {
		t.Fatal("remote work occurred before limit validation")
	}
}
