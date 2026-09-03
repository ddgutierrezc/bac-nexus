package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bac-nexus/internal/localstate"
)

type approvedSecurePath struct{}

func (approvedSecurePath) VerifyManagedDirectory(string, ...string) (localstate.Evidence, error) {
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (approvedSecurePath) CreateManagedFile(string, ...string) (localstate.Evidence, error) {
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func TestControlledValidationApprovalCheck(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	p := approvedV3Profile(t)
	binding, err := DeriveEligibilityBinding(p)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewControlledValidationApproval(p, binding, "PISA061", "BACLIB", now.Add(-time.Minute), now.Add(time.Minute), now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*ControlledValidationApproval)
		want   ControlledValidationRejection
	}{
		{"valid", func(*ControlledValidationApproval) {}, ControlledValidationApproved},
		{"profile mismatch", func(a *ControlledValidationApproval) { a.Profile = "other" }, ControlledValidationMismatch},
		{"policy mismatch", func(a *ControlledValidationApproval) { a.PolicyID = "other-policy" }, ControlledValidationMismatch},
		{"target mismatch", func(a *ControlledValidationApproval) {
			a.TargetDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}, ControlledValidationMismatch},
		{"source mismatch", func(a *ControlledValidationApproval) { a.Item = "OTHER" }, ControlledValidationMismatch},
		{"future issued", func(a *ControlledValidationApproval) { a.IssuedAt = now.Add(time.Second) }, ControlledValidationInvalid},
		{"expired", func(a *ControlledValidationApproval) {
			a.WindowEnd, a.ExpiresAt = now.Add(-2*time.Second), now.Add(-time.Second)
		}, ControlledValidationExpired},
		{"window not yet valid", func(a *ControlledValidationApproval) { a.WindowStart = now.Add(time.Second) }, ControlledValidationWindowInvalid},
		{"window mismatch", func(a *ControlledValidationApproval) { a.WindowEnd = now.Add(2 * time.Minute) }, ControlledValidationMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := controlledValidationStore(root)
			got := approval
			tc.mutate(&got)
			writeApproval(t, store, p.Name, got)
			if result := store.Check(p, binding, ControlledValidationRequest{Item: "PISA061", Library: "BACLIB", Window: approval.WindowStart.Format(time.RFC3339) + "/" + approval.WindowEnd.Format(time.RFC3339)}, now); result != tc.want {
				t.Fatalf("Check() = %q, want %q", result, tc.want)
			}
		})
	}
}

func TestControlledValidationApprovalRejectsUnsafeOrAbsentRecords(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	p := approvedV3Profile(t)
	binding, err := DeriveEligibilityBinding(p)
	if err != nil {
		t.Fatal(err)
	}
	store := controlledValidationStore(t.TempDir())
	request := ControlledValidationRequest{Item: "PISA061", Library: "BACLIB"}
	if got := store.Check(p, binding, request, now); got != ControlledValidationMissing {
		t.Fatalf("missing record = %q", got)
	}
	if got := store.Check(Profile{Name: "../unsafe"}, binding, request, now); got != ControlledValidationInvalid {
		t.Fatalf("unsafe profile = %q", got)
	}
	unsafeRoot := ControlledValidationApprovalStore{UserConfigDir: func() (string, error) { return "relative", nil }, Platform: approvedSecurePath{}}
	if got := unsafeRoot.Check(p, binding, request, now); got != ControlledValidationInvalid {
		t.Fatalf("unsafe root = %q", got)
	}
}

func TestControlledValidationApprovalRemovalRevokesAccess(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	p := approvedV3Profile(t)
	binding, err := DeriveEligibilityBinding(p)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewControlledValidationApproval(p, binding, "PISA061", "BACLIB", now.Add(-time.Minute), now.Add(time.Minute), now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	store := controlledValidationStore(t.TempDir())
	writeApproval(t, store, p.Name, approval)
	if got := store.Check(p, binding, ControlledValidationRequest{Item: "PISA061", Library: "BACLIB"}, now); got != ControlledValidationApproved {
		t.Fatalf("initial approval = %q", got)
	}
	if err := os.Remove(store.path(p.Name)); err != nil {
		t.Fatal(err)
	}
	if got := store.Check(p, binding, ControlledValidationRequest{Item: "PISA061", Library: "BACLIB"}, now); got != ControlledValidationMissing {
		t.Fatalf("removed approval = %q", got)
	}
}

func TestControlledValidationApprovalRejectsMalformedAndUnknownRecords(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	p := approvedV3Profile(t)
	binding, err := DeriveEligibilityBinding(p)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewControlledValidationApproval(p, binding, "PISA061", "BACLIB", now.Add(-time.Minute), now.Add(time.Minute), now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{[]byte(`{"schemaVersion":1}`), []byte(`{"schemaVersion":1,"unknown":true}`)} {
		store := controlledValidationStore(t.TempDir())
		if len(raw) > 30 {
			writeApproval(t, store, p.Name, approval)
			raw, _ = json.Marshal(map[string]any{"schemaVersion": approval.SchemaVersion, "profile": approval.Profile, "unknown": true})
		}
		path := store.path(p.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := store.Check(p, binding, ControlledValidationRequest{Item: "PISA061", Library: "BACLIB"}, now); got != ControlledValidationUnavailable {
			t.Fatalf("malformed record = %q", got)
		}
	}
}

func controlledValidationStore(configRoot string) ControlledValidationApprovalStore {
	return ControlledValidationApprovalStore{UserConfigDir: func() (string, error) { return configRoot, nil }, Platform: approvedSecurePath{}}
}

func writeApproval(t *testing.T, store ControlledValidationApprovalStore, name string, approval ControlledValidationApproval) {
	t.Helper()
	data, err := json.Marshal(approval)
	if err != nil {
		t.Fatal(err)
	}
	path := store.path(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func approvedV3Profile(t *testing.T) Profile {
	t.Helper()
	p := Profile{SchemaVersion: SchemaVersionV3, Name: "controlled", Host: "ibmi.example", Port: 22, Username: "NEXUS", CredentialMode: CredentialModeKeyring}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	return p
}
