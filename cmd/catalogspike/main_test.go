package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
)

func TestCommandExitBehavior(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantOutput string
		wantAbsent string
	}{
		{name: "root help succeeds", args: []string{"-h"}, wantOutput: "Usage: catalogspike"},
		{name: "root long help succeeds", args: []string{"--help"}, wantOutput: "Usage: catalogspike"},
		{name: "root help command succeeds", args: []string{"help"}, wantOutput: "Usage: catalogspike"},
		{name: "offline help succeeds", args: []string{"offline", "-h"}, wantOutput: "Usage of offline:"},
		{name: "configure help succeeds", args: []string{"configure", "--help"}, wantOutput: "Usage of configure:"},
		{name: "setup help succeeds", args: []string{"setup", "-h"}, wantOutput: "Usage of setup:"},
		{name: "credentials help succeeds", args: []string{"credentials", "status", "-h"}, wantOutput: "Usage of credentials status:"},
		{name: "credentials root help succeeds", args: []string{"credentials", "-h"}, wantOutput: "Usage: catalogspike credentials"},
		{name: "status help omits replace", args: []string{"credentials", "status", "-h"}, wantOutput: "Usage of credentials status:", wantAbsent: "-replace"},
		{name: "delete help omits replace", args: []string{"credentials", "delete", "-h"}, wantOutput: "Usage of credentials delete:", wantAbsent: "-replace"},
		{name: "unknown command fails", args: []string{"unknown"}, wantExit: 2, wantOutput: `catalog spike: unknown command "unknown"`},
		{name: "missing subcommand fails with usage", wantExit: 2, wantOutput: "Usage: catalogspike"},
		{name: "offline flag at root fails with usage", args: []string{"-item", "PISA061"}, wantExit: 2, wantOutput: "explicit subcommand is required"},
		{name: "any first argument flag fails with usage", args: []string{"-production-library", "PRODLIB"}, wantExit: 2, wantOutput: "Usage: catalogspike"},
		{name: "unknown credentials action help fails", args: []string{"credentials", "unknown", "-h"}, wantExit: 2, wantOutput: `credentials unknown action "unknown"`},
		{name: "offline positional fails", args: []string{"offline", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "configure positional fails", args: []string{"configure", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "setup positional fails", args: []string{"setup", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "live positional fails", args: []string{"live", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "credentials set positional fails", args: []string{"credentials", "set", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "credentials positional fails", args: []string{"credentials", "status", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "credentials delete positional fails", args: []string{"credentials", "delete", "extra"}, wantExit: 2, wantOutput: "does not accept positional arguments"},
		{name: "status rejects replace flag", args: []string{"credentials", "status", "-replace"}, wantExit: 2, wantOutput: "flag provided but not defined: -replace"},
		{name: "delete rejects replace flag", args: []string{"credentials", "delete", "-replace"}, wantExit: 2, wantOutput: "flag provided but not defined: -replace"},
		{name: "invalid offline usage fails", args: []string{"offline"}, wantExit: 2, wantOutput: "catalog spike: invalid catalog item"},
		{name: "invalid credentials usage fails", args: []string{"credentials"}, wantExit: 2, wantOutput: "catalog spike: credentials requires"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestCommandHelper$", "--")
			command.Args = append(command.Args, tt.args...)
			command.Env = append(os.Environ(), "CATALOGSPIKE_COMMAND_HELPER=1")
			output, err := command.CombinedOutput()

			gotExit := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					gotExit = exitError.ExitCode()
				} else {
					t.Fatalf("run command: %v", err)
				}
			}
			if gotExit != tt.wantExit {
				t.Fatalf("exit code = %d, want %d; output: %s", gotExit, tt.wantExit, output)
			}
			if !strings.Contains(string(output), tt.wantOutput) {
				t.Fatalf("output = %q, want containing %q", output, tt.wantOutput)
			}
			if tt.wantAbsent != "" && strings.Contains(string(output), tt.wantAbsent) {
				t.Fatalf("output = %q, want absent %q", output, tt.wantAbsent)
			}
			if tt.wantExit == 0 && strings.Contains(string(output), "catalog spike:") {
				t.Fatalf("help used fatal error path: %s", output)
			}
		})
	}
}

func TestOfflineOutputRedactsSQLAndParameters(t *testing.T) {
	query, err := catalog.BuildQuery("PISA061", "PRODLIB")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeOfflineOutput(&output, query, false); err != nil {
		t.Fatal(err)
	}
	for _, sensitive := range append([]string{query.Statement}, query.Parameters...) {
		if strings.Contains(output.String(), sensitive) {
			t.Fatalf("offline output exposed sensitive query content: %q", output.String())
		}
	}
}

type fakeProfiles struct {
	saved   profile.Profile
	saveErr error
	deleted string
}

func (f *fakeProfiles) Save(p profile.Profile) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saved = p
	return "profile", nil
}
func (f *fakeProfiles) Delete(name string) (bool, error) { f.deleted = name; return true, nil }

type fakeVaults struct {
	setProfile string
	password   []byte
	master     []byte
	setErr     error
	deleted    string
}

func (f *fakeVaults) Set(name string, password, master []byte, replace bool) (credential.SetResult, error) {
	f.setProfile = name
	f.password = append([]byte(nil), password...)
	f.master = append([]byte(nil), master...)
	return credential.SetResult{Path: "vault", Committed: f.setErr == nil}, f.setErr
}
func (f *fakeVaults) Delete(name string) (bool, error) { f.deleted = name; return true, nil }

func setupReaders(lines []string, secrets [][]byte) (func(string) (string, error), secretReader) {
	lineIndex, secretIndex := 0, 0
	return func(string) (string, error) {
			if lineIndex >= len(lines) {
				return "", io.EOF
			}
			value := lines[lineIndex]
			lineIndex++
			return value, nil
		}, func(string) ([]byte, error) {
			if secretIndex >= len(secrets) {
				return nil, io.EOF
			}
			value := append([]byte(nil), secrets[secretIndex]...)
			secretIndex++
			return value, nil
		}
}

func TestExecuteSetupHappyPathDoesNotExposeSecrets(t *testing.T) {
	profiles := &fakeProfiles{}
	vaults := &fakeVaults{}
	readLine, readSecret := setupReaders(
		[]string{"dev", "ibmi.example.test", "", "NEXUSUSER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"},
		[][]byte{[]byte("ibmi-password"), []byte("master-passphrase"), []byte("master-passphrase")},
	)
	var output bytes.Buffer
	var notices bytes.Buffer
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, VerifyJAR: func(string) error { return nil }, Output: &output, Notices: &notices})
	if err != nil {
		t.Fatal(err)
	}
	if profiles.saved.Port != 22 || profiles.saved.MapepireJAR == "" || profiles.saved.CredentialMode != profile.CredentialModeVault || vaults.setProfile != "dev" {
		t.Fatalf("setup state = %#v, %#v", profiles.saved, vaults)
	}
	var document map[string]any
	decoder := json.NewDecoder(strings.NewReader(output.String()))
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&document); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout contains more than one JSON document: %q", output.String())
	}
	if strings.Contains(output.String(), "Host-key fingerprint discovery") || !strings.Contains(notices.String(), "Host-key fingerprint discovery") {
		t.Fatalf("stdout/notices = %q/%q", output.String(), notices.String())
	}
	for _, secret := range []string{"ibmi-password", "master-passphrase"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("output contains secret %q", secret)
		}
	}
}

