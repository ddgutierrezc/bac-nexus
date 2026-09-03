// Package mcp exposes the v1 read-only catalog-context service as a
// typed, stdio Model Context Protocol server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/app"
	"bac-nexus/internal/audit"
	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

type fakeService struct {
	resolveFn    func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error)
	readFn       func(ctx context.Context, selection catalog.Candidate, cursor string, page source.Range) (source.Page, error)
	resolveCalls int
	readCalls    int
	lastSearch   catalog.Search
	lastSelector security.Selector
	lastCursor   string
	lastRange    source.Range
}

func (f *fakeService) ResolveCatalog(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
	f.resolveCalls++
	f.lastSearch = search
	f.lastSelector = selector
	if f.resolveFn != nil {
		return f.resolveFn(ctx, search, selector)
	}
	return nil, nil
}

func (f *fakeService) ReadSelectedSource(ctx context.Context, selection catalog.Candidate, cursor string, page source.Range) (source.Page, error) {
	f.readCalls++
	f.lastCursor = cursor
	f.lastRange = page
	if f.readFn != nil {
		return f.readFn(ctx, selection, cursor, page)
	}
	return source.Page{}, nil
}

func validCandidate() catalog.Candidate {
	return catalog.Candidate{Item: "PISA061", SourceLibrary: "QRPGLESRC", SourceFileBase: "QRPGLESRC", ObjectType: "RPGLE", SourceType: "RPG", Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB", Description: "test"}
}

func validConfig() (Config, *fakeService) {
	svc := &fakeService{}
	return Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}, Service: svc}, svc
}

// forbiddenSurfaceSubstrings is the structural guard list. Adding
// an entry requires an explicit decision and a matching red test.
var forbiddenSurfaceSubstrings = []string{"path", "command", "shell", "exec", "sql", "ssh", "dial", "connect", "remote", "clientinfo", "parent"}

func hasFieldContaining(typ reflect.Type, substring string) (bool, string) {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return false, ""
	}
	for i := 0; i < typ.NumField(); i++ {
		if strings.Contains(strings.ToLower(typ.Field(i).Name), substring) {
			return true, typ.Field(i).Name
		}
	}
	return false, ""
}

// canned resolve/read functions shared by the behavior tables.
var (
	resolveSuccess = func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
		return []catalog.Candidate{validCandidate(), validCandidate()}, nil
	}
	resolvePassThrough = func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, nil
	}
	resolveCtx = func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, context.Canceled
	}
	resolveCreds = func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, credential.ErrCredentialsUnavailable
	}
	resolveUnauth = func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
		return nil, security.ErrUnauthorized
	}

	readSuccess = func(ctx context.Context, _ catalog.Candidate, cursor string, _ source.Range) (source.Page, error) {
		return source.Page{StartLine: 1, LineCount: 2, Lines: []string{"line-one", "line-two"}, EOF: false, NextStartLine: 3}, nil
	}
	readPassThrough = func(ctx context.Context, _ catalog.Candidate, cursor string, _ source.Range) (source.Page, error) {
		return source.Page{}, nil
	}
	readCtx = func(ctx context.Context, _ catalog.Candidate, cursor string, _ source.Range) (source.Page, error) {
		return source.Page{}, context.Canceled
	}
	readStale = func(ctx context.Context, _ catalog.Candidate, cursor string, _ source.Range) (source.Page, error) {
		return source.Page{}, source.ErrStaleCoordinate
	}
	readInvalid = func(ctx context.Context, _ catalog.Candidate, cursor string, _ source.Range) (source.Page, error) {
		return source.Page{}, source.ErrInvalidRequest
	}
)

func TestServerRegistersExactlyTwoTools(t *testing.T) {
	cfg, _ := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	want := []string{"resolve_catalog_candidates", "read_selected_source"}
	if got := srv.ToolNames(); !slices.Equal(got, want) {
		t.Fatalf("ToolNames = %v, want %v", got, want)
	}
}

func TestServerRejectsNilService(t *testing.T) {
	if _, err := New(Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}}); err == nil {
		t.Fatal("New with nil service = nil error, want rejection")
	}
}

