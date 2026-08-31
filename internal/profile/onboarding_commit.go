package profile

import (
	"context"
	"errors"
)

// OnboardingCommit keeps the mutation and compensation order explicit for a
// prepared onboarding transaction. Callback implementations own concrete
// profile, pin, keyring, audit, and journal persistence.
type OnboardingCommit struct {
	Prepare        func(context.Context) error
	StoreKeyring   func() error
	SaveProfile    func() error
	CommitPin      func() error
	AuditCommitted func(context.Context) error

	RollbackPin     func() error
	RollbackProfile func() error
	RollbackKeyring func() error
	ClearJournal    func() error
}

type OnboardingCommitResult struct {
	Saved           bool
	CleanupRequired bool
	Err             error
}

// Commit writes credential, profile, and pin state before the mandatory
// committed audit. Any later failure is compensated in reverse order. The
// journal is only cleared after a complete successful transaction or complete
// compensation.
func (c OnboardingCommit) Commit(ctx context.Context) OnboardingCommitResult {
	if err := callContext(ctx, c.Prepare); err != nil {
		return OnboardingCommitResult{Err: err}
	}
	if err := call(c.StoreKeyring); err != nil {
		return OnboardingCommitResult{Err: err}
	}
	keyringStored := c.StoreKeyring != nil
	if err := call(c.SaveProfile); err != nil {
		return c.compensate(keyringStored, false, false, err)
	}
	profileSaved := c.SaveProfile != nil
	if err := call(c.CommitPin); err != nil {
		return c.compensate(keyringStored, profileSaved, false, err)
	}
	pinCommitted := c.CommitPin != nil
	if err := callContext(ctx, c.AuditCommitted); err != nil {
		return c.compensate(keyringStored, profileSaved, pinCommitted, err)
	}
	if err := call(c.ClearJournal); err != nil {
		return OnboardingCommitResult{Err: err, CleanupRequired: true}
	}
	return OnboardingCommitResult{Saved: true}
}

func (c OnboardingCommit) compensate(keyringStored, profileSaved, pinCommitted bool, cause error) OnboardingCommitResult {
	var rollbackErr error
	if pinCommitted {
		rollbackErr = errors.Join(rollbackErr, call(c.RollbackPin))
	}
	if profileSaved {
		rollbackErr = errors.Join(rollbackErr, call(c.RollbackProfile))
	}
	if keyringStored {
		rollbackErr = errors.Join(rollbackErr, call(c.RollbackKeyring))
	}
	if rollbackErr != nil {
		return OnboardingCommitResult{CleanupRequired: true, Err: errors.Join(cause, rollbackErr)}
	}
	if err := call(c.ClearJournal); err != nil {
		return OnboardingCommitResult{CleanupRequired: true, Err: errors.Join(cause, err)}
	}
	return OnboardingCommitResult{Err: cause}
}

func call(action func() error) error {
	if action == nil {
		return nil
	}
	return action()
}

func callContext(ctx context.Context, action func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if action == nil {
		return nil
	}
	return action(ctx)
}
