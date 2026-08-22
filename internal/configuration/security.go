package configuration

import (
	"context"
	"errors"
	"fmt"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

var (
	ErrCredentialUnavailable = errors.New("credentials_unavailable")
	ErrCredentialExists      = errors.New("credential already exists")
	ErrConfirmationRequired  = errors.New("exact confirmation required")
	ErrWarningRequired       = errors.New("explicit TOFU warning required")
)

type SecretInput interface {
	WithSecret(context.Context, string, func([]byte) error) error
}

type SecretInputFunc func(string) ([]byte, error)

func (f SecretInputFunc) WithSecret(ctx context.Context, label string, use func([]byte) error) error {
	if f == nil {
		return ErrCredentialUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	secret, err := f(label)
	if err != nil {
		return err
	}
	defer credential.Zero(secret)
	if len(secret) < 1 || len(secret) > 4096 {
		return ErrCredentialUnavailable
	}
	return use(secret)
}

type CredentialBackend interface {
	credential.CredentialStore
	credential.StatusStore
	Migrate(string, credential.LegacyVault) error
}

type CredentialOutcome string

const (
	CredentialOutcomeStored   CredentialOutcome = "stored"
	CredentialOutcomeRotated  CredentialOutcome = "rotated"
	CredentialOutcomeDeleted  CredentialOutcome = "deleted"
	CredentialOutcomeMigrated CredentialOutcome = "migrated"
)

type CredentialService struct {
	store CredentialBackend
	input SecretInput
}

func NewCredentialService(store CredentialBackend, input SecretInput) *CredentialService {
	return &CredentialService{store: store, input: input}
}

func (s *CredentialService) Status(ctx context.Context, profileName string) (credential.Presence, error) {
	if err := ctx.Err(); err != nil {
		return credential.PresenceUnavailable, err
	}
	if s.store == nil {
		return credential.PresenceUnavailable, ErrCredentialUnavailable
	}
	presence, err := s.store.Status(profileName)
	if err != nil {
		return credential.PresenceUnavailable, ErrCredentialUnavailable
	}
	return presence, nil
}

func (s *CredentialService) Set(ctx context.Context, profileName string) (CredentialOutcome, error) {
	return s.write(ctx, profileName, CredentialOutcomeStored)
}

func (s *CredentialService) Rotate(ctx context.Context, profileName string) (CredentialOutcome, error) {
	presence, err := s.Status(ctx, profileName)
	if err != nil {
		return "", err
	}
	if presence != credential.PresencePresent {
		return "", ErrCredentialUnavailable
	}
	return s.write(ctx, profileName, CredentialOutcomeRotated)
}

func (s *CredentialService) write(ctx context.Context, profileName string, outcome CredentialOutcome) (CredentialOutcome, error) {
	if s.store == nil || s.input == nil {
		return "", ErrCredentialUnavailable
	}
	presence, err := s.Status(ctx, profileName)
	if err != nil {
		return "", err
	}
	if outcome == CredentialOutcomeStored && presence == credential.PresencePresent {
		return "", ErrCredentialExists
	}
	if presence == credential.PresenceUnavailable {
		return "", ErrCredentialUnavailable
	}
	err = s.input.WithSecret(ctx, profileName, func(secret []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return s.store.Set(profileName, secret)
	})
	if err != nil {
		return "", mapCredentialError(err)
	}
	return outcome, nil
}

func (s *CredentialService) Delete(ctx context.Context, profileName, confirmation string) (CredentialOutcome, error) {
	if confirmation != "delete "+profileName {
		return "", ErrConfirmationRequired
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.store == nil {
		return "", ErrCredentialUnavailable
	}
	if err := s.store.Delete(profileName); err != nil {
		return "", mapCredentialError(err)
	}
	return CredentialOutcomeDeleted, nil
}

func (s *CredentialService) Migrate(ctx context.Context, profileName string, vault credential.LegacyVault, confirmed bool) (CredentialOutcome, error) {
	if !confirmed {
		return "", ErrConfirmationRequired
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.store == nil || vault == nil {
		return "", ErrCredentialUnavailable
	}
	if err := s.store.Migrate(profileName, vault); err != nil {
		return "", mapCredentialError(err)
	}
	return CredentialOutcomeMigrated, nil
}

func mapCredentialError(err error) error {
	if errors.Is(err, credential.ErrCredentialsUnavailable) {
		return ErrCredentialUnavailable
	}
	return err
}

type ProfileTrustStore interface {
	Read(string) (profile.Profile, error)
	Update(profile.Profile, string) (profile.ProfileUpdateResult, error)
}

type TrustOutcome string

const TrustOutcomeEnrolled TrustOutcome = "enrolled"

type TrustService struct {
	profiles ProfileTrustStore
	inspect  func(context.Context, string, int) (remote.HostKeyObservation, error)
}

func NewTrustService(profiles ProfileTrustStore, inspect func(context.Context, string, int) (remote.HostKeyObservation, error)) *TrustService {
	return &TrustService{profiles: profiles, inspect: inspect}
}

func (s *TrustService) EnrollManual(ctx context.Context, name, fingerprint, provenance, confirmation string) (TrustOutcome, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if confirmation != "enroll "+fingerprint || provenance == "" {
		return "", ErrConfirmationRequired
	}
	if err := profile.ValidateHostKey(fingerprint, profile.HostKeyTrustVerified); err != nil {
		return "", err
	}
	return s.persist(ctx, name, fingerprint, profile.HostKeyTrustVerified, provenance)
}

func (s *TrustService) InspectAndEnroll(ctx context.Context, name string, warned bool, confirmation string) (TrustOutcome, error) {
	if !warned {
		return "", ErrWarningRequired
	}
	if s.inspect == nil {
		return "", errors.New("host inspection unavailable")
	}
	stored, err := s.profiles.Read(name)
	if err != nil {
		return "", err
	}
	observation, err := s.inspect(ctx, stored.Host, stored.Port)
	if err != nil {
		return "", err
	}
	if observation.Verified || observation.TrustCandidate != profile.HostKeyTrustTOFU {
		return "", errors.New("host inspection is not an unverified TOFU observation")
	}
	if confirmation != "enroll "+observation.Fingerprint {
		return "", ErrConfirmationRequired
	}
	return s.persist(ctx, name, observation.Fingerprint, profile.HostKeyTrustTOFU, "unverified TOFU inspection")
}

func (s *TrustService) Verify(ctx context.Context, name, observed string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stored, err := s.profiles.Read(name)
	if err != nil {
		return err
	}
	if stored.HostKeyFingerprint == "" || observed == "" || stored.HostKeyFingerprint != observed {
		return remote.ErrHostKeyChanged
	}
	return nil
}

func (s *TrustService) persist(ctx context.Context, name, fingerprint string, trust profile.HostKeyTrust, provenance string) (TrustOutcome, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	stored, err := s.profiles.Read(name)
	if err != nil {
		return "", err
	}
	stored.HostKeyFingerprint = fingerprint
	stored.HostKeyTrust = trust
	stored.HostKeyProvenance = provenance
	if _, err := s.profiles.Update(stored, name); err != nil {
		return "", fmt.Errorf("enroll host key: %w", err)
	}
	return TrustOutcomeEnrolled, nil
}

type Clipboard interface {
	Copy(context.Context, string) error
}

func CopySecretFree(ctx context.Context, clipboard Clipboard, preview string) error {
	if clipboard == nil || preview == "" {
		return errors.New("clipboard preview is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return clipboard.Copy(ctx, preview)
}