func TestServerSurfaceHasNoRemotePathOrShellCommands(t *testing.T) {
	checks := []reflect.Type{
		reflect.TypeOf((*Server)(nil)), reflect.TypeOf((*Service)(nil)).Elem(),
		reflect.TypeOf((*Config)(nil)), reflect.TypeOf(Info{}),
		reflect.TypeOf(ResolveCatalogInput{}), reflect.TypeOf(ResolveCatalogOutput{}),
		reflect.TypeOf(ReadSelectedSourceInput{}), reflect.TypeOf(ReadSelectedSourceOutput{}),
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
	inputs := []reflect.Type{reflect.TypeOf(ResolveCatalogInput{}), reflect.TypeOf(ReadSelectedSourceInput{})}
	for _, typ := range inputs {
		for _, forbidden := range []string{"path", "list", "delete", "remove", "command", "shell", "ssh", "exec", "sql", "remote"} {
			if found, name := hasFieldContaining(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
	outputs := []reflect.Type{reflect.TypeOf(ResolveCatalogOutput{}), reflect.TypeOf(ReadSelectedSourceOutput{})}
	for _, typ := range outputs {
		for _, forbidden := range []string{"raw", "source", "path", "host", "user", "command", "sql"} {
			if found, name := hasFieldContaining(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
}

func TestResolveCatalogHandlerBehavior(t *testing.T) {
	tests := []struct {
		name      string
		resolveFn func(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error)
		input     ResolveCatalogInput
		check     func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error)
	}{
		{
			name: "returns service result", resolveFn: resolveSuccess,
			input: ResolveCatalogInput{Item: "PISA061", ProductionLibrary: "PRODLIB"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				if len(out.Candidates) != 2 {
					t.Fatalf("Candidates len = %d, want 2", len(out.Candidates))
				}
				if svc.resolveCalls != 1 {
					t.Fatalf("resolveCalls = %d, want 1", svc.resolveCalls)
				}
			},
		},
		{
			name: "forwards selector and parameters", resolveFn: resolvePassThrough,
			input: ResolveCatalogInput{Item: "PISA061", ProductionLibrary: "PRODLIB"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				if svc.lastSelector != security.SelectorResolveCatalog {
					t.Fatalf("selector = %q, want resolve_catalog_candidates", svc.lastSelector)
				}
				if svc.lastSearch.Item != "PISA061" || svc.lastSearch.ProductionLibrary != "PRODLIB" {
					t.Fatalf("search = %+v, want normalized catalog criteria", svc.lastSearch)
				}
			},
		},
		{
			name: "maps context cancelled", resolveFn: resolveCtx,
			input: ResolveCatalogInput{Item: "PISA061"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v, want context.Canceled", err)
				}
			},
		},
		{
			name: "maps credentials unavailable", resolveFn: resolveCreds,
			input: ResolveCatalogInput{Item: "PISA061"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if !errors.Is(err, credential.ErrCredentialsUnavailable) {
					t.Fatalf("error = %v, want ErrCredentialsUnavailable", err)
				}
			},
		},
		{
			name: "maps unauthorized", resolveFn: resolveUnauth,
			input: ResolveCatalogInput{Item: "PISA061"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if !errors.Is(err, security.ErrUnauthorized) {
					t.Fatalf("error = %v, want ErrUnauthorized", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, svc := validConfig()
			svc.resolveFn = tt.resolveFn
			srv, err := New(cfg)
			if err != nil {
				t.Fatalf("New error = %v", err)
			}
			_, out, err := srv.resolveCatalog(context.Background(), nil, tt.input)
			tt.check(t, out, svc, err)
		})
	}
}

func TestResolveCatalogHandlerHonorsPreCancelledContext(t *testing.T) {
	cfg, _ := validConfig()
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = srv.resolveCatalog(ctx, nil, ResolveCatalogInput{Item: "PISA061"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveCatalog error = %v, want context.Canceled", err)
	}
}

func TestReadSelectedSourceHandlerBehavior(t *testing.T) {
	tests := []struct {
		name   string
		readFn func(ctx context.Context, selection catalog.Candidate, cursor string, page source.Range) (source.Page, error)
		input  ReadSelectedSourceInput
		check  func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error)
	}{
		{
			name: "returns service result", readFn: readSuccess,
			input: ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				if !reflect.DeepEqual(out.Page, readExpected) {
					t.Fatalf("Page = %+v, want %+v", out.Page, readExpected)
				}
				if svc.readCalls != 1 {
					t.Fatalf("readCalls = %d, want 1", svc.readCalls)
				}
			},
		},
		{
			name: "forwards cursor and range", readFn: readPassThrough,
			input: ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 5, MaxLines: 25},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				if svc.lastCursor != "opaque-cursor" {
					t.Fatalf("cursor = %q, want %q", svc.lastCursor, "opaque-cursor")
				}
				if svc.lastRange.StartLine != 5 || svc.lastRange.MaxLines != 25 {
					t.Fatalf("range = %+v, want start=5 max=25", svc.lastRange)
				}
			},
		},
		{
			name: "maps context cancelled", readFn: readCtx,
			input: ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v, want context.Canceled", err)
				}
			},
		},
		{
			name: "maps stale coordinate", readFn: readStale,
			input: ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if !errors.Is(err, source.ErrStaleCoordinate) {
					t.Fatalf("error = %v, want ErrStaleCoordinate", err)
				}
			},
		},
		{
			name: "maps invalid request", readFn: readInvalid,
			input: ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if !errors.Is(err, source.ErrInvalidRequest) {
					t.Fatalf("error = %v, want ErrInvalidRequest", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, svc := validConfig()
			svc.readFn = tt.readFn
			srv, err := New(cfg)
			if err != nil {
				t.Fatalf("New error = %v", err)
			}
			_, out, err := srv.readSelectedSource(context.Background(), nil, tt.input)
			tt.check(t, out, svc, err)
		})
	}
}

func TestServerServesBothToolsOverInMemoryMCPTransport(t *testing.T) {
	cfg, service := validConfig()
	service.resolveFn = func(context.Context, catalog.Search, security.Selector) ([]catalog.Candidate, error) {
		return []catalog.Candidate{validCandidate()}, nil
	}
	service.readFn = func(_ context.Context, _ catalog.Candidate, cursor string, page source.Range) (source.Page, error) {
		switch cursor {
		case "":
			if page != (source.Range{StartLine: 1, MaxLines: 2}) {
				return source.Page{}, source.ErrInvalidRequest
			}
			return source.Page{StartLine: 1, LineCount: 2, Lines: []string{"one", "two"}, NextStartLine: 3, Cursor: "page-one"}, nil
		case "page-one":
			if page != (source.Range{StartLine: 3, MaxLines: 2}) {
				return source.Page{}, source.ErrInvalidRequest
			}
			return source.Page{StartLine: 3, LineCount: 1, Lines: []string{"three"}, EOF: true}, nil
		default:
			return source.Page{}, source.ErrInvalidRequest
		}
	}

	client, wait := connectInMemoryMCP(t, cfg)
	defer wait()

	resolved := callMCPTool(t, client, "resolve_catalog_candidates", ResolveCatalogInput{Item: "PISA061"})
	if resolved.IsError {
		t.Fatal("resolve_catalog_candidates returned an MCP tool error")
	}
	var resolvedOutput ResolveCatalogOutput
	decodeStructured(t, resolved, &resolvedOutput)
	if !slices.Equal(resolvedOutput.Candidates, []catalog.Candidate{validCandidate()}) {
		t.Fatalf("resolved candidates = %+v, want the exact selected candidate", resolvedOutput.Candidates)
	}

	first := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Selection: validCandidate(), StartLine: 1, MaxLines: 2})
	if first.IsError {
		t.Fatal("first source page returned an MCP tool error")
	}
	var firstOutput ReadSelectedSourceOutput
	decodeStructured(t, first, &firstOutput)
	if firstOutput.Page.Cursor != "page-one" || firstOutput.Page.EOF || firstOutput.Page.NextStartLine != 3 || !slices.Equal(firstOutput.Page.Lines, []string{"one", "two"}) {
		t.Fatalf("first page = %+v, want continuation page", firstOutput.Page)
	}

	last := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Cursor: firstOutput.Page.Cursor, StartLine: firstOutput.Page.NextStartLine, MaxLines: 2})
	if last.IsError {
		t.Fatal("continuation source page returned an MCP tool error")
	}
	var lastOutput ReadSelectedSourceOutput
	decodeStructured(t, last, &lastOutput)
	if !lastOutput.Page.EOF || lastOutput.Page.Cursor != "" || lastOutput.Page.NextStartLine != 0 || !slices.Equal(lastOutput.Page.Lines, []string{"three"}) {
		t.Fatalf("last page = %+v, want EOF without cursor or next start", lastOutput.Page)
	}
}

