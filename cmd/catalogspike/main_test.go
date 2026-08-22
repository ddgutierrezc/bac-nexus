package main

import (
	"bytes"
	"context"
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
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"

	"golang.org/x/crypto/ssh"
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
	events  *[]string
}

func (f *fakeProfiles) Save(p profile.Profile) (string, error) {
	if f.events != nil {
		*f.events = append(*f.events, "profile-save")
	}
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saved = p
	return "profile", nil
}
func (f *fakeProfiles) List(int) ([]profile.Profile, error) { return nil, nil }
func (f *fakeProfiles) Read(string) (profile.Profile, error) {
	return profile.Profile{}, profile.ErrProfileNotFound
}
func (f *fakeProfiles) Update(profile.Profile, string) (profile.ProfileUpdateResult, error) {
	return profile.ProfileUpdateResult{ReplacementCommitted: true}, nil
}
func (f *fakeProfiles) Delete(name string, _ profile.DeleteConfirmation) (profile.ProfileDeleteResult, error) {
	f.deleted = name
	return profile.ProfileDeleteResult{Deleted: true, CredentialOutcome: profile.CredentialOutcomeUntouched}, nil
}
func (f *fakeProfiles) Restore(string) error { return nil }

type fakeVaults struct {
	setProfile string
	password   []byte
	master     []byte
	setErr     error
	deleteErr  error
	deleted    string
	events     *[]string
	store      *credential.Store
}

func (f *fakeVaults) Set(name string, password, master []byte, replace bool) (credential.SetResult, error) {
	if f.events != nil {
		*f.events = append(*f.events, "vault-set")
	}
	f.setProfile = name
	f.password = append([]byte(nil), password...)
	f.master = append([]byte(nil), master...)
	if f.store != nil && f.setErr == nil {
		return f.store.Set(name, password, master, replace)
	}
	return credential.SetResult{Path: "vault", Committed: f.setErr == nil}, f.setErr
}
func (f *fakeVaults) Delete(name string) (bool, error) {
	if f.events != nil {
		*f.events = append(*f.events, "vault-delete")
	}
	f.deleted = name
	if f.deleteErr == nil && f.store != nil {
		return f.store.Delete(name)
	}
	return f.deleteErr == nil, f.deleteErr
}

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

func fakeHostKeyInspection(context.Context, string, int) (remote.HostKeyObservation, error) {
	return remote.HostKeyObservation{Algorithm: ssh.KeyAlgoED25519, Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
}

func noJARDiscovery() mapepire.DiscoveryResult {
	return mapepire.DiscoveryResult{Status: mapepire.DiscoveryNotFound}
}

func TestExecuteSetupHappyPathDoesNotExposeSecrets(t *testing.T) {
	var events []string
	profiles := &fakeProfiles{events: &events}
	vaults := &fakeVaults{events: &events}
	readLine, readSecret := setupReaders(
		[]string{"dev", "ibmi.example.test", "", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "NEXUSUSER", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"},
		[][]byte{[]byte("ibmi-password"), []byte("master-passphrase"), []byte("master-passphrase")},
	)
	var output bytes.Buffer
	var notices bytes.Buffer
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return nil }, InspectKey: fakeHostKeyInspection, Output: &output, Notices: &notices})
	if err != nil {
		t.Fatal(err)
	}
	if profiles.saved.Port != 22 || profiles.saved.MapepireJAR == "" || profiles.saved.CredentialMode != profile.CredentialModeVault || profiles.saved.HostKeyTrust != profile.HostKeyTrustVerified || vaults.setProfile != "dev" {
		t.Fatalf("setup state = %#v, %#v", profiles.saved, vaults)
	}
	if strings.Join(events, ",") != "vault-set,profile-save" {
		t.Fatalf("publication order = %v, want vault before profile commit marker", events)
	}
	var document map[string]any
	decoder := json.NewDecoder(strings.NewReader(output.String()))
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&document); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout contains more than one JSON document: %q", output.String())
	}
	if strings.Contains(output.String(), "Host-key inspection") || !strings.Contains(notices.String(), "Host-key inspection") {
		t.Fatalf("stdout/notices = %q/%q", output.String(), notices.String())
	}
	for _, secret := range []string{"ibmi-password", "master-passphrase"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("output contains secret %q", secret)
		}
	}
}

