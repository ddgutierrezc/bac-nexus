package main

import (
	"context"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

// step8MarkerAdapter binds configuration's bounded marker contract to profile storage.
type step8MarkerAdapter struct{ store profile.Step8MarkerStore }

func newStep8MarkerAdapter(store profile.Step8MarkerStore) step8MarkerAdapter {
	return step8MarkerAdapter{store: store}
}

func (a step8MarkerAdapter) Clear(ctx context.Context, p profile.Profile) error {
	return a.store.Clear(ctx, p)
}

func (a step8MarkerAdapter) Write(ctx context.Context, p profile.Profile, marker configuration.Marker) error {
	return a.store.Write(ctx, p, profile.Step8Marker{
		SchemaVersion: marker.SchemaVersion,
		AtUnixMs:      marker.AtUnixMs,
		Outcome:       string(marker.Outcome),
		ProofRevision: marker.ProofRevision,
	})
}
