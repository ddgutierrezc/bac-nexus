package credential

import (
	"context"
	"strings"

	"bac-nexus/internal/profile"
)

// PromptFunc obtains one credential without owning its persistence or use.
// Implementations must return a newly allocated buffer that the provider can
// zero after transferring it into the orchestration-owned boundary.
type PromptFunc func(context.Context, string) ([]byte, error)

// PromptProvider implements the prompt-only branch of the Step 8 credential
// boundary. It neither persists credentials nor exposes prompt failures.
type PromptProvider struct {
	prompt PromptFunc
}

func NewPromptProvider(prompt PromptFunc) PromptProvider {
	return PromptProvider{prompt: prompt}
}

// Get retrieves one short-lived prompt credential for a validated profile key.
// Every unavailable, denied, empty, invalid, or cancelled input maps to the
// same deterministic public error and returns no credential material.
func (p PromptProvider) Get(ctx context.Context, profileKey string, mode profile.CredentialMode) ([]byte, error) {
	if ctx.Err() != nil || mode != profile.CredentialModePrompt || !validPromptKey(profileKey) || p.prompt == nil {
		return nil, ErrCredentialsUnavailable
	}
	input, err := p.prompt(ctx, profileKey)
	if err != nil || ctx.Err() != nil || !validSecret(input) {
		Zero(input)
		return nil, ErrCredentialsUnavailable
	}
	credential := append([]byte(nil), input...)
	Zero(input)
	return credential, nil
}

func validPromptKey(key string) bool {
	name, ok := strings.CutPrefix(key, "ibmi/")
	return ok && name != "" && !strings.Contains(name, "/") && validateProfileName(name) == nil
}
