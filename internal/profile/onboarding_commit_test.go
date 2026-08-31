package profile

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestOnboardingCommitCompensatesInReverseOrderAfterCommittedAuditFailure(t *testing.T) {
	var events []string
	transaction := OnboardingCommit{
		Prepare:      func(context.Context) error { events = append(events, "prepare"); return nil },
		StoreKeyring: func() error { events = append(events, "keyring-store"); return nil },
		SaveProfile:  func() error { events = append(events, "profile-save"); return nil },
		CommitPin:    func() error { events = append(events, "pin-commit"); return nil },
		AuditCommitted: func(context.Context) error {
			events = append(events, "audit-committed")
			return errors.New("audit unavailable")
		},
		RollbackPin:     func() error { events = append(events, "pin-rollback"); return nil },
		RollbackProfile: func() error { events = append(events, "profile-rollback"); return nil },
		RollbackKeyring: func() error { events = append(events, "keyring-rollback"); return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || result.CleanupRequired {
		t.Fatalf("Commit() = %#v", result)
	}
	want := []string{"prepare", "keyring-store", "profile-save", "pin-commit", "audit-committed", "pin-rollback", "profile-rollback", "keyring-rollback"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestOnboardingCommitRetainsJournalWhenCompensationFails(t *testing.T) {
	cleared := false
	transaction := OnboardingCommit{
		Prepare:         func(context.Context) error { return nil },
		StoreKeyring:    func() error { return nil },
		SaveProfile:     func() error { return nil },
		AuditCommitted:  func(context.Context) error { return errors.New("audit unavailable") },
		RollbackProfile: func() error { return errors.New("profile rollback unavailable") },
		ClearJournal:    func() error { cleared = true; return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || !result.CleanupRequired || result.Err == nil {
		t.Fatalf("Commit() = %#v", result)
	}
	if cleared {
		t.Fatal("journal was cleared despite failed compensation")
	}
}
