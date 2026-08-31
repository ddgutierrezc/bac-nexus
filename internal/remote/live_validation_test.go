package remote

import "testing"

func TestLiveValidationSkipsWithoutApprovedConfigurationAndNeverAttemptsConnection(t *testing.T) {
	lookups := 0
	lookup := func(string) (string, bool) {
		lookups++
		return "", false
	}
	if LiveValidationEnabled(lookup, "BAC_NEXUS_SSH_INTEGRATION") {
		t.Fatal("absent approved configuration enabled live validation")
	}
	if lookups != 1 {
		t.Fatalf("configuration lookup calls = %d, want 1", lookups)
	}
}
