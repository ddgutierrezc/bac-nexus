// Package inspection defines the narrow, connector-neutral program metadata
// use case used by the Companion MCP surface.
package inspection

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// SelectionTTL bounds the lifetime of a resolved-object selection.
	SelectionTTL  = 5 * time.Minute
	MaxSelections = 256
)

var identifier = regexp.MustCompile(`^[A-Z@$#][A-Z0-9@$#]{0,9}$`)

var errInvalidIdentifier = errors.New("invalid IBM i system identifier")
var ErrSelectionCapacity = errors.New("selection capacity exceeded")

// State is the stable outcome classification for program metadata operations.
type State string

const (
	StateResolved    State = "resolved"
	StateAmbiguous   State = "ambiguous"
	StateNotFound    State = "not_found"
	StateTruncated   State = "truncated"
	StateUnavailable State = "unavailable"
)

// Provenance identifies how a concrete program match was located.
type Provenance string

const (
	ProvenanceExplicitLibrary        Provenance = "explicit_library"
	ProvenanceCodeForICurrentLibrary Provenance = "code_for_i_current_library"
	ProvenanceCodeForIConfiguredList Provenance = "code_for_i_configured_library_list"
)

// ResolveRequest is a normalized program lookup request.
type ResolveRequest struct {
	Name    string `json:"name"`
	Library string `json:"library,omitempty"`
}

// NewResolveRequest validates and normalizes IBM i system identifiers.
func NewResolveRequest(name, library string) (ResolveRequest, error) {
	name, err := normalizeIdentifier(name)
	if err != nil {
		return ResolveRequest{}, err
	}
	if library == "" {
		return ResolveRequest{Name: name}, nil
	}
	library, err = normalizeIdentifier(library)
	if err != nil {
		return ResolveRequest{}, err
	}
	return ResolveRequest{Name: name, Library: library}, nil
}

func normalizeIdentifier(value string) (string, error) {
	normalized := strings.ToUpper(value)
	if !identifier.MatchString(normalized) {
		return "", errInvalidIdentifier
	}
	return normalized, nil
}

// ResolvedProgram is the exact supported object identity. This RC supports
// only *PGM objects.
type ResolvedProgram struct {
	Library       string     `json:"library"`
	Name          string     `json:"name"`
	ObjectType    string     `json:"objectType"`
	Provenance    Provenance `json:"provenance"`
	MatchPosition int        `json:"matchPosition"`
}

// Decision tells an agent which bounded user choice is required.
type Decision struct {
	Kind    string            `json:"kind"`
	Options []ResolvedProgram `json:"options"`
}

const DecisionSelectProgram = "select_program"

// ResolveResult is the provider result before local selection handles are issued.
type ResolveResult struct {
	State               State             `json:"state"`
	SearchStrategy      string            `json:"searchStrategy"`
	LibrariesSearched   []string          `json:"librariesSearched"`
	Matches             []ResolvedProgram `json:"matches"`
	Completeness        string            `json:"completeness"`
	Truncated           bool              `json:"truncated"`
	RuntimeLiblVerified bool              `json:"runtimeLiblVerified"`
	Reason              string            `json:"reason,omitempty"`
	RequiredDecision    *Decision         `json:"decision,omitempty"`
}

// SourceResult carries source coordinates only when independently evidenced.
type SourceResult struct {
	State               State  `json:"state"`
	Reason              string `json:"reason,omitempty"`
	NextStep            string `json:"nextStep,omitempty"`
	Certainty           string `json:"certainty"`
	Completeness        string `json:"completeness"`
	Truncated           bool   `json:"truncated"`
	RuntimeLiblVerified bool   `json:"runtimeLiblVerified"`
	SourceLibrary       string `json:"sourceLibrary,omitempty"`
	SourceFile          string `json:"sourceFile,omitempty"`
	SourceMember        string `json:"sourceMember,omitempty"`
}