func TestServerProtocolFailuresHaveNoPartialSource(t *testing.T) {
	cfg, service := validConfig()
	service.resolveFn = func(ctx context.Context, _ catalog.Search, _ security.Selector) ([]catalog.Candidate, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	service.readFn = func(_ context.Context, _ catalog.Candidate, cursor string, page source.Range) (source.Page, error) {
		if page.MaxLines == 0 {
			return source.Page{}, source.ErrInvalidRequest
		}
		partial := source.Page{Lines: []string{"must-not-leak"}, Cursor: "must-not-leak", LineCount: 1, NextStartLine: 2}
		switch cursor {
		case "acquisition":
			return partial, errors.New("source acquisition unavailable")
		case "page":
			return partial, source.ErrResponseTooLarge
		case "range":
			return partial, source.ErrRangeStartOutOfBounds
		default:
			return partial, source.ErrStaleCoordinate
		}
	}

	client, wait := connectInMemoryMCP(t, cfg)
	defer wait()

	malformed := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Selection: validCandidate(), StartLine: 1, MaxLines: 0})
	assertMCPToolFailureHasNoSource(t, malformed)

	for _, cursor := range []string{"expired-or-wrong-selection", "acquisition", "page", "range"} {
		t.Run(cursor, func(t *testing.T) {
			result := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Cursor: cursor, StartLine: 1, MaxLines: 1})
			assertMCPToolFailureHasNoSource(t, result)
		})
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.CallTool(cancelled, &sdk.CallToolParams{Name: "resolve_catalog_candidates", Arguments: ResolveCatalogInput{Item: "PISA061"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled tool call error = %v, want context.Canceled", err)
	}
}

func TestServerShutdownCancelsAcceptedHandlerBeforeWaiting(t *testing.T) {
	cfg, service := validConfig()
	started := make(chan struct{})
	handlerCancelled := make(chan struct{})
	service.readFn = func(ctx context.Context, _ catalog.Candidate, _ string, _ source.Range) (source.Page, error) {
		close(started)
		<-ctx.Done()
		close(handlerCancelled)
		return source.Page{}, ctx.Err()
	}
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	cfg.Transport = serverTransport
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	serverContext, cancelServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Run(serverContext) }()
	client := sdk.NewClient(&sdk.Implementation{Name: "mcp-test-client", Version: "v0.0.0"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		cancelServer()
		t.Fatalf("client Connect error = %v", err)
	}
	defer session.Close()

	callDone := make(chan error, 1)
	go func() {
		_, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "read_selected_source", Arguments: ReadSelectedSourceInput{Selection: validCandidate(), StartLine: 1, MaxLines: 1}})
		callDone <- err
	}()
	<-started
	cancelServer()
	select {
	case <-handlerCancelled:
	case <-time.After(time.Second):
		t.Fatal("accepted handler context was not cancelled during shutdown")
	}
	select {
	case err := <-serverDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("server Run error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server shutdown did not complete after cancelling the accepted handler")
	}
	if err := <-callDone; err == nil {
		t.Fatal("CallTool error = nil, want cancellation after server shutdown")
	}
}

