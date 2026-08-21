package release

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestManifestVerifiesDeterministicBinaryIdentity(t *testing.T) {
	binary := []byte("nexus-binary")
	digest := sha256.Sum256(binary)
	manifest := NewManifest(Identity{Version: "v1.0.0", Revision: "abc123"}, "linux", "amd64", binary)
	if manifest.SchemaVersion != 1 || manifest.ReleaseVersion != "v1.0.0" || manifest.VCSRevision != "abc123" {
		t.Fatalf("manifest identity = %+v", manifest)
	}
	if manifest.ByteLength != int64(len(binary)) || manifest.BinarySHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("manifest binary identity = %+v", manifest)
	}
	if err := VerifyManifest(manifest, "build/v1-mcp-foundation/v1.0.0/linux-amd64/nexus", binary, Identity{Version: "v1.0.0", Revision: "abc123"}); err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}
}

func TestManifestRejectsTamperingMismatchAndUnsafePath(t *testing.T) {
	binary := []byte("nexus-binary")
	identity := Identity{Version: "v1.0.0", Revision: "abc123"}
	manifest := NewManifest(identity, "windows", "amd64", binary)
	cases := []struct {
		name string
		edit func(*Manifest)
		path string
	}{
		{"checksum mismatch", func(m *Manifest) { m.BinarySHA256 = strings.Repeat("0", 64) }, "build/v1-mcp-foundation/v1.0.0/windows-amd64/nexus.exe"},
		{"version mismatch", func(m *Manifest) { m.ReleaseVersion = "v2.0.0" }, "build/v1-mcp-foundation/v1.0.0/windows-amd64/nexus.exe"},
		{"path mismatch", func(m *Manifest) {}, "build/v1-mcp-foundation/v1.0.0/linux-amd64/nexus.exe"},
		{"status claim", func(m *Manifest) { m.Status = "validated_on_ibmi" }, "build/v1-mcp-foundation/v1.0.0/windows-amd64/nexus.exe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := manifest
			tc.edit(&candidate)
			if err := VerifyManifest(candidate, tc.path, binary, identity); err == nil {
				t.Fatal("VerifyManifest() error = nil, want rejection")
			}
		})
	}
}

func TestManifestStatusLanguageIsExplicitNonClaim(t *testing.T) {
	manifest := NewManifest(Identity{Version: "dev", Revision: "unknown"}, "linux", "amd64", []byte("x"))
	if manifest.Status != "ready_for_controlled_ibmi_validation" || manifest.IBMIStatus != "not_validated_on_ibmi" {
		t.Fatalf("status = %q/%q", manifest.Status, manifest.IBMIStatus)
	}
}
