// Package mapepirestdio owns the IBM i SSH/stdio launch and JAR activation
// policy for the pinned Mapepire Server artifact.
package mapepirestdio

import (
	"errors"
	"strings"
)

const (
	ServerVersion     = "2.3.6"
	ServerSHA256      = "6371d64f5684fcbee96f27107512f712fc1676ffded00726f2752dcfc30977b7"
	ServerURL         = "https://github.com/Mapepire-IBMi/mapepire-server/releases/download/v2.3.6/mapepire-server.jar"
	RemoteJar         = ".bac-nexus/components/mapepire/2.3.6/mapepire-server.jar"
	MaxServerJARBytes = 64 << 20
	DefaultJavaHome   = "/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit"
)

const (
	ServerExecutable = DefaultJavaHome + "/bin/java"
	serverEnvCommand = "env"
)

var singleModeEnvironment = [...]string{"QIBM_JAVA_STDIO_CONVERT=N", "QIBM_PASE_DESCRIPTOR_STDIO=B", "QIBM_USE_DESCRIPTOR_STDIO=Y", "QIBM_MULTI_THREADED=Y"}

func VerifyServerJAR(filePath string) error { return verifyServerJAR(filePath, ServerSHA256) }

// SingleModeDiagnostic returns owned diagnostic values that cannot alter the
// private launch policy used by BuildCommand.
func SingleModeDiagnostic() (environment, arguments []string) {
	environment = append([]string(nil), singleModeEnvironment[:]...)
	arguments = []string{ServerExecutable, "-jar", "--single"}
	return environment, arguments
}

func ValidateJavaHome(javaHome string) error {
	if javaHome == "" {
		javaHome = DefaultJavaHome
	}
	if !strings.HasPrefix(javaHome, "/QOpenSys/QIBM/ProdData/JavaVM/") || strings.Contains(javaHome, "..") {
		return errors.New("unsafe IBM i Java home")
	}
	return nil
}

func verifyServerJAR(filePath, expected string) error {
	file, _, err := openVerifiedLocalJAR(filePath, expected)
	if err != nil {
		return err
	}
	return file.Close()
}

// BuildCommand renders only an issued receipt; callers cannot choose launch
// values, path, environment, Java executable, or arguments.
func BuildCommand(receipt VerifiedMapepireArtifactReceipt) (string, error) {
	remotePath, err := receipt.commandPath()
	if err != nil {
		return "", err
	}
	argv := make([]string, 0, len(singleModeEnvironment)+4)
	argv = append(argv, serverEnvCommand)
	argv = append(argv, singleModeEnvironment[:]...)
	argv = append(argv, ServerExecutable, "-jar", remotePath, "--single")
	for _, token := range argv {
		if !safeShellToken(token) {
			return "", errors.New("unsafe Mapepire SSH launch token")
		}
	}
	return strings.Join(argv, " "), nil
}

func safeRemoteJARPath(remotePath string) bool {
	return strings.HasPrefix(remotePath, "/") && !strings.Contains(remotePath, "..") && strings.HasSuffix(remotePath, "/"+RemoteJar) && safeShellToken(remotePath)
}

func safeShellToken(token string) bool {
	if token == "" {
		return false
	}
	for _, value := range token {
		if !(value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || strings.ContainsRune("/._=-", value)) {
			return false
		}
	}
	return true
}
