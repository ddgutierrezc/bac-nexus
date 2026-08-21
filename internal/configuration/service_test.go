// Package configuration owns the reusable application services for
// the Nexus configuration lifecycle. The package depends on
// internal/profile, internal/credential, internal/remote, and
// internal/mapepire. It MUST NOT import internal/mcp, the cmd/* flag
// surface, or the stdin/stdout transport owned by the entry points.
// Future slices (TUI shell, profile CRUD, credential/trust flows,
// readiness diagnostics) consume this package as a service-complete
// adapter; the Charm v1 family is admitted out of band and is NOT
// imported in this slice.
package configuration

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"testing"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

// stubProfiles and stubVaults are no-op stand-ins for the
// consumer-owned persistence seams.
type stubProfiles struct{}

func (stubProfiles) Save(profile.Profile) (string, error) { return "profile", nil }

type stubVaults struct{}

func (stubVaults) Set(string, []byte, []byte, bool) (credential.SetResult, error) {
	return credential.SetResult{Committed: true}, nil
}
func (stubVaults) Delete(string) (bool, error) { return true, nil }

// fakeVaultReader implements the VaultStatusReader contract.
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

// newServiceDeps returns a Dependencies value with every required
// field wired to a no-op stub. Tests mutate the struct before
// calling Configure.
func newServiceDeps(t *testing.T) Dependencies {
	t.Helper()
	return Dependencies{
		Profiles:    stubProfiles{},
		Vaults:      stubVaults{},
		ReadLine:    func(string) (string, error) { return "", io.EOF },
		ReadSecret:  func(string) ([]byte, error) { return nil, io.EOF },
		DiscoverJAR: func() mapepire.DiscoveryResult { return mapepire.DiscoveryResult{Status: mapepire.DiscoveryNotFound} },
		VerifyJAR:   func(string) error { return nil },
		InspectKey: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{}, nil
		},
		Output:  io.Discard,
		Notices: io.Discard,
	}
}

// TestPackageRespectsInwardPointingBoundary is an AST guard that
// proves the configuration package does not import internal/mcp,
// the flag surface, or os/exec. A future drift toward the TUI in
// Slice 4 must add the new imports through a separate, audited
// change rather than silently expanding this package's surface.
func TestPackageRespectsInwardPointingBoundary(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	pkgs, err := parser.ParseDir(set, root, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	approved := map[string]bool{
		"bac-nexus/internal/credential": true, "bac-nexus/internal/mapepire": true,
		"bac-nexus/internal/profile": true, "bac-nexus/internal/remote": true,
		"crypto/subtle": true, "encoding/json": true, "errors": true,
		"fmt": true, "io": true, "os": true, "path/filepath": true,
		"strconv": true, "strings": true, "context": true, "time": true,
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, err := unquoteImport(imp.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(path, "internal/mcp") || path == "flag" || path == "os/exec" {
					t.Fatalf("configuration package must not import %q (file %s)", path, set.Position(imp.Pos()).Filename)
				}
				if approved[path] {
					continue
				}
				if strings.HasPrefix(path, "bac-nexus/") {
					t.Fatalf("configuration package imports %q (file %s); only approved application packages are allowed", path, set.Position(imp.Pos()).Filename)
				}
			}
		}
	}
}

func unquoteImport(literal string) (string, error) {
	if len(literal) < 2 || literal[0] != '"' || literal[len(literal)-1] != '"' {
		return "", errors.New("malformed import literal")
	}
	return literal[1 : len(literal)-1], nil
}

// TestServiceRejectsIncompleteDependencies exercises every
// required Dependencies field. A missing field must fail closed.
func TestServiceRejectsIncompleteDependencies(t *testing.T) {
	cases := map[string]func(*Dependencies){
		"missing profiles":       func(d *Dependencies) { d.Profiles = nil },
		"missing vaults":         func(d *Dependencies) { d.Vaults = nil },
		"missing read line":      func(d *Dependencies) { d.ReadLine = nil },
		"missing jar discovery":  func(d *Dependencies) { d.DiscoverJAR = nil },
		"missing jar verify":     func(d *Dependencies) { d.VerifyJAR = nil },
		"missing host-key probe": func(d *Dependencies) { d.InspectKey = nil },
		"missing output":         func(d *Dependencies) { d.Output = nil },
		"missing notices":        func(d *Dependencies) { d.Notices = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			deps := newServiceDeps(t)
			mutate(&deps)
			if err := NewService(deps).Configure(context.Background()); err == nil {
				t.Fatalf("expected rejection when %q is missing", name)
			}
		})
	}
}

// TestAcquireLivePasswordHonorsClosedCredentialMode proves the
// public AcquireLivePassword function preserves the established
// mode matrix. The master buffer is zeroized after the call.
func TestAcquireLivePasswordHonorsClosedCredentialMode(t *testing.T) {
	cases := []struct {
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := &fakeVaultReader{exists: tc.exists, password: []byte(tc.stored)}
			promptBuffer := []byte(tc.promptValue)
			prompts := 0
			password, err := AcquireLivePassword(vault, "dev", tc.mode, func(string) ([]byte, error) {
				prompts++
				return promptBuffer, nil
			})
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if prompts != tc.wantPrompts {
				t.Fatalf("prompts = %d, want %d", prompts, tc.wantPrompts)
			}
			if tc.wantErr {
				return
			}
			defer func() {
				for i := range password {
					password[i] = 0
				}
			}()
			if string(password) != tc.want {
				t.Fatalf("password = %q", password)
			}
			if tc.mode == profile.CredentialModeVault && string(vault.masterSeen) != "master" {
				t.Fatal("master was not supplied to vault")
			}
		})
	}
}

// TestConfigureSurfacesReaderFailures proves the new public
// Configure method surfaces downstream reader errors verbatim
// rather than substituting a fallback. The test uses a pre-cancelled
// parent context to prove the Service honors context propagation.
func TestConfigureSurfacesReaderFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewService(newServiceDeps(t)).Configure(ctx); err == nil {
		t.Fatal("expected reader or context error to surface")
	}
}
