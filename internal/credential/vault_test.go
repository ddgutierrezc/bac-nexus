package credential

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	parameters := Parameters{Time: 1, MemoryKiB: 64, Threads: 1, KeyLen: 32}
	return Store{Root: t.TempDir(), Random: bytes.NewReader(bytes.Repeat([]byte{7}, 128)), Parameters: parameters, policy: &parameterPolicy{approved: []Parameters{parameters}}}
}

func TestVaultRoundTripAndAuthentication(t *testing.T) {
	store := testStore(t)
	password := []byte("ibmi-secret")
	master := []byte("master-secret")
	if _, err := store.Set("dev", password, master, false); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("dev", master)
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(got)
	if !bytes.Equal(got, password) {
		t.Fatal("decrypted password differs")
	}
	if _, err := store.Get("dev", []byte("wrong")); err == nil || strings.Contains(err.Error(), "ibmi-secret") {
		t.Fatalf("wrong-master error = %v", err)
	}
	if err := os.Link(filepath.Join(store.Root, "dev.vault"), filepath.Join(store.Root, "other.vault")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("other", master); err == nil {
		t.Fatal("vault profile swap authenticated")
	}
	data, err := os.ReadFile(filepath.Join(store.Root, "dev.vault"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, password) || bytes.Contains(data, master) {
		t.Fatal("vault JSON contains plaintext secret")
	}
}

func TestVaultRejectsTamperingAndInvalidEnvelopes(t *testing.T) {
	store := testStore(t)
	if _, err := store.Set("dev", []byte("password"), []byte("master"), false); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(store.Root, "dev.vault"))
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]any
	if err := json.Unmarshal(original, &base); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any) []byte
	}{
		{"ciphertext", func(v map[string]any) []byte {
			v["ciphertext"] = "AAAAAAAAAAAAAAAAAAAAAAA"
			b, _ := json.Marshal(v)
			return b
		}},
		{"salt", func(v map[string]any) []byte { v["salt"] = "AAAAAAAAAAAAAAAAAAAAAA"; b, _ := json.Marshal(v); return b }},
		{"nonce", func(v map[string]any) []byte { v["nonce"] = "AAAAAAAAAAAAAAAA"; b, _ := json.Marshal(v); return b }},
		{"malicious memory", func(v map[string]any) []byte {
			v["kdfMemoryKiB"] = float64(1024 * 1024)
			b, _ := json.Marshal(v)
			return b
		}},
		{"duplicate key", func(v map[string]any) []byte {
			return bytes.Replace(original, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
		}},
		{"case-variant key", func(v map[string]any) []byte {
			return bytes.Replace(original, []byte(`"version":1`), []byte(`"version":1,"Version":1`), 1)
		}},
		{"base64 newline", func(v map[string]any) []byte {
			v["salt"] = v["salt"].(string) + "\n"
			b, _ := json.Marshal(v)
			return b
		}},
		{"base64 whitespace", func(v map[string]any) []byte {
			v["nonce"] = " " + v["nonce"].(string)
			b, _ := json.Marshal(v)
			return b
		}},
		{"base64 padding", func(v map[string]any) []byte {
			v["salt"] = v["salt"].(string) + "=="
			b, _ := json.Marshal(v)
			return b
		}},
		{"unknown field", func(v map[string]any) []byte { v["extra"] = true; b, _ := json.Marshal(v); return b }},
		{"trailing", func(v map[string]any) []byte { return append(original, []byte(" {}")...) }},
		{"invalid JSON", func(v map[string]any) []byte { return []byte("{") }},
		{"oversized", func(v map[string]any) []byte { return bytes.Repeat([]byte{'x'}, MaxVaultBytes+1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyValue := make(map[string]any, len(base))
			for key, value := range base {
				copyValue[key] = value
			}
			if err := os.WriteFile(filepath.Join(store.Root, "case.vault"), tt.mutate(copyValue), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get("case", []byte("master")); err == nil {
				t.Fatal("tampered or malformed vault accepted")
			}
		})
	}
}

func TestProductionKDFPolicyAcceptsOnlyApprovedVersionedTuple(t *testing.T) {
	base := envelope{
		Version: Version, KDFTime: DefaultTime, KDFMemoryKiB: DefaultMemoryKiB, KDFThreads: DefaultThreads, KDFKeyLen: DefaultKeyLen,
		Salt: encode(make([]byte, 16)), Nonce: encode(make([]byte, 12)), Ciphertext: encode(make([]byte, 17)),
	}
	tests := []struct {
		name   string
		mutate func(*envelope)
		valid  bool
	}{
		{"approved", func(*envelope) {}, true},
		{"weak", func(v *envelope) { v.KDFTime = 1 }, false},
		{"modified", func(v *envelope) { v.KDFThreads = 3 }, false},
		{"excessive", func(v *envelope) { v.KDFMemoryKiB = 1024 * 1024 }, false},
		{"different key length", func(v *envelope) { v.KDFKeyLen = 16 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := base
			tt.mutate(&value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, _, err = decodeEnvelope(data, &productionPolicy)
			if (err == nil) != tt.valid {
				t.Fatalf("decode success = %v, want %v; error = %v", err == nil, tt.valid, err)
			}
		})
	}
}

func TestVaultCreateRotateStatusAndDelete(t *testing.T) {
	store := testStore(t)
	master := []byte("master")
	if exists, err := store.Status("dev"); err != nil || exists {
		t.Fatalf("initial status = %v, %v", exists, err)
	}
	if _, err := store.Set("dev", []byte("first"), master, false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set("dev", []byte("second"), master, false); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite error = %v", err)
	}
	if _, err := store.Set("dev", []byte("second"), master, true); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("dev", master)
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(got)
	if string(got) != "second" {
		t.Fatal("rotation was not published")
	}
	if exists, err := store.Status("dev"); err != nil || !exists {
		t.Fatalf("status = %v, %v", exists, err)
	}
	if deleted, err := store.Delete("dev"); err != nil || !deleted {
		t.Fatalf("delete = %v, %v", deleted, err)
	}
	if deleted, err := store.Delete("dev"); err != nil || deleted {
		t.Fatalf("idempotent delete = %v, %v", deleted, err)
	}
}

func TestVaultUsesCanonicalProfileNameValidation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		valid bool
	}{
		{"CRI400F-Dev_1", true},
		{"CRI400F.Dev", false},
		{"CRIñ400F", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := testStore(t).Set(tt.name, []byte("password"), []byte("master"), false)
			if (err == nil) != tt.valid {
				t.Fatalf("Set(%q) error = %v, want valid=%v", tt.name, err, tt.valid)
			}
		})
	}
}

