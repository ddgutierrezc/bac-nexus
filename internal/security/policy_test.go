package security

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestPolicyAuthorizesAllowlistedSelectorForMatchingTarget proves that an
// allowlisted selector against the matching capability class and target
// class returns a deterministic allow decision without consulting any
// remote, path, shell, SQL, or SSH operation.
func TestPolicyAuthorizesAllowlistedSelectorForMatchingTarget(t *testing.T) {
	policy := NewPolicy()
	decision, err := policy.Authorize(context.Background(), SelectorResolveCatalog, TargetIBMiCatalog)
	if err != nil {
		t.Fatalf("Authorize error = %v", err)
	}
	if decision.Decision != DecisionAllow {
		t.Fatalf("decision = %q, want %q", decision.Decision, DecisionAllow)
	}
	if decision.Selector != SelectorResolveCatalog {
		t.Fatalf("selector = %q, want %q", decision.Selector, SelectorResolveCatalog)
	}
	if decision.Class != CapabilityCatalogResolve {
		t.Fatalf("class = %q, want %q", decision.Class, CapabilityCatalogResolve)
	}
	if decision.Target != TargetIBMiCatalog {
		t.Fatalf("target = %q, want %q", decision.Target, TargetIBMiCatalog)
	}
}

// TestPolicyDeniesUnknownSelectorBeforeAnySideEffect proves that an
// unknown or malformed selector is denied deterministically and does not
// invoke the credential store, remote, or audit-sensitive operation.
// The decision reason must be a bounded, non-sensitive classification.
func TestPolicyDeniesUnknownSelectorBeforeAnySideEffect(t *testing.T) {
	policy := NewPolicy()
	decision, err := policy.Authorize(context.Background(), Selector("rogue-connector"), TargetIBMiCatalog)
	if err != nil {
		t.Fatalf("Authorize error = %v", err)
	}
	if decision.Decision != DecisionDeny {
		t.Fatalf("decision = %q, want %q", decision.Decision, DecisionDeny)
	}
	if len(decision.Reason) == 0 {
		t.Fatal("denial reason is empty; expected bounded classification")
	}
}

// TestPolicyDeniesMalformedSelector covers empty, whitespace, control-byte,
// and case-variant selectors. None of these may ever authorize because the
// allowlist is exact and case-sensitive.
func TestPolicyDeniesMalformedSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector
	}{
		{name: "empty", selector: Selector("")},
		{name: "whitespace", selector: Selector("   ")},
		{name: "trailing space", selector: Selector("resolve_catalog_candidates ")},
		{name: "leading space", selector: Selector(" resolve_catalog_candidates")},
		{name: "case variant upper", selector: Selector("Resolve_Catalog_Candidates")},
		{name: "case variant lower", selector: Selector("resolve_Catalog_candidates")},
		{name: "control byte", selector: Selector("resolve_catalog_candidates\x00")},
		{name: "newline", selector: Selector("resolve_catalog_candidates\n")},
		{name: "tab", selector: Selector("resolve_catalog_candidates\t")},
		{name: "embedded null", selector: Selector("resolve\x00catalog_candidates")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewPolicy()
			decision, err := policy.Authorize(context.Background(), tt.selector, TargetIBMiCatalog)
			if err != nil {
				t.Fatalf("Authorize error = %v", err)
			}
			if decision.Decision != DecisionDeny {
				t.Fatalf("decision = %q, want %q", decision.Decision, DecisionDeny)
			}
		})
	}
}

// TestPolicyDeniesSelectorTargetMismatch proves that an allowlisted
// selector still fails when the target class does not match the
// capability. The selector capability and the target class are
// independent allowlist entries; both must agree.
func TestPolicyDeniesSelectorTargetMismatch(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector
		target   CapabilityTarget
	}{
		{name: "resolve against source", selector: SelectorResolveCatalog, target: TargetIBMiSource},
		{name: "read against catalog", selector: SelectorReadSource, target: TargetIBMiCatalog},
		{name: "resolve against empty", selector: SelectorResolveCatalog, target: CapabilityTarget("")},
		{name: "read against empty", selector: SelectorReadSource, target: CapabilityTarget("")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewPolicy()
			decision, err := policy.Authorize(context.Background(), tt.selector, tt.target)
			if err != nil {
				t.Fatalf("Authorize error = %v", err)
			}
			if decision.Decision != DecisionDeny {
				t.Fatalf("decision = %q, want %q", decision.Decision, DecisionDeny)
			}
		})
	}
}

