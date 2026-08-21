//go:build darwin

package credential

import (
	"strings"
	"testing"
)

func TestMacOSSetCommandUsesFixedSecurityAndEncodedStdin(t *testing.T) {
	command, input := macOSSetCommand("production-1", []byte("native-secret"))

	if command.Path != "/usr/bin/security" {
		t.Fatalf("command path = %q, want fixed security executable", command.Path)
	}
	if got, want := command.Args, []string{"-i"}; !sameStrings(got, want) {
		t.Fatalf("command args = %v, want %v", got, want)
	}
	if strings.Contains(input, "native-secret") {
		t.Fatalf("stdin command contains plaintext secret: %q", input)
	}
	if !strings.Contains(input, "add-generic-password") || !strings.Contains(input, "ibmi/production-1") {
		t.Fatalf("stdin command = %q, want encoded exact set operation", input)
	}
}
