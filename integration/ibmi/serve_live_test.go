//go:build ibmi_integration

package ibmi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/credential"
	nexusmcp "bac-nexus/internal/mcp"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/release"
)

const controlledGateOptIn = "approved-operator-window"

func TestControlledChildEnvironmentExcludesUnrelatedSecrets(t *testing.T) {
	t.Setenv("BAC_NEXUS_UNRELATED_SECRET", "must-not-reach-child")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "must-not-reach-child")
	t.Setenv("SSH_AUTH_SOCK", "/private/agent.sock")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1000/bus")

	environment := controlledChildEnvironment(gateConfig{configRoot: "/approved/config"})
	got := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("child environment entry %q is invalid", entry)
		}
		got[key] = value
	}

	if got["XDG_CONFIG_HOME"] != "/approved/config" || got["XDG_RUNTIME_DIR"] != "/run/user/1000" || got["DBUS_SESSION_BUS_ADDRESS"] != "unix:path=/run/user/1000/bus" {
		t.Fatalf("child environment did not retain required local configuration/keyring access: %#v", got)
	}
	if _, ok := got["BAC_NEXUS_UNRELATED_SECRET"]; ok {
		t.Fatal("child environment forwarded an unrelated secret")
	}
	if _, ok := got["AWS_SECRET_ACCESS_KEY"]; ok {
		t.Fatal("child environment forwarded an unrelated secret")
	}
	if _, ok := got["SSH_AUTH_SOCK"]; ok {
		t.Fatal("child environment forwarded an SSH credential capability")
	}
}

func TestControlledWindowAdmissionRejectsNonCanonicalAndMismatchBeforeChild(t *testing.T) {
	approval := profile.ControlledValidationApproval{
		WindowStart: time.Date(2026, time.January, 2, 9, 0, 0, 0, time.FixedZone("CST", -6*60*60)),
		WindowEnd:   time.Date(2026, time.January, 2, 10, 0, 0, 0, time.FixedZone("CST", -6*60*60)),
	}

	requested := "2026-01-02T15:00:00Z/2026-01-02T16:00:00Z"
	if !controlledWindowMatchesApproval(requested, approval) {
		t.Fatal("canonical UTC request was rejected")
	}
	if controlledWindowMatchesApproval("2026-01-02T09:00:00-06:00/2026-01-02T10:00:00-06:00", approval) {
		t.Fatal("semantically equal non-canonical offset request was accepted")
	}
	if controlledWindowMatchesApproval("2026-01-02T15:00:00Z/2026-01-02T16:01:00Z", approval) {
		t.Fatal("mismatched request was accepted")
	}
}

