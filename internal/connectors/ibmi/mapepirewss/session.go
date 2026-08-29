package mapepirewss

import (
	"context"

	"bac-nexus/internal/mapepire/wss"
)

type Options struct {
	Pin  string
	TOFU bool
}

type ProofMetadata struct {
	Rows     int
	Revision string
}

type Session interface {
	Prove(context.Context, string, []byte) (ProofMetadata, error)
	Close() error
}

type Factory struct{ factory wss.Factory }

func NewFactory(endpoint string, options Options) Factory {
	return Factory{factory: wss.NewFactory(endpoint, wss.Options{Pin: options.Pin, TOFU: options.TOFU})}
}

func (f Factory) Open(ctx context.Context) (Session, error) {
	session, err := f.factory.Open(ctx)
	if err != nil {
		return nil, err
	}
	return wrappedSession{session: session}, nil
}

type wrappedSession struct{ session *wss.Session }

func (s wrappedSession) Prove(ctx context.Context, username string, credential []byte) (ProofMetadata, error) {
	proof, err := s.session.Prove(ctx, username, credential)
	if err != nil {
		return ProofMetadata{}, err
	}
	return ProofMetadata{Rows: proof.Rows, Revision: proof.Revision}, nil
}

func (s wrappedSession) Close() error { return s.session.Close() }
