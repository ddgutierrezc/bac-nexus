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

const (
	CredentialModeVault  CredentialMode = "vault"
	CredentialModePrompt CredentialMode = "prompt"
	HostKeyTrustTOFU     HostKeyTrust   = "tofu"
	HostKeyTrustVerified HostKeyTrust   = "verified"
)

var (
	namePattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	userPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._$#@-]{0,127}$`)
	fingerprintPattern = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)
	javaHomePattern    = regexp.MustCompile(`^/QOpenSys/QIBM/ProdData/JavaVM/[A-Za-z0-9._/-]+$`)
)

type Profile struct {
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
}

func (p Profile) Validate() error {
	if !namePattern.MatchString(p.Name) {
		return errors.New("profile name must use 1-64 letters, digits, dot, underscore, or hyphen")
	}
	if err := ValidateEndpoint(p.Host, p.Port); err != nil {
		return err
	}
	if !userPattern.MatchString(p.Username) {
		return errors.New("username contains unsupported characters")
	}
	if err := ValidateHostKey(p.HostKeyFingerprint, p.HostKeyTrust); err != nil {
		return err
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
	if err := validateHost(host); err != nil {
		return err
	}
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func validateHost(host string) error {
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
	if err := strictjson.ValidateObjectKeys(data, "name", "host", "port", "username", "hostKeyFingerprint", "hostKeyTrust", "hostKeyProvenance", "javaHome", "mapepireJar", "credentialMode"); err != nil {
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