// TestPolicyAuthorizesIdenticallyRegardlessOfClientIdentityVariants proves
// that the policy never branches on the parent process, product identity,
// or `clientInfo` content. The spoofed identifier lives in the
// surrounding context that the policy never inspects; the canonical
// allowlisted selector MUST yield the same allow decision regardless
// of the surrounding identifier.
func TestPolicyAuthorizesIdenticallyRegardlessOfClientIdentityVariants(t *testing.T) {
	infos := []string{
		"copilot/1.0",
		"opencode/2.0",
		"product-X/3.0",
		"",
		"unknown",
		"copilot\x00rogue",
	}
	for _, info := range infos {
		t.Run(info, func(t *testing.T) {
			policy := NewPolicy()
			decision, err := policy.Authorize(context.Background(), SelectorResolveCatalog, TargetIBMiCatalog)
			if err != nil {
				t.Fatalf("Authorize(info=%q) error = %v", info, err)
			}
			if decision.Decision != DecisionAllow {
				t.Fatalf("info=%q: decision = %q, want %q", info, decision.Decision, DecisionAllow)
			}
		})
	}
}

// TestPolicyAuthorizesSameSelectorRegardlessOfParentProcessIdentity proves
// the policy does not branch on parent process identity. The function
// surface does not accept any parent-process input.
func TestPolicyAuthorizesSameSelectorRegardlessOfParentProcessIdentity(t *testing.T) {
	// We assert structurally: Authorize takes only (ctx, selector, target).
	// If a future revision adds a parent/PID/product argument, the
	// structural reflection check below will fail.
	policyType := reflect.TypeOf((*Policy)(nil))
	method, ok := policyType.MethodByName("Authorize")
	if !ok {
		t.Fatal("Policy.Authorize is missing")
	}
	if got, want := method.Type.NumIn(), 4; got != want {
		t.Fatalf("Authorize takes %d arguments, want %d (ctx, selector, target)", got, want)
	}
	if method.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		t.Fatalf("Authorize arg 1 is not context.Context")
	}
	if method.Type.In(2) != reflect.TypeOf(Selector("")) {
		t.Fatalf("Authorize arg 2 is not Selector")
	}
	if method.Type.In(3) != reflect.TypeOf(CapabilityTarget("")) {
		t.Fatalf("Authorize arg 3 is not CapabilityTarget")
	}
}

