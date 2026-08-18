package mapepire

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparedQueryIsProperlyJSONEncoded(t *testing.T) {
	request := PreparedQueryRequest("query-1", "select ? from x", 51, []string{"%A\"$B%"})
	encoded, err := Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(encoded, []byte{'\n'}) != 1 || !bytes.HasSuffix(encoded, []byte{'\n'}) {
		t.Fatalf("encoded frame is not one newline-delimited message: %q", encoded)
	}
	var decoded Request
	if err := DecodeFrame(bytes.NewReader(encoded), &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Parameters[0]; got != "%A\"$B%" {
		t.Fatalf("parameter = %q", got)
	}
}

func TestDecodeFrame(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		calls     int
		wantIDs   []string
		wantError string
	}{
		{
			name:    "valid frame",
			input:   "{\"id\":\"first\",\"type\":\"connect\"}\n",
			calls:   1,
			wantIDs: []string{"first"},
		},
		{
			name:      "missing terminator",
			input:     "{\"id\":\"first\",\"type\":\"connect\"}",
			calls:     1,
			wantError: "before newline terminator",
		},
		{
			name:    "multiple frames remain available to persistent reader",
			input:   "{\"id\":\"first\",\"type\":\"connect\"}\n{\"id\":\"second\",\"type\":\"exit\"}\n",
			calls:   2,
			wantIDs: []string{"first", "second"},
		},
		{
			name:      "malformed JSON",
			input:     "not-json\n",
			calls:     1,
			wantError: "invalid character",
		},
		{
			name:      "oversized frame",
			input:     strings.Repeat("x", MaxFrameBytes) + "\n",
			calls:     1,
			wantError: "exceeds byte limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			for call := 0; call < tt.calls; call++ {
				var got Request
				err := DecodeFrame(reader, &got)
				if tt.wantError != "" {
					if err == nil || !strings.Contains(err.Error(), tt.wantError) {
						t.Fatalf("DecodeFrame() error = %v, want error containing %q", err, tt.wantError)
					}
					return
				}
				if err != nil {
					t.Fatalf("DecodeFrame() error = %v", err)
				}
				if got.ID != tt.wantIDs[call] {
					t.Fatalf("DecodeFrame() ID = %q, want %q", got.ID, tt.wantIDs[call])
				}
			}
		})
	}
}

func TestVerifyServerJARRejectsDifferentArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mapepire-server-2.3.5.jar")
	if err := os.WriteFile(path, []byte("not the approved artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyServerJAR(path); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestJavaCommandUsesOnlyPinnedShape(t *testing.T) {
	command, err := JavaCommand("", "/home/NEXUS/"+RemoteJar)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"QIBM_JAVA_STDIO_CONVERT=N", "'/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit/bin/java'", "-Dos400.stdio.convert=N", "--single"} {
		if !strings.Contains(command, required) {
			t.Fatalf("command omitted %q: %s", required, command)
		}
	}
	if _, err := JavaCommand("/tmp/java;id", "/home/NEXUS/"+RemoteJar); err == nil {
		t.Fatal("expected unsafe Java path rejection")
	}
}
