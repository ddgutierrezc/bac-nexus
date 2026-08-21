// Package mcp exposes the v1 read-only catalog-context service as a
// typed, stdio Model Context Protocol server. The package owns no
// remote, path, shell, SQL, or SSH capability of its own; it adapts
// internal/app.Service calls to the official MCP wire protocol and
// surfaces only the two allowed tools: resolve_catalog_candidates
// and read_selected_source.
package mcp

import (
	"context"
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

// fakeService is the deterministic, package-local Service double used
// by every handler test. It records the last call and returns the
// configured result.
type fakeService struct {
	resolveFn    func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error)
	readFn       func(ctx context.Context, cursor string, page source.Range) (source.Page, error)
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

func validConfig() (Config, *fakeService) {
	svc := &fakeService{}
	return Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}, Service: svc}, svc
}

func validBuildInput() ResolveCatalogInput {
	return ResolveCatalogInput{Statement: "SELECT * FROM catalog", Parameters: []string{"%PISA061%"}}
}

func validReadInput() ReadSelectedSourceInput {
	return ReadSelectedSourceInput{Cursor: "opaque-cursor-token", StartLine: 1, MaxLines: 50}
}

// forbiddenSurfaceSubstrings is the authoritative structural guard
// list. Adding an entry requires an explicit decision and a matching
// red test.
var forbiddenSurfaceSubstrings = []string{
	"path", "command", "shell", "exec", "sql", "ssh",
	"dial", "connect", "remote", "clientinfo", "parent",
}

// CallToolResult is an alias used in test helpers so we do not
// have to import the mcp package directly in the test file's
// helper signatures.
type CallToolResult = mcp.CallToolResult

// hasFieldContaining returns whether the supplied struct type exposes
// a field whose lower-cased name contains the supplied substring. It
// recurses into anonymous embedded structs.
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
// Tool registration and structural surface
// ---------------------------------------------------------------------------

func TestServerRegistersExactlyTwoTools(t *testing.T) {
	cfg, _ := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	names := srv.ToolNames()
	want := []string{"resolve_catalog_candidates", "read_selected_source"}
	if !slices.Equal(names, want) {
		t.Fatalf("ToolNames = %v, want %v", names, want)
	}
}

func TestServerRejectsNilService(t *testing.T) {
	if _, err := New(Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}}); err == nil {
		t.Fatal("New with nil service = nil error, want rejection")
	}
}

