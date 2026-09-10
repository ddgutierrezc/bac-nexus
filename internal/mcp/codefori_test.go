package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/codefori"
	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
	"bac-nexus/internal/source"
)

type companionProviderStub struct {
	status      provider.SessionStatusResult
	query       provider.QueryResult
	statusCalls int
	queryCalls  int
}

func (s *companionProviderStub) SessionStatus(context.Context) provider.SessionStatusResult {
	s.statusCalls++
	return s.status
}

func (s *companionProviderStub) Query(context.Context, provider.QueryRequest) provider.QueryResult {
	s.queryCalls++
	return s.query
}

type companionInspectorStub struct{ resolve inspection.ResolveResult }

type companionCatalogStub struct {
	candidates []catalog.Candidate
	err        error
	calls      int
	resolve    func(context.Context, catalog.Search) ([]catalog.Candidate, error)
}

type companionSourceStub struct {
	page     func(context.Context, codefori.SourcePageRequest) (codefori.SourcePageResult, error)
	dispose  func(context.Context, string) (codefori.SourceDisposeResult, error)
	requests []codefori.SourcePageRequest
	cursors  []string
}

func (s *companionSourceStub) PageSource(ctx context.Context, request codefori.SourcePageRequest) (codefori.SourcePageResult, error) {
	s.requests = append(s.requests, request)
	if s.page != nil {
		return s.page(ctx, request)
	}
	return sourceResult("line  ", 1, 1, false), nil
}
func (s *companionSourceStub) DisposeSource(ctx context.Context, cursor string) (codefori.SourceDisposeResult, error) {
	s.cursors = append(s.cursors, cursor)
	if s.dispose != nil {
		return s.dispose(ctx, cursor)
	}
	return codefori.SourceDisposeResult{State: codefori.SourceDisposed}, nil
}

func (s *companionCatalogStub) ResolveCatalog(ctx context.Context, search catalog.Search) ([]catalog.Candidate, error) {
	s.calls++
	if s.resolve != nil {
		return s.resolve(ctx, search)
	}
	return s.candidates, s.err
}

func (s companionInspectorStub) ResolveProgram(context.Context, inspection.ResolveRequest) inspection.ResolveResult {
	return s.resolve
}

func (companionInspectorStub) FindProgramSource(context.Context, inspection.ResolvedProgram) inspection.SourceResult {
	return inspection.SourceResult{}
}

func TestCodeForIServerRegistersExactlyCompanionTools(t *testing.T) {
	server, err := NewCodeForI(CodeForIConfig{Provider: &companionProviderStub{}})
	if err != nil {
		t.Fatalf("NewCodeForI() error = %v", err)
	}
	want := []string{"session_status", "sql_query", "resolve_program", "find_program_source", "resolve_catalog_candidates", "read_selected_source"}
	if got := server.ToolNames(); !slices.Equal(got, want) {
		t.Fatalf("ToolNames() = %v, want %v", got, want)
	}
}

