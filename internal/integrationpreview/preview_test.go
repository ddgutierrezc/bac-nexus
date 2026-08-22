package integrationpreview_test

import (
	"strings"
	"testing"

	"bac-nexus/internal/integrationpreview/copilot"
	"bac-nexus/internal/integrationpreview/opencode"
)

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

func TestClientPreviewsRejectUnsupportedVersionsAndNeverWriteFiles(t *testing.T) {
	if _, err := copilot.Build(copilot.Request{Profile: "dev", Version: "v99"}); err == nil {
		t.Fatal("unsupported Copilot version was accepted")
	}
	if _, err := opencode.Build(opencode.Request{Profile: "dev", Version: "v99"}); err == nil {
		t.Fatal("unsupported OpenCode version was accepted")
	}
}
