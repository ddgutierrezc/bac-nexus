package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bac-nexus/internal/strictjson"
)

const appDirectory = "BAC Nexus"
const maxProfileBytes = 16 * 1024
const MaxListLimit = 128

type CredentialMode string
type HostKeyTrust string
type TrustMode string

const SchemaVersionV2 = 2

const (
	CredentialModeVault  CredentialMode = "vault"
	CredentialModePrompt CredentialMode = "prompt"
	HostKeyTrustTOFU     HostKeyTrust   = "tofu"
	HostKeyTrustVerified HostKeyTrust   = "verified"
	TrustModeCA          TrustMode      = "ca"
	TrustModePin         TrustMode      = "pin"
	TrustModeTOFU        TrustMode      = "tofu"
)

// TrustEvidence is transport-specific approved identity evidence. Pin is
// either a TLS leaf digest or an OpenSSH fingerprint; the two formats are
// deliberately validated by their owning transport mode.
type TrustEvidence struct {
	Mode       TrustMode `json:"mode"`
	Pin        string    `json:"pin,omitempty"`
	Provenance string    `json:"provenance,omitempty"`
}

var (
	namePattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	userPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._$#@-]{0,127}$`)
	fingerprintPattern = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)
	javaHomePattern    = regexp.MustCompile(`^/QOpenSys/QIBM/ProdData/JavaVM/[A-Za-z0-9._/-]+$`)
)

var profileV1Keys = []string{"name", "host", "port", "username", "hostKeyFingerprint", "hostKeyTrust", "hostKeyProvenance", "javaHome", "mapepireJar", "credentialMode"}
var profileV2Keys = append(append([]string{}, profileV1Keys...), "schemaVersion", "policyRef", "fallbackAllowed", "tlsTrust", "sshTrust")

type Profile struct {
	SchemaVersion      int            `json:"schemaVersion,omitempty"`
	Name               string         `json:"name"`
	Host               string         `json:"host"`
	Port               int            `json:"port"`
	Username           string         `json:"username"`
	HostKeyFingerprint string         `json:"hostKeyFingerprint"`
	HostKeyTrust       HostKeyTrust   `json:"hostKeyTrust"`
	HostKeyProvenance  string         `json:"hostKeyProvenance,omitempty"`
	JavaHome           string         `json:"javaHome,omitempty"`
	MapepireJAR        string         `json:"mapepireJar,omitempty"`
	CredentialMode     CredentialMode `json:"credentialMode"`
	EndpointPolicyRef  string         `json:"policyRef,omitempty"`
	FallbackAllowed    bool           `json:"fallbackAllowed"`
	TLSTrust           TrustEvidence  `json:"tlsTrust,omitempty"`
	SSHTrust           TrustEvidence  `json:"sshTrust,omitempty"`
}

// MarshalJSON keeps legacy files byte-compatible while ensuring empty v2
// trust objects are not serialized as misleading evidence.
func (p Profile) MarshalJSON() ([]byte, error) {
	type plain Profile
	data, err := json.Marshal(plain(p))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if p.SchemaVersion == 0 {
		delete(fields, "schemaVersion")
	}
	if p.EndpointPolicyRef == "" {
		delete(fields, "policyRef")
	}
	if !p.FallbackAllowed && p.SchemaVersion == 0 {
		delete(fields, "fallbackAllowed")
	}
	if p.SchemaVersion == SchemaVersionV2 {
		if p.HostKeyFingerprint == "" {
			delete(fields, "hostKeyFingerprint")
		}
		if p.HostKeyTrust == "" {
			delete(fields, "hostKeyTrust")
		}
		if p.HostKeyProvenance == "" {
			delete(fields, "hostKeyProvenance")
		}
		if p.JavaHome == "" {
			delete(fields, "javaHome")
		}
		if p.MapepireJAR == "" {
			delete(fields, "mapepireJar")
		}
	}
	if p.TLSTrust == (TrustEvidence{}) {
		delete(fields, "tlsTrust")
	}
	if p.SSHTrust == (TrustEvidence{}) {
		delete(fields, "sshTrust")
	}
	return json.Marshal(fields)
}