// TestControlledNexusServe is the sole controlled live IBM i gate. It is
// intentionally excluded from normal tests and CI by the ibmi_integration tag.
func TestControlledNexusServe(t *testing.T) {
	config := controlledConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, config.binary, "serve", "-profile", config.profile)
	command.Env = controlledChildEnvironment(config)
	client := mcp.NewClient(&mcp.Implementation{Name: "bac-nexus-controlled-gate", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal("controlled gate could not start the approved MCP server")
	}

	defer func() {
		if err := session.Close(); err != nil {
			t.Error("controlled gate shutdown was not confirmed")
		}
		waited := make(chan error, 1)
		go func() { waited <- session.Wait() }()
		select {
		case <-waited:
		case <-time.After(10 * time.Second):
			t.Error("controlled gate shutdown timed out")
		}
	}()

	resolved := callTool[nexusmcp.ResolveCatalogOutput](t, ctx, session, "resolve_catalog_candidates", map[string]any{
		"item": config.item, "productionLibrary": config.library,
	})
	if len(resolved.Candidates) == 0 {
		t.Fatal("controlled gate did not return an approved candidate")
	}

	selection := resolved.Candidates[0]
	startLine, cursor := 1, ""
	for pages := 0; ; pages++ {
		if pages >= 20_972 {
			t.Fatal("controlled gate paging did not reach EOF within the bounded source limit")
		}
		page := callTool[nexusmcp.ReadSelectedSourceOutput](t, ctx, session, "read_selected_source", map[string]any{
			"selection": selection, "cursor": cursor, "startLine": startLine, "maxLines": 200,
		})
		if page.Page.EOF {
			break
		}
		cursor, startLine = page.Page.Cursor, page.Page.NextStartLine
	}

	cancelled, stop := context.WithCancel(ctx)
	stop()
	_, err = session.CallTool(cancelled, &mcp.CallToolParams{Name: "read_selected_source", Arguments: map[string]any{
		"selection": selection, "startLine": 1, "maxLines": 1,
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("controlled gate cancellation was not confirmed")
	}
}

type gateConfig struct {
	binary, manifest, profile, target, window, item, library string
	configRoot, auditRoot, ownershipRoot                     string
}

func controlledConfig(t *testing.T) gateConfig {
	t.Helper()
	if os.Getenv("BAC_NEXUS_CONTROLLED_IBM_I_GATE") != controlledGateOptIn {
		t.Skip("controlled IBM i gate requires its dedicated exact opt-in")
	}
	config := gateConfig{
		binary: os.Getenv("BAC_NEXUS_CONTROLLED_BINARY"), manifest: os.Getenv("BAC_NEXUS_CONTROLLED_MANIFEST"),
		profile: os.Getenv("BAC_NEXUS_CONTROLLED_PROFILE"), target: os.Getenv("BAC_NEXUS_CONTROLLED_TARGET"),
		window: os.Getenv("BAC_NEXUS_CONTROLLED_WINDOW"), item: os.Getenv("BAC_NEXUS_CONTROLLED_ITEM"), library: os.Getenv("BAC_NEXUS_CONTROLLED_LIBRARY"),
		configRoot: os.Getenv("BAC_NEXUS_CONTROLLED_CONFIG_ROOT"), auditRoot: os.Getenv("BAC_NEXUS_CONTROLLED_AUDIT_ROOT"), ownershipRoot: os.Getenv("BAC_NEXUS_CONTROLLED_OWNERSHIP_ROOT"),
	}
	if config.binary == "" || config.manifest == "" || config.profile == "" || config.target == "" || config.window == "" || config.item == "" || config.library == "" || os.Getenv("BAC_NEXUS_CONTROLLED_POLICY") != "verified-readonly" || os.Getenv("BAC_NEXUS_CONTROLLED_ARTIFACT") != "mapepire-2.3.6" {
		t.Fatal("controlled gate prerequisites are incomplete or unapproved")
	}
	if config.auditRoot != filepath.Join(config.configRoot, "BAC Nexus", "audit") || config.ownershipRoot != filepath.Join(config.configRoot, "BAC Nexus", "ownership") {
		t.Fatal("controlled gate local roots are not approved")
	}
	userConfigRoot, err := os.UserConfigDir()
	if err != nil || userConfigRoot != config.configRoot {
		t.Fatal("controlled gate configuration root is not the current protected user root")
	}
	for _, root := range []string{config.configRoot, config.auditRoot, config.ownershipRoot} {
		if !filepath.IsAbs(root) {
			t.Fatal("controlled gate local roots are not approved")
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			t.Fatal("controlled gate local roots are unavailable")
		}
	}
	profiles := profile.Store{Root: filepath.Join(config.configRoot, "BAC Nexus", "profiles")}
	loaded, err := profiles.Load(config.profile)
	if err != nil {
		t.Fatal("controlled gate current V3 profile is unavailable")
	}
	binding, err := profile.DeriveEligibilityBinding(loaded)
	if loaded.SchemaVersion != profile.SchemaVersionV3 || loaded.CredentialMode != profile.CredentialModeKeyring || err != nil || !strings.HasPrefix(binding.CredentialRef, "keyring:") {
		t.Fatal("controlled gate current V3 profile is ineligible")
	}
	keyring := credential.NewNativeCredentialStore()
	if keyring.Capability() != credential.CapabilitySupported {
		t.Fatal("controlled gate native keyring capability is unavailable")
	}
	eligibilities := profile.EligibilityStore{Root: profiles.Root}
	if eligibilities.Check(loaded, binding, true, time.Now()) != profile.EligibilityApproved {
		t.Fatal("controlled gate persisted eligibility binding is unavailable")
	}
	if config.target != loaded.Host {
		t.Fatal("controlled gate requested target does not match the current profile")
	}
	approvals := profile.ControlledValidationApprovalStore{}
	approval, approvalResult := approvals.Load(config.profile)
	if approvalResult != profile.ControlledValidationApproved || !controlledWindowMatchesApproval(config.window, approval) {
		t.Fatal("controlled gate requested window does not match the approved canonical window")
	}
	if approvals.Check(loaded, binding, profile.ControlledValidationRequest{Item: config.item, Library: config.library, Window: config.window}, time.Now()) != profile.ControlledValidationApproved {
		t.Fatal("controlled gate operator approval is unavailable or does not match")
	}
	verifyHandoff(t, config)
	return config
}

func controlledWindowMatchesApproval(requested string, approval profile.ControlledValidationApproval) bool {
	return requested == approval.WindowStart.UTC().Format(time.RFC3339)+"/"+approval.WindowEnd.UTC().Format(time.RFC3339)
}

func controlledChildEnvironment(config gateConfig) []string {
	environment := []string{"XDG_CONFIG_HOME=" + config.configRoot}
	for _, key := range []string{"XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	if runtime.GOOS == "windows" {
		if value, ok := os.LookupEnv("SYSTEMROOT"); ok && value != "" {
			environment = append(environment, "SYSTEMROOT="+value)
		}
	}
	return environment
}

func verifyHandoff(t *testing.T, config gateConfig) {
	t.Helper()
	binary, err := os.ReadFile(config.binary)
	if err != nil {
		t.Fatal("controlled gate approved binary is unavailable")
	}
	manifestBytes, err := os.ReadFile(config.manifest)
	if err != nil {
		t.Fatal("controlled gate approved manifest is unavailable")
	}
	var manifest release.Manifest
	if json.Unmarshal(manifestBytes, &manifest) != nil || manifest.Status != release.ReadyStatus || manifest.IBMIStatus != release.NotValidated || release.VerifyManifest(manifest, config.binary, binary, release.Identity{Version: manifest.ReleaseVersion, Revision: manifest.VCSRevision}) != nil {
		t.Fatal("controlled gate approved handoff is invalid")
	}
}

func callTool[T any](t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) T {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil || result.IsError {
		t.Fatal("controlled gate MCP tool invocation failed")
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal("controlled gate MCP result was invalid")
	}
	var output T
	if json.Unmarshal(encoded, &output) != nil {
		t.Fatal("controlled gate MCP result was invalid")
	}
	return output
}
