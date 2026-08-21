// Package mcp exposes the v1 read-only catalog-context service as a
// typed, stdio Model Context Protocol server. The package owns no
// remote, path, shell, SQL, or SSH capability of its own; it adapts
// internal/app.Service calls to the official MCP wire protocol and
// surfaces only the two allowed tools.
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
type Service interface {
	ResolveCatalog(ctx context.Context, query catalog.Query, selector security.Selector) ([]catalog.Candidate, error)
	ReadSelectedSource(ctx context.Context, cursor string, page source.Range) (source.Page, error)
}

// Info identifies the MCP server to clients.
type Info struct {
	Name    string
	Version string
}

// Config is the construction input for a Server.
type Config struct {
	Info      Info
	Service   Service
	Transport mcp.Transport // optional test override; nil → StdioTransport at Run time
}

// Server is the stdio MCP facade. It registers exactly two typed
// tools and exposes no other surface.
type Server struct {
	impl      *mcp.Server
	service   Service
	transport mcp.Transport
	toolNames []string
}

// ResolveCatalogInput is the typed MCP request for
// resolve_catalog_candidates.
type ResolveCatalogInput struct {
	Statement  string   `json:"statement" jsonschema:"bounded SQL statement to execute"`
	Parameters []string `json:"parameters,omitempty" jsonschema:"positional query parameters"`
}

// ResolveCatalogOutput is the typed MCP response for
// resolve_catalog_candidates. It carries only the bounded candidate
// coordinates; source content is never returned.
type ResolveCatalogOutput struct {
	Candidates []catalog.Candidate `json:"candidates"`
}

// ReadSelectedSourceInput is the typed MCP request for
// read_selected_source. The cursor is the only selection binding;
// no path, listing, or delete field exists.
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
// It fails closed when Service is nil.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("mcp server requires a service")
	}
	if cfg.Info.Name == "" {
		cfg.Info.Name = "bac-nexus"
	}
	if cfg.Info.Version == "" {
		cfg.Info.Version = "v0.0.0"
	}
	srv := &Server{
		service:   cfg.Service,
		transport: cfg.Transport,
		impl:      mcp.NewServer(&mcp.Implementation{Name: cfg.Info.Name, Version: cfg.Info.Version}, nil),
	}
	srv.toolNames = []string{"resolve_catalog_candidates", "read_selected_source"}
	mcp.AddTool(srv.impl, &mcp.Tool{Name: "resolve_catalog_candidates", Description: "Resolve up to 50 catalog candidates for a bounded query."}, srv.resolveCatalog)
	mcp.AddTool(srv.impl, &mcp.Tool{Name: "read_selected_source", Description: "Read a single page of source for the exact selection bound to the supplied cursor."}, srv.readSelectedSource)
	return srv, nil
}

// ToolNames returns the registered tool names in registration order.
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

func (s *Server) resolveCatalog(ctx context.Context, _ *mcp.CallToolRequest, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error) {
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

func (s *Server) readSelectedSource(ctx context.Context, _ *mcp.CallToolRequest, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	page, err := s.service.ReadSelectedSource(ctx, input.Cursor, source.Range{StartLine: input.StartLine, MaxLines: input.MaxLines})
	if err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	return nil, ReadSelectedSourceOutput{Page: page}, nil
}
