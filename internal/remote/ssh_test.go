package remote

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestFingerprintCallbackFailsClosed(t *testing.T) {
	key := testPublicKey(t)
	address := &net.TCPAddr{}
	tests := []struct {
		name     string
		expected string
		wantErr  error
		mismatch bool
	}{
		{"match", ssh.FingerprintSHA256(key), nil, false},
		{"unknown", "", ErrUnknownHostKey, false},
		{"mismatch", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FingerprintCallback(tt.expected)("host:22", address, key)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			var mismatch *HostKeyMismatchError
			if errors.As(err, &mismatch) != tt.mismatch {
				t.Fatalf("mismatch error = %v, want %v", err, tt.mismatch)
			}
			if tt.wantErr == nil && !tt.mismatch && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPasswordPromptRequiresTerminal(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	readCalled := false
	prompt := PasswordPrompt{
		Input: input, Output: io.Discard,
		IsTerminal: func(int) bool { return false },
		Read:       func(int) ([]byte, error) { readCalled = true; return nil, nil },
	}
	if _, err := prompt.Prompt("test"); err == nil {
		t.Fatal("expected non-terminal rejection")
	}
	if readCalled {
		t.Fatal("password reader called for non-terminal input")
	}
}

func TestPasswordPromptReadsHiddenTerminalWithoutLoggingValue(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var output strings.Builder
	prompt := PasswordPrompt{
		Input: input, Output: &output,
		IsTerminal: func(int) bool { return true },
		Read:       func(int) ([]byte, error) { return []byte{'x', 'y'}, nil },
	}
	password, err := prompt.Prompt("test-profile")
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(password)
	if len(password) != 2 {
		t.Fatalf("password length = %d", len(password))
	}
	if strings.Contains(output.String(), string(password)) {
		t.Fatal("password value was written to prompt output")
	}
}

func TestPasswordBytesAreZeroed(t *testing.T) {
	value := []byte{'x', 'y'}
	Zero(value)
	if value[0] != 0 || value[1] != 0 {
		t.Fatal("password bytes were not zeroed")
	}
}
