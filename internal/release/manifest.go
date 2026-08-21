// Package release owns the non-sensitive identity and handoff manifest contract.
package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	SchemaVersion = 1
	ReadyStatus   = "ready_for_controlled_ibmi_validation"
	NotValidated  = "not_validated_on_ibmi"
)

type Identity struct {
	Version  string
	Revision string
}

type Manifest struct {
	SchemaVersion  int    `json:"schema_version"`
	ReleaseVersion string `json:"release_version"`
	VCSRevision    string `json:"vcs_revision"`
	GOOS           string `json:"goos"`
	GOARCH         string `json:"goarch"`
	ByteLength     int64  `json:"byte_length"`
	BinarySHA256   string `json:"binary_sha256"`
	Status         string `json:"status"`
	IBMIStatus     string `json:"ibmi_status"`
}

func NewManifest(identity Identity, goos, goarch string, binary []byte) Manifest {
	digest := sha256.Sum256(binary)
	return Manifest{
		SchemaVersion: SchemaVersion, ReleaseVersion: identity.Version, VCSRevision: identity.Revision,
		GOOS: goos, GOARCH: goarch, ByteLength: int64(len(binary)), BinarySHA256: hex.EncodeToString(digest[:]),
		Status: ReadyStatus, IBMIStatus: NotValidated,
	}
}

func (m Manifest) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func VerifyManifest(m Manifest, binaryPath string, binary []byte, identity Identity) error {
	if m.SchemaVersion != SchemaVersion || m.ReleaseVersion != identity.Version || m.VCSRevision != identity.Revision {
		return errors.New("manifest identity mismatch")
	}
	if strings.TrimSpace(m.GOOS) == "" || strings.TrimSpace(m.GOARCH) == "" || m.ByteLength < 0 || m.BinarySHA256 == "" {
		return errors.New("manifest fields are incomplete")
	}
	if m.Status != ReadyStatus || m.IBMIStatus != NotValidated {
		return errors.New("manifest status is not a non-claim")
	}
	digest := sha256.Sum256(binary)
	if m.ByteLength != int64(len(binary)) || !strings.EqualFold(m.BinarySHA256, hex.EncodeToString(digest[:])) {
		return errors.New("manifest binary checksum mismatch")
	}
	want := filepath.ToSlash(filepath.Join("build", "v1-mcp-foundation", identity.Version, m.GOOS+"-"+m.GOARCH, binaryName(m.GOOS)))
	if filepath.ToSlash(binaryPath) != want {
		return fmt.Errorf("binary path mismatch: want %s", want)
	}
	return nil
}

func binaryName(goos string) string {
	if strings.EqualFold(goos, "windows") {
		return "nexus.exe"
	}
	return "nexus"
}

func VersionJSON(identity Identity) ([]byte, error) {
	return json.Marshal(struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	}{identity.Version, identity.Revision})
}

func ManifestFilename(identity Identity, goos, goarch string) string {
	return filepath.ToSlash(filepath.Join("build", "v1-mcp-foundation", identity.Version, goos+"-"+goarch, "nexus.manifest.json"))
}

func ParseByteLength(value string) (int64, error) { return strconv.ParseInt(value, 10, 64) }