func TestCodeForIServerCatalogToolSDKBoundary(t *testing.T) {
	first := catalog.Candidate{Item: "PISA061", SourceLibrary: "SRCLIB", SourceFileBase: "QRPG", ObjectType: "M", SourceType: "RPGLE", Application: "PAYMENTS", Version: "V1", ProductionLibrary: "PRODLIB", Description: "first candidate"}
	second := catalog.Candidate{Item: "PISA061", SourceLibrary: "OTHERLIB", SourceFileBase: "QSRC", ObjectType: "M", SourceType: "CLLE", Application: "LEDGER", Version: "V2", ProductionLibrary: "TESTLIB", Description: "second candidate"}
	for _, tt := range []struct {
		name           string
		catalog        *companionCatalogStub
		input          ResolveCatalogInput
		wantCandidates []catalog.Candidate
		wantError      string
		wantCalls      int
	}{
		{"ordered success", &companionCatalogStub{candidates: []catalog.Candidate{first, second}}, ResolveCatalogInput{Item: "PISA061", ProductionLibrary: "PRODLIB"}, []catalog.Candidate{first, second}, "", 1},
		{"not found", &companionCatalogStub{err: catalog.ErrCandidateNotFound}, ResolveCatalogInput{Item: "PISA061"}, nil, catalog.ErrCandidateNotFound.Error(), 1},
		{"limit", &companionCatalogStub{err: catalog.ErrCandidateLimit}, ResolveCatalogInput{Item: "PISA061"}, nil, catalog.ErrCandidateLimit.Error(), 1},
		{"unavailable", &companionCatalogStub{err: codefori.ErrCatalogUnavailable}, ResolveCatalogInput{Item: "PISA061"}, nil, codefori.ErrCatalogUnavailable.Error(), 1},
		{"timeout", &companionCatalogStub{err: context.DeadlineExceeded}, ResolveCatalogInput{Item: "PISA061"}, nil, context.DeadlineExceeded.Error(), 1},
		{"cancelled", &companionCatalogStub{err: context.Canceled}, ResolveCatalogInput{Item: "PISA061"}, nil, context.Canceled.Error(), 1},
		{"failed", &companionCatalogStub{err: codefori.ErrCatalogFailed}, ResolveCatalogInput{Item: "PISA061"}, nil, codefori.ErrCatalogFailed.Error(), 1},
		// The provider interface reports malformed and oversized Companion responses as failed.
		{"malformed provider result", &companionCatalogStub{err: errors.New("raw host token source sql path")}, ResolveCatalogInput{Item: "PISA061"}, nil, codefori.ErrCatalogFailed.Error(), 1},
		{"oversized provider result", &companionCatalogStub{err: codefori.ErrCatalogFailed}, ResolveCatalogInput{Item: "PISA061"}, nil, codefori.ErrCatalogFailed.Error(), 1},
		{"invalid input", &companionCatalogStub{}, ResolveCatalogInput{Item: "invalid item"}, nil, `invalid catalog item "INVALID ITEM"`, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Catalog: tt.catalog})
			defer closeSession()
			result := callCodeForITool(t, session, "resolve_catalog_candidates", tt.input)
			if tt.catalog.calls != tt.wantCalls {
				t.Fatalf("catalog calls = %d, want %d", tt.catalog.calls, tt.wantCalls)
			}
			if tt.wantError != "" {
				if got := codeForICatalogErrorText(t, result); got != tt.wantError {
					t.Fatalf("catalog MCP error = %q, want %q", got, tt.wantError)
				}
				encoded, _ := json.Marshal(result)
				if strings.Contains(string(encoded), "raw host token source sql path") {
					t.Fatalf("catalog error leaked: %s", encoded)
				}
				return
			}
			if result.IsError {
				t.Fatalf("successful catalog result is an MCP error: %#v", result)
			}
			output := structuredCodeForIJSON(t, result)
			assertCodeForIJSONArray(t, output, "candidates", false)
			if string(output["candidates"]) == "null" {
				t.Fatalf("candidates = %s", output["candidates"])
			}
			var candidates []catalog.Candidate
			if err := json.Unmarshal(output["candidates"], &candidates); err != nil {
				t.Fatalf("unmarshal candidates: %v", err)
			}
			if !slices.Equal(candidates, tt.wantCandidates) {
				t.Fatalf("catalog candidates = %#v, want %#v", candidates, tt.wantCandidates)
			}
		})
	}
}

func TestCodeForIServerCatalogToolSchema(t *testing.T) {
	session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Catalog: &companionCatalogStub{}})
	defer closeSession()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	found, foundSource := false, false
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Name != "resolve_catalog_candidates" {
			if tool.Name != "read_selected_source" {
				continue
			}
			input, output := mustSchema(t, tool.InputSchema), mustSchema(t, tool.OutputSchema)
			for _, forbidden := range []string{"path", "uri", "command", "sql", "ssh", "credential", "filesystem", "transport"} {
				if _, found := input[forbidden]; found {
					t.Fatalf("source input exposes %q", forbidden)
				}
			}
			if _, found := input["selection"]; !found {
				t.Fatal("source input has no selection")
			}
			if _, found := output["page"]; !found {
				t.Fatal("source output has no page")
			}
			foundSource = true
			continue
		}
		input, output := mustSchema(t, tool.InputSchema), mustSchema(t, tool.OutputSchema)
		_, hasLibrary := input["productionLibrary"]
		_, hasCandidates := output["candidates"]
		if !hasLibrary || !hasCandidates {
			t.Fatalf("catalog schemas = %#v %#v", input, output)
		}
		found = true
	}
	if !found {
		t.Fatal("resolve_catalog_candidates was not listed through MCP")
	}
	if !foundSource {
		t.Fatal("read_selected_source was not listed through MCP")
	}
	want := []string{"find_program_source", "read_selected_source", "resolve_catalog_candidates", "resolve_program", "session_status", "sql_query"}
	if !slices.Equal(names, want) {
		t.Fatalf("MCP tool inventory = %v, want %v", names, want)
	}
}

