package configuration

import (
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
)

func TestStep8DecisionReasonsAreExhaustiveAndFailClosed(t *testing.T) {
	tests := []struct {
		reason   Step8Reason
		decision Decision
	}{
		{ReasonWSSSelected, DecisionWSSSelected},
		{ReasonDaemonRefused, DecisionSSHEligible}, {ReasonDaemonUnavailable, DecisionSSHEligible},
		{ReasonDaemonAvailabilityTimeout, DecisionSSHEligible}, {ReasonDaemonPolicyDisabled, DecisionSSHEligible},
		{ReasonUnsupportedVersion, DecisionSSHEligible}, {ReasonIdentityFailure, DecisionTerminal},
		{ReasonProtocolFailure, DecisionTerminal}, {ReasonMalformedResponse, DecisionTerminal},
		{ReasonDowngradeBlocked, DecisionTerminal}, {ReasonCancelled, DecisionTerminal},
		{ReasonOperationTimeout, DecisionTerminal}, {ReasonLimitExceeded, DecisionTerminal},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			if got := DecisionForReason(tt.reason); got != tt.decision {
				t.Fatalf("decision=%q want %q", got, tt.decision)
			}
		})
	}
	if got := DecisionForReason(Step8Reason("unknown")); got != DecisionTerminal {
		t.Fatalf("unknown decision=%q", got)
	}
	if got := DecisionForReason(ReasonCredentialsUnavailable); got != DecisionTerminal {
		t.Fatalf("credential decision=%q", got)
	}
}

func TestStep8ResultClassesMapOnlyKnownValues(t *testing.T) {
	classes := []ResultClass{ResultIdentityFailure, ResultTrustMismatch, ResultProtocolFailure, ResultFramingFailure, ResultMalformedResponse, ResultDowngradeBlocked, ResultCredentialsUnavailable, ResultAuthenticationFailed, ResultAuthorizationDenied, ResultCancelled, ResultOperationTimeout, ResultProofTimeout, ResultCleanupTimeout, ResultCleanupFailure, ResultLimitExceeded, ResultConsentDeclined, ResultArtifactFailure, ResultJavaFailure, ResultUploadFailure, ResultLaunchFailure, ResultSessionFailure, ResultProofFailure}
	for _, class := range classes {
		if !IsTerminalResult(class) {
			t.Fatalf("%q is not terminal", class)
		}
	}
	for _, class := range []ResultClass{ResultTrustMismatch, ResultCredentialsUnavailable, ResultDowngradeBlocked} {
		if class == ResultIdentityFailure {
			t.Fatalf("distinct class collapsed: %q", class)
		}
	}
	if IsTerminalResult(ResultClass("unknown")) {
		t.Fatal("unknown result was accepted")
	}
}

func TestStep8SavedProfileAndCredentialContract(t *testing.T) {
	if err := ValidateStep8Profile(profile.Profile{Name: "draft"}); err == nil {
		t.Fatal("unsaved profile accepted")
	}
	for _, mode := range []profile.CredentialMode{profile.CredentialModePrompt, profile.CredentialModeKeyring} {
		if err := ValidateCredentialMode(mode); err != nil {
			t.Fatalf("mode %q rejected: %v", mode, err)
		}
	}
	if err := ValidateCredentialMode(profile.CredentialMode("vault")); err == nil {
		t.Fatal("legacy vault silently accepted")
	}
	key, err := credential.KeyForProfile("production")
	if err != nil || key != "ibmi/production" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if _, err := credential.KeyForProfile("bad/profile"); err == nil {
		t.Fatal("invalid profile key accepted")
	}
	p := profile.Profile{SchemaVersion: profile.SchemaVersionV2, Name: "saved", Host: "ibmi.example.test", Port: 22, Username: "USER", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: profile.HostKeyTrustVerified, CredentialMode: profile.CredentialModePrompt, EndpointPolicyRef: "policy"}
	if migrated, err := profile.MigrateToV3(p); err != nil || migrated.SchemaVersion != profile.SchemaVersionV3 {
		t.Fatalf("prompt migration=%#v err=%v", migrated, err)
	}
	p.CredentialMode = profile.CredentialModeVault
	if _, err := profile.MigrateToV3(p); err == nil {
		t.Fatal("vault mode silently migrated")
	}
}

func TestStep8CredentialFailuresAndProofMetadataFailClosed(t *testing.T) {
	for _, err := range []error{ErrPromptUnavailable, ErrPromptDenied, ErrKeyringUnavailable, ErrKeyringDenied, ErrCredentialNotFound, ErrInvalidCredentialMode} {
		if got := ClassifyCredentialFailure(err); got != ResultCredentialsUnavailable {
			t.Fatalf("%v => %q", err, got)
		}
	}
	if got := ClassifyCredentialFailure(errors.New("credentials_unavailable")); got != ResultClass("") {
		t.Fatalf("wrapped/string guess => %q", got)
	}
	if err := ValidateProofMetadata(ProofMetadata{Rows: 2, ProofRevision: "r"}); err == nil {
		t.Fatal("multiple proof rows accepted")
	}
	if err := ValidateProofMetadata(ProofMetadata{Rows: 1, ProofRevision: ProofRevision}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProofMetadata(ProofMetadata{Rows: 1, ProofRevision: "other"}); err == nil {
		t.Fatal("unknown proof revision accepted")
	}
}

func TestStep8MarkerIsBoundedAndInvalidated(t *testing.T) {
	marker := Marker{SchemaVersion: MarkerSchemaVersion, AtUnixMs: time.Now().UnixMilli(), Outcome: ResultProofSuccess, ProofRevision: ProofRevision}
	if err := ValidateMarker(marker); err != nil {
		t.Fatal(err)
	}
	if !MarkerValid(marker, ConfigUnchanged) {
		t.Fatal("fresh marker rejected")
	}
	for _, changed := range []ConfigChange{ConfigEndpointChanged, ConfigPolicyChanged, ConfigTrustChanged} {
		if MarkerValid(marker, changed) {
			t.Fatalf("marker valid after change: %v", changed)
		}
	}
	if MarkerIsReadiness(marker) {
		t.Fatal("marker became readiness evidence")
	}
}

func TestStep8ResultInvariantsFailClosed(t *testing.T) {
	if err := (Step8Result{RequestID: "req-1", Decision: DecisionWSSSelected, Class: ResultProofSuccess, ProofRevision: ProofRevision, Outcome: "validated"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Step8Result{Decision: DecisionSSHEligible, Class: ResultProofFailure}).Validate(); err == nil {
		t.Fatal("eligible terminal result accepted")
	}
	if err := (Step8Result{Decision: DecisionTerminal, Class: ResultClass("unknown")}).Validate(); err == nil {
		t.Fatal("unknown terminal result accepted")
	}
}
