package main

import (
	"context"
	"errors"
	"fmt"

	"bac-nexus/internal/connectors/ibmi/codefori"
	internalmcp "bac-nexus/internal/mcp"
	"bac-nexus/internal/provider"
)

type serveMode string

const (
	serveModeCompanion serveMode = "companion"
	serveModeNative    serveMode = "native"
)

type companionDeps struct {
	Provider      provider.Provider
	ServerFactory func(*internalmcp.CodeForIServer) (runner, error)
}

var runNativeServe = runNativeServeComposition
var runCompanionServe = runCompanionServeComposition

func selectServeMode(profileName string) (serveMode, error) {
	if profileName != "" {
		return serveModeNative, nil
	}
	return serveModeCompanion, nil
}

func runNativeServeComposition(ctx context.Context, profileName string) error {
	deps := defaultDeps()
	deps.Profile = profileName
	return runWithDeps(ctx, deps)
}

func runCompanionServeComposition(ctx context.Context) error {
	return runCompanionWithDeps(ctx, defaultCompanionDeps())
}

func defaultCompanionDeps() companionDeps {
	return companionDeps{
		Provider: codefori.NewClient(),
		ServerFactory: func(server *internalmcp.CodeForIServer) (runner, error) {
			return server, nil
		},
	}
}

func runCompanionWithDeps(ctx context.Context, deps companionDeps) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deps.ServerFactory == nil {
		return errors.New("serve companion composition unavailable")
	}
	server, err := internalmcp.NewCodeForI(internalmcp.CodeForIConfig{
		Info:     internalmcp.Info{Name: "bac-nexus", Version: "v0.0.0"},
		Provider: deps.Provider,
	})
	if err != nil {
		return fmt.Errorf("build companion mcp server: %w", err)
	}
	r, err := deps.ServerFactory(server)
	if err != nil {
		return fmt.Errorf("build companion mcp server: %w", err)
	}
	if err := r.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return errServeMCPUnavailable
	}
	return nil
}
