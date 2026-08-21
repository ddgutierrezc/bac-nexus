// Package mcp exposes the v1 read-only catalog-context service as a
// typed, stdio Model Context Protocol server. The package owns no
// remote, path, shell, SQL, or SSH capability of its own; it adapts
// internal/app.Service calls to the official MCP wire protocol and
// surfaces only the two allowed tools: resolve_catalog_candidates
// and read_selected_source.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// fakeService is the deterministic, package-local Service double used
// by every handler test. It records the last call and returns the
// configured result.
type fakeService struct {
	resolveFn func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error)
	readFn    func(ctx context.Context, cursor string, page source.Range) (source.Page, error)
	resolveCalls int
	readCalls    int
	lastQuery    catalog.Query
	lastSelector security.Selector
	lastCursor   string
	lastRange    source.Range
}

func (f *fakeService) ResolveCatalog(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
	f.resolveCalls++
	f.lastQuery = query
	f.lastSelector = selector
	if f.resolveFn == nil {
		return nil, nil
	}
	return f.resolveFn(ctx, query, selector)
}

func (f *fakeService) ReadSelectedSource(ctx context.Context, cursor string, page source.Range) (source.Page, error) {
	f.readCalls++
	f.lastCursor = cursor
	f.lastRange = page
	if f.readFn == nil {
		return source.Page{}, nil
	}
	return f.readFn(ctx, cursor, page)
}

// validCandidate returns the canonical catalog candidate used by every
// test. The values are system-name valid and bounded.
func validCandidate() catalog.Candidate {
	return catalog.Candidate{
		Item:              "PISA061",
		SourceLibrary:     "QRPGLESRC",
		SourceFileBase:    "QRPGLESRC",
		ObjectType:        "RPGLE",
		SourceType:        "RPG",
		Application:       "APP",
		Version:           "V1",
		ProductionLibrary: "PRODLIB",
		Description:       "test program",
	}
}

// validConfig returns a Config wired to a fake service so tests can
// override individual fields. It never panics and is always usable.
func validConfig() (Config, *fakeService) {
	svc := &fakeService{}
	return Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}, Service: svc, Profile: "test-profile"}, svc
}

// validBuildInput returns the canonical typed input for the
// resolve_catalog_candidates tool.
func validBuildInput() ResolveCatalogInput {
	return ResolveCatalogInput{Statement: "SELECT * FROM catalog", Parameters: []string{"%PISA061%"}}
}

// validReadInput returns the canonical typed input for the
// read_selected_source tool.
func validReadInput() ReadSelectedSourceInput {
	return ReadSelectedSourceInput{Cursor: "opaque-cursor-token", StartLine: 1, MaxLines: 50}
}

// ---------------------------------------------------------------------------
// Tool registration and surface guard
// ---------------------------------------------------------------------------

// TestServerRegistersExactlyTwoTools proves the MCP server registers
// only the two canonical tools and exposes no other tool to clients.
func TestServerRegistersExactlyTwoTools(t *testing.T) {
	cfg, _ := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	names := srv.ToolNames()
	if len(names) != 2 {
		t.Fatalf("ToolNames len = %d, want 2 (%v)", len(names), names)
	}
	want := []string{"read_selected_source", "resolve_catalog_candidates"}
	if !slices.Equal(names, want) {
		t.Fatalf("ToolNames = %v, want %v", names, want)
	}
}

// TestServerRejectsNilService proves the construction seam refuses a
// nil Service before any tool is registered. The MCP server must
// never register tools that would panic when invoked.
func TestServerRejectsNilService(t *testing.T) {
	if _, err := New(Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}}); err == nil {
		t.Fatal("New with nil service = nil error, want rejection")
	}
}

// TestServerSurfaceHasNoRemotePathOrShellCommands is a structural
// reflection guard: the public types of the mcp package must never
// expose generic remote, path, shell, SQL, or SSH capabilities.
func TestServerSurfaceHasNoRemotePathOrShellCommands(t *testing.T) {
	checks := []struct {
		typ   reflect.Type
		label string
	}{
		{typ: reflect.TypeOf((*Server)(nil)), label: "Server"},
		{typ: reflect.TypeOf((*Service)(nil)).Elem(), label: "Service"},
		{typ: reflect.TypeOf((*Config)(nil)), label: "Config"},
		{typ: reflect.TypeOf(Info{}), label: "Info"},
		{typ: reflect.TypeOf(ResolveCatalogInput{}), label: "ResolveCatalogInput"},
		{typ: reflect.TypeOf(ResolveCatalogOutput{}), label: "ResolveCatalogOutput"},
		{typ: reflect.TypeOf(ReadSelectedSourceInput{}), label: "ReadSelectedSourceInput"},
		{typ: reflect.TypeOf(ReadSelectedSourceOutput{}), label: "ReadSelectedSourceOutput"},
	}
	for _, check := range checks {
		for _, forbidden := range forbiddenSurfaceSubstrings {
			found, name := hasFieldContaining(check.typ, forbidden)
			if found {
				t.Fatalf("%s has forbidden field %q (matched %q)", check.label, name, forbidden)
			}
		}
	}
}

