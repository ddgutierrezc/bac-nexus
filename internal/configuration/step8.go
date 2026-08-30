package configuration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"bac-nexus/internal/profile"
)

type Decision string

const (
	DecisionWSSSelected Decision = "wss_selected"
	DecisionSSHEligible Decision = "ssh_eligible"
	DecisionTerminal    Decision = "terminal"
)

type Step8Reason string

const (
	ReasonWSSSelected               Step8Reason = "wss_selected"
	ReasonDaemonRefused             Step8Reason = "daemon_connection_refused"
	ReasonDaemonUnavailable         Step8Reason = "daemon_unavailable"
	ReasonDaemonAvailabilityTimeout Step8Reason = "daemon_availability_timeout"
	ReasonDaemonPolicyDisabled      Step8Reason = "daemon_policy_disabled"
	ReasonUnsupportedVersion        Step8Reason = "daemon_version_verified_unsupported"
	ReasonIdentityFailure           Step8Reason = "identity_hostname_pin_tofu_trust_mismatch_or_rotation"
	ReasonProtocolFailure           Step8Reason = "protocol_or_framing_failure"
	ReasonMalformedResponse         Step8Reason = "malformed_response"
	ReasonDowngradeBlocked          Step8Reason = "unsafe_downgrade"
	ReasonCancelled                 Step8Reason = "cancelled"
	ReasonOperationTimeout          Step8Reason = "operation_timeout"
	ReasonLimitExceeded             Step8Reason = "limit_exceeded"
	ReasonCredentialsUnavailable    Step8Reason = "credentials_unavailable"
)

func DecisionForReason(reason Step8Reason) Decision {
	switch reason {
	case ReasonWSSSelected:
		return DecisionWSSSelected
	case ReasonDaemonRefused, ReasonDaemonUnavailable, ReasonDaemonAvailabilityTimeout, ReasonDaemonPolicyDisabled, ReasonUnsupportedVersion:
		return DecisionSSHEligible
	default:
		return DecisionTerminal
	}
}

type ResultClass string

const (
	ResultProofSuccess           ResultClass = "proof_success"
	ResultIdentityFailure        ResultClass = "identity_failure"
	ResultTrustMismatch          ResultClass = "trust_mismatch"
	ResultProtocolFailure        ResultClass = "protocol_failure"
	ResultFramingFailure         ResultClass = "framing_failure"
	ResultMalformedResponse      ResultClass = "malformed_response"
	ResultDowngradeBlocked       ResultClass = "downgrade_blocked"
	ResultCredentialsUnavailable ResultClass = "credentials_unavailable"
	ResultAuthenticationFailed   ResultClass = "authentication_failed"
	ResultAuthorizationDenied    ResultClass = "authorization_denied"
	ResultCancelled              ResultClass = "cancelled"
	ResultOperationTimeout       ResultClass = "operation_timeout"
	ResultProofTimeout           ResultClass = "proof_timeout"
	ResultCleanupTimeout         ResultClass = "cleanup_timeout"
	ResultCleanupFailure         ResultClass = "cleanup_failure"
	ResultLimitExceeded          ResultClass = "limit_exceeded"
	ResultConsentDeclined        ResultClass = "consent_declined_or_absent"
	ResultArtifactFailure        ResultClass = "artifact_failure"
	ResultJavaFailure            ResultClass = "java_failure"
	ResultUploadFailure          ResultClass = "upload_failure"
	ResultLaunchFailure          ResultClass = "launch_failure"
	ResultSessionFailure         ResultClass = "session_failure"
	ResultProofFailure           ResultClass = "proof_failure"
)

func IsTerminalResult(c ResultClass) bool {
	switch c {
	case ResultIdentityFailure, ResultTrustMismatch, ResultProtocolFailure, ResultFramingFailure, ResultMalformedResponse, ResultDowngradeBlocked, ResultCredentialsUnavailable, ResultAuthenticationFailed, ResultAuthorizationDenied, ResultCancelled, ResultOperationTimeout, ResultProofTimeout, ResultCleanupTimeout, ResultCleanupFailure, ResultLimitExceeded, ResultConsentDeclined, ResultArtifactFailure, ResultJavaFailure, ResultUploadFailure, ResultLaunchFailure, ResultSessionFailure, ResultProofFailure:
		return true
	}
	return false
}

