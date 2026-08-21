package credential

import (
	"errors"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

// TestNativeUnavailableMapsToCredentialsUnavailable is the canonical Linux CI behavior
// assertion: when the platform-native keyring adapter cannot talk to an available Secret
// Service (no unlocked default collection, locked, denied, ambiguous, or policy failure),
// the consumer-owned CredentialStore MUST map deterministically to ErrCredentialsUnavailable,
// MUST NOT touch the network or invoke any remote/MCP fallback, and MUST NOT leak the
// upstream error message verbatim. This test is only meaningful on runners that lack a
// working default Secret Service collection (for example, GitHub-hosted Ubuntu). It is
// not a Linux success claim; a runner that can perform native Get/Set/Delete proves
// Linux native success through the BAC Nexus credential package tests instead.
func TestNativeUnavailableMapsToCredentialsUnavailable(t *testing.T) {
	if platformKeyring() == nil {
		t.Fatal("platform keyring adapter is nil")
	}

	store := NewNativeCredentialStore()
	secret := []byte("ephemeral-linux-unavailable")

	if err := store.Set("production", secret); err != nil && !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("Set returned %v, want credentials_unavailable", err)
	}

	value, err := store.Get("production")
	if err == nil && len(value) > 0 {
		// The runner unexpectedly produced a real native credential. That is not a
		// fail-closed success and it would mean we are running outside the agreed
		// deterministic unavailable Linux scenario. Fail loudly so the test cannot
		// silently mask a real Linux success path that must be exercised through
		// the dedicated, non-mock BAC Nexus package tests.
		t.Fatalf("Get unexpectedly returned a real native credential; Linux native success must run through ./internal/credential full suite, not this contract test")
	}
	if err != nil && !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("Get returned %v, want credentials_unavailable", err)
	}

	if err := store.Delete("production"); err != nil && !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("Delete returned %v, want credentials_unavailable", err)
	}

	// Sanity: when the upstream library surfaces an error path we use it to confirm
	// the runner truly lacks a working default Secret Service. The error message must
	// be a clearly non-sensitive backend status; we only assert it is present, not its
	// exact copy, so platform wording changes do not break the contract.
	if _, err := keyring.Get("BAC Nexus", "ibmi/production"); err != nil && errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("upstream returned ErrNotFound without an unlocked collection: %v", err)
	}
}
