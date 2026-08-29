package credential

import (
	"context"
	"strings"

	"bac-nexus/internal/profile"
)

// Step8Prompt is the prompt branch consumed by the Step 8 credential dispatcher.
type Step8Prompt interface {
	Get(context.Context, string, profile.CredentialMode) ([]byte, error)
}

// Step8Keyring is the native-keyring branch consumed by the Step 8 credential dispatcher.
type Step8Keyring interface {
	Get(string) ([]byte, error)
}

// Step8Provider dispatches the two saved-profile Step 8 credential modes.
// It is structurally compatible with configuration.CredentialProvider without
// importing configuration and creating a package cycle.
type Step8Provider struct {
	prompt  Step8Prompt
	keyring Step8Keyring
}

func NewStep8Provider(prompt Step8Prompt, keyring Step8Keyring) Step8Provider {
	return Step8Provider{prompt: prompt, keyring: keyring}
}

// Get returns a fresh caller-owned credential buffer. All unavailable or
// invalid states collapse to ErrCredentialsUnavailable without fallback.
func (p Step8Provider) Get(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ErrCredentialsUnavailable
	}
	profileName, ok := step8ProfileName(key)
	if !ok {
		return nil, ErrCredentialsUnavailable
	}

	var (
		input []byte
		err   error
	)
	switch mode {
	case profile.CredentialModePrompt:
		if p.prompt == nil {
			return nil, ErrCredentialsUnavailable
		}
		input, err = p.prompt.Get(ctx, key, mode)
	case profile.CredentialModeKeyring:
		if p.keyring == nil {
			return nil, ErrCredentialsUnavailable
		}
		input, err = p.keyring.Get(profileName)
	default:
		return nil, ErrCredentialsUnavailable
	}
	if err != nil || ctx.Err() != nil || !validSecret(input) {
		Zero(input)
		return nil, ErrCredentialsUnavailable
	}
	credential := append([]byte(nil), input...)
	Zero(input)
	return credential, nil
}

func step8ProfileName(key string) (string, bool) {
	name, ok := strings.CutPrefix(key, "ibmi/")
	if !ok || name == "" {
		return "", false
	}
	expected, err := KeyForProfile(name)
	return name, err == nil && expected == key
}