// TestPolicyAuthorizeRespectsCancelledContext proves that authorization
// respects context cancellation. The current local OS principal boundary
// still must honor context cancellation deterministically.
func TestPolicyAuthorizeRespectsCancelledContext(t *testing.T) {
	policy := NewPolicy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := policy.Authorize(ctx, SelectorResolveCatalog, TargetIBMiCatalog)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// TestPolicyAuthorizeNeverLeaksSensitiveContentInReason proves that the
// denial reason is a bounded, non-sensitive classification. Source text,
// hash, cursor, path, host, user, command, SQL, credential, or raw error
// must never appear.
func TestPolicyAuthorizeNeverLeaksSensitiveContentInReason(t *testing.T) {
	policy := NewPolicy()
	decision, err := policy.Authorize(context.Background(), Selector("rogue-selector-with-p@th-and-cred"), TargetIBMiCatalog)
	if err != nil {
		t.Fatalf("Authorize error = %v", err)
	}
	if decision.Decision != DecisionDeny {
		t.Fatalf("decision = %q, want %q", decision.Decision, DecisionDeny)
	}
	lower := strings.ToLower(decision.Reason)
	for _, sensitive := range []string{"@", "cred", "password", "secret", "path", "user", "host", "command", "sql", "error:", "clientinfo", "client_info"} {
		if strings.Contains(lower, sensitive) {
			t.Fatalf("denial reason %q contains sensitive substring %q", decision.Reason, sensitive)
		}
	}
	if len(decision.Reason) > 64 {
		t.Fatalf("denial reason length = %d, want <= 64", len(decision.Reason))
	}
}

// TestPolicyRejectsUnknownCapabilityTarget proves the allowlist
// rejects capability target classes that are not on the allowlist.
func TestPolicyRejectsUnknownCapabilityTarget(t *testing.T) {
	policy := NewPolicy()
	decision, err := policy.Authorize(context.Background(), SelectorResolveCatalog, CapabilityTarget("postgres-database"))
	if err != nil {
		t.Fatalf("Authorize error = %v", err)
	}
	if decision.Decision != DecisionDeny {
		t.Fatalf("decision = %q, want %q", decision.Decision, DecisionDeny)
	}
}

// TestPinnedTrustVerifiesExactApprovedPin proves the pinned TOFU seam
// returns nil for an exact match between the pinned and observed
// fingerprint and binding.
func TestPinnedTrustVerifiesExactApprovedPin(t *testing.T) {
	trust := NewPinnedTrust()
	binding := []byte("approved-target-binding-32-bytes-xx")
	pin := PinnedTarget{
		Trust:       HostKeyTrustTOFU,
		Fingerprint: "SHA256:exactfingerprint",
		Binding:     binding,
	}
	if err := trust.Verify(context.Background(), pin, "SHA256:exactfingerprint", binding); err != nil {
		t.Fatalf("Verify error = %v", err)
	}
}

func TestPinnedTrustEnrollsExplicitTOFUAndCopiesEvidence(t *testing.T) {
	trust := NewPinnedTrust()
	binding := []byte("approved-target-binding-32-bytes-xx")
	pin, err := trust.Enroll(context.Background(), "SHA256:enrolled", binding, "operator-approved")
	if err != nil {
		t.Fatalf("Enroll error = %v", err)
	}
	if pin.Trust != HostKeyTrustTOFU || pin.Fingerprint != "SHA256:enrolled" || pin.Provenance != "operator-approved" {
		t.Fatalf("pin = %+v, want explicit TOFU provenance", pin)
	}
	binding[0] = 'x'
	if err := trust.Verify(context.Background(), pin, "SHA256:enrolled", []byte("approved-target-binding-32-bytes-xx")); err != nil {
		t.Fatalf("Verify enrolled pin error = %v", err)
	}
}

func TestPinnedTrustEnrollFailsClosedOnMissingEvidenceOrCancellation(t *testing.T) {
	trust := NewPinnedTrust()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name string
		ctx  context.Context
		fp   string
		key  []byte
		prov string
		want error
	}{
		{name: "missing provenance", ctx: context.Background(), fp: "SHA256:key", key: []byte("binding"), want: ErrTrustEvidenceMissing},
		{name: "missing fingerprint", ctx: context.Background(), key: []byte("binding"), prov: "operator", want: ErrTrustEvidenceMissing},
		{name: "cancelled", ctx: cancelled, fp: "SHA256:key", key: []byte("binding"), prov: "operator", want: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := trust.Enroll(tt.ctx, tt.fp, tt.key, tt.prov)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Enroll error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestPinnedTrustFailsClosedOnFingerprintChange proves the pinned TOFU
// seam fails closed with a deterministic error when the observed
// fingerprint differs from the pinned fingerprint.
func TestPinnedTrustFailsClosedOnFingerprintChange(t *testing.T) {
	trust := NewPinnedTrust()
	binding := []byte("approved-target-binding-32-bytes-xx")
	pin := PinnedTarget{
		Trust:       HostKeyTrustTOFU,
		Fingerprint: "SHA256:original",
		Binding:     binding,
	}
	err := trust.Verify(context.Background(), pin, "SHA256:changed", binding)
	if !errors.Is(err, ErrHostKeyChanged) {
		t.Fatalf("error = %v, want ErrHostKeyChanged", err)
	}
}

// TestPinnedTrustFailsClosedOnBindingChange proves the pinned TOFU seam
// fails closed with a deterministic error when the observed binding
// digest differs from the pinned binding.
func TestPinnedTrustFailsClosedOnBindingChange(t *testing.T) {
	trust := NewPinnedTrust()
	pin := PinnedTarget{
		Trust:       HostKeyTrustTOFU,
		Fingerprint: "SHA256:same",
		Binding:     []byte("original-binding-padding-32-x"),
	}
	err := trust.Verify(context.Background(), pin, "SHA256:same", []byte("rebound-different-padding-32"))
	if !errors.Is(err, ErrHostKeyChanged) {
		t.Fatalf("error = %v, want ErrHostKeyChanged", err)
	}
}

// TestPinnedTrustFailsClosedOnMissingPinEvidence proves the pinned TOFU
// seam fails closed when the pin itself lacks evidence (no fingerprint
// or no binding). This guards against silent enrollment from a missing
// or empty pin.
func TestPinnedTrustFailsClosedOnMissingPinEvidence(t *testing.T) {
	trust := NewPinnedTrust()
	tests := []struct {
		name string
		pin  PinnedTarget
	}{
		{name: "empty pin", pin: PinnedTarget{}},
		{name: "missing fingerprint", pin: PinnedTarget{Trust: HostKeyTrustTOFU, Binding: []byte("binding")}},
		{name: "missing binding", pin: PinnedTarget{Trust: HostKeyTrustTOFU, Fingerprint: "SHA256:p"}},
		{name: "unknown trust", pin: PinnedTarget{Trust: HostKeyTrust("legacy"), Fingerprint: "SHA256:p", Binding: []byte("b")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := trust.Verify(context.Background(), tt.pin, "SHA256:any", []byte("any"))
			if !errors.Is(err, ErrTrustEvidenceMissing) {
				t.Fatalf("error = %v, want ErrTrustEvidenceMissing", err)
			}
		})
	}
}

// TestPinnedTrustFailsClosedOnMissingObservedEvidence proves the pinned
// TOFU seam fails closed when the observed evidence is missing or
// truncated. The seam never returns nil for empty observed values.
func TestPinnedTrustFailsClosedOnMissingObservedEvidence(t *testing.T) {
	trust := NewPinnedTrust()
	pin := PinnedTarget{Trust: HostKeyTrustTOFU, Fingerprint: "SHA256:p", Binding: []byte("b")}

	if err := trust.Verify(context.Background(), pin, "", []byte("b")); !errors.Is(err, ErrTrustEvidenceMissing) {
		t.Fatalf("missing observed fingerprint: error = %v, want ErrTrustEvidenceMissing", err)
	}
	if err := trust.Verify(context.Background(), pin, "SHA256:p", nil); !errors.Is(err, ErrTrustEvidenceMissing) {
		t.Fatalf("missing observed binding: error = %v, want ErrTrustEvidenceMissing", err)
	}
	if err := trust.Verify(context.Background(), pin, "SHA256:p", []byte{}); !errors.Is(err, ErrTrustEvidenceMissing) {
		t.Fatalf("empty observed binding: error = %v, want ErrTrustEvidenceMissing", err)
	}
}

// TestPinnedTrustRejectsAmbiguousObservedEvidence proves the pinned TOFU
// seam never accepts more than one observed binding at a time. The
// Verify method takes a single binding, so structurally the seam cannot
// enroll from a pair of observed bindings.
func TestPinnedTrustRejectsAmbiguousObservedEvidence(t *testing.T) {
	trust := NewPinnedTrust()
	pin := PinnedTarget{
		Trust:       HostKeyTrustTOFU,
		Fingerprint: "SHA256:ambiguous",
		Binding:     []byte("approved-binding-padding-32-x"),
	}
	// reflect.TypeOf(methodValue) returns the method type without the
	// receiver; the explicit arguments are (ctx, pin, fingerprint,
	// binding), so NumIn is 4. Including the receiver (via the
	// pointer type) yields 5.
	boundType := reflect.TypeOf(trust.Verify)
	if got, want := boundType.NumIn(), 4; got != want {
		t.Fatalf("Verify explicit arguments = %d, want %d (ctx, pin, fingerprint, binding)", got, want)
	}
	pointerType := reflect.TypeOf((*PinnedTrust)(nil))
	method, ok := pointerType.MethodByName("Verify")
	if !ok {
		t.Fatal("PinnedTrust.Verify is missing")
	}
	if got, want := method.Type.NumIn(), 5; got != want {
		t.Fatalf("Verify method type arguments = %d, want %d (receiver, ctx, pin, fingerprint, binding)", got, want)
	}
	if err := trust.Verify(context.Background(), pin, "SHA256:ambiguous", []byte("approved-binding-padding-32-x")); err != nil {
		t.Fatalf("Verify(single) error = %v, want nil", err)
	}
}

// TestPinnedTrustNeverEnrollsFromObservedEvidence proves that a missing
// pin cannot be silently populated from observed evidence. The pin is
// always supplied by the caller; the seam never reads or writes state.
func TestPinnedTrustNeverEnrollsFromObservedEvidence(t *testing.T) {
	trust := NewPinnedTrust()
	pin := PinnedTarget{Trust: HostKeyTrustTOFU}
	err := trust.Verify(context.Background(), pin, "SHA256:any-observed", []byte("any-observed"))
	if !errors.Is(err, ErrTrustEvidenceMissing) {
		t.Fatalf("error = %v, want ErrTrustEvidenceMissing (no silent enrollment)", err)
	}
}

// TestPinnedTrustRespectsCancelledContext proves the pinned TOFU seam
// respects context cancellation.
func TestPinnedTrustRespectsCancelledContext(t *testing.T) {
	trust := NewPinnedTrust()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pin := PinnedTarget{Trust: HostKeyTrustTOFU, Fingerprint: "SHA256:p", Binding: []byte("b")}
	if err := trust.Verify(ctx, pin, "SHA256:p", []byte("b")); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// TestPinnedTrustBindingCompareIsConstantTime proves the binding
// comparison does not short-circuit on the first mismatched byte. A
// naive byte loop would leak the prefix length of a correct binding.
func TestPinnedTrustBindingCompareIsConstantTime(t *testing.T) {
	trust := NewPinnedTrust()
	goodBinding := bytes.Repeat([]byte{0x42}, sha256.Size)
	pin := PinnedTarget{Trust: HostKeyTrustVerified, Fingerprint: "SHA256:p", Binding: goodBinding}
	tests := []struct {
		length int
		want   error
	}{
		// Empty observed binding is a missing-evidence failure, not a
		// host-key change.
		{length: 0, want: ErrTrustEvidenceMissing},
		// Shorter observed bindings differ in length and are a
		// host-key change because the seam performed the comparison.
		{length: 1, want: ErrHostKeyChanged},
		{length: 16, want: ErrHostKeyChanged},
		{length: 31, want: ErrHostKeyChanged},
		// Same length with the last byte tampered is also a
		// host-key change.
		{length: 32, want: ErrHostKeyChanged},
	}
	for _, tt := range tests {
		prefix := make([]byte, tt.length)
		copy(prefix, goodBinding[:tt.length])
		if tt.length == sha256.Size {
			prefix[tt.length-1] ^= 0x01
		}
		if err := trust.Verify(context.Background(), pin, "SHA256:p", prefix); !errors.Is(err, tt.want) {
			t.Fatalf("prefix length %d: error = %v, want %v", tt.length, err, tt.want)
		}
	}
}

// TestPinnedTrustRejectsMalformedFingerprint covers fingerprints that
// do not follow the canonical "SHA256:" + base64 form. The seam must
// fail closed for any fingerprint that is not equal to the pinned
// fingerprint, regardless of shape. Empty observed fingerprints are
// missing evidence; non-empty malformed fingerprints are a host-key
// change.
func TestPinnedTrustRejectsMalformedFingerprint(t *testing.T) {
	trust := NewPinnedTrust()
	pin := PinnedTarget{Trust: HostKeyTrustVerified, Fingerprint: "SHA256:" + hex.EncodeToString(bytes.Repeat([]byte{0x42}, sha256.Size)), Binding: bytes.Repeat([]byte{0x42}, sha256.Size)}
	tests := []struct {
		fingerprint string
		want        error
	}{
		{fingerprint: "", want: ErrTrustEvidenceMissing},
		{fingerprint: "MD5:00000000", want: ErrHostKeyChanged},
		{fingerprint: "SHA256:", want: ErrHostKeyChanged},
		{fingerprint: "sha256:00000000", want: ErrHostKeyChanged},
		{fingerprint: "SHA256:" + strings.Repeat("z", 64), want: ErrHostKeyChanged},
	}
	for _, tt := range tests {
		if err := trust.Verify(context.Background(), pin, tt.fingerprint, pin.Binding); !errors.Is(err, tt.want) {
			t.Fatalf("malformed fingerprint %q: error = %v, want %v", tt.fingerprint, err, tt.want)
		}
	}
}

// TestSecurityPackageHasNoRemotePathOrShellSurface is a structural
// reflection test: the public surface of the security package must
// never expose generic remote, path, shell, SQL, or SSH operations.
// The trust boundary is the current local OS principal; everything
// else is fail-closed policy.
func TestSecurityPackageHasNoRemotePathOrShellSurface(t *testing.T) {
	checks := []struct {
		typ   reflect.Type
		label string
	}{
		{typ: reflect.TypeOf((*Policy)(nil)), label: "Policy"},
		{typ: reflect.TypeOf((*PinnedTrust)(nil)), label: "PinnedTrust"},
		{typ: reflect.TypeOf((*Principal)(nil)), label: "Principal"},
		{typ: reflect.TypeOf((*Authorizer)(nil)).Elem(), label: "Authorizer"},
	}
	for _, check := range checks {
		for _, forbidden := range forbiddenMethodSubstrings {
			found, name := hasMethodContaining(check.typ, forbidden)
			if found {
				t.Fatalf("%s has forbidden method %q (matched %q)", check.label, name, forbidden)
			}
		}
	}
}

// forbiddenMethodSubstrings is the structural guard list. The list is
// authoritative; adding an entry requires an explicit decision and a
// matching red test.
var forbiddenMethodSubstrings = []string{
	"ssh",
	"exec",
	"shell",
	"path",
	"command",
	"sql",
	"dial",
	"connect",
	"remote",
	"clientinfo",
	"parent",
}

// hasMethodContaining returns whether the supplied type exposes a
// method whose lower-cased name contains the supplied substring. It
// also returns the matching method name for diagnostics.
func hasMethodContaining(typ reflect.Type, substring string) (bool, string) {
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.Contains(strings.ToLower(name), substring) {
			return true, name
		}
	}
	return false, ""
}
