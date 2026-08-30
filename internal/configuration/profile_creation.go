package configuration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// CredentialProvisioner keeps transient credential handling outside profile
// creation requests and results. It must not return secret material.
type CredentialProvisioner interface {
	Status(string) (credential.Presence, error)
	Provision(context.Context, string) (CredentialProvisionResult, error)
}

// CredentialProvisionResult communicates only recovery state. Ownership tokens
// are intentionally not retained because the native keyring cannot conditionally
// delete a credential by ownership.
type CredentialProvisionResult struct {
	CleanupRequired bool
}

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
	profiles    profile.Store
	credentials CredentialProvisioner

	mu       sync.Mutex
	requests map[string]*createCall
}

func NewProfileCreator(profiles profile.Store, credentials CredentialProvisioner) *ProfileCreator {
	return &ProfileCreator{profiles: profiles, credentials: credentials, requests: make(map[string]*createCall)}
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
	if c.credentials == nil {
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

		journal := profile.PreparedCreateJournal{
			Profile: request.Profile.Name, TransactionID: createTransactionID(), Phase: profile.PreparedCreateProvisioning,
		}
		if err := c.profiles.WritePreparedCreate(journal); err != nil {
			return err
		}
		presence, err := c.credentials.Status(request.Profile.Name)
		if err != nil || presence == credential.PresenceUnavailable {
			_ = c.profiles.ClearPreparedCreate(request.Profile.Name)
			return ErrCredentialUnavailable
		}
		if presence == credential.PresencePresent {
			_ = c.profiles.ClearPreparedCreate(request.Profile.Name)
			return ErrCredentialExists
		}
		provisioned, err := c.credentials.Provision(ctx, request.Profile.Name)
		if err != nil {
			if provisioned.CleanupRequired {
				journal.Phase, journal.CleanupRequired = profile.PreparedCreateCleanupRequired, true
				if writeErr := c.profiles.WritePreparedCreate(journal); writeErr != nil {
					return ErrCredentialCleanupRequired
				}
				return ErrCredentialCleanupRequired
			}
			_ = c.profiles.ClearPreparedCreate(request.Profile.Name)
			return ErrCredentialUnavailable
		}
		journal.Phase = profile.PreparedCreateSaving
		if err := c.profiles.WritePreparedCreate(journal); err != nil {
			return ErrCredentialCleanupRequired
		}
		if _, err := c.profiles.Save(request.Profile); err != nil {
			journal.Phase, journal.CleanupRequired = profile.PreparedCreateCleanupRequired, true
			_ = c.profiles.WritePreparedCreate(journal)
			return ErrCredentialCleanupRequired
		}
		return c.profiles.ClearPreparedCreate(request.Profile.Name)
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

func createTransactionID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(value[:])
}
