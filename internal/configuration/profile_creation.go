package configuration

import (
	"context"
	"errors"
	"sync"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
)

var (
	ErrCreateIdentityMismatch    = errors.New("profile create identity mismatch")
	ErrCredentialCleanupRequired = errors.New("credential cleanup required")
	ErrProfileAlreadyExists      = errors.New("profile already exists")
)

// CreateProfileRequest binds one save intent to an immutable non-secret draft.
type CreateProfileRequest struct {
	RequestID   string
	Generation  uint64
	DraftDigest string
	Profile     profile.Profile
}

// CreateProfileResult returns the exact saved profile and request identity.
type CreateProfileResult struct {
	RequestID   string
	Generation  uint64
	DraftDigest string
	Profile     profile.Profile
}

type createCall struct {
	request CreateProfileRequest
	done    chan struct{}
	result  CreateProfileResult
	err     error
}

// ProfileCreator coordinates prepared profile creation and idempotent request
// replay. It never accepts, stores, or returns credential bytes.
type ProfileCreator struct {
	profiles profile.Store
	mu       sync.Mutex
	requests map[string]*createCall
}

func NewProfileCreator(profiles profile.Store, _ CredentialProvisioner) *ProfileCreator {
	return &ProfileCreator{profiles: profiles, requests: make(map[string]*createCall)}
}

func (c *ProfileCreator) Create(ctx context.Context, request CreateProfileRequest) (CreateProfileResult, error) {
	if err := validateCreateRequest(request); err != nil {
		return CreateProfileResult{}, err
	}
	c.mu.Lock()
	if current, exists := c.requests[request.RequestID]; exists {
		if !sameCreateIdentity(current.request, request) {
			c.mu.Unlock()
			return CreateProfileResult{}, ErrCreateIdentityMismatch
		}
		done := current.done
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return CreateProfileResult{}, ctx.Err()
		case <-done:
			return current.result, current.err
		}
	}
	call := &createCall{request: request, done: make(chan struct{})}
	c.requests[request.RequestID] = call
	c.mu.Unlock()

	call.result, call.err = c.create(ctx, request)
	close(call.done)
	return call.result, call.err
}

func (c *ProfileCreator) create(ctx context.Context, request CreateProfileRequest) (CreateProfileResult, error) {
	if request.Profile.CredentialMode == profile.CredentialModeKeyring {
		return CreateProfileResult{}, ErrCredentialUnavailable
	}
	if request.Profile.CredentialMode != profile.CredentialModePrompt {
		return CreateProfileResult{}, ErrCredentialUnavailable
	}
	result := CreateProfileResult{RequestID: request.RequestID, Generation: request.Generation, DraftDigest: request.DraftDigest, Profile: request.Profile}
	err := c.profiles.WithPreparedCreateLock(ctx, request.Profile.Name, func() error {
		if journal, journalErr := c.profiles.ReadPreparedCreate(request.Profile.Name); journalErr == nil {
			if journal.CleanupRequired {
				return ErrCredentialCleanupRequired
			}
			return ErrCredentialCleanupRequired
		} else if !errors.Is(journalErr, profile.ErrPreparedCreateNotFound) {
			return ErrCredentialCleanupRequired
		}
		exists, err := c.profiles.Exists(request.Profile.Name)
		if err != nil {
			return ErrProfileAlreadyExists
		}
		if exists {
			return ErrProfileAlreadyExists
		}
		// Prompt mode deliberately persists no credential material. It is local-only
		// profile creation and does not contact IBM i or provision a native keyring.
		_, err = c.profiles.Save(request.Profile)
		return err
	})
	if err != nil {
		return CreateProfileResult{}, err
	}
	return result, nil
}

func validateCreateRequest(request CreateProfileRequest) error {
	if request.RequestID == "" || request.Generation == 0 || request.DraftDigest == "" || request.Profile.Validate() != nil {
		return ErrCreateIdentityMismatch
	}
	return nil
}

func sameCreateIdentity(left, right CreateProfileRequest) bool {
	return left.RequestID == right.RequestID && left.Generation == right.Generation && left.DraftDigest == right.DraftDigest && left.Profile == right.Profile
}

// CredentialProvisioner is retained as the composition boundary for a future
// explicitly approved secure-secret flow. Prompt-first creation never calls it.
type CredentialProvisioner interface {
	Status(string) (credential.Presence, error)
	Provision(context.Context, string) (CredentialProvisionResult, error)
}

type CredentialProvisionResult struct{ CleanupRequired bool }