func TestExecuteSetupRejectsConfirmationAndInvalidInputs(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "mapepire.jar")
	tests := []struct {
		name      string
		lines     []string
		secrets   [][]byte
		verifyErr error
	}{
		{"master mismatch", []string{"dev", "ibmi.example.test", "22", "USER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", jar}, [][]byte{[]byte("password"), []byte("master-a"), []byte("master-b")}, nil},
		{"invalid profile", []string{"../dev", "ibmi.example.test", "22", "USER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", jar}, [][]byte{[]byte("password"), []byte("master"), []byte("master")}, nil},
		{"invalid fingerprint", []string{"dev", "ibmi.example.test", "22", "USER", "untrusted", "", jar}, [][]byte{[]byte("password"), []byte("master"), []byte("master")}, nil},
		{"invalid jar", []string{"dev", "ibmi.example.test", "22", "USER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", jar}, [][]byte{[]byte("password"), []byte("master"), []byte("master")}, errors.New("checksum mismatch")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles, vaults := &fakeProfiles{}, &fakeVaults{}
			readLine, readSecret := setupReaders(tt.lines, tt.secrets)
			err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, VerifyJAR: func(string) error { return tt.verifyErr }, Output: io.Discard, Notices: io.Discard})
			if err == nil {
				t.Fatal("expected setup rejection")
			}
			if profiles.saved.Name != "" || vaults.setProfile != "" {
				t.Fatal("invalid setup persisted state")
			}
		})
	}
}

