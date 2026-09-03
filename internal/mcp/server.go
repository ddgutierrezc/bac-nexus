// Package mcp exposes the v1 read-only catalog-context service as a
// typed, stdio Model Context Protocol server. The package owns no
// remote, path, shell, SQL, or SSH capability of its own; it adapts
// internal/app.Service calls to the official MCP wire protocol and
// surfaces only the two allowed tools.
package mcp

import (
	"context"
	"errors"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// ErrLifecycleUnavailable is the stable public classification for MCP
// transport and session lifecycle failures. Raw peer or SDK errors must not
// cross the process diagnostic boundary.
var ErrLifecycleUnavailable = errors.New("mcp lifecycle unavailable")

// Service is the narrow dependency surface the MCP server requires.
type Service interface {
	ResolveCatalog(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error)
	ReadSelectedSource(ctx context.Context, selection catalog.Candidate, cursor string, page source.Range) (source.Page, error)
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

	lifecycleMu sync.Mutex
	running     bool
	stopping    bool
	handlers    sync.WaitGroup
}

// ResolveCatalogInput is the typed MCP request for
// resolve_catalog_candidates.
type ResolveCatalogInput struct {
	Item              string `json:"item" jsonschema:"catalog item name"`
	ProductionLibrary string `json:"productionLibrary,omitempty" jsonschema:"optional production library name"`
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
	Selection catalog.Candidate `json:"selection" jsonschema:"exact catalog selection; required only on the first page"`
	Cursor    string            `json:"cursor,omitempty" jsonschema:"opaque snapshot cursor for later pages"`
	StartLine int               `json:"startLine" jsonschema:"one-based inclusive start line"`
	MaxLines  int               `json:"maxLines" jsonschema:"maximum lines in this page"`
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

// Run blocks until the context is cancelled or the transport disconnects.
// It stops intake before waiting for accepted handlers so cancellation cannot
// truncate a source response that was already admitted. The server uses the
// official StdioTransport by default; tests may inject a different transport
// through Config.
func (s *Server) Run(ctx context.Context) error {
	transport := s.transport
	if transport == nil {
		transport = &mcp.StdioTransport{}
	}
	if err := s.start(); err != nil {
		return err
	}
	defer s.stop()

	session, err := s.impl.Connect(ctx, transport, nil)
	if err != nil {
		return ErrLifecycleUnavailable
	}
	ended := make(chan error, 1)
	go func() { ended <- session.Wait() }()

	select {
	case err := <-ended:
		s.stopIntake()
		if closeErr := session.Close(); closeErr != nil {
			return ErrLifecycleUnavailable
		}
		s.handlers.Wait()
		if err != nil {
			return ErrLifecycleUnavailable
		}
		return nil
	case <-ctx.Done():
		s.stopIntake()
		// The session inherits ctx, so cancellation reaches accepted request
		// contexts before Close waits for them. Its raw lifecycle result is not
		// safe to expose; the caller receives the deterministic context result.
		_ = session.Close()
		<-ended
		s.handlers.Wait()
		return ctx.Err()
	}
}

func (s *Server) resolveCatalog(ctx context.Context, _ *mcp.CallToolRequest, input ResolveCatalogInput) (*mcp.CallToolResult, ResolveCatalogOutput, error) {
	accepted := s.acceptHandler()
	if !accepted {
		return nil, ResolveCatalogOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	if err := ctx.Err(); err != nil {
		return nil, ResolveCatalogOutput{}, err
	}
	search, err := catalog.NewSearch(input.Item, input.ProductionLibrary)
	if err != nil {
		return nil, ResolveCatalogOutput{}, err
	}
	candidates, err := s.service.ResolveCatalog(ctx, search, security.SelectorResolveCatalog)
	if err != nil {
		return nil, ResolveCatalogOutput{}, err
	}
	return nil, ResolveCatalogOutput{Candidates: candidates}, nil
}

func (s *Server) readSelectedSource(ctx context.Context, _ *mcp.CallToolRequest, input ReadSelectedSourceInput) (*mcp.CallToolResult, ReadSelectedSourceOutput, error) {
	accepted := s.acceptHandler()
	if !accepted {
		return nil, ReadSelectedSourceOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	if err := ctx.Err(); err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	page, err := s.service.ReadSelectedSource(ctx, input.Selection, input.Cursor, source.Range{StartLine: input.StartLine, MaxLines: input.MaxLines})
	if err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	return nil, ReadSelectedSourceOutput{Page: page}, nil
}

func (s *Server) start() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.running {
		return errors.New("mcp server already running")
	}
	s.running = true
	s.stopping = false
	return nil
}

func (s *Server) stopIntake() {
	s.lifecycleMu.Lock()
	s.stopping = true
	s.lifecycleMu.Unlock()
}

func (s *Server) stop() {
	s.stopIntake()
	s.lifecycleMu.Lock()
	s.running = false
	s.lifecycleMu.Unlock()
}

func (s *Server) acceptHandler() bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.running {
		return true
	}
	if s.stopping {
		return false
	}
	s.handlers.Add(1)
	return true
}

func (s *Server) finishHandler() {
	s.lifecycleMu.Lock()
	running := s.running
	s.lifecycleMu.Unlock()
	if running {
		s.handlers.Done()
	}
}
