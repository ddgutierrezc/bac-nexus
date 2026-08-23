package mapepire

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	MaxFrameBytes = 1 << 20
	MaxQueryRows  = 1000
)

var ErrInvalidQueryRowLimit = errors.New("Mapepire query row limit must be between 1 and 1000")

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

func ConnectRequest(id, application string) Request {
	return Request{ID: id, Type: "connect", Technique: "tcp", Application: application, Props: "access=read only"}
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