func TestServerSanitizesTransportLifecycleErrors(t *testing.T) {
	cfg, _ := validConfig()
	cfg.Transport = failingTransport{err: errors.New("peer payload secret=/srv/nexus")}
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = server.Run(context.Background())
	if !errors.Is(err, ErrLifecycleUnavailable) {
		t.Fatalf("Run() error = %v, want %v", err, ErrLifecycleUnavailable)
	}
	if strings.Contains(err.Error(), "secret=") || strings.Contains(err.Error(), "/srv/nexus") {
		t.Fatalf("Run() leaked transport detail: %q", err)
	}
}

type failingTransport struct{ err error }

func (t failingTransport) Connect(context.Context) (sdk.Connection, error) { return nil, t.err }

func connectInMemoryMCP(t *testing.T, cfg Config) (*sdk.ClientSession, func()) {
	t.Helper()
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	cfg.Transport = serverTransport
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
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

func callMCPTool(t *testing.T, session *sdk.ClientSession, name string, input any) *sdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: input})
	if err != nil {
		t.Fatalf("CallTool(%q) error = %v", name, err)
	}
	return result
}

func decodeStructured(t *testing.T, result *sdk.CallToolResult, target any) {
	t.Helper()
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured output error = %v", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		t.Fatalf("unmarshal structured output error = %v", err)
	}
}

