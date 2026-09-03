package profile

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/localstate"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/strictjson"
)

const EligibilitySchemaVersionV1 = 1

const maxEligibilityBytes = 4 * 1024

const EligibilityLifetime = 30 * 24 * time.Hour

const eligibilityPolicyID = "verified-readonly"

var (
	ErrEligibilityMissing     = errors.New("serving eligibility missing")
	ErrEligibilityInvalid     = errors.New("serving eligibility invalid")
	ErrEligibilityUnavailable = errors.New("serving eligibility unavailable")
	digestReferencePattern    = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	policyIDPattern           = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
)

// Eligibility contains no network target, user, path, credential, or proof
// material. It only retains canonical references proving one approved binding.
type Eligibility struct {
	SchemaVersion int       `json:"schemaVersion"`
	Profile       string    `json:"profile"`
	TargetDigest  string    `json:"targetDigest"`
	PolicyID      string    `json:"policyID"`
	PinDigest     string    `json:"pinDigest"`
	CredentialRef string    `json:"credentialRef"`
	ArtifactRef   string    `json:"artifactRef"`
	ProofDigest   string    `json:"proofDigest"`
	ApprovedAt    time.Time `json:"approvedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// EligibilityBinding is the caller-provided, non-secret identity expected for
// a serving attempt. It prevents a valid proof from being reused for another
// target, policy, pin, credential, artifact, or proof result.
type EligibilityBinding struct {
	TargetDigest, PolicyID, PinDigest, CredentialRef, ArtifactRef, ProofDigest string
}

// DeriveEligibilityBinding reconstructs the serving identity solely from
// controlled V3 profile and release constants; it never receives a secret.
func DeriveEligibilityBinding(p Profile) (EligibilityBinding, error) {
	if p.SchemaVersion != SchemaVersionV3 || p.CredentialMode != CredentialModeKeyring || p.Validate() != nil {
		return EligibilityBinding{}, ErrEligibilityInvalid
	}
	return EligibilityBinding{
		TargetDigest:  eligibilityReference("target/v1", strings.ToLower(p.Host), strconv.Itoa(p.Port), p.Username),
		PolicyID:      eligibilityPolicyID,
		PinDigest:     eligibilityReference("pin/v1", p.HostKeyFingerprint, string(p.HostKeyTrust)),
		CredentialRef: "keyring:" + eligibilityReference("credential/v1", "BAC Nexus", "ibmi/"+p.Name),
		ArtifactRef:   eligibilityReference("artifact/v1", mapepirestdio.ServerVersion, mapepirestdio.ServerSHA256, mapepirestdio.RemoteJar, mapepirestdio.ArtifactPolicyRevision()),
		ProofDigest:   eligibilityReference("proof/v1", mapepire.FixedProofRevision),
	}, nil
}

// NewEligibility issues one bounded proof-derived eligibility record.
func NewEligibility(p Profile, approvedAt time.Time) (Eligibility, error) {
	binding, err := DeriveEligibilityBinding(p)
	if err != nil || approvedAt.IsZero() {
		return Eligibility{}, ErrEligibilityInvalid
	}
	return Eligibility{SchemaVersion: EligibilitySchemaVersionV1, Profile: p.Name, TargetDigest: binding.TargetDigest, PolicyID: binding.PolicyID, PinDigest: binding.PinDigest, CredentialRef: binding.CredentialRef, ArtifactRef: binding.ArtifactRef, ProofDigest: binding.ProofDigest, ApprovedAt: approvedAt.UTC(), ExpiresAt: approvedAt.UTC().Add(EligibilityLifetime)}, nil
}

func eligibilityReference(domain string, fields ...string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("BAC Nexus/eligibility/" + domain + "\x00"))
	for _, field := range fields {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func (e Eligibility) Binding() EligibilityBinding {
	return EligibilityBinding{e.TargetDigest, e.PolicyID, e.PinDigest, e.CredentialRef, e.ArtifactRef, e.ProofDigest}
}

func (e Eligibility) Validate() error {
	if e.SchemaVersion != EligibilitySchemaVersionV1 || ValidateName(e.Profile) != nil ||
		!digestReferencePattern.MatchString(e.TargetDigest) || !policyIDPattern.MatchString(e.PolicyID) ||
		!digestReferencePattern.MatchString(e.PinDigest) || !validCredentialRef(e.CredentialRef) ||
		!digestReferencePattern.MatchString(e.ArtifactRef) || !digestReferencePattern.MatchString(e.ProofDigest) ||
		e.ApprovedAt.IsZero() || e.ExpiresAt.IsZero() || !e.ExpiresAt.After(e.ApprovedAt) {
		return ErrEligibilityInvalid
	}
	return nil
}

func validCredentialRef(reference string) bool {
	return strings.HasPrefix(reference, "keyring:") && digestReferencePattern.MatchString(strings.TrimPrefix(reference, "keyring:"))
}

type EligibilityRejection string

const (
	EligibilityApproved           EligibilityRejection = "approved"
	EligibilityMissing            EligibilityRejection = "missing"
	EligibilityStale              EligibilityRejection = "stale"
	EligibilityMismatch           EligibilityRejection = "mismatch"
	EligibilityKeyringUnavailable EligibilityRejection = "keyring_unavailable"
	EligibilityLegacyProfile      EligibilityRejection = "legacy_profile"
	EligibilityUnavailable        EligibilityRejection = "unavailable"
)

// EligibilityStore persists owner-only serving proof separately from profiles;
// existing profiles therefore remain ineligible until an explicit approval writes
// this record.
type EligibilityStore struct {
	Root          string
	UserConfigDir func() (string, error)
	Platform      localstate.SecurePathPlatform
	replace       func(string, string) error
}

func (s EligibilityStore) Save(eligibility Eligibility) error {
	root, platform, err := s.rootAndPlatform()
	if eligibility.Validate() != nil || err != nil || s.verifyRoot(root, platform) != nil {
		return ErrEligibilityInvalid
	}
	data, err := json.Marshal(eligibility)
	if err != nil || len(data) > maxEligibilityBytes {
		return ErrEligibilityInvalid
	}
	temporary, err := s.writeTemp(root, platform, data)
	if err != nil {
		return ErrEligibilityUnavailable
	}
	live := filepath.Join(root, eligibility.Profile+".eligibility.json")
	if err := s.replaceFile(temporary, live); err != nil {
		_ = os.Remove(temporary)
		return ErrEligibilityUnavailable
	}
	if err := syncEligibilityDirectory(root); err != nil {
		return ErrEligibilityUnavailable
	}
	confirmed, err := s.Load(eligibility.Profile)
	if err != nil || confirmed != eligibility {
		return ErrEligibilityUnavailable
	}
	return nil
}

func (s EligibilityStore) Load(name string) (Eligibility, error) {
	root, platform, err := s.rootAndPlatform()
	if ValidateName(name) != nil || err != nil || s.verifyExistingRoot(root, platform) != nil {
		return Eligibility{}, ErrEligibilityMissing
	}
	path := filepath.Join(root, name+".eligibility.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Eligibility{}, ErrEligibilityMissing
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	if _, err := platform.CreateManagedFile(path, "BAC Nexus", "profiles", filepath.Base(path)); err != nil {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	data, err := readEligibility(path)
	if err != nil || strictjson.ValidateObjectKeys(data, "schemaVersion", "profile", "targetDigest", "policyID", "pinDigest", "credentialRef", "artifactRef", "proofDigest", "approvedAt", "expiresAt") != nil {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	var eligibility Eligibility
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&eligibility) != nil || eligibility.Profile != name || eligibility.Validate() != nil {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	return eligibility, nil
}

func (s EligibilityStore) Check(profile Profile, binding EligibilityBinding, keyringAvailable bool, now time.Time) EligibilityRejection {
	if profile.SchemaVersion != SchemaVersionV3 || profile.CredentialMode != CredentialModeKeyring || profile.Validate() != nil {
		return EligibilityLegacyProfile
	}
	if !keyringAvailable {
		return EligibilityKeyringUnavailable
	}
	eligibility, err := s.Load(profile.Name)
	if errors.Is(err, ErrEligibilityMissing) {
		return EligibilityMissing
	}
	if err != nil {
		return EligibilityUnavailable
	}
	if !eligibility.ExpiresAt.After(now) {
		return EligibilityStale
	}
	if eligibility.Binding() != binding {
		return EligibilityMismatch
	}
	return EligibilityApproved
}

func (s EligibilityStore) Revoke(name string) error {
	root, platform, err := s.rootAndPlatform()
	if ValidateName(name) != nil || err != nil || s.verifyExistingRoot(root, platform) != nil {
		return ErrEligibilityMissing
	}
	path := filepath.Join(root, name+".eligibility.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrEligibilityMissing
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		return ErrEligibilityUnavailable
	}
	if _, err := platform.CreateManagedFile(path, "BAC Nexus", "profiles", filepath.Base(path)); err != nil {
		return ErrEligibilityUnavailable
	}
	if err := os.Remove(path); err != nil {
		return ErrEligibilityUnavailable
	}
	if err := syncEligibilityDirectory(root); err != nil {
		return ErrEligibilityUnavailable
	}
	return nil
}

func (s EligibilityStore) rootAndPlatform() (string, localstate.SecurePathPlatform, error) {
	userConfigDir := s.UserConfigDir
	if userConfigDir == nil {
		userConfigDir = os.UserConfigDir
	}
	configRoot, err := userConfigDir()
	if err != nil || !filepath.IsAbs(configRoot) {
		return "", nil, ErrEligibilityInvalid
	}
	root := filepath.Join(configRoot, "BAC Nexus", "profiles")
	if s.Root != "" && filepath.Clean(s.Root) != filepath.Clean(root) {
		return "", nil, ErrEligibilityInvalid
	}
	platform := s.Platform
	if platform == nil {
		platform = localstate.NewPlatform(userConfigDir)
	}
	return root, platform, nil
}

func (s EligibilityStore) verifyRoot(root string, platform localstate.SecurePathPlatform) error {
	if platform == nil {
		return ErrEligibilityInvalid
	}
	if _, err := platform.VerifyManagedDirectory(root, "BAC Nexus", "profiles"); err != nil {
		return ErrEligibilityUnavailable
	}
	return nil
}

func (s EligibilityStore) verifyExistingRoot(root string, platform localstate.SecurePathPlatform) error {
	return s.verifyRoot(root, platform)
}

func (s EligibilityStore) writeTemp(root string, platform localstate.SecurePathPlatform, data []byte) (string, error) {
	temporary, err := os.CreateTemp(root, ".eligibility-*.tmp")
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	cleanup := func() { _ = temporary.Close(); _ = os.Remove(path) }
	if _, err := platform.CreateManagedFile(path, "BAC Nexus", "profiles", filepath.Base(path)); err != nil {
		cleanup()
		return "", ErrEligibilityUnavailable
	}
	if temporary.Chmod(0o600) != nil {
		cleanup()
		return "", ErrEligibilityUnavailable
	}
	if _, err := temporary.Write(data); err != nil {
		cleanup()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (s EligibilityStore) replaceFile(source, destination string) error {
	if s.replace != nil {
		return s.replace(source, destination)
	}
	return os.Rename(source, destination)
}

func readEligibility(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxEligibilityBytes+1))
	if err != nil || len(data) > maxEligibilityBytes {
		return nil, ErrEligibilityUnavailable
	}
	return data, nil
}

func syncEligibilityDirectory(root string) error {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
