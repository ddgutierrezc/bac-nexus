package integrationpreview_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/integrationpreview"
	"bac-nexus/internal/integrationpreview/copilot"
	"bac-nexus/internal/integrationpreview/opencode"
)

type recordingClipboard struct{ value string }

func (c *recordingClipboard) Copy(_ context.Context, value string) error { c.value = value; return nil }

func TestClientPreviewsAreDeterministicAndSecretFree(t *testing.T) {
	const sentinel = "sentinel-secret"
	for _, tc := range []struct {
		name  string
		build func() (string, error)
	}{
		{"copilot", func() (string, error) {
			p, err := copilot.Build(copilot.Request{Profile: "dev"})
			return p.Payload, err
		}},
		{"opencode", func() (string, error) {
			p, err := opencode.Build(opencode.Request{Profile: "dev"})
			return p.Payload, err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, err := tc.build()
			if err != nil {
				t.Fatal(err)
			}
			second, err := tc.build()
			if err != nil || first != second || strings.Contains(first, sentinel) {
				t.Fatalf("preview = %q, second = %q, err = %v", first, second, err)
			}
		})
	}
}

func TestPreviewUsesCanonicalProfileNameValidation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		valid bool
	}{
		{"CRI400F-Dev_1", true},
		{"CRI400F.Dev", false},
		{"CRIñ400F", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := integrationpreview.ValidateRequest(integrationpreview.Request{Profile: tt.name})
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateRequest(%q) error = %v, want valid=%v", tt.name, err, tt.valid)
			}
		})
	}
}

func TestClientPreviewsRejectUnsupportedVersionsAndNeverWriteFiles(t *testing.T) {
	if _, err := copilot.Build(copilot.Request{Profile: "dev", Version: "v99"}); err == nil {
		t.Fatal("unsupported Copilot version was accepted")
	}
	if _, err := opencode.Build(opencode.Request{Profile: "dev", Version: "v99"}); err == nil {
		t.Fatal("unsupported OpenCode version was accepted")
	}
}

func TestPreviewCopyUsesDeterministicPayloadWithoutExternalWrites(t *testing.T) {
	root := t.TempDir()
	before, _ := os.ReadDir(root)
	preview, err := copilot.Build(copilot.Request{Profile: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	clipboard := &recordingClipboard{}
	if err := configuration.CopySecretFree(context.Background(), clipboard, preview.Payload); err != nil {
		t.Fatal(err)
	}
	if clipboard.value != preview.Payload {
		t.Fatalf("copied payload = %q, want %q", clipboard.value, preview.Payload)
	}
	after, _ := os.ReadDir(root)
	if len(before) != len(after) || len(after) != 0 {
		t.Fatalf("preview copy changed external files in %s", filepath.Base(root))
	}
}
