package mcp

import (
	"context"
	"errors"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
)

// CodeForIConfig is the isolated construction input for the Companion MCP surface.
type CodeForIConfig struct {
	Info      Info
	Provider  provider.Provider
	Inspector inspection.Provider
	Transport sdk.Transport
}

// CodeForIServer is the stdio MCP facade for the isolated Companion proof surface.
type CodeForIServer struct {
	impl       *sdk.Server
	provider   provider.Provider
	inspection *inspection.Service
	transport  sdk.Transport
	toolNames  []string

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

// NewCodeForI constructs the Companion-only MCP server and registers no Native tools.
func NewCodeForI(cfg CodeForIConfig) (*CodeForIServer, error) {
	if cfg.Info.Name == "" {
		cfg.Info.Name = "bac-nexus"
	}
	if cfg.Info.Version == "" {
		cfg.Info.Version = "v0.0.0"
	}
	server := &CodeForIServer{
		provider:   cfg.Provider,
		inspection: inspection.NewService(cfg.Inspector, inspection.NewSelectionStore()),
		transport:  cfg.Transport,
		impl:       sdk.NewServer(&sdk.Implementation{Name: cfg.Info.Name, Version: cfg.Info.Version}, nil),
		toolNames:  []string{"session_status", "sql_query", "resolve_program", "find_program_source"},
	}
	sdk.AddTool(server.impl, &sdk.Tool{Name: "session_status", Description: "Return the bounded Companion session state."}, server.sessionStatus)
	sdk.AddTool(server.impl, &sdk.Tool{Name: "sql_query", Description: "Run the one bounded Companion proof query."}, server.query)
	openWorld := true
	sdk.AddTool(server.impl, &sdk.Tool{Name: "resolve_program", Description: "Resolve a supported IBM i program using configured Code for IBM i context, not runtime *LIBL.", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}}, server.resolveProgram)
	sdk.AddTool(server.impl, &sdk.Tool{Name: "find_program_source", Description: "Return metadata-only evidenced source coordinates for an opaque resolved program selection.", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &openWorld}}, server.findProgramSource)
	return server, nil
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
	return nil, output, nil
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
		if err != nil {
			return ErrLifecycleUnavailable
		}
		return nil
	case <-ctx.Done():
		s.stopIntake()
		_ = session.Close()
		<-ended
		s.handlers.Wait()
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
