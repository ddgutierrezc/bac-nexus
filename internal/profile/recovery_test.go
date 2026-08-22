package profile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func recoveryProfile(name string) Profile {
	p := validProfile()
	p.Name = name
	return p
}

func writeRecoveryProfile(t *testing.T, root string, p Profile) {
	t.Helper()
	data := []byte(`{"name":"` + p.Name + `","host":"ibmi.example.test","port":22,"username":"NEXUS$USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified","credentialMode":"vault"}`)
	if err := os.WriteFile(filepath.Join(root, p.Name+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStoreListIsBoundedDeterministicAndSkipsUnsafeEntries(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"zeta", "alpha", "middle"} {
		writeRecoveryProfile(t, root, recoveryProfile(name))
	}
	if err := os.WriteFile(filepath.Join(root, "unknown.json"), []byte(`{"name":"unknown","unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wrong.json"), []byte(`{"name":"other","host":"ibmi.example.test","port":22,"username":"NEXUS$USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified","credentialMode":"vault"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "too-large.json"), []byte(strings.Repeat("x", maxProfileBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alpha.bak"), []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".profile-x.tmp"), []byte("temp"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(root, "alpha.json"), filepath.Join(root, "linked.json")); err != nil {
			t.Fatal(err)
		}
	}

	got, err := (Store{Root: root}).List(128)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Name != "alpha" || got[1].Name != "middle" || got[2].Name != "zeta" {
		t.Fatalf("List() = %#v, want sorted valid profiles", got)
	}

	limited, err := (Store{Root: root}).List(2)
	if err != nil || len(limited) != 2 || limited[0].Name != "alpha" || limited[1].Name != "middle" {
		t.Fatalf("bounded List() = %#v, %v", limited, err)
	}
	if _, err := (Store{Root: root}).List(129); err == nil {
		t.Fatal("List(129) unexpectedly succeeded")
	}
}

func TestStoreListEmptyAndInvalidRootsFailClosed(t *testing.T) {
	if got, err := (Store{Root: t.TempDir()}).List(1); err != nil || len(got) != 0 {
		t.Fatalf("empty List() = %#v, %v", got, err)
	}
	for _, root := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if _, err := (Store{Root: root}).List(1); !errors.Is(err, ErrInvalidRoot) {
			t.Fatalf("List(%q) error = %v, want ErrInvalidRoot", root, err)
		}
	}
}

func TestStoreUpdateCreatesBackupAndAtomicallyPublishes(t *testing.T) {
	root := t.TempDir()
	old := recoveryProfile("dev")
	writeRecoveryProfile(t, root, old)
	updated := old
	updated.Host = "new.example.test"

	result, err := (Store{Root: root}).Update(updated, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !result.ReplacementCommitted || !result.FileReplaced || result.PreviousBackup != "dev.bak" {
		t.Fatalf("Update() result = %#v", result)
	}
	got, err := (Store{Root: root}).Load("dev")
	if err != nil || got.Host != updated.Host {
		t.Fatalf("updated profile = %#v, %v", got, err)
	}
	backup, err := os.Stat(filepath.Join(root, "dev.bak"))
	if err != nil || !backup.Mode().IsRegular() || (runtime.GOOS != "windows" && backup.Mode().Perm()&0o077 != 0) {
		t.Fatalf("backup = %#v, %v", backup, err)
	}
	backupData, err := os.ReadFile(filepath.Join(root, "dev.bak"))
	if err != nil || !strings.Contains(string(backupData), `"host":"ibmi.example.test"`) {
		t.Fatalf("backup does not retain previous profile: %s, %v", backupData, err)
	}
}

func TestStoreUpdateValidatesAndRejectsConflictsBeforeReplacement(t *testing.T) {
	root := t.TempDir()
	old := recoveryProfile("dev")
	writeRecoveryProfile(t, root, old)
	before, err := os.ReadFile(filepath.Join(root, "dev.json"))
	if err != nil {
		t.Fatal(err)
	}

	invalid := old
	invalid.Host = "not a host"
	if _, err := (Store{Root: root}).Update(invalid, "dev"); err == nil {
		t.Fatal("invalid update unexpectedly succeeded")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "dev.json")); string(got) != string(before) {
		t.Fatal("invalid update changed the live profile")
	}
	if _, err := (Store{Root: root}).Update(old, "other"); !errors.Is(err, ErrInvalidUpdateTarget) {
		t.Fatalf("conflicting update error = %v", err)
	}
	if _, err := (Store{Root: root}).Update(old, "../dev"); !errors.Is(err, ErrInvalidUpdateTarget) {
		t.Fatalf("traversal update error = %v", err)
	}
}

func TestStoreUpdateRejectsLinkedOrNonRegularTargetsWithoutLeakage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows runners")
	}
	root := t.TempDir()
	p := recoveryProfile("dev")
	outside := t.TempDir()
	writeRecoveryProfile(t, outside, p)
	if err := os.Symlink(filepath.Join(outside, "dev.json"), filepath.Join(root, "dev.json")); err != nil {
		t.Fatal(err)
	}
	err := func() error {
		_, err := (Store{Root: root}).Update(p, "dev")
		return err
	}()
	if !errors.Is(err, ErrInvalidUpdateTarget) || strings.Contains(err.Error(), root) || strings.Contains(err.Error(), "dev") {
		t.Fatalf("linked target error = %v, want sanitized invalid target", err)
	}
}

func TestStoreUpdateRestoresWhenReplacementCannotCommit(t *testing.T) {
	root := t.TempDir()
	old := recoveryProfile("dev")
	writeRecoveryProfile(t, root, old)
	updated := old
	updated.Host = "new.example.test"
	if err := os.Mkdir(filepath.Join(root, "dev.bak"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Root: root}).Update(updated, "dev"); err == nil {
		t.Fatal("update with non-regular backup unexpectedly succeeded")
	}
	got, err := (Store{Root: root}).Load("dev")
	if err != nil || got.Name != old.Name || got.Host != old.Host {
		t.Fatalf("old profile was not retained: %#v, %v", got, err)
	}
}