func TestCodeForIServerCatalogRequestCancelsWithServerShutdown(t *testing.T) {
	started, completed := make(chan struct{}), make(chan struct{})
	resolver := &companionCatalogStub{resolve: func(ctx context.Context, _ catalog.Search) ([]catalog.Candidate, error) {
		close(started)
		<-ctx.Done()
		close(completed)
		return nil, ctx.Err()
	}}
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	server, err := NewCodeForI(CodeForIConfig{Catalog: resolver, Transport: serverTransport})
	if err != nil {
		t.Fatal(err)
	}
	serverContext, stopServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Run(serverContext) }()
	client := sdk.NewClient(&sdk.Implementation{Name: "mcp-test-client", Version: "v0.0.0"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		stopServer()
		t.Fatal(err)
	}
	callDone := make(chan struct{})
	go func() {
		_, _ = session.CallTool(context.Background(), &sdk.CallToolParams{Name: "resolve_catalog_candidates", Arguments: map[string]any{"item": "PISA061"}})
		close(callDone)
	}()
	<-started
	stopServer()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("catalog resolver did not receive server cancellation")
	}
	select {
	case <-callDone:
	case <-time.After(time.Second):
		t.Fatal("catalog SDK call did not finish after shutdown")
	}
	_ = session.Close()
	if err := <-serverDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("server Run error = %v", err)
	}
}

func TestCodeForIServerReturnsUnavailableWithoutNativeFallback(t *testing.T) {
	source := &companionProviderStub{
		status: provider.SessionStatusResult{State: provider.SessionCompanionUnavailable},
		query:  provider.QueryResult{State: provider.QueryUnavailable},
	}
	session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Provider: source})
	defer closeSession()

	statusResult := callCodeForITool(t, session, "session_status", map[string]any{})
	if statusResult.IsError {
		t.Fatalf("session_status returned MCP error: %#v", statusResult)
	}
	var status CodeForISessionStatusOutput
	decodeCodeForIStructured(t, statusResult, &status)
	if status.State != provider.SessionCompanionUnavailable {
		t.Fatalf("session_status state = %q, want %q", status.State, provider.SessionCompanionUnavailable)
	}

	queryResult := callCodeForITool(t, session, "sql_query", map[string]any{"sql": provider.CanonicalProofQuery})
	if queryResult.IsError {
		t.Fatalf("sql_query returned MCP error: %#v", queryResult)
	}
	var query CodeForIQueryOutput
	decodeCodeForIStructured(t, queryResult, &query)
	if query.State != provider.QueryUnavailable || len(query.Rows) != 0 {
		t.Fatalf("sql_query = %#v, want unavailable without rows", query)
	}
	if source.statusCalls != 1 || source.queryCalls != 1 {
		t.Fatalf("Companion provider calls = status %d, query %d; want one each", source.statusCalls, source.queryCalls)
	}
}

func TestCodeForIServerPreservesCanonicalQueryBoundary(t *testing.T) {
	source := &companionProviderStub{query: provider.QueryResult{
		State: provider.QueryOK,
		Rows:  []provider.NormalizedRow{{Value: "NEXUSUSR"}},
	}}
	session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Provider: source})
	defer closeSession()

	result := callCodeForITool(t, session, "sql_query", map[string]any{"sql": "\tselect CURRENT_USER\r\nfrom sysibm.sysdummy1 "})
	if result.IsError {
		t.Fatalf("sql_query returned MCP error: %#v", result)
	}
	var output CodeForIQueryOutput
	decodeCodeForIStructured(t, result, &output)
	if output.State != provider.QueryOK || len(output.Rows) != 1 || output.Rows[0].Value != "NEXUSUSR" {
		t.Fatalf("sql_query output = %#v, want normalized success", output)
	}
	if source.queryCalls != 1 {
		t.Fatalf("Companion provider query calls = %d, want 1", source.queryCalls)
	}
}

