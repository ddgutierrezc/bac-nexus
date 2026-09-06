//go:build windows

package codefori

import (
	"errors"
	"strings"
	"testing"
)

func TestWindowsDescriptorReaderRejectsResidueBeforeReadingToken(t *testing.T) {
	valid := []byte(`{"version":1,"generation":"0123456789abcdef0123456789abcdef","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`)
	tests := []struct {
		name          string
		directorySafe bool
		fileSafe      bool
		body          []byte
		readErr       error
		wantRead      bool
		wantValid     bool
	}{
		{name: "valid protected descriptor", directorySafe: true, fileSafe: true, body: valid, wantRead: true, wantValid: true},
		{name: "wrong owner directory", fileSafe: true, body: valid},
		{name: "inherited directory ACE", fileSafe: true, body: valid},
		{name: "extra directory ACE", fileSafe: true, body: valid},
		{name: "permissive directory ACE", fileSafe: true, body: valid},
		{name: "directory reparse point", fileSafe: true, body: valid},
		{name: "wrong owner file", directorySafe: true, body: valid},
		{name: "inherited file ACE", directorySafe: true, body: valid},
		{name: "extra file ACE", directorySafe: true, body: valid},
		{name: "permissive file ACE", directorySafe: true, body: valid},
		{name: "file reparse point", directorySafe: true, body: valid},
		{name: "locked descriptor", directorySafe: true, fileSafe: true, readErr: errors.New("sharing violation"), wantRead: true},
		{name: "oversized descriptor", directorySafe: true, fileSafe: true, body: []byte(strings.Repeat("x", maxDescriptorBytes+1)), wantRead: true},
		{name: "malformed descriptor", directorySafe: true, fileSafe: true, body: []byte(`{"version":`), wantRead: true},
		{name: "wrong version", directorySafe: true, fileSafe: true, body: []byte(`{"version":2,"generation":"0123456789abcdef0123456789abcdef","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`), wantRead: true},
		{name: "wrong generation", directorySafe: true, fileSafe: true, body: []byte(`{"version":1,"generation":"stale","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`), wantRead: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := 0
			reader := newWindowsDescriptorReader(windowsDescriptorSource{
				path: func() (string, string, bool) {
					return `C:\\test\\BAC Nexus\\companion-v1`, `C:\\test\\BAC Nexus\\companion-v1\\descriptor.json`, true
				},
				validate: func(path string, directory bool) bool {
					if directory {
						return tt.directorySafe
					}
					return tt.fileSafe
				},
				readFile: func(string) ([]byte, error) {
					reads++
					return tt.body, tt.readErr
				},
			})

			descriptor, err := reader()
			if tt.wantValid {
				if err != nil {
					t.Fatalf("reader() error = %v", err)
				}
				if descriptor.Version != protocolVersion || descriptor.Generation != "0123456789abcdef0123456789abcdef" || descriptor.Token != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
					t.Fatalf("reader() descriptor = %#v, want validated descriptor", descriptor)
				}
			} else if err == nil || descriptor != (Descriptor{}) {
				t.Fatalf("reader() = (%#v, %v), want unavailable descriptor", descriptor, err)
			}
			if got := reads > 0; got != tt.wantRead {
				t.Fatalf("token reads = %t, want %t", got, tt.wantRead)
			}
		})
	}
}

func TestWindowsDescriptorReaderRejectsDuplicateFieldsAndTrailingBytes(t *testing.T) {
	valid := `{"version":1,"generation":"0123456789abcdef0123456789abcdef","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`
	for _, body := range []string{
		valid + " trailing",
		`{"version":1,"version":1,"generation":"0123456789abcdef0123456789abcdef","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`,
		`{"version":1,"generation":"0123456789abcdef0123456789abcdef","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","extra":true}`,
	} {
		reader := newWindowsDescriptorReader(windowsDescriptorSource{
			path:     func() (string, string, bool) { return "directory", "file", true },
			validate: func(string, bool) bool { return true },
			readFile: func(string) ([]byte, error) { return []byte(body), nil },
		})
		if descriptor, err := reader(); err == nil || descriptor != (Descriptor{}) {
			t.Fatalf("reader() = (%#v, %v), want malformed descriptor rejection", descriptor, err)
		}
	}
}