func TestServerSurfaceHasNoRemotePathOrShellCommands(t *testing.T) {
	checks := []reflect.Type{
		reflect.TypeOf((*Server)(nil)),
		reflect.TypeOf((*Service)(nil)).Elem(),
		reflect.TypeOf((*Config)(nil)),
		reflect.TypeOf(Info{}),
		reflect.TypeOf(ResolveCatalogInput{}),
		reflect.TypeOf(ResolveCatalogOutput{}),
		reflect.TypeOf(ReadSelectedSourceInput{}),
		reflect.TypeOf(ReadSelectedSourceOutput{}),
	}
	for _, typ := range checks {
		for _, forbidden := range forbiddenSurfaceSubstrings {
			if found, name := hasFieldContaining(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
}

func TestTypedInputOutputSchemasAreBounded(t *testing.T) {
	inputs := map[string]reflect.Type{
		"ResolveCatalogInput":    reflect.TypeOf(ResolveCatalogInput{}),
		"ReadSelectedSourceInput": reflect.TypeOf(ReadSelectedSourceInput{}),
	}
	for _, typ := range inputs {
		for _, forbidden := range []string{"path", "list", "delete", "remove", "command", "shell", "ssh", "exec", "sql", "remote"} {
			if found, name := hasFieldContaining(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
	outputs := map[string]reflect.Type{
		"ResolveCatalogOutput":    reflect.TypeOf(ResolveCatalogOutput{}),
		"ReadSelectedSourceOutput": reflect.TypeOf(ReadSelectedSourceOutput{}),
	}
	for _, typ := range outputs {
		for _, forbidden := range []string{"cursor", "raw", "source", "path", "host", "user", "command", "sql"} {
			if found, name := hasFieldContaining(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// resolve_catalog_candidates handler behavior
// ---------------------------------------------------------------------------

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

func TestResolveCatalogHandlerForwardsSelectorAndQuery(t *testing.T) {
	cfg, svc := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	input := validBuildInput()
	input.Parameters = []string{"%PISA%"}
	if _, _, err := callResolveCatalog(t, srv, input); err != nil {
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

func TestResolveCatalogHandlerErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"context cancelled", context.Canceled},
		{"credentials unavailable", credential.ErrCredentialsUnavailable},
		{"unauthorized", security.ErrUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, svc := validConfig()
			svc.resolveFn = func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
				return nil, tt.err
			}
			srv, err := New(cfg)
			if err != nil {
				t.Fatalf("New error = %v", err)
			}
			_, _, err = callResolveCatalog(t, srv, validBuildInput())
			if !errors.Is(err, tt.err) {
				t.Fatalf("resolveCatalog error = %v, want %v", err, tt.err)
			}
		})
	}
}

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

// ---------------------------------------------------------------------------
// read_selected_source handler behavior
// ---------------------------------------------------------------------------

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

func TestReadSelectedSourceHandlerForwardsCursorAndRange(t *testing.T) {
	cfg, svc := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	input := validReadInput()
	input.StartLine = 5
	input.MaxLines = 25
	if _, _, err := callReadSelectedSource(t, srv, input); err != nil {
		t.Fatalf("readSelectedSource error = %v", err)
	}
	if svc.lastCursor != input.Cursor {
		t.Fatalf("cursor = %q, want %q", svc.lastCursor, input.Cursor)
	}
	if svc.lastRange.StartLine != 5 || svc.lastRange.MaxLines != 25 {
		t.Fatalf("range = %+v, want start=5 max=25", svc.lastRange)
	}
}

func TestReadSelectedSourceHandlerErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"context cancelled", context.Canceled},
		{"stale coordinate", source.ErrStaleCoordinate},
		{"invalid request", source.ErrInvalidRequest},
		{"credentials unavailable", credential.ErrCredentialsUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, svc := validConfig()
			svc.readFn = func(ctx context.Context, cursor string, page source.Range) (source.Page, error) {
				return source.Page{}, tt.err
			}
			srv, err := New(cfg)
			if err != nil {
				t.Fatalf("New error = %v", err)
			}
			_, _, err = callReadSelectedSource(t, srv, validReadInput())
			if !errors.Is(err, tt.err) {
				t.Fatalf("readSelectedSource error = %v, want %v", err, tt.err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Handler invocation helpers
// ---------------------------------------------------------------------------

func callResolveCatalog(t *testing.T, srv *Server, input ResolveCatalogInput) (*CallToolResult, ResolveCatalogOutput, error) {
	t.Helper()
	return callResolveCatalogWithContext(t, srv, context.Background(), input)
}

func callResolveCatalogWithContext(t *testing.T, srv *Server, ctx context.Context, input ResolveCatalogInput) (*CallToolResult, ResolveCatalogOutput, error) {
	t.Helper()
	raw, ok := srv.handlers["resolve_catalog_candidates"]
	if !ok {
		t.Fatalf("resolve_catalog_candidates handler is not registered")
	}
	handler, ok := raw.(resolveCatalogFn)
	if !ok {
		t.Fatalf("resolve_catalog_candidates handler has wrong type %T", raw)
	}
	return handler(ctx, input)
}

func callReadSelectedSource(t *testing.T, srv *Server, input ReadSelectedSourceInput) (*CallToolResult, ReadSelectedSourceOutput, error) {
	t.Helper()
	return callReadSelectedSourceWithContext(t, srv, context.Background(), input)
}

func callReadSelectedSourceWithContext(t *testing.T, srv *Server, ctx context.Context, input ReadSelectedSourceInput) (*CallToolResult, ReadSelectedSourceOutput, error) {
	t.Helper()
	raw, ok := srv.handlers["read_selected_source"]
	if !ok {
		t.Fatalf("read_selected_source handler is not registered")
	}
	handler, ok := raw.(readSelectedSourceFn)
	if !ok {
		t.Fatalf("read_selected_source handler has wrong type %T", raw)
	}
	return handler(ctx, input)
}