// TestResolveCatalogInputHasNoPathListOrDeleteFields proves the
// typed input schema for resolve_catalog_candidates does not accept
// any temporary, listing, or delete path.
func TestResolveCatalogInputHasNoPathListOrDeleteFields(t *testing.T) {
	typ := reflect.TypeOf(ResolveCatalogInput{})
	for _, forbidden := range []string{"path", "list", "delete", "remove", "command", "shell", "ssh", "exec", "sql", "remote"} {
		if found, name := hasFieldContaining(typ, forbidden); found {
			t.Fatalf("ResolveCatalogInput has forbidden field %q (matched %q)", name, forbidden)
		}
	}
}

// TestReadSelectedSourceInputHasNoPathListOrDeleteFields proves the
// typed input schema for read_selected_source does not accept any
// temporary, listing, or delete path.
func TestReadSelectedSourceInputHasNoPathListOrDeleteFields(t *testing.T) {
	typ := reflect.TypeOf(ReadSelectedSourceInput{})
	for _, forbidden := range []string{"path", "list", "delete", "remove", "command", "shell", "ssh", "exec", "sql", "remote"} {
		if found, name := hasFieldContaining(typ, forbidden); found {
			t.Fatalf("ReadSelectedSourceInput has forbidden field %q (matched %q)", name, forbidden)
		}
	}
}

// forbiddenSurfaceSubstrings is the structural guard list. The list
// is authoritative; adding an entry requires an explicit decision
// and a matching red test.
var forbiddenSurfaceSubstrings = []string{
	"path",
	"command",
	"shell",
	"exec",
	"sql",
	"ssh",
	"dial",
	"connect",
	"remote",
	"clientinfo",
	"parent",
}

// hasFieldContaining returns whether the supplied struct type exposes
// a field whose lower-cased name contains the supplied substring. It
// also returns the matching field name for diagnostics. It recurses
// into anonymous embedded structs so composition is covered.
func hasFieldContaining(typ reflect.Type, substring string) (bool, string) {
	return fieldContains(typ, substring, map[reflect.Type]bool{})
}

func fieldContains(typ reflect.Type, substring string, visited map[reflect.Type]bool) (bool, string) {
	if typ == nil || visited[typ] {
		return false, ""
	}
	visited[typ] = true
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return false, ""
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			if found, name := fieldContains(field.Type, substring, visited); found {
				return true, name
			}
			continue
		}
		if strings.Contains(strings.ToLower(field.Name), substring) {
			return true, field.Name
		}
	}
	return false, ""
}

// ---------------------------------------------------------------------------
// resolve_catalog_candidates handler behavior
// ---------------------------------------------------------------------------

// TestResolveCatalogHandlerReturnsServiceResult proves a successful
// service resolution produces a structured payload of bounded
// candidates with no source content.
func TestResolveCatalogHandlerReturnsServiceResult(t *testing.T) {
	cfg, svc := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	expected := []catalog.Candidate{validCandidate(), validCandidate()}
	svc.resolveFn = func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
		return expected, nil
	}

	_, out, err := callResolveCatalog(t, srv, validBuildInput())
	if err != nil {
		t.Fatalf("resolveCatalog error = %v", err)
	}
	if len(out.Candidates) != len(expected) {
		t.Fatalf("Candidates len = %d, want %d", len(out.Candidates), len(expected))
	}
	if svc.resolveCalls != 1 {
		t.Fatalf("ResolveCatalog calls = %d, want 1", svc.resolveCalls)
	}
}

// TestResolveCatalogHandlerHonorsContextCancellation proves a
// pre-cancelled context fails closed before any service call.
func TestResolveCatalogHandlerHonorsContextCancellation(t *testing.T) {
	cfg, svc := validConfig()
	svc.resolveFn = func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, ctx.Err()
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = callResolveCatalogWithContext(t, srv, ctx, validBuildInput())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveCatalog error = %v, want context.Canceled", err)
	}
}

// TestResolveCatalogHandlerMapsCredentialsUnavailable proves the
// canonical credential failure is returned verbatim so MCP marks the
// tool result as an error.
func TestResolveCatalogHandlerMapsCredentialsUnavailable(t *testing.T) {
	cfg, svc := validConfig()
	svc.resolveFn = func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, credential.ErrCredentialsUnavailable
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	_, _, err = callResolveCatalog(t, srv, validBuildInput())
	if !errors.Is(err, credential.ErrCredentialsUnavailable) {
		t.Fatalf("resolveCatalog error = %v, want ErrCredentialsUnavailable", err)
	}
}

