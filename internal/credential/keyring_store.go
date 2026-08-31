package credential

import (
	"bytes"
	"errors"
)

const nativeService = "BAC Nexus"

var ErrCredentialsUnavailable = errors.New("credentials_unavailable")

type Presence string

const (
	PresencePresent     Presence = "present"
	PresenceAbsent      Presence = "absent"
	PresenceUnavailable Presence = "unavailable"
)

// Capability describes whether this installation can use a native keyring.
// It intentionally says nothing about a specific profile account.
type Capability string

const (
	CapabilitySupported   Capability = "supported"
	CapabilityUnsupported Capability = "unsupported"
	CapabilityUnavailable Capability = "unavailable"
)

type StatusStore interface {
	Status(profile string) (Presence, error)
}

// CredentialStore permits only exact, profile-scoped native credential operations.
type CredentialStore interface {
	Get(profile string) ([]byte, error)
	Set(profile string, secret []byte) error
	Delete(profile string) error
}

type NativeKeyring interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

// LegacyVault is supplied only by the command that explicitly owns old-master prompting.
type LegacyVault interface {
	Get() ([]byte, error)
	Delete() error
}

type KeyringStore struct {
	native NativeKeyring
}

// nativeKeyring is an unexported construction seam so platform failure tests
// never need to read or mutate a developer's real native credential store.
var nativeKeyring = platformKeyring

func NewKeyringStore(native NativeKeyring) *KeyringStore {
	return &KeyringStore{native: native}
}

func NewNativeCredentialStore() *KeyringStore {
	return NewKeyringStore(nativeKeyring())
}

func (s *KeyringStore) Capability() Capability {
	if s == nil || s.native == nil {
		return CapabilityUnsupported
	}
	return CapabilitySupported
}

func (s *KeyringStore) Get(profile string) ([]byte, error) {
	account, err := nativeAccount(profile)
	if err != nil || s.native == nil {
		return nil, ErrCredentialsUnavailable
	}
	secret, err := s.native.Get(nativeService, account)
	if err != nil || len(secret) < 1 || len(secret) > maxSecretBytes {
		return nil, ErrCredentialsUnavailable
	}
	return []byte(secret), nil
}

func (s *KeyringStore) Status(profile string) (Presence, error) {
	account, err := nativeAccount(profile)
	if err != nil || s.native == nil {
		return PresenceUnavailable, nil
	}
	secret, err := s.native.Get(nativeService, account)
	if err != nil {
		return PresenceUnavailable, nil
	}
	value := []byte(secret)
	defer Zero(value)
	if len(value) == 0 {
		return PresenceAbsent, nil
	}
	if !validSecret(value) {
		return PresenceUnavailable, nil
	}
	return PresencePresent, nil
}

func (s *KeyringStore) Set(profile string, secret []byte) error {
	account, err := nativeAccount(profile)
	if err != nil || s.native == nil || !validSecret(secret) {
		return ErrCredentialsUnavailable
	}
	temporary := append([]byte(nil), secret...)
	defer Zero(temporary)
	if err := s.native.Set(nativeService, account, string(temporary)); err != nil {
		return ErrCredentialsUnavailable
	}
	return nil
}

func (s *KeyringStore) Delete(profile string) error {
	account, err := nativeAccount(profile)
	if err != nil || s.native == nil {
		return ErrCredentialsUnavailable
	}
	if err := s.native.Delete(nativeService, account); err != nil {
		return ErrCredentialsUnavailable
	}
	return nil
}

func (s *KeyringStore) Migrate(profile string, vault LegacyVault) error {
	if _, err := nativeAccount(profile); err != nil || vault == nil {
		return ErrCredentialsUnavailable
	}
	secret, err := vault.Get()
	if err != nil || !validSecret(secret) {
		Zero(secret)
		return ErrCredentialsUnavailable
	}
	defer Zero(secret)
	if err := s.Set(profile, secret); err != nil {
		return ErrCredentialsUnavailable
	}
	readback, err := s.Get(profile)
	if err != nil {
		return ErrCredentialsUnavailable
	}
	defer Zero(readback)
	if !bytes.Equal(secret, readback) || vault.Delete() != nil {
		return ErrCredentialsUnavailable
	}
	return nil
}

func nativeAccount(profile string) (string, error) {
	if err := validateProfileName(profile); err != nil {
		return "", err
	}
	return "ibmi/" + profile, nil
}

func validSecret(secret []byte) bool {
	return len(secret) >= 1 && len(secret) <= maxSecretBytes
}

var _ CredentialStore = (*KeyringStore)(nil)
var _ StatusStore = (*KeyringStore)(nil)
