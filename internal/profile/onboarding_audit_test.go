package profile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboardingAuditStorePersistsOnlyApprovedSecretFreeEvents(t *testing.T) {
	root := t.TempDir()
	store := OnboardingAuditStore{Profiles: Store{Root: root}}
	if err := store.Record(context.Background(), "user-host", "identity_bootstrap_allowed"); err != nil {
		t.Fatalf("Record bootstrap: %v", err)
	}
	if err := store.Record(context.Background(), "user-host", "identity_pin_committed"); err != nil {
		t.Fatalf("Record committed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".onboarding-audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "identity_bootstrap_allowed") || !strings.Contains(text, "identity_pin_committed") || strings.Contains(text, "password") {
		t.Fatalf("unexpected durable audit content: %q", text)
	}
}

func TestOnboardingAuditStoreRejectsUnknownEvent(t *testing.T) {
	store := OnboardingAuditStore{Profiles: Store{Root: t.TempDir()}}
	if err := store.Record(context.Background(), "user-host", "password=unsafe"); err == nil {
		t.Fatal("unknown audit event was accepted")
	}
}
