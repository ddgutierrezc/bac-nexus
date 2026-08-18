package mapepire

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
)

const (
	ServerVersion     = "2.3.5"
	ServerSHA256      = "41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876"
	RemoteJar         = ".bac-nexus/components/mapepire/2.3.5/mapepire-server-2.3.5.jar"
	MaxFrameBytes     = 1 << 20
	MaxServerJARBytes = 64 << 20
)

const DefaultJavaHome = "/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit"

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

type Request struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Technique   string   `json:"technique,omitempty"`
	Application string   `json:"application,omitempty"`
	Props       string   `json:"props,omitempty"`
	SQL         string   `json:"sql,omitempty"`
	Rows        int      `json:"rows,omitempty"`
	Parameters  []string `json:"parameters,omitempty"`
	ContID      string   `json:"cont_id,omitempty"`
}

func ConnectRequest(id string) Request {
	return Request{ID: id, Type: "connect", Technique: "tcp", Application: "BAC Nexus catalog spike", Props: "access=read only"}
}

func PreparedQueryRequest(id, sql string, rows int, parameters []string) Request {
	return Request{ID: id, Type: "prepare_sql_execute", SQL: sql, Rows: rows, Parameters: parameters}
}

func CloseCursorRequest(id, correlationID string) Request {
	return Request{ID: id, Type: "sqlclose", ContID: correlationID}
}

func ExitRequest(id string) Request { return Request{ID: id, Type: "exit"} }

func Encode(request Request) ([]byte, error) {
	var output bytes.Buffer
	if err := json.NewEncoder(&output).Encode(request); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func DecodeFrame(reader io.Reader, target any) error {
	data := make([]byte, 0, 1024)
	var next [1]byte
	for {
		n, err := reader.Read(next[:])
		if n > 0 {
			if len(data)+1 > MaxFrameBytes {
				return errors.New("Mapepire frame exceeds byte limit")
			}
			if next[0] == '\n' {
				return json.Unmarshal(bytes.TrimSpace(data), target)
			}
			data = append(data, next[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("Mapepire frame ended before newline terminator")
			}
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
}

func VerifyServerJAR(path string) error {
	return verifyServerJAR(path, ServerSHA256)
}

func verifyServerJAR(path, expected string) error {
	file, _, err := openVerifiedLocalJAR(path, expected)
	if err != nil {
		return err
	}
	return file.Close()
}

func JavaCommand(javaHome, remoteJar string) (string, error) {
	if javaHome == "" {
		javaHome = DefaultJavaHome
	}
	if !strings.HasPrefix(javaHome, "/QOpenSys/QIBM/ProdData/JavaVM/") || strings.Contains(javaHome, "..") {
		return "", errors.New("unsafe IBM i Java home")
	}
	if !strings.HasPrefix(remoteJar, "/") || strings.Contains(remoteJar, "..") {
		return "", errors.New("unsafe remote Mapepire path")
	}
	java := path.Join(javaHome, "bin", "java")
	return strings.Join(SingleModeEnvironment, " ") + " " + shellQuote(java) + " -Dos400.stdio.convert=N -jar " + shellQuote(remoteJar) + " --single", nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
