package profile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func validProfile() Profile {
	return Profile{Name: "dev", Host: "ibmi.example.test", Port: 22, Username: "NEXUS$USER", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", JavaHome: "/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit", CredentialMode: CredentialModeVault}
}

func TestProfileValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Profile)
		valid  bool
	}{
		{"valid", func(*Profile) {}, true},
		{"host with port", func(p *Profile) { p.Host = "host:22" }, false},
		{"unknown fingerprint", func(p *Profile) { p.HostKeyFingerprint = "" }, false},
		{"invalid fingerprint", func(p *Profile) { p.HostKeyFingerprint = "SHA256:not-base64" }, false},
		{"non-canonical fingerprint base64", func(p *Profile) { p.HostKeyFingerprint = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB" }, false},
		{"unsafe Java path", func(p *Profile) { p.JavaHome = "/tmp/java;id" }, false},
		{"default Java discovery", func(p *Profile) { p.JavaHome = "" }, true},
		{"relative Mapepire JAR", func(p *Profile) { p.MapepireJAR = "mapepire.jar" }, false},
		{"prompt credentials", func(p *Profile) { p.CredentialMode = CredentialModePrompt }, true},
		{"missing credential mode", func(p *Profile) { p.CredentialMode = "" }, false},
		{"unknown credential mode", func(p *Profile) { p.CredentialMode = "environment" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)
			if got := p.Validate() == nil; got != tt.valid {
				t.Fatalf("Validate() success = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestStoreRoundTripUsesTemporaryRoot(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	p := validProfile()
	path, err := store.Save(p)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != root {
		t.Fatalf("saved outside temporary root: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("profile permissions = %o, want no group/other access", info.Mode().Perm())
	}
	got, err := store.Load(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("Load() = %#v, want %#v", got, p)
	}
	if _, err := store.Save(p); err == nil {
		t.Fatal("expected existing profile to fail closed")
	}
}

func TestLoadRejectsTraversal(t *testing.T) {
	if _, err := (Store{Root: t.TempDir()}).Load("../profile"); err == nil {
		t.Fatal("expected traversal name rejection")
	}
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
	root := t.TempDir()
	p := validProfile()
	data := `{"name":"dev","host":"ibmi.example.test","port":22,"username":"NEXUS$USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","credentialMode":"vault"} {}`
	if err := os.WriteFile(filepath.Join(root, p.Name+".json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Root: root}).Load(p.Name); err == nil {
		t.Fatal("expected trailing JSON rejection")
	}
}

func TestLoadRejectsDuplicateCaseVariantAndLegacyMode(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"duplicate", `{"name":"dev","name":"other","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","credentialMode":"vault"}`},
		{"case variant", `{"name":"dev","Name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","credentialMode":"vault"}`},
		{"legacy missing mode", `{"name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "dev.json"), []byte(tt.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := (Store{Root: root}).Load("dev"); err == nil {
				t.Fatal("malformed or legacy profile was accepted")
			}
		})
	}
}

func TestConcurrentSaveCreatesExactlyOneProfile(t *testing.T) {
	store := Store{Root: t.TempDir()}
	p := validProfile()
	const attempts = 16
	start := make(chan struct{})
	errorsByAttempt := make(chan error, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := store.Save(p)
			errorsByAttempt <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByAttempt)
	successes := 0
	for err := range errorsByAttempt {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("unexpected save error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful saves = %d, want 1", successes)
	}
	if _, err := store.Load(p.Name); err != nil {
		t.Fatalf("winning profile is incomplete: %v", err)
	}
}
