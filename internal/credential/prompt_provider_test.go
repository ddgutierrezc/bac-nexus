package credential

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/profile"
)

func TestPromptProviderReturnsOnlyEphemeralPromptCredential(t *testing.T) {
	provided := []byte("prompt-secret")
	provider := NewPromptProvider(func(ctx context.Context, profileKey string) ([]byte, error) {
		if profileKey != "ibmi/dev" {
			t.Fatalf("profile key = %q, want ibmi/dev", profileKey)
		}
		return provided, nil
	})

	credential, err := provider.Get(context.Background(), "ibmi/dev", profile.CredentialModePrompt)
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if string(credential) != "prompt-secret" {
		t.Fatalf("credential = %q, want prompt input", credential)
	}
	if string(provided) != strings.Repeat("\x00", len(provided)) {
		t.Fatal("prompt input was not zeroized after handoff")
	}
	Zero(credential)
}

func TestPromptProviderFailsClosedWithoutLeakingPromptInput(t *testing.T) {
	secret := "do-not-leak"
	tests := []struct {
		name string
		ctx  context.Context
		mode profile.CredentialMode
		ask  PromptFunc
	}{
		{name: "denied", ctx: context.Background(), mode: profile.CredentialModePrompt, ask: func(context.Context, string) ([]byte, error) { return nil, errors.New(secret) }},
		{name: "empty", ctx: context.Background(), mode: profile.CredentialModePrompt, ask: func(context.Context, string) ([]byte, error) { return []byte{}, nil }},
		{name: "unavailable", ctx: context.Background(), mode: profile.CredentialModePrompt, ask: nil},
		{name: "wrong mode", ctx: context.Background(), mode: profile.CredentialModeKeyring, ask: func(context.Context, string) ([]byte, error) { return []byte(secret), nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credential, err := NewPromptProvider(tt.ask).Get(tt.ctx, "ibmi/dev", tt.mode)
			if !errors.Is(err, ErrCredentialsUnavailable) {
				t.Fatalf("error = %v, want credentials unavailable", err)
			}
			if credential != nil {
				t.Fatalf("credential = %q, want nil", credential)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error leaked prompt input: %q", err)
			}
		})
	}
}

func TestPromptProviderCancellationDoesNotPrompt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	credential, err := NewPromptProvider(func(context.Context, string) ([]byte, error) {
		called = true
		return []byte("prompt-secret"), nil
	}).Get(ctx, "ibmi/dev", profile.CredentialModePrompt)
	if !errors.Is(err, ErrCredentialsUnavailable) || credential != nil {
		t.Fatalf("Get() = %q, %v; want nil credentials unavailable", credential, err)
	}
	if called {
		t.Fatal("cancelled prompt provider called prompt callback")
	}
}
