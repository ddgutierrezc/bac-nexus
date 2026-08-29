package profile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func validSchemaV2Profile() Profile {
	return Profile{
		SchemaVersion:     SchemaVersionV2,
		Name:              "daemon",
		Host:              "ibmi.example.test",
		Port:              8076,
		Username:          "NEXUS$USER",
		CredentialMode:    CredentialModePrompt,
		EndpointPolicyRef: "managed-default",
		FallbackAllowed:   true,
		TLSTrust:          TrustEvidence{Mode: TrustModePin, Pin: "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Provenance: "operator-approved"},
		SSHTrust:          TrustEvidence{Mode: TrustModeTOFU, Pin: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Provenance: "operator-confirmed"},
	}
}

func TestSchemaV2RoundTripPersistsOnlyPolicyAndIndependentTrust(t *testing.T) {
	root := t.TempDir()
	p := validSchemaV2Profile()
	if _, err := (Store{Root: root}).Save(p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, p.Name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"selectedTransport", "observedVersion", "readiness", "lastError", "password", "privateKey", "secret"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("schema-v2 profile persisted ephemeral or secret field %q: %s", forbidden, raw)
		}
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document["schemaVersion"] != float64(SchemaVersionV2) {
		t.Fatalf("schemaVersion = %v, want %d", document["schemaVersion"], SchemaVersionV2)
	}
	got, err := (Store{Root: root}).Load(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("Load() = %#v, want %#v", got, p)
	}
}

func TestSchemaV2AllowsUnenrolledTrustEvidence(t *testing.T) {
	p := validSchemaV2Profile()
	p.FallbackAllowed = false
	p.TLSTrust = TrustEvidence{}
	p.SSHTrust = TrustEvidence{}

	if err := p.Validate(); err != nil {
		t.Fatalf("unenrolled schema-v2 profile rejected: %v", err)
	}
	if p.FallbackAllowed {
		t.Fatal("empty trust evidence granted fallback")
	}
}

func TestSchemaV2TrustModesAreTransportSpecific(t *testing.T) {
	tlsCA := validSchemaV2Profile()
	tlsCA.TLSTrust = TrustEvidence{Mode: TrustModeCA, Provenance: "managed-ca"}
	if err := tlsCA.Validate(); err != nil {
		t.Fatalf("TLS CA trust rejected: %v", err)
	}

	sshCA := validSchemaV2Profile()
	sshCA.SSHTrust = TrustEvidence{Mode: TrustModeCA, Provenance: "managed-ca"}
	if err := sshCA.Validate(); err == nil {
		t.Fatal("SSH CA trust accepted")
	}
}

func TestSchemaV2RejectsAmbiguousOrCrossTransportTrustEvidence(t *testing.T) {
	for _, mutate := range []func(*Profile){
		func(p *Profile) {
			p.TLSTrust.Mode = TrustModeTOFU
			p.TLSTrust.Pin = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		},
		func(p *Profile) {
			p.SSHTrust.Mode = TrustModePin
			p.SSHTrust.Pin = "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		},
		func(p *Profile) {
			p.TLSTrust = TrustEvidence{Mode: TrustModePin, Pin: "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
		},
		func(p *Profile) {
			p.SSHTrust = TrustEvidence{Mode: TrustModeTOFU, Pin: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
			p.EndpointPolicyRef = ""
		},
	} {
		p := validSchemaV2Profile()
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Fatalf("Validate() accepted ambiguous trust profile: %#v", p)
		}
	}
}

func TestMigrateV1IsConservativeAndDeterministic(t *testing.T) {
	legacy, err := json.Marshal(map[string]any{
		"name":               "legacy",
		"host":               "ibmi.example.test",
		"port":               22,
		"username":           "USER",
		"hostKeyFingerprint": "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"hostKeyTrust":       "verified",
		"mapepireJar":        filepath.Join(t.TempDir(), "mapepire.jar"),
		"credentialMode":     "vault",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := MigrateV1(legacy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MigrateV1(legacy)
	if err != nil || first != second {
		t.Fatalf("migration is not deterministic: first=%#v second=%#v err=%v", first, second, err)
	}
	if first.SchemaVersion != SchemaVersionV2 || first.FallbackAllowed || first.TLSTrust != (TrustEvidence{}) || first.SSHTrust != (TrustEvidence{}) {
		t.Fatalf("migration trusted or enabled unsupported evidence: %#v", first)
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("migrated schema-v2 profile does not validate: %v", err)
	}
	root := t.TempDir()
	if _, err := (Store{Root: root}).Save(first); err != nil {
		t.Fatalf("save migrated profile: %v", err)
	}
	loaded, err := (Store{Root: root}).Load(first.Name)
	if err != nil {
		t.Fatalf("load migrated profile: %v", err)
	}
	if loaded != first {
		t.Fatalf("migrated round-trip = %#v, want %#v", loaded, first)
	}
	if _, err := MigrateV1([]byte(`{"name":"legacy","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified","credentialMode":"vault","trust":{"tls":"ssh"}}`)); err == nil {
		t.Fatal("migration accepted ambiguous trust evidence")
	}
}

func validProfile() Profile {
	return Profile{Name: "dev", Host: "ibmi.example.test", Port: 22, Username: "NEXUS$USER", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: HostKeyTrustVerified, JavaHome: "/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit", CredentialMode: CredentialModeVault}
}

func TestProfileValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Profile)
		valid  bool
	}{
		{"valid", func(*Profile) {}, true},
		{"host with port", func(p *Profile) { p.Host = "host:22" }, false},
		{"unknown fingerprint", func(p *Profile) { p.HostKeyFingerprint = "" }, false},
		{"invalid fingerprint", func(p *Profile) { p.HostKeyFingerprint = "SHA256:not-base64" }, false},
		{"non-canonical fingerprint base64", func(p *Profile) { p.HostKeyFingerprint = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB" }, false},
		{"tofu host-key trust", func(p *Profile) { p.HostKeyTrust = HostKeyTrustTOFU }, true},
		{"missing host-key trust", func(p *Profile) { p.HostKeyTrust = "" }, false},
		{"unknown host-key trust", func(p *Profile) { p.HostKeyTrust = "legacy" }, false},
		{"case-variant host-key trust", func(p *Profile) { p.HostKeyTrust = "TOFU" }, false},
		{"unsafe Java path", func(p *Profile) { p.JavaHome = "/tmp/java;id" }, false},
		{"default Java discovery", func(p *Profile) { p.JavaHome = "" }, true},
		{"relative Mapepire JAR", func(p *Profile) { p.MapepireJAR = "mapepire.jar" }, false},
		{"prompt credentials", func(p *Profile) { p.CredentialMode = CredentialModePrompt }, true},
		{"missing credential mode", func(p *Profile) { p.CredentialMode = "" }, false},
		{"unknown credential mode", func(p *Profile) { p.CredentialMode = "environment" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)
			if got := p.Validate() == nil; got != tt.valid {
				t.Fatalf("Validate() success = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestValidateNameUsesStrictASCIIProfileContract(t *testing.T) {
	valid64 := "A" + strings.Repeat("a", 63)
	invalid65 := valid64 + "a"
	for _, tt := range []struct {
		name  string
		value string
		valid bool
	}{
		{"empty", "", false},
		{"one letter", "A", true},
		{"one digit", "1", true},
		{"64 characters", valid64, true},
		{"65 characters", invalid65, false},
		{"letters and digits", "Cri400F9", true},
		{"hyphen and underscore after first", "CRI-400_F", true},
		{"leading hyphen", "-CRI", false},
		{"leading underscore", "_CRI", false},
		{"dot", "CRI.400", false},
		{"internal whitespace", "CRI 400", false},
		{"outer whitespace", " CRI400", false},
		{"tab", "CRI\t400", false},
		{"newline", "CRI\n400", false},
		{"accent", "CRÍ400", false},
		{"enye", "CRIñ400", false},
		{"emoji", "CRI😀400", false},
		{"slash", "CRI/400", false},
		{"backslash", `CRI\400`, false},
		{"punctuation", "CRI!400", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.value)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateName(%q) error = %v, want valid=%v", tt.value, err, tt.valid)
			}
			if !tt.valid && err != nil && err.Error() != "profile name must use 1-64 ASCII letters or digits, then ASCII letters, digits, hyphen, or underscore" {
				t.Fatalf("unexpected deterministic error: %v", err)
			}
		})
	}
}

func TestFieldValidatorsPreserveEndpointAndUsernameContracts(t *testing.T) {
	for _, tt := range []struct {
		name, host, username string
		port                 int
		valid                bool
	}{
		{"DNS and username", "ibmi.example.test", "NEXUS$USER", 22, true},
		{"IPv4", "192.0.2.10", "USER", 2222, true},
		{"IPv6 remains rejected", "::1", "USER", 22, false},
		{"host whitespace", " ibmi.example.test", "USER", 22, false},
		{"username whitespace", "ibmi.example.test", "bad user", 22, false},
		{"port zero", "ibmi.example.test", "USER", 0, false},
		{"port too large", "ibmi.example.test", "USER", 65536, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			valid := ValidateHost(tt.host) == nil && ValidateUsername(tt.username) == nil && ValidatePort(tt.port) == nil
			if valid != tt.valid {
				t.Fatalf("field validators valid=%v, want %v", valid, tt.valid)
			}
			endpointValid := ValidateEndpoint(tt.host, tt.port) == nil
			if endpointValid != (ValidateHost(tt.host) == nil && ValidatePort(tt.port) == nil) {
				t.Fatalf("endpoint contract drifted for host=%q port=%d", tt.host, tt.port)
			}
		})
	}
}

func TestStoreRoundTripUsesTemporaryRoot(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	p := validProfile()
	path, err := store.Save(p)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != root {
		t.Fatalf("saved outside temporary root: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("profile permissions = %o, want no group/other access", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"hostKeyTrust": "verified"`) {
		t.Fatalf("profile JSON lacks explicit host-key trust provenance: %s", data)
	}
	got, err := store.Load(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("Load() = %#v, want %#v", got, p)
	}
	if _, err := store.Save(p); err == nil {
		t.Fatal("expected existing profile to fail closed")
	}
}

func TestLoadRejectsTraversal(t *testing.T) {
	if _, err := (Store{Root: t.TempDir()}).Load("../profile"); err == nil {
		t.Fatal("expected traversal name rejection")
	}
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
	root := t.TempDir()
	p := validProfile()
	data := `{"name":"dev","host":"ibmi.example.test","port":22,"username":"NEXUS$USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified","credentialMode":"vault"} {}`
	if err := os.WriteFile(filepath.Join(root, p.Name+".json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Root: root}).Load(p.Name); err == nil {
		t.Fatal("expected trailing JSON rejection")
	}
}

func TestLoadRejectsDuplicateCaseVariantAndLegacyMode(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"duplicate", `{"name":"dev","name":"other","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified","credentialMode":"vault"}`},
		{"case variant", `{"name":"dev","Name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified","credentialMode":"vault"}`},
		{"legacy missing credential mode", `{"name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"verified"}`},
		{"legacy missing host-key trust", `{"name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","credentialMode":"vault"}`},
		{"unknown host-key trust", `{"name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"legacy","credentialMode":"vault"}`},
		{"case-variant host-key trust", `{"name":"dev","host":"ibmi.example.test","port":22,"username":"USER","hostKeyFingerprint":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","hostKeyTrust":"TOFU","credentialMode":"vault"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "dev.json"), []byte(tt.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := (Store{Root: root}).Load("dev"); err == nil {
				t.Fatal("malformed or legacy profile was accepted")
			}
		})
	}
}

func TestConcurrentSaveCreatesExactlyOneProfile(t *testing.T) {
	store := Store{Root: t.TempDir()}
	p := validProfile()
	const attempts = 16
	start := make(chan struct{})
	errorsByAttempt := make(chan error, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := store.Save(p)
			errorsByAttempt <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByAttempt)
	successes := 0
	for err := range errorsByAttempt {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("unexpected save error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful saves = %d, want 1", successes)
	}
	if _, err := store.Load(p.Name); err != nil {
		t.Fatalf("winning profile is incomplete: %v", err)
	}
}
