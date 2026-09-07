package inspection

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResolveRequestNormalizesAndBoundsIdentifiers(t *testing.T) {
	request, err := NewResolveRequest("pisa061", "prodlib")
	if err != nil {
		t.Fatalf("NewResolveRequest() error = %v", err)
	}
	if request.Name != "PISA061" || request.Library != "PRODLIB" {
		t.Fatalf("request = %#v, want uppercase identifiers", request)
	}

	for _, input := range []string{"", "TOO-LONG-NAME", "PISA 061", "PISA061;"} {
		if _, err := NewResolveRequest(input, ""); err == nil {
			t.Fatalf("NewResolveRequest(%q) succeeded, want validation failure", input)
		}
	}
}

func TestSelectionStoreBindsAndExpiresSelection(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	store := NewSelectionStoreForTest(func() time.Time { return now }, "test-secret")
	resolved := ResolvedProgram{Library: "PRODLIB", Name: "PISA061", ObjectType: "*PGM", MatchPosition: 1}
	handle, err := store.Issue(resolved)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if got, err := store.Lookup(handle); err != nil || got != resolved {
		t.Fatalf("Lookup() = %#v, %v; want %#v, nil", got, err, resolved)
	}
	if _, err := store.Lookup(handle); err != nil {
		t.Fatalf("Lookup() rejected retryable selection: %v", err)
	}
	if _, err := store.Lookup(handle + "forged"); err == nil {
		t.Fatal("Lookup() accepted forged selection")
	}

	now = now.Add(SelectionTTL + time.Second)
	if _, err := store.Lookup(handle); err == nil {
		t.Fatal("Lookup() accepted expired selection")
	}
}

func TestSelectionStoreFailsClosedAtCapacity(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	store := NewSelectionStoreForTest(func() time.Time { return now }, "test-secret")
	program := ResolvedProgram{Library: "PRODLIB", Name: "PISA061", ObjectType: "*PGM"}
	for range MaxSelections {
		if _, err := store.Issue(program); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Issue(program); !errors.Is(err, ErrSelectionCapacity) {
		t.Fatalf("Issue() error = %v, want capacity", err)
	}
}

func TestServiceSearchesConfiguredLibrariesInOrderAndRequiresDecision(t *testing.T) {
	provider := fakeProvider{resolve: func(_ context.Context, request ResolveRequest) ResolveResult {
		if request.Library != "" {
			return ResolveResult{State: StateResolved, Matches: []ResolvedProgram{{Library: request.Library, Name: request.Name, ObjectType: "*PGM", MatchPosition: 1}}}
		}
		return ResolveResult{State: StateAmbiguous, LibrariesSearched: []string{"CURLIB", "LIBA"}, Matches: []ResolvedProgram{{Library: "CURLIB", Name: request.Name, ObjectType: "*PGM", MatchPosition: 1}, {Library: "LIBA", Name: request.Name, ObjectType: "*PGM", MatchPosition: 2}}}
	}}
	service := NewService(provider, NewSelectionStoreForTest(func() time.Time { return time.Unix(1_000, 0) }, "test-secret"))

	result := service.Resolve(context.Background(), ResolveRequest{Name: "PISA061"})
	if result.State != StateAmbiguous || result.RequiredDecision == nil || result.RequiredDecision.Kind != DecisionSelectProgram {
		t.Fatalf("Resolve() = %#v, want ambiguous select-program decision", result)
	}
	if result.RuntimeLiblVerified {
		t.Fatal("Resolve() claimed runtime library-list verification")
	}
}

type fakeProvider struct {
	resolve func(context.Context, ResolveRequest) ResolveResult
}

func (f fakeProvider) ResolveProgram(ctx context.Context, request ResolveRequest) ResolveResult {
	return f.resolve(ctx, request)
}

func (f fakeProvider) FindProgramSource(context.Context, ResolvedProgram) SourceResult {
	return SourceResult{State: StateUnavailable, Reason: "source_metadata_unsupported"}
}
