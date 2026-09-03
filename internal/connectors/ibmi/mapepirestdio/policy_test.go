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

func TestBuildCommandRendersOnlyReceiptBoundJavaJARLaunch(t *testing.T) {
	receipt := newArtifactReceipt(newMemoryRemote(), "SHA256:approved-host", "/home/NEXUS/"+RemoteJar, ServerSHA256)
	command, err := BuildCommand(receipt)
	if err != nil {
		t.Fatal(err)
	}
	want := "env QIBM_JAVA_STDIO_CONVERT=N QIBM_PASE_DESCRIPTOR_STDIO=B QIBM_USE_DESCRIPTOR_STDIO=Y QIBM_MULTI_THREADED=Y /QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit/bin/java -jar /home/NEXUS/.bac-nexus/components/mapepire/2.3.6/mapepire-server.jar --single"
	if command != want {
		t.Fatalf("command = %q", command)
	}
	if strings.Contains(command, "QIBM_PASE_CCSID") {
		t.Fatal("removed CCSID override remains in the launch command")
	}
}

func TestMapepireServerURLMatchesPinnedReleaseIdentity(t *testing.T) {
	if ServerURL != "https://github.com/Mapepire-IBMi/mapepire-server/releases/download/v2.3.6/mapepire-server.jar" {
		t.Fatalf("ServerURL = %q", ServerURL)
	}
}

func TestSingleModeDiagnosticReturnsOwnedCopies(t *testing.T) {
	environment, arguments := SingleModeDiagnostic()
	environment[0] = "MUTATED=Y"
	arguments[0] = "unsafe-java"
	command, err := BuildCommand(newArtifactReceipt(newMemoryRemote(), "SHA256:approved-host", "/home/NEXUS/"+RemoteJar, ServerSHA256))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(command, "MUTATED=Y") || strings.Contains(command, "unsafe-java") {
		t.Fatalf("diagnostic mutation changed launch command %q", command)
	}
}

func TestBuildCommandRejectsZeroOrMutatedReceipt(t *testing.T) {
	if _, err := BuildCommand(VerifiedMapepireArtifactReceipt{}); err == nil {
		t.Fatal("zero receipt was accepted")
	}
	receipt := newArtifactReceipt(newMemoryRemote(), "SHA256:approved-host", "/unsafe;fragment", ServerSHA256)
	if _, err := BuildCommand(receipt); err == nil {
		t.Fatal("unsafe receipt path was accepted")
	}
}
