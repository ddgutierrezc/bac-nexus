package wss

import (
	"context"

	"bac-nexus/internal/mapepire"
)

// Factory opens only the trusted, unauthenticated WSS transport. Credentials
// are deliberately absent from this boundary and enter only through Prove.
type Factory struct {
	Endpoint    string
	Options     Options
	Application string
}

func NewFactory(endpoint string, options Options) Factory {
	return Factory{Endpoint: endpoint, Options: options, Application: "BAC Nexus"}
}

func (f Factory) Open(ctx context.Context) (*Session, error) {
	transport, err := Dial(ctx, f.Endpoint, f.Options)
	if err != nil {
		return nil, err
	}
	return &Session{transport: transport, client: mapepire.NewMessageSession(transport, f.Application)}, nil
}

type Session struct {
	transport *Transport
	client    *mapepire.Client
}

func (s *Session) Prove(ctx context.Context, username string, password []byte) (mapepire.ProofMetadata, error) {
	if s == nil || s.client == nil {
		return mapepire.ProofMetadata{}, mapepire.ErrSessionClosed
	}
	return s.client.FixedProof(ctx, username, password)
}

func (s *Session) Close() error {
	if s == nil || s.transport == nil {
		return nil
	}
	return s.transport.Close()
}
