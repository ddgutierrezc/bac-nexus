// Package mapepirestdio owns the IBM i SSH/stdio launch and JAR activation
// policy for the pinned Mapepire Server artifact.
package mapepirestdio

import (
	"errors"
	"path"
	"strings"
)

const (
	ServerVersion     = "2.3.5"
	ServerSHA256      = "41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876"
	RemoteJar         = ".bac-nexus/components/mapepire/2.3.5/mapepire-server-2.3.5.jar"
	MaxServerJARBytes = 64 << 20
	DefaultJavaHome   = "/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit"
)

var SingleModeEnvironment = []string{
	"QIBM_JAVA_STDIO_CONVERT=N",
	"QIBM_PASE_DESCRIPTOR_STDIO=B",
	"QIBM_USE_DESCRIPTOR_STDIO=Y",
	"QIBM_MULTI_THREADED=Y",
}

var SingleModeJavaArguments = []string{
	"-Dos400.stdio.convert=N",
	"-jar",
	RemoteJar,
	"--single",
}

// LaunchPolicy carries the only approved Mapepire stdio command inputs.
type LaunchPolicy struct {
	JavaHome  string
	RemoteJAR string
}

func VerifyServerJAR(filePath string) error { return verifyServerJAR(filePath, ServerSHA256) }

func verifyServerJAR(filePath, expected string) error {
	file, _, err := openVerifiedLocalJAR(filePath, expected)
	if err != nil {
		return err
	}
	return file.Close()
}

func BuildCommand(policy LaunchPolicy) (string, error) {
	javaHome := policy.JavaHome
	if javaHome == "" {
		javaHome = DefaultJavaHome
	}
	if !strings.HasPrefix(javaHome, "/QOpenSys/QIBM/ProdData/JavaVM/") || strings.Contains(javaHome, "..") {
		return "", errors.New("unsafe IBM i Java home")
	}
	if !strings.HasPrefix(policy.RemoteJAR, "/") || strings.Contains(policy.RemoteJAR, "..") {
		return "", errors.New("unsafe remote Mapepire path")
	}
	java := path.Join(javaHome, "bin", "java")
	return strings.Join(SingleModeEnvironment, " ") + " " + shellQuote(java) + " -Dos400.stdio.convert=N -jar " + shellQuote(policy.RemoteJAR) + " --single", nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