// Provider is the connector-neutral remote program inspection port.
type Provider interface {
	ResolveProgram(context.Context, ResolveRequest) ResolveResult
	FindProgramSource(context.Context, ResolvedProgram) SourceResult
}

// Service issues local opaque bindings over provider results.
type Service struct {
	provider   Provider
	selections *SelectionStore
}

func NewService(provider Provider, selections *SelectionStore) *Service {
	return &Service{provider: provider, selections: selections}
}

// Resolve obtains metadata and issues an opaque handle only for one exact match.
func (s *Service) Resolve(ctx context.Context, request ResolveRequest) ResolveResult {
	if s == nil || s.provider == nil || s.selections == nil || ctx.Err() != nil {
		return unavailableResolve()
	}
	result := s.provider.ResolveProgram(ctx, request)
	if result.RuntimeLiblVerified {
		result.RuntimeLiblVerified = false
	}
	if result.State == StateAmbiguous {
		result.RequiredDecision = &Decision{Kind: DecisionSelectProgram, Options: result.Matches}
	}
	return result
}

// FindSource validates the opaque resolved selection before contacting a provider.
func (s *Service) FindSource(ctx context.Context, selection string) SourceResult {
	if s == nil || s.provider == nil || s.selections == nil || ctx.Err() != nil {
		return unavailableSource()
	}
	program, err := s.selections.Lookup(selection)
	if err != nil {
		return SourceResult{State: StateUnavailable, Reason: "invalid_or_expired_selection", NextStep: "resolve_program", Certainty: "unavailable", Completeness: "complete", RuntimeLiblVerified: false}
	}
	result := s.provider.FindProgramSource(ctx, program)
	result.RuntimeLiblVerified = false
	return result
}

// IssueSelection binds one already-resolved program to this process.
func (s *Service) IssueSelection(program ResolvedProgram) (string, error) {
	if s == nil || s.selections == nil {
		return "", errors.New("selection store unavailable")
	}
	return s.selections.Issue(program)
}

func unavailableResolve() ResolveResult {
	return ResolveResult{State: StateUnavailable, Completeness: "complete", RuntimeLiblVerified: false}
}

func unavailableSource() SourceResult {
	return SourceResult{State: StateUnavailable, Reason: "companion_unavailable", Certainty: "unavailable", Completeness: "complete", RuntimeLiblVerified: false}
}

type selectionRecord struct {
	program ResolvedProgram
	expires time.Time
}

// SelectionStore is a bounded server-side registry. Handles do not encode the
// object identity and are valid only in the Nexus process that issued them.
// Lookups are replayable until TTL expiry because read-only MCP calls may retry.
type SelectionStore struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]selectionRecord
}

func NewSelectionStore() *SelectionStore { return newSelectionStore(time.Now) }

func NewSelectionStoreForTest(now func() time.Time, _ string) *SelectionStore {
	return newSelectionStore(now)
}

func newSelectionStore(now func() time.Time) *SelectionStore {
	return &SelectionStore{now: now, entries: make(map[string]selectionRecord)}
}

func (s *SelectionStore) Issue(program ResolvedProgram) (string, error) {
	if program.ObjectType != "*PGM" || !identifier.MatchString(program.Library) || !identifier.MatchString(program.Name) {
		return "", errInvalidIdentifier
	}
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	handle := base64.RawURLEncoding.EncodeToString(bytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked()
	if len(s.entries) >= MaxSelections {
		return "", ErrSelectionCapacity
	}
	s.entries[handle] = selectionRecord{program: program, expires: s.now().Add(SelectionTTL)}
	return handle, nil
}

func (s *SelectionStore) Lookup(handle string) (ResolvedProgram, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked()
	record, ok := s.entries[handle]
	if !ok || !s.now().Before(record.expires) {
		delete(s.entries, handle)
		return ResolvedProgram{}, errors.New("invalid or expired selection")
	}
	return record.program, nil
}

func (s *SelectionStore) purgeLocked() {
	for handle, record := range s.entries {
		if !s.now().Before(record.expires) {
			delete(s.entries, handle)
		}
	}
}