func assertMCPToolFailureHasNoSource(t *testing.T, result *sdk.CallToolResult) {
	t.Helper()
	if !result.IsError {
		t.Fatal("tool result is not an MCP error")
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured error output = %v", err)
	}
	if strings.Contains(string(encoded), "must-not-leak") || strings.Contains(string(encoded), "cursor") || strings.Contains(string(encoded), "lineCount") || strings.Contains(string(encoded), "nextStartLine") {
		t.Fatalf("error structured output leaked source page data: %s", encoded)
	}
}

var readExpected = source.Page{StartLine: 1, LineCount: 2, Lines: []string{"line-one", "line-two"}, EOF: false, NextStartLine: 3}

func TestServerUsesRealAppServiceForPagingAndCursorFailures(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	candidate := validCandidate()
	resolver := &mcpRealResolver{candidates: []catalog.Candidate{candidate}}
	snapshot, err := source.NewSnapshot([]byte("one\ntwo\nthree\n"))
	if err != nil {
		t.Fatal(err)
	}
	acquirer := &mcpRealAcquirer{snapshot: snapshot}
	leases := source.NewLeaseStoreForTest(func() time.Time { return now }, strings.NewReader(strings.Repeat("r", 512)))
	service := app.NewService(app.ServiceDeps{
		Credentials: mcpRealCredentials{}, Authorizer: mcpRealAuthorizer{}, Auditor: audit.NewRecorder(),
		Resolver: resolver, Acquirer: acquirer,
		Leases:   leases,
		Recovery: mcpRealRecovery{}, Profile: "test-profile", Now: func() time.Time { return now },
	})
	if err := service.Startup(context.Background()); err != nil {
		t.Fatalf("Startup() error = %v", err)
	}
	client, wait := connectInMemoryMCP(t, Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}, Service: service})
	defer wait()

	resolved := callMCPTool(t, client, "resolve_catalog_candidates", ResolveCatalogInput{Item: candidate.Item, ProductionLibrary: candidate.ProductionLibrary})
	if resolved.IsError {
		t.Fatal("real service catalog resolve returned MCP error")
	}
	first := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Selection: candidate, StartLine: 1, MaxLines: 2})
	if first.IsError {
		t.Fatal("real service first page returned MCP error")
	}
	var firstOutput ReadSelectedSourceOutput
	decodeStructured(t, first, &firstOutput)
	if got, want := firstOutput.Page.Lines, []string{"one", "two"}; !slices.Equal(got, want) || firstOutput.Page.EOF || firstOutput.Page.NextStartLine != 3 || firstOutput.Page.Cursor == "" {
		t.Fatalf("first real page = %+v, want continuation", firstOutput.Page)
	}
	last := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Cursor: firstOutput.Page.Cursor, StartLine: 3, MaxLines: 2})
	if last.IsError {
		t.Fatal("real service continuation returned MCP error")
	}
	var lastOutput ReadSelectedSourceOutput
	decodeStructured(t, last, &lastOutput)
	if got, want := lastOutput.Page.Lines, []string{"three"}; !slices.Equal(got, want) || !lastOutput.Page.EOF || lastOutput.Page.Cursor != "" {
		t.Fatalf("last real page = %+v, want EOF", lastOutput.Page)
	}

	for _, input := range []ReadSelectedSourceInput{
		{Cursor: "malformed", StartLine: 1, MaxLines: 1},
		{Cursor: firstOutput.Page.Cursor, StartLine: 99, MaxLines: 1},
	} {
		assertMCPToolFailureHasNoSource(t, callMCPTool(t, client, "read_selected_source", input))
	}
	resolver.candidates = nil
	assertMCPToolFailureHasNoSource(t, callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Cursor: firstOutput.Page.Cursor, StartLine: 1, MaxLines: 1}))
	resolver.candidates = []catalog.Candidate{candidate}
	wrongSelection := candidate
	wrongSelection.SourceLibrary = "QWRONGSRC"
	wrongCursor, err := leases.Acquire(snapshot, wrongSelection, source.ClientPolicy("test-profile"))
	if err != nil {
		t.Fatalf("Acquire(wrong selection) error = %v", err)
	}
	assertMCPToolFailureHasNoSource(t, callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Cursor: string(wrongCursor), StartLine: 1, MaxLines: 1}))
	now = now.Add(11 * time.Minute)
	assertMCPToolFailureHasNoSource(t, callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Cursor: firstOutput.Page.Cursor, StartLine: 1, MaxLines: 1}))
}

