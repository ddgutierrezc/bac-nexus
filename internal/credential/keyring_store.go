package credential

import (
	"bytes"
	"errors"
)

const nativeService = "BAC Nexus"

var ErrCredentialsUnavailable = errors.New("credentials_unavailable")

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

func NewKeyringStore(native NativeKeyring) *KeyringStore {
	return &KeyringStore{native: native}
}

func NewNativeCredentialStore() *KeyringStore {
	return NewKeyringStore(platformKeyring())
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
