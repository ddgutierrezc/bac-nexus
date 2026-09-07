package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

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