// TestResolveCatalogHandlerMapsUnauthorized proves a policy denial
// returns the typed security error so the client sees a
// deterministic unauthorized classification.
func TestResolveCatalogHandlerMapsUnauthorized(t *testing.T) {
	cfg, svc := validConfig()
	svc.resolveFn = func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, security.ErrUnauthorized
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	_, _, err = callResolveCatalog(t, srv, validBuildInput())
	if !errors.Is(err, security.ErrUnauthorized) {
		t.Fatalf("resolveCatalog error = %v, want ErrUnauthorized", err)
	}
}

// TestResolveCatalogHandlerForwardsSelectorAndQuery proves the
// handler maps the typed input into the canonical service
// parameters.
func TestResolveCatalogHandlerForwardsSelectorAndQuery(t *testing.T) {
	cfg, svc := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	input := validBuildInput()
	input.Parameters = []string{"%PISA%"}

	_, _, err = callResolveCatalog(t, srv, input)
	if err != nil {
		t.Fatalf("resolveCatalog error = %v", err)
	}
	if svc.lastSelector != security.SelectorResolveCatalog {
		t.Fatalf("selector = %q, want resolve_catalog_candidates", svc.lastSelector)
	}
	if svc.lastQuery.Statement != input.Statement {
		t.Fatalf("statement = %q, want %q", svc.lastQuery.Statement, input.Statement)
	}
	if !slices.Equal(svc.lastQuery.Parameters, input.Parameters) {
		t.Fatalf("parameters = %v, want %v", svc.lastQuery.Parameters, input.Parameters)
	}
}

// ---------------------------------------------------------------------------
// read_selected_source handler behavior
// ---------------------------------------------------------------------------

// TestReadSelectedSourceHandlerReturnsServiceResult proves a
// successful service page is forwarded to the typed MCP output.
func TestReadSelectedSourceHandlerReturnsServiceResult(t *testing.T) {
	cfg, svc := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	expected := source.Page{StartLine: 1, LineCount: 2, Lines: []string{"line-one", "line-two"}, EOF: false, NextStartLine: 3}
	svc.readFn = func(ctx context.Context, cursor string, page source.Range) (source.Page, error) {
		return expected, nil
	}

	_, out, err := callReadSelectedSource(t, srv, validReadInput())
	if err != nil {
		t.Fatalf("readSelectedSource error = %v", err)
	}
	if !reflect.DeepEqual(out.Page, expected) {
		t.Fatalf("Page = %+v, want %+v", out.Page, expected)
	}
	if svc.readCalls != 1 {
		t.Fatalf("ReadSelectedSource calls = %d, want 1", svc.readCalls)
	}
}

// TestReadSelectedSourceHandlerHonorsContextCancellation proves a
// pre-cancelled context fails closed before any service call.
func TestReadSelectedSourceHandlerHonorsContextCancellation(t *testing.T) {
	cfg, svc := validConfig()
	svc.readFn = func(ctx context.Context, cursor string, page source.Range) (source.Page, error) {
		return source.Page{}, ctx.Err()
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = callReadSelectedSourceWithContext(t, srv, ctx, validReadInput())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("readSelectedSource error = %v, want context.Canceled", err)
	}
}

