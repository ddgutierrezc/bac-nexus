package remote

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"bac-nexus/internal/profile"
)

const (
	sshHarnessIdentityEnableEnv = "BAC_NEXUS_SSH_IDENTITY_INTEGRATION"
	sshHarnessAuthEnableEnv     = "BAC_NEXUS_SSH_INTEGRATION"
	sshHarnessHostEnv           = "BAC_NEXUS_SSH_TEST_HOST"
	sshHarnessPortEnv           = "BAC_NEXUS_SSH_TEST_PORT"
	sshHarnessUserEnv           = "BAC_NEXUS_SSH_TEST_USER"
	sshHarnessPasswordFileEnv   = "BAC_NEXUS_SSH_TEST_PASSWORD_FILE"
	sshHarnessDefaultHost       = "127.0.0.1"
	sshHarnessDefaultPort       = 2222
	sshHarnessDefaultUser       = "transporttest"
	sshHarnessMaxPasswordSize   = 1024
	sshHarnessTimeout           = 10 * time.Second
)

type sshHarnessConfig struct {
	host string
	port int
	user string
}

func TestSSHTransportHarnessObservesIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("SSH identity harness is skipped in short mode")
	}
	if os.Getenv(sshHarnessIdentityEnableEnv) != "1" {
		t.Skip("set BAC_NEXUS_SSH_IDENTITY_INTEGRATION=1 to run the local SSH identity harness")
	}

	config := loadSSHHarnessConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), sshHarnessTimeout)
	defer cancel()
	candidate, err := (HostIdentityInspector{}).InspectHostKey(ctx, config.host, config.port)
	if err != nil {
		t.Fatalf("InspectHostKey() error = %v", err)
	}
	if candidate.Algorithm == "" || candidate.Fingerprint == "" {
		t.Fatalf("InspectHostKey() candidate = %#v, want algorithm and fingerprint", candidate)
	}
}

func TestSSHTransportHarnessAuthenticatesSFTP(t *testing.T) {
	if testing.Short() {
		t.Skip("SSH authenticated harness is skipped in short mode")
	}
	if os.Getenv(sshHarnessAuthEnableEnv) != "1" {
		t.Skip("set BAC_NEXUS_SSH_INTEGRATION=1 to run the authenticated local SSH transport harness")
	}

	config := loadSSHHarnessConfig(t)
	password := readSSHHarnessPassword(t, os.Getenv(sshHarnessPasswordFileEnv))
	defer Zero(password)

	ctx, cancel := context.WithTimeout(context.Background(), sshHarnessTimeout)
	defer cancel()
	candidate, err := (HostIdentityInspector{}).InspectHostKey(ctx, config.host, config.port)
	if err != nil {
		t.Fatalf("InspectHostKey() error = %v", err)
	}
	if candidate.Algorithm == "" || candidate.Fingerprint == "" {
		t.Fatalf("InspectHostKey() candidate = %#v, want algorithm and fingerprint", candidate)
	}

	t.Run("authenticates with observed exact fingerprint and starts SFTP", func(t *testing.T) {
		client := dialSSHHarness(t, ctx, config, candidate.Fingerprint, password)
		t.Cleanup(func() { _ = client.Close() })
		if client.SFTP() == nil {
			t.Fatal("Dial() returned a nil SFTP client")
		}
		if _, err := client.WorkingDirectory(); err != nil {
			_ = client.Close()
			t.Fatalf("authenticated SFTP WorkingDirectory() error = %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("Client.Close() error = %v", err)
		}
	})

	t.Run("rejects an incorrect fingerprint before authentication", func(t *testing.T) {
		_, err := Dial(ctx, sshHarnessProfile(config, "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), password)
		var mismatch *HostKeyMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("Dial() error = %v, want HostKeyMismatchError", err)
		}
	})
}

func loadSSHHarnessConfig(t *testing.T) sshHarnessConfig {
	t.Helper()
	config := sshHarnessConfig{
		host: environmentOrDefault(sshHarnessHostEnv, sshHarnessDefaultHost),
		user: environmentOrDefault(sshHarnessUserEnv, sshHarnessDefaultUser),
	}
	port, err := strconv.Atoi(environmentOrDefault(sshHarnessPortEnv, strconv.Itoa(sshHarnessDefaultPort)))
	if err != nil || profile.ValidatePort(port) != nil {
		t.Fatal("SSH transport harness port must be an integer between 1 and 65535")
	}
	config.port = port

	ip := net.ParseIP(config.host)
	if ip == nil || !ip.IsLoopback() {
		t.Fatal("SSH transport harness host must be a loopback IP address")
	}
	if err := profile.ValidateUsername(config.user); err != nil {
		t.Fatal("SSH transport harness user is invalid")
	}
	return config
}

func environmentOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func readSSHHarnessPassword(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal("open SSH transport harness password file failed")
	}
	defer file.Close()
	password, err := io.ReadAll(io.LimitReader(file, sshHarnessMaxPasswordSize+1))
	if err != nil || len(password) == 0 || len(password) > sshHarnessMaxPasswordSize || bytes.ContainsAny(password, "\x00\r\n") {
		Zero(password)
		t.Fatal("SSH transport harness password file must contain one non-empty line within the size limit")
	}
	return password
}

func dialSSHHarness(t *testing.T, ctx context.Context, config sshHarnessConfig, fingerprint string, password []byte) *Client {
	t.Helper()
	client, err := Dial(ctx, sshHarnessProfile(config, fingerprint), password)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	return client
}

func sshHarnessProfile(config sshHarnessConfig, fingerprint string) profile.Profile {
	return profile.Profile{
		Name:               "ssh-transport-test",
		Host:               config.host,
		Port:               config.port,
		Username:           config.user,
		HostKeyFingerprint: fingerprint,
		HostKeyTrust:       profile.HostKeyTrustTOFU,
		CredentialMode:     profile.CredentialModePrompt,
	}
}
