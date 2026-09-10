package mcp

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/codefori"
	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
	"bac-nexus/internal/source"
)

// CodeForIConfig is the isolated construction input for the Companion MCP surface.
type CodeForIConfig struct {
	Info      Info
	Provider  provider.Provider
	Inspector inspection.Provider
	Catalog   CatalogProvider
	Source    SourceProvider
	Transport sdk.Transport
}

// CatalogProvider is the Companion server's metadata-only catalog boundary.
type CatalogProvider interface {
	ResolveCatalog(context.Context, catalog.Search) ([]catalog.Candidate, error)
}

// SourceProvider is the narrow Companion boundary for paged source artifacts.
type SourceProvider interface {
	PageSource(context.Context, codefori.SourcePageRequest) (codefori.SourcePageResult, error)
	DisposeSource(context.Context, string) (codefori.SourceDisposeResult, error)
}

// CodeForIServer is the stdio MCP facade for the isolated Companion proof surface.
type CodeForIServer struct {
	impl        *sdk.Server
	provider    provider.Provider
	inspection  *inspection.Service
	catalog     CatalogProvider
	source      SourceProvider
	transport   sdk.Transport
	toolNames   []string
	liveCursors map[string]struct{}

	lifecycleMu sync.Mutex
	running     bool
	stopping    bool
	handlers    sync.WaitGroup
}

type ResolveProgramInput struct {
	Name    string `json:"name" jsonschema:"required IBM i program name"`
	Library string `json:"library,omitempty" jsonschema:"optional explicit IBM i library"`
}
type ResolveProgramOutput struct {
	inspection.ResolveResult
	Selection string `json:"selection,omitempty"`
}
type FindProgramSourceInput struct {
	Selection string `json:"selection" jsonschema:"opaque resolved program selection"`
}
type FindProgramSourceOutput struct{ inspection.SourceResult }

// CodeForISessionStatusInput accepts only the empty object.
type CodeForISessionStatusInput struct{}

// CodeForISessionStatusOutput is the bounded Companion status result.
type CodeForISessionStatusOutput struct {
	State provider.SessionState `json:"state"`
}

// CodeForIQueryInput contains the only supported Companion query input.
type CodeForIQueryInput struct {
	SQL string `json:"sql" jsonschema:"the bounded proof query"`
}

// CodeForIQueryOutput is the bounded normalized Companion query result.
type CodeForIQueryOutput struct {
	State provider.QueryState     `json:"state"`
	Rows  []CodeForINormalizedRow `json:"rows,omitempty"`
}

// CodeForINormalizedRow deliberately exposes only the normalized value.
type CodeForINormalizedRow struct {
	Value string `json:"value"`
}

// companionReadSelectedSourceInput permits the cursor-only continuation shape.
type companionReadSelectedSourceInput struct {
	Selection catalog.Candidate `json:"selection,omitempty" jsonschema:"exact catalog selection; required only on the first page"`
	Cursor    string            `json:"cursor,omitempty" jsonschema:"opaque snapshot cursor for later pages"`
	StartLine int               `json:"startLine" jsonschema:"one-based inclusive start line"`
	MaxLines  int               `json:"maxLines" jsonschema:"maximum lines in this page"`
}

