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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// Service is the narrow dependency surface the MCP server requires.
// The internal/app.Service type implements this interface; the MCP
// package depends on the interface to keep the wire-protocol adapter
// testable and the dependency direction one-way.
type Service interface {
	ResolveCatalog(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error)
	ReadSelectedSource(ctx context.Context, cursor string, page source.Range) (source.Page, error)
}

// Info identifies the MCP server to clients. The defaults match
// the canonical v1 implementation name and version.
type Info struct {
	Name    string
	Version string
}

// Config is the construction input for a Server. Service is
// required; Info, Profile, and Transport are optional.
type Config struct {
	Info      Info
	Service   Service
	Profile   string
	Transport mcp.Transport // optional test override; nil → StdioTransport at Run time
}

// Server is the stdio MCP facade. It registers exactly two typed
// tools and exposes no other surface. The internal/ subpackage
// name and the field names of this struct are part of the v1 wire
// contract.
type Server struct {
	impl      *mcp.Server
	service   Service
	info      Info
	transport mcp.Transport
	toolNames []string
	handlers  map[string]any
}

// resolveCatalogFn is the public handler signature used by direct
// test invocation. The SDK adapter below maps to this shape.
type resolveCatalogFn func(ctx context.Context, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error)

// readSelectedSourceFn is the public handler signature used by
// direct test invocation.
type readSelectedSourceFn func(ctx context.Context, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error)

// ResolveCatalogInput is the typed MCP request for
// resolve_catalog_candidates. The shape is bounded: the caller
// supplies a SQL statement and positional parameters, and never
// any temporary, listing, or delete path.
type ResolveCatalogInput struct {
	Statement  string   `json:"statement" jsonschema:"bounded SQL statement to execute"`
	Parameters []string `json:"parameters,omitempty" jsonschema:"positional query parameters"`
}

// ResolveCatalogOutput is the typed MCP response for
// resolve_catalog_candidates. It carries only the bounded
// candidate coordinates; source content is never returned.
type ResolveCatalogOutput struct {
	Candidates []catalog.Candidate `json:"candidates"`
}

// ReadSelectedSourceInput is the typed MCP request for
// read_selected_source. The caller supplies the opaque cursor and
// the one-based inclusive page range. The cursor is the only
// selection binding; no path, listing, or delete field exists.
type ReadSelectedSourceInput struct {
	Cursor    string `json:"cursor" jsonschema:"opaque snapshot cursor"`
	StartLine int    `json:"startLine" jsonschema:"one-based inclusive start line"`
	MaxLines  int    `json:"maxLines" jsonschema:"maximum lines in this page"`
}

// ReadSelectedSourceOutput is the typed MCP response for
// read_selected_source. The output carries only the page; the
// cursor is never echoed.
type ReadSelectedSourceOutput struct {
	Page source.Page `json:"page"`
}

// New constructs a Server and registers the two canonical tools.
// It fails closed when Service is nil so a misconfigured main never
// silently registers a tool that would panic on invocation.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("mcp server requires a service")
	}
	info := cfg.Info
	if info.Name == "" {
		info.Name = "bac-nexus"
	}
	if info.Version == "" {
		info.Version = "v0.0.0"
	}
	srv := &Server{
		service:   cfg.Service,
		info:      info,
		transport: cfg.Transport,
		handlers:  make(map[string]any, 2),
	}
	srv.impl = mcp.NewServer(&mcp.Implementation{Name: info.Name, Version: info.Version}, nil)
	srv.toolNames = []string{"resolve_catalog_candidates", "read_selected_source"}
	mcp.AddTool(srv.impl, &mcp.Tool{Name: "resolve_catalog_candidates", Description: "Resolve up to 50 catalog candidates for a bounded query."}, srv.resolveCatalogSDK)
	mcp.AddTool(srv.impl, &mcp.Tool{Name: "read_selected_source", Description: "Read a single page of source for the exact selection bound to the supplied cursor."}, srv.readSelectedSourceSDK)
	srv.handlers["resolve_catalog_candidates"] = resolveCatalogFn(srv.resolveCatalogDirect)
	srv.handlers["read_selected_source"] = readSelectedSourceFn(srv.readSelectedSourceDirect)
	return srv, nil
}

// ToolNames returns the registered tool names in registration
// order. Tests use it to verify the surface is exactly the two
// allowed tools.
func (s *Server) ToolNames() []string {
	out := make([]string, len(s.toolNames))
	copy(out, s.toolNames)
	return out
}

// Run blocks until the context is cancelled or the transport
// disconnects. The server uses the official StdioTransport by
// default; tests may inject a different transport through Config.
func (s *Server) Run(ctx context.Context) error {
	transport := s.transport
	if transport == nil {
		transport = &mcp.StdioTransport{}
	}
	return s.impl.Run(ctx, transport)
}

// resolveCatalogSDK is the typed handler signature required by
// mcp.AddTool. It forwards to the simpler direct handler, dropping
// the unused *mcp.CallToolRequest parameter.
func (s *Server) resolveCatalogSDK(ctx context.Context, _ *mcp.CallToolRequest, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error) {
	return s.resolveCatalogDirect(ctx, input)
}

// resolveCatalogDirect authorizes the canonical selector, builds
// the catalog query, and returns the bounded candidate set. Any
// service error is returned verbatim so the SDK marks the result
// as a tool error with the sanitized error message.
func (s *Server) resolveCatalogDirect(ctx context.Context, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, ResolveCatalogOutput{}, err
	}
	candidates, err := s.service.ResolveCatalog(ctx, catalog.Query{
		Statement:  input.Statement,
		Parameters: append([]string(nil), input.Parameters...),
		RowLimit:   catalog.MaxCandidates + 1,
	}, security.SelectorResolveCatalog)
	if err != nil {
		return nil, ResolveCatalogOutput{}, err
	}
	return nil, ResolveCatalogOutput{Candidates: candidates}, nil
}

// readSelectedSourceSDK is the typed handler signature required
// by mcp.AddTool. It forwards to the simpler direct handler.
func (s *Server) readSelectedSourceSDK(ctx context.Context, _ *mcp.CallToolRequest, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error) {
	return s.readSelectedSourceDirect(ctx, input)
}

// readSelectedSourceDirect forwards the cursor and range to the
// service. The handler never inspects, logs, or echoes the cursor;
// the cursor is the opaque server binding and stays inside the
// lease store.
func (s *Server) readSelectedSourceDirect(ctx context.Context, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	page, err := s.service.ReadSelectedSource(ctx, input.Cursor, source.Range{
		StartLine: input.StartLine,
		MaxLines:  input.MaxLines,
	})
	if err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	return nil, ReadSelectedSourceOutput{Page: page}, nil
}