func TestCodeForIServerRejectsNonEmptyStatusInput(t *testing.T) {
	session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Provider: &companionProviderStub{}})
	defer closeSession()

	_, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "session_status", Arguments: map[string]any{"unexpected": true}})
	if err == nil {
		t.Fatal("session_status accepted non-empty input")
	}
}

func TestCodeForIServerResolveProgramSerializesRequiredCollections(t *testing.T) {
	match := inspection.ResolvedProgram{Library: "PRODLIB", Name: "PISA061", ObjectType: "*PGM", MatchPosition: 1}
	tests := []struct {
		name               string
		inspector          inspection.Provider
		wantState          inspection.State
		wantMatches        int
		wantSelection      bool
		wantDecision       bool
		wantEmptyLibraries bool
		wantEmptyMatches   bool
	}{
		{
			name:               "unavailable fallback uses empty arrays",
			wantState:          inspection.StateUnavailable,
			wantEmptyLibraries: true,
			wantEmptyMatches:   true,
		},
		{
			name: "not found uses empty arrays",
			inspector: companionInspectorStub{resolve: inspection.ResolveResult{
				State: inspection.StateNotFound,
			}},
			wantState:          inspection.StateNotFound,
			wantEmptyLibraries: true,
			wantEmptyMatches:   true,
		},
		{
			name: "resolved preserves one match and issues an opaque selection",
			inspector: companionInspectorStub{resolve: inspection.ResolveResult{
				State:             inspection.StateResolved,
				LibrariesSearched: []string{"PRODLIB"},
				Matches:           []inspection.ResolvedProgram{match},
			}},
			wantState:     inspection.StateResolved,
			wantMatches:   1,
			wantSelection: true,
		},
		{
			name: "ambiguous preserves decision options as arrays",
			inspector: companionInspectorStub{resolve: inspection.ResolveResult{
				State:             inspection.StateAmbiguous,
				LibrariesSearched: []string{"PRODLIB", "TESTLIB"},
				Matches: []inspection.ResolvedProgram{
					match,
					{Library: "TESTLIB", Name: "PISA061", ObjectType: "*PGM", MatchPosition: 2},
				},
			}},
			wantState:    inspection.StateAmbiguous,
			wantMatches:  2,
			wantDecision: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Inspector: tt.inspector})
			defer closeSession()

			result := callCodeForITool(t, session, "resolve_program", ResolveProgramInput{Name: "PISA061"})
			if result.IsError {
				t.Fatalf("resolve_program failed MCP output schema validation: %#v", result)
			}
			output := structuredCodeForIJSON(t, result)
			if got := string(output["state"]); got != `"`+string(tt.wantState)+`"` {
				t.Fatalf("state JSON = %s, want %q", got, tt.wantState)
			}
			assertCodeForIJSONArray(t, output, "librariesSearched", tt.wantEmptyLibraries)
			assertCodeForIJSONArray(t, output, "matches", tt.wantEmptyMatches)
			if got := jsonArrayLength(t, output["matches"]); got != tt.wantMatches {
				t.Fatalf("matches JSON length = %d, want %d", got, tt.wantMatches)
			}
			selection, hasSelection := output["selection"]
			if hasSelection != tt.wantSelection {
				t.Fatalf("selection present = %t, want %t", hasSelection, tt.wantSelection)
			}
			if tt.wantSelection && string(selection) == `""` {
				t.Fatal("selection JSON is empty, want an opaque selection")
			}
			decision, hasDecision := output["decision"]
			if hasDecision != tt.wantDecision {
				t.Fatalf("decision present = %t, want %t", hasDecision, tt.wantDecision)
			}
			if tt.wantDecision {
				var decoded map[string]json.RawMessage
				if err := json.Unmarshal(decision, &decoded); err != nil {
					t.Fatalf("unmarshal decision JSON: %v", err)
				}
				assertCodeForIJSONArray(t, decoded, "options", false)
				if got := jsonArrayLength(t, decoded["options"]); got != tt.wantMatches {
					t.Fatalf("decision options JSON length = %d, want %d", got, tt.wantMatches)
				}
			}
		})
	}
}