func TestExecuteSetupRollsBackNewProfileWhenVaultFails(t *testing.T) {
	profiles := &fakeProfiles{}
	vaults := &fakeVaults{setErr: errors.New("vault failed")}
	readLine, readSecret := setupReaders([]string{"dev", "ibmi.example.test", "22", "USER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"}, [][]byte{[]byte("password"), []byte("master"), []byte("master")})
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, VerifyJAR: func(string) error { return nil }, Output: io.Discard, Notices: io.Discard})
	if err == nil || profiles.deleted != "dev" {
		t.Fatalf("error/deleted = %v/%q", err, profiles.deleted)
	}
}

func TestExecuteSetupDoesNotCreateVaultWhenProfilePublicationFails(t *testing.T) {
	profiles := &fakeProfiles{saveErr: errors.New("profile failed")}
	vaults := &fakeVaults{}
	readLine, readSecret := setupReaders([]string{"dev", "ibmi.example.test", "22", "USER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"}, [][]byte{[]byte("password"), []byte("master"), []byte("master")})
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, VerifyJAR: func(string) error { return nil }, Output: io.Discard, Notices: io.Discard})
	if err == nil || vaults.setProfile != "" {
		t.Fatalf("error/vault = %v/%q", err, vaults.setProfile)
	}
}

type fakeVaultReader struct {
	exists     bool
	password   []byte
	masterSeen []byte
}

func (f *fakeVaultReader) Status(string) (bool, error) { return f.exists, nil }
func (f *fakeVaultReader) Get(_ string, master []byte) ([]byte, error) {
	f.masterSeen = append([]byte(nil), master...)
	return append([]byte(nil), f.password...), nil
}

func TestAcquireLivePasswordHonorsClosedCredentialMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        profile.CredentialMode
		exists      bool
		promptValue string
		stored      string
		want        string
		wantErr     bool
		wantPrompts int
	}{
		{"vault", profile.CredentialModeVault, true, "master", "stored-password", "stored-password", false, 1},
		{"vault missing fails closed", profile.CredentialModeVault, false, "terminal-password", "", "", true, 0},
		{"prompt mode", profile.CredentialModePrompt, false, "terminal-password", "", "terminal-password", false, 1},
		{"unknown mode", "legacy", false, "terminal-password", "", "", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := &fakeVaultReader{exists: tt.exists, password: []byte(tt.stored)}
			promptBuffer := []byte(tt.promptValue)
			prompts := 0
			password, err := acquireLivePassword(vault, "dev", tt.mode, func(string) ([]byte, error) { prompts++; return promptBuffer, nil })
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if prompts != tt.wantPrompts {
				t.Fatalf("prompts = %d, want %d", prompts, tt.wantPrompts)
			}
			if tt.wantErr {
				return
			}
			defer func() {
				for i := range password {
					password[i] = 0
				}
			}()
			if string(password) != tt.want {
				t.Fatalf("password = %q", password)
			}
			if tt.mode == profile.CredentialModeVault {
				if string(vault.masterSeen) != "master" {
					t.Fatal("master was not supplied to vault")
				}
				if !bytes.Equal(promptBuffer, make([]byte, len(promptBuffer))) {
					t.Fatal("master prompt buffer was not zeroed")
				}
			}
		})
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestExecuteSetupOutputFailureReportsCommittedState(t *testing.T) {
	profiles := &fakeProfiles{}
	vaults := &fakeVaults{}
	readLine, readSecret := setupReaders(
		[]string{"dev", "ibmi.example.test", "22", "USER", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"},
		[][]byte{[]byte("password"), []byte("master"), []byte("master")},
	)
	writeErr := errors.New("injected write failure")
	err := executeSetup(setupDependencies{
		Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret,
		VerifyJAR: func(string) error { return nil }, Output: failingWriter{writeErr}, Notices: io.Discard,
	})
	var committed *CommittedOutputError
	if !errors.As(err, &committed) || !committed.Committed() || !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want committed output error", err)
	}
	if profiles.saved.Name != "dev" || vaults.setProfile != "dev" {
		t.Fatalf("committed state missing: %#v/%#v", profiles, vaults)
	}
}

