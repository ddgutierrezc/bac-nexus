package main

import (
	"context"
	"io"
	"testing"

	"bac-nexus/internal/configuration"
)

func TestRunCommandConfigureIsSeparateFromServe(t *testing.T) {
	old := runConfigureTUI
	defer func() { runConfigureTUI = old }()
	called := false
	runConfigureTUI = func(_ context.Context, _ configuration.ProfilesStore) error { called = true; return nil }
	if err := runCommand([]string{"configure"}, io.Discard); err != nil {
		t.Fatalf("configure returned error: %v", err)
	}
	if !called {
		t.Fatal("configure did not invoke the TUI adapter")
	}
}