func TestCodeForIServerReadSelectedSourceContract(t *testing.T) {
	candidate, cursor := sourceCandidateForMCP(), strings.Repeat("A", 43)
	provider := &companionSourceStub{}
	server, err := NewCodeForI(CodeForIConfig{Source: provider})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		input  companionReadSelectedSourceInput
		result codefori.SourcePageResult
		err    error
		want   error
		calls  int
	}{
		{"first forwards exact candidate", companionReadSelectedSourceInput{Selection: candidate, StartLine: 1, MaxLines: 2}, sourceResult("first  \nsecond", 1, 2, false), nil, nil, 1},
		{"later forwards only cursor", companionReadSelectedSourceInput{Cursor: cursor, StartLine: 3, MaxLines: 1}, sourceResult("third", 3, 1, false), nil, nil, 1},
		{"conflicting selection and cursor stops before IO", companionReadSelectedSourceInput{Selection: candidate, Cursor: cursor, StartLine: 1, MaxLines: 1}, codefori.SourcePageResult{}, nil, source.ErrInvalidRequest, 0},
		{"missing selection stops before IO", companionReadSelectedSourceInput{StartLine: 1, MaxLines: 1}, codefori.SourcePageResult{}, nil, source.ErrInvalidRequest, 0},
		{"invalid ranges stop before IO", companionReadSelectedSourceInput{Selection: candidate, StartLine: 0, MaxLines: 201}, codefori.SourcePageResult{}, nil, source.ErrInvalidRequest, 0},
		{"provider failure is sanitized", companionReadSelectedSourceInput{Selection: candidate, StartLine: 1, MaxLines: 1}, codefori.SourcePageResult{}, errors.New("host token path"), codefori.ErrSourceFailed, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider.requests = nil
			provider.page = func(context.Context, codefori.SourcePageRequest) (codefori.SourcePageResult, error) {
				return tt.result, tt.err
			}
			_, output, got := server.readSelectedSource(context.Background(), nil, tt.input)
			if !errors.Is(got, tt.want) || len(provider.requests) != tt.calls {
				t.Fatalf("error=%v calls=%d", got, len(provider.requests))
			}
			if tt.want == nil {
				request := provider.requests[0]
				if tt.input.Cursor == "" && (request.Candidate == nil || *request.Candidate != candidate || request.Cursor != "") {
					t.Fatalf("first request = %#v", request)
				}
				if tt.input.Cursor != "" && (request.Candidate != nil || request.Cursor != cursor) {
					t.Fatalf("later request = %#v", request)
				}
				if tt.input.Cursor == "" && output.Page.Lines[0] != "first  " {
					t.Fatal("trailing spaces changed")
				}
				if !output.Page.EOF && (output.Page.Cursor != cursor || output.Page.NextStartLine != tt.input.StartLine+output.Page.LineCount) {
					t.Fatalf("next page metadata = %#v", output.Page)
				}
			}
		})
	}
	session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Source: &companionSourceStub{page: func(_ context.Context, request codefori.SourcePageRequest) (codefori.SourcePageResult, error) {
		return sourceResult("later", request.StartLine, 1, false), nil
	}}})
	defer closeSession()
	if result := callCodeForITool(t, session, "read_selected_source", map[string]any{"cursor": cursor, "startLine": 2, "maxLines": 1}); result.IsError {
		t.Fatalf("cursor-only MCP request failed: %#v", result)
	}
}

