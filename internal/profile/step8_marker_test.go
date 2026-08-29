package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStep8MarkerStoreBoundsAndInvalidates(t *testing.T) {
	p := step8MarkerProfile(t)
	s := Step8MarkerStore{Profiles: Store{Root: t.TempDir()}}
	m := Step8Marker{SchemaVersion: Step8MarkerSchemaVersion, AtUnixMs: time.Now().UnixMilli(), Outcome: Step8MarkerProofSuccess, ProofRevision: Step8MarkerProofRevision}
	if err := s.Write(context.Background(), p, m); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Profiles.Root, p.Name+".step8.json")
	b, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(b), p.Host) || strings.Contains(string(b), p.Username) {
		t.Fatalf("marker=%q err=%v", b, err)
	}
	if err := s.Clear(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker remained: %v", err)
	}
}

func TestStep8MarkerStoreRejectsInvalidAndCancelledWrites(t *testing.T) {
	p := step8MarkerProfile(t)
	s := Step8MarkerStore{Profiles: Store{Root: t.TempDir()}}
	bad := Step8Marker{SchemaVersion: Step8MarkerSchemaVersion, AtUnixMs: 1, Outcome: "failed", ProofRevision: "secret"}
	if err := s.Write(context.Background(), p, bad); !errors.Is(err, ErrStep8MarkerRejected) {
		t.Fatalf("invalid marker = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	good := Step8Marker{SchemaVersion: Step8MarkerSchemaVersion, AtUnixMs: time.Now().UnixMilli(), Outcome: Step8MarkerProofSuccess, ProofRevision: Step8MarkerProofRevision}
	if err := s.Write(ctx, p, good); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled marker = %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Profiles.Root, p.Name+".step8.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled write persisted: %v", err)
	}
}

func step8MarkerProfile(t *testing.T) Profile {
	t.Helper()
	return Profile{SchemaVersion: SchemaVersionV3, Name: "step8", Host: "ibmi.example", Port: 22, Username: "user", CredentialMode: CredentialModePrompt}
}