func TestExecuteSetupStoresAutomaticallyDiscoveredJARWithoutManualPrompt(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "mapepire-server-2.3.5.jar")
	lines := []string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", "yes"}
	lineIndex := 0
	var labels []string
	readLine := func(label string) (string, error) {
		labels = append(labels, label)
		if lineIndex >= len(lines) {
			return "", io.EOF
		}
		value := lines[lineIndex]
		lineIndex++
		return value, nil
	}
	_, readSecret := setupReaders(nil, [][]byte{[]byte("password"), []byte("master"), []byte("master")})
	profiles, vaults := &fakeProfiles{}, &fakeVaults{}
	var notices bytes.Buffer
	verifiedPath := ""
	err := executeSetup(setupDependencies{
		Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret,
		DiscoverJAR: func() mapepire.DiscoveryResult {
			return mapepire.DiscoveryResult{Status: mapepire.DiscoveryFound, Path: jar, VerifiedCandidateCount: 1}
		},
		VerifyJAR: func(path string) error { verifiedPath = path; return nil }, InspectKey: fakeHostKeyInspection,
		Output: io.Discard, Notices: &notices,
	})
	if err != nil {
		t.Fatal(err)
	}
	if profiles.saved.MapepireJAR != jar || verifiedPath != jar || vaults.setProfile != "dev" {
		t.Fatalf("stored/verified/vault = %q/%q/%q", profiles.saved.MapepireJAR, verifiedPath, vaults.setProfile)
	}
	if strings.Contains(strings.Join(labels, "|"), "JAR path") {
		t.Fatalf("manual JAR prompt was used: %v", labels)
	}
	if !strings.Contains(notices.String(), "automatically found and verified") || strings.Contains(notices.String(), jar) {
		t.Fatalf("automatic discovery notice is missing or exposes a path: %q", notices.String())
	}
}

func TestExecuteSetupDiscoveryFallbackIsSanitizedAndVerifiesBeforeSecrets(t *testing.T) {
	tests := []struct {
		name       string
		discovery  mapepire.DiscoveryResult
		wantNotice string
	}{
		{
			name:       "invalid exact-location candidate",
			discovery:  mapepire.DiscoveryResult{Status: mapepire.DiscoveryNotFound, RejectedCandidateCount: 1},
			wantNotice: "1 exact-location candidate(s) failed verification",
		},
		{
			name:       "multiple verified candidates",
			discovery:  mapepire.DiscoveryResult{Status: mapepire.DiscoveryAmbiguous, VerifiedCandidateCount: 2},
			wantNotice: "2 verified candidates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manualPath := filepath.Join(t.TempDir(), "manual-mapepire.jar")
			labels := []string{}
			lines := []string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", manualPath}
			index := 0
			readLine := func(label string) (string, error) {
				labels = append(labels, label)
				value := lines[index]
				index++
				return value, nil
			}
			secretPrompts := 0
			var notices bytes.Buffer
			err := executeSetup(setupDependencies{
				Profiles: &fakeProfiles{}, Vaults: &fakeVaults{}, ReadLine: readLine,
				ReadSecret:  func(string) ([]byte, error) { secretPrompts++; return []byte("secret"), nil },
				DiscoverJAR: func() mapepire.DiscoveryResult { return tt.discovery },
				VerifyJAR:   func(string) error { return errors.New("sensitive verifier detail: C:\\private\\candidate.jar") },
				InspectKey:  fakeHostKeyInspection, Output: io.Discard, Notices: &notices,
			})
			if err == nil || err.Error() != "Mapepire Server JAR verification failed" {
				t.Fatalf("error = %v", err)
			}
			if secretPrompts != 0 {
				t.Fatalf("secret prompts = %d", secretPrompts)
			}
			if !strings.Contains(strings.Join(labels, "|"), "Local Mapepire Server 2.3.5 JAR path") {
				t.Fatalf("manual fallback was not prompted: %v", labels)
			}
			if !strings.Contains(notices.String(), tt.wantNotice) || strings.Contains(notices.String(), manualPath) || strings.Contains(err.Error(), "private") {
				t.Fatalf("fallback output is missing or unsafe: notice=%q error=%q", notices.String(), err)
			}
		})
	}
}