var (
	ErrPromptUnavailable     = errors.New("prompt unavailable")
	ErrPromptDenied          = errors.New("prompt denied")
	ErrKeyringUnavailable    = errors.New("keyring unavailable")
	ErrKeyringDenied         = errors.New("keyring denied")
	ErrCredentialNotFound    = errors.New("credential not found")
	ErrInvalidCredentialMode = errors.New("invalid credential mode")
)

func ClassifyCredentialFailure(err error) ResultClass {
	switch err {
	case ErrPromptUnavailable, ErrPromptDenied, ErrKeyringUnavailable, ErrKeyringDenied, ErrCredentialNotFound, ErrInvalidCredentialMode:
		return ResultCredentialsUnavailable
	}
	return ""
}

// TerminalResultForObservation converts only known pre-auth terminal reasons
// into their public result class. Unknown values block downgrade.
func TerminalResultForObservation(reason Step8Reason) ResultClass {
	switch reason {
	case ReasonIdentityFailure:
		return ResultIdentityFailure
	case ReasonProtocolFailure:
		return ResultProtocolFailure
	case ReasonMalformedResponse:
		return ResultMalformedResponse
	case ReasonDowngradeBlocked:
		return ResultDowngradeBlocked
	case ReasonCancelled:
		return ResultCancelled
	case ReasonOperationTimeout:
		return ResultOperationTimeout
	case ReasonLimitExceeded:
		return ResultLimitExceeded
	case ReasonCredentialsUnavailable:
		return ResultCredentialsUnavailable
	default:
		return ResultDowngradeBlocked
	}
}

func ValidateCredentialMode(mode profile.CredentialMode) error {
	if mode != profile.CredentialModePrompt && mode != profile.CredentialModeKeyring {
		return ErrInvalidCredentialMode
	}
	return nil
}

type Step8Request struct {
	RequestID      string
	Profile        profile.Profile
	Generation     uint64
	WSSConsent     bool
	SSHConsent     bool
	FallbackTicket string
	FallbackClass  Step8Reason
	// Consent is retained for direct gate callers. Step8Service never uses it
	// to infer either WSS or SSH consent.
	Consent bool
}

const fallbackTicketLifetime = 5 * time.Minute

type fallbackTicketClaim struct {
	Profile    string
	RequestID  string
	Generation uint64
	Class      Step8Reason
}

type fallbackTicketRecord struct {
	claim   fallbackTicketClaim
	expires time.Time
}

// fallbackTicketStore keeps only digests, never capability values. Its mutex
// makes consume a single atomic admission point for SSH fallback.
type fallbackTicketStore struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[[sha256.Size]byte]fallbackTicketRecord
	latest  map[string]uint64
}

func newFallbackTicketStore(now func() time.Time) *fallbackTicketStore {
	if now == nil {
		now = time.Now
	}
	return &fallbackTicketStore{now: now, records: make(map[[sha256.Size]byte]fallbackTicketRecord), latest: make(map[string]uint64)}
}

func (s *fallbackTicketStore) issue(claim fallbackTicketClaim) (string, error) {
	if s == nil || !validFallbackTicketClaim(claim) {
		return "", errors.New("invalid fallback ticket claim")
	}
	capability := make([]byte, 24)
	if _, err := rand.Read(capability); err != nil {
		return "", errors.New("fallback ticket unavailable")
	}
	ticket := base64.RawURLEncoding.EncodeToString(capability)
	digest := sha256.Sum256([]byte(ticket))
	s.mu.Lock()
	defer s.mu.Unlock()
	if latest := s.latest[claim.Profile]; claim.Generation < latest {
		return "", errors.New("superseded fallback ticket claim")
	}
	s.latest[claim.Profile] = claim.Generation
	for key, record := range s.records {
		if record.claim.Profile == claim.Profile && record.claim.Generation <= claim.Generation {
			delete(s.records, key)
		}
	}
	s.records[digest] = fallbackTicketRecord{claim: claim, expires: s.now().Add(fallbackTicketLifetime)}
	return ticket, nil
}

