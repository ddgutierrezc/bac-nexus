package codefori

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/provider"
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

func TestFileTokenSourceSelectsOnlyOneEligibleV2Registration(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, tokenStateFilename)
	source := newFileTokenSource(func() (string, error) { return legacy, nil }).(fileTokenSource)
	directory := filepath.Join(root, registryDirectory)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRegistration(t, directory, "one", "127.0.0.1:41001", testToken(t), true, true, 1, time.Now().UnixMilli())
	target, ok := source.Target(context.Background())
	if !ok || target.endpoint != "http://127.0.0.1:41001" || target.token == "" {
		t.Fatalf("Target() = %#v, %v", target, ok)
	}
	writeRegistration(t, directory, "two", "127.0.0.1:41002", testToken(t), true, true, 2, time.Now().UnixMilli())
	if _, ok := source.Target(context.Background()); ok {
		t.Fatal("Target() selected ambiguous registrations")
	}
}

func TestFileTokenSourceUsesV1WhenV2RegistryIsEmptyOrExpiredOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, tokenStateFilename)
	writeTokenState(t, legacy, testToken(t))
	source := newFileTokenSource(func() (string, error) { return legacy, nil }).(fileTokenSource)
	if target, ok := source.Target(context.Background()); !ok || target.instance != "v1" {
		t.Fatalf("Target() = %#v, %v", target, ok)
	}
	directory := filepath.Join(root, registryDirectory)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRegistration(t, directory, "live", "127.0.0.1:41001", testToken(t), true, true, 1, time.Now().Add(-time.Minute).UnixMilli())
	if target, ok := source.Target(context.Background()); !ok || target.instance != "v1" {
		t.Fatal("expired-only v2 state did not permit v1 fallback")
	}
}

func TestFileTokenSourceFailsClosedForMalformedOrInsecureV2Sibling(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, tokenStateFilename)
	writeTokenState(t, legacy, testToken(t))
	directory := filepath.Join(root, registryDirectory)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRegistration(t, directory, "valid", "127.0.0.1:41001", testToken(t), true, true, 1, time.Now().UnixMilli())
	source := newFileTokenSource(func() (string, error) { return legacy, nil }).(fileTokenSource)
	for _, tt := range []struct {
		name  string
		write func(string)
	}{
		{"malformed", func(path string) { writeTokenBytes(t, path, []byte(`{"version":2}`)) }},
		{"insecure", func(path string) {
			writeRegistration(t, directory, "bad", "127.0.0.1:41002", testToken(t), true, true, 2, time.Now().UnixMilli())
			_ = os.Chmod(path, 0o644)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bad := filepath.Join(directory, "bad.json")
			_ = os.Remove(bad)
			tt.write(bad)
			if _, ok := source.Target(context.Background()); ok {
				t.Fatal("selected a sibling registration despite unsafe v2 state")
			}
		})
	}
}

func TestRegistrationEndpointValidationIsExactLoopback(t *testing.T) {
	valid := registrationState{Version: 2, Instance: base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes)), Token: testToken(t), Generation: 0, Connected: true, Focused: true, UpdatedAt: time.Now().UnixMilli(), LeaseMS: 30_000}
	for _, endpoint := range []string{"127.0.0.1:1", "127.0.0.1:65535"} {
		valid.Endpoint = endpoint
		if !validRegistration(nil, valid) {
			t.Fatalf("rejected %q", endpoint)
		}
	}
	for _, endpoint := range []string{"localhost:1", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:1/path", "127.0.0.1:1?x", "user@127.0.0.1:1"} {
		valid.Endpoint = endpoint
		if validRegistration(nil, valid) {
			t.Fatalf("accepted %q", endpoint)
		}
	}
}

func TestInvalidV2RegistrationPreventsHTTPFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, tokenStateFilename)
	directory := filepath.Join(root, registryDirectory)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRegistration(t, directory, "invalid-endpoint", "127.0.0.1:65536", testToken(t), true, true, 1, time.Now().UnixMilli())
	client := NewClient()
	client.tokens = newFileTokenSource(func() (string, error) { return legacy, nil })
	calls := 0
	client.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected HTTP") })
	result := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery})
	if result.State != provider.QueryUnavailable || calls != 0 {
		t.Fatalf("Query()=%#v calls=%d", result, calls)
	}
}

func writeRegistration(t *testing.T, directory, name, endpoint, token string, connected, focused bool, generation, updatedAt int64) {
	t.Helper()
	state := registrationState{Version: 2, Instance: base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes)), Endpoint: endpoint, Token: token, Generation: generation, Connected: connected, Focused: focused, UpdatedAt: updatedAt, LeaseMS: 30_000}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	writeTokenBytes(t, filepath.Join(directory, name+".json"), data)
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