func TestExecuteSetupProductionReaderRequiresByteExactYesBeforeCredentials(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "mapepire.jar")
	tests := []struct {
		name         string
		confirmation string
		wantSuccess  bool
	}{
		{name: "exact yes enrolls TOFU", confirmation: "yes", wantSuccess: true},
		{name: "leading space rejects", confirmation: " yes"},
		{name: "trailing space rejects", confirmation: "yes "},
		{name: "leading tab rejects", confirmation: "\tyes"},
		{name: "trailing tab rejects", confirmation: "yes\t"},
		{name: "uppercase rejects", confirmation: "YES"},
		{name: "empty rejects", confirmation: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles, vaults := &fakeProfiles{}, &fakeVaults{}
			input := strings.Join([]string{" dev ", " ibmi.example.test ", "22", " inspect ", tt.confirmation, "USER", "", jar, "yes"}, "\n") + "\n"
			readLine, readExact := setupLineReaders(strings.NewReader(input), io.Discard)
			_, baseSecretReader := setupReaders(nil, [][]byte{[]byte("ibmi-password"), []byte("master-passphrase"), []byte("master-passphrase")})
			secretPrompts := 0
			readSecret := func(label string) ([]byte, error) {
				secretPrompts++
				return baseSecretReader(label)
			}
			var notices bytes.Buffer
			err := executeSetup(setupDependencies{
				Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadExact: readExact, ReadSecret: readSecret,
				DiscoverJAR: noJARDiscovery,
				VerifyJAR:   func(string) error { return nil },
				InspectKey: func(ctx context.Context, host string, port int) (remote.HostKeyObservation, error) {
					if _, ok := ctx.Deadline(); !ok || host != "ibmi.example.test" || port != 22 {
						t.Fatalf("invalid probe request: deadline=%v host=%q port=%d", ok, host, port)
					}
					return fakeHostKeyInspection(ctx, host, port)
				},
				Output: io.Discard, Notices: &notices,
			})
			if tt.wantSuccess {
				if err != nil {
					t.Fatal(err)
				}
				if secretPrompts != 3 || profiles.saved.HostKeyTrust != profile.HostKeyTrustTOFU || profiles.saved.HostKeyFingerprint == "" || vaults.setProfile != "dev" {
					t.Fatalf("prompts/profile/vault = %d/%#v/%q", secretPrompts, profiles.saved, vaults.setProfile)
				}
				if !strings.Contains(notices.String(), "not independently verified") || !strings.Contains(notices.String(), ssh.KeyAlgoED25519) {
					t.Fatalf("notices = %q", notices.String())
				}
				return
			}
			if err == nil {
				t.Fatal("expected inspection enrollment rejection")
			}
			if secretPrompts != 0 || profiles.saved.Name != "" || vaults.setProfile != "" {
				t.Fatalf("rejected setup prompted or persisted: %d/%#v/%#v", secretPrompts, profiles, vaults)
			}
		})
	}
}

func TestExecuteSetupRejectsManualFingerprintBeforeCredentials(t *testing.T) {
	profiles, vaults := &fakeProfiles{}, &fakeVaults{}
	readLine, _ := setupReaders([]string{"dev", "ibmi.example.test", "22", "manual", "not-a-fingerprint"}, nil)
	secretPrompts := 0
	err := executeSetup(setupDependencies{
		Profiles: profiles, Vaults: vaults, ReadLine: readLine,
		ReadSecret:  func(string) ([]byte, error) { secretPrompts++; return []byte("should-not-be-read"), nil },
		DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return nil }, InspectKey: fakeHostKeyInspection,
		Output: io.Discard, Notices: io.Discard,
	})
	if err == nil || secretPrompts != 0 || profiles.saved.Name != "" || vaults.setProfile != "" {
		t.Fatalf("error/prompts/profile/vault = %v/%d/%#v/%#v", err, secretPrompts, profiles, vaults)
	}
}

func TestConfigureRequiresExplicitHostKeyTrust(t *testing.T) {
	err := runConfigure([]string{
		"-name", "dev", "-host", "ibmi.example.test", "-port", "22", "-username", "USER",
		"-host-key-sha256", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"-credential-mode", "prompt", "-config-root", t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "host-key trust") {
		t.Fatalf("error = %v, want explicit host-key trust rejection", err)
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
		{"master mismatch", []string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", jar}, [][]byte{[]byte("password"), []byte("master-a"), []byte("master-b")}, nil},
		{"invalid profile", []string{"../dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", jar}, [][]byte{[]byte("password"), []byte("master"), []byte("master")}, nil},
		{"invalid fingerprint", []string{"dev", "ibmi.example.test", "22", "manual", "untrusted", "USER", "", jar}, [][]byte{[]byte("password"), []byte("master"), []byte("master")}, nil},
		{"invalid jar", []string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", jar}, [][]byte{[]byte("password"), []byte("master"), []byte("master")}, errors.New("checksum mismatch")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles, vaults := &fakeProfiles{}, &fakeVaults{}
			readLine, readSecret := setupReaders(tt.lines, tt.secrets)
			err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return tt.verifyErr }, InspectKey: fakeHostKeyInspection, Output: io.Discard, Notices: io.Discard})
			if err == nil {
				t.Fatal("expected setup rejection")
			}
			if profiles.saved.Name != "" || vaults.setProfile != "" {
				t.Fatal("invalid setup persisted state")
			}
		})
	}
}

