package mapepirestdio

import "testing"

func TestArtifactPolicyRevision(t *testing.T) {
	const want = "mapepire-stdio-v2.3.6"

	if got := ArtifactPolicyRevision(); got != want {
		t.Fatalf("ArtifactPolicyRevision() = %q, want %q", got, want)
	}
}