func TestCredentialRotationWarningReportsCommittedSuccessWithoutDetails(t *testing.T) {
	cleanupErr := errors.New("sensitive cleanup detail")
	var stdout, stderr bytes.Buffer
	err := writeCredentialSetResult(&stdout, &stderr, "dev", credential.SetResult{Committed: true, CleanupWarning: cleanupErr})
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "{\"status\":\"stored\",\"profile\":\"dev\",\"cleanupPending\":true}\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "committed") || strings.Contains(stderr.String(), cleanupErr.Error()) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCredentialRotationOutputFailuresRemainExplicitlyCommitted(t *testing.T) {
	tests := []struct {
		name       string
		stdout     func(error) io.Writer
		stderr     func(error) io.Writer
		wantOutput string
	}{
		{
			name:       "stdout result fails",
			stdout:     func(err error) io.Writer { return failingWriter{err} },
			stderr:     func(error) io.Writer { return io.Discard },
			wantOutput: "stdout result",
		},
		{
			name:       "stderr warning fails after success document",
			stdout:     func(error) io.Writer { return new(bytes.Buffer) },
			stderr:     func(err error) io.Writer { return failingWriter{err} },
			wantOutput: "stderr warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := credential.Store{Root: t.TempDir()}
			master := []byte("master-passphrase")
			if _, err := store.Set("dev", []byte("old-password"), master, false); err != nil {
				t.Fatal(err)
			}
			result, err := store.Set("dev", []byte("new-password"), master, true)
			if err != nil {
				t.Fatal(err)
			}
			result.CleanupWarning = errors.New("injected cleanup warning")
			writeErr := errors.New("injected delivery failure")
			stdout := tt.stdout(writeErr)
			err = writeCredentialSetResult(stdout, tt.stderr(writeErr), "dev", result)

			var committed *CommittedOutputError
			if !errors.As(err, &committed) || !committed.Committed() || committed.Operation != "credential rotation" || committed.Output != tt.wantOutput || !errors.Is(err, writeErr) {
				t.Fatalf("outcome = %#v, %v; want queryable committed %s failure", committed, err, tt.wantOutput)
			}
			if !strings.Contains(err.Error(), "do not retry") {
				t.Fatalf("outcome does not prohibit retry: %v", err)
			}
			stored, err := store.Get("dev", master)
			if err != nil {
				t.Fatal(err)
			}
			defer credential.Zero(stored)
			if string(stored) != "new-password" {
				t.Fatalf("stored credential = %q, want rotated credential", stored)
			}
			if buffer, ok := stdout.(*bytes.Buffer); ok && buffer.String() != "{\"status\":\"stored\",\"profile\":\"dev\",\"cleanupPending\":true}\n" {
				t.Fatalf("stdout = %q, want one committed success document", buffer.String())
			}
		})
	}
}

func TestCommandHelper(t *testing.T) {
	if os.Getenv("CATALOGSPIKE_COMMAND_HELPER") != "1" {
		return
	}
	os.Args = append([]string{"catalogspike"}, os.Args[3:]...)
	main()
}
