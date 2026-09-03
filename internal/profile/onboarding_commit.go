package profile

import (
	"context"
	"errors"
)

// OnboardingCommit keeps the mutation and compensation order explicit for a
// prepared onboarding transaction. Callback implementations own concrete
// profile, pin, keyring, audit, and journal persistence.
type OnboardingCommit struct {
	Prepare                func(context.Context) error
	RecordPhase            func(PreparedCreatePhase) error
	RevokePriorEligibility func() error
	StoreKeyring           func() error
	SaveProfile            func() error
	CommitPin              func() error
	SaveEligibility        func() error
	AuditCommitted         func(context.Context) error

	RollbackEligibility     func() error
	RollbackPin             func() error
	RollbackProfile         func() error
	RollbackKeyring         func() error
	RestorePriorEligibility func() error
	ClearJournal            func() error
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
	if err := call(c.RevokePriorEligibility); err != nil {
		return OnboardingCommitResult{Err: err, CleanupRequired: true}
	}
	priorEligibilityRevoked := c.RevokePriorEligibility != nil
	if err := c.record(ctx, PreparedCreateKeyring); err != nil {
		return c.compensate(false, false, false, false, priorEligibilityRevoked, err)
	}
	if err := call(c.StoreKeyring); err != nil {
		// A credential backend can fail after mutating its durable state. Treat
		// the write as partial until its compensator proves otherwise.
		return c.compensate(c.StoreKeyring != nil, false, false, false, priorEligibilityRevoked, err)
	}
	keyringStored := c.StoreKeyring != nil
	if err := c.record(ctx, PreparedCreateProfile); err != nil {
		return c.compensate(keyringStored, false, false, false, priorEligibilityRevoked, err)
	}
	if err := call(c.SaveProfile); err != nil {
		return c.compensate(keyringStored, false, false, false, priorEligibilityRevoked, err)
	}
	profileSaved := c.SaveProfile != nil
	if err := c.record(ctx, PreparedCreatePin); err != nil {
		return c.compensate(keyringStored, profileSaved, false, false, priorEligibilityRevoked, err)
	}
	if err := call(c.CommitPin); err != nil {
		return c.compensate(keyringStored, profileSaved, false, false, priorEligibilityRevoked, err)
	}
	pinCommitted := c.CommitPin != nil
	if err := c.record(ctx, PreparedCreateEligibility); err != nil {
		return c.compensate(keyringStored, profileSaved, pinCommitted, false, priorEligibilityRevoked, err)
	}
	if err := call(c.SaveEligibility); err != nil {
		return c.compensate(keyringStored, profileSaved, pinCommitted, c.SaveEligibility != nil, priorEligibilityRevoked, err)
	}
	eligibilitySaved := c.SaveEligibility != nil
	if err := c.record(ctx, PreparedCreateCommittedAudit); err != nil {
		return c.compensate(keyringStored, profileSaved, pinCommitted, eligibilitySaved, priorEligibilityRevoked, err)
	}
	if err := callContext(ctx, c.AuditCommitted); err != nil {
		return c.compensate(keyringStored, profileSaved, pinCommitted, eligibilitySaved, priorEligibilityRevoked, err)
	}
	if err := call(c.ClearJournal); err != nil {
		return OnboardingCommitResult{Err: err, CleanupRequired: true}
	}
	return OnboardingCommitResult{Saved: true}
}

func (c OnboardingCommit) record(ctx context.Context, phase PreparedCreatePhase) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.RecordPhase == nil {
		return nil
	}
	if err := c.RecordPhase(phase); err != nil {
		return err
	}
	return ctx.Err()
}

func (c OnboardingCommit) compensate(keyringStored, profileSaved, pinCommitted, eligibilitySaved, priorEligibilityRevoked bool, cause error) OnboardingCommitResult {
	var rollbackErr error
	if eligibilitySaved {
		rollbackErr = errors.Join(rollbackErr, call(c.RollbackEligibility))
	}
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
	if priorEligibilityRevoked {
		if err := call(c.RestorePriorEligibility); err != nil {
			return OnboardingCommitResult{CleanupRequired: true, Err: errors.Join(cause, err)}
		}
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