// NewCodeForI constructs the Companion-only MCP server and registers no Native tools.
func NewCodeForI(cfg CodeForIConfig) (*CodeForIServer, error) {
	if cfg.Info.Name == "" {
		cfg.Info.Name = "bac-nexus"
	}
	if cfg.Info.Version == "" {
		cfg.Info.Version = "v0.0.0"
	}
	server := &CodeForIServer{
		provider:    cfg.Provider,
		inspection:  inspection.NewService(cfg.Inspector, inspection.NewSelectionStore()),
		catalog:     cfg.Catalog,
		source:      cfg.Source,
		transport:   cfg.Transport,
		impl:        sdk.NewServer(&sdk.Implementation{Name: cfg.Info.Name, Version: cfg.Info.Version}, nil),
		toolNames:   []string{"session_status", "sql_query", "resolve_program", "find_program_source", "resolve_catalog_candidates", "read_selected_source"},
		liveCursors: make(map[string]struct{}),
	}
	sdk.AddTool(server.impl, &sdk.Tool{Name: "session_status", Description: "Return the bounded Companion session state."}, server.sessionStatus)
	sdk.AddTool(server.impl, &sdk.Tool{Name: "sql_query", Description: "Run the one bounded Companion proof query."}, server.query)
	openWorld := true
	sdk.AddTool(server.impl, &sdk.Tool{Name: "resolve_program", Description: "Resolve a supported IBM i program using configured Code for IBM i context, not runtime *LIBL.", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}}, server.resolveProgram)
	sdk.AddTool(server.impl, &sdk.Tool{Name: "find_program_source", Description: "Return metadata-only evidenced source coordinates for an opaque resolved program selection.", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &openWorld}}, server.findProgramSource)
	sdk.AddTool(server.impl, &sdk.Tool{Name: "resolve_catalog_candidates", Description: "Resolve up to 50 ordered catalog candidates for a bounded query.", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &openWorld}}, server.resolveCatalog)
	sdk.AddTool(server.impl, &sdk.Tool{Name: "read_selected_source", Description: "Read a bounded page from an explicitly selected Catalogados source member.", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}}, server.readSelectedSource)
	return server, nil
}

func (s *CodeForIServer) readSelectedSource(ctx context.Context, _ *sdk.CallToolRequest, input companionReadSelectedSourceInput) (*sdk.CallToolResult, ReadSelectedSourceOutput, error) {
	if !s.acceptHandler() {
		return nil, ReadSelectedSourceOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	if err := ctx.Err(); err != nil {
		return nil, ReadSelectedSourceOutput{}, err
	}
	if input.StartLine < 1 || input.MaxLines < 1 || input.MaxLines > source.MaxPageLines || (input.Cursor != "" && input.Selection != (catalog.Candidate{})) || (input.Cursor == "" && input.Selection == (catalog.Candidate{})) {
		return nil, ReadSelectedSourceOutput{}, source.ErrInvalidRequest
	}
	if s.source == nil {
		return nil, ReadSelectedSourceOutput{}, codefori.ErrSourceUnavailable
	}
	request := codefori.SourcePageRequest{Cursor: input.Cursor, StartLine: input.StartLine, MaxLines: input.MaxLines}
	if input.Cursor == "" {
		request.Candidate = &input.Selection
	}
	result, err := s.source.PageSource(ctx, request)
	if err != nil {
		return nil, ReadSelectedSourceOutput{}, sanitizeSourceError(err)
	}
	if result.State != codefori.SourceOK {
		return nil, ReadSelectedSourceOutput{}, sourceStateError(result.State)
	}
	page, err := companionSourcePage(result, input.StartLine)
	if err != nil {
		_ = s.disposeSource(result.Cursor)
		return nil, ReadSelectedSourceOutput{}, err
	}
	if page.EOF {
		if err := s.disposeSource(result.Cursor); err != nil {
			return nil, ReadSelectedSourceOutput{}, err
		}
		return nil, ReadSelectedSourceOutput{Page: page}, nil
	}
	s.trackCursor(result.Cursor)
	return nil, ReadSelectedSourceOutput{Page: page}, nil
}

func companionSourcePage(result codefori.SourcePageResult, startLine int) (source.Page, error) {
	lines := []string(nil)
	if result.Page.Content != "" {
		lines = strings.Split(result.Page.Content, "\n")
	}
	if len(lines) > 0 && strings.HasSuffix(result.Page.Content, "\n") {
		lines = lines[:len(lines)-1]
	}
	if result.Page.StartLine != startLine || result.Page.LineCount != len(lines) || (len(lines) == 0 && !result.Page.EOF) {
		return source.Page{}, codefori.ErrSourceFailed
	}
	page := source.Page{StartLine: startLine, LineCount: len(lines), Lines: lines, EOF: result.Page.EOF}
	if !page.EOF {
		page.NextStartLine = startLine + len(lines)
		page.Cursor = result.Cursor
	}
	return page, nil
}

func sourceStateError(state codefori.SourceState) error {
	switch state {
	case codefori.SourceInvalidRequest:
		return source.ErrInvalidRequest
	case codefori.SourceNotFound:
		return catalog.ErrCandidateNotFound
	case codefori.SourceAmbiguous:
		return &catalog.AmbiguousError{}
	case codefori.SourceExpired:
		return source.ErrExpiredLease
	case codefori.SourceInvalidEncoding:
		return source.ErrInvalidSourceEncoding
	case codefori.SourceResponseTooLarge:
		return source.ErrResponseTooLarge
	case codefori.SourceUnavailable:
		return codefori.ErrSourceUnavailable
	default:
		return codefori.ErrSourceFailed
	}
}

func sanitizeSourceError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, codefori.ErrSourceInvalidRequest) || errors.Is(err, codefori.ErrSourceUnavailable) || errors.Is(err, codefori.ErrSourceFailed) {
		return err
	}
	return codefori.ErrSourceFailed
}

