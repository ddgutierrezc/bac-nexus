package credential

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestKeyringCredentialStoreUsesExactNativeIdentityAndOperations(t *testing.T) {
	native := &keyringFake{secret: "native-secret"}
	store := NewKeyringStore(native)

	credential, err := store.Get("production-1")
	if err != nil {
		t.Fatalf("get credential: %v", err)
	}
	defer Zero(credential)
	if !bytes.Equal(credential, []byte("native-secret")) {
		t.Fatalf("credential = %q, want native secret", credential)
	}
	if got, want := native.events, []string{"get:BAC Nexus:ibmi/production-1"}; !sameStrings(got, want) {
		t.Fatalf("get events = %v, want %v", got, want)
	}

	secret := []byte("new-native-secret")
	if err := store.Set("production-1", secret); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if got, want := native.events, []string{
		"get:BAC Nexus:ibmi/production-1",
		"set:BAC Nexus:ibmi/production-1:new-native-secret",
	}; !sameStrings(got, want) {
		t.Fatalf("set events = %v, want %v", got, want)
	}

	if err := store.Delete("production-1"); err != nil {
		t.Fatalf("delete credential: %v", err)
	}
	if got, want := native.events, []string{
		"get:BAC Nexus:ibmi/production-1",
		"set:BAC Nexus:ibmi/production-1:new-native-secret",
		"delete:BAC Nexus:ibmi/production-1",
	}; !sameStrings(got, want) {
		t.Fatalf("delete events = %v, want %v", got, want)
	}
}

func TestKeyringCredentialStoreRotatesOnlyTheExactProfileAccount(t *testing.T) {
	native := &keyringFake{}
	store := NewKeyringStore(native)
	if err := store.Set("production-1", []byte("first")); err != nil {
		t.Fatalf("first Set() error = %v", err)
	}
	if err := store.Set("production-1", []byte("second")); err != nil {
		t.Fatalf("rotated Set() error = %v", err)
	}
	want := []string{"set:BAC Nexus:ibmi/production-1:first", "set:BAC Nexus:ibmi/production-1:second"}
	if !sameStrings(native.events, want) {
		t.Fatalf("rotation events = %v, want %v", native.events, want)
	}
}

func TestKeyringCredentialStoreRejectsInvalidInputsBeforeNativeAccess(t *testing.T) {
	tests := []struct {
		name string
		call func(*KeyringStore) error
	}{
		{name: "missing profile", call: func(store *KeyringStore) error { _, err := store.Get(""); return err }},
		{name: "invalid profile", call: func(store *KeyringStore) error { return store.Set("invalid/profile", []byte("secret")) }},
		{name: "profile over 64 bytes", call: func(store *KeyringStore) error { _, err := store.Get("a" + strings.Repeat("b", 64)); return err }},
		{name: "empty secret", call: func(store *KeyringStore) error { return store.Set("production", nil) }},
		{name: "secret over 4096 bytes", call: func(store *KeyringStore) error { return store.Set("production", bytes.Repeat([]byte{'x'}, 4097)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native := &keyringFake{}
			store := NewKeyringStore(native)
			if err := tt.call(store); !errors.Is(err, ErrCredentialsUnavailable) {
				t.Fatalf("error = %v, want credentials_unavailable", err)
			}
			if len(native.events) != 0 {
				t.Fatalf("invalid input accessed native store: %v", native.events)
			}
		})
	}
}

func TestKeyringCredentialStoreRedactsNativeFailuresAndFailsClosed(t *testing.T) {
	const secret = "must-not-appear"
	for _, operation := range []string{"get", "set", "delete"} {
		t.Run(operation, func(t *testing.T) {
			native := &keyringFake{err: errors.New(secret)}
			store := NewKeyringStore(native)

			var err error
			switch operation {
			case "get":
				_, err = store.Get("production")
			case "set":
				err = store.Set("production", []byte("secret"))
			case "delete":
				err = store.Delete("production")
			}
			if !errors.Is(err, ErrCredentialsUnavailable) {
				t.Fatalf("error = %v, want credentials_unavailable", err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("native failure leaked secret: %v", err)
			}
		})
	}
}

func TestKeyringCredentialStoreMigrationDeletesVaultOnlyAfterExactReadback(t *testing.T) {
	secret := []byte("legacy-secret")
	vault := &legacyVaultFake{secret: append([]byte(nil), secret...)}
	native := &keyringFake{secret: string(secret)}
	store := NewKeyringStore(native)

	if err := store.Migrate("production", vault); err != nil {
		t.Fatalf("migrate credential: %v", err)
	}
	if !vault.deleted {
		t.Fatal("migration did not delete confirmed legacy vault")
	}
	if !allZero(vault.secret) {
		t.Fatalf("legacy secret was not zeroed: %q", vault.secret)
	}
}

func TestKeyringCredentialStoreMigrationRetainsVaultOnNativeUncertainty(t *testing.T) {
	vault := &legacyVaultFake{secret: []byte("legacy-secret")}
	native := &keyringFake{err: errors.New("native store unavailable")}
	store := NewKeyringStore(native)

	err := store.Migrate("production", vault)
	if !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("error = %v, want credentials_unavailable", err)
	}
	if vault.deleted {
		t.Fatal("migration deleted legacy vault without native confirmation")
	}
}

type keyringFake struct {
	secret string
	err    error
	events []string
}

func (f *keyringFake) Get(service, account string) (string, error) {
	f.events = append(f.events, "get:"+service+":"+account)
	return f.secret, f.err
}

func (f *keyringFake) Set(service, account, secret string) error {
	f.events = append(f.events, "set:"+service+":"+account+":"+secret)
	return f.err
}

func (f *keyringFake) Delete(service, account string) error {
	f.events = append(f.events, "delete:"+service+":"+account)
	return f.err
}

type legacyVaultFake struct {
	secret  []byte
	deleted bool
}

func (f *legacyVaultFake) Get() ([]byte, error) { return f.secret, nil }
func (f *legacyVaultFake) Delete() error {
	f.deleted = true
	return nil
}

func allZero(value []byte) bool {
	for _, current := range value {
		if current != 0 {
			return false
		}
	}
	return true
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
