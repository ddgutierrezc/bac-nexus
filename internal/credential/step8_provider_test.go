package credential

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/profile"
)

func TestStep8ProviderDispatchesOnlySavedPromptAndKeyringModes(t *testing.T) {
	promptInput := []byte("prompt-secret")
	keyringInput := []byte("keyring-secret")
	promptCalls, keyringCalls := 0, 0
	provider := NewStep8Provider(
		step8PromptFunc(func(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
			promptCalls++
			if key != "ibmi/dev" || mode != profile.CredentialModePrompt {
				t.Fatalf("prompt request = (%q, %q), want (ibmi/dev, prompt)", key, mode)
			}
			return promptInput, nil
		}),
		step8KeyringFunc(func(name string) ([]byte, error) {
			keyringCalls++
			if name != "dev" {
				t.Fatalf("keyring profile = %q, want dev", name)
			}
			return keyringInput, nil
		}),
	)

	credential, err := provider.Get(context.Background(), "ibmi/dev", profile.CredentialModePrompt)
	if err != nil {
		t.Fatalf("prompt Get error = %v", err)
	}
	if string(credential) != "prompt-secret" {
		t.Fatalf("prompt credential = %q", credential)
	}
	if &credential[0] == &promptInput[0] || string(promptInput) != strings.Repeat("\x00", len(promptInput)) {
		t.Fatal("prompt credential was not copied into a caller-owned buffer")
	}
	Zero(credential)

	credential, err = provider.Get(context.Background(), "ibmi/dev", profile.CredentialModeKeyring)
	if err != nil {
		t.Fatalf("keyring Get error = %v", err)
	}
	if string(credential) != "keyring-secret" {
		t.Fatalf("keyring credential = %q", credential)
	}
	if &credential[0] == &keyringInput[0] || string(keyringInput) != strings.Repeat("\x00", len(keyringInput)) {
		t.Fatal("keyring credential was not copied into a caller-owned buffer")
	}
	Zero(credential)
	if promptCalls != 1 || keyringCalls != 1 {
		t.Fatalf("calls = prompt:%d keyring:%d, want one each", promptCalls, keyringCalls)
	}
}

func TestStep8ProviderFailsClosedWithoutFallbackOrLeaks(t *testing.T) {
	secret := "do-not-leak"
	tests := []struct {
		name       string
		ctx        context.Context
		key        string
		mode       profile.CredentialMode
		prompt     step8Prompt
		keyring    step8Keyring
		wantPrompt int
		wantStore  int
	}{
		{name: "prompt denied", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialModePrompt, prompt: step8PromptFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) { return nil, errors.New(secret) }), wantPrompt: 1},
		{name: "prompt unavailable", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialModePrompt},
		{name: "prompt empty", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialModePrompt, prompt: step8PromptFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) { return []byte{}, nil }), wantPrompt: 1},
		{name: "keyring unavailable", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialModeKeyring},
		{name: "keyring not found", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialModeKeyring, keyring: step8KeyringFunc(func(string) ([]byte, error) { return nil, errors.New(secret) }), wantStore: 1},
		{name: "keyring empty", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialModeKeyring, keyring: step8KeyringFunc(func(string) ([]byte, error) { return []byte{}, nil }), wantStore: 1},
		{name: "invalid mode", ctx: context.Background(), key: "ibmi/dev", mode: profile.CredentialMode("vault")},
		{name: "invalid key", ctx: context.Background(), key: "ibmi/dev/other", mode: profile.CredentialModePrompt},
		{name: "cancelled", ctx: cancelledStep8Context(), key: "ibmi/dev", mode: profile.CredentialModeKeyring},
		{name: "deadline exceeded", ctx: expiredStep8Context(), key: "ibmi/dev", mode: profile.CredentialModePrompt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptCalls, keyringCalls := 0, 0
			prompt := tt.prompt
			if prompt != nil {
				prompt = step8PromptFunc(func(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
					promptCalls++
					return tt.prompt.Get(ctx, key, mode)
				})
			}
			keyring := tt.keyring
			if keyring != nil {
				keyring = step8KeyringFunc(func(name string) ([]byte, error) {
					keyringCalls++
					return tt.keyring.Get(name)
				})
			}
			if tt.mode == profile.CredentialModePrompt && keyring == nil {
				keyring = step8KeyringFunc(func(string) ([]byte, error) {
					keyringCalls++
					return []byte(secret), nil
				})
			}
			if tt.mode == profile.CredentialModeKeyring && prompt == nil {
				prompt = step8PromptFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
					promptCalls++
					return []byte(secret), nil
				})
			}
			credential, err := NewStep8Provider(prompt, keyring).Get(tt.ctx, tt.key, tt.mode)
			if !errors.Is(err, ErrCredentialsUnavailable) || credential != nil {
				t.Fatalf("Get() = %q, %v; want nil credentials_unavailable", credential, err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error leaked backend detail: %q", err)
			}
			if promptCalls != tt.wantPrompt || keyringCalls != tt.wantStore {
				t.Fatalf("calls = prompt:%d keyring:%d, want prompt:%d keyring:%d", promptCalls, keyringCalls, tt.wantPrompt, tt.wantStore)
			}
		})
	}
}

func cancelledStep8Context() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredStep8Context() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	cancel()
	return ctx
}

type step8Prompt interface {
	Get(context.Context, string, profile.CredentialMode) ([]byte, error)
}

type step8PromptFunc func(context.Context, string, profile.CredentialMode) ([]byte, error)

func (f step8PromptFunc) Get(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
	return f(ctx, key, mode)
}

type step8Keyring interface {
	Get(string) ([]byte, error)
}

type step8KeyringFunc func(string) ([]byte, error)

func (f step8KeyringFunc) Get(profile string) ([]byte, error) { return f(profile) }
