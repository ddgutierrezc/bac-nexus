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

func TestStoreRejectsDottedProfileNamesAtEveryBoundary(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	dotted := recoveryProfile("CRI400F.Dev")
	if _, err := store.Save(dotted); err == nil {
		t.Fatal("Save accepted a dotted profile name")
	}
	writeRecoveryProfile(t, root, dotted)
	writeRecoveryProfile(t, root, recoveryProfile("CRI400FDev"))
	listed, err := store.List(16)
	if err != nil || len(listed) != 1 || listed[0].Name != "CRI400FDev" {
		t.Fatalf("List() included dotted profile: %#v, %v", listed, err)
	}
	if _, err := store.Load(dotted.Name); err == nil {
		t.Fatal("Load accepted a dotted profile name")
	}
	if _, err := store.Read(dotted.Name); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Read dotted name error = %v", err)
	}
	if _, err := store.Update(dotted, dotted.Name); err == nil {
		t.Fatal("Update accepted a dotted profile name")
	}
	if _, err := store.Delete(dotted.Name, DeleteConfirmation("delete "+dotted.Name)); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Delete dotted name error = %v", err)
	}
	if err := store.Restore(dotted.Name); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Restore dotted name error = %v", err)
	}
}

func TestStoreListEmptyAndInvalidRootsFailClosed(t *testing.T) {
	if got, err := (Store{Root: t.TempDir()}).List(1); err != nil || len(got) != 0 {
		t.Fatalf("empty List() = %#v, %v", got, err)
	}
	for _, root := range []string{""} {
		if _, err := (Store{Root: root}).List(1); !errors.Is(err, ErrInvalidRoot) {
			t.Fatalf("List(%q) error = %v, want ErrInvalidRoot", root, err)
		}
	}
}

func TestStoreListTreatsMissingRootAsEmptyFirstRun(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	got, err := (Store{Root: root}).List(8)
	if err != nil {
		t.Fatalf("List missing root = %v, want empty first-run list", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing root List() = %#v, want empty", got)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing root was created by List(): stat = %v", err)
	}
}

func TestStoreListRejectsSymlinkAndFileRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows runners")
	}
	parent := t.TempDir()
	real := t.TempDir()
	if err := os.WriteFile(filepath.Join(real, "anchor"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkRoot := filepath.Join(parent, "linked")
	if err := os.Symlink(real, symlinkRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Root: symlinkRoot}).List(1); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("symlink root List() = %v, want ErrInvalidRoot", err)
	}
	fileRoot := filepath.Join(parent, "file-root")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Root: fileRoot}).List(1); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("file root List() = %v, want ErrInvalidRoot", err)
	}
}

func TestStoreMutationRejectsMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	p := validProfile()
	if _, err := (Store{Root: root}).Update(p, "dev"); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("Update missing root = %v, want ErrInvalidRoot", err)
	}
	if _, err := (Store{Root: root}).Delete("dev", DeleteConfirmation("delete dev")); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("Delete missing root = %v, want ErrInvalidRoot", err)
	}
	if err := (Store{Root: root}).Restore("dev"); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("Restore missing root = %v, want ErrInvalidRoot", err)
	}
	if _, err := (Store{Root: root}).Read("dev"); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("Read missing root = %v, want ErrInvalidRoot", err)
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

func TestStoreUpdateRestoresAfterReplacementStarts(t *testing.T) {
	root := t.TempDir()
	old := recoveryProfile("dev")
	writeRecoveryProfile(t, root, old)
	updated := old
	updated.Host = "new.example.test"
	store := Store{Root: root}
	calls := 0
	store.replace = func(source, destination string) error {
		calls++
		if calls == 2 {
			return errors.New("injected replacement failure")
		}
		return store.atomicReplace(source, destination)
	}
	if _, err := store.Update(updated, "dev"); err == nil {
		t.Fatal("replacement failure unexpectedly succeeded")
	}
	got, err := store.Load("dev")
	if err != nil || got.Host != old.Host {
		t.Fatalf("replacement failure did not restore old profile: %#v, %v", got, err)
	}
}

func TestStoreDeleteRequiresExactConfirmationAndRetainsBackup(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	p := validProfile()
	if _, err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	for _, confirmation := range []DeleteConfirmation{"", "yes", "delete DEV", "delete dev "} {
		if _, err := store.Delete(p.Name, confirmation); err == nil {
			t.Fatalf("Delete accepted %q", confirmation)
		}
		if _, err := os.Stat(filepath.Join(root, p.Name+".json")); err != nil {
			t.Fatalf("rejected delete removed live profile: %v", err)
		}
	}
	result, err := store.Delete(p.Name, DeleteConfirmation("delete "+p.Name))
	if err != nil || !result.Deleted || result.CredentialOutcome != CredentialOutcomeUntouched {
		t.Fatalf("Delete() = %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, p.Name+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("live profile still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, p.Name+".bak")); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if err := store.Restore(p.Name); err != nil {
		t.Fatalf("Restore() = %v", err)
	}
	if got, err := store.Read(p.Name); err != nil || got != p {
		t.Fatalf("restored profile = %#v, %v", got, err)
	}
}

func TestStoreDeleteRejectsUnsafeRecoveryArtifactsWithoutLeakage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows runners")
	}
	root := t.TempDir()
	target := t.TempDir()
	p := validProfile()
	if _, err := (Store{Root: target}).Save(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(target, p.Name+".json"), filepath.Join(root, p.Name+".json")); err != nil {
		t.Fatal(err)
	}
	_, err := (Store{Root: root}).Delete(p.Name, DeleteConfirmation("delete "+p.Name))
	if !errors.Is(err, ErrInvalidUpdateTarget) || strings.Contains(err.Error(), root) || strings.Contains(err.Error(), p.Name) {
		t.Fatalf("unsafe delete error = %v", err)
	}
}
