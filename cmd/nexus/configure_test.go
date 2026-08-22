package main

import (
	"context"
	"io"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/tui"
)

func TestRunCommandConfigureIsSeparateFromServe(t *testing.T) {
	old := runConfigureTUI
	defer func() { runConfigureTUI = old }()
	calls := 0
	oldVersion, oldRevision := releaseVersion, vcsRevision
	defer func() { releaseVersion, vcsRevision = oldVersion, oldRevision }()
	releaseVersion, vcsRevision = "v9.8.7", "abc123"
	runConfigureTUI = func(_ context.Context, _ configuration.ProfilesStore, build tui.BuildInfo) error {
		calls++
		if build != (tui.BuildInfo{Version: "v9.8.7", Revision: "abc123"}) {
			t.Fatalf("configure build identity = %#v", build)
		}
		return nil
	}
	if err := runCommand([]string{"configure"}, io.Discard); err != nil {
		t.Fatalf("configure returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("configure TUI calls = %d, want 1", calls)
	}
}
