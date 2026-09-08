package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
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
	want := []string{"session_status", "sql_query", "resolve_program", "find_program_source"}
	if got := server.ToolNames(); !slices.Equal(got, want) {
		t.Fatalf("ToolNames() = %v, want %v", got, want)
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
