//go:build !darwin

package credential

import keyring "github.com/zalando/go-keyring"

type goKeyring struct{}

func (goKeyring) Get(service, account string) (string, error) { return keyring.Get(service, account) }
func (goKeyring) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}
func (goKeyring) Delete(service, account string) error { return keyring.Delete(service, account) }

func platformKeyring() NativeKeyring { return goKeyring{} }
