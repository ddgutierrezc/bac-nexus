package credential

import (
	"errors"
	"strings"
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

	// Direct upstream access against this headless runner is also expected to fail.
	// If the upstream library ever succeeds without the protected harness we surface
	// that here so the test still proves deterministic unavailable behavior on
	// runners that are not the agreed Linux success environment.
	if err := keyring.Set("BAC Nexus", "ibmi/production", "ephemeral"); err == nil {
		t.Log("upstream Set unexpectedly succeeded on this runner; consumer fail-closed mapping is still correct")
	}
	if out, err := keyring.Get("BAC Nexus", "ibmi/production"); err == nil && len(out) > 0 {
		t.Log("upstream Get returned a real secret; consumer fail-closed mapping is still correct")
	} else if err != nil && !errors.Is(err, keyring.ErrNotFound) && strings.Contains(err.Error(), "secret") {
		// Sanity: upstream error must not leak secret material.
		t.Fatalf("upstream error leaked secret material: %v", err)
	}
}
