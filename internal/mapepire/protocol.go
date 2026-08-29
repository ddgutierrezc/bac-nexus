package mapepire

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	MaxFrameBytes      = 1 << 20
	MaxQueryRows       = 1000
	MaxPageRows        = 200
	MaxColumns         = 256
	MaxCursors         = 8
	MaxPendingRequests = 64
	MaxAggregateBytes  = 1 << 20
	MaxFieldBytes      = 4096
	MaxRequestTimeout  = 15 * time.Second
	MaxSessionLifetime = 60 * time.Second
)

var ErrInvalidQueryRowLimit = errors.New("Mapepire query row limit must be between 1 and 1000")
var ErrSessionClosed = errors.New("Mapepire session is closed")
var ErrProtocolViolation = errors.New("Mapepire protocol violation")
var ErrLimitExceeded = errors.New("Mapepire request or response exceeds a release limit")

const (
	OperationGetVersion        = "getversion"
	OperationConnect           = "connect"
	OperationPrepareSQLExecute = "prepare_sql_execute"
	OperationSQLMore           = "sqlmore"
	OperationSQLClose          = "sqlclose"
	OperationPing              = "ping"
	OperationExit              = "exit"
)

type Request struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Technique   string   `json:"technique,omitempty"`
	Application string   `json:"application,omitempty"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	Props       string   `json:"props,omitempty"`
	SQL         string   `json:"sql,omitempty"`
	Rows        int      `json:"rows,omitempty"`
	Parameters  []string `json:"parameters,omitempty"`
	ContID      string   `json:"cont_id,omitempty"`
}

func NewRequestID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate Mapepire request ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func ValidateRequest(r Request) error {
	if r.Type == "" || len(r.Type) > MaxFieldBytes {
		return ErrProtocolViolation
	}
	if len(r.Application) > MaxFieldBytes || len(r.SQL) > MaxFieldBytes || len(r.Props) > MaxFieldBytes || len(r.ContID) > MaxFieldBytes {
		return ErrLimitExceeded
	}
	if r.Rows < 0 || (r.Type == OperationSQLMore && r.Rows > MaxPageRows) || (r.Type == OperationPrepareSQLExecute && r.Rows > MaxQueryRows) {
		return ErrLimitExceeded
	}
	switch r.Type {
	case OperationGetVersion, OperationPing, OperationExit:
		if r.Application != "" || r.Username != "" || r.Password != "" || r.SQL != "" || r.Rows != 0 || r.ContID != "" || len(r.Parameters) != 0 {
			return ErrProtocolViolation
		}
	case OperationConnect:
		if r.Application == "" || len(r.Username) > MaxFieldBytes || len(r.Password) > MaxFieldBytes || r.SQL != "" || r.Rows != 0 || r.ContID != "" || len(r.Parameters) != 0 {
			return ErrProtocolViolation
		}
	case OperationPrepareSQLExecute:
		if r.Username != "" || r.Password != "" || r.SQL == "" || r.Rows < 1 || r.ContID != "" {
			return ErrProtocolViolation
		}
	case OperationSQLMore:
		if r.Username != "" || r.Password != "" || r.ContID == "" || r.Rows < 1 {
			return ErrProtocolViolation
		}
	case OperationSQLClose:
		if r.Username != "" || r.Password != "" || r.ContID == "" || r.SQL != "" || r.Rows != 0 {
			return ErrProtocolViolation
		}
	default:
		return fmt.Errorf("%w: unsupported operation", ErrProtocolViolation)
	}
	for _, p := range r.Parameters {
		if len(p) > MaxFieldBytes {
			return ErrLimitExceeded
		}
	}
	return nil
}

func ConnectRequest(id, application string) Request {
	return Request{ID: id, Type: "connect", Technique: "tcp", Application: application, Props: "access=read only"}
}

func AuthenticatedConnectRequest(id, application, username string, password []byte) Request {
	return Request{ID: id, Type: OperationConnect, Technique: "tcp", Application: application, Username: username, Password: string(password), Props: "access=read only"}
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