func TestCodeForIServerReadSelectedSourceStatesAndCleanup(t *testing.T) {
	candidate := sourceCandidateForMCP()
	for _, tt := range []struct {
		name  string
		state codefori.SourceState
		want  error
	}{
		{"invalid request", codefori.SourceInvalidRequest, source.ErrInvalidRequest}, {"not found", codefori.SourceNotFound, catalog.ErrCandidateNotFound},
		{"ambiguous does not select", codefori.SourceAmbiguous, &catalog.AmbiguousError{}}, {"expired", codefori.SourceExpired, source.ErrExpiredLease},
		{"encoding", codefori.SourceInvalidEncoding, source.ErrInvalidSourceEncoding}, {"oversized", codefori.SourceResponseTooLarge, source.ErrResponseTooLarge},
		{"unavailable", codefori.SourceUnavailable, codefori.ErrSourceUnavailable}, {"cleanup failed", codefori.SourceCleanupFailed, codefori.ErrSourceFailed}, {"disposed", codefori.SourceDisposed, codefori.ErrSourceFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := &companionSourceStub{page: func(context.Context, codefori.SourcePageRequest) (codefori.SourcePageResult, error) {
				return codefori.SourcePageResult{State: tt.state}, nil
			}}
			server, _ := NewCodeForI(CodeForIConfig{Source: provider})
			_, output, err := server.readSelectedSource(context.Background(), nil, companionReadSelectedSourceInput{Selection: candidate, StartLine: 1, MaxLines: 1})
			ambiguous := &catalog.AmbiguousError{}
			if (!errors.Is(err, tt.want) && !errors.As(err, &ambiguous)) || output.Page.LineCount != 0 || len(provider.requests) != 1 {
				t.Fatalf("error=%v output=%#v requests=%d", err, output, len(provider.requests))
			}
		})
	}
	for _, tt := range []struct {
		name, content string
		eof           bool
		lineCount     int
		dispose       codefori.SourceDisposeResult
		want          error
	}{
		{"EOF omits cursor after disposal", "last  ", true, 1, codefori.SourceDisposeResult{State: codefori.SourceDisposed}, nil},
		{"invalid conversion disposes and leaks no page", "one", false, 2, codefori.SourceDisposeResult{State: codefori.SourceDisposed}, codefori.ErrSourceFailed},
		{"cleanup failure suppresses EOF page", "last", true, 1, codefori.SourceDisposeResult{State: codefori.SourceExpired}, codefori.ErrSourceFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := &companionSourceStub{page: func(context.Context, codefori.SourcePageRequest) (codefori.SourcePageResult, error) {
				return sourceResult(tt.content, 1, tt.lineCount, tt.eof), nil
			}, dispose: func(context.Context, string) (codefori.SourceDisposeResult, error) { return tt.dispose, nil }}
			server, _ := NewCodeForI(CodeForIConfig{Source: provider})
			_, output, err := server.readSelectedSource(context.Background(), nil, companionReadSelectedSourceInput{Selection: candidate, StartLine: 1, MaxLines: 2})
			if !errors.Is(err, tt.want) || len(provider.cursors) != 1 {
				t.Fatalf("error=%v cursors=%v", err, provider.cursors)
			}
			if tt.want == nil && (output.Page.Cursor != "" || output.Page.NextStartLine != 0 || output.Page.Lines[0] != "last  ") {
				t.Fatalf("EOF page=%#v", output.Page)
			}
			if tt.want != nil && output.Page.LineCount != 0 {
				t.Fatalf("failure leaked page=%#v", output.Page)
			}
		})
	}
}

func TestCodeForIServerDisposesLiveSourceOnShutdown(t *testing.T) {
	cursor := strings.Repeat("A", 43)
	provider := &companionSourceStub{}
	session, closeSession := connectInMemoryCodeForI(t, CodeForIConfig{Source: provider})
	result := callCodeForITool(t, session, "read_selected_source", companionReadSelectedSourceInput{Selection: sourceCandidateForMCP(), StartLine: 1, MaxLines: 1})
	if result.IsError {
		t.Fatalf("source page failed: %#v", result)
	}
	closeSession()
	if !slices.Equal(provider.cursors, []string{cursor}) {
		t.Fatalf("shutdown cursors=%v", provider.cursors)
	}
}

func TestCodeForIServerDisposesEveryLiveSourceAfterFailure(t *testing.T) {
	first, second := strings.Repeat("A", 43), strings.Repeat("B", 43)
	provider := &companionSourceStub{dispose: func(_ context.Context, cursor string) (codefori.SourceDisposeResult, error) {
		if cursor == first {
			return codefori.SourceDisposeResult{State: codefori.SourceExpired}, nil
		}
		return codefori.SourceDisposeResult{State: codefori.SourceDisposed}, nil
	}}
	server, _ := NewCodeForI(CodeForIConfig{Source: provider})
	server.trackCursor(first)
	server.trackCursor(second)
	if err := server.disposeLiveSources(); !errors.Is(err, codefori.ErrSourceFailed) || !slices.Equal(provider.cursors, []string{first, second}) {
		t.Fatalf("cleanup error=%v cursors=%v", err, provider.cursors)
	}
}

