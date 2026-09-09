package codefori

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileTokenSourceAcceptsOnlyPrivateVersionedState(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, tokenStateFilename)
	source := newFileTokenSource(func() (string, error) { return path, nil })

	writeTokenState(t, path, testToken(t))
	if _, ok := source.Token(context.Background()); !ok {
		t.Fatal("Token() rejected valid private state")
	}

	for _, tt := range []struct {
		name  string
		write func()
	}{
		{"malformed", func() { writeTokenBytes(t, path, []byte(`{"version":1,"token":true}`)) }},
		{"unknown field", func() { writeTokenBytes(t, path, []byte(`{"version":1,"token":"invalid","extra":true}`)) }},
		{"duplicate field", func() { writeTokenBytes(t, path, []byte(`{"version":1,"version":1,"token":"invalid"}`)) }},
		{"oversized", func() { writeTokenBytes(t, path, []byte(strings.Repeat("x", maxTokenStateBytes+1))) }},
		{"non regular", func() {
			_ = os.Remove(path)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.RemoveAll(path)
			tt.write()
			if token, ok := source.Token(context.Background()); ok || token != "" {
				t.Fatalf("Token() = %q, %v; want rejection", token, ok)
			}
		})
	}

	t.Run("symlink", func(t *testing.T) {
		_ = os.RemoveAll(path)
		target := filepath.Join(directory, "target")
		writeTokenState(t, target, testToken(t))
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, ok := source.Token(context.Background()); ok {
			t.Fatal("Token() accepted a symlink")
		}
	})

	if runtime.GOOS != "windows" {
		t.Run("insecure permissions", func(t *testing.T) {
			_ = os.RemoveAll(path)
			writeTokenState(t, path, testToken(t))
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, ok := source.Token(context.Background()); ok {
				t.Fatal("Token() accepted group or world readable state")
			}
		})
	}
}

func TestFileTokenSourceRejectsMissingAndCancelledState(t *testing.T) {
	source := newFileTokenSource(func() (string, error) { return filepath.Join(t.TempDir(), tokenStateFilename), nil })
	if _, ok := source.Token(context.Background()); ok {
		t.Fatal("Token() accepted missing state")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := source.Token(ctx); ok {
		t.Fatal("Token() accepted cancelled context")
	}
}

func writeTokenState(t *testing.T, path, token string) {
	t.Helper()
	data, err := json.Marshal(tokenState{Version: 1, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	writeTokenBytes(t, path, data)
}

func writeTokenBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func testToken(t *testing.T) string {
	t.Helper()
	data := make([]byte, tokenBytes)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