func (p Profile) Validate() error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	if err := ValidateEndpoint(p.Host, p.Port); err != nil {
		return err
	}
	if err := ValidateUsername(p.Username); err != nil {
		return err
	}
	if p.SchemaVersion == 0 {
		if err := ValidateHostKey(p.HostKeyFingerprint, p.HostKeyTrust); err != nil {
			return err
		}
	} else if p.SchemaVersion != SchemaVersionV2 {
		return errors.New("unsupported profile schema version")
	}
	if p.JavaHome != "" && (!javaHomePattern.MatchString(p.JavaHome) || strings.Contains(p.JavaHome, "..")) {
		return errors.New("Java home must be an absolute IBM i JavaVM path")
	}
	if p.MapepireJAR != "" && (len(p.MapepireJAR) > 4096 || !filepath.IsAbs(p.MapepireJAR) || strings.ContainsAny(p.MapepireJAR, "\x00\r\n")) {
		return errors.New("Mapepire JAR path must be an absolute local path")
	}
	if p.CredentialMode != CredentialModeVault && p.CredentialMode != CredentialModePrompt {
		return errors.New("credential mode must be vault or prompt")
	}
	if len(p.HostKeyProvenance) > 128 || strings.ContainsAny(p.HostKeyProvenance, "\x00\r\n") {
		return errors.New("host-key provenance is invalid")
	}
	if p.SchemaVersion == SchemaVersionV2 {
		if p.EndpointPolicyRef == "" || len(p.EndpointPolicyRef) > 128 || strings.ContainsAny(p.EndpointPolicyRef, "\x00\r\n") {
			return errors.New("profile policy reference is invalid")
		}
		if err := validateTrustEvidence(p.TLSTrust, true); err != nil {
			return err
		}
		if err := validateTrustEvidence(p.SSHTrust, false); err != nil {
			return err
		}
	}
	return nil
}

func validateTrustEvidence(e TrustEvidence, tls bool) error {
	if e == (TrustEvidence{}) {
		return nil
	}
	if e.Mode != TrustModeCA && e.Mode != TrustModePin && e.Mode != TrustModeTOFU {
		return errors.New("trust mode is invalid")
	}
	if len(e.Provenance) > 128 || strings.ContainsAny(e.Provenance, "\x00\r\n") {
		return errors.New("trust provenance is invalid")
	}
	if e.Mode == TrustModeCA {
		if !tls {
			return errors.New("SSH trust cannot use CA mode")
		}
		if e.Pin != "" {
			return errors.New("CA trust cannot contain a pin")
		}
		return nil
	}
	if e.Pin == "" {
		return errors.New("pinned trust evidence is missing")
	}
	if tls {
		if !strings.HasPrefix(e.Pin, "sha256/") || len(e.Pin) != len("sha256/")+43 {
			return errors.New("TLS trust pin is invalid")
		}
	} else if err := ValidateHostKey(e.Pin, HostKeyTrustVerified); err != nil {
		return errors.New("SSH trust pin is invalid")
	}
	if e.Provenance == "" {
		return errors.New("trust provenance is missing")
	}
	return nil
}

// MigrateV1 validates a legacy profile and produces a schema-v2 profile with
// no automatically trusted transport identity or fallback permission.
func MigrateV1(data []byte) (Profile, error) {
	if err := strictjson.ValidateObjectKeys(data, "name", "host", "port", "username", "hostKeyFingerprint", "hostKeyTrust", "hostKeyProvenance", "javaHome", "mapepireJar", "credentialMode"); err != nil {
		return Profile{}, fmt.Errorf("migrate profile: %w", err)
	}
	var p Profile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("migrate profile: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Profile{}, errors.New("migrate profile: trailing data")
	}
	if p.SchemaVersion != 0 || p.Validate() != nil {
		return Profile{}, errors.New("migrate profile: invalid v1 profile")
	}
	p.SchemaVersion, p.EndpointPolicyRef = SchemaVersionV2, "legacy-migrated"
	p.FallbackAllowed, p.TLSTrust, p.SSHTrust = false, TrustEvidence{}, TrustEvidence{}
	return p, nil
}

