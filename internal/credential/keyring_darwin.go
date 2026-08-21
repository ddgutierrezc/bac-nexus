//go:build darwin

package credential

import (
	"encoding/base64"
	"os/exec"
	"strings"
)

type macOSKeyring struct{}

type macOSCommand struct {
	Path string
	Args []string
}

func platformKeyring() NativeKeyring { return macOSKeyring{} }

func (macOSKeyring) Get(service, account string) (string, error) {
	output, err := runMacOSSecurity(macOSCommand{Path: "/usr/bin/security", Args: []string{"find-generic-password", "-w", "-s", service, "-a", account}}, "")
	if err != nil {
		return "", err
	}
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(output))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (macOSKeyring) Set(_ string, account, secret string) error {
	command, input := macOSSetCommand(strings.TrimPrefix(account, "ibmi/"), []byte(secret))
	_, err := runMacOSSecurity(command, input)
	return err
}

func (macOSKeyring) Delete(service, account string) error {
	_, err := runMacOSSecurity(macOSCommand{Path: "/usr/bin/security", Args: []string{"delete-generic-password", "-s", service, "-a", account}}, "")
	return err
}

func macOSSetCommand(profile string, secret []byte) (macOSCommand, string) {
	encoded := base64.RawStdEncoding.EncodeToString(secret)
	return macOSCommand{Path: "/usr/bin/security", Args: []string{"-i"}}, "add-generic-password -U -s \"BAC Nexus\" -a \"ibmi/" + profile + "\" -w \"" + encoded + "\"\nquit\n"
}

func runMacOSSecurity(command macOSCommand, input string) (string, error) {
	process := exec.Command(command.Path, command.Args...)
	process.Stdin = strings.NewReader(input)
	output, err := process.Output()
	if err != nil {
		return "", err
	}
	if len(output) > maxSecretBytes*2 {
		return "", ErrCredentialsUnavailable
	}
	return string(output), nil
}