func TestCodeForIServerReadSelectedSourceCancellationStopsBeforeProvider(t *testing.T) {
	provider := &companionSourceStub{page: func(context.Context, codefori.SourcePageRequest) (codefori.SourcePageResult, error) {
		t.Fatal("cancelled request reached provider")
		return codefori.SourcePageResult{}, nil
	}}
	server, _ := NewCodeForI(CodeForIConfig{Source: provider})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, output, err := server.readSelectedSource(ctx, nil, companionReadSelectedSourceInput{Selection: sourceCandidateForMCP(), StartLine: 1, MaxLines: 1})
	if !errors.Is(err, context.Canceled) || output.Page.LineCount != 0 || len(provider.requests) != 0 {
		t.Fatalf("error=%v output=%#v requests=%d", err, output, len(provider.requests))
	}
}

func sourceCandidateForMCP() catalog.Candidate {
	return catalog.Candidate{Item: "PISA061", SourceLibrary: "SRCLIB", SourceFileBase: "QRPG", ObjectType: "M", SourceType: "RPGLE", Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB", Description: "description"}
}

func sourceResult(content string, startLine, lineCount int, eof bool) codefori.SourcePageResult {
	return codefori.SourcePageResult{State: codefori.SourceOK, Cursor: strings.Repeat("A", 43), Page: codefori.SourcePage{Content: content, StartLine: startLine, LineCount: lineCount, EOF: eof}}
}

func connectInMemoryCodeForI(t *testing.T, cfg CodeForIConfig) (*sdk.ClientSession, func()) {
	t.Helper()
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	cfg.Transport = serverTransport
	server, err := NewCodeForI(cfg)
	if err != nil {
		t.Fatalf("NewCodeForI() error = %v", err)
	}
	serverContext, stopServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Run(serverContext) }()

	client := sdk.NewClient(&sdk.Implementation{Name: "mcp-test-client", Version: "v0.0.0"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		stopServer()
		t.Fatalf("client Connect error = %v", err)
	}
	return session, func() {
		if err := session.Close(); err != nil {
			t.Errorf("client Close error = %v", err)
		}
		stopServer()
		if err := <-serverDone; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("server Run error = %v", err)
		}
	}
}

func callCodeForITool(t *testing.T, session *sdk.ClientSession, name string, input any) *sdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: input})
	if err != nil {
		t.Fatalf("CallTool(%q) error = %v", name, err)
	}
	return result
}

func decodeCodeForIStructured(t *testing.T, result *sdk.CallToolResult, target any) {
	t.Helper()
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured output error = %v", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		t.Fatalf("unmarshal structured output error = %v", err)
	}
}

func structuredCodeForIJSON(t *testing.T, result *sdk.CallToolResult) map[string]json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured output error = %v", err)
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("unmarshal structured output JSON error = %v", err)
	}
	return output
}

func codeForICatalogErrorText(t *testing.T, result *sdk.CallToolResult) string {
	t.Helper()
	if !result.IsError {
		t.Fatalf("catalog result is not an MCP error: %#v", result)
	}
	encoded, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal catalog MCP content: %v", err)
	}
	var content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(encoded, &content); err != nil {
		t.Fatalf("unmarshal catalog MCP content: %v", err)
	}
	if len(content) != 1 || content[0].Type != "text" || content[0].Text == "" {
		t.Fatalf("catalog MCP content = %s, want one non-empty text error", encoded)
	}
	return content[0].Text
}

func mustSchema(t *testing.T, schema any) map[string]json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Properties
}

func assertCodeForIJSONArray(t *testing.T, output map[string]json.RawMessage, name string, wantEmpty bool) {
	t.Helper()
	raw, ok := output[name]
	if !ok {
		t.Fatalf("structured JSON does not contain %q", name)
	}
	if wantEmpty && string(raw) != "[]" {
		t.Fatalf("%s JSON = %s, want []", name, raw)
	}
	if len(raw) == 0 || raw[0] != '[' {
		t.Fatalf("%s JSON = %s, want an array", name, raw)
	}
}

func jsonArrayLength(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("unmarshal array JSON %s: %v", raw, err)
	}
	return len(values)
}
