package mapepirestdio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyServerJARRejectsDifferentArtifact(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "mapepire-server-2.3.5.jar")
	if err := os.WriteFile(filePath, []byte("not the approved artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyServerJAR(filePath); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestBuildCommandUsesOnlyPinnedShape(t *testing.T) {
	command, err := BuildCommand(LaunchPolicy{RemoteJAR: "/home/NEXUS/" + RemoteJar})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"QIBM_JAVA_STDIO_CONVERT=N", "'/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit/bin/java'", "-Dos400.stdio.convert=N", "--single"} {
		if !strings.Contains(command, required) {
			t.Fatalf("command omitted %q: %s", required, command)
		}
	}
	if _, err := BuildCommand(LaunchPolicy{JavaHome: "/tmp/java;id", RemoteJAR: "/home/NEXUS/" + RemoteJar}); err == nil {
		t.Fatal("expected unsafe Java path rejection")
	}
}
