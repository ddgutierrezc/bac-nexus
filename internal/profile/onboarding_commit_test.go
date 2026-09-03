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

func TestOnboardingCommitCompensatesPartialCredentialFailureBeforeProfilePersistence(t *testing.T) {
	var events []string
	transaction := OnboardingCommit{
		Prepare:      func(context.Context) error { events = append(events, "prepare"); return nil },
		StoreKeyring: func() error { events = append(events, "keyring-store"); return errors.New("keyring write failed") },
		RollbackKeyring: func() error {
			events = append(events, "keyring-rollback")
			return nil
		},
		SaveProfile:  func() error { t.Fatal("profile must not persist after credential failure"); return nil },
		ClearJournal: func() error { events = append(events, "journal-clear"); return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || result.CleanupRequired || result.Err == nil {
		t.Fatalf("Commit() = %#v", result)
	}
	want := []string{"prepare", "keyring-store", "keyring-rollback", "journal-clear"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestOnboardingCommitPersistsEligibilityBeforeCommittedAuditAndCompensatesInReverseOrder(t *testing.T) {
	var events []string
	transaction := OnboardingCommit{
		Prepare:             func(context.Context) error { events = append(events, "prepare"); return nil },
		StoreKeyring:        func() error { events = append(events, "keyring"); return nil },
		SaveProfile:         func() error { events = append(events, "profile"); return nil },
		CommitPin:           func() error { events = append(events, "pin"); return nil },
		SaveEligibility:     func() error { events = append(events, "eligibility"); return nil },
		AuditCommitted:      func(context.Context) error { events = append(events, "audit"); return errors.New("audit unavailable") },
		RollbackEligibility: func() error { events = append(events, "eligibility-rollback"); return nil },
		RollbackPin:         func() error { events = append(events, "pin-rollback"); return nil },
		RollbackProfile:     func() error { events = append(events, "profile-rollback"); return nil },
		RollbackKeyring:     func() error { events = append(events, "keyring-rollback"); return nil },
		ClearJournal:        func() error { events = append(events, "clear"); return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || result.CleanupRequired || result.Err == nil {
		t.Fatalf("Commit() = %#v", result)
	}
	want := []string{"prepare", "keyring", "profile", "pin", "eligibility", "audit", "eligibility-rollback", "pin-rollback", "profile-rollback", "keyring-rollback", "clear"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestOnboardingCommitRetainsJournalWhenEligibilityRollbackIsUncertain(t *testing.T) {
	cleared := false
	transaction := OnboardingCommit{
		Prepare:             func(context.Context) error { return nil },
		SaveEligibility:     func() error { return nil },
		AuditCommitted:      func(context.Context) error { return errors.New("audit unavailable") },
		RollbackEligibility: func() error { return errors.New("eligibility rollback unavailable") },
		ClearJournal:        func() error { cleared = true; return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || !result.CleanupRequired || result.Err == nil {
		t.Fatalf("Commit() = %#v", result)
	}
	if cleared {
		t.Fatal("journal was cleared after uncertain eligibility recovery")
	}
}

func TestOnboardingCommitRecordsEligibilityJournalPhasesBeforeEachDurableStep(t *testing.T) {
	var events []string
	transaction := OnboardingCommit{
		Prepare:      func(context.Context) error { events = append(events, "prepare"); return nil },
		RecordPhase:  func(phase PreparedCreatePhase) error { events = append(events, string(phase)); return nil },
		StoreKeyring: func() error { events = append(events, "keyring"); return nil },
		SaveProfile:  func() error { events = append(events, "profile"); return nil },
		CommitPin:    func() error { events = append(events, "pin"); return nil },
		SaveEligibility: func() error {
			events = append(events, "eligibility")
			return nil
		},
		AuditCommitted: func(context.Context) error { events = append(events, "audit"); return nil },
		ClearJournal:   func() error { events = append(events, "clear"); return nil },
	}
	if result := transaction.Commit(context.Background()); !result.Saved {
		t.Fatalf("Commit() = %#v", result)
	}
	want := []string{"prepare", string(PreparedCreateKeyring), "keyring", string(PreparedCreateProfile), "profile", string(PreparedCreatePin), "pin", string(PreparedCreateEligibility), "eligibility", string(PreparedCreateCommittedAudit), "audit", "clear"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestOnboardingCommitRevokesChangedEligibilityBeforeReplacementAndRestoresItOnlyAfterProvenRollback(t *testing.T) {
	var events []string
	transaction := OnboardingCommit{
		Prepare:                 func(context.Context) error { events = append(events, "prepare"); return nil },
		RevokePriorEligibility:  func() error { events = append(events, "revoke-prior"); return nil },
		StoreKeyring:            func() error { events = append(events, "keyring"); return nil },
		SaveProfile:             func() error { events = append(events, "profile"); return errors.New("profile failed") },
		RollbackKeyring:         func() error { events = append(events, "keyring-rollback"); return nil },
		RestorePriorEligibility: func() error { events = append(events, "restore-prior"); return nil },
		ClearJournal:            func() error { events = append(events, "clear"); return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || result.CleanupRequired || result.Err == nil {
		t.Fatalf("Commit() = %#v", result)
	}
	want := []string{"prepare", "revoke-prior", "keyring", "profile", "keyring-rollback", "restore-prior", "clear"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestOnboardingCommitRetainsJournalRatherThanClaimingPriorEligibilityRestoredAfterFailedRollback(t *testing.T) {
	cleared, restored := false, false
	transaction := OnboardingCommit{
		Prepare:                func(context.Context) error { return nil },
		RevokePriorEligibility: func() error { return nil },
		StoreKeyring:           func() error { return nil },
		SaveProfile:            func() error { return errors.New("profile failed") },
		RollbackKeyring:        func() error { return errors.New("keyring rollback failed") },
		RestorePriorEligibility: func() error {
			restored = true
			return nil
		},
		ClearJournal: func() error { cleared = true; return nil },
	}
	result := transaction.Commit(context.Background())
	if result.Saved || !result.CleanupRequired || result.Err == nil || restored || cleared {
		t.Fatalf("Commit() = %#v, restored=%t cleared=%t", result, restored, cleared)
	}
}

func TestOnboardingCommitCancellationAfterKeyringCompensatesWithoutReplacingProfile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	profileSaved, keyringRolledBack := false, false
	transaction := OnboardingCommit{
		Prepare: func(context.Context) error { return nil },
		RecordPhase: func(phase PreparedCreatePhase) error {
			if phase == PreparedCreateProfile {
				cancel()
			}
			return nil
		},
		StoreKeyring:    func() error { return nil },
		SaveProfile:     func() error { profileSaved = true; return nil },
		RollbackKeyring: func() error { keyringRolledBack = true; return nil },
		ClearJournal:    func() error { return nil },
	}
	result := transaction.Commit(ctx)
	if result.Saved || result.CleanupRequired || !errors.Is(result.Err, context.Canceled) || profileSaved || !keyringRolledBack {
		t.Fatalf("Commit() = %#v, profileSaved=%t keyringRolledBack=%t", result, profileSaved, keyringRolledBack)
	}
}