func (s *fallbackTicketStore) consume(ticket string, claim fallbackTicketClaim) error {
	if s == nil || !validFallbackTicketClaim(claim) || ticket == "" {
		return errors.New("invalid fallback ticket")
	}
	digest := sha256.Sum256([]byte(ticket))
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[digest]
	if !ok || !s.now().Before(record.expires) || record.claim != claim {
		return errors.New("fallback ticket rejected")
	}
	delete(s.records, digest)
	return nil
}

func (s *fallbackTicketStore) cancel(claim fallbackTicketClaim) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.records {
		if record.claim == claim {
			delete(s.records, key)
		}
	}
}

func (s *fallbackTicketStore) supersede(profileName string, generation uint64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.records {
		if record.claim.Profile == profileName && record.claim.Generation < generation {
			delete(s.records, key)
		}
	}
	if s.latest[profileName] < generation {
		s.latest[profileName] = generation
	}
}

func validFallbackTicketClaim(claim fallbackTicketClaim) bool {
	return claim.Profile != "" && claim.RequestID != "" && claim.Generation > 0 && DecisionForReason(claim.Class) == DecisionSSHEligible
}

// Observation is the typed, credential-free WSS outcome that precedes all
// fallback consideration.
type Observation struct {
	Decision Decision
	Reason   Step8Reason
}

// SSHPolicy authorizes managed SSH fallback without exposing a transport.
type SSHPolicy interface {
	AllowSSH(context.Context, profile.Profile) error
}

// SSHTrust verifies independent persisted SSH trust without reusing WSS trust.
type SSHTrust interface {
	VerifySSH(context.Context, profile.Profile) error
}

// CredentialProvider retrieves one opaque credential at the final gate.
type CredentialProvider interface {
	Get(context.Context, string, profile.CredentialMode) ([]byte, error)
}

// PostObservationGate stops unsafe fallback before any SSH runtime exists.
// Runtime acquisition remains a later-slice responsibility.
type PostObservationGate struct {
	Policy      SSHPolicy
	Trust       SSHTrust
	Credentials CredentialProvider
}

func (g PostObservationGate) Apply(ctx context.Context, request Step8Request, observation Observation) Step8Result {
	return g.ApplyWithCredential(ctx, request, observation, func([]byte) Step8Result {
		return Step8Result{RequestID: request.RequestID, Decision: DecisionSSHEligible, Class: ResultProofSuccess}
	})
}

// ApplyWithCredential retains the sole opaque credential reference inside the
// configuration/runtime boundary until its caller has settled the SSH proof.
func (g PostObservationGate) ApplyWithCredential(ctx context.Context, request Step8Request, observation Observation, run func([]byte) Step8Result) Step8Result {
	result := Step8Result{RequestID: request.RequestID}
	if request.RequestID == "" || ValidateStep8Profile(request.Profile) != nil {
		return terminalGateResult(result, ResultDowngradeBlocked)
	}
	if err := ctx.Err(); err != nil {
		return terminalGateResult(result, ResultCancelled)
	}
	if observation.Decision != DecisionForReason(observation.Reason) {
		return terminalGateResult(result, ResultDowngradeBlocked)
	}
	switch observation.Decision {
	case DecisionWSSSelected:
		return Step8Result{RequestID: request.RequestID, Decision: DecisionWSSSelected, Class: ResultProofSuccess}
	case DecisionTerminal:
		return terminalGateResult(result, TerminalResultForObservation(observation.Reason))
	case DecisionSSHEligible:
		// Continue through every independent gate below.
	default:
		return terminalGateResult(result, ResultDowngradeBlocked)
	}
	if g.Policy == nil || g.Trust == nil || g.Credentials == nil || run == nil {
		return terminalGateResult(result, ResultDowngradeBlocked)
	}
	if err := g.Policy.AllowSSH(ctx, request.Profile); err != nil {
		return terminalGateResult(result, gateContextResult(ctx, ResultAuthorizationDenied))
	}
	if err := g.Trust.VerifySSH(ctx, request.Profile); err != nil {
		return terminalGateResult(result, gateContextResult(ctx, ResultTrustMismatch))
	}
	if !request.Consent {
		return terminalGateResult(result, ResultConsentDeclined)
	}
	key := "ibmi/" + request.Profile.Name
	credential, err := g.Credentials.Get(ctx, key, request.Profile.CredentialMode)
	defer zeroCredential(credential)
	if err != nil || len(credential) == 0 {
		return terminalGateResult(result, gateContextResult(ctx, ResultCredentialsUnavailable))
	}
	return run(credential)
}