func TestExecuteSetupVaultFailureLeavesNoProfile(t *testing.T) {
	profiles := &fakeProfiles{}
	vaults := &fakeVaults{setErr: errors.New("vault failed")}
	readLine, readSecret := setupReaders([]string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"}, [][]byte{[]byte("password"), []byte("master"), []byte("master")})
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return nil }, InspectKey: fakeHostKeyInspection, Output: io.Discard, Notices: io.Discard})
	if err == nil || profiles.saved.Name != "" {
		t.Fatalf("error/profile = %v/%#v", err, profiles.saved)
	}
}

func TestExecuteSetupProfileFailureDeletesVault(t *testing.T) {
	profiles := &fakeProfiles{saveErr: errors.New("profile failed")}
	vaults := &fakeVaults{}
	readLine, readSecret := setupReaders([]string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"}, [][]byte{[]byte("password"), []byte("master"), []byte("master")})
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return nil }, InspectKey: fakeHostKeyInspection, Output: io.Discard, Notices: io.Discard})
	if err == nil || profiles.saved.Name != "" || vaults.setProfile != "dev" || vaults.deleted != "dev" {
		t.Fatalf("error/profile/vault = %v/%#v/%#v", err, profiles.saved, vaults)
	}
}

func TestExecuteSetupProfileFailureAndVaultCleanupFailureIsRecoverable(t *testing.T) {
	profileErr := errors.New("profile publication failed")
	cleanupErr := errors.New("vault deletion failed")
	profiles := &fakeProfiles{saveErr: profileErr}
	store := credential.Store{Root: t.TempDir()}
	vaults := &fakeVaults{deleteErr: cleanupErr, store: &store}
	readLine, readSecret := setupReaders([]string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"}, [][]byte{[]byte("password"), []byte("master"), []byte("master")})
	err := executeSetup(setupDependencies{Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret, DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return nil }, InspectKey: fakeHostKeyInspection, Output: io.Discard, Notices: io.Discard})

	var orphan *OrphanVaultError
	if !errors.As(err, &orphan) || !orphan.Recoverable() || orphan.Profile != "dev" || !errors.Is(err, profileErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("error = %#v, want joined recoverable orphan evidence", err)
	}
	if profiles.saved.Name != "" || vaults.setProfile != "dev" || vaults.deleted != "dev" {
		t.Fatalf("profile/vault = %#v/%#v", profiles.saved, vaults)
	}
	if strings.Contains(err.Error(), "password") || !strings.Contains(err.Error(), "credentials status/delete -profile \"dev\"") {
		t.Fatalf("orphan evidence is unsafe or not actionable: %v", err)
	}
	var status bytes.Buffer
	if err := writeCredentialStatus(&status, store, "dev"); err != nil {
		t.Fatal(err)
	}
	if status.String() != "{\"exists\":true}\n" {
		t.Fatalf("status = %q", status.String())
	}
	var deletion bytes.Buffer
	if err := writeCredentialDelete(&deletion, store, "dev"); err != nil {
		t.Fatal(err)
	}
	if deletion.String() != "{\"deleted\":true}\n" {
		t.Fatalf("delete = %q", deletion.String())
	}
	exists, err := store.Status("dev")
	if err != nil || exists {
		t.Fatalf("post-delete status = %v, %v", exists, err)
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
		[]string{"dev", "ibmi.example.test", "22", "manual", "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "USER", "", filepath.Join(t.TempDir(), "mapepire.jar"), "yes"},
		[][]byte{[]byte("password"), []byte("master"), []byte("master")},
	)
	writeErr := errors.New("injected write failure")
	err := executeSetup(setupDependencies{
		Profiles: profiles, Vaults: vaults, ReadLine: readLine, ReadSecret: readSecret,
		DiscoverJAR: noJARDiscovery, VerifyJAR: func(string) error { return nil }, InspectKey: fakeHostKeyInspection, Output: failingWriter{writeErr}, Notices: io.Discard,
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
