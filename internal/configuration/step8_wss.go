package configuration

import (
	"context"
	"errors"
	"net"
	"strconv"

	"bac-nexus/internal/connectors/ibmi/mapepirewss"
	"bac-nexus/internal/profile"
)

type managedStep8WSSOpener interface {
	Open(context.Context) (Step8WSSSession, error)
}

type managedStep8WSSOpenFunc func(context.Context) (Step8WSSSession, error)

func (f managedStep8WSSOpenFunc) Open(ctx context.Context) (Step8WSSSession, error) {
	return f(ctx)
}

type managedStep8WSSFactory struct{ factory mapepirewss.Factory }

func (f managedStep8WSSFactory) Open(ctx context.Context) (Step8WSSSession, error) {
	session, err := f.factory.Open(ctx)
	if err != nil {
		return nil, err
	}
	return managedStep8WSSSession{session: session}, nil
}

type managedStep8WSSSession struct{ session mapepirewss.Session }

func (s managedStep8WSSSession) Prove(ctx context.Context, username string, credential []byte) (ProofMetadata, error) {
	proof, err := s.session.Prove(ctx, username, credential)
	if err != nil {
		return ProofMetadata{}, err
	}
	return ProofMetadata{Rows: proof.Rows, ProofRevision: proof.Revision}, nil
}

func (s managedStep8WSSSession) Close() error { return s.session.Close() }

// ManagedStep8WSS binds a saved profile's managed WSS endpoint and independent
// TLS evidence to the fixed-proof session boundary owned by Step8Service.
type ManagedStep8WSS struct {
	newFactory func(string, mapepirewss.Options) managedStep8WSSOpener
}

func NewManagedStep8WSS() ManagedStep8WSS {
	return ManagedStep8WSS{
		newFactory: func(endpoint string, options mapepirewss.Options) managedStep8WSSOpener {
			return managedStep8WSSFactory{factory: mapepirewss.NewFactory(endpoint, options)}
		},
	}
}

func (a ManagedStep8WSS) Open(ctx context.Context, p profile.Profile) (Step8WSSSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ValidateStep8Profile(p) != nil || a.newFactory == nil {
		return nil, errors.New("managed Step 8 WSS configuration is invalid")
	}
	endpoint := "wss://" + net.JoinHostPort(p.Host, strconv.Itoa(managedDaemonPort))
	opener := a.newFactory(endpoint, mapepirewss.Options{
		Pin:  p.TLSTrust.Pin,
		TOFU: p.TLSTrust.Mode == profile.TrustModeTOFU,
	})
	if opener == nil {
		return nil, errors.New("managed Step 8 WSS factory is unavailable")
	}
	return opener.Open(ctx)
}