func zeroCredential(credential []byte) {
	for i := range credential {
		credential[i] = 0
	}
}

func terminalGateResult(result Step8Result, class ResultClass) Step8Result {
	result.Decision = DecisionTerminal
	result.Class = class
	return result
}

func gateContextResult(ctx context.Context, fallback ResultClass) ResultClass {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ResultCancelled
	}
	return fallback
}

type Step8Result struct {
	RequestID      string
	Decision       Decision
	Class          ResultClass
	ProofRevision  string
	Outcome        string
	Cleanup        bool
	FallbackTicket string
	FallbackClass  Step8Reason
}
type Step8Runner interface {
	Run(context.Context, Step8Request) Step8Result
}

// Step8ProofService is the application-owned service boundary. Transport
// implementations are supplied by later slices behind Step8Runner.
type Step8ProofService struct{ Runner Step8Runner }

func (r Step8Result) Validate() error {
	if r.Decision == DecisionSSHEligible || r.Decision == DecisionWSSSelected {
		if r.Class != ResultProofSuccess {
			return errors.New("non-terminal decision has terminal result")
		}
		return nil
	}
	if r.Decision != DecisionTerminal || !IsTerminalResult(r.Class) {
		return errors.New("invalid terminal result")
	}
	return nil
}
func ValidateStep8Profile(p profile.Profile) error {
	if p.SchemaVersion != profile.SchemaVersionV3 || p.Name == "" {
		return errors.New("step8 requires a saved schema-v3 profile")
	}
	return p.Validate()
}

const ProofRevision = "values-1-v1"

type ProofMetadata struct {
	Rows          int
	ProofRevision string
}

func ValidateProofMetadata(m ProofMetadata) error { return validateProofMetadata(m) }
func validateProofMetadata(m ProofMetadata) error {
	if m.Rows != 1 || m.ProofRevision != ProofRevision {
		return errors.New("invalid fixed proof metadata")
	}
	return nil
}

const MarkerSchemaVersion = 1

type Marker struct {
	SchemaVersion int         `json:"schemaVersion"`
	AtUnixMs      int64       `json:"atUnixMs"`
	Outcome       ResultClass `json:"outcome"`
	ProofRevision string      `json:"proofRevision"`
}
type ConfigChange uint8

const (
	ConfigUnchanged ConfigChange = iota
	ConfigEndpointChanged
	ConfigPolicyChanged
	ConfigTrustChanged
)

func ValidateMarker(m Marker) error {
	if m.SchemaVersion != MarkerSchemaVersion || m.AtUnixMs <= 0 || m.AtUnixMs > time.Now().Add(5*time.Minute).UnixMilli() || m.Outcome == "" || m.ProofRevision != ProofRevision {
		return errors.New("invalid proof marker")
	}
	return nil
}
func MarkerValid(m Marker, change ConfigChange) bool {
	return change == ConfigUnchanged && ValidateMarker(m) == nil
}
func MarkerIsReadiness(Marker) bool { return false }
