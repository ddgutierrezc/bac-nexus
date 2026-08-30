package credential

import (
	"errors"
	"testing"
)

// TestNativeUnavailableMapsToCredentialsUnavailable proves the Linux adapter's
// fail-closed mapping without reading a developer's Secret Service collection.
func TestNativeUnavailableMapsToCredentialsUnavailable(t *testing.T) {
	old := nativeKeyring
	nativeKeyring = func() NativeKeyring { return linuxUnavailableKeyring{} }
	t.Cleanup(func() { nativeKeyring = old })
	store := NewNativeCredentialStore()
	for _, operation := range []struct {
		name string
		run  func() error
	}{
		{"get", func() error { _, err := store.Get("production"); return err }},
		{"set", func() error { return store.Set("production", []byte("ephemeral")) }},
		{"delete", func() error { return store.Delete("production") }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); !errors.Is(err, ErrCredentialsUnavailable) {
				t.Fatalf("error = %v, want credentials_unavailable", err)
			}
		})
	}
}

type linuxUnavailableKeyring struct{}

func (linuxUnavailableKeyring) Get(string, string) (string, error) {
	return "", errors.New("dbus unavailable")
}
func (linuxUnavailableKeyring) Set(string, string, string) error {
	return errors.New("dbus unavailable")
}
func (linuxUnavailableKeyring) Delete(string, string) error { return errors.New("dbus unavailable") }
