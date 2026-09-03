package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bac-nexus/internal/localstate"
	"bac-nexus/internal/strictjson"
)

const ControlledValidationApprovalSchemaVersion = 1

const maxControlledValidationApprovalBytes = 4 * 1024

var controlledValidationNamePattern = regexp.MustCompile(`^[A-Z0-9#$@_]{1,10}$`)

// ControlledValidationApproval is separate operator authority for one live
// validation attempt. It never grants normal serve eligibility.
type ControlledValidationApproval struct {
	SchemaVersion     int       `json:"schemaVersion"`
	Profile           string    `json:"profile"`
	EligibilityDigest string    `json:"eligibilityDigest"`
	PolicyID          string    `json:"policyID"`
	TargetDigest      string    `json:"targetDigest"`
	WindowStart       time.Time `json:"windowStart"`
	WindowEnd         time.Time `json:"windowEnd"`
	Item              string    `json:"item"`
	Library           string    `json:"library"`
	IssuedAt          time.Time `json:"issuedAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ControlledValidationRequest struct{ Item, Library, Window string }

type ControlledValidationRejection string

const (
	ControlledValidationApproved      ControlledValidationRejection = "approved"
	ControlledValidationMissing       ControlledValidationRejection = "missing"
	ControlledValidationInvalid       ControlledValidationRejection = "invalid"
	ControlledValidationUnavailable   ControlledValidationRejection = "unavailable"
	ControlledValidationMismatch      ControlledValidationRejection = "mismatch"
	ControlledValidationExpired       ControlledValidationRejection = "expired"
	ControlledValidationWindowInvalid ControlledValidationRejection = "window_invalid"
)

// ControlledValidationApprovalStore loads exactly one protected, profile-scoped
// operator record from <UserConfigDir>/BAC Nexus/controlled-validation-approvals.
// Removing that exact record revokes approval immediately and fails closed.
type ControlledValidationApprovalStore struct {
	UserConfigDir func() (string, error)
	Platform      localstate.SecurePathPlatform
}

// Load reads the protected profile-scoped operator record for pre-child checks.
func (s ControlledValidationApprovalStore) Load(name string) (ControlledValidationApproval, ControlledValidationRejection) {
	return s.load(name)
}

func NewControlledValidationApproval(p Profile, binding EligibilityBinding, item, library string, windowStart, windowEnd, issuedAt, expiresAt time.Time) (ControlledValidationApproval, error) {
	a := ControlledValidationApproval{SchemaVersion: ControlledValidationApprovalSchemaVersion, Profile: p.Name, EligibilityDigest: controlledValidationDigest(binding), PolicyID: binding.PolicyID, TargetDigest: binding.TargetDigest, WindowStart: windowStart.UTC(), WindowEnd: windowEnd.UTC(), Item: item, Library: library, IssuedAt: issuedAt.UTC(), ExpiresAt: expiresAt.UTC()}
	if p.Validate() != nil || a.Validate() != nil {
		return ControlledValidationApproval{}, errors.New("controlled validation approval is invalid")
	}
	return a, nil
}

func (a ControlledValidationApproval) Validate() error {
	if a.SchemaVersion != ControlledValidationApprovalSchemaVersion || ValidateName(a.Profile) != nil || !digestReferencePattern.MatchString(a.EligibilityDigest) || !policyIDPattern.MatchString(a.PolicyID) || !digestReferencePattern.MatchString(a.TargetDigest) || !controlledValidationNamePattern.MatchString(a.Item) || !controlledValidationNamePattern.MatchString(a.Library) || a.WindowStart.IsZero() || a.WindowEnd.IsZero() || a.IssuedAt.IsZero() || a.ExpiresAt.IsZero() || !a.WindowEnd.After(a.WindowStart) || !a.ExpiresAt.After(a.IssuedAt) || a.ExpiresAt.Before(a.WindowEnd) {
		return errors.New("controlled validation approval is invalid")
	}
	return nil
}

func (s ControlledValidationApprovalStore) Check(p Profile, binding EligibilityBinding, request ControlledValidationRequest, now time.Time) ControlledValidationRejection {
	if p.Validate() != nil || ValidateName(p.Name) != nil || !controlledValidationNamePattern.MatchString(request.Item) || !controlledValidationNamePattern.MatchString(request.Library) {
		return ControlledValidationInvalid
	}
	a, result := s.load(p.Name)
	if result != ControlledValidationApproved {
		return result
	}
	if a.IssuedAt.After(now.UTC()) {
		return ControlledValidationInvalid
	}
	if !a.ExpiresAt.After(now.UTC()) {
		return ControlledValidationExpired
	}
	if now.UTC().Before(a.WindowStart) || !now.UTC().Before(a.WindowEnd) {
		return ControlledValidationWindowInvalid
	}
	if request.Window != "" && request.Window != a.WindowStart.Format(time.RFC3339)+"/"+a.WindowEnd.Format(time.RFC3339) {
		return ControlledValidationMismatch
	}
	if a.Profile != p.Name || a.EligibilityDigest != controlledValidationDigest(binding) || a.PolicyID != binding.PolicyID || a.TargetDigest != binding.TargetDigest || a.Item != request.Item || a.Library != request.Library {
		return ControlledValidationMismatch
	}
	return ControlledValidationApproved
}

func (s ControlledValidationApprovalStore) load(name string) (ControlledValidationApproval, ControlledValidationRejection) {
	path, platform, err := s.pathAndPlatform(name)
	if err != nil {
		return ControlledValidationApproval{}, ControlledValidationInvalid
	}
	if _, err := platform.VerifyManagedDirectory(filepath.Dir(path), "BAC Nexus", "controlled-validation-approvals"); err != nil {
		return ControlledValidationApproval{}, ControlledValidationUnavailable
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ControlledValidationApproval{}, ControlledValidationMissing
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return ControlledValidationApproval{}, ControlledValidationUnavailable
	}
	data, err := readControlledValidationApproval(path)
	if err != nil || strictjson.ValidateObjectKeys(data, "schemaVersion", "profile", "eligibilityDigest", "policyID", "targetDigest", "windowStart", "windowEnd", "item", "library", "issuedAt", "expiresAt") != nil {
		return ControlledValidationApproval{}, ControlledValidationUnavailable
	}
	var approval ControlledValidationApproval
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&approval) != nil || approval.Validate() != nil {
		return ControlledValidationApproval{}, ControlledValidationUnavailable
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return ControlledValidationApproval{}, ControlledValidationUnavailable
	}
	return approval, ControlledValidationApproved
}

func (s ControlledValidationApprovalStore) path(name string) string {
	root, _ := s.configRoot()
	return filepath.Join(root, "BAC Nexus", "controlled-validation-approvals", name+".json")
}

func (s ControlledValidationApprovalStore) pathAndPlatform(name string) (string, localstate.SecurePathPlatform, error) {
	if ValidateName(name) != nil {
		return "", nil, errors.New("invalid approval profile")
	}
	root, err := s.configRoot()
	if err != nil {
		return "", nil, err
	}
	platform := s.Platform
	if platform == nil {
		userConfigDir := s.UserConfigDir
		if userConfigDir == nil {
			userConfigDir = os.UserConfigDir
		}
		platform = localstate.NewPlatform(userConfigDir)
	}
	return filepath.Join(root, "BAC Nexus", "controlled-validation-approvals", name+".json"), platform, nil
}

func (s ControlledValidationApprovalStore) configRoot() (string, error) {
	userConfigDir := s.UserConfigDir
	if userConfigDir == nil {
		userConfigDir = os.UserConfigDir
	}
	root, err := userConfigDir()
	if err != nil || !filepath.IsAbs(root) {
		return "", errors.New("controlled validation configuration root is unavailable")
	}
	return root, nil
}

func controlledValidationDigest(binding EligibilityBinding) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{binding.TargetDigest, binding.PolicyID, binding.PinDigest, binding.CredentialRef, binding.ArtifactRef, binding.ProofDigest}, "\x00")))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func readControlledValidationApproval(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxControlledValidationApprovalBytes+1))
	if err != nil || len(data) > maxControlledValidationApprovalBytes {
		return nil, errors.New("controlled validation approval is unavailable")
	}
	return data, nil
}