func TestVaultRotationReportsCommittedCleanupWarningAndRetriesCleanup(t *testing.T) {
	store := testStore(t)
	master := []byte("master")
	if _, err := store.Set("dev", []byte("first"), master, false); err != nil {
		t.Fatal(err)
	}
	removeCalls := 0
	store.files = &fileOperations{
		link: os.Link,
		stat: os.Stat,
		remove: func(path string) error {
			if strings.HasSuffix(path, ".rollback") {
				if _, err := os.Stat(path); err == nil {
					removeCalls++
					if removeCalls == 1 {
						return errors.New("injected cleanup failure")
					}
				}
			}
			return os.Remove(path)
		},
	}
	result, err := store.Set("dev", []byte("second"), master, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Committed || result.CleanupWarning == nil {
		t.Fatalf("rotation result = %#v", result)
	}
	if exists, err := store.Status("dev"); err != nil || !exists {
		t.Fatalf("status/retry cleanup = %v, %v", exists, err)
	}
	if _, err := os.Stat(filepath.Join(store.Root, "dev.vault.rollback")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback remains after retry: %v", err)
	}
}

func TestVaultRotationRollsBackInjectedPublicationFailures(t *testing.T) {
	steps := []string{"backup", "remove-final", "publish"}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			store := testStore(t)
			master := []byte("master")
			if _, err := store.Set("dev", []byte("first"), master, false); err != nil {
				t.Fatal(err)
			}
			links, removes := 0, 0
			store.files = &fileOperations{
				stat: os.Stat,
				link: func(oldPath, newPath string) error {
					links++
					if (step == "backup" && links == 1) || (step == "publish" && links == 2) {
						return errors.New("injected link failure")
					}
					return os.Link(oldPath, newPath)
				},
				remove: func(path string) error {
					if !strings.HasSuffix(path, ".rollback") {
						removes++
						if step == "remove-final" && removes == 1 {
							return errors.New("injected remove failure")
						}
					}
					return os.Remove(path)
				},
			}
			if _, err := store.Set("dev", []byte("second"), master, true); err == nil {
				t.Fatal("expected injected rotation failure")
			}
			store.files = nil
			got, err := store.Get("dev", master)
			if err != nil {
				t.Fatal(err)
			}
			defer Zero(got)
			if string(got) != "first" {
				t.Fatalf("original vault not preserved: %q", got)
			}
		})
	}
}

func TestZero(t *testing.T) {
	value := []byte("secret")
	Zero(value)
	if !bytes.Equal(value, make([]byte, len(value))) {
		t.Fatal("bytes were not zeroed")
	}
}