// TestReadSelectedSourceHandlerMapsStaleCoordinate proves a stale
// coordinate error from the service is forwarded verbatim.
func TestReadSelectedSourceHandlerMapsStaleCoordinate(t *testing.T) {
	cfg, svc := validConfig()
	svc.readFn = func(ctx context.Context, cursor string, page source.Range) (source.Page, error) {
		return source.Page{}, source.ErrStaleCoordinate
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	_, _, err = callReadSelectedSource(t, srv, validReadInput())
	if !errors.Is(err, source.ErrStaleCoordinate) {
		t.Fatalf("readSelectedSource error = %v, want ErrStaleCoordinate", err)
	}
}

// TestReadSelectedSourceHandlerForwardsCursorAndRange proves the
// handler maps the typed input into the canonical service
// parameters.
func TestReadSelectedSourceHandlerForwardsCursorAndRange(t *testing.T) {
	cfg, svc := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	input := validReadInput()
	input.StartLine = 5
	input.MaxLines = 25

	_, _, err = callReadSelectedSource(t, srv, input)
	if err != nil {
		t.Fatalf("readSelectedSource error = %v", err)
	}
	if svc.lastCursor != input.Cursor {
		t.Fatalf("cursor = %q, want %q", svc.lastCursor, input.Cursor)
	}
	if svc.lastRange.StartLine != 5 || svc.lastRange.MaxLines != 25 {
		t.Fatalf("range = %+v, want start=5 max=25", svc.lastRange)
	}
}

// TestReadSelectedSourceOutputHasNoCursorField proves the typed
// output never embeds the raw cursor string. The cursor is the
// opaque server binding and must remain inside the lease store; the
// MCP output is the page alone.
func TestReadSelectedSourceOutputHasNoCursorField(t *testing.T) {
	typ := reflect.TypeOf(ReadSelectedSourceOutput{})
	if found, name := hasFieldContaining(typ, "cursor"); found {
		t.Fatalf("ReadSelectedSourceOutput has forbidden field %q (matched %q)", name, "cursor")
	}
	if found, name := hasFieldContaining(typ, "raw"); found {
		t.Fatalf("ReadSelectedSourceOutput has forbidden field %q (matched %q)", name, "raw")
	}
}

// TestResolveCatalogOutputHasNoCursorOrSourceField proves the typed
// catalog output contains only the bounded candidate set and never
// embeds source content or the cursor.
func TestResolveCatalogOutputHasNoCursorOrSourceField(t *testing.T) {
	typ := reflect.TypeOf(ResolveCatalogOutput{})
	for _, forbidden := range []string{"cursor", "raw", "source", "path", "host", "user", "command", "sql"} {
		if found, name := hasFieldContaining(typ, forbidden); found {
			t.Fatalf("ResolveCatalogOutput has forbidden field %q (matched %q)", name, forbidden)
		}
	}
}

// ---------------------------------------------------------------------------
// In-memory transport integration
// ---------------------------------------------------------------------------

// TestServerListsOnlyTwoToolsThroughTransport proves an MCP client
// connected through the official in-memory transport sees exactly the
// two registered tools and nothing else.
func TestServerListsOnlyTwoToolsThroughTransport(t *testing.T) {
	cfg, _ := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.impl.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server Connect error = %v", err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("client Connect error = %v", err)
	}
	defer cs.Close()

	names := []string{}
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("Tools iterator error = %v", err)
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{"read_selected_source", "resolve_catalog_candidates"}
	if !slices.Equal(names, want) {
		t.Fatalf("transport tools = %v, want %v", names, want)
	}
}

// TestServerResolvesCatalogOverTransport proves a tool call
// initiated through the in-memory transport produces the typed
// output produced by the service.
func TestServerResolvesCatalogOverTransport(t *testing.T) {
	cfg, svc := validConfig()
	expected := []catalog.Candidate{validCandidate(), validCandidate()}
	svc.resolveFn = func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
		return expected, nil
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.impl.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server Connect error = %v", err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("client Connect error = %v", err)
	}
	defer cs.Close()

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "resolve_catalog_candidates",
		Arguments: validBuildInput(),
	})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool IsError = true: %+v", result.Content)
	}
	if len(svc.resolveCalls) != 1 && svc.resolveCalls != 1 {
		t.Fatalf("resolve calls = %d, want 1", svc.resolveCalls)
	}
}

// ---------------------------------------------------------------------------
// Handler invocation helpers
// ---------------------------------------------------------------------------

// callResolveCatalog invokes the resolve handler through the registered
// tool function map. It returns the result, typed output, and error.
// Tests use this so the wire protocol does not need to be exercised
// for every case.
func callResolveCatalog(t *testing.T, srv *Server, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error) {
	t.Helper()
	return callResolveCatalogWithContext(t, srv, context.Background(), input)
}

func callResolveCatalogWithContext(t *testing.T, srv *Server, ctx context.Context, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error) {
	t.Helper()
	handler, ok := srv.handlers["resolve_catalog_candidates"]
	if !ok {
		t.Fatalf("resolve_catalog_candidates handler is not registered")
	}
	return handler(ctx, input)
}

// callReadSelectedSource invokes the read handler through the
// registered tool function map.
func callReadSelectedSource(t *testing.T, srv *Server, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error) {
	t.Helper()
	return callReadSelectedSourceWithContext(t, srv, context.Background(), input)
}

func callReadSelectedSourceWithContext(t *testing.T, srv *Server, ctx context.Context, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error) {
	t.Helper()
	handler, ok := srv.handlers["read_selected_source"]
	if !ok {
		t.Fatalf("read_selected_source handler is not registered")
	}
	return handler(ctx, input)
}

// marshalCallReadSelectedSource invokes the read handler and
// marshals the typed output for sensitive-content assertions. The
// helper always returns a non-nil raw payload so the test can scan
// the full JSON for forbidden content.
func marshalCallReadSelectedSource(t *testing.T, srv *Server, input ReadSelectedSourceInput) ([]byte, error) {
	t.Helper()
	_, out, err := callReadSelectedSource(t, srv, input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}