// ValidateName applies the stable profile-name contract independently from
// the remaining connection fields. Wizard steps use it before a full profile
// exists, so the same syntax is enforced at every boundary.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return errors.New("profile name must use 1-64 ASCII letters or digits, then ASCII letters, digits, hyphen, or underscore")
	}
	return nil
}

func ValidateHostKey(fingerprintValue string, trust HostKeyTrust) error {
	if !fingerprintPattern.MatchString(fingerprintValue) {
		return errors.New("host-key fingerprint must be an OpenSSH SHA256 fingerprint")
	}
	fingerprint := strings.TrimPrefix(fingerprintValue, "SHA256:")
	decodedFingerprint, err := base64.RawStdEncoding.Strict().DecodeString(fingerprint)
	if err != nil || len(decodedFingerprint) != sha256.Size || base64.RawStdEncoding.EncodeToString(decodedFingerprint) != fingerprint {
		return errors.New("host-key fingerprint must use canonical unpadded base64")
	}
	if trust != HostKeyTrustTOFU && trust != HostKeyTrustVerified {
		return errors.New("host-key trust must be tofu or verified")
	}
	return nil
}

func ValidateEndpoint(host string, port int) error {
	if err := ValidateHost(host); err != nil {
		return err
	}
	return ValidatePort(port)
}

// ValidateHost applies the existing DNS/IPv4 host contract without accepting a
// port, whitespace, or IPv6 literals.
func ValidateHost(host string) error {
	if host == "" || strings.TrimSpace(host) != host || strings.ContainsAny(host, "/\\:@[]\t\r\n ") {
		return errors.New("host must be a DNS name or IP address without a port")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if len(host) > 253 {
		return errors.New("host is too long")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return errors.New("host is not a valid DNS name")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' {
				return errors.New("host is not a valid DNS name")
			}
		}
	}
	return nil
}

// ValidateUsername applies the stable IBM i username character contract.
func ValidateUsername(username string) error {
	if !userPattern.MatchString(username) {
		return errors.New("username contains unsupported characters")
	}
	return nil
}

// ValidatePort applies the existing SSH port range contract.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

type Store struct {
	Root    string
	replace func(string, string) error
}

func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, appDirectory, "profiles"), nil
}

func (s Store) Save(p Profile) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if s.Root == "" {
		return "", errors.New("profile store root is required")
	}
	if info, err := os.Lstat(s.Root); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	} else if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsDir() {
		return "", ErrInvalidRoot
	}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(s.Root, p.Name+".json")
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(s.Root, ".profile-*.tmp")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o600); err != nil {
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		return "", err
	}
	if err := temp.Sync(); err != nil {
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Link(tempPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("profile %q already exists: %w", p.Name, os.ErrExist)
		}
		return "", err
	}
	return path, nil
}

func (s Store) Load(name string) (Profile, error) {
	if !namePattern.MatchString(name) {
		return Profile{}, errors.New("invalid profile name")
	}
	file, err := os.Open(filepath.Join(s.Root, name+".json"))
	if err != nil {
		return Profile{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxProfileBytes+1))
	if err != nil {
		return Profile{}, err
	}
	if len(data) > maxProfileBytes {
		return Profile{}, errors.New("profile exceeds byte limit")
	}
	keys := profileV1Keys
	var header struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	if header.SchemaVersion != 0 {
		keys = profileV2Keys
	}
	if err := strictjson.ValidateObjectKeys(data, keys...); err != nil {
		return Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	var p Profile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Profile{}, errors.New("decode profile: trailing JSON value")
		}
		return Profile{}, fmt.Errorf("decode profile: trailing data: %w", err)
	}
	if p.Name != name {
		return Profile{}, errors.New("profile name does not match file name")
	}
	if err := p.Validate(); err != nil {
		return Profile{}, fmt.Errorf("invalid stored profile: %w", err)
	}
	return p, nil
}
