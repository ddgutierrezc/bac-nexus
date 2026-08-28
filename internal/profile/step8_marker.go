package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	Step8MarkerSchemaVersion = 1
	Step8MarkerProofSuccess  = "proof_success"
	Step8MarkerProofRevision = "values-1-v1"
)

var (
	ErrStep8MarkerRejected    = errors.New("step8 marker rejected")
	ErrStep8MarkerUnavailable = errors.New("step8 marker unavailable")
)

// Step8Marker is historical evidence only; it is never readiness state.
type Step8Marker struct {
	SchemaVersion int    `json:"schemaVersion"`
	AtUnixMs      int64  `json:"atUnixMs"`
	Outcome       string `json:"outcome"`
	ProofRevision string `json:"proofRevision"`
}

// Step8MarkerStore persists proof history beside a validated saved profile.
type Step8MarkerStore struct{ Profiles Store }

func (s Step8MarkerStore) Clear(ctx context.Context, p Profile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validStep8MarkerProfile(p) {
		return ErrStep8MarkerRejected
	}
	if err := s.Profiles.verifyRoot(); err != nil {
		return ErrStep8MarkerUnavailable
	}
	err := os.Remove(s.path(p))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return ErrStep8MarkerUnavailable
}

func (s Step8MarkerStore) Write(ctx context.Context, p Profile, m Step8Marker) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validStep8MarkerProfile(p) || !validStep8Marker(m) {
		return ErrStep8MarkerRejected
	}
	if err := s.Profiles.verifyRoot(); err != nil {
		return ErrStep8MarkerUnavailable
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ErrStep8MarkerRejected
	}
	temp, err := s.Profiles.writeTemp(append(b, '\n'))
	if err != nil {
		return ErrStep8MarkerUnavailable
	}
	path := s.path(p)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		err = os.Rename(temp, path)
	} else if err == nil {
		err = s.Profiles.atomicReplace(temp, path)
	}
	if err != nil {
		_ = os.Remove(temp)
		return ErrStep8MarkerUnavailable
	}
	return nil
}

func (s Step8MarkerStore) path(p Profile) string {
	return filepath.Join(s.Profiles.Root, p.Name+".step8.json")
}
func validStep8MarkerProfile(p Profile) bool {
	return p.SchemaVersion == SchemaVersionV3 && p.Validate() == nil
}
func validStep8Marker(m Step8Marker) bool {
	return m.SchemaVersion == Step8MarkerSchemaVersion && m.AtUnixMs > 0 && m.AtUnixMs <= time.Now().Add(5*time.Minute).UnixMilli() && m.Outcome == Step8MarkerProofSuccess && m.ProofRevision == Step8MarkerProofRevision
}
