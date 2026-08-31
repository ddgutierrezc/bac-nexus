package remote

import (
	"context"
	"io"
	"os"
	"testing"
)

func TestSecretPromptCaptureFailsBeforeReadingWithoutTerminalFiles(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	calls := 0
	prompt := SecretPrompt{
		Input: input, Output: output,
		IsTerminal: func(int) bool { return false },
		Read: func(int) ([]byte, error) {
			calls++
			return []byte("secret"), nil
		},
	}
	secret, code := prompt.Capture(context.Background(), input, output, "IBM i password")
	if code != PromptTerminalUnavailable || secret != nil || calls != 0 {
		t.Fatalf("Capture() = %q, %v, calls=%d", code, secret, calls)
	}
}

func TestSecretPromptCaptureZeroesRejectedAndCancelledInput(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	for _, tt := range []struct {
		name string
		err  error
		code PromptCode
	}{
		{name: "EOF", err: io.EOF, code: PromptEOF},
		{name: "interrupt", err: context.Canceled, code: PromptCancelled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			secretBytes := []byte("secret")
			prompt := SecretPrompt{
				Input: input, Output: output,
				IsTerminal: func(int) bool { return true },
				Read:       func(int) ([]byte, error) { return secretBytes, tt.err },
			}
			secret, code := prompt.Capture(context.Background(), input, output, "IBM i password")
			if code != tt.code || secret != nil {
				t.Fatalf("Capture() = %q, %v", code, secret)
			}
			for i, value := range secretBytes {
				if value != 0 {
					t.Fatalf("secretBytes[%d] = %d, want zero", i, value)
				}
			}
		})
	}
}
