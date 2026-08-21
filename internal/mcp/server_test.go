// Package mcp exposes the v1 read-only catalog-context service as a
// typed, stdio Model Context Protocol server.
package mcp

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

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
	if f.resolveFn != nil {
		return f.resolveFn(ctx, query, selector)
	}
	return nil, nil
}

func (f *fakeService) ReadSelectedSource(ctx context.Context, cursor string, page source.Range) (source.Page, error) {
	f.readCalls++
	f.lastCursor = cursor
	f.lastRange = page
	if f.readFn != nil {
		return f.readFn(ctx, cursor, page)
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

// forbiddenSurfaceSubstrings is the structural guard list.
var forbiddenSurfaceSubstrings = []string{"path", "command", "shell", "exec", "sql", "ssh", "dial", "connect", "remote", "clientinfo", "parent"}

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
		for _, forbidden := range []string{"cursor", "raw", "source", "path", "host", "user", "command", "sql"} {
			if found, name := hasFieldContaining(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
}

func TestResolveCatalogHandlerBehavior(t *testing.T) {
	candidate := validCandidate()
	tests := []struct {
		name      string
		resolveFn func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error)
		input     ResolveCatalogInput
		check     func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error)
	}{
		{
			name: "returns service result",
			resolveFn: func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) {
				return []catalog.Candidate{candidate, candidate}, nil
			},
			input: ResolveCatalogInput{Statement: "SELECT 1", Parameters: []string{"%PISA%"}},
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
			name:      "forwards selector and parameters",
			resolveFn: func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) { return nil, nil },
			input:     ResolveCatalogInput{Statement: "SELECT 1", Parameters: []string{"%PISA%"}},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				if svc.lastSelector != security.SelectorResolveCatalog {
					t.Fatalf("selector = %q, want resolve_catalog_candidates", svc.lastSelector)
				}
				if svc.lastQuery.Statement != "SELECT 1" || !slices.Equal(svc.lastQuery.Parameters, []string{"%PISA%"}) {
					t.Fatalf("query = %+v, want SELECT 1 [%%PISA%%]", svc.lastQuery)
				}
			},
		},
		{
			name:      "maps context cancelled",
			resolveFn: func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) { return nil, context.Canceled },
			input:     ResolveCatalogInput{Statement: "SELECT 1"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v, want context.Canceled", err)
				}
			},
		},
		{
			name:      "maps credentials unavailable",
			resolveFn: func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) { return nil, credential.ErrCredentialsUnavailable },
			input:     ResolveCatalogInput{Statement: "SELECT 1"},
			check: func(t *testing.T, out ResolveCatalogOutput, svc *fakeService, err error) {
				if !errors.Is(err, credential.ErrCredentialsUnavailable) {
					t.Fatalf("error = %v, want ErrCredentialsUnavailable", err)
				}
			},
		},
		{
			name:      "maps unauthorized",
			resolveFn: func(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error) { return nil, security.ErrUnauthorized },
			input:     ResolveCatalogInput{Statement: "SELECT 1"},
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
	_, _, err = srv.resolveCatalog(ctx, nil, ResolveCatalogInput{Statement: "SELECT 1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveCatalog error = %v, want context.Canceled", err)
	}
}

func TestReadSelectedSourceHandlerBehavior(t *testing.T) {
	expected := source.Page{StartLine: 1, LineCount: 2, Lines: []string{"line-one", "line-two"}, EOF: false, NextStartLine: 3}
	tests := []struct {
		name   string
		readFn func(ctx context.Context, cursor string, page source.Range) (source.Page, error)
		input  ReadSelectedSourceInput
		check  func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error)
	}{
		{
			name:   "returns service result",
			readFn: func(ctx context.Context, cursor string, _ source.Range) (source.Page, error) { return expected, nil },
			input:  ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				if !reflect.DeepEqual(out.Page, expected) {
					t.Fatalf("Page = %+v, want %+v", out.Page, expected)
				}
				if svc.readCalls != 1 {
					t.Fatalf("readCalls = %d, want 1", svc.readCalls)
				}
			},
		},
		{
			name:   "forwards cursor and range",
			readFn: func(ctx context.Context, cursor string, _ source.Range) (source.Page, error) { return source.Page{}, nil },
			input:  ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 5, MaxLines: 25},
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
			name:   "maps context cancelled",
			readFn: func(ctx context.Context, cursor string, _ source.Range) (source.Page, error) { return source.Page{}, context.Canceled },
			input:  ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v, want context.Canceled", err)
				}
			},
		},
		{
			name:   "maps stale coordinate",
			readFn: func(ctx context.Context, cursor string, _ source.Range) (source.Page, error) { return source.Page{}, source.ErrStaleCoordinate },
			input:  ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
			check: func(t *testing.T, out ReadSelectedSourceOutput, svc *fakeService, err error) {
				if !errors.Is(err, source.ErrStaleCoordinate) {
					t.Fatalf("error = %v, want ErrStaleCoordinate", err)
				}
			},
		},
		{
			name:   "maps invalid request",
			readFn: func(ctx context.Context, cursor string, _ source.Range) (source.Page, error) { return source.Page{}, source.ErrInvalidRequest },
			input:  ReadSelectedSourceInput{Cursor: "opaque-cursor", StartLine: 1, MaxLines: 50},
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