func TestServerUsesRealAppServiceForAcquisitionFailureWithoutPartialSource(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	candidate := validCandidate()
	resolver := &mcpRealResolver{candidates: []catalog.Candidate{candidate}}
	cleanupCalls := 0
	acquirer := &mcpRealAcquirer{
		err:     errors.New("deterministic acquisition failure"),
		cleanup: func() { cleanupCalls++ },
	}
	leases := source.NewLeaseStoreForTest(func() time.Time { return now }, strings.NewReader(strings.Repeat("r", 512)))
	service := app.NewService(app.ServiceDeps{
		Credentials: mcpRealCredentials{}, Authorizer: mcpRealAuthorizer{}, Auditor: audit.NewRecorder(),
		Resolver: resolver, Acquirer: acquirer, Leases: leases,
		Recovery: mcpRealRecovery{}, Profile: "test-profile", Now: func() time.Time { return now },
	})
	if err := service.Startup(context.Background()); err != nil {
		t.Fatalf("Startup() error = %v", err)
	}
	client, wait := connectInMemoryMCP(t, Config{Info: Info{Name: "bac-nexus", Version: "v0.0.0"}, Service: service})
	defer wait()

	result := callMCPTool(t, client, "read_selected_source", ReadSelectedSourceInput{Selection: candidate, StartLine: 1, MaxLines: 1})
	assertMCPToolFailureHasNoSource(t, result)
	if acquirer.calls != 1 || cleanupCalls != 1 {
		t.Fatalf("acquisition calls/cleanup = %d/%d, want 1/1", acquirer.calls, cleanupCalls)
	}
	if leases.Resident() != 0 {
		t.Fatalf("resident lease bytes after acquisition failure = %d, want 0", leases.Resident())
	}
}

type mcpRealCredentials struct{}

func (mcpRealCredentials) Get(string) ([]byte, error) { return []byte("test"), nil }
func (mcpRealCredentials) Set(string, []byte) error   { return nil }
func (mcpRealCredentials) Delete(string) error        { return nil }

type mcpRealAuthorizer struct{}

func (mcpRealAuthorizer) Authorize(_ context.Context, selector security.Selector, target security.CapabilityTarget) (security.Decision_, error) {
	return security.Decision_{Selector: selector, Target: target, Decision: security.DecisionAllow, Reason: "allowlisted selector and matching target class"}, nil
}

type mcpRealResolver struct{ candidates []catalog.Candidate }

func (r *mcpRealResolver) Resolve(context.Context, catalog.Search) ([]catalog.Candidate, error) {
	return r.candidates, nil
}

type mcpRealAcquirer struct {
	snapshot *source.Snapshot
	err      error
	cleanup  func()
	calls    int
}

func (a *mcpRealAcquirer) Acquire(context.Context, catalog.Candidate) (*source.Snapshot, error) {
	a.calls++
	if a.cleanup != nil {
		defer a.cleanup()
	}
	return a.snapshot, a.err
}

type mcpRealRecovery struct{}

func (mcpRealRecovery) Recover(context.Context) error { return nil }