func (s *CodeForIServer) resolveCatalog(ctx context.Context, _ *sdk.CallToolRequest, input ResolveCatalogInput) (*sdk.CallToolResult, ResolveCatalogOutput, error) {
	if !s.acceptHandler() {
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
	if s.catalog == nil {
		return nil, ResolveCatalogOutput{}, codefori.ErrCatalogUnavailable
	}
	candidates, err := s.catalog.ResolveCatalog(ctx, search)
	if err != nil {
		return nil, ResolveCatalogOutput{}, sanitizeCatalogError(err)
	}
	if len(candidates) == 0 {
		return nil, ResolveCatalogOutput{}, catalog.ErrCandidateNotFound
	}
	return nil, ResolveCatalogOutput{Candidates: candidates}, nil
}

func sanitizeCatalogError(err error) error {
	switch {
	case errors.Is(err, catalog.ErrCandidateNotFound), errors.Is(err, catalog.ErrCandidateLimit), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, codefori.ErrCatalogUnavailable), errors.Is(err, codefori.ErrCatalogFailed):
		return err
	default:
		return codefori.ErrCatalogFailed
	}
}

func (s *CodeForIServer) resolveProgram(ctx context.Context, _ *sdk.CallToolRequest, input ResolveProgramInput) (*sdk.CallToolResult, ResolveProgramOutput, error) {
	if !s.acceptHandler() {
		return nil, ResolveProgramOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	request, err := inspection.NewResolveRequest(input.Name, input.Library)
	if err != nil {
		return nil, ResolveProgramOutput{}, err
	}
	result := s.inspection.Resolve(ctx, request)
	output := ResolveProgramOutput{ResolveResult: result}
	if result.State == inspection.StateResolved && len(result.Matches) == 1 {
		output.Selection, err = s.inspection.IssueSelection(result.Matches[0])
		if err != nil {
			reason := "selection_unavailable"
			if errors.Is(err, inspection.ErrSelectionCapacity) {
				reason = "selection_capacity_exceeded"
			}
			output = ResolveProgramOutput{ResolveResult: inspection.ResolveResult{State: inspection.StateUnavailable, Completeness: "complete", Reason: reason}}
		}
	}
	return nil, normalizeResolveProgramOutput(output), nil
}

// normalizeResolveProgramOutput satisfies the public MCP schema at the
// serialization boundary without changing inspection-provider semantics.
func normalizeResolveProgramOutput(output ResolveProgramOutput) ResolveProgramOutput {
	if output.LibrariesSearched == nil {
		output.LibrariesSearched = []string{}
	}
	if output.Matches == nil {
		output.Matches = []inspection.ResolvedProgram{}
	}
	if output.RequiredDecision != nil && output.RequiredDecision.Options == nil {
		output.RequiredDecision.Options = []inspection.ResolvedProgram{}
	}
	return output
}

func (s *CodeForIServer) findProgramSource(ctx context.Context, _ *sdk.CallToolRequest, input FindProgramSourceInput) (*sdk.CallToolResult, FindProgramSourceOutput, error) {
	if !s.acceptHandler() {
		return nil, FindProgramSourceOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	return nil, FindProgramSourceOutput{SourceResult: s.inspection.FindSource(ctx, input.Selection)}, nil
}

// ToolNames returns the registered Companion tool names in registration order.
func (s *CodeForIServer) ToolNames() []string {
	out := make([]string, len(s.toolNames))
	copy(out, s.toolNames)
	return out
}

// Run blocks until the context is cancelled or the MCP transport disconnects.
func (s *CodeForIServer) Run(ctx context.Context) error {
	transport := s.transport
	if transport == nil {
		transport = &sdk.StdioTransport{}
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
		if cleanupErr := s.disposeLiveSources(); cleanupErr != nil {
			return ErrLifecycleUnavailable
		}
		if err != nil {
			return ErrLifecycleUnavailable
		}
		return nil
	case <-ctx.Done():
		s.stopIntake()
		_ = session.Close()
		<-ended
		s.handlers.Wait()
		if cleanupErr := s.disposeLiveSources(); cleanupErr != nil {
			return ErrLifecycleUnavailable
		}
		return ctx.Err()
	}
}

func (s *CodeForIServer) sessionStatus(ctx context.Context, _ *sdk.CallToolRequest, _ CodeForISessionStatusInput) (*sdk.CallToolResult, CodeForISessionStatusOutput, error) {
	if !s.acceptHandler() {
		return nil, CodeForISessionStatusOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	if err := ctx.Err(); err != nil {
		return nil, CodeForISessionStatusOutput{}, err
	}
	if s.provider == nil {
		return nil, CodeForISessionStatusOutput{State: provider.SessionCompanionUnavailable}, nil
	}
	status := s.provider.SessionStatus(ctx)
	if !status.State.Valid() {
		status.State = provider.SessionCompanionUnavailable
	}
	return nil, CodeForISessionStatusOutput{State: status.State}, nil
}

func (s *CodeForIServer) query(ctx context.Context, _ *sdk.CallToolRequest, input CodeForIQueryInput) (*sdk.CallToolResult, CodeForIQueryOutput, error) {
	if !s.acceptHandler() {
		return nil, CodeForIQueryOutput{}, errors.New("mcp server unavailable")
	}
	defer s.finishHandler()
	result := provider.RunProofQuery(ctx, s.provider, provider.QueryRequest{SQL: input.SQL})
	output := CodeForIQueryOutput{State: result.State}
	if result.State == provider.QueryOK {
		output.Rows = []CodeForINormalizedRow{{Value: result.Rows[0].Value}}
	}
	return nil, output, nil
}

func (s *CodeForIServer) start() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.running {
		return errors.New("mcp server already running")
	}
	s.running = true
	s.stopping = false
	return nil
}

func (s *CodeForIServer) stopIntake() {
	s.lifecycleMu.Lock()
	s.stopping = true
	s.lifecycleMu.Unlock()
}

func (s *CodeForIServer) stop() {
	s.stopIntake()
	s.lifecycleMu.Lock()
	s.running = false
	s.lifecycleMu.Unlock()
}

func (s *CodeForIServer) acceptHandler() bool {
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

func (s *CodeForIServer) finishHandler() {
	s.lifecycleMu.Lock()
	running := s.running
	s.lifecycleMu.Unlock()
	if running {
		s.handlers.Done()
	}
}

func (s *CodeForIServer) trackCursor(cursor string) {
	s.lifecycleMu.Lock()
	s.liveCursors[cursor] = struct{}{}
	s.lifecycleMu.Unlock()
}

func (s *CodeForIServer) disposeSource(cursor string) error {
	result, err := s.source.DisposeSource(context.Background(), cursor)
	if err != nil || result.State != codefori.SourceDisposed {
		return codefori.ErrSourceFailed
	}
	s.lifecycleMu.Lock()
	delete(s.liveCursors, cursor)
	s.lifecycleMu.Unlock()
	return nil
}

func (s *CodeForIServer) disposeLiveSources() error {
	s.lifecycleMu.Lock()
	cursors := make([]string, 0, len(s.liveCursors))
	for cursor := range s.liveCursors {
		cursors = append(cursors, cursor)
	}
	s.lifecycleMu.Unlock()
	sort.Strings(cursors)
	var failed bool
	for _, cursor := range cursors {
		if s.disposeSource(cursor) != nil {
			failed = true
		}
	}
	if failed {
		return codefori.ErrSourceFailed
	}
	return nil
}
